package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateStopsGitDiscardOncePerCommand(t *testing.T) {
	t.Parallel()

	for _, command := range []string{
		"git reset --hard",
		"git reset --hard origin/main",
		"git -C /repo reset --hard HEAD~1",
		"git clean -fdx",
		"git clean --force -d",
		"git clean -f",
		"git checkout .",
		"git checkout -- .",
		"git checkout -f main",
		"git checkout --force main",
		"git restore .",
		"git restore --staged --worktree .",
		"git restore :/",
		"git branch -D feature",
		"git branch -Dr origin/feature",
		"git branch --delete --force feature",
		"git stash drop",
		"git stash drop stash@{1}",
		"git stash clear",
		"cd repo && git reset --hard origin/main",
	} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			first, _, rc := runGate(t, env, "session-discard", command)
			parsed := decodeGate(t, first)
			if rc != 0 || parsed.HookSpecificOutput.PermissionDecision != "deny" || !strings.Contains(parsed.HookSpecificOutput.PermissionDecisionReason, "git status") {
				t.Fatalf("first discard must be denied once with a status check: rc=%d %q", rc, first)
			}
			second, stderr, rc := runGate(t, env, "session-discard", command)
			if rc != 0 || second != "" || stderr != "" {
				t.Fatalf("identical retry must run silently: rc=%d stdout=%q stderr=%q", rc, second, stderr)
			}
		})
	}
}

func TestGateDiscardKeysOnTheExactCommand(t *testing.T) {
	t.Parallel()

	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	runGate(t, env, "session-k", "git reset --hard")
	if retry, _, _ := runGate(t, env, "session-k", "git reset --hard"); retry != "" {
		t.Fatalf("retry must run: %q", retry)
	}
	other, _, _ := runGate(t, env, "session-k", "git clean -fd")
	if !strings.Contains(other, `"permissionDecision":"deny"`) {
		t.Fatalf("a different discard command must stop once: %q", other)
	}
	fresh, _, _ := runGate(t, env, "session-other", "git reset --hard")
	if !strings.Contains(fresh, `"permissionDecision":"deny"`) {
		t.Fatalf("another session must stop once: %q", fresh)
	}
}

func TestGateDiscardStaysQuietForSafeGit(t *testing.T) {
	t.Parallel()

	for _, command := range []string{
		"git clean -n -f",
		"git clean -fn",
		"git clean -f --dry-run",
		"git clean -ndx",
		"git reset --soft HEAD~1",
		"git reset HEAD src/main.go",
		"git reset",
		"git restore --staged .",
		"git restore src/main.go",
		"git checkout main",
		"git checkout -b feat/x",
		"git checkout main -- docs/README.md",
		"git branch -d merged",
		"git branch -D",
		"git stash",
		"git stash pop",
		"git stash list",
		"echo 'git reset --hard'",
		"git log --grep 'reset --hard'",
	} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runGate(t, env, "session-safe", command)
			if rc != 0 || stdout != "" || stderr != "" {
				t.Fatalf("rc=%d stdout=%q stderr=%q, want silence", rc, stdout, stderr)
			}
		})
	}
}

func TestGateDiscardFallsBackToContextWithoutMarker(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, env := range []map[string]string{{"MEGAPOWERS_HOOK_CACHE_DIR": blocked}, {"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}} {
		session := "session-f"
		if env["MEGAPOWERS_HOOK_CACHE_DIR"] != blocked {
			session = ""
		}
		stdout, _, _ := runGate(t, env, session, "git reset --hard")
		parsed := decodeGate(t, stdout)
		if parsed.HookSpecificOutput.PermissionDecision != "" || !strings.Contains(parsed.HookSpecificOutput.AdditionalContext, "git status") {
			t.Fatalf("session %q: %q, want non-blocking context", session, stdout)
		}
	}
}

func TestGateDiscardJoinsSkillGateInOneReason(t *testing.T) {
	t.Parallel()

	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	stdout, _, _ := runGate(t, env, "session-j", "git stash drop && git commit -m x")
	reason := decodeGate(t, stdout).HookSpecificOutput.PermissionDecisionReason
	if !strings.Contains(reason, "verify-and-finish") || !strings.Contains(reason, "git status") {
		t.Fatalf("reason = %q, want both the skill gate and the discard check", reason)
	}
}
