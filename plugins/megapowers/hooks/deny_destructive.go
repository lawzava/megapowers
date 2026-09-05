package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const (
	maxCommandBytes = 16_000
	maxScanDepth    = 8
)

type decision struct {
	Deny   bool
	Reason string
}

var (
	assignmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	redirectPattern   = regexp.MustCompile(`^([0-9]*(&?>>?|<{1,3}>?)|[0-9]*[<>]&[0-9-]*)`)
	parentPattern     = regexp.MustCompile(`^\.\.(?:/\.\.)*(?:/\*)?$`)
	broadModePattern  = regexp.MustCompile(`^([ugoa]*)([-+=])([rwxXst]*)$`)
	driveRootPattern  = regexp.MustCompile(`^[A-Za-z]:[\\/]?(?:/\*)?$`)
	rawDeviceRedirect = regexp.MustCompile(`(^|[^<])>[[:space:]]*/dev/(sd|nvme|vd|xvd|mmcblk|disk|rdisk|mapper/|dm-|md[0-9]|md/|loop[0-9]|mtdblock|hd|sr|vblk|rbd|nbd|drbd|pmem|zvol/)`)
	forkBombPattern   = regexp.MustCompile(`[A-Za-z_:][A-Za-z0-9_:]*[[:space:]]*\(\)[[:space:]]*\{[[:space:]]*[^|;&{}]+\|[^|;&{}]*&[[:space:]]*\}`)
)

var systemRoots = map[string]struct{}{
	"Applications": {}, "Library": {}, "Network": {}, "System": {}, "Users": {}, "Volumes": {},
	"bin": {}, "boot": {}, "cores": {}, "dev": {}, "etc": {}, "home": {}, "lib": {}, "lib32": {},
	"lib64": {}, "libx32": {}, "opt": {}, "private": {}, "proc": {}, "root": {}, "run": {}, "sbin": {},
	"srv": {}, "sys": {}, "usr": {}, "var": {},
}

var wrappers = map[string]struct{}{
	"command": {}, "doas": {}, "env": {}, "exec": {}, "ionice": {}, "nice": {}, "nohup": {},
	"setsid": {}, "stdbuf": {}, "sudo": {}, "time": {}, "timeout": {}, "xargs": {},
}

var wrapperValueOptions = map[string]struct{}{
	"doas:-u": {}, "env:--chdir": {}, "env:--unset": {}, "env:-C": {}, "env:-u": {}, "exec:-a": {},
	"ionice:-c": {}, "ionice:-n": {}, "ionice:-p": {}, "nice:-n": {}, "stdbuf:-e": {}, "stdbuf:-i": {},
	"stdbuf:-o": {}, "sudo:-C": {}, "sudo:-U": {}, "sudo:-g": {}, "sudo:-h": {}, "sudo:-p": {},
	"sudo:-r": {}, "sudo:-t": {}, "sudo:-u": {}, "timeout:--kill-after": {}, "timeout:--signal": {},
	"timeout:-k": {}, "timeout:-s": {}, "xargs:-E": {}, "xargs:-I": {}, "xargs:-L": {}, "xargs:-P": {},
	"xargs:-a": {}, "xargs:-d": {}, "xargs:-n": {}, "xargs:-s": {},
}

func classifyCommand(command, home string) decision {
	if len(command) > maxCommandBytes {
		return decision{}
	}
	return analyzeCommand(command, home, 0)
}

func analyzeCommand(command, home string, depth int) decision {
	verdict, payloads := scanLevel(command, home)
	if verdict.Deny {
		return verdict
	}
	if depth >= maxScanDepth {
		return decision{}
	}
	for _, payload := range payloads {
		if payload == "" {
			continue
		}
		if nested := analyzeCommand(payload, home, depth+1); nested.Deny {
			return nested
		}
	}
	return decision{}
}

func scanLevel(command, home string) (decision, []string) {
	var payloads []string
	for _, segment := range splitSegments(command) {
		name, tail, ok := resolveCommand(segment)
		if !ok {
			continue
		}

		if strings.EqualFold(name, "remove-item") {
			name = "remove-item"
		} else if strings.EqualFold(name, "rd") {
			name = "rd"
		}

		switch name {
		case "rm":
			if rmIsCatastrophic(tail, home) {
				return decision{true, "recursive rm of a root, home, or system directory. Delete a specific subdirectory instead (e.g. rm -rf ./dist)."}, nil
			}
		case "find":
			catastrophic, nested := findIsCatastrophic(tail, home)
			payloads = append(payloads, nested...)
			if catastrophic {
				return decision{true, "find deleting or shredding from a root, home, or system start path. Use a specific relative start path."}, nil
			}
		case "chmod":
			if chmodIsCatastrophic(tail, home) {
				return decision{true, "chmod 777 on a root/system path."}, nil
			}
		case "dd":
			if ddIsCatastrophic(tail) {
				return decision{true, "dd writing to a raw block device (would overwrite a disk)."}, nil
			}
		case "mkfs", "wipefs", "blkdiscard", "shred", "truncate":
			if formatIsCatastrophic(tail) {
				return decision{true, name + " targeting a block device (would wipe a disk). Against a plain file this is allowed."}, nil
			}
		case "cp":
			if cpIsCatastrophic(tail) {
				return decision{true, "cp overwriting a raw block device (would clobber a disk)."}, nil
			}
		case "remove-item", "rd":
			if recursiveRemoveIsCatastrophic(tail, home) {
				return decision{true, "recursive Remove-Item/rd of a root, home, or system path. Delete a specific subdirectory instead."}, nil
			}
		case "bash", "sh", "zsh", "dash", "ash", "ksh":
			words, ok := shellWords(tail)
			if !ok {
				break
			}
			take := false
			for _, word := range words {
				if take {
					if word != "" {
						payloads = append(payloads, word)
					}
					take = false
					continue
				}
				if word == "-c" || shellFlagRunsCommand(word) {
					take = true
				}
			}
		case "eval":
			if words, ok := shellWords(tail); ok {
				if payload := strings.Join(nonempty(words), " "); payload != "" {
					payloads = append(payloads, payload)
				}
			}
		case "ssh":
			if words, ok := shellWords(tail); ok {
				if payload := sshPayload(words); payload != "" {
					payloads = append(payloads, payload)
				}
			}
		default:
			if strings.HasPrefix(name, "mkfs.") && formatIsCatastrophic(tail) {
				return decision{true, name + " targeting a block device (would wipe a disk). Against a plain file this is allowed."}, nil
			}
		}
	}

	dequoted := stripQuoted(command)
	if rawDeviceRedirect.MatchString(dequoted) {
		return decision{true, "redirect to a raw block device (would overwrite a disk)"}, nil
	}
	if forkBombPattern.MatchString(dequoted) {
		return decision{true, "fork bomb"}, nil
	}
	return decision{}, payloads
}

func splitSegments(command string) []string {
	segments := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(command); {
		if command[i] == '\'' || command[i] == '"' || command[i] == '`' {
			quote := command[i]
			i++
			for i < len(command) && command[i] != quote {
				i++
			}
			if i < len(command) {
				i++
			}
			continue
		}
		if command[i] == ';' || command[i] == '&' || command[i] == '|' || command[i] == '\n' {
			segments = append(segments, command[start:i])
			separator := command[i]
			i++
			if i < len(command) && command[i] == separator && (separator == '&' || separator == '|') {
				i++
			}
			start = i
			continue
		}
		i++
	}
	segments = append(segments, command[start:])
	return segments
}

func shellWords(text string) ([]string, bool) {
	var words []string
	var token strings.Builder
	inToken := false
	for i := 0; i < len(text); {
		if text[i] == '\'' || text[i] == '"' {
			quote := text[i]
			inToken = true
			i++
			start := i
			for i < len(text) && text[i] != quote {
				i++
			}
			if i == len(text) {
				return nil, false
			}
			token.WriteString(text[start:i])
			i++
			continue
		}
		if isSpace(text[i]) {
			if inToken {
				words = append(words, token.String())
				token.Reset()
				inToken = false
			}
			i++
			continue
		}
		token.WriteByte(text[i])
		inToken = true
		i++
	}
	if inToken {
		words = append(words, token.String())
	}
	return words, true
}

func resolveCommand(segment string) (string, string, bool) {
	text := trimLeftSpace(segment)
	for {
		word, rest, ok := takeWord(text)
		if !ok {
			return "", "", false
		}
		if redirectPattern.MatchString(word) || assignmentPattern.MatchString(word) {
			text = trimLeftSpace(rest)
			continue
		}
		if _, ok := wrappers[word]; !ok {
			break
		}

		wrapper := word
		text = trimLeftSpace(rest)
		for {
			option, after, ok := takeWord(text)
			if !ok || (!strings.HasPrefix(option, "-") && !assignmentPattern.MatchString(option)) {
				break
			}
			text = trimLeftSpace(after)
			if _, takesValue := wrapperValueOptions[wrapper+":"+option]; takesValue {
				_, afterValue, hasValue := takeWord(text)
				if hasValue {
					text = trimLeftSpace(afterValue)
				}
			}
		}
		if wrapper == "timeout" {
			_, afterDuration, ok := takeWord(text)
			if ok {
				text = trimLeftSpace(afterDuration)
			}
		}
	}

	word, rest, ok := takeWord(text)
	if !ok {
		return "", "", false
	}
	word = strings.TrimLeft(word, `\`)
	return filepath.Base(word), rest, true
}

func takeWord(text string) (string, string, bool) {
	if text == "" {
		return "", "", false
	}
	for i := 0; i < len(text); i++ {
		if isSpace(text[i]) {
			return text[:i], text[i:], true
		}
	}
	return text, "", true
}

func trimLeftSpace(text string) string {
	return strings.TrimLeftFunc(text, unicode.IsSpace)
}

func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

func normalizePath(path string) string {
	absolute := strings.HasPrefix(path, "/")
	parts := make([]string, 0, strings.Count(path, "/")+1)
	for _, part := range strings.Split(path, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(parts) == 0 {
				if !absolute {
					parts = append(parts, part)
				}
				continue
			}
			previous := parts[len(parts)-1]
			if previous == ".." || strings.ContainsAny(previous, "$*") || strings.HasPrefix(previous, "~") {
				parts = append(parts, part)
				continue
			}
			parts = parts[:len(parts)-1]
		default:
			parts = append(parts, part)
		}
	}
	normalized := strings.Join(parts, "/")
	if absolute {
		normalized = "/" + normalized
	}
	if normalized == "" {
		return "."
	}
	return normalized
}

func isCatastrophicTarget(target, home string) bool {
	target = strings.TrimLeft(target, `\`)
	target = strings.TrimSuffix(target, `\`)
	if target == "" {
		return false
	}
	target = normalizePath(target)
	if target == "/" || target == "/*" || target == "/.*" || target == "~" || target == "~/*" {
		return true
	}
	if target == "$HOME" || target == "${HOME}" || target == "$HOME/*" || target == "${HOME}/*" {
		return true
	}
	if suffix, ok := bracedHomeSuffix(target); ok && (suffix == "" || suffix == "/*") {
		return true
	}
	if target == "/private/etc" || target == "/private/etc/*" || target == "/private/var" || target == "/private/var/*" {
		return true
	}
	for root := range systemRoots {
		if target == "/"+root || target == "/"+root+"/*" {
			return true
		}
	}

	base := strings.TrimSuffix(target, "/*")
	parts := strings.Split(strings.TrimPrefix(base, "/"), "/")
	if strings.HasPrefix(base, "/") && len(parts) == 2 && (parts[0] == "home" || parts[0] == "Users") && parts[1] != "" {
		return true
	}
	if home != "" && home != "/" && base == strings.TrimSuffix(home, "/") {
		return true
	}
	if strings.HasPrefix(target, "~") && !strings.Contains(strings.TrimSuffix(target, "/*"), "/") {
		return true
	}
	if suffix, ok := homeSuffix(target); ok && parentPattern.MatchString(suffix) {
		return true
	}
	return parentPattern.MatchString(target)
}

func bracedHomeSuffix(target string) (string, bool) {
	if !strings.HasPrefix(target, "${HOME:") {
		return "", false
	}
	end := strings.IndexByte(target, '}')
	if end < 0 {
		return "", false
	}
	return target[end+1:], true
}

func homeSuffix(target string) (string, bool) {
	switch {
	case strings.HasPrefix(target, "$HOME/"):
		return strings.TrimPrefix(target, "$HOME/"), true
	case strings.HasPrefix(target, "${HOME}/"):
		return strings.TrimPrefix(target, "${HOME}/"), true
	case strings.HasPrefix(target, "${HOME:"):
		suffix, ok := bracedHomeSuffix(target)
		return strings.TrimPrefix(suffix, "/"), ok && strings.HasPrefix(suffix, "/")
	case strings.HasPrefix(target, "~"):
		slash := strings.IndexByte(target, '/')
		if slash >= 0 {
			return target[slash+1:], true
		}
	}
	return "", false
}

func rmIsCatastrophic(tail, home string) bool {
	words, ok := shellWords(tail)
	if !ok || !hasRecursiveFlag(words) {
		return false
	}
	endOptions := false
	for _, word := range words {
		if !endOptions && word == "--" {
			endOptions = true
			continue
		}
		if !endOptions && strings.HasPrefix(word, "-") {
			continue
		}
		if isCatastrophicTarget(word, home) {
			return true
		}
	}
	return false
}

func hasRecursiveFlag(words []string) bool {
	for _, word := range words {
		if word == "--recursive" {
			return true
		}
		if strings.HasPrefix(word, "--") {
			continue
		}
		if strings.HasPrefix(word, "-") && len(word) > 1 && strings.ContainsAny(word[1:], "rR") {
			return true
		}
	}
	return false
}

func findIsCatastrophic(tail, home string) (bool, []string) {
	words, ok := shellWords(tail)
	if !ok {
		return false, nil
	}
	inStarts, dangerStart, deny := true, false, false
	inExec, firstExec := false, false
	var execArgs []string
	var payloads []string
	for _, word := range words {
		if inExec {
			if isFindExecTerminator(word) {
				if payload := quotePayload(execArgs); payload != "" {
					payloads = append(payloads, payload)
				}
				execArgs = nil
				inExec = false
				continue
			}
			if firstExec {
				firstExec = false
				if isFindDestroyer(word) {
					deny = true
				}
			}
			execArgs = append(execArgs, word)
			continue
		}
		if inStarts {
			switch {
			case word == "-H" || word == "-L" || word == "-P" || word == "-D" || strings.HasPrefix(word, "-O"):
				continue
			case strings.HasPrefix(word, "-") || word == "(" || word == "!":
				inStarts = false
			default:
				if isCatastrophicTarget(word, home) {
					dangerStart = true
				}
				continue
			}
		}
		switch word {
		case "-delete":
			deny = true
		case "-exec", "-execdir", "-ok", "-okdir":
			inExec = true
			firstExec = true
		}
	}
	if inExec {
		if payload := quotePayload(execArgs); payload != "" {
			payloads = append(payloads, payload)
		}
	}
	return dangerStart && deny, payloads
}

func isFindExecTerminator(word string) bool {
	switch word {
	case ";", "+", `\`, `\;`, `\+`:
		return true
	default:
		return false
	}
}

func isFindDestroyer(word string) bool {
	word = strings.TrimLeft(word, `\`)
	switch filepath.Base(word) {
	case "rm", "unlink", "shred":
		return true
	default:
		return false
	}
}

func quotePayload(words []string) string {
	quoted := make([]string, 0, len(words))
	for _, word := range words {
		if strings.IndexFunc(word, func(r rune) bool {
			return unicode.IsSpace(r) || strings.ContainsRune(`'"`+"`$;&|()<>", r)
		}) >= 0 {
			switch {
			case !strings.ContainsRune(word, '\''):
				word = "'" + word + "'"
			case !strings.ContainsRune(word, '"'):
				word = `"` + word + `"`
			}
		}
		quoted = append(quoted, word)
	}
	return strings.Join(quoted, " ")
}

func isBroadWriteMode(mode string) bool {
	if mode == "777" || mode == "0777" || mode == "1777" {
		return true
	}
	match := broadModePattern.FindStringSubmatch(mode)
	if match == nil || match[2] == "-" || !strings.Contains(match[3], "w") {
		return false
	}
	return match[1] == "" || strings.ContainsAny(match[1], "ao")
}

func chmodIsCatastrophic(tail, home string) bool {
	words, ok := shellWords(tail)
	if !ok {
		return false
	}
	badMode, catastrophicTarget := false, false
	for _, word := range words {
		if strings.HasPrefix(word, "-") {
			continue
		}
		badMode = badMode || isBroadWriteMode(word)
		catastrophicTarget = catastrophicTarget || isCatastrophicTarget(word, home)
	}
	return badMode && catastrophicTarget
}

func isBlockDevice(path string) bool {
	for _, prefix := range []string{
		"/dev/sd", "/dev/nvme", "/dev/vd", "/dev/xvd", "/dev/mmcblk", "/dev/disk", "/dev/rdisk",
		"/dev/mapper/", "/dev/dm-", "/dev/md/", "/dev/mtdblock", "/dev/hd", "/dev/sr", "/dev/vblk",
		"/dev/rbd", "/dev/nbd", "/dev/drbd", "/dev/pmem", "/dev/zvol/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	for _, numbered := range []string{"/dev/md", "/dev/loop"} {
		if strings.HasPrefix(path, numbered) && len(path) > len(numbered) && path[len(numbered)] >= '0' && path[len(numbered)] <= '9' {
			return true
		}
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode()&os.ModeDevice != 0 && info.Mode()&os.ModeCharDevice == 0
}

func ddIsCatastrophic(tail string) bool {
	words, ok := shellWords(tail)
	if !ok {
		return false
	}
	for _, word := range words {
		if strings.HasPrefix(word, "of=") && isBlockDevice(strings.TrimPrefix(word, "of=")) {
			return true
		}
	}
	return false
}

func formatIsCatastrophic(tail string) bool {
	words, ok := shellWords(tail)
	if !ok {
		return false
	}
	for _, word := range words {
		if isBlockDevice(word) {
			return true
		}
	}
	return false
}

func cpIsCatastrophic(tail string) bool {
	words, ok := shellWords(tail)
	if !ok {
		return false
	}
	destination := ""
	for _, word := range words {
		if !strings.HasPrefix(word, "-") {
			destination = word
		}
	}
	return isBlockDevice(destination)
}

func recursiveRemoveIsCatastrophic(tail, home string) bool {
	words, ok := shellWords(tail)
	if !ok {
		return false
	}
	recursive := false
	for _, word := range words {
		lower := strings.ToLower(word)
		if lower == "-recurse" || lower == "-r" || lower == "/s" {
			recursive = true
		}
	}
	if !recursive {
		return false
	}
	for _, word := range words {
		lower := strings.ToLower(word)
		if strings.HasPrefix(word, "-") || lower == "/s" || lower == "/q" {
			continue
		}
		if isCatastrophicTarget(word, home) || driveRootPattern.MatchString(word) {
			return true
		}
	}
	return false
}

func stripQuoted(text string) string {
	var output strings.Builder
	for i := 0; i < len(text); {
		if text[i] != '\'' && text[i] != '"' && text[i] != '`' {
			output.WriteByte(text[i])
			i++
			continue
		}
		quote := text[i]
		i++
		for i < len(text) && text[i] != quote {
			i++
		}
		if i < len(text) {
			i++
		}
	}
	return output.String()
}

func shellFlagRunsCommand(word string) bool {
	if len(word) < 3 || word[0] != '-' || word[len(word)-1] != 'c' {
		return false
	}
	return (word[1] >= 'A' && word[1] <= 'Z') || (word[1] >= 'a' && word[1] <= 'z')
}

func sshPayload(words []string) string {
	skipArgument, hostSeen := false, false
	var remote []string
	for _, word := range words {
		if word == "" {
			continue
		}
		if hostSeen {
			remote = append(remote, word)
			continue
		}
		if skipArgument {
			skipArgument = false
			continue
		}
		if sshOptionTakesValue(word) {
			skipArgument = true
			continue
		}
		if strings.HasPrefix(word, "-") {
			continue
		}
		hostSeen = true
	}
	return strings.Join(remote, " ")
}

func sshOptionTakesValue(word string) bool {
	return len(word) == 2 && strings.ContainsRune("bBcDeFIiJLlmOopQRSWw", rune(word[1]))
}

func nonempty(words []string) []string {
	result := make([]string, 0, len(words))
	for _, word := range words {
		if word != "" {
			result = append(result, word)
		}
	}
	return result
}
