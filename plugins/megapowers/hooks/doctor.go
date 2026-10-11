package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Doctor is a plain-text, read-only diagnostic. It never reads stdin, never
// writes outside stdout, and exits 0 with WARN lines for problems so a skill
// can interpret the report. Deterministic policy lives here; judgment lives
// in the megapowers-doctor skill.

const expectedOutputStyle = "megapowers:Megapowers"

type doctorReport struct {
	lines []string
}

func (r *doctorReport) add(format string, args ...any) {
	r.lines = append(r.lines, fmt.Sprintf(format, args...))
}

func (r *doctorReport) warn(format string, args ...any) {
	r.lines = append(r.lines, "WARN: "+fmt.Sprintf(format, args...))
}

func runDoctor(getenv getenvFunc, output, errors io.Writer) int {
	report := &doctorReport{}
	report.add("megapowers doctor")

	root := getenv("MEGAPOWERS_PLUGIN_ROOT")
	if root == "" {
		root = getenv("PLUGIN_ROOT")
	}
	if root == "" {
		root = getenv("CLAUDE_PLUGIN_ROOT")
	}
	if root == "" {
		report.warn("plugin root unknown: run through hooks/run-hook.cmd doctor so MEGAPOWERS_PLUGIN_ROOT is set")
	} else {
		report.add("plugin root: %s", root)
	}
	doctorVersion(report, root)

	harness, signal := detectHarness(getenv)
	report.add("harness: %s (%s)", harness, signal)

	doctorGo(report)
	doctorRunnerCache(report, getenv)

	if harness != "codex" {
		doctorClaudeOutputStyle(report, getenv)
	}
	if harness != "claude" {
		doctorCodexOutputStyle(report, getenv)
	}
	doctorHooks(report, root)
	doctorHookRuns(report, getenv, harness)
	doctorMarketplace(report, getenv, harness, root)
	doctorSkillLoads(report, getenv, root)
	doctorAgentRegistry(report, getenv)

	if _, err := io.WriteString(output, strings.Join(report.lines, "\n")+"\n"); err != nil {
		fmt.Fprintln(errors, "megapowers doctor: cannot write report")
		return 1
	}
	return 0
}

func manifestVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", err
	}
	if manifest.Version == "" {
		return "", fmt.Errorf("no version field")
	}
	return manifest.Version, nil
}

func doctorVersion(report *doctorReport, root string) {
	claude, claudeErr := manifestVersion(filepath.Join(root, ".claude-plugin", "plugin.json"))
	codex, codexErr := manifestVersion(filepath.Join(root, ".codex-plugin", "plugin.json"))
	switch {
	case claudeErr == nil:
		report.add("plugin version: %s (.claude-plugin/plugin.json)", claude)
		if codexErr == nil && codex != claude {
			report.warn(".codex-plugin/plugin.json version %s differs from %s", codex, claude)
		}
	case codexErr == nil:
		report.add("plugin version: %s (.codex-plugin/plugin.json)", codex)
		report.warn(".claude-plugin/plugin.json unreadable: %v", claudeErr)
	default:
		report.add("plugin version: unknown")
		report.warn("no readable plugin manifest under %s: %v", root, claudeErr)
	}
}

// detectHarness mirrors the runtime detection the hooks use: an explicit
// MEGAPOWERS_HARNESS wins, then Codex's PLUGIN_ROOT, then Claude's markers.
func detectHarness(getenv getenvFunc) (string, string) {
	switch getenv("MEGAPOWERS_HARNESS") {
	case "claude":
		return "claude", "MEGAPOWERS_HARNESS"
	case "codex":
		return "codex", "MEGAPOWERS_HARNESS"
	}
	if getenv("PLUGIN_ROOT") != "" {
		return "codex", "PLUGIN_ROOT"
	}
	if getenv("CLAUDECODE") != "" {
		return "claude", "CLAUDECODE"
	}
	if getenv("CLAUDE_PLUGIN_ROOT") != "" {
		return "claude", "CLAUDE_PLUGIN_ROOT"
	}
	return "unknown", "no harness variables; both style checks follow"
}

func doctorGo(report *doctorReport) {
	report.add("runner built with: %s (%s)", runtime.Version(), runnerPath())
	goBinary, err := exec.LookPath("go")
	if err != nil {
		report.warn("go not on PATH: the cached runner keeps working, but a plugin update needs Go 1.25 or newer to rebuild it")
		return
	}
	cmd := exec.Command(goBinary, "env", "GOVERSION")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GO111MODULE=off")
	out, err := cmd.Output()
	version := strings.TrimSpace(string(out))
	if err != nil || version == "" {
		report.warn("go toolchain: cannot read `go env GOVERSION` from %s", goBinary)
		return
	}
	report.add("go toolchain: %s (%s)", version, goBinary)
	if err := requireGo125(version); err != nil {
		report.warn("go toolchain %s: the launcher refuses to build with it; install Go 1.25 or newer", version)
	}
}

func runnerPath() string {
	path, err := os.Executable()
	if err != nil {
		return "runner path unknown"
	}
	return path
}

func doctorRunnerCache(report *doctorReport, getenv getenvFunc) {
	dir := gateMarkerRoot(getenv)
	if dir == "" {
		report.warn("runner cache: no HOME, XDG_CACHE_HOME, or MEGAPOWERS_HOOK_CACHE; the launcher falls back to a temporary cache and rebuilds every call")
		return
	}
	runners, _ := filepath.Glob(filepath.Join(dir, "megapowers-hook-*"))
	report.add("runner cache: %s (%d cached runner(s))", dir, len(runners))
}

func claudeSettingsPath(getenv getenvFunc) string {
	if dir := getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "settings.json")
	}
	if home := getenv("HOME"); home != "" {
		return filepath.Join(home, ".claude", "settings.json")
	}
	return ""
}

// doctorClaudeOutputStyle reports the effective outputStyle: project-local
// and project settings override the user file, so a stale value there turns
// the style off even when the user file is correct.
func doctorClaudeOutputStyle(report *doctorReport, getenv getenvFunc) {
	project := getenv("CLAUDE_PROJECT_DIR")
	if project == "" {
		project = getenv("PWD")
	}
	user := claudeSettingsPath(getenv)
	if project != "" {
		for _, path := range []string{filepath.Join(project, ".claude", "settings.local.json"), filepath.Join(project, ".claude", "settings.json")} {
			if path == user {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			style, err := outputStyleOf(data)
			if err != nil {
				report.warn("claude outputStyle: %s is not valid JSON (%v)", path, err)
				return
			}
			if style != "" {
				reportClaudeStyle(report, style, path+" overrides the user settings")
				return
			}
		}
	}
	if user == "" {
		report.warn("claude outputStyle: cannot locate settings.json (no HOME or CLAUDE_CONFIG_DIR)")
		return
	}
	data, err := os.ReadFile(user)
	if err != nil {
		report.warn("claude outputStyle: %s not readable (%v); the plugin style is optional, set \"outputStyle\": %q there to enable it", user, err, expectedOutputStyle)
		return
	}
	style, err := outputStyleOf(data)
	if err != nil {
		report.warn("claude outputStyle: %s is not valid JSON (%v)", user, err)
		return
	}
	if style == "" {
		report.warn("claude outputStyle: not set in %s; the plugin style is optional, set \"outputStyle\": %q to enable it", user, expectedOutputStyle)
		return
	}
	reportClaudeStyle(report, style, user)
}

func outputStyleOf(data []byte) (string, error) {
	var settings struct {
		OutputStyle string `json:"outputStyle"`
	}
	err := json.Unmarshal(data, &settings)
	return settings.OutputStyle, err
}

func reportClaudeStyle(report *doctorReport, style, source string) {
	if style == expectedOutputStyle {
		report.add("claude outputStyle: %s (OK, %s)", style, source)
		return
	}
	report.warn("claude outputStyle: %s in %s; the plugin style resolves only as %q (bare \"Megapowers\" is silently ignored)", style, source, expectedOutputStyle)
}

func doctorCodexOutputStyle(report *doctorReport, getenv getenvFunc) {
	switch value := getenv("MEGAPOWERS_OUTPUT_STYLE"); value {
	case "":
		report.add("MEGAPOWERS_OUTPUT_STYLE: unset (style injection enabled)")
	case "off":
		report.add("MEGAPOWERS_OUTPUT_STYLE: off (style injection disabled)")
	default:
		report.add("MEGAPOWERS_OUTPUT_STYLE: %s (style injection enabled; only \"off\" disables it)", value)
	}
}

// doctorNow is replaceable so registry expiry tests do not depend on the clock.
var doctorNow = time.Now

// registryExpiry tolerates the inline YAML comment the shipped template keeps.
var registryExpiry = regexp.MustCompile(`(?m)^\s*expires_at:\s*"?(\d{4}-\d{2}-\d{2})"?\s*(#.*)?$`)

// doctorAgentRegistry reports the optional personal registry that
// orchestrating reads. Orchestrating ignores a registry it cannot date, so an
// expired or undated file is a silent fallback to native defaults worth a WARN.
func doctorAgentRegistry(report *doctorReport, getenv getenvFunc) {
	home := getenv("HOME")
	if home == "" {
		return
	}
	path := filepath.Join(home, ".config", "megapowers", "agent-capabilities.md")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		report.add("agent registry: none (optional; orchestrating uses native defaults)")
		return
	}
	if err != nil {
		report.warn("agent registry: %s not readable (%v); orchestrating ignores it", path, err)
		return
	}
	match := registryExpiry.FindSubmatch(data)
	if match == nil {
		report.warn("agent registry: %s has no readable expires_at; orchestrating ignores it", path)
		return
	}
	expires, err := time.Parse(time.DateOnly, string(match[1]))
	if err != nil {
		report.warn("agent registry: %s has no readable expires_at (%s); orchestrating ignores it", path, match[1])
		return
	}
	if doctorNow().UTC().Format(time.DateOnly) > expires.Format(time.DateOnly) {
		report.warn("agent registry: %s expired %s; orchestrating ignores it until refreshed", path, match[1])
		return
	}
	report.add("agent registry: %s (expires %s)", path, match[1])
}

func doctorHooks(report *doctorReport, root string) {
	path := filepath.Join(root, "hooks", "hooks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		report.warn("hooks.json unreadable at %s: %v", path, err)
		return
	}
	var manifest struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		report.warn("hooks.json at %s is not valid JSON: %v", path, err)
		return
	}
	var events []string
	for event, groups := range manifest.Hooks {
		var matchers []string
		for _, group := range groups {
			if group.Matcher != "" {
				matchers = append(matchers, group.Matcher)
			}
		}
		if len(matchers) > 0 {
			event += " (" + strings.Join(matchers, ", ") + ")"
		}
		events = append(events, event)
	}
	sort.Strings(events)
	report.add("hooks.json events: %s (declared; a loaded session shows whether the harness ran them)", strings.Join(events, ", "))
	// An event key with no command entry registers nothing the harness can run.
	for _, required := range []string{"PreToolUse", "SessionStart", "SubagentStart"} {
		registered := false
		for _, group := range manifest.Hooks[required] {
			for _, hook := range group.Hooks {
				if strings.TrimSpace(hook.Command) != "" {
					registered = true
				}
			}
		}
		if !registered {
			report.warn("hooks.json does not register %s", required)
		}
	}
}

// markHookRun touches one marker per harness and hook in the runner cache, so
// doctor can tell a harness that never ran the hooks, such as a Codex install
// whose hooks were never trusted. It never creates the cache directory.
func markHookRun(getenv getenvFunc, hook string) {
	dir := gateMarkerRoot(getenv)
	harness, _ := detectHarness(getenv)
	if dir == "" || harness == "unknown" {
		return
	}
	path := filepath.Join(dir, "hook-ran-"+harness+"-"+hook)
	now := time.Now()
	if os.Chtimes(path, now, now) == nil {
		return
	}
	if file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		file.Close()
	}
}

func doctorHookRuns(report *doctorReport, getenv getenvFunc, harness string) {
	dir := gateMarkerRoot(getenv)
	if dir == "" {
		return
	}
	harnesses := []string{harness}
	if harness == "unknown" {
		harnesses = []string{"claude", "codex"}
	}
	for _, name := range harnesses {
		markers, _ := filepath.Glob(filepath.Join(dir, "hook-ran-"+name+"-*"))
		if len(markers) == 0 {
			fix := "check that the plugin is enabled, then start a new session"
			if name == "codex" {
				fix = "review and trust the plugin hooks, then start a new session"
			}
			report.warn("no megapowers hook has run on %s; %s", name, fix)
			continue
		}
		sort.Strings(markers)
		var runs []string
		for _, marker := range markers {
			info, err := os.Stat(marker)
			if err != nil {
				continue
			}
			hook := strings.TrimPrefix(filepath.Base(marker), "hook-ran-"+name+"-")
			runs = append(runs, fmt.Sprintf("%s %s ago", hook, time.Since(info.ModTime()).Round(time.Second)))
		}
		report.add("hooks last ran (%s): %s", name, strings.Join(runs, ", "))
	}
}

// doctorSkillLoads counts, per skill, the sessions in this workspace from the
// last seven days whose transcript records the skill loading, using the same
// detection as the gates. It reads session stores only and prints counts.
func doctorSkillLoads(report *doctorReport, getenv getenvFunc, root string) {
	workspace := getenv("PWD")
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	var catalog struct {
		Skills []struct {
			Name string `json:"name"`
		} `json:"skills"`
	}
	data, err := os.ReadFile(filepath.Join(root, "skills", "catalog.json"))
	if err != nil || json.Unmarshal(data, &catalog) != nil || workspace == "" {
		return
	}
	files := sessionFiles(getenv, workspace, time.Now().Add(-7*24*time.Hour))
	if len(files) == 0 {
		report.add("skill loads, last 7 days in %s: no sessions found", workspace)
		return
	}
	var loads []string
	for _, skill := range catalog.Skills {
		count := 0
		for _, file := range files {
			if loaded, _ := skillLoaded(file, skill.Name); loaded {
				count++
			}
		}
		if count > 0 {
			loads = append(loads, fmt.Sprintf("%s %d", skill.Name, count))
		}
	}
	sort.Strings(loads)
	summary := "none"
	if len(loads) > 0 {
		summary = strings.Join(loads, ", ")
	}
	report.add("skill loads, last 7 days in %s: %d transcript(s); %s", workspace, len(files), summary)
}

// maxDoctorSessions bounds the transcripts doctor reads in one report.
const maxDoctorSessions = 500

var claudeProjectSlug = regexp.MustCompile(`[^A-Za-z0-9]`)

// sessionFiles lists Claude Code transcripts in the workspace's project
// directory and Codex rollouts whose session_meta names the workspace,
// modified after cutoff.
func sessionFiles(getenv getenvFunc, workspace string, cutoff time.Time) []string {
	home := getenv("HOME")
	claudeDir := getenv("CLAUDE_CONFIG_DIR")
	if claudeDir == "" && home != "" {
		claudeDir = filepath.Join(home, ".claude")
	}
	codexDir := getenv("CODEX_HOME")
	if codexDir == "" && home != "" {
		codexDir = filepath.Join(home, ".codex")
	}
	recent := func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.Mode().IsRegular() && info.ModTime().After(cutoff)
	}
	var files []string
	if claudeDir != "" {
		project := filepath.Join(claudeDir, "projects", claudeProjectSlug.ReplaceAllString(workspace, "-"))
		for _, pattern := range []string{"*.jsonl", "*/subagents/*.jsonl"} {
			matches, _ := filepath.Glob(filepath.Join(project, pattern))
			for _, match := range matches {
				if recent(match) && len(files) < maxDoctorSessions {
					files = append(files, match)
				}
			}
		}
	}
	if codexDir == "" {
		return files
	}
	quoted, _ := json.Marshal(workspace)
	cwdField := []byte(`"cwd":` + string(quoted))
	filepath.WalkDir(filepath.Join(codexDir, "sessions"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || len(files) >= maxDoctorSessions {
			return nil
		}
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(entry.Name(), ".jsonl") || !recent(path) {
			return nil
		}
		if firstLineContains(path, cwdField) {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func firstLineContains(path string, needle []byte) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	line, _ := bufio.NewReaderSize(file, 64<<10).ReadBytes('\n')
	return bytes.Contains(line, needle)
}

// doctorMarketplace compares the installed version with the harness's local
// marketplace copy, which a refresh updates before the install does.
func doctorMarketplace(report *doctorReport, getenv getenvFunc, harness, root string) {
	installed, err := manifestVersion(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil {
		return
	}
	home := getenv("HOME")
	copies := map[string]string{}
	if harness != "codex" {
		if dir := getenv("CLAUDE_CONFIG_DIR"); dir != "" {
			copies["claude"] = filepath.Join(dir, "plugins", "marketplaces", "megapowers", "plugins", "megapowers", ".claude-plugin", "plugin.json")
		} else if home != "" {
			copies["claude"] = filepath.Join(home, ".claude", "plugins", "marketplaces", "megapowers", "plugins", "megapowers", ".claude-plugin", "plugin.json")
		}
	}
	if harness != "claude" {
		if dir := getenv("CODEX_HOME"); dir != "" {
			copies["codex"] = filepath.Join(dir, ".tmp", "marketplaces", "megapowers", "plugins", "megapowers", ".codex-plugin", "plugin.json")
		} else if home != "" {
			copies["codex"] = filepath.Join(home, ".codex", ".tmp", "marketplaces", "megapowers", "plugins", "megapowers", ".codex-plugin", "plugin.json")
		}
	}
	for _, name := range []string{"claude", "codex"} {
		path, ok := copies[name]
		if !ok {
			continue
		}
		version, err := manifestVersion(path)
		if err != nil {
			continue
		}
		if version != installed {
			report.warn("%s marketplace copy is %s but %s is installed; use upgrading-megapowers", name, version, installed)
			continue
		}
		report.add("%s marketplace copy: %s", name, version)
	}
}
