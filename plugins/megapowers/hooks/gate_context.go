package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Gate context is advisory: an allowed command that completes work or reaches
// outside the machine earns one short reminder naming the skill to load. It
// never sets a permission decision.
const (
	completionSkill = "verify-and-finish"
	effectSkill     = "safe-effects"

	completionContext = "Before claiming this complete, load and follow the megapowers verify-and-finish skill if it is not already loaded for this outcome."
	effectContext     = "Before running this outward effect, load and follow the megapowers safe-effects skill if it is not already loaded for this effect."

	gateMarkerDir = "gate-context"
	gateMarkerTTL = 7 * 24 * time.Hour
)

var (
	gitValueOptions = map[string]struct{}{"-C": {}, "-c": {}, "--git-dir": {}, "--work-tree": {}, "--namespace": {}}
	ghValueOptions  = map[string]struct{}{"-R": {}, "--repo": {}}
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

func gateContext(command, sessionID string, getenv getenvFunc) string {
	skills := gateSkills(command)
	if len(skills) == 0 {
		return ""
	}
	var lines []string
	for _, skill := range skills {
		if markGateSeen(gateMarkerRoot(getenv), sessionID, skill) {
			continue
		}
		if skill == completionSkill {
			lines = append(lines, completionContext)
		} else {
			lines = append(lines, effectContext)
		}
	}
	return strings.Join(lines, "\n")
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
		}
	case "npm", "pnpm", "cargo":
		if subcommand(words, nil) == "publish" {
			return effectSkill
		}
	case "yarn":
		sub := subcommand(words, nil)
		if sub == "publish" || (sub == "npm" && secondSubcommand(words, nil) == "publish") {
			return effectSkill
		}
	case "docker":
		if subcommand(words, nil) == "push" {
			return effectSkill
		}
	case "kubectl":
		if sub := subcommand(words, nil); sub == "apply" || sub == "delete" {
			return effectSkill
		}
	case "helm":
		if sub := subcommand(words, nil); sub == "upgrade" || sub == "install" {
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

// ghAPIMutates: gh api sends POST when given fields or an input body, and any
// explicit non-GET method is a write.
func ghAPIMutates(words []string) bool {
	for i, word := range words {
		lower := strings.ToUpper(word)
		switch {
		case word == "-X" || word == "--method":
			if i+1 < len(words) && isMutatingMethod(words[i+1]) {
				return true
			}
		case strings.HasPrefix(word, "-X") && isMutatingMethod(word[2:]):
			return true
		case strings.HasPrefix(lower, "--METHOD=") && isMutatingMethod(word[len("--method="):]):
			return true
		case word == "-f" || word == "-F" || word == "--field" || word == "--raw-field" || word == "--input" ||
			strings.HasPrefix(word, "--field=") || strings.HasPrefix(word, "--raw-field=") || strings.HasPrefix(word, "--input="):
			return true
		}
	}
	return false
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

// markGateSeen records that a session already received the reminder for a
// skill. It reports true only when a marker already existed; any error fails
// open so the reminder is emitted.
func markGateSeen(root, sessionID, skill string) bool {
	if root == "" || sessionID == "" {
		return false
	}
	dir := filepath.Join(root, gateMarkerDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	digest := sha256.Sum256([]byte(sessionID))
	marker := filepath.Join(dir, hex.EncodeToString(digest[:8])+"-"+skill)
	file, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.Is(err, os.ErrExist)
	}
	file.Close()
	pruneGateMarkers(dir, time.Now().Add(-gateMarkerTTL))
	return false
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
