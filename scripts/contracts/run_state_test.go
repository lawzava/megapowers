package contracts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRunStateContract(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	tool := filepath.Join(tmp, "run-state")
	if output, code := run(t, root, nil, "go", "build", "-buildvcs=false", "-o", tool, "./plugins/megapowers/skills/autonomous-run/scripts/run-state.go"); code != 0 {
		t.Fatalf("build run-state (%d): %s", code, output)
	}
	gitEnv := []string{
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Run State Test",
		"GIT_AUTHOR_EMAIL=run-state@example.invalid",
		"GIT_COMMITTER_NAME=Run State Test",
		"GIT_COMMITTER_EMAIL=run-state@example.invalid",
	}
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
	commit := func(dir, name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		git(dir, "add", name)
		git(dir, "commit", "-q", "-m", "add "+name)
	}
	git(repo, "init", "-q", "-b", "main")
	commit(repo, "one.txt")
	// Resolve symlinked temp roots so path comparisons match Git's output.
	repo = git(repo, "rev-parse", "--show-toplevel")

	state := func(dir string, args ...string) (string, int) {
		t.Helper()
		return run(t, dir, gitEnv, tool, args...)
	}
	runDir := filepath.Join(repo, ".megapowers", "run", "alpha")
	exclude := filepath.Join(repo, ".git", "info", "exclude")
	excludeCount := func() int {
		t.Helper()
		data, err := os.ReadFile(exclude)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == ".megapowers/" {
				count++
			}
		}
		return count
	}

	if output, code := state(repo, "--help"); code != 0 || !strings.Contains(output, "resume") {
		t.Errorf("--help failed (%d): %s", code, output)
	}
	for _, bad := range []string{"Bad", "-lead", "a/b", "../x", strings.Repeat("a", 64)} {
		if output, code := state(repo, "init", "--id", bad, "--objective", "x"); code != 125 {
			t.Errorf("bad id %q: want 125, got %d: %s", bad, code, output)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".megapowers")); err == nil {
		t.Error("bad id created run state")
	}
	if output, _ := state(repo, "init", "--id", "Bad", "--objective", "x"); !strings.Contains(output, "result: error (exit 125)") {
		t.Errorf("error output lacks result line: %s", output)
	}

	for _, target := range []string{"file", "dir"} {
		t.Run("symlinked exclude "+target, func(t *testing.T) {
			symRepo := filepath.Join(tmp, "sym-"+target)
			if err := os.Mkdir(symRepo, 0o700); err != nil {
				t.Fatal(err)
			}
			git(symRepo, "init", "-q", "-b", "main")
			notes := filepath.Join(tmp, "notes-"+target)
			if err := os.MkdirAll(notes, 0o700); err != nil {
				t.Fatal(err)
			}
			notesFile := filepath.Join(notes, "exclude")
			original := "unrelated notes\n"
			if err := os.WriteFile(notesFile, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			info := filepath.Join(symRepo, ".git", "info")
			link, dest := filepath.Join(info, "exclude"), notesFile
			if target == "dir" {
				link, dest = info, notes
			}
			if err := os.RemoveAll(link); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(dest, link); err != nil {
				t.Fatal(err)
			}
			output, code := state(symRepo, "init", "--id", "alpha", "--objective", "x")
			if code != 125 || !strings.Contains(output, "symlink") {
				t.Errorf("symlinked exclude: want 125 naming symlink, got %d: %s", code, output)
			}
			if got := readFile(t, notesFile); got != original {
				t.Errorf("init wrote through symlink: %q", got)
			}
			if _, err := os.Lstat(filepath.Join(symRepo, ".megapowers")); err == nil {
				t.Error("refused init still created run state")
			}
		})
	}

	output, code := state(repo, "init", "--id", "alpha", "--objective", "Ship the\nwidget")
	if code != 0 {
		t.Fatalf("init failed (%d): %s", code, output)
	}
	for _, name := range []string{"charter.md", "journal.jsonl"} {
		info, err := os.Stat(filepath.Join(runDir, name))
		if err != nil {
			t.Fatalf("init did not create %s: %v", name, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s mode %v, want 0600", name, info.Mode().Perm())
		}
	}
	if info, err := os.Stat(runDir); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("run dir mode wrong: %v %v", info, err)
	}
	charter := readFile(t, filepath.Join(runDir, "charter.md"))
	for _, heading := range []string{"## Objective", "## Done criteria", "## Scope", "## Authority", "## Cap", "Ship the widget"} {
		requireContains(t, charter, heading, "charter")
	}
	if got := excludeCount(); got != 1 {
		t.Errorf("exclude entries after first init: %d", got)
	}
	if output, code := state(repo, "init", "--id", "beta", "--objective", "Second run"); code != 0 {
		t.Fatalf("second init failed (%d): %s", code, output)
	}
	if got := excludeCount(); got != 1 {
		t.Errorf("exclude entries after second init: %d", got)
	}
	if status := git(repo, "status", "--porcelain"); status != "" {
		t.Errorf("run state visible to git status: %q", status)
	}
	if output, code := state(repo, "init", "--id", "alpha", "--objective", "Again"); code != 125 || !strings.Contains(output, "exists") {
		t.Errorf("duplicate init: want 125 naming exists, got %d: %s", code, output)
	}
	if !strings.Contains(readFile(t, filepath.Join(runDir, "charter.md")), "Ship the widget") {
		t.Error("duplicate init overwrote charter")
	}

	head := git(repo, "rev-parse", "HEAD")
	output, code = state(tmp, "checkpoint", "--repo", repo, "--id", "alpha", "--milestone", "Parser done", "--next", "Run the full suite", "--blocker", "Waiting on review", "--evidence", "go test passed", "--evidence", "lint passed")
	if code != 0 {
		t.Fatalf("checkpoint failed (%d): %s", code, output)
	}
	checkpoint := readFile(t, filepath.Join(runDir, "checkpoint.md"))
	for _, want := range []string{head, "main", repo, "Parser done", "go test passed", "lint passed", "Waiting on review", "Run the full suite", "Dirty: no"} {
		requireContains(t, checkpoint, want, "checkpoint")
	}
	handoff := readFile(t, filepath.Join(runDir, "handoff.md"))
	for _, want := range []string{filepath.Join(runDir, "charter.md"), filepath.Join(runDir, "checkpoint.md"), "Run the full suite"} {
		requireContains(t, handoff, want, "handoff")
	}
	for _, name := range []string{"checkpoint.md", "handoff.md"} {
		if info, err := os.Stat(filepath.Join(runDir, name)); err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("%s mode wrong: %v %v", name, info, err)
		}
	}

	output, code = state(repo, "resume", "--id", "alpha")
	if code != 0 {
		t.Fatalf("matching resume: want 0, got %d: %s", code, output)
	}
	for _, want := range []string{"head: match", "branch: match", "worktree: match", "workspace: match", "Run the full suite", "Waiting on review", "checkpoint"} {
		requireContains(t, output, want, "matching resume")
	}

	journalPath := filepath.Join(runDir, "journal.jsonl")
	beforeJournal := readFile(t, journalPath)
	long := strings.Repeat("x", 3000)
	if output, code := state(repo, "journal", "--id", "alpha", "--event", "decision", "--evidence", "bench\nshowed 2x", "--decision", "Keep the cache", "--why", "Cold start dominated"); code != 0 {
		t.Fatalf("journal failed (%d): %s", code, output)
	}
	if output, code := state(repo, "journal", "--id", "alpha", "--event", "observation", "--evidence", long); code != 0 {
		t.Fatalf("long journal failed (%d): %s", code, output)
	}
	if output, code := state(repo, "journal", "--id", "alpha", "--event", "decision", "--evidence", "x", "--decision", "No reason"); code != 125 {
		t.Errorf("decision without why: want 125, got %d: %s", code, output)
	}
	afterJournal := readFile(t, journalPath)
	if !strings.HasPrefix(afterJournal, beforeJournal) {
		t.Error("journal was rewritten instead of appended")
	}
	records := journalRecords(t, afterJournal)
	if len(records) != 4 {
		t.Fatalf("journal records: want 4 (init, checkpoint, decision, observation), got %d", len(records))
	}
	if records[0]["event"] != "init" || records[1]["event"] != "checkpoint" || records[1]["head"] != head || records[1]["branch"] != "main" || records[1]["worktree"] != repo {
		t.Errorf("init/checkpoint records wrong: %v %v", records[0], records[1])
	}
	decision := records[2]
	if decision["decision"] != "Keep the cache" || decision["why"] != "Cold start dominated" || decision["evidence"] != "bench showed 2x" || decision["head"] != head {
		t.Errorf("decision record wrong: %v", decision)
	}
	if got := len([]rune(records[3]["evidence"].(string))); got != 2000 {
		t.Errorf("long evidence length %d, want 2000", got)
	}

	commit(repo, "two.txt")
	output, code = state(repo, "resume", "--id", "alpha")
	if code != 3 || !strings.Contains(output, "head: mismatch: recorded "+head) || !strings.Contains(output, "(exit 3)") {
		t.Errorf("HEAD drift: want 3 naming head mismatch, got %d: %s", code, output)
	}
	if output, code := state(repo, "checkpoint", "--id", "alpha", "--milestone", "Second", "--next", "Continue"); code != 0 {
		t.Fatalf("second checkpoint failed (%d): %s", code, output)
	}
	if output, code := state(repo, "resume", "--id", "alpha"); code != 0 {
		t.Errorf("resume after fresh checkpoint: want 0, got %d: %s", code, output)
	}
	git(repo, "switch", "-q", "-c", "other")
	output, code = state(repo, "resume", "--id", "alpha")
	if code != 3 || !strings.Contains(output, "branch: mismatch: recorded main, live other") || !strings.Contains(output, "head: match") {
		t.Errorf("branch drift: want 3 naming branch mismatch, got %d: %s", code, output)
	}
	if output, code := state(repo, "resume", "--id", "beta"); code != 125 || !strings.Contains(output, "checkpoint") {
		t.Errorf("resume without checkpoint: want 125, got %d: %s", code, output)
	}
	if output, code := state(repo, "resume", "--id", "missing"); code != 125 {
		t.Errorf("resume of unknown run: want 125, got %d: %s", code, output)
	}
	journalRecords(t, readFile(t, journalPath))

	linked := filepath.Join(tmp, "linked")
	git(repo, "worktree", "add", "-q", "-b", "lane", linked)
	linked = git(linked, "rev-parse", "--show-toplevel")
	if output, code := state(linked, "init", "--id", "gamma", "--objective", "Lane work"); code != 0 {
		t.Fatalf("init in linked worktree failed (%d): %s", code, output)
	}
	if got := excludeCount(); got != 1 {
		t.Errorf("exclude entries after linked init: %d", got)
	}
	if output, code := state(linked, "checkpoint", "--id", "gamma", "--milestone", "Start", "--next", "Edit"); code != 0 {
		t.Fatalf("linked checkpoint failed (%d): %s", code, output)
	}
	linkedCheckpoint := readFile(t, filepath.Join(linked, ".megapowers", "run", "gamma", "checkpoint.md"))
	requireContains(t, linkedCheckpoint, "Workspace: "+repo, "linked checkpoint workspace")
	requireContains(t, linkedCheckpoint, "Worktree: "+linked, "linked checkpoint worktree")
	requireContains(t, linkedCheckpoint, "Branch: lane", "linked checkpoint branch")
}

func TestRunStateContinuationCapAndCoverage(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	tool := filepath.Join(tmp, "run-state")
	if output, code := run(t, root, nil, "go", "build", "-buildvcs=false", "-o", tool, "./plugins/megapowers/skills/autonomous-run/scripts/run-state.go"); code != 0 {
		t.Fatalf("build run-state (%d): %s", code, output)
	}
	gitEnv := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}
	repo := filepath.Join(tmp, "repo")
	if err := os.Mkdir(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if output, code := run(t, repo, gitEnv, "git", "init", "-q", "-b", "main"); code != 0 {
		t.Fatalf("git init (%d): %s", code, output)
	}
	state := func(args ...string) (string, int) {
		t.Helper()
		return run(t, repo, gitEnv, tool, args...)
	}
	if output, code := state("init", "--id", "cap", "--objective", "Keep CI green"); code != 0 {
		t.Fatalf("init failed (%d): %s", code, output)
	}
	runDir := filepath.Join(repo, ".megapowers", "run", "cap")
	charterPath := filepath.Join(runDir, "charter.md")
	requireContains(t, readFile(t, charterPath), "- C1:", "charter criterion ID skeleton")

	// The cap is enforced by the tool, not left to the agent's count.
	for i, want := range []string{"1 of 3", "2 of 3", "3 of 3"} {
		output, code := state("continue", "--id", "cap", "--item", "fix lint on CI")
		if code != 0 || !strings.Contains(output, want) {
			t.Fatalf("continuation %d: want 0 naming %q, got %d: %s", i+1, want, code, output)
		}
	}
	for i := 0; i < 2; i++ {
		output, code := state("continue", "--id", "cap", "--item", "fix lint on CI")
		if code != 4 || !strings.Contains(output, "blocked for review") || !strings.Contains(output, "(exit 4)") {
			t.Fatalf("over cap: want 4 naming blocked, got %d: %s", code, output)
		}
	}
	if output, code := state("continue", "--id", "cap", "--item", "rebase on main", "--max", "1"); code != 0 || !strings.Contains(output, "1 of 1") {
		t.Fatalf("independent item: want 0, got %d: %s", code, output)
	}
	if output, code := state("continue", "--id", "cap", "--item", "rebase on main", "--max", "1"); code != 4 {
		t.Fatalf("custom cap: want 4, got %d: %s", code, output)
	}
	if output, code := state("continue", "--id", "cap", "--item", "x", "--max", "0"); code != 125 {
		t.Errorf("--max 0: want 125, got %d: %s", code, output)
	}
	events := map[string]int{}
	for _, r := range journalRecords(t, readFile(t, filepath.Join(runDir, "journal.jsonl"))) {
		events[r["event"].(string)+"|"+fmt.Sprint(r["item"])]++
	}
	if events["continuation|fix lint on CI"] != 3 || events["blocked|fix lint on CI"] != 1 ||
		events["continuation|rebase on main"] != 1 || events["blocked|rebase on main"] != 1 {
		t.Errorf("journal continuation records wrong: %v", events)
	}

	brief := filepath.Join(tmp, "brief.md")
	if err := os.WriteFile(brief, []byte("Lane brief: satisfy C1 and C2.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, code := state("coverage", "--id", "cap", "--brief", brief); code != 125 || !strings.Contains(output, "fill in") {
		t.Errorf("coverage on unfilled charter: want 125 naming fill in, got %d: %s", code, output)
	}
	charter := strings.Replace(readFile(t, charterPath),
		"## Done criteria\n\n", "## Done criteria\n\n- C1: lint passes | oracle: make lint\n- C2: tests pass | oracle: go test ./...\n- C10: docs updated | oracle: review\n", 1)
	charter = regexp.MustCompile(`(?m)^- C1: \(fill in[^\n]*\n`).ReplaceAllString(charter, "")
	if err := os.WriteFile(charterPath, []byte(charter), 0o600); err != nil {
		t.Fatal(err)
	}
	output, code := state("coverage", "--id", "cap", "--brief", brief)
	if code != 3 || !strings.Contains(output, "missing: C10") || strings.Contains(output, "missing: C1,") || !strings.Contains(output, "(exit 3)") {
		t.Errorf("coverage gap: want 3 naming only C10, got %d: %s", code, output)
	}
	if err := os.WriteFile(brief, []byte("Lane brief: satisfy C1, C2 and C10.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, code := state("coverage", "--id", "cap", "--brief", brief); code != 0 || !strings.Contains(output, "C1, C2, C10") {
		t.Errorf("full coverage: want 0 naming all IDs, got %d: %s", code, output)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func journalRecords(t *testing.T, body string) []map[string]any {
	t.Helper()
	if !strings.HasSuffix(body, "\n") {
		t.Error("journal does not end with a newline")
	}
	var records []map[string]any
	for i, line := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("journal line %d is not JSON: %v: %q", i+1, err, line)
		}
		for _, key := range []string{"ts", "event", "evidence", "head"} {
			if _, ok := record[key]; !ok {
				t.Errorf("journal line %d missing %s", i+1, key)
			}
		}
		records = append(records, record)
	}
	return records
}
