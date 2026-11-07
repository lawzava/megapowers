package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A manual actor records the documented artifacts with file tools. It emits
// no shell trace, so naming or executing an extras helper cannot buy a pass.
type manualRecoveryActor struct{ damage string }

func (manualRecoveryActor) name() string          { return "manual" }
func (manualRecoveryActor) supportsCompact() bool { return true }
func (a manualRecoveryActor) session(ctx context.Context, dir string, spec sessionSpec) ([]byte, string, error) {
	_, id, err := (scripted{label: "fixture-work"}).session(ctx, dir, spec)
	if err != nil || spec.prompt == promptCompac {
		return nil, id, err
	}
	state := filepath.Join(dir, ".megapowers", "run", runID)
	_, head := command(dir, "git", "rev-parse", "HEAD")
	_, branch := command(dir, "git", "branch", "--show-current")
	_, worktree := command(dir, "git", "rev-parse", "--show-toplevel")
	if a.damage == "short-head" {
		head = strings.TrimSpace(head)[:7]
	}
	checkpoint := fmt.Sprintf("- Workspace: %s\n- Branch: %s\n- Worktree: %s\n- HEAD: %s\n", dir, strings.TrimSpace(branch), strings.TrimSpace(worktree), strings.TrimSpace(head))
	journal := func(evidence string) string {
		data, _ := json.Marshal(map[string]string{"ts": "2026-11-07T12:00:00Z", "event": "oracle", "evidence": evidence, "head": strings.TrimSpace(head)})
		return string(data) + "\n"
	}
	text := journal("C1 TestParse passed")
	if spec.prompt == prompt1 {
		checkpoint += "- Milestone: C1\n"
	} else {
		checkpoint += "- Milestone: C1 C2 C3 complete\n"
		text += journal("C1 TestParse, C2 TestSum, C3 TestDocs passed; go test ./... exit 0")
	}
	if err := os.WriteFile(filepath.Join(state, "checkpoint.md"), []byte(checkpoint), 0600); err != nil {
		return nil, id, err
	}
	if err := os.WriteFile(filepath.Join(state, "handoff.md"), []byte(checkpoint), 0600); err != nil {
		return nil, id, err
	}
	if err := os.WriteFile(filepath.Join(state, "journal.jsonl"), []byte(text), 0600); err != nil {
		return nil, id, err
	}
	if spec.prompt != prompt1 {
		switch a.damage {
		case "journal":
			err = os.WriteFile(filepath.Join(state, "journal.jsonl"), []byte(journal("replaced history")), 0600)
		case "checkpoint":
			err = os.WriteFile(filepath.Join(state, "checkpoint.md"), []byte(strings.ReplaceAll(checkpoint, strings.TrimSpace(head), "wrong-head")), 0600)
		case "charter":
			err = os.WriteFile(filepath.Join(state, "charter.md"), []byte("C1 TestParse\nC3 TestDocs\n"), 0600)
		}
	} else {
		switch a.damage {
		case "missing-s1-journal":
			err = os.Remove(filepath.Join(state, "journal.jsonl"))
		case "missing-s1-checkpoint":
			err = os.Remove(filepath.Join(state, "checkpoint.md"))
		}
	}
	return nil, id, err
}

func TestManualRecoveryUsesDurableState(t *testing.T) {
	for _, tc := range []struct{ damage, want, check string }{
		{"", "pass", ""},
		{"short-head", "pass", ""},
		{"journal", "fail", "journal_appended"},
		{"checkpoint", "fail", "checkpoint_updated_matches_checkout"},
		{"charter", "fail", "charter_preserves_criteria"},
		{"missing-s1-journal", "indeterminate", "s1_journal_present"},
		{"missing-s1-checkpoint", "indeterminate", "s1_checkpoint_present"},
	} {
		t.Run("manual-"+tc.damage, func(t *testing.T) {
			r := runOne(context.Background(), manualRecoveryActor{damage: tc.damage}, t.TempDir(), "fresh", 1)
			if r.Verdict != tc.want {
				t.Fatalf("got %s (%s), want %s", r.Verdict, r.Reason, tc.want)
			}
			if tc.check != "" {
				if ok, present := r.Checks[tc.check]; !present || ok {
					t.Fatalf("expected %s to fail: %v", tc.check, r.Checks)
				}
			} else {
				for name, ok := range r.Checks {
					if !ok {
						t.Errorf("unexpected failing check: %s", name)
					}
				}
			}
		})
	}
}

func TestCharterNamesCriteriaAcrossLines(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"C1: TestParse\nC2: TestSum\nC3: TestDocs", true},
		{"## C1\n- TestParse\n## C2\n- TestSum\n## C3\n- TestDocs", true},
		{"C1: TestSum\nC2: TestParse\nC3: TestDocs", false},
		{"C1: TestParse\nC3: TestDocs", false},
	} {
		if got := charterNamesCriteria([]byte(tc.text)); got != tc.want {
			t.Errorf("charter %q: got %v, want %v", tc.text, got, tc.want)
		}
	}
}
