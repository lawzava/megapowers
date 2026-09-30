package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The gate stops the first command in a session that completes work or
// reaches outside the machine until the matching skill has loaded: it denies
// that command once with a reason naming the skill, and the retry runs. A
// reminder alone arrived with the command it was meant to precede and was
// mostly ignored. When the once-marker cannot be recorded (no session ID or an
// unusable cache), the gate falls back to non-blocking context so it can never
// block the same command twice.
const (
	completionSkill = "verify-and-finish"
	effectSkill     = "safe-effects"

	completionContext = "Before claiming this complete, load and follow the megapowers verify-and-finish skill if it is not already loaded for this outcome."
	effectContext     = "Before running this outward effect, load and follow the megapowers safe-effects skill if it is not already loaded for this effect."
	completionDeny    = "Load and follow the megapowers verify-and-finish skill before this completion step, then run the command again; this check blocks once per session."
	effectDeny        = "Load and follow the megapowers safe-effects skill before this outward effect, then run the command again; this check blocks once per session."

	gateMarkerDir = "gate-context"
	gateMarkerTTL = 7 * 24 * time.Hour
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

// gateDecision returns the skills whose first gated command this session
// must stop on, and the skills that only get a reminder because the
// once-marker could not be recorded. A skill the transcript shows as already
// loaded needs neither.
func gateDecision(command, sessionID, transcriptPath string, getenv getenvFunc) (deny, remind []string) {
	root := gateMarkerRoot(getenv)
	for _, skill := range gateSkills(command) {
		state := markGate(root, sessionID, skill)
		if state == markerSeen || skillLoaded(transcriptPath, skill) {
			continue
		}
		if state == markerNew {
			deny = append(deny, skill)
		} else {
			remind = append(remind, skill)
		}
	}
	return deny, remind
}

func gateMessages(skills []string, completion, effect string) string {
	var lines []string
	for _, skill := range skills {
		if skill == completionSkill {
			lines = append(lines, completion)
		} else {
			lines = append(lines, effect)
		}
	}
	return strings.Join(lines, "\n")
}

// skillLoaded reports whether the transcript records the skill body loading:
// a Skill tool call, an injected skill body, or a tool call that reads the
// skill's SKILL.md. Catalog entries and prose mentions do not count. An
// unreadable transcript counts as not loaded.
func skillLoaded(transcriptPath, skill string) bool {
	if transcriptPath == "" {
		return false
	}
	file, err := os.Open(transcriptPath)
	if err != nil {
		return false
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
			return true
		case bytes.Contains(line, []byte("Base directory for this skill:")) && bytes.Contains(line, skillDir):
			return true
		case bytes.Contains(line, skillBody) && (bytes.Contains(line, []byte("\"function_call\"")) ||
			bytes.Contains(line, []byte("\"custom_tool_call\"")) || bytes.Contains(line, []byte("\"local_shell_call\""))):
			return true
		}
		if err != nil {
			return false
		}
	}
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
	}
	return ""
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
	digest := sha256.Sum256([]byte(sessionID))
	marker := filepath.Join(dir, hex.EncodeToString(digest[:8])+"-"+skill)
	file, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
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
