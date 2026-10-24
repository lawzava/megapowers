package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Lessons are the promotion ledger of the learning loop: a candidate needs
// evidence, a check lesson must catch its past failure, a promotion is read
// back from its target, and review retires lessons that stopped paying off.
func TestLessonsContract(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	tool := filepath.Join(tmp, "lessons")
	if output, code := run(t, root, nil, "go", "build", "-buildvcs=false", "-o", tool, "./plugins/megapowers/skills/writing-agent-instructions/scripts/lessons.go"); code != 0 {
		t.Fatalf("build lessons (%d): %s", code, output)
	}
	gitEnv := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid"}
	repo := filepath.Join(tmp, "repo")
	if err := os.Mkdir(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		output, code := run(t, repo, gitEnv, "git", args...)
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
	git("init", "-q", "-b", "main")
	write(filepath.Join(repo, "wait.sh"), "sleep 30\nmake check\n")
	git("add", "wait.sh")
	git("commit", "-q", "-m", "poll with sleep")
	failRef := git("rev-parse", "HEAD")
	write(filepath.Join(repo, "wait.sh"), "make check\n")
	git("commit", "-q", "-am", "block on the check")
	repo = git("rev-parse", "--show-toplevel")

	store := filepath.Join(tmp, "store")
	lessons := func(args ...string) (string, int) {
		t.Helper()
		return run(t, repo, gitEnv, tool, append([]string{args[0], "--store", store}, args[1:]...)...)
	}
	add := []string{"add", "--id", "no-sleep-poll", "--class", "waiting", "--lesson", "Block on the check instead of sleeping.",
		"--surface", "check", "--target", "scripts/lint-wait", "--evidence", "direct-observation", "--source", "s1.jsonl 2026-10-10T09:02:00Z"}

	if output, code := run(t, repo, gitEnv, tool, append(add, "--store", filepath.Join(repo, ".lessons"))...); code != 125 || !strings.Contains(output, "outside") {
		t.Errorf("store inside the worktree: want 125, got %d: %s", code, output)
	}
	inferred := append([]string{}, add...)
	inferred[len(inferred)-3] = "inferred"
	if output, code := lessons(inferred...); code != 125 || !strings.Contains(output, "evidence") {
		t.Errorf("inferred evidence: want 125, got %d: %s", code, output)
	}
	if output, code := lessons(add...); code != 0 || !strings.Contains(output, "candidate") {
		t.Fatalf("add: want 0 naming candidate, got %d: %s", code, output)
	}
	if output, code := lessons(add...); code != 125 || !strings.Contains(output, "exists") {
		t.Errorf("duplicate add: want 125, got %d: %s", code, output)
	}
	matches, _ := filepath.Glob(filepath.Join(store, "*", "no-sleep-poll.json"))
	if len(matches) != 1 {
		t.Fatalf("lesson file not stored per project: %v", matches)
	}
	if info, err := os.Stat(matches[0]); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("lesson file mode: %v %v", info.Mode(), err)
	}

	if output, code := lessons("promote", "--id", "no-sleep-poll"); code != 3 || !strings.Contains(output, "proof") {
		t.Errorf("promote unproven single-source lesson: want 3, got %d: %s", code, output)
	}
	// A check that cannot fail proves nothing about the past mistake.
	if output, code := lessons("prove", "--id", "no-sleep-poll", "--fail-ref", failRef, "--", "true"); code != 3 || !strings.Contains(output, "passes at") {
		t.Errorf("vacuous check: want 3, got %d: %s", code, output)
	}
	check := []string{"sh", "-c", "! grep -q 'sleep 30' wait.sh"}
	output, code := lessons(append([]string{"prove", "--id", "no-sleep-poll", "--fail-ref", failRef, "--"}, check...)...)
	if code != 0 || !strings.Contains(output, "proven") {
		t.Fatalf("prove: want 0 naming proven, got %d: %s", code, output)
	}
	if strings.Contains(git("worktree", "list"), "lessons-prove") {
		t.Error("prove left a worktree behind")
	}
	if output, code := lessons("promote", "--id", "no-sleep-poll"); code != 0 || !strings.Contains(output, "promoted") {
		t.Fatalf("promote proven check: want 0, got %d: %s", code, output)
	}

	// An instruction lesson is promoted only when the target reads back.
	prose := []string{"add", "--id", "say-unverified", "--class", "verification", "--lesson", "Report NOT VERIFIED when the oracle did not run.",
		"--surface", "instructions", "--target", "AGENTS.md", "--evidence", "direct-statement",
		"--source", "s2.jsonl 2026-10-11T10:00:00Z", "--source", "s3.jsonl 2026-10-12T10:00:00Z"}
	if output, code := lessons(prose...); code != 0 {
		t.Fatalf("add prose: %d %s", code, output)
	}
	if output, code := lessons("promote", "--id", "say-unverified", "--anchor", "Report NOT VERIFIED"); code != 3 || !strings.Contains(output, "AGENTS.md") {
		t.Errorf("promote without target text: want 3, got %d: %s", code, output)
	}
	write(filepath.Join(repo, "AGENTS.md"), "Report NOT VERIFIED when the oracle did not run.\n")
	if output, code := lessons("promote", "--id", "say-unverified", "--anchor", "Report NOT VERIFIED", "--today", "2026-10-12"); code != 0 {
		t.Fatalf("promote prose after read-back: want 0, got %d: %s", code, output)
	}
	output, code = lessons("list")
	for _, want := range []string{"no-sleep-poll", "say-unverified", "promoted"} {
		if code != 0 || !strings.Contains(output, want) {
			t.Errorf("list missing %q (%d): %s", want, code, output)
		}
	}

	card := func(since string, corrections map[string]int) string {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"scope": repo, "since": since, "corrections": corrections})
		path := filepath.Join(tmp, "card-"+since+".json")
		write(path, string(body))
		return path
	}
	// The class recurred after promotion: the lesson is not working.
	output, code = lessons("review", "--scorecard", card("2026-10-13", map[string]int{"verification": 2}), "--today", "2026-10-20")
	if code != 3 || !strings.Contains(output, "say-unverified: recurring") {
		t.Errorf("recurring class: want 3 naming recurring, got %d: %s", code, output)
	}
	// A window that starts before promotion cannot judge the lesson.
	output, code = lessons("review", "--scorecard", card("2026-10-01", map[string]int{"verification": 2}), "--today", "2026-10-20")
	if !strings.Contains(output, "say-unverified: unknown") {
		t.Errorf("early window: want unknown, got %d: %s", code, output)
	}
	// Quiet for longer than max_age_days: candidate for retirement.
	output, code = lessons("review", "--scorecard", card("2026-10-13", map[string]int{}), "--today", "2027-03-01")
	if code != 3 || !strings.Contains(output, "say-unverified: stale") {
		t.Errorf("stale lesson: want 3 naming stale, got %d: %s", code, output)
	}
	other := filepath.Join(tmp, "card-other.json")
	write(other, `{"scope":"/elsewhere","since":"2026-10-13","corrections":{}}`)
	if output, code := lessons("review", "--scorecard", other); code != 125 || !strings.Contains(output, "scope") {
		t.Errorf("scorecard for another project: want 125, got %d: %s", code, output)
	}

	if output, code := lessons("retire", "--id", "say-unverified", "--why", "a check now covers it"); code != 0 {
		t.Fatalf("retire: %d %s", code, output)
	}
	output, code = lessons("review", "--scorecard", card("2026-10-13", map[string]int{"verification": 2}), "--today", "2026-10-20")
	if code != 0 || strings.Contains(output, "say-unverified: recurring") {
		t.Errorf("review must skip retired lessons, got %d: %s", code, output)
	}

	// Export is what may leave the machine: no project path, no sources.
	output, code = lessons("export", "--id", "no-sleep-poll")
	if code != 0 || !strings.Contains(output, "Block on the check") {
		t.Fatalf("export: %d %s", code, output)
	}
	for _, leak := range []string{repo, tmp, "s1.jsonl", failRef} {
		if strings.Contains(output, leak) {
			t.Errorf("export leaks %q: %s", leak, output)
		}
	}
	var exported map[string]any
	if err := json.Unmarshal([]byte(output), &exported); err != nil {
		t.Fatalf("export is not JSON: %v\n%s", err, output)
	}
	if exported["case"] == nil {
		t.Errorf("export must include an installed-ab case draft: %s", output)
	}
}
