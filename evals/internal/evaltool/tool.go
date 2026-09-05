package evaltool

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Main runs one evaluation maintenance command.
func Main(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if err := ensureGoCache(); err != nil {
		fmt.Fprintln(stderr, "evaltool: no writable Go cache")
		return 2
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evaltool <run-all|coverage-inventory|check-portability-boundary> [args]")
		return 2
	}
	switch args[0] {
	case "run-all":
		return runAll(ctx, root, args[1:], stdout, stderr)
	case "coverage-inventory":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: coverage-inventory.sh")
			return 2
		}
		if err := writeCoverageInventory(stdout, root); err != nil {
			fmt.Fprintf(stderr, "coverage-inventory: %v\n", err)
			return 1
		}
		return 0
	case "check-portability-boundary":
		target := root
		if len(args) > 2 {
			fmt.Fprintln(stderr, "usage: check-portability-boundary.sh [root]")
			return 2
		}
		if len(args) == 2 {
			target = args[1]
		}
		return checkPortability(target, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evaltool: unknown command: %s\n", args[0])
		return 2
	}
}

func ensureGoCache() error {
	if os.Getenv("GOCACHE") != "" {
		return nil
	}
	base := os.Getenv("TMPDIR")
	if base == "" {
		base = os.TempDir()
	}
	cache := filepath.Join(base, "megapowers-gocache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return err
	}
	return os.Setenv("GOCACHE", cache)
}

func positiveSeconds(value string) (time.Duration, error) {
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return 0, errors.New("timeout must be a positive integer")
	}
	return time.Duration(seconds) * time.Second, nil
}

var portabilityPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)gpt-[0-9]+[.][0-9]+`),
	regexp.MustCompile(`(?i)claude-[a-z]+-[0-9]`),
	regexp.MustCompile(`(?i)(^|[^[:alnum:]_-])codex([^[:alnum:]_]|$)`),
	regexp.MustCompile(`(?i)(^|[^[:alnum:]_-])claude([^[:alnum:]_]|$)`),
	regexp.MustCompile(`(?i)(^|[^[:alnum:]_-])fork_turns([^[:alnum:]_]|$)`),
}

func portabilityHits(body string) []int {
	var hits []int
	for index, line := range strings.Split(body, "\n") {
		for _, pattern := range portabilityPatterns {
			if pattern.MatchString(line) {
				hits = append(hits, index+1)
				break
			}
		}
	}
	return hits
}

func portabilityHitsFor(rel, body string) []int {
	if rel != "plugins/megapowers/skills/writing-agent-instructions/SKILL.md" {
		return portabilityHits(body)
	}
	lines := strings.Split(body, "\n")
	inFrontmatter := len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"
	for index := range lines {
		if index > 0 && strings.TrimSpace(lines[index]) == "---" {
			inFrontmatter = false
		}
		if inFrontmatter {
			lines[index] = strings.ReplaceAll(lines[index], "CLAUDE.md", "instruction-file")
			lines[index] = strings.ReplaceAll(lines[index], "AGENTS.md", "instruction-file")
		}
	}
	return portabilityHits(strings.Join(lines, "\n"))
}

func checkPortability(root string, stdout, stderr io.Writer) int {
	absolute, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "check-portability-boundary: cannot resolve root: %s\n", root)
		return 2
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "check-portability-boundary: cannot resolve root: %s\n", root)
		return 2
	}
	allowed := "plugins/megapowers/skills/independent-review/SKILL.md"
	var files []string
	err = filepath.WalkDir(filepath.Join(absolute, "plugins"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == "SKILL.md" && strings.Contains(filepath.ToSlash(path), "/skills/") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(stderr, "check-portability-boundary: skill discovery failed under %s/plugins\n", absolute)
		return 2
	}
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Fprintf(stderr, "check-portability-boundary: no skills discovered under %s/plugins\n", absolute)
		return 2
	}
	bad := false
	for _, path := range files {
		rel, _ := filepath.Rel(absolute, path)
		if filepath.ToSlash(rel) == allowed {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "check-portability-boundary: scanner failed for %s\n", filepath.ToSlash(rel))
			return 2
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range portabilityHitsFor(filepath.ToSlash(rel), string(data)) {
			fmt.Fprintf(stdout, "%s:%d:%s\n", path, line, lines[line-1])
			bad = true
		}
	}
	if bad {
		return 1
	}
	return 0
}

func writeCoverageInventory(w io.Writer, root string) error {
	var skills []string
	err := filepath.WalkDir(filepath.Join(root, "plugins"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == "SKILL.md" {
			skills = append(skills, filepath.Base(filepath.Dir(path)))
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(skills)
	unique := skills[:0]
	for _, skill := range skills {
		if len(unique) == 0 || unique[len(unique)-1] != skill {
			unique = append(unique, skill)
		}
	}
	counts := map[string]int{}
	coverage, err := os.Open(filepath.Join(root, "evals/studies/coverage.tsv"))
	if err != nil {
		return err
	}
	defer coverage.Close()
	scanner := bufio.NewScanner(coverage)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) >= 3 && fields[2] == "behavioral" {
			counts[fields[1]]++
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	fmt.Fprintln(w, "# megapowers eval coverage inventory")
	fmt.Fprintf(w, "\n%d shipped skills\n\n", len(unique))
	fmt.Fprintln(w, "Per-skill deterministic contract regressions live in scripts/contracts.")
	fmt.Fprintln(w, "They validate executable contracts and are not behavioral skill evidence.")
	fmt.Fprintln(w, "\n| skill | behavioral studies | behavioral evidence |")
	fmt.Fprintln(w, "|---|---:|---|")
	for _, skill := range unique {
		evidence := "none"
		if counts[skill] > 0 {
			evidence = "study-declared"
		}
		fmt.Fprintf(w, "| %s | %d | %s |\n", skill, counts[skill], evidence)
	}
	return nil
}

type resultRow struct {
	SchemaVersion string            `json:"schema_version"`
	Study         string            `json:"study"`
	EvidenceClass string            `json:"evidence_class"`
	CaseID        string            `json:"case_id"`
	RunID         string            `json:"run_id"`
	BlockID       string            `json:"block_id"`
	Arm           string            `json:"arm"`
	Harness       map[string]string `json:"harness"`
	Source        map[string]string `json:"source"`
	PromptHash    string            `json:"prompt_hash"`
	FixtureHash   string            `json:"fixture_hash"`
	PluginHash    string            `json:"plugin_hash"`
	Status        string            `json:"status"`
	RC            int               `json:"rc"`
	DurationMS    int64             `json:"duration_ms"`
	Verdict       string            `json:"verdict"`
	Metrics       map[string]any    `json:"metrics"`
	Artifacts     map[string]string `json:"artifacts"`
	Environment   map[string]string `json:"environment"`
	Timestamp     string            `json:"timestamp"`
	Phase         string            `json:"phase"`
}

type selftest struct {
	id, target string
	command    []string
}

func runAll(parent context.Context, root string, args []string, stdout, stderr io.Writer) int {
	jsonOut := ""
	limit := 60 * time.Second
	for len(args) > 0 {
		switch args[0] {
		case "--json":
			if len(args) < 2 {
				fmt.Fprintln(stderr, "--json requires a path")
				return 2
			}
			jsonOut = args[1]
			args = args[2:]
		case "--timeout":
			if len(args) < 2 {
				fmt.Fprintln(stderr, "--timeout requires seconds")
				return 2
			}
			var err error
			limit, err = positiveSeconds(args[1])
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			args = args[2:]
		default:
			fmt.Fprintf(stderr, "unknown flag: %s\n", args[0])
			return 2
		}
	}
	pluginHash := hashPluginTree(root)
	revision := gitRevision(root)
	tests := []selftest{
		{"score-go-selftest", "evals/score.go", []string{"go", "run", "./evals/score.go", "--selftest"}},
		{"install-smoke-runner-selftest", "evals/studies/install-smoke/run-smoke.sh", []string{"go", "run", "./scripts/cmd/maintainer", "install-smoke", "--selftest"}},
		{"installed-ab-runner-selftest", "evals/studies/installed-ab/run.go", []string{"go", "run", "./evals/studies/installed-ab", "--selftest"}},
		{"pr-replay-runner-selftest", "evals/studies/pr-replay/replay.go", []string{"go", "run", "./evals/studies/pr-replay", "--selftest"}},
		{"session-observability-selftest", "evals/studies/session-observability/run.go", []string{"go", "run", "./evals/studies/session-observability", "--selftest"}},
		{"trigger-recall-runner-selftest", "evals/studies/trigger-recall/run.go", []string{"go", "run", "./evals/studies/trigger-recall", "--selftest"}},
	}
	var rows []resultRow
	pass, fail, indeterminate, harnessErrors := 0, 0, 0, 0
	var failed []string
	persist := func() error {
		if jsonOut == "" {
			return nil
		}
		var buffer bytes.Buffer
		encoder := json.NewEncoder(&buffer)
		for _, row := range rows {
			if err := encoder.Encode(row); err != nil {
				return err
			}
		}
		return os.WriteFile(jsonOut, buffer.Bytes(), 0o644)
	}
	persistFailed := false
	for _, test := range tests {
		row := executeSelftest(parent, root, test, limit, pluginHash, revision)
		rows = append(rows, row)
		if err := persist(); err != nil {
			persistFailed = true
		}
		switch row.Verdict {
		case "pass":
			pass++
			fmt.Fprintf(stdout, "  \x1b[32mPASS\x1b[0m %s\n", test.id)
		case "fail":
			fail++
			failed = append(failed, test.id)
			fmt.Fprintf(stdout, "  \x1b[31mFAIL\x1b[0m %s\n", test.id)
		case "indeterminate":
			indeterminate++
			failed = append(failed, test.id)
			fmt.Fprintf(stdout, "  \x1b[31mINDET\x1b[0m %s\n", test.id)
		default:
			harnessErrors++
			failed = append(failed, test.id)
			fmt.Fprintf(stdout, "  \x1b[31mHERR\x1b[0m %s\n", test.id)
		}
	}
	strictFailed := true
	if !persistFailed {
		rowsFile := jsonOut
		cleanup := func() {}
		if rowsFile == "" {
			tmp, err := os.CreateTemp(os.TempDir(), "megapowers-eval-rows-*.jsonl")
			if err == nil {
				rowsFile = tmp.Name()
				tmp.Close()
				cleanup = func() { os.Remove(rowsFile) }
				var buffer bytes.Buffer
				encoder := json.NewEncoder(&buffer)
				for _, row := range rows {
					_ = encoder.Encode(row)
				}
				_ = os.WriteFile(rowsFile, buffer.Bytes(), 0o600)
			}
		}
		defer cleanup()
		if rowsFile != "" {
			ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
			strictFailed = runCommand(ctx, root, io.Discard, stderr, "go", "run", "./evals/score.go", "--strict", rowsFile) != nil
			cancel()
		}
	}
	fmt.Fprintf(stdout, "\n== evals: %d passed, %d failed, %d indeterminate, %d harness errors ==\n", pass, fail, indeterminate, harnessErrors)
	if len(failed) > 0 {
		fmt.Fprintf(stdout, "   failed: %s\n", strings.Join(failed, " "))
	}
	if persistFailed || strictFailed || fail != 0 || indeterminate != 0 || harnessErrors != 0 {
		return 1
	}
	return 0
}

func executeSelftest(parent context.Context, root string, test selftest, limit time.Duration, pluginHash, revision string) resultRow {
	ctx, cancel := context.WithTimeout(parent, limit)
	defer cancel()
	var trace bytes.Buffer
	started := time.Now()
	err := runCommand(ctx, root, &trace, &trace, test.command[0], test.command[1:]...)
	rc := processExitCode(err)
	status, verdict := "completed", "fail"
	if err == nil {
		verdict = "pass"
	} else if ctx.Err() != nil {
		status, verdict, rc = "timeout", "harness_error", 124
	} else if rc == 126 || rc == 127 || rc == 125 {
		status, verdict = "harness_error", "harness_error"
	}
	now := time.Now().UTC()
	fixtureHash := hashPath(filepath.Join(root, filepath.FromSlash(test.target)))
	traceHash := hashBytes(trace.Bytes())
	taskSuccess := 0
	if verdict == "pass" {
		taskSuccess = 1
	}
	locale := os.Getenv("LC_ALL")
	if locale == "" {
		locale = os.Getenv("LANG")
	}
	if locale == "" {
		locale = "C"
	}
	return resultRow{
		SchemaVersion: "1", Study: "deterministic-regression/selftest", EvidenceClass: "regression",
		CaseID: test.id, RunID: fmt.Sprintf("%s-%d-%d", test.id, now.UnixMilli(), os.Getpid()), BlockID: test.id, Arm: "regression",
		Harness:     map[string]string{"name": "local", "cli_version": "local", "model": "none", "effort": "none"},
		Source:      map[string]string{"repository": filepath.Base(root), "revision": revision},
		PromptHash:  "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		FixtureHash: fixtureHash, PluginHash: pluginHash, Status: status, RC: rc, DurationMS: time.Since(started).Milliseconds(), Verdict: verdict,
		Metrics: map[string]any{"task_success": taskSuccess}, Artifacts: map[string]string{"trace": traceHash},
		Environment: map[string]string{"os": runtime.GOOS, "arch": runtime.GOARCH, "sandbox": defaultString(os.Getenv("CODEX_SANDBOX"), "unknown"), "locale": strings.ReplaceAll(locale, " ", "_")},
		Timestamp:   now.Format(time.RFC3339), Phase: "selftest",
	}
}

func runCommand(ctx context.Context, root string, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = root, stdout, stderr
	return cmd.Run()
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return 125
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hashPath(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		data = []byte("missing:" + path + "\n")
	}
	return hashBytes(data)
}

func hashPluginTree(root string) string {
	var names []string
	cmd := exec.Command("git", "-C", root, "ls-files", "-co", "--exclude-standard", "--", "plugins")
	if data, err := cmd.Output(); err == nil {
		for _, name := range strings.Split(string(data), "\n") {
			if name != "" {
				names = append(names, name)
			}
		}
	} else {
		_ = filepath.WalkDir(filepath.Join(root, "plugins"), func(path string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				names = append(names, path)
			}
			return nil
		})
	}
	sort.Strings(names)
	var stream bytes.Buffer
	for _, name := range names {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, filepath.FromSlash(name))
		}
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			fmt.Fprintf(&stream, "%s\t%s\n", name, hashPath(path))
		}
	}
	return hashBytes(stream.Bytes())
}

func gitRevision(root string) string {
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	data, err := cmd.Output()
	if err != nil {
		return "unversioned"
	}
	revision := strings.TrimSpace(string(data))
	dirty := exec.Command("git", "-C", root, "diff", "--quiet", "--", "plugins", "evals").Run()
	if dirty != nil {
		revision += "-dirty"
	}
	return revision
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
