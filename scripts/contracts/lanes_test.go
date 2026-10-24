package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Lanes give parallel writers isolated worktrees when the harness cannot, and
// return patches so the lead stays the single writer on its branch.
func TestLanesContract(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	tool := filepath.Join(tmp, "lanes")
	if output, code := run(t, root, nil, "go", "build", "-buildvcs=false", "-o", tool, "./plugins/megapowers/skills/orchestrating/scripts/lanes.go"); code != 0 {
		t.Fatalf("build lanes (%d): %s", code, output)
	}
	gitEnv := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid"}
	repo := filepath.Join(tmp, "repo")
	if err := os.Mkdir(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		output, code := run(t, dir, gitEnv, "git", args...)
		if code != 0 {
			t.Fatalf("git %v (%d): %s", args, code, output)
		}
		return strings.TrimSpace(output)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git(repo, "init", "-q", "-b", "main")
	write(filepath.Join(repo, "a.txt"), "a\n")
	write(filepath.Join(repo, "b.txt"), "b\n")
	// Git quotes non-ASCII names in patch headers by default.
	write(filepath.Join(repo, "café.txt"), "1\n2\n3\n4\n5\n6\n7\n8\n")
	write(filepath.Join(repo, ".gitignore"), "*.secret\n")
	git(repo, "add", "a.txt", "b.txt", "café.txt", ".gitignore")
	git(repo, "commit", "-q", "-m", "base")
	base := git(repo, "rev-parse", "HEAD")
	lanesRoot := filepath.Join(tmp, "lane-root")
	lanes := func(args ...string) (string, int) {
		t.Helper()
		return run(t, repo, gitEnv, tool, append(args, "--root", lanesRoot)...)
	}

	for _, bad := range []string{"Bad", "-x", "a/b", "../x"} {
		if output, code := lanes("add", "--name", bad); code != 125 {
			t.Errorf("bad name %q: want 125, got %d: %s", bad, code, output)
		}
	}
	output, code := lanes("add", "--name", "x")
	if code != 0 || !strings.Contains(output, "base "+base) {
		t.Fatalf("add x: want 0 naming base, got %d: %s", code, output)
	}
	if output, code := lanes("add", "--name", "x"); code != 125 || !strings.Contains(output, "exists") {
		t.Errorf("duplicate lane: want 125, got %d: %s", code, output)
	}
	write(filepath.Join(repo, "lead.txt"), "uncommitted\n")
	if output, code := lanes("add", "--name", "y"); code != 0 || !strings.Contains(output, "uncommitted") {
		t.Fatalf("add y from a dirty lead: want 0 with an uncommitted note, got %d: %s", code, output)
	}
	if err := os.Remove(filepath.Join(repo, "lead.txt")); err != nil {
		t.Fatal(err)
	}
	laneX := filepath.Join(lanesRoot, "x")
	laneY := filepath.Join(lanesRoot, "y")
	if git(laneX, "rev-parse", "HEAD") != base || git(laneX, "rev-parse", "--abbrev-ref", "HEAD") != "HEAD" {
		t.Fatal("lane x must be a detached worktree at the base commit")
	}

	// Lane x edits, adds an untracked file, and commits one change; lane y
	// edits a different file. Both must appear in the lane patch.
	write(filepath.Join(laneX, "a.txt"), "a from x\n")
	write(filepath.Join(laneX, "new.txt"), "new from x\n")
	write(filepath.Join(laneX, "committed.txt"), "committed in x\n")
	git(laneX, "add", "committed.txt")
	git(laneX, "commit", "-q", "-m", "lane commit")
	write(filepath.Join(laneY, "b.txt"), "b from y\n")

	output, code = lanes("diff", "--name", "x")
	for _, want := range []string{"a/a.txt", "b/new.txt", "b/committed.txt"} {
		if code != 0 || !strings.Contains(output, want) {
			t.Errorf("diff x: want 0 containing %s, got %d: %s", want, code, output)
		}
	}
	if output, code := lanes("list"); code != 0 || !strings.Contains(output, "x") || !strings.Contains(output, "y") {
		t.Errorf("list: want both lanes, got %d: %s", code, output)
	}
	if output, code := lanes("check"); code != 0 || !strings.Contains(output, "overlap: none") {
		t.Errorf("disjoint lanes: want 0 naming no overlap, got %d: %s", code, output)
	}

	// Two lanes writing one file breaks disjoint ownership.
	write(filepath.Join(laneY, "a.txt"), "a from y\n")
	if output, code := lanes("check"); code != 3 || !strings.Contains(output, "overlap: a.txt") {
		t.Errorf("overlapping lanes: want 3 naming a.txt, got %d: %s", code, output)
	}
	write(filepath.Join(laneY, "a.txt"), "a\n")

	// Separate hunks of one quoted-name file still break disjoint ownership.
	write(filepath.Join(laneX, "café.txt"), "1 from x\n2\n3\n4\n5\n6\n7\n8\n")
	write(filepath.Join(laneY, "café.txt"), "1\n2\n3\n4\n5\n6\n7\n8 from y\n")
	if output, code := lanes("check"); code != 3 || !strings.Contains(output, "overlap: café.txt") {
		t.Errorf("quoted-name overlap: want 3 naming café.txt, got %d: %s", code, output)
	}
	write(filepath.Join(laneX, "café.txt"), "1\n2\n3\n4\n5\n6\n7\n8\n")
	write(filepath.Join(laneY, "café.txt"), "1\n2\n3\n4\n5\n6\n7\n8\n")

	// A lead change that conflicts with the lane makes its patch unappliable.
	write(filepath.Join(repo, "b.txt"), "b from lead\n")
	if output, code := lanes("check"); code != 3 || !strings.Contains(output, "y: conflicts") {
		t.Errorf("conflicting lane: want 3 naming y conflicts, got %d: %s", code, output)
	}
	write(filepath.Join(repo, "b.txt"), "b\n")

	if output, code := lanes("apply", "--name", "x"); code != 0 {
		t.Fatalf("apply x: want 0, got %d: %s", code, output)
	}
	for path, want := range map[string]string{"a.txt": "a from x\n", "new.txt": "new from x\n", "committed.txt": "committed in x\n"} {
		if got := readFile(t, filepath.Join(repo, path)); got != want {
			t.Errorf("after apply, %s = %q, want %q", path, got, want)
		}
	}
	if git(repo, "rev-parse", "HEAD") != base {
		t.Error("apply must not commit on the lead branch")
	}
	if output, code := lanes("check"); code != 0 || !strings.Contains(output, "x: applied") {
		t.Errorf("after apply: want 0 naming x applied, got %d: %s", code, output)
	}

	if output, code := lanes("remove", "--name", "y"); code != 3 || !strings.Contains(output, "unapplied") {
		t.Errorf("remove lane with unapplied work: want 3, got %d: %s", code, output)
	}
	// Ignored files are not in the patch, so removing the worktree would
	// delete their only copy.
	write(filepath.Join(laneX, "keep.secret"), "only copy\n")
	if output, code := lanes("remove", "--name", "x"); code != 3 || !strings.Contains(output, "keep.secret") {
		t.Errorf("remove lane holding ignored files: want 3 naming keep.secret, got %d: %s", code, output)
	}
	if err := os.Remove(filepath.Join(laneX, "keep.secret")); err != nil {
		t.Fatal(err)
	}
	if output, code := lanes("remove", "--name", "x"); code != 0 {
		t.Errorf("remove applied lane: want 0, got %d: %s", code, output)
	}
	if output, code := lanes("remove", "--name", "y", "--discard"); code != 0 {
		t.Errorf("remove --discard: want 0, got %d: %s", code, output)
	}
	if _, err := os.Stat(laneX); !os.IsNotExist(err) {
		t.Errorf("lane x directory still exists: %v", err)
	}
	if output, code := lanes("list"); code != 0 || !strings.Contains(output, "lanes: 0") {
		t.Errorf("list after remove: want lanes: 0, got %d: %s", code, output)
	}
	if _, err := os.Stat(lanesRoot); !os.IsNotExist(err) {
		t.Errorf("empty lane root still exists: %v", err)
	}
	if strings.Contains(git(repo, "worktree", "list"), lanesRoot) {
		t.Error("git still lists a removed lane worktree")
	}
}
