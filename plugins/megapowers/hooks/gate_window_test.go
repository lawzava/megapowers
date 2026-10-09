package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A parallel sibling of the first gated call reaches the hook after the
// once-marker exists but before the skill loads; it must stop too.
func TestGateDeniesParallelSiblingBeforeSkillLoads(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ first, sibling, skill string }{
		{first: "gh pr comment 12 --body a", sibling: "gh pr comment 13 --body b", skill: "safe-effects"},
		{first: "git commit -m one", sibling: "git push origin main", skill: "verify-and-finish"},
	} {
		t.Run(tt.sibling, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			transcript := writeTranscript(t, `{"type":"user","message":{"content":"post both comments"}}`)
			first, _, _ := runGateWithTranscript(t, env, "session-par", transcript, tt.first)
			if decodeGate(t, first).HookSpecificOutput.PermissionDecision != "deny" {
				t.Fatalf("first call must be denied: %q", first)
			}
			sibling, _, rc := runGateWithTranscript(t, env, "session-par", transcript, tt.sibling)
			parsed := decodeGate(t, sibling)
			if rc != 0 || parsed.HookSpecificOutput.PermissionDecision != "deny" || !strings.Contains(parsed.HookSpecificOutput.PermissionDecisionReason, "megapowers "+tt.skill+" skill") {
				t.Fatalf("sibling inside the window must be denied naming %s: rc=%d %q", tt.skill, rc, sibling)
			}
		})
	}
}

func TestGateWindowEndsWhenSkillLoads(t *testing.T) {
	t.Parallel()

	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	path := writeTranscript(t, `{"type":"user","message":{"content":"post it"}}`)
	runGateWithTranscript(t, env, "session-load", path, "gh pr comment 1 --body a")
	loaded := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"megapowers:safe-effects"}}]}}`
	if err := os.WriteFile(path, []byte(`{"type":"user","message":{"content":"post it"}}`+"\n"+loaded+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if retry, _, _ := runGateWithTranscript(t, env, "session-load", path, "gh pr comment 1 --body a"); retry != "" {
		t.Fatalf("retry after the skill loaded must run silently: %q", retry)
	}
}

// When transcript detection fails, the window expires and the retry runs, so
// the gate can never block forever.
func TestGateWindowExpiresWithoutSkillLoad(t *testing.T) {
	t.Parallel()

	cache := t.TempDir()
	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": cache}
	transcript := writeTranscript(t, `{"type":"user","message":{"content":"post it"}}`)
	runGateWithTranscript(t, env, "session-exp", transcript, "gh pr comment 1 --body a")
	markers, err := filepath.Glob(filepath.Join(cache, "gate-context", "*-safe-effects"))
	if err != nil || len(markers) != 1 {
		t.Fatalf("markers = %v (err %v), want 1", markers, err)
	}
	past := time.Now().Add(-gateRetryWindow - time.Second)
	if err := os.Chtimes(markers[0], past, past); err != nil {
		t.Fatal(err)
	}
	if retry, _, _ := runGateWithTranscript(t, env, "session-exp", transcript, "gh pr comment 1 --body a"); retry != "" {
		t.Fatalf("retry after the window must run silently: %q", retry)
	}
}

// Claude Code records a Read or Bash tool call on SKILL.md as a tool_use; the
// window must not keep denying an agent that loaded the skill that way.
func TestGateCountsClaudeToolReadOfSkillBody(t *testing.T) {
	t.Parallel()

	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	transcript := writeTranscript(t,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/cache/megapowers/0.34.0/skills/safe-effects/SKILL.md"}}]}}`)
	if stdout, _, _ := runGateWithTranscript(t, env, "session-read", transcript, "gh pr comment 1 --body a"); stdout != "" {
		t.Fatalf("a Read of the skill body is a load: %q", stdout)
	}
}
