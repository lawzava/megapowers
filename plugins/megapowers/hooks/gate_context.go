package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// The gate stops the first command in a session that completes work or
// reaches outside the machine until the matching skill has loaded: it denies
// that command once with a reason naming the skill, and the retry runs. Calls
// of the same kind inside gateRetryWindow also stop while the transcript shows
// no load, so parallel siblings cannot slip past the first denial. MCP tool
// calls whose name says they write share the safe-effects gate. A
// reminder alone arrived with the command it was meant to precede and was
// mostly ignored. When the once-marker cannot be recorded (no session ID or an
// unusable cache), the gate falls back to non-blocking context so it can never
// block the same command twice.
//
// A git command that discards uncommitted, stashed, or unmerged work has no
// matching skill. It stops once per session and exact command instead, so the
// agent checks what it would lose before the retry runs.
const (
	completionSkill = "verify-and-finish"
	effectSkill     = "safe-effects"
	discardGate     = "git-discard"

	completionContext = "Before claiming this complete, load and follow the megapowers verify-and-finish skill if it is not already loaded for this outcome."
	effectContext     = "Before running this outward effect, load and follow the megapowers safe-effects skill if it is not already loaded for this effect."
	completionDeny    = "Load and follow the megapowers verify-and-finish skill before this completion step, then run the command again; this check blocks once per session."
	effectDeny        = "Load and follow the megapowers safe-effects skill before this outward effect, then run the command again; this check blocks once per session."
	discardContext    = "This git command discards uncommitted, stashed, or unmerged work. Check git status and git stash list and confirm the work is disposable or the user asked to discard it."
	discardDeny       = "This git command discards uncommitted, stashed, or unmerged work. Check git status and git stash list, confirm the work is disposable or the user asked to discard it, then run the same command again; this check blocks once per command."

	gateMarkerDir = "gate-context"
	gateMarkerTTL = 7 * 24 * time.Hour
	// gateRetryWindow keeps denying same-kind calls after the first denial
	// until the transcript shows the skill loaded. Parallel siblings arrive
	// within about a second of the first call; loading the skill takes one
	// model turn, after which the retry passes on the transcript check. If
	// that check misses a load, the agent loses at most one minute before the
	// once-per-session pass-through resumes.
	gateRetryWindow = 60 * time.Second
	// branchCheckTimeout bounds the merge-base checks for one command.
	branchCheckTimeout = 2 * time.Second
)

var (
	gitValueOptions = map[string]struct{}{"-C": {}, "-c": {}, "--git-dir": {}, "--work-tree": {}, "--namespace": {}}
	ghValueOptions  = map[string]struct{}{"-R": {}, "--repo": {}}
	// Global options that take a separate value word, so "kubectl --context
	// prod apply" still resolves the subcommand to apply.
	kubectlValueOptions = map[string]struct{}{"--context": {}, "-n": {}, "--namespace": {}, "--kubeconfig": {}, "--cluster": {}, "--user": {},
		"-s": {}, "--server": {}, "--token": {}, "--as": {}, "--as-group": {}, "--request-timeout": {}, "--cache-dir": {}}
	helmValueOptions = map[string]struct{}{"--kube-context": {}, "-n": {}, "--namespace": {}, "--kubeconfig": {}, "--kube-apiserver": {},
		"--kube-token": {}, "--kube-as-user": {}, "--registry-config": {}, "--repository-config": {}, "--repository-cache": {}}
	dockerValueOptions = map[string]struct{}{"--context": {}, "-c": {}, "-H": {}, "--host": {}, "--config": {}, "-l": {}, "--log-level": {}}
	npmValueOptions    = map[string]struct{}{"--prefix": {}, "-w": {}, "--workspace": {}, "--registry": {}, "--userconfig": {}, "-C": {}, "--dir": {}, "--filter": {}, "-F": {}}
	ghPRWrites         = map[string]struct{}{"comment": {}, "review": {}, "edit": {}, "close": {}, "reopen": {}, "ready": {}, "lock": {}, "unlock": {}}
	ghIssueWrites      = map[string]struct{}{"create": {}, "comment": {}, "edit": {}, "close": {}, "reopen": {}, "delete": {}, "transfer": {}, "lock": {}, "unlock": {}, "pin": {}, "unpin": {}}
	// ghAPIValueOptions take a separate value word, so the endpoint is the
	// first word that is neither an option nor such a value.
	ghAPIValueOptions = map[string]struct{}{"-X": {}, "--method": {}, "-f": {}, "-F": {}, "--field": {}, "--raw-field": {},
		"--input": {}, "-H": {}, "--header": {}, "-q": {}, "--jq": {}, "-t": {}, "--template": {}, "--cache": {}, "--hostname": {}, "-p": {}, "--preview": {}}
)

// isDryRun reports a rehearsal flag: --dry-run, or --dry-run=<mode> for any
// mode other than none or false.
func isDryRun(words []string) bool {
	for _, word := range words {
		if word == "--dry-run" {
			return true
		}
		if mode, ok := strings.CutPrefix(word, "--dry-run="); ok && mode != "none" && mode != "false" {
			return true
		}
	}
	return false
}

// gateDecision returns the gates a shell command must stop on and the gates
// that only get a reminder: the skill gates, then the git discard check.
func gateDecision(command, sessionID, transcriptPath, cwd string, getenv getenvFunc) (deny, remind []string) {
	root := gateMarkerRoot(getenv)
	deny, remind = skillGates(gateSkills(command), sessionID, transcriptPath, getenv)
	if gitDiscards(command, cwd) {
		digest := sha256.Sum256([]byte(command))
		switch markGate(root, sessionID, discardGate+"-"+hex.EncodeToString(digest[:8])) {
		case markerNew:
			deny = append(deny, discardGate)
		case markerUnavailable:
			remind = append(remind, discardGate)
		}
	}
	return deny, remind
}

// skillGates returns the skills whose first gated call this session must stop
// on, and the skills that only get a reminder because the once-marker could
// not be recorded. A skill the transcript shows as already loaded needs
// neither. A same-kind call inside gateRetryWindow of the first denial also
// stops, but only when the transcript is readable, so a missing transcript
// keeps the plain once-per-session behavior.
func skillGates(skills []string, sessionID, transcriptPath string, getenv getenvFunc) (deny, remind []string) {
	root := gateMarkerRoot(getenv)
	for _, skill := range skills {
		state := markGate(root, sessionID, skill)
		loaded, readable := skillLoaded(transcriptPath, skill)
		switch {
		case loaded:
		case state == markerNew:
			deny = append(deny, skill)
		case state == markerUnavailable:
			remind = append(remind, skill)
		case readable && gateRecentlyDenied(root, sessionID, skill):
			deny = append(deny, skill)
		}
	}
	return deny, remind
}

// gateMessages returns the deny reasons, or the non-blocking context when
// deny is false, for each gate in order.
func gateMessages(gates []string, deny bool) string {
	var lines []string
	for _, gate := range gates {
		switch {
		case gate == completionSkill && deny:
			lines = append(lines, completionDeny)
		case gate == completionSkill:
			lines = append(lines, completionContext)
		case gate == discardGate && deny:
			lines = append(lines, discardDeny)
		case gate == discardGate:
			lines = append(lines, discardContext)
		case deny:
			lines = append(lines, effectDeny)
		default:
			lines = append(lines, effectContext)
		}
	}
	return strings.Join(lines, "\n")
}

// skillLoaded reports whether the transcript records the skill body loading:
// a Skill tool call, an injected skill body, or a tool call that reads the
// skill's SKILL.md. Catalog entries and prose mentions do not count. An
// unreadable transcript counts as not loaded and reports readable false.
func skillLoaded(transcriptPath, skill string) (loaded, readable bool) {
	if transcriptPath == "" {
		return false, false
	}
	file, err := os.Open(transcriptPath)
	if err != nil {
		return false, false
	}
	defer file.Close()
	skillCall := []byte("\"megapowers:" + skill + "\"")
	skillDir := []byte("skills/" + skill)
	skillBody := []byte("skills/" + skill + "/SKILL.md")
	reader := bufio.NewReaderSize(file, 64<<10)
	for {
		line, err := reader.ReadBytes('\n')
		switch {
		case bytes.Contains(line, []byte("\"name\":\"Skill\"")) && bytes.Contains(line, skillCall):
			return true, true
		case bytes.Contains(line, []byte("Base directory for this skill:")) && bytes.Contains(line, skillDir):
			return true, true
		case bytes.Contains(line, skillBody) && (bytes.Contains(line, []byte("\"function_call\"")) ||
			bytes.Contains(line, []byte("\"custom_tool_call\"")) || bytes.Contains(line, []byte("\"local_shell_call\"")) ||
			bytes.Contains(line, []byte("\"type\":\"tool_use\""))):
			return true, true
		}
		if err != nil {
			return false, true
		}
	}
}

// gateRecentlyDenied reports a skill marker created within gateRetryWindow.
// The marker is written once, when the first call is denied, so its mtime is
// that denial's time. A missing marker or a future mtime counts as not recent.
func gateRecentlyDenied(root, sessionID, skill string) bool {
	info, err := os.Stat(gateMarkerPath(root, sessionID, skill))
	if err != nil {
		return false
	}
	age := time.Since(info.ModTime())
	return age >= 0 && age < gateRetryWindow
}

func hasKey(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}

// gateSkills returns the skills a command implicates, completion first, each
// at most once.
func gateSkills(command string) []string {
	if len(command) > maxCommandBytes {
		return nil
	}
	completion, effect := false, false
	for _, segment := range splitSegments(command) {
		name, tail, ok := resolveCommand(segment)
		if !ok {
			continue
		}
		words, ok := shellWords(tail)
		if !ok {
			words = strings.Fields(tail)
		}
		switch gateKind(name, nonempty(words)) {
		case completionSkill:
			completion = true
		case effectSkill:
			effect = true
		}
	}
	var skills []string
	if completion {
		skills = append(skills, completionSkill)
	}
	if effect {
		skills = append(skills, effectSkill)
	}
	return skills
}

func gateKind(name string, words []string) string {
	if isDryRun(words) {
		return ""
	}
	if binary, args, ok := packageRunnerTarget(name, words); ok {
		if kind := gateKind(binary, args); kind != "" {
			return kind
		}
	}
	switch name {
	case "git":
		if sub := subcommand(words, gitValueOptions); sub == "commit" || sub == "push" {
			return completionSkill
		}
	case "gh":
		sub := subcommand(words, ghValueOptions)
		second := secondSubcommand(words, ghValueOptions)
		switch {
		case sub == "pr" && (second == "create" || second == "merge"):
			return completionSkill
		case sub == "release" && second == "create":
			return completionSkill
		case sub == "api" && ghAPIMutates(words):
			return effectSkill
		case sub == "pr" && hasKey(ghPRWrites, second):
			return effectSkill
		case sub == "issue" && hasKey(ghIssueWrites, second):
			return effectSkill
		}
	case "npm", "pnpm", "cargo":
		if subcommand(words, npmValueOptions) == "publish" {
			return effectSkill
		}
	case "yarn":
		sub := subcommand(words, nil)
		if sub == "publish" || (sub == "npm" && secondSubcommand(words, nil) == "publish") {
			return effectSkill
		}
	case "docker":
		if subcommand(words, dockerValueOptions) == "push" {
			return effectSkill
		}
	case "kubectl":
		if sub := subcommand(words, kubectlValueOptions); sub == "apply" || sub == "delete" {
			return effectSkill
		}
	case "helm":
		if sub := subcommand(words, helmValueOptions); sub == "upgrade" || sub == "install" {
			return effectSkill
		}
	case "terraform", "tofu":
		if sub := subcommand(words, nil); sub == "apply" || sub == "destroy" {
			return effectSkill
		}
	case "railway":
		if sub := subcommand(words, nil); sub == "up" || sub == "deploy" {
			return effectSkill
		}
	case "fly", "flyctl":
		if subcommand(words, nil) == "deploy" {
			return effectSkill
		}
	case "vercel":
		if hasWord(words, "--prod") {
			return effectSkill
		}
	case "wrangler":
		if wranglerDeploys(words) {
			return effectSkill
		}
	case "curl":
		if curlWrites(words) {
			return effectSkill
		}
	case "wget":
		if wgetWrites(words) {
			return effectSkill
		}
	case "http", "https", "xh", "xhs":
		if httpieWrites(words) {
			return effectSkill
		}
	}
	if httpVerbWrites(name, words) {
		return effectSkill
	}
	return ""
}

// gitDiscards reports a git command that discards work the repository cannot
// recover on its own: a hard reset, a forced clean, a forced or whole-tree
// checkout, a whole-tree worktree restore, a forced branch delete, or a dropped
// or cleared stash.
//
// A forced branch delete passes when every named branch is already reachable
// from HEAD in the hook's working directory, because the delete loses no
// commits. A cd before it makes the directory unknown, so the check stays.
func gitDiscards(command, cwd string) bool {
	if len(command) > maxCommandBytes {
		return false
	}
	dir := cwd
	for _, segment := range splitSegments(command) {
		name, tail, ok := resolveCommand(segment)
		if ok && (name == "cd" || name == "pushd" || name == "popd") {
			dir = ""
		}
		if !ok || name != "git" {
			continue
		}
		words, ok := shellWords(tail)
		if !ok {
			words = strings.Fields(tail)
		}
		words = nonempty(words)
		if !gitDiscardWords(words) {
			continue
		}
		// A wrapper or assignment before git can move the repository.
		first, _, _ := takeWord(trimLeftSpace(segment))
		if commandBaseName(first) == "git" && mergedBranchDelete(words, dir) {
			continue
		}
		return true
	}
	return false
}

// mergedBranchDelete reports a branch delete whose every named branch is an
// ancestor of HEAD in dir. Any doubt reports false: a relative or empty
// directory, an option it does not know, a git failure, or the timeout.
func mergedBranchDelete(words []string, dir string) bool {
	if !filepath.IsAbs(dir) {
		return false
	}
	i := 0
	for ; i < len(words) && words[i] != "branch"; i++ {
		switch words[i] {
		case "-C":
			if i+1 >= len(words) {
				return false
			}
			i++
			if filepath.IsAbs(words[i]) {
				dir = words[i]
			} else {
				dir = filepath.Join(dir, words[i])
			}
		case "-c":
			i++
		case "--no-pager", "-P", "--no-optional-locks":
		default:
			return false
		}
	}
	prefix := "refs/heads/"
	var names []string
	for j := i + 1; j < len(words); j++ {
		word := words[j]
		switch {
		case word == "--":
			names = append(names, words[j+1:]...)
			j = len(words)
		case word == "--remotes":
			prefix = "refs/remotes/"
		case word == "--delete" || word == "--force" || word == "--quiet":
		case strings.HasPrefix(word, "--"):
			return false
		case strings.HasPrefix(word, "-"):
			if strings.Trim(word[1:], "Ddfqr") != "" {
				return false
			}
			if strings.IndexByte(word, 'r') >= 0 {
				prefix = "refs/remotes/"
			}
		default:
			names = append(names, word)
		}
	}
	if len(names) == 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), branchCheckTimeout)
	defer cancel()
	for _, name := range names {
		if exec.CommandContext(ctx, "git", "-C", dir, "merge-base", "--is-ancestor", prefix+name, "HEAD").Run() != nil {
			return false
		}
	}
	return true
}

func gitDiscardWords(words []string) bool {
	subs := subcommands(words, gitValueOptions, 1)
	if len(subs) == 0 {
		return false
	}
	var args []string
	for i, word := range words {
		if word == subs[0] {
			args = words[i+1:]
			break
		}
	}
	switch subs[0] {
	case "reset":
		return hasWord(args, "--hard")
	case "clean":
		return (hasWord(args, "--force") || hasShortFlag(args, 'f')) && !hasWord(args, "--dry-run") && !hasShortFlag(args, 'n')
	case "checkout":
		return hasWord(args, "-f") || hasWord(args, "--force") || hasWholeTreePath(args)
	case "restore":
		stagedOnly := (hasWord(args, "--staged") || hasShortFlag(args, 'S')) && !hasWord(args, "--worktree") && !hasShortFlag(args, 'W')
		return hasWholeTreePath(args) && !stagedOnly
	case "branch":
		forced := hasShortFlag(args, 'D') || ((hasWord(args, "--delete") || hasShortFlag(args, 'd')) && (hasWord(args, "--force") || hasShortFlag(args, 'f')))
		return forced && len(subcommands(args, nil, 1)) == 1
	case "stash":
		second := subcommands(args, nil, 1)
		return len(second) == 1 && (second[0] == "drop" || second[0] == "clear")
	}
	return false
}

// hasShortFlag reports a single-dash option cluster containing flag, so -fdx
// counts as -f.
func hasShortFlag(words []string, flag byte) bool {
	for _, word := range words {
		if word == "--" {
			return false
		}
		if len(word) > 1 && word[0] == '-' && word[1] != '-' && strings.IndexByte(word[1:], flag) >= 0 {
			return true
		}
	}
	return false
}

// hasWholeTreePath reports a pathspec naming the whole working tree.
func hasWholeTreePath(words []string) bool {
	for _, word := range words {
		switch word {
		case ".", "./", ":/", ":/.", "*", ":(top)":
			return true
		}
	}
	return false
}

// subcommand returns the first non-option word, skipping values of options
// listed in valueOptions.
func subcommand(words []string, valueOptions map[string]struct{}) string {
	subs := subcommands(words, valueOptions, 1)
	if len(subs) == 0 {
		return ""
	}
	return subs[0]
}

func secondSubcommand(words []string, valueOptions map[string]struct{}) string {
	subs := subcommands(words, valueOptions, 2)
	if len(subs) < 2 {
		return ""
	}
	return subs[1]
}

func subcommands(words []string, valueOptions map[string]struct{}, limit int) []string {
	var subs []string
	skip := false
	for _, word := range words {
		if skip {
			skip = false
			continue
		}
		if strings.HasPrefix(word, "-") {
			if _, takesValue := valueOptions[word]; takesValue {
				skip = true
			}
			continue
		}
		subs = append(subs, word)
		if len(subs) == limit {
			break
		}
	}
	return subs
}

func hasWord(words []string, want string) bool {
	for _, word := range words {
		if word == want {
			return true
		}
	}
	return false
}

// ghAPIMutates: an explicit method decides; otherwise gh api sends POST when
// given fields or an input body. A GraphQL POST is a read unless its query is
// a mutation or comes from a file the hook cannot inspect.
func ghAPIMutates(words []string) bool {
	endpoint, query := "", ""
	hasBody, opaqueQuery := false, false
	for i := 0; i < len(words); i++ {
		word := words[i]
		value := ""
		if i+1 < len(words) {
			value = words[i+1]
		}
		switch {
		case word == "-X" || word == "--method":
			return isMutatingMethod(value)
		case strings.HasPrefix(word, "-X") && len(word) > 2:
			return isMutatingMethod(word[2:])
		case strings.HasPrefix(strings.ToLower(word), "--method="):
			return isMutatingMethod(word[len("--method="):])
		case word == "-f" || word == "-F" || word == "--field" || word == "--raw-field":
			hasBody = true
			query, opaqueQuery = graphQLQuery(value, word == "-F" || word == "--field", query, opaqueQuery)
			i++
		case strings.HasPrefix(word, "--field=") || strings.HasPrefix(word, "--raw-field="):
			hasBody = true
			_, field, _ := strings.Cut(word, "=")
			query, opaqueQuery = graphQLQuery(field, strings.HasPrefix(word, "--field="), query, opaqueQuery)
		case word == "--input" || strings.HasPrefix(word, "--input="):
			hasBody, opaqueQuery = true, true
			if word == "--input" {
				i++
			}
		case hasKey(ghAPIValueOptions, word):
			i++
		case strings.HasPrefix(word, "-"):
		case endpoint == "" && word != "api":
			endpoint = word
		}
	}
	if !hasBody {
		return false
	}
	if endpoint != "graphql" {
		return true
	}
	return opaqueQuery || strings.HasPrefix(strings.TrimSpace(query), "mutation")
}

// graphQLQuery captures the value of a query field; a typed field (-F) that
// reads @file is opaque.
func graphQLQuery(field string, typed bool, query string, opaque bool) (string, bool) {
	key, value, ok := strings.Cut(field, "=")
	if !ok || key != "query" {
		return query, opaque
	}
	if typed && strings.HasPrefix(value, "@") {
		return query, true
	}
	return value, opaque
}

func isMutatingMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "POST", "PATCH", "PUT", "DELETE":
		return true
	default:
		return false
	}
}

// gateMarkerRoot resolves the private hook cache directory the launcher
// passes down, or the same default it would compute.
func gateMarkerRoot(getenv getenvFunc) string {
	if dir := getenv("MEGAPOWERS_HOOK_CACHE_DIR"); dir != "" {
		return dir
	}
	return defaultHookCacheDir(getenv)
}

func defaultHookCacheDir(getenv getenvFunc) string {
	switch {
	case getenv("MEGAPOWERS_HOOK_CACHE") != "":
		return filepath.Join(getenv("MEGAPOWERS_HOOK_CACHE"), "megapowers-hooks")
	case getenv("XDG_CACHE_HOME") != "":
		return filepath.Join(getenv("XDG_CACHE_HOME"), "megapowers-hooks")
	case getenv("HOME") != "":
		return filepath.Join(getenv("HOME"), ".cache", "megapowers-hooks")
	default:
		return ""
	}
}

type markerState int

const (
	markerNew markerState = iota
	markerSeen
	markerUnavailable
)

// markGate records that a session reached the gate for a skill. It reports
// markerSeen when a marker already existed, and markerUnavailable when no
// marker can be recorded, so the caller never blocks without a way to let the
// retry through.
func markGate(root, sessionID, skill string) markerState {
	if root == "" || sessionID == "" {
		return markerUnavailable
	}
	dir := filepath.Join(root, gateMarkerDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return markerUnavailable
	}
	file, err := os.OpenFile(gateMarkerPath(root, sessionID, skill), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return markerSeen
		}
		return markerUnavailable
	}
	file.Close()
	pruneGateMarkers(dir, time.Now().Add(-gateMarkerTTL))
	return markerNew
}

// pruneGateMarkers runs only when a new marker was written, at most once per
// session per skill, so the directory stays small without a scheduled job.
func pruneGateMarkers(dir string, before time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err == nil && info.Mode().IsRegular() && info.ModTime().Before(before) {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

func gateMarkerPath(root, sessionID, skill string) string {
	digest := sha256.Sum256([]byte(sessionID))
	return filepath.Join(root, gateMarkerDir, hex.EncodeToString(digest[:8])+"-"+skill)
}

// Both harnesses name MCP tools mcp__<server>__<tool>; Claude Code plugin
// servers use mcp__plugin_<plugin>_<server>__<tool>.
const mcpToolPrefix = "mcp__"

var (
	// mcpWriteVerbs and mcpReadVerbs classify the action part of an MCP tool
	// name. The first word found in either set decides, so get_comment is a
	// read and slack_send_message is a write.
	mcpWriteVerbs = map[string]struct{}{
		"save": {}, "create": {}, "update": {}, "delete": {}, "remove": {}, "send": {}, "post": {}, "publish": {},
		"deploy": {}, "redeploy": {}, "upload": {}, "write": {}, "edit": {}, "comment": {}, "reply": {}, "merge": {},
		"close": {}, "archive": {}, "add": {}, "set": {}, "patch": {}, "put": {}, "cancel": {}, "invite": {},
		"assign": {}, "move": {}, "rename": {}, "restore": {}, "trigger": {}, "restart": {}, "submit": {},
		"approve": {}, "insert": {}, "upsert": {},
	}
	mcpReadVerbs = map[string]struct{}{
		"get": {}, "list": {}, "search": {}, "read": {}, "fetch": {}, "query": {}, "describe": {}, "view": {},
		"find": {}, "lookup": {},
	}
)

// mcpGateSkills returns the safe-effects gate for an MCP tool whose action
// names a write. Reads and unrecognized actions return nothing.
func mcpGateSkills(toolName string) []string {
	rest, ok := strings.CutPrefix(toolName, mcpToolPrefix)
	if !ok {
		return nil
	}
	cut := strings.LastIndex(rest, "__")
	if cut < 0 {
		return nil
	}
	for _, word := range actionWords(rest[cut+2:]) {
		if hasKey(mcpReadVerbs, word) {
			return nil
		}
		if hasKey(mcpWriteVerbs, word) {
			return []string{effectSkill}
		}
	}
	return nil
}

// actionWords splits a tool name on underscores, hyphens, dots, and camelCase
// boundaries, lowercased: createPullRequest becomes create, pull, request.
func actionWords(name string) []string {
	var words []string
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			words = append(words, strings.ToLower(word.String()))
			word.Reset()
		}
	}
	previous := rune(0)
	for _, r := range name {
		switch {
		case r == '_' || r == '-' || r == '.':
			flush()
		case unicode.IsUpper(r) && unicode.IsLower(previous):
			flush()
			word.WriteRune(r)
		default:
			word.WriteRune(r)
		}
		previous = r
	}
	flush()
	return words
}

var (
	wranglerValueOptions = map[string]struct{}{"-c": {}, "--config": {}, "-e": {}, "--env": {}, "--cwd": {}}
	// packageRunners run a package binary: npx wrangler deploy, pnpm exec
	// wrangler deploy. The runner subcommands name the word that follows as
	// the binary; otherwise the first word is the binary.
	packageRunners      = map[string]struct{}{"npx": {}, "pnpx": {}, "bunx": {}, "pnpm": {}, "yarn": {}, "bun": {}}
	packageRunnerVerbs  = map[string]struct{}{"exec": {}, "dlx": {}, "x": {}}
	packageRunnerValues = map[string]struct{}{"-p": {}, "--package": {}, "-C": {}, "--dir": {}, "--filter": {}, "-F": {}, "-w": {}, "--workspace": {}}

	// httpTextTools print, search, or edit text; an HTTP verb in their
	// arguments is data, not a request. Tools with their own request parser
	// are listed too, so the generic wrapper rule never overrides them.
	httpTextTools = map[string]struct{}{
		"echo": {}, "printf": {}, "grep": {}, "egrep": {}, "fgrep": {}, "rg": {}, "ag": {}, "ack": {}, "sed": {}, "awk": {},
		"jq": {}, "cat": {}, "man": {}, "git": {}, "gh": {}, "curl": {}, "wget": {}, "http": {}, "https": {}, "xh": {}, "xhs": {},
	}
	// filesystemRoots extends systemRoots with top-level directories that
	// hold files, so "POST /tmp/body.json" is not read as an API path.
	filesystemRoots = map[string]struct{}{"tmp": {}, "mnt": {}, "media": {}, "nix": {}, "snap": {}, "workspace": {}}

	curlShortValues     = "AbcCdDeEFHKmoPQrtTuUwxXyYz"
	curlLongDataOptions = map[string]struct{}{"--data": {}, "--data-raw": {}, "--data-binary": {}, "--data-urlencode": {},
		"--data-ascii": {}, "--json": {}, "--form": {}, "--form-string": {}, "--upload-file": {}}
	curlLongValueOptions = map[string]struct{}{"--header": {}, "--user": {}, "--output": {}, "--user-agent": {}, "--cookie": {},
		"--cookie-jar": {}, "--max-time": {}, "--connect-timeout": {}, "--retry": {}, "--proxy": {}, "--cacert": {}, "--cert": {},
		"--key": {}, "--config": {}, "--write-out": {}, "--referer": {}, "--unix-socket": {}, "--abstract-unix-socket": {},
		"--resolve": {}, "--connect-to": {}, "--range": {}, "--limit-rate": {}, "--oauth2-bearer": {}, "--interface": {},
		"--retry-delay": {}, "--retry-max-time": {}, "--output-dir": {}, "--noproxy": {}, "--proxy-user": {}, "--url-query": {},
		"--variable": {}, "--dump-header": {}, "--trace": {}, "--trace-ascii": {}, "--stderr": {}, "--aws-sigv4": {}}
	wgetShortValues  = "aABDeiIloOPQRtTUwX"
	httpieValueFlags = map[string]struct{}{"-a": {}, "--auth": {}, "-A": {}, "--auth-type": {}, "-o": {}, "--output": {},
		"--session": {}, "--session-read-only": {}, "--verify": {}, "--cert": {}, "--cert-key": {}, "--proxy": {}, "--timeout": {},
		"-p": {}, "--print": {}, "--pretty": {}, "-s": {}, "--style": {}, "--format-options": {}, "--max-redirects": {},
		"--boundary": {}, "--ssl": {}, "--ciphers": {}, "--response-charset": {}, "--response-mime": {}, "--unix-socket": {}}
)

// packageRunnerTarget returns the binary and arguments a package runner
// would execute, or false for anything else.
func packageRunnerTarget(name string, words []string) (string, []string, bool) {
	if !hasKey(packageRunners, name) {
		return "", nil, false
	}
	explicit := name == "npx" || name == "pnpx" || name == "bunx"
	for i := 0; i < len(words); i++ {
		word := words[i]
		switch {
		case hasKey(packageRunnerValues, word):
			i++
		case strings.HasPrefix(word, "-"):
		case !explicit && hasKey(packageRunnerVerbs, word):
			explicit = true
		default:
			// npx wrangler@3 runs wrangler.
			binary, _, _ := strings.Cut(commandBaseName(word), "@")
			return binary, words[i+1:], binary != ""
		}
	}
	return "", nil, false
}

func wranglerDeploys(words []string) bool {
	subs := subcommands(words, wranglerValueOptions, 2)
	if len(subs) == 0 {
		return false
	}
	second := ""
	if len(subs) == 2 {
		second = subs[1]
	}
	switch subs[0] {
	case "deploy", "publish":
		return true
	case "pages":
		return second == "deploy" || second == "publish"
	case "versions":
		return second == "deploy"
	}
	return false
}

// httpVerbWrites reports a wrapper invocation such as "api.sh POST /api/x":
// an uppercase write method word followed by a non-loopback http(s) URL or an
// absolute path outside the filesystem roots.
func httpVerbWrites(name string, words []string) bool {
	if hasKey(httpTextTools, name) {
		return false
	}
	method := false
	for _, word := range words {
		switch {
		case !method:
			method = isWriteMethodWord(word)
		case strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://"):
			if !isLoopbackTarget(word) {
				return true
			}
		case strings.HasPrefix(word, "/") && !strings.HasPrefix(word, "//"):
			root, _, _ := strings.Cut(word[1:], "/")
			if root != "" && !hasKey(systemRoots, root) && !hasKey(filesystemRoots, root) {
				return true
			}
		}
	}
	return false
}

func isWriteMethodWord(word string) bool {
	switch word {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}

// curlWrites: an explicit -X method decides; otherwise a body or upload
// option sends POST or PUT unless -G or -I turns it into GET or HEAD.
func curlWrites(words []string) bool {
	method, body, read := "", false, false
	var targets []string
	for i := 0; i < len(words); i++ {
		word := words[i]
		next := func() string {
			if i+1 < len(words) {
				i++
				return words[i]
			}
			return ""
		}
		switch {
		case word == "--":
			targets = append(targets, words[i+1:]...)
			i = len(words)
		case word == "-X" || word == "--request":
			method = next()
		case word == "--url":
			targets = append(targets, next())
		case hasKey(curlLongDataOptions, word):
			body = true
			next()
		case word == "--get" || word == "--head":
			read = true
		case hasKey(curlLongValueOptions, word):
			next()
		case strings.HasPrefix(word, "--"):
		case strings.HasPrefix(word, "-") && len(word) > 1:
		cluster:
			for k := 1; k < len(word); k++ {
				flag := word[k]
				if strings.IndexByte(curlShortValues, flag) < 0 {
					read = read || flag == 'G' || flag == 'I'
					continue
				}
				value := word[k+1:]
				if value == "" {
					value = next()
				}
				switch flag {
				case 'X':
					method = value
				case 'd', 'F', 'T':
					body = true
				}
				break cluster
			}
		default:
			targets = append(targets, word)
		}
	}
	writes := isMutatingMethod(method) || (method == "" && body && !read)
	return writes && reachesRemote(targets)
}

// wgetWrites: --method names the request; --post-data or --post-file sends
// POST.
func wgetWrites(words []string) bool {
	method, body := "", false
	var targets []string
	for i := 0; i < len(words); i++ {
		word := words[i]
		option, value, hasValue := strings.Cut(word, "=")
		switch {
		case option == "--method":
			if !hasValue && i+1 < len(words) {
				i++
				value = words[i]
			}
			method = value
		case option == "--post-data" || option == "--post-file":
			body = true
			if !hasValue {
				i++
			}
		case option == "--body-data" || option == "--body-file":
			if !hasValue {
				i++
			}
		case strings.HasPrefix(word, "--"):
		case strings.HasPrefix(word, "-") && len(word) > 1:
			if strings.IndexByte(wgetShortValues, word[len(word)-1]) >= 0 {
				i++
			}
		default:
			targets = append(targets, word)
		}
	}
	return (isMutatingMethod(method) || body) && reachesRemote(targets)
}

// httpieWrites covers HTTPie and xh: [METHOD] URL [ITEMS]. Without a method,
// a data item (name=value, name:=json, name@file) sends POST; query (==) and
// header (:) items do not.
func httpieWrites(words []string) bool {
	method, target, data := "", "", false
	for i := 0; i < len(words); i++ {
		word := words[i]
		switch {
		case word == "--raw":
			data = true
			i++
		case hasKey(httpieValueFlags, word):
			i++
		case strings.HasPrefix(word, "-"):
		case method == "" && target == "" && word == strings.ToUpper(word) && strings.Trim(word, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") == "":
			method = word
		case target == "":
			target = word
		default:
			data = data || httpieDataItem(word)
		}
	}
	writes := isMutatingMethod(method) || (method == "" && data)
	return writes && target != "" && !isLoopbackTarget(target)
}

func httpieDataItem(item string) bool {
	cut := strings.IndexAny(item, ":=@")
	if cut < 0 {
		return false
	}
	rest := item[cut+1:]
	switch item[cut] {
	case '=':
		return !strings.HasPrefix(rest, "=")
	case ':':
		return strings.HasPrefix(rest, "=")
	default:
		return true
	}
}

// reachesRemote reports a request target list with any non-loopback entry.
// No target at all, such as a URL in a variable, counts as remote.
func reachesRemote(targets []string) bool {
	if len(targets) == 0 {
		return true
	}
	for _, target := range targets {
		if !isLoopbackTarget(target) {
			return true
		}
	}
	return false
}

// isLoopbackTarget reports a URL, with or without a scheme, whose host is
// localhost, a *.localhost name, a loopback address, or 0.0.0.0, or the
// HTTPie ":port" shorthand for localhost. Anything unparseable is remote.
func isLoopbackTarget(target string) bool {
	if strings.HasPrefix(target, ":") {
		return true
	}
	if !strings.Contains(target, "://") {
		target = "http://" + target
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "0.0.0.0" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}
