package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGateIn(t *testing.T, env map[string]string, session, cwd, command string) (string, string, int) {
	t.Helper()
	input := `{"session_id":"` + session + `","cwd":` + strconvQuote(cwd) + `,"tool_name":"Bash","tool_input":{"command":` + strconvQuote(command) + `}}`
	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"deny-destructive"}, func(key string) string { return env[key] }, strings.NewReader(input), &stdout, &stderr)
	return stdout.String(), stderr.String(), rc
}

// branchRepo returns a repository on main with branch merged at HEAD,
// lane-done behind HEAD, and unmerged one commit ahead of main.
func branchRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitEnv := append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"commit", "-q", "--allow-empty", "-m", "base"},
		{"branch", "lane-done"},
		{"commit", "-q", "--allow-empty", "-m", "next"},
		{"branch", "merged"},
		{"checkout", "-q", "-b", "unmerged"},
		{"commit", "-q", "--allow-empty", "-m", "extra"},
		{"checkout", "-q", "main"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = gitEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestGateDiscardSkipsDeletingBranchesMergedIntoHead(t *testing.T) {
	t.Parallel()

	repo := branchRepo(t)
	for _, tt := range []struct{ cwd, command string }{
		{cwd: repo, command: "git branch -D merged"},
		{cwd: repo, command: "git branch -D merged lane-done"},
		{cwd: repo, command: "git branch -d --force merged"},
		{cwd: repo, command: "git branch --delete --force lane-done"},
		{cwd: repo, command: "git worktree remove --force ../wt-lane; git branch -D lane-done"},
		{cwd: t.TempDir(), command: "git -C " + repo + " branch -D merged"},
		{cwd: filepath.Dir(repo), command: "git -C " + filepath.Base(repo) + " branch -D merged"},
	} {
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runGateIn(t, env, "session-merged", tt.cwd, tt.command)
			if rc != 0 || stdout != "" || stderr != "" {
				t.Fatalf("rc=%d stdout=%q stderr=%q, want silence for a merged branch", rc, stdout, stderr)
			}
		})
	}
}

func TestGateDiscardKeepsBranchDeleteWhenWorkCouldBeLost(t *testing.T) {
	t.Parallel()

	repo := branchRepo(t)
	for _, tt := range []struct{ name, cwd, command string }{
		{name: "unmerged", cwd: repo, command: "git branch -D unmerged"},
		{name: "one of two unmerged", cwd: repo, command: "git branch -D merged unmerged"},
		{name: "missing branch", cwd: repo, command: "git branch -D nosuch"},
		{name: "no cwd", cwd: "", command: "git branch -D merged"},
		{name: "relative cwd", cwd: "repo", command: "git branch -D merged"},
		{name: "not a repository", cwd: t.TempDir(), command: "git branch -D merged"},
		{name: "cd first", cwd: repo, command: "cd ../other && git branch -D merged"},
		{name: "git dir override", cwd: repo, command: "git --git-dir=/elsewhere/.git branch -D merged"},
		{name: "merged plus reset", cwd: repo, command: "git branch -D merged && git reset --hard"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			first, _, rc := runGateIn(t, env, "session-unmerged", tt.cwd, tt.command)
			parsed := decodeGate(t, first)
			if rc != 0 || parsed.HookSpecificOutput.PermissionDecision != "deny" || !strings.Contains(parsed.HookSpecificOutput.PermissionDecisionReason, "git status") {
				t.Fatalf("first delete must be denied once: rc=%d %q", rc, first)
			}
			if retry, _, _ := runGateIn(t, env, "session-unmerged", tt.cwd, tt.command); retry != "" {
				t.Fatalf("identical retry must run: %q", retry)
			}
		})
	}
}
