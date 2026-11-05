package contracts

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSessionMineRepositoryOnly(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "plugins/megapowers/skills/writing-agent-instructions/scripts/session-mine.go")); !os.IsNotExist(err) {
		t.Fatalf("session miner must stay outside the plugin: %v", err)
	}
	buildSessionMine(t, root)
}

func TestSessionMineClaudeCodexBoundary(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	base := []string{"--all-workspaces", "--claude-root", t.TempDir(), "--codex-root", t.TempDir(), "--format", "json"}
	for _, removed := range [][]string{{"--opencode-export", t.TempDir()}, {"--harness", "opencode"}, {"--harness", "all"}} {
		if out, code := run(t, root, nil, tool, append(append([]string{}, base...), removed...)...); code != 2 {
			t.Errorf("removed option accepted (%d): %s", code, out)
		}
	}
	cmd := exec.Command(tool, base...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil || stderr.Len() != 0 {
		t.Fatalf("default scan: %v %s", err, stderr.String())
	}
	var report struct {
		BySource map[string]json.RawMessage `json:"by_source"`
	}
	if err := json.Unmarshal(data, &report); err != nil || len(report.BySource) != 2 || report.BySource["claude"] == nil || report.BySource["codex"] == nil {
		t.Fatalf("default sources: %s (%v)", data, err)
	}
}

func TestOpenCodeMineDefaultSource(t *testing.T) {
	root := repoRoot(t)
	tool := buildOpenCodeMine(t, root)
	dir := t.TempDir()
	input := filepath.Join(dir, "session.json")
	if err := os.WriteFile(input, []byte(openCodeExport(openCodeInfo("default-source", "/work/demo", ""), openCodeMessage("user", "Did you test it?"))), 0600); err != nil {
		t.Fatal(err)
	}
	output, code := run(t, root, nil, tool, "--all-workspaces", "--opencode-export", input, "--claude-root", t.TempDir(), "--codex-root", t.TempDir(), "--format", "json")
	var report struct {
		BySource map[string]json.RawMessage `json:"by_source"`
		Sessions int                        `json:"sessions_scanned"`
	}
	if code != 0 || json.Unmarshal([]byte(output), &report) != nil || len(report.BySource) != 1 || report.BySource["opencode"] == nil || report.Sessions != 1 {
		t.Fatalf("default source (%d): %s", code, output)
	}
}

func TestSessionMineSourceSummaryIsolation(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join(root, sessionMineTool))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "session-mine.go")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	check := filepath.Join(dir, "summary_test.go")
	const unit = `package main
import "testing"
func TestSummaryIsolation(t *testing.T) {
 sessions := []session{{harness:"claude", msgs:[]message{{role:"human"}}}, {harness:"codex"}, {harness:"unexpected"}}
 report := summarizeSources(options{harness:"claude"}, sessions)
 if len(report.BySource) != 1 || report.BySource["claude"].SessionsScanned != 1 || report.BySource["claude"].HumanTurns != 1 { t.Fatalf("unexpected counts: %+v", report) }
}
`
	if err := os.WriteFile(check, []byte(unit), 0600); err != nil {
		t.Fatal(err)
	}
	if out, code := run(t, dir, nil, "go", "test", source, check); code != 0 {
		t.Fatal(out)
	}
}
