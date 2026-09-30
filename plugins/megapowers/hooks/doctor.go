package main

import (
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
