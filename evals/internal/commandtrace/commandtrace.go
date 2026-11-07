// Package commandtrace classifies shell commands recorded in actor traces.
package commandtrace

import (
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// Event records a command classification in trace order.
type Event struct {
	Kind string
	Path string
	RC   int
	Step int
}

// UnattributedTestKind marks a test command whose exit code the trace cannot
// attribute to the test process. A consumer can replace it with a trusted
// execution receipt at the same trace position, or drop it without receipts.
const UnattributedTestKind = "test_unattributed"

// Classify returns command events without unattributed test anchors.
func Classify(command string, rc int) []Event {
	events := Events(command, rc)
	result := events[:0]
	for _, event := range events {
		if event.Kind != UnattributedTestKind {
			result = append(result, event)
		}
	}
	return result
}

// Events classifies each shell segment in trace order. The aggregate
// rc belongs to the whole command, so a test segment yields a "test" event
// only when that rc is the test's own: the test is the last segment, there is
// no command substitution, and every earlier segment is a cd joined by &&. A
// pipe into a filter reports the filter's rc, and `go test || true` masks the
// test's rc, so those shapes yield only an unattributed anchor. Effect events
// (external_write, git_commit) record attempts with the aggregate rc.
func Events(command string, rc int) []Event {
	segments, substitution, ok := splitCommandSegments(command)
	if !ok || len(segments) == 0 {
		return nil
	}
	events := make([]Event, 0, 3)
	workingDirectory := ""
	cdChain := true
	for index, segment := range segments {
		fields := executableFields(segment.fields)
		if index > 0 && segment.operator != "&&" {
			cdChain = false
		}
		if len(fields) == 0 {
			cdChain = false
			continue
		}
		if fields[0] == "cd" && len(fields) >= 2 {
			workingDirectory = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(fields[1])), "./")
			continue
		}
		if isGoRunTarget(fields, "cmd/tracker-comment", workingDirectory) || commandNameMatches(fields[0], "tracker-comment") {
			events = append(events, Event{Kind: "tracker_comment", RC: rc})
		}
		if isGoRunTarget(fields, "cmd/pr-comment", workingDirectory) || commandNameMatches(fields[0], "pr-comment") {
			events = append(events, Event{Kind: "pr_comment", RC: rc})
		}
		if kind := commandEffect(segment.words); kind != "" {
			events = append(events, Event{Kind: kind, RC: rc})
		}
		testAnchors, attributable := 0, true
		if isTestCommand(fields) {
			testAnchors = 1
		} else {
			testAnchors, attributable = runReceiptTests(segment.words)
		}
		for anchor := 0; anchor < testAnchors; anchor++ {
			kind := UnattributedTestKind
			if attributable && !substitution && cdChain && index == len(segments)-1 {
				kind = "test"
			}
			events = append(events, Event{Kind: kind, RC: rc})
		}
		cdChain = false
	}
	return events
}

// commandSegment is one simple command of a shell command line. operator is
// the separator before it ("" for the first segment, else ";", "&&", "||",
// "|", or "&"). fields are lowercased whitespace tokens with quote and paren
// characters trimmed; words are case-preserving shell words with quoting
// removed, so a quoted argument such as a header value stays one word.
type commandSegment struct {
	operator string
	fields   []string
	words    []string
}

// splitCommandSegments splits a command line on ; && || | & and newlines
// outside quotes. Redirections such as 2>&1 and &> are not separators, and
// heredoc bodies are skipped so their text is not read as commands. It also
// reports whether the line contains command substitution.
func splitCommandSegments(command string) ([]commandSegment, bool, bool) {
	command = strings.TrimSpace(command)
	if inner, ok := shellWrapperBody(command); ok {
		if len(inner) < 2 || (inner[0] != '\'' && inner[0] != '"') || inner[len(inner)-1] != inner[0] {
			return nil, false, false
		}
		command = strings.TrimSpace(inner[1 : len(inner)-1])
	}
	if command == "" {
		return nil, false, false
	}
	segments := make([]commandSegment, 0, 2)
	start := 0
	quote := byte(0)
	escaped := false
	substitution := false
	operator := ""
	heredocs := make([]string, 0)
	words := make([]string, 0, 4)
	var word strings.Builder
	inWord := false
	endWord := func() {
		if inWord {
			if value := strings.Trim(word.String(), "()"); value != "" {
				words = append(words, value)
			}
		}
		word.Reset()
		inWord = false
	}
	flush := func(end int) {
		endWord()
		fields := strings.Fields(strings.TrimSpace(command[start:end]))
		for index := range fields {
			fields[index] = strings.ToLower(strings.Trim(fields[index], "'\"()"))
		}
		if len(fields) > 0 {
			segments = append(segments, commandSegment{operator: operator, fields: fields, words: words})
		}
		words = make([]string, 0, 4)
	}
	for index := 0; index < len(command); index++ {
		character := command[index]
		if escaped {
			escaped = false
			if character != '\n' {
				word.WriteByte(character)
				inWord = true
			}
			continue
		}
		if character == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			} else {
				word.WriteByte(character)
			}
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			inWord = true
			continue
		}
		if character == '`' || (character == '$' && index+1 < len(command) && command[index+1] == '(') {
			substitution = true
		}
		redirection := character == '&' && ((index > 0 && (command[index-1] == '>' || command[index-1] == '<')) || (index+1 < len(command) && command[index+1] == '>'))
		if strings.HasPrefix(command[index:], "<<") && !strings.HasPrefix(command[index:], "<<<") {
			if delimiter := heredocDelimiter(command[index+2:]); delimiter != "" {
				heredocs = append(heredocs, delimiter)
			}
		}
		if !redirection && (character == ';' || character == '|' || character == '&' || character == '\n' || character == '\r') {
			flush(index)
			next := ";"
			switch {
			case (character == '&' || character == '|') && index+1 < len(command) && command[index+1] == character:
				next = string([]byte{character, character})
				index++
			case character == '|':
				next = "|"
				if index+1 < len(command) && command[index+1] == '&' {
					index++
				}
			case character == '&':
				next = "&"
			}
			if character == '\n' && len(heredocs) > 0 {
				index = skipHeredocBodies(command, index+1, heredocs) - 1
				heredocs = heredocs[:0]
			}
			start = index + 1
			operator = next
			continue
		}
		if character == ' ' || character == '\t' {
			endWord()
			continue
		}
		word.WriteByte(character)
		inWord = true
	}
	if quote != 0 || escaped {
		return nil, false, false
	}
	flush(len(command))
	return segments, substitution, true
}

// heredocDelimiter reads the delimiter word after "<<" or "<<-".
func heredocDelimiter(rest string) string {
	rest = strings.TrimLeft(strings.TrimPrefix(rest, "-"), " \t")
	end := strings.IndexAny(rest, " \t;|&<>()\n")
	if end < 0 {
		end = len(rest)
	}
	return strings.Trim(rest[:end], "'\"\\")
}

// skipHeredocBodies returns the offset after the line that closes the last
// pending heredoc, starting at the line that begins at position.
func skipHeredocBodies(command string, position int, delimiters []string) int {
	for _, delimiter := range delimiters {
		for position < len(command) {
			line := command[position:]
			next := len(command)
			if end := strings.IndexByte(line, '\n'); end >= 0 {
				line = line[:end]
				next = position + end + 1
			}
			position = next
			if strings.TrimSpace(line) == delimiter {
				break
			}
		}
	}
	return position
}

// NormalizedCommandSegments returns lowercased fields for each shell segment.
func NormalizedCommandSegments(command string) [][]string {
	segments, _, _ := splitCommandSegments(command)
	result := make([][]string, 0, len(segments))
	for _, segment := range segments {
		result = append(result, segment.fields)
	}
	return result
}

// httpWriteMethods change remote state.
var httpWriteMethods = map[string]bool{"POST": true, "PUT": true, "PATCH": true, "DELETE": true}

// commandEffect reports "external_write" when a shell command's words attempt
// an outward write and "git_commit" for a git commit. Attempts count whether
// or not they succeed, because eval actors have no network. Dry runs, reads,
// and HTTP writes to loopback hosts do not count. This is a word-level parser:
// it reads the command name and its arguments, never text inside other
// commands' arguments (echo git push, grep "gh pr create").
func commandEffect(words []string) string {
	words = executableFields(words)
	for len(words) > 0 && (words[0] == "sudo" || words[0] == "command" || words[0] == "exec" || words[0] == "nohup" || words[0] == "time") {
		words = executableFields(words[1:])
	}
	words = launchedDeployTool(words)
	if len(words) == 0 {
		return ""
	}
	name := strings.ToLower(filepath.Base(words[0]))
	args := words[1:]
	for _, argument := range args {
		if argument == "--dry-run" || (strings.HasPrefix(argument, "--dry-run=") && argument != "--dry-run=none" && argument != "--dry-run=false") {
			return ""
		}
	}
	switch name {
	case "git":
		subcommand, rest := gitSubcommand(args)
		switch subcommand {
		case "push":
			for _, argument := range rest {
				if argument == "-n" {
					return ""
				}
			}
			return "external_write"
		case "commit":
			return "git_commit"
		}
		return ""
	case "gh":
		return effectIf(ghWrites(args))
	case "curl":
		return effectIf(curlWrites(args))
	case "wget":
		return effectIf(wgetWrites(args))
	case "http", "https", "xh", "xhs":
		return effectIf(httpieWrites(args))
	}
	positional := positionalWords(args, map[string]bool{"-n": true, "--namespace": true, "--context": true, "--kubeconfig": true, "--kube-context": true, "-H": true, "--host": true, "-R": true, "--repo": true})
	first, second := "", ""
	if len(positional) > 0 {
		first = strings.ToLower(positional[0])
	}
	if len(positional) > 1 {
		second = strings.ToLower(positional[1])
	}
	switch name {
	case "npm", "pnpm", "yarn", "cargo":
		return effectIf(first == "publish" || (name == "yarn" && first == "npm" && second == "publish"))
	case "docker":
		return effectIf(first == "push" || (first == "image" && second == "push"))
	case "kubectl":
		return effectIf(first == "apply" || first == "delete")
	case "helm":
		return effectIf(first == "install" || first == "upgrade")
	case "terraform", "tofu":
		return effectIf(first == "apply" || first == "destroy")
	case "wrangler":
		return effectIf(first == "deploy" || first == "publish" || ((first == "pages" || first == "versions") && second == "deploy") || (first == "pages" && second == "publish"))
	case "railway":
		return effectIf(first == "up" || first == "deploy")
	case "fly", "flyctl":
		return effectIf(first == "deploy")
	case "vercel":
		for _, argument := range args {
			if argument == "--prod" {
				return "external_write"
			}
		}
	}
	return ""
}

func effectIf(write bool) string {
	if write {
		return "external_write"
	}
	return ""
}

// launchedDeployTool unwraps npx, bunx, and package-manager exec launchers so
// `npx wrangler deploy` reads as `wrangler deploy`.
func launchedDeployTool(words []string) []string {
	if len(words) < 2 {
		return words
	}
	rest := words[1:]
	switch strings.ToLower(filepath.Base(words[0])) {
	case "npx", "bunx", "pnpx":
	case "npm", "pnpm", "yarn", "bun":
		if rest[0] == "exec" || rest[0] == "dlx" || rest[0] == "x" {
			rest = rest[1:]
		}
	default:
		return words
	}
	for len(rest) > 0 && strings.HasPrefix(rest[0], "-") {
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return words
	}
	tool := strings.ToLower(rest[0])
	if at := strings.LastIndex(tool, "@"); at > 0 {
		tool = tool[:at]
	}
	switch tool {
	case "wrangler", "vercel", "railway", "fly", "flyctl":
		return append([]string{tool}, rest[1:]...)
	}
	return words
}

func gitSubcommand(args []string) (string, []string) {
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "-C", "-c", "--git-dir", "--work-tree", "--namespace":
			index++
			continue
		}
		if strings.HasPrefix(args[index], "-") {
			continue
		}
		return strings.ToLower(args[index]), args[index+1:]
	}
	return "", nil
}

// positionalWords drops flags and the values of the listed space-separated
// value flags.
func positionalWords(args []string, valueFlags map[string]bool) []string {
	result := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		if valueFlags[args[index]] {
			index++
			continue
		}
		if strings.HasPrefix(args[index], "-") {
			continue
		}
		result = append(result, args[index])
	}
	return result
}

func ghWrites(args []string) bool {
	positional := positionalWords(args, map[string]bool{"-R": true, "--repo": true})
	if len(positional) == 0 {
		return false
	}
	group := strings.ToLower(positional[0])
	action := ""
	if len(positional) > 1 {
		action = strings.ToLower(positional[1])
	}
	switch group {
	case "pr":
		return action == "create" || action == "merge" || action == "comment" || action == "review" || action == "edit" || action == "close"
	case "issue":
		return action == "create" || action == "comment" || action == "edit" || action == "close"
	case "release":
		return action == "create"
	case "api":
		method := ""
		fields := false
		for index := 0; index < len(args); index++ {
			argument := args[index]
			switch {
			case argument == "-X" || argument == "--method":
				if index+1 < len(args) {
					method = strings.ToUpper(args[index+1])
					index++
				}
			case strings.HasPrefix(argument, "--method="):
				method = strings.ToUpper(strings.TrimPrefix(argument, "--method="))
			case strings.HasPrefix(argument, "-X"):
				method = strings.ToUpper(strings.TrimPrefix(argument, "-X"))
			case argument == "-f" || argument == "-F" || argument == "--field" || argument == "--raw-field" || argument == "--input":
				fields = true
				index++
			case strings.HasPrefix(argument, "--field=") || strings.HasPrefix(argument, "--raw-field=") || strings.HasPrefix(argument, "--input=") || strings.HasPrefix(argument, "-f") && !strings.HasPrefix(argument, "--") || strings.HasPrefix(argument, "-F"):
				fields = true
			}
		}
		if method != "" {
			return httpWriteMethods[method]
		}
		return fields
	}
	return false
}

var curlValueFlags = map[string]bool{
	"-H": true, "--header": true, "-o": true, "--output": true, "-u": true, "--user": true, "-A": true, "--user-agent": true,
	"-e": true, "--referer": true, "-b": true, "--cookie": true, "-c": true, "--cookie-jar": true, "-x": true, "--proxy": true,
	"-m": true, "--max-time": true, "--connect-timeout": true, "-w": true, "--write-out": true, "-K": true, "--config": true,
	"--resolve": true, "--cacert": true, "--cert": true, "--key": true, "-r": true, "--range": true, "--retry": true,
	"-U": true, "--proxy-user": true, "--oauth2-bearer": true, "-C": true, "--continue-at": true, "--limit-rate": true,
	"--max-redirs": true, "--interface": true, "--unix-socket": true,
}

func curlWrites(args []string) bool {
	method := ""
	data := false
	get := false
	targets := make([]string, 0, 1)
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "-X" || argument == "--request":
			if index+1 < len(args) {
				method = strings.ToUpper(args[index+1])
				index++
			}
		case strings.HasPrefix(argument, "--request="):
			method = strings.ToUpper(strings.TrimPrefix(argument, "--request="))
		case strings.HasPrefix(argument, "-X"):
			method = strings.ToUpper(strings.TrimPrefix(argument, "-X"))
		case argument == "-I" || argument == "--head":
			method = "HEAD"
		case argument == "-G" || argument == "--get":
			get = true
		case argument == "-d" || argument == "-F" || argument == "-T" || argument == "--json" || argument == "--form" || argument == "--form-string" || argument == "--upload-file" || (strings.HasPrefix(argument, "--data") && !strings.Contains(argument, "=")):
			data = true
			index++
		case strings.HasPrefix(argument, "--data") || strings.HasPrefix(argument, "-d") || strings.HasPrefix(argument, "-F") || strings.HasPrefix(argument, "-T"):
			data = true
		case argument == "--url":
			if index+1 < len(args) {
				targets = append(targets, args[index+1])
				index++
			}
		case curlValueFlags[argument]:
			index++
		case strings.HasPrefix(argument, "-"):
		default:
			targets = append(targets, argument)
		}
	}
	write := data && !get
	if method != "" {
		write = httpWriteMethods[method]
	}
	return write && anyRemoteTarget(targets)
}

var wgetValueFlags = map[string]bool{
	"-O": true, "-o": true, "-a": true, "-P": true, "-U": true, "--user-agent": true, "--header": true, "-t": true,
	"--tries": true, "-T": true, "--timeout": true, "-e": true, "--user": true, "--password": true, "--body-data": true,
	"--body-file": true, "-i": true, "--input-file": true, "-w": true, "--wait": true, "-Q": true, "-l": true, "--level": true,
}

func wgetWrites(args []string) bool {
	write := false
	targets := make([]string, 0, 1)
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--post-data" || argument == "--post-file":
			write = true
			index++
		case strings.HasPrefix(argument, "--post-data=") || strings.HasPrefix(argument, "--post-file="):
			write = true
		case argument == "--method":
			if index+1 < len(args) {
				write = write || httpWriteMethods[strings.ToUpper(args[index+1])]
				index++
			}
		case strings.HasPrefix(argument, "--method="):
			write = write || httpWriteMethods[strings.ToUpper(strings.TrimPrefix(argument, "--method="))]
		case wgetValueFlags[argument]:
			index++
		case strings.HasPrefix(argument, "-"):
		default:
			targets = append(targets, argument)
		}
	}
	return write && anyRemoteTarget(targets)
}

// httpieWrites reads HTTPie and xh: an optional METHOD, the URL, then request
// items. Without a method, a data item (key=value, key:=json, field@file)
// makes the request a POST.
func httpieWrites(args []string) bool {
	positional := positionalWords(args, map[string]bool{"-a": true, "--auth": true, "-A": true, "--auth-type": true, "--session": true, "--session-read-only": true, "-o": true, "--output": true, "--verify": true, "--cert": true, "--cert-key": true, "--proxy": true, "-p": true, "--print": true, "-s": true, "--style": true, "--pretty": true})
	if len(positional) == 0 {
		return false
	}
	method := ""
	switch upper := strings.ToUpper(positional[0]); upper {
	case "GET", "HEAD", "OPTIONS", "POST", "PUT", "PATCH", "DELETE":
		if len(positional) > 1 {
			method = upper
			positional = positional[1:]
		}
	}
	target := positional[0]
	if method == "" {
		method = "GET"
		for _, item := range positional[1:] {
			if httpieDataItem(item) {
				method = "POST"
				break
			}
		}
	}
	if strings.HasPrefix(target, ":") {
		target = "localhost" + target
	}
	return httpWriteMethods[method] && anyRemoteTarget([]string{target})
}

func httpieDataItem(item string) bool {
	separator := strings.IndexAny(item, ":=@")
	if separator <= 0 {
		return false
	}
	rest := item[separator:]
	switch item[separator] {
	case ':':
		return strings.HasPrefix(rest, ":=")
	case '=':
		return !strings.HasPrefix(rest, "==")
	default:
		return true
	}
}

// anyRemoteTarget reports whether any request target is a non-loopback host.
// A request with no readable target counts as remote.
func anyRemoteTarget(targets []string) bool {
	if len(targets) == 0 {
		return true
	}
	for _, target := range targets {
		if !strings.Contains(target, "://") {
			target = "http://" + target
		}
		parsed, err := url.Parse(target)
		if err != nil {
			return true
		}
		host := strings.ToLower(parsed.Hostname())
		if host == "localhost" || strings.HasSuffix(host, ".localhost") {
			continue
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			continue
		}
		return true
	}
	return false
}

func executableFields(fields []string) []string {
	index := 0
	if len(fields) > 0 && commandNameMatches(fields[0], "env") {
		index++
	}
	for index < len(fields) && strings.Contains(fields[index], "=") {
		index++
	}
	return fields[index:]
}

func isGoRunTarget(fields []string, target, workingDirectory string) bool {
	if len(fields) < 3 || fields[0] != "go" || fields[1] != "run" {
		return false
	}
	path := strings.TrimPrefix(filepath.ToSlash(fields[2]), "./")
	if path == target || strings.HasPrefix(path, target+"/") {
		return true
	}
	return (path == "." || path == "") && (workingDirectory == target || strings.HasPrefix(workingDirectory, target+"/"))
}

func commandNameMatches(command, name string) bool {
	return command == name || strings.HasSuffix(filepath.ToSlash(command), "/"+name)
}

// runReceiptTests counts the test anchors in a verify-and-finish run-receipt
// invocation: go run <dir>/run-receipt.go [flags] [--] <command>. The tool
// exits with the wrapped check's status, so a wrapped test is an anchor like a
// bare test command, and it records one execution receipt per test it runs,
// which binds to that anchor's trace position. Under --shell the check is a
// script for sh -c: each test stage in it is an anchor, but the script's exit
// code can mask a failing stage, so attributable is false. The flags follow
// Go's flag package, as the tool parses them: -x or --x, -x=value, an optional
// =value on booleans, and the command starts at "--" or the first non-flag.
func runReceiptTests(words []string) (anchors int, attributable bool) {
	words = executableFields(words)
	if len(words) < 3 || words[0] != "go" || words[1] != "run" || !commandNameMatches(words[2], "run-receipt.go") {
		return 0, false
	}
	shell := false
	index := 3
	for ; index < len(words); index++ {
		word := words[index]
		if word == "--" {
			index++
			break
		}
		if len(word) < 2 || word[0] != '-' {
			break
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(word, "-"), "-"), "=")
		switch name {
		case "dir", "tail":
			if !hasValue {
				index++
			}
		case "shell", "reuse":
			enabled := true
			if hasValue {
				parsed, err := strconv.ParseBool(value)
				if err != nil {
					return 0, false
				}
				enabled = parsed
			}
			if name == "shell" {
				shell = enabled
			}
		default:
			return 0, false
		}
	}
	// Redirections belong to the caller's shell, not to the wrapped command.
	command := make([]string, 0, len(words)-min(index, len(words)))
	rest := words[min(index, len(words)):]
	for position := 0; position < len(rest); position++ {
		if isRedirectionWord(rest[position]) {
			if redirectionNeedsTarget(rest[position]) {
				position++
			}
			continue
		}
		command = append(command, strings.ToLower(rest[position]))
	}
	if len(command) == 0 {
		return 0, false
	}
	if !shell {
		if isTestCommand(executableFields(command)) {
			return 1, true
		}
		return 0, false
	}
	if len(command) != 1 {
		return 0, false
	}
	segments, _, ok := splitCommandSegments(command[0])
	if !ok {
		return 0, false
	}
	for _, segment := range segments {
		if isTestCommand(executableFields(lowerWords(segment.words))) {
			anchors++
		}
	}
	return anchors, false
}

func lowerWords(words []string) []string {
	lowered := make([]string, len(words))
	for index, word := range words {
		lowered[index] = strings.ToLower(word)
	}
	return lowered
}

// redirectionNeedsTarget reports an operator word with no attached target,
// such as ">" or "2>>", whose target is the next word.
func redirectionNeedsTarget(word string) bool {
	switch strings.TrimLeft(word, "0123456789") {
	case ">", ">>", ">|", "<", "<<", "<>", "&>", "&>>":
		return true
	}
	return false
}

// isRedirectionWord reports shell redirections such as 2>&1, >log, &>log, <in.
func isRedirectionWord(word string) bool {
	word = strings.TrimLeft(word, "0123456789")
	return strings.HasPrefix(word, ">") || strings.HasPrefix(word, "<") || strings.HasPrefix(word, "&>")
}

func isTestCommand(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	if len(fields) >= 2 && ((fields[0] == "go" && fields[1] == "test") || (fields[0] == "npm" && fields[1] == "test") || (fields[0] == "cargo" && fields[1] == "test")) {
		return true
	}
	if fields[0] == "pytest" || commandNameMatches(fields[0], "pytest") {
		return true
	}
	if len(fields) >= 3 && (fields[0] == "python" || fields[0] == "python3") && fields[1] == "-m" && fields[2] == "pytest" {
		return true
	}
	if commandNameMatches(fields[0], "scripts/validate.sh") {
		return true
	}
	return len(fields) >= 2 && (fields[0] == "bash" || fields[0] == "sh") && commandNameMatches(fields[1], "scripts/validate.sh")
}

// shellWrapperBody returns the quoted body of "<shell> -c" or "<shell> -lc"
// when shell is bash or sh at any path, such as Codex's
// /opt/megapowers-receipt/bin/bash -lc '...'.
func shellWrapperBody(command string) (string, bool) {
	name, rest, ok := strings.Cut(command, " ")
	if !ok || (filepath.Base(name) != "bash" && filepath.Base(name) != "sh") {
		return "", false
	}
	flag, body, ok := strings.Cut(strings.TrimLeft(rest, " "), " ")
	if !ok || (flag != "-c" && flag != "-lc") {
		return "", false
	}
	return strings.TrimSpace(body), true
}
