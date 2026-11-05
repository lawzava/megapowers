package contracts

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"
)

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
