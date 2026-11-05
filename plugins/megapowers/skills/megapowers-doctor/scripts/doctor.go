package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
		report.warn("plugin root unknown: pass --root with the installed plugin directory")
	} else {
		report.add("plugin root: %s", root)
	}
	doctorVersion(report, root)

	harness, signal := detectHarness(getenv)
	report.add("harness: %s (%s)", harness, signal)

	if harness != "codex" {
		doctorClaudeOutputStyle(report, getenv)
	}
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
	default:
		report.add("plugin version: unknown")
		report.warn("no readable plugin manifest under %s: %v", root, claudeErr)
	}
}

// detectHarness uses environment markers.
// Explicit MEGAPOWERS_HARNESS wins, then Codex's PLUGIN_ROOT, then Claude's markers.
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
	return "unknown", "no harness variables; Claude style check follows"
}

// userHome is HOME, or USERPROFILE where HOME is unset (Windows).
func userHome(getenv getenvFunc) string {
	if home := getenv("HOME"); home != "" {
		return home
	}
	return getenv("USERPROFILE")
}

func claudeSettingsPath(getenv getenvFunc) string {
	if dir := getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "settings.json")
	}
	if home := userHome(getenv); home != "" {
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
		report.warn("claude outputStyle: cannot locate settings.json (no HOME, USERPROFILE, or CLAUDE_CONFIG_DIR)")
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

// doctorNow is replaceable so registry expiry tests do not depend on the clock.
var doctorNow = time.Now

// registryExpiry tolerates the inline YAML comment the shipped template keeps.
var registryExpiry = regexp.MustCompile(`(?m)^\s*expires_at:\s*"?(\d{4}-\d{2}-\d{2})"?\s*(#.*)?$`)

// doctorAgentRegistry reports the optional personal registry that
// orchestrating reads. Orchestrating ignores a registry it cannot date, so an
// expired or undated file is a silent fallback to native defaults worth a WARN.
func doctorAgentRegistry(report *doctorReport, getenv getenvFunc) {
	home := userHome(getenv)
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

// doctorMarketplace compares the installed version with the harness's local
// marketplace copy, which a refresh updates before the install does.
func doctorMarketplace(report *doctorReport, getenv getenvFunc, harness, root string) {
	installed, err := manifestVersion(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil {
		return
	}
	home := userHome(getenv)
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

type getenvFunc func(string) string

func main() {
	root := flag.String("root", "", "installed plugin root")
	flag.Parse()
	getenv := func(key string) string {
		if key == "MEGAPOWERS_PLUGIN_ROOT" && *root != "" {
			return *root
		}
		return os.Getenv(key)
	}
	os.Exit(runDoctor(getenv, os.Stdout, os.Stderr))
}

// doctorSkillLoads counts, per skill, the sessions in this workspace from the
// last seven days whose transcript records the skill loading, from successful skill read records. It reads session stores only and prints counts.
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
	workspaces := []string{workspace}
	if output, err := exec.Command("git", "-C", workspace, "rev-parse", "--show-toplevel").Output(); err == nil {
		if top := strings.TrimSpace(string(output)); top != "" && top != workspace {
			workspace = top
			workspaces = append([]string{workspace}, workspaces...)
		}
	}
	files := sessionFiles(getenv, workspaces, time.Now().Add(-7*24*time.Hour))
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

// sessionFiles lists Claude Code transcripts in the workspaces' project
// directories and Codex rollouts whose session_meta names a workspace,
// modified after cutoff.
func sessionFiles(getenv getenvFunc, workspaces []string, cutoff time.Time) []string {
	home := userHome(getenv)
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
		for _, workspace := range workspaces {
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
	}
	if codexDir == "" {
		return files
	}
	// Codex stores rollouts under sessions/YYYY/MM/DD by start date. Reading
	// only the window's days, plus a week for long sessions, and capping the
	// headers examined bounds the work on a large store.
	examined := 0
	for day := 0; day <= 14 && len(files) < maxDoctorSessions && examined < maxDoctorHeaders; day++ {
		dir := filepath.Join(codexDir, "sessions", filepath.FromSlash(time.Now().AddDate(0, 0, -day).Format("2006/01/02")))
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if len(files) >= maxDoctorSessions || examined >= maxDoctorHeaders {
				break
			}
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(entry.Name(), ".jsonl") || !recent(path) {
				continue
			}
			examined++
			if cwd := codexSessionCwd(path); cwd != "" {
				for _, workspace := range workspaces {
					if filepath.Clean(cwd) == filepath.Clean(workspace) {
						files = append(files, path)
						break
					}
				}
			}
		}
	}
	return files
}

// maxDoctorHeaders bounds the Codex rollout headers doctor reads, and
// maxCodexHeaderBytes the first line it buffers from one rollout.
const (
	maxDoctorHeaders    = 2000
	maxCodexHeaderBytes = 8 << 20
)

// codexSessionCwd returns the cwd from a rollout's session_meta first line,
// or "" when the line is not a session_meta record or exceeds the limit.
func codexSessionCwd(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	line, err := bufio.NewReader(io.LimitReader(file, maxCodexHeaderBytes)).ReadBytes('\n')
	if err != nil {
		return ""
	}
	var meta struct {
		Type    string `json:"type"`
		Payload struct {
			Cwd string `json:"cwd"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &meta) != nil || meta.Type != "session_meta" {
		return ""
	}
	return meta.Payload.Cwd
}

// skillLoaded reports whether the transcript records the skill body loading:
// an injected skill body, or a successful tool result paired with a read or
// Skill invocation. Catalog entries, pending reads, and failed calls do not
// count. An unreadable
// transcript counts as not loaded and reports readable false.
func skillLoaded(transcriptPath, skill string) (loaded, readable bool) {
	if transcriptPath == "" {
		return false, false
	}
	file, err := os.Open(transcriptPath)
	if err != nil {
		return false, false
	}
	defer file.Close()
	pending := make(map[string]bool)
	reader := bufio.NewReaderSize(file, 64<<10)
	for {
		line, err := reader.ReadBytes('\n')
		if len(pending) != 0 || bytes.Contains(line, []byte(skill)) {
			var event skillTranscriptEvent
			if json.Unmarshal(line, &event) == nil && event.loadedSkill(skill, pending) {
				return true, true
			}
		}
		if err != nil {
			return false, true
		}
	}
}

type skillTranscriptEvent struct {
	Type, Role, Name, ID string
	CallID               string `json:"call_id"`
	ToolUseID            string `json:"tool_use_id"`
	IsError              bool   `json:"is_error"`
	Arguments            json.RawMessage
	Action               json.RawMessage
	Input                json.RawMessage
	Output               json.RawMessage
	Content              json.RawMessage
	Text                 string
	Message, Payload     *skillTranscriptEvent
}

func (event skillTranscriptEvent) loadedSkill(skill string, pending map[string]bool) bool {
	if event.Payload != nil {
		return event.Payload.loadedSkill(skill, pending)
	}
	if event.Message != nil {
		if event.Message.Role == "" {
			event.Message.Role = event.Type
		}
		return event.Message.loadedSkill(skill, pending)
	}
	if event.Role == "user" {
		text, _ := skillResultText(event.Content)
		if strings.HasPrefix(text, "Base directory for this skill:") && strings.Contains(text, "skills/"+skill) && strings.Contains(text, "\n# ") {
			return true
		}
		if strings.HasPrefix(text, "<skill>\n<name>megapowers:"+skill+"</name>") && strings.Contains(text, "skills/"+skill+"/SKILL.md") && strings.Contains(text, "\n# ") {
			return true
		}
	}
	if len(event.Content) > 0 && event.Content[0] == '[' {
		var blocks []skillTranscriptEvent
		if json.Unmarshal(event.Content, &blocks) == nil {
			for _, block := range blocks {
				if block.loadedSkill(skill, pending) {
					return true
				}
			}
		}
	}
	id := event.CallID
	if id == "" {
		id = event.ID
	}
	switch event.Type {
	case "tool_use", "function_call", "custom_tool_call", "local_shell_call":
		input := string(event.Input) + string(event.Arguments) + string(event.Action)
		var invocation struct{ Skill string }
		isSkill := event.Name == "Skill" && json.Unmarshal(event.Input, &invocation) == nil && invocation.Skill == "megapowers:"+skill
		reads := event.Type == "local_shell_call"
		switch event.Name {
		case "Read", "Bash", "exec_command", "shell_command", "shell", "exec", "functions.exec", "functions.exec_command":
			reads = true
		}
		if id != "" && (isSkill || (reads && strings.Contains(input, "skills/"+skill+"/SKILL.md"))) {
			pending[id] = isSkill
		}
	case "tool_result", "function_call_output", "custom_tool_call_output", "local_shell_call_output":
		if event.ToolUseID != "" {
			id = event.ToolUseID
		}
		invocation, ok := pending[id]
		delete(pending, id)
		if !ok || event.IsError {
			return false
		}
		output := event.Output
		if len(output) == 0 {
			output = event.Content
		}
		if invocation {
			text, success := skillResultText(output)
			return success && strings.TrimSpace(text) != ""
		}
		for _, text := range successfulResultTexts(output) {
			if containsSkillBody(text, skill) {
				return true
			}
		}
		return false
	}
	return false
}

// successfulResultTexts returns the text of each command result inside a tool
// output that did not fail. A batched code-mode call prints one result per
// command, often as Promise.allSettled entries ({"i":N,"result":{"status",
// "value"}}), so each result counts on its own: a failed sibling command does
// not hide a successful skill read, and a failed or rejected read never counts.
func successfulResultTexts(raw json.RawMessage) []string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		trimmed := strings.TrimSpace(text)
		if (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(trimmed)) {
			return successfulResultTexts(json.RawMessage(trimmed))
		}
		if failedReadExit.MatchString(text) {
			return nil
		}
		return []string{text}
	}
	var blocks []json.RawMessage
	if json.Unmarshal(raw, &blocks) == nil {
		var texts []string
		for _, block := range blocks {
			texts = append(texts, successfulResultTexts(block)...)
		}
		return texts
	}
	var result struct {
		IsError                                      bool   `json:"is_error"`
		MCPError                                     bool   `json:"isError"`
		ExitCode                                     *int   `json:"exit_code"`
		Status                                       string `json:"status"`
		Output, Content, Text, Stdout, Value, Result json.RawMessage
	}
	if json.Unmarshal(raw, &result) != nil || result.IsError || result.MCPError || (result.ExitCode != nil && *result.ExitCode != 0) || result.Status == "rejected" {
		return nil
	}
	for _, field := range []json.RawMessage{result.Output, result.Content, result.Text, result.Stdout, result.Value, result.Result} {
		if len(field) != 0 {
			return successfulResultTexts(field)
		}
	}
	return nil
}

var readLineNumber = regexp.MustCompile(`(?m)^\s*[0-9]+[→\t :]+`)
var failedReadExit = regexp.MustCompile(`(?m)(?:Process exited with code|Exit code:)[ \t]+[1-9][0-9]*\b`)

func containsSkillBody(text, skill string) bool {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = readLineNumber.ReplaceAllString(text, "")
	for _, name := range []string{skill, `"` + skill + `"`, "'" + skill + "'"} {
		if strings.Contains(text, "\nname: "+name+"\n") && strings.Contains(text, "\n# ") {
			return true
		}
	}
	return false
}

// Tool results can be plain text, text blocks, or an encoded exec result.
// Decode only output fields; a path in an input or an error is not evidence.
func skillResultText(raw json.RawMessage) (string, bool) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			if nested, ok := skillResultText(json.RawMessage(trimmed)); nested != "" || !ok {
				return nested, ok
			}
		}
		return text, !failedReadExit.MatchString(text)
	}
	var blocks []json.RawMessage
	if json.Unmarshal(raw, &blocks) == nil {
		var texts []string
		for _, block := range blocks {
			text, ok := skillResultText(block)
			if !ok {
				return "", false
			}
			texts = append(texts, text)
		}
		return strings.Join(texts, "\n"), true
	}
	var result struct {
		IsError                              bool `json:"is_error"`
		ExitCode                             *int `json:"exit_code"`
		Output, Content, Text, Stdout, Value json.RawMessage
	}
	if json.Unmarshal(raw, &result) != nil || result.IsError || (result.ExitCode != nil && *result.ExitCode != 0) {
		return "", false
	}
	for _, field := range []json.RawMessage{result.Output, result.Content, result.Text, result.Stdout, result.Value} {
		if len(field) != 0 {
			return skillResultText(field)
		}
	}
	return "", true
}
