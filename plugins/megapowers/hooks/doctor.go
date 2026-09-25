package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
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

	doctorGo(report, getenv)
	doctorRunnerCache(report, getenv)

	if harness != "codex" {
		doctorClaudeOutputStyle(report, getenv)
	}
	if harness != "claude" {
		doctorCodexOutputStyle(report, getenv)
	}
	doctorHooks(report, root)

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

func doctorGo(report *doctorReport, getenv getenvFunc) {
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
	_ = getenv
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

func doctorClaudeOutputStyle(report *doctorReport, getenv getenvFunc) {
	path := claudeSettingsPath(getenv)
	if path == "" {
		report.warn("claude outputStyle: cannot locate settings.json (no HOME or CLAUDE_CONFIG_DIR)")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		report.warn("claude outputStyle: %s not readable (%v); the plugin style is optional, set \"outputStyle\": %q there to enable it", path, err, expectedOutputStyle)
		return
	}
	var settings struct {
		OutputStyle string `json:"outputStyle"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		report.warn("claude outputStyle: %s is not valid JSON (%v)", path, err)
		return
	}
	switch settings.OutputStyle {
	case expectedOutputStyle:
		report.add("claude outputStyle: %s (OK, %s)", settings.OutputStyle, path)
	case "":
		report.warn("claude outputStyle: not set in %s; the plugin style is optional, set \"outputStyle\": %q to enable it", path, expectedOutputStyle)
	default:
		report.warn("claude outputStyle: %s in %s; the plugin style resolves only as %q (bare \"Megapowers\" is silently ignored)", settings.OutputStyle, path, expectedOutputStyle)
	}
}

func doctorCodexOutputStyle(report *doctorReport, getenv getenvFunc) {
	switch value := getenv("MEGAPOWERS_OUTPUT_STYLE"); value {
	case "":
		report.add("MEGAPOWERS_OUTPUT_STYLE: unset (style injected)")
	case "off":
		report.add("MEGAPOWERS_OUTPUT_STYLE: off (style injection disabled)")
	default:
		report.add("MEGAPOWERS_OUTPUT_STYLE: %s (style injected; only \"off\" disables it)", value)
	}
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
	report.add("hooks.json events: %s", strings.Join(events, ", "))
	for _, required := range []string{"PreToolUse", "SessionStart", "SubagentStart"} {
		if _, ok := manifest.Hooks[required]; !ok {
			report.warn("hooks.json does not register %s", required)
		}
	}
}
