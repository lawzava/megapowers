package contracts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFenceCheckoutAndLeakContract(t *testing.T) {
	root, tmp := repoRoot(t), t.TempDir()
	tool := filepath.Join(tmp, "fence-check")
	if out, code := run(t, root, nil, "go", "build", "-o", tool, "./plugins/megapowers/skills/orchestrating/scripts/fence-check/fence.go"); code != 0 {
		t.Fatalf("build fence-check (%d): %s", code, out)
	}
	env := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid"}
	repo := filepath.Join(tmp, "lead")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		out, code := run(t, dir, env, "git", args...)
		if code != 0 {
			t.Fatalf("git %v (%d): %s", args, code, out)
		}
		return strings.TrimSpace(out)
	}
	write := func(dir, path, body string) {
		t.Helper()
		p := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git(repo, "init", "-q", "-b", "main")
	for _, p := range []string{"src/a.go", "src/nested/b.go", "src-other/no.go", "old.txt", "staged.txt"} {
		write(repo, p, "base\n")
	}
	write(repo, ".gitignore", "*.ignored\n")
	git(repo, "add", ".")
	git(repo, "commit", "-qm", "base")
	base := git(repo, "rev-parse", "HEAD")
	worker := filepath.Join(tmp, "arbitrary-worker")
	git(repo, "worktree", "add", "--detach", "-q", worker, base)
	check := func(head string, fences ...string) (string, int) {
		t.Helper()
		args := []string{"check", "--repo", repo, "--base", base, "--head", head}
		for _, f := range fences {
			args = append(args, "--fence", f)
		}
		return run(t, repo, env, tool, args...)
	}
	// Committed, staged, unstaged, untracked and both rename endpoints count.
	write(worker, "src/a.go", "committed\n")
	git(worker, "add", "src/a.go")
	git(worker, "commit", "-qm", "worker commit")
	head := git(worker, "rev-parse", "HEAD")
	// Base refs belong to the lead, not the worker's newly advanced HEAD.
	write(worker, "committed-outside.md", "outside\n")
	git(worker, "add", "committed-outside.md")
	git(worker, "commit", "-qm", "outside fence")
	for _, baseRef := range []string{"HEAD", "main"} {
		out, code := run(t, repo, env, tool, "check", "--repo", repo, "--base", baseRef, "--head", worker, "--fence", "src/")
		if code != 3 || !strings.Contains(out, `"committed-outside.md"`) {
			t.Fatalf("lead base %s: %d: %s", baseRef, code, out)
		}
	}
	write(worker, "src/nested/b.go", "unstaged\n")
	write(worker, "staged.txt", "index only\n")
	git(worker, "add", "staged.txt")
	write(worker, "staged.txt", "base\n")
	write(worker, "new café\nfile.txt", "untracked\n")
	write(worker, "src-other/no.go", "not prefix\n")
	write(worker, "hidden.ignored", "ignored\n")
	git(worker, "mv", "old.txt", "renamed.txt")
	before := git(worker, "ls-files", "--stage", "-z")
	workerIndex := git(worker, "rev-parse", "--path-format=absolute", "--git-path", "index")
	indexBytes := readFile(t, workerIndex)
	out, code := check(worker, "src/")
	if code != 3 {
		t.Fatalf("checkout: want 3, got %d: %s", code, out)
	}
	for _, p := range []string{"staged.txt", "old.txt", "renamed.txt", "new café\nfile.txt", "src-other/no.go"} {
		if !strings.Contains(out, strconv.Quote(p)) {
			t.Errorf("missing violation %q: %s", p, out)
		}
	}
	for _, p := range []string{"src/a.go", "src/nested/b.go", "hidden.ignored"} {
		if strings.Contains(out, strconv.Quote(p)) {
			t.Errorf("unexpected violation %q: %s", p, out)
		}
	}
	if after := git(worker, "ls-files", "--stage", "-z"); after != before {
		t.Fatal("check mutated the real index")
	}
	if readFile(t, workerIndex) != indexBytes {
		t.Fatal("check rewrote index bytes")
	}
	if out, code := check(worker, "src/", "src-other/no.go", "*.txt", "committed-outside.md"); code != 0 {
		t.Fatalf("union fence: %d: %s", code, out)
	}
	if out, code := check(head, "src/a.go"); code != 0 {
		t.Fatalf("revision must ignore dirty worktree: %d: %s", code, out)
	}
	if out, code := check(worker, "src/*", "*.txt"); code != 3 || !strings.Contains(out, `"src-other/no.go"`) {
		t.Fatalf("glob crosses slash, not prefix: %d: %s", code, out)
	}
	if out, code := check(worker, "*"); code != 0 {
		t.Fatalf("star crosses slash: %d: %s", code, out)
	}
	// Scope checks must not inherit index flags that hide tracked worktree edits.
	git(worker, "update-index", "--assume-unchanged", "src/nested/b.go")
	if out, code := check(worker, "*.txt"); code != 3 || !strings.Contains(out, `"src/nested/b.go"`) {
		t.Fatalf("assume-unchanged must not hide paths: %d: %s", code, out)
	}
	for _, bad := range []string{"", "/src/", "../src/", "src/../", "./src", "src//", `src\*`} {
		if out, code := check(worker, bad); code != 125 {
			t.Errorf("invalid fence %q: %d: %s", bad, code, out)
		}
	}
	if out, code := check(worker); code != 125 {
		t.Errorf("missing fence: %d: %s", code, out)
	}
	if out, code := check("does-not-exist", "*"); code != 125 {
		t.Errorf("invalid head: %d: %s", code, out)
	}
	// A plain ref name that is also a directory must not silently pick a mode.
	write(repo, "main/file.txt", "directory collides with branch\n")
	if out, code := check("main", "*"); code != 125 || !strings.Contains(out, "ambiguous") {
		t.Fatalf("ambiguous head: %d: %s", code, out)
	}
	// Hook environments must not redirect worker commands into the lead.
	hookEnv := append(append([]string{}, env...), "GIT_DIR="+filepath.Join(repo, ".git"), "GIT_WORK_TREE="+repo, "GIT_INDEX_FILE="+filepath.Join(repo, ".git/index"), "GIT_COMMON_DIR="+filepath.Join(repo, ".git"))
	if out, code := run(t, repo, hookEnv, tool, "check", "--repo", repo, "--base", base, "--head", worker, "--fence", "src/"); code != 3 || !strings.Contains(out, `"committed-outside.md"`) {
		t.Fatalf("hook environment: %d: %s", code, out)
	}
	clone := filepath.Join(tmp, "independent-checkout")
	git(repo, "clone", "-q", worker, clone)
	for _, extra := range [][]string{
		{"GIT_CONFIG_PARAMETERS='core.worktree=" + repo + "'"},
		{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.worktree", "GIT_CONFIG_VALUE_0=" + repo},
		{"GIT_CONFIG_PARAMETERS='core.sparseCheckout=true'"},
	} {
		inherited := append(append([]string{}, hookEnv...), extra...)
		if out, code := run(t, repo, inherited, tool, "check", "--repo", repo, "--base", base, "--head", clone, "--fence", "src/"); code != 3 || !strings.Contains(out, `"committed-outside.md"`) {
			t.Fatalf("hook config override: %d: %s", code, out)
		}
	}
	// Capture a dirty lead before dispatch; do not blame pre-existing edits.
	write(repo, "staged.txt", "pre-existing staged\n")
	git(repo, "add", "staged.txt")
	write(repo, "staged.txt", "base\n")
	write(repo, "user.txt", "pre-existing untracked\n")
	indexBefore := git(repo, "ls-files", "--stage", "-z")
	leadIndex := git(repo, "rev-parse", "--path-format=absolute", "--git-path", "index")
	leadIndexBytes := readFile(t, leadIndex)
	snapshot := filepath.Join(tmp, "lead-before.json")
	if out, code := run(t, repo, env, tool, "snapshot", "--repo", repo, "--out", snapshot); code != 0 {
		t.Fatalf("snapshot: %d: %s", code, out)
	}
	if info, err := os.Stat(snapshot); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("snapshot must be private: %v %v", info, err)
	}
	leak := func() (string, int) { return run(t, repo, env, tool, "leak", "--repo", repo, "--since", snapshot) }
	if out, code := leak(); code != 0 {
		t.Fatalf("unchanged dirty lead: %d: %s", code, out)
	}
	if after := git(repo, "ls-files", "--stage", "-z"); after != indexBefore {
		t.Fatal("snapshot/leak mutated the real index")
	}
	if readFile(t, leadIndex) != leadIndexBytes {
		t.Fatal("snapshot/leak rewrote index bytes")
	}
	write(repo, "src/a.go", "worker escaped\n")
	write(repo, "leaked.txt", "worker escaped too\n")
	git(repo, "reset", "-q", "HEAD", "--", "staged.txt")
	if out, code := leak(); code != 3 || !strings.Contains(out, `"src/a.go"`) || !strings.Contains(out, `"leaked.txt"`) || !strings.Contains(out, `"staged.txt"`) || strings.Contains(out, `"user.txt"`) {
		t.Fatalf("lead drift including index-only: %d: %s", code, out)
	}
	write(repo, "committed-leak.txt", "committed\n")
	git(repo, "add", "committed-leak.txt")
	git(repo, "commit", "-qm", "lead escape")
	if err := os.Remove(filepath.Join(repo, "committed-leak.txt")); err != nil {
		t.Fatal(err)
	}
	if out, code := leak(); code != 3 || !strings.Contains(out, `"committed-leak.txt"`) {
		t.Fatalf("committed drift must survive worktree reversal: %d: %s", code, out)
	}
	if out, code := run(t, repo, env, tool, "leak", "--repo", worker, "--since", snapshot); code != 125 {
		t.Fatalf("wrong checkout baseline: %d: %s", code, out)
	}
	if out, code := run(t, repo, env, tool, "snapshot", "--repo", repo, "--out", snapshot); code != 125 {
		t.Fatalf("must not overwrite baseline: %d: %s", code, out)
	}
	if out, code := run(t, repo, env, tool, "snapshot", "--repo", repo, "--out", filepath.Join(repo, "baseline.json")); code != 125 {
		t.Fatalf("baseline inside lead: %d: %s", code, out)
	}
	write(repo, "bad-baseline.json", "{}")
	if out, code := run(t, repo, env, tool, "leak", "--repo", repo, "--since", filepath.Join(repo, "bad-baseline.json")); code != 125 {
		t.Fatalf("malformed baseline: %d: %s", code, out)
	}
	// Refuse baseline option injection before invoking Git with those values.
	data := readFile(t, snapshot)
	var saved map[string]any
	if err := json.Unmarshal([]byte(data), &saved); err != nil {
		t.Fatal(err)
	}
	injected := filepath.Join(tmp, "injected.txt")
	saved["head"] = "--output=" + injected
	bad, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "tampered.json"), bad, 0600); err != nil {
		t.Fatal(err)
	}
	if out, code := run(t, repo, env, tool, "leak", "--repo", repo, "--since", filepath.Join(tmp, "tampered.json")); code != 125 {
		t.Fatalf("tampered baseline: %d: %s", code, out)
	}
	if _, err := os.Stat(injected); !os.IsNotExist(err) {
		t.Fatalf("tampered baseline caused output write: %v", err)
	}
	// Unsupported index/repository shapes fail closed rather than claiming scope.
	git(worker, "update-index", "--add", "--cacheinfo", "160000,"+base+",submodule")
	if out, code := check(worker, "*"); code != 125 || !strings.Contains(out, "submodules are unsupported") {
		t.Fatalf("submodule: %d: %s", code, out)
	}
	git(worker, "update-index", "--force-remove", "submodule")
	git(worker, "config", "core.sparseCheckout", "true")
	if out, code := check(worker, "*"); code != 125 || !strings.Contains(out, "sparse checkouts are unsupported") {
		t.Fatalf("sparse checkout: %d: %s", code, out)
	}
	git(worker, "config", "--unset", "core.sparseCheckout")
	blob := git(worker, "rev-parse", base+":staged.txt")
	unmerged := "100644 " + blob + " 1\tconflict.txt\n100644 " + blob + " 2\tconflict.txt\n"
	cmd := exec.Command("git", "-C", worker, "update-index", "--index-info")
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(unmerged)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("unmerged fixture: %v: %s", err, out)
	}
	if out, code := check(worker, "*"); code != 125 || !strings.Contains(out, "unmerged") {
		t.Fatalf("unmerged index: %d: %s", code, out)
	}
}
