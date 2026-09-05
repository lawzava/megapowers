package contracts

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func root(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate eval contracts")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func command(t *testing.T, dir string, env []string, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.WaitDelay = 5 * time.Second
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if err == nil {
		return output.String(), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return output.String(), exit.ExitCode()
	}
	t.Fatalf("start %s: %v", name, err)
	return "", 125
}

func requireBrokerBubblewrap(t *testing.T, repo string) {
	t.Helper()
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bubblewrap executable is unavailable")
	}
	output, code := command(t, repo, nil, "bwrap",
		"--die-with-parent", "--new-session", "--unshare-ipc", "--unshare-pid", "--unshare-uts", "--unshare-cgroup-try", "--unshare-net", "--cap-drop", "ALL",
		"--ro-bind", "/", "/", "--", "/usr/bin/true")
	if code == 0 {
		return
	}
	detail := strings.TrimSpace(output)
	if newline := strings.IndexByte(detail, '\n'); newline >= 0 {
		detail = detail[:newline]
	}
	if len(detail) > 240 {
		detail = detail[:240]
	}
	if detail == "" {
		detail = "no diagnostic output"
	}
	t.Skipf("bubblewrap cannot create the broker's required isolated namespaces, including --unshare-net (rc=%d): %s", code, detail)
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCoverageInventory(t *testing.T) {
	repo := root(t)
	output, code := command(t, repo, nil, "go", "run", "./evals/cmd/evaltool", "coverage-inventory")
	if code != 0 {
		t.Fatalf("coverage inventory failed: %s", output)
	}
	var catalog struct {
		Skills []struct{ Name string }
	}
	if err := json.Unmarshal([]byte(read(t, repo, "plugins/megapowers/skills/catalog.json")), &catalog); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, fmt.Sprintf("%d shipped skills", len(catalog.Skills))) {
		t.Errorf("inventory count is not derived from catalog:\n%s", output)
	}
	for _, skill := range catalog.Skills {
		if !strings.Contains(output, "| "+skill.Name+" |") {
			t.Errorf("inventory omits %s", skill.Name)
		}
	}
	if strings.Contains(strings.ToLower(output), "coverage means") || !strings.Contains(output, "not behavioral skill evidence") {
		t.Error("inventory overstates deterministic coverage")
	}
}

func TestPortabilityBoundary(t *testing.T) {
	repo := root(t)
	fixture := t.TempDir()
	plain := filepath.Join(fixture, "plugins/megapowers/skills/plain/SKILL.md")
	independent := filepath.Join(fixture, "plugins/megapowers/skills/independent-review/SKILL.md")
	for _, path := range []string{plain, independent} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(plain, []byte("Use a provider SDK when it fits the stack.\n"), 0o644)
	os.WriteFile(independent, []byte("Review with Claude or Codex explicitly.\n"), 0o644)
	if output, code := command(t, repo, nil, "go", "run", "./evals/cmd/evaltool", "check-portability-boundary", fixture); code != 0 {
		t.Fatalf("portable fixture failed: %s", output)
	}
	for _, forbidden := range []string{"In Codex, use native nested agents.", "Use Claude-specific mechanics.", "Run `claude` directly.", "Set fork_turns to none.", "Pin gpt-5.6-sol."} {
		os.WriteFile(plain, []byte(forbidden+"\n"), 0o644)
		if _, code := command(t, repo, nil, "go", "run", "./evals/cmd/evaltool", "check-portability-boundary", fixture); code == 0 {
			t.Errorf("portability guard accepted %q", forbidden)
		}
	}
	empty := t.TempDir()
	os.Mkdir(filepath.Join(empty, "plugins"), 0o755)
	if _, code := command(t, repo, nil, "go", "run", "./evals/cmd/evaltool", "check-portability-boundary", empty); code == 0 {
		t.Error("portability guard accepted zero skills")
	}
	if output, code := command(t, repo, nil, "go", "run", "./evals/cmd/evaltool", "check-portability-boundary", repo); code != 0 {
		t.Errorf("repository violates portability boundary: %s", output)
	}
}

func TestRunAllReporting(t *testing.T) {
	repo := root(t)
	rows := filepath.Join(t.TempDir(), "results.jsonl")
	output, code := command(t, repo, nil, "go", "run", "./evals/cmd/evaltool", "run-all", "--json", rows)
	if code != 0 {
		t.Fatalf("deterministic suite failed: %s", output)
	}
	data, err := os.ReadFile(rows)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(data))
	ids := map[string]bool{}
	for scanner.Scan() {
		var row map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatalf("invalid result row: %v", err)
		}
		decoded = append(decoded, row)
		if row["schema_version"] != "1" || row["evidence_class"] != "regression" || row["arm"] != "regression" {
			t.Errorf("invalid regression row: %+v", row)
		}
		ids[row["case_id"].(string)] = true
	}
	if len(decoded) != 6 || len(ids) != 6 {
		t.Errorf("run-all emitted %d rows and %d unique ids, want 6", len(decoded), len(ids))
	}
	if !regexp.MustCompile(`== evals: 6 passed, 0 failed, 0 indeterminate, 0 harness errors ==`).MatchString(output) {
		t.Errorf("unexpected summary:\n%s", output)
	}
	if score, code := command(t, repo, nil, "go", "run", "./evals/score.go", "--strict", rows); code != 0 || !strings.Contains(score, "## Deterministic regressions") {
		t.Errorf("strict scorer rejected run-all rows (%d): %s", code, score)
	}
}

func baseBehavioralRow(runID, block, arm string) map[string]any {
	plugin := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	if arm == "control" {
		plugin = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	}
	return map[string]any{
		"schema_version": "1", "study": "humanizing-prose-ab", "evidence_class": "behavioral",
		"case_id": "case_1", "run_id": runID, "block_id": block, "arm": arm,
		"harness":      map[string]any{"name": "claude-code", "cli_version": "2.1.0", "model": "claude-fable-5", "effort": "high"},
		"source":       map[string]any{"repository": "megapowers", "revision": "0123456789abcdef0123456789abcdef01234567"},
		"prompt_hash":  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"fixture_hash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"plugin_hash":  plugin, "status": "completed", "rc": 0, "duration_ms": 1234, "verdict": "pass",
		"metrics":     map[string]any{"task_success": 1, "fact_retention": 1},
		"artifacts":   map[string]any{"response": "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", "manifest": "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", "rows": "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
		"environment": map[string]any{"os": "linux", "arch": "amd64", "sandbox": "workspace-write", "locale": "C.UTF-8"},
		"timestamp":   "2026-08-16T12:00:00Z",
	}
}

func baseActivationRow(runID, caseID, block, verdict string) map[string]any {
	success := 0
	if verdict == "pass" {
		success = 1
	}
	row := baseBehavioralRow(runID, block, "treatment")
	row["study"], row["evidence_class"], row["case_id"], row["verdict"] = "trigger-recall", "activation", caseID, verdict
	row["metrics"] = map[string]any{"activation_success": success}
	row["artifacts"] = map[string]any{"response": "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", "trace": "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}
	return row
}

func writeRows(t *testing.T, rows ...map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rows.jsonl")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			t.Fatal(err)
		}
	}
	file.Close()
	return path
}

func score(t *testing.T, rows string, extra ...string) (string, int) {
	t.Helper()
	repo := root(t)
	args := []string{"run", "./evals/score.go", "--strict"}
	args = append(args, extra...)
	args = append(args, rows)
	return command(t, repo, nil, "go", args...)
}

func TestScoreFailClosed(t *testing.T) {
	repo := root(t)
	empty := writeRows(t)
	if _, code := score(t, empty); code == 0 {
		t.Error("strict scorer accepted empty input")
	}
	malformed := filepath.Join(t.TempDir(), "bad.jsonl")
	os.WriteFile(malformed, []byte("{not json}\n"), 0o600)
	if _, code := score(t, malformed); code == 0 {
		t.Error("strict scorer accepted malformed JSON")
	}
	valid := []map[string]any{baseBehavioralRow("treatment-1", "block-1", "treatment"), baseBehavioralRow("control-1", "block-1", "control")}
	if output, code := score(t, writeRows(t, valid...)); code != 0 || !strings.Contains(output, "mcnemar_p") {
		t.Errorf("strict scorer rejected valid pair (%d): %s", code, output)
	}
	tests := []struct {
		name   string
		mutate func([]map[string]any)
		want   string
	}{
		{"duplicate run", func(rows []map[string]any) { rows[1]["run_id"] = rows[0]["run_id"] }, "duplicate"},
		{"mixed revision", func(rows []map[string]any) {
			rows[1]["source"].(map[string]any)["revision"] = "fedcba9876543210fedcba9876543210fedcba98"
		}, "provenance"},
		{"missing metric", func(rows []map[string]any) { delete(rows[1]["metrics"].(map[string]any), "fact_retention") }, "metric"},
		{"indeterminate", func(rows []map[string]any) { rows[1]["verdict"] = "indeterminate" }, "indeterminate"},
		{"timeout", func(rows []map[string]any) { rows[1]["status"], rows[1]["rc"] = "timeout", 124 }, "timed-out"},
		{"unknown field", func(rows []map[string]any) { rows[1]["surprise"] = true }, "unknown"},
		{"missing rc", func(rows []map[string]any) { delete(rows[1], "rc") }, "required field \"rc\""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := []map[string]any{baseBehavioralRow("treatment-1", "block-1", "treatment"), baseBehavioralRow("control-1", "block-1", "control")}
			test.mutate(rows)
			output, code := score(t, writeRows(t, rows...))
			if code == 0 || !strings.Contains(strings.ToLower(output), strings.ToLower(test.want)) {
				t.Errorf("invalid rows accepted or wrong error (%d): %s", code, output)
			}
		})
	}
	_ = repo
}

func TestScoreActivation(t *testing.T) {
	valid := []map[string]any{
		baseActivationRow("a-1", "debug-paraphrase", "rep-1", "pass"),
		baseActivationRow("a-2", "debug-paraphrase", "rep-2", "fail"),
		baseActivationRow("a-3", "prose-adjacent", "rep-1", "pass"),
		baseActivationRow("a-4", "prose-adjacent", "rep-2", "pass"),
	}
	if output, code := score(t, writeRows(t, valid...)); code != 0 || !strings.Contains(output, "Activation evidence") || strings.Contains(output, "mcnemar_p") {
		t.Errorf("activation scoring changed (%d): %s", code, output)
	}
	tests := []struct {
		name string
		row  map[string]any
		want string
	}{
		{"control arm", func() map[string]any {
			r := baseActivationRow("b", "debug", "rep-1", "pass")
			r["arm"] = "control"
			return r
		}(), "treatment"},
		{"missing metric", func() map[string]any {
			r := baseActivationRow("c", "debug", "rep-1", "pass")
			r["metrics"] = map[string]any{"other": 1}
			return r
		}(), "activation_success"},
		{"metric mismatch", func() map[string]any {
			r := baseActivationRow("d", "debug", "rep-1", "fail")
			r["metrics"].(map[string]any)["activation_success"] = 1
			return r
		}(), "match"},
		{"sentinel revision", func() map[string]any {
			r := baseActivationRow("e", "debug", "rep-1", "pass")
			r["source"].(map[string]any)["revision"] = "main"
			return r
		}(), "full git commit hash"},
	}
	for _, test := range tests {
		output, code := score(t, writeRows(t, test.row))
		if code == 0 || !strings.Contains(strings.ToLower(output), strings.ToLower(test.want)) {
			t.Errorf("%s accepted or wrong error (%d): %s", test.name, code, output)
		}
	}
}

func TestSubprocessBounds(t *testing.T) {
	repo := root(t)
	for _, rel := range []string{"evals/studies/pr-replay/replay.go", "evals/studies/installed-ab/run.go", "evals/studies/trigger-recall/run.go"} {
		body := read(t, repo, rel)
		for _, marker := range []string{"WaitDelay", "10485760"} {
			if !strings.Contains(body, marker) {
				t.Errorf("%s lacks bounded subprocess marker %s", rel, marker)
			}
		}
	}
}

func TestStudyRunnerContracts(t *testing.T) {
	repo := root(t)
	runners := []struct {
		name, path, pass string
		config           []string
	}{
		{"installed-ab", "./evals/studies/installed-ab", "installed-ab selftest: PASS", []string{"--validate-config", "--cases", "evals/studies/installed-ab/cases.json", "--gates", "evals/studies/installed-ab/gates.json"}},
		{"pr-replay", "./evals/studies/pr-replay", "pr-replay selftest: PASS", []string{"--validate-config", "--cases", "evals/studies/pr-replay/cases.json"}},
		{"session-observability", "./evals/studies/session-observability", "session-observability selftest: PASS", nil},
		{"trigger-recall", "./evals/studies/trigger-recall", "trigger-recall selftest: PASS", []string{"--validate-config", "--cases", "evals/studies/trigger-recall/cases.json", "--gates", "evals/studies/trigger-recall/gates.json"}},
	}
	for _, runner := range runners {
		t.Run(runner.name, func(t *testing.T) {
			output, code := command(t, repo, []string{"TMPDIR=" + t.TempDir()}, "go", "run", runner.path, "--selftest")
			if code != 0 || !strings.Contains(output, runner.pass) {
				t.Fatalf("selftest failed (%d): %s", code, output)
			}
			if runner.config != nil {
				args := append([]string{"run", runner.path}, runner.config...)
				if output, code := command(t, repo, nil, "go", args...); code != 0 {
					t.Errorf("config rejected (%d): %s", code, output)
				}
			}
		})
	}
}

func TestBrokerContract(t *testing.T) {
	repo := root(t)
	source := read(t, repo, "evals/tools/sandbox-broker/main.go")
	for _, forbidden := range []string{"--dangerously-skip-permissions", "--dangerously-bypass-approvals-and-sandbox", `"--bare"`, `"ephemeral": true`} {
		if strings.Contains(source, forbidden) {
			t.Errorf("broker contains forbidden marker %s", forbidden)
		}
	}
	for _, required := range []string{`"defaultMode": "acceptEdits"`, `"Agent", "Task", "Skill"`, `"skills_catalog,omitempty"`} {
		if !strings.Contains(source, required) {
			t.Errorf("broker lacks %s", required)
		}
	}
	requireBrokerBubblewrap(t, repo)
	output, code := command(t, repo, []string{"TMPDIR=" + t.TempDir()}, "go", "run", "./evals/tools/sandbox-broker", "--selftest")
	if code != 0 || !strings.Contains(output, "sandbox broker selftest: PASS") {
		t.Fatalf("broker selftest failed (%d): %s", code, output)
	}
}

func TestTriggerRecallCorpusPolicy(t *testing.T) {
	repo := root(t)
	var catalog struct{ Skills []struct{ Name string } }
	json.Unmarshal([]byte(read(t, repo, "plugins/megapowers/skills/catalog.json")), &catalog)
	var cases struct {
		Cases []struct {
			ID, Kind, Expected, Provenance string
		} `json:"cases"`
	}
	if err := json.Unmarshal([]byte(read(t, repo, "evals/studies/trigger-recall/cases.json")), &cases); err != nil {
		t.Fatal(err)
	}
	counts, ids, noSkill := map[string]int{}, map[string]bool{}, 0
	for _, c := range cases.Cases {
		if ids[c.ID] || c.Provenance == "" {
			t.Errorf("duplicate id or missing provenance: %s", c.ID)
		}
		ids[c.ID] = true
		if c.Kind == "no-skill" {
			noSkill++
			if c.Expected != "" {
				t.Errorf("no-skill %s declares expected %s", c.ID, c.Expected)
			}
		} else if c.Expected != "" {
			counts[c.Expected]++
		}
	}
	if noSkill < 10 {
		t.Errorf("no-skill pool has %d probes", noSkill)
	}
	for _, skill := range catalog.Skills {
		frontmatter := read(t, repo, "plugins/megapowers/skills/"+skill.Name+"/SKILL.md")
		disabled := strings.Contains(strings.SplitN(frontmatter, "---", 3)[1], "disable-model-invocation: true")
		if disabled && counts[skill.Name] != 1 {
			t.Errorf("explicit-only skill %s has %d expected probes, want its single explicit probe", skill.Name, counts[skill.Name])
		}
		if !disabled && counts[skill.Name] < 3 {
			t.Errorf("selectable skill %s has %d probes", skill.Name, counts[skill.Name])
		}
	}
	var gates struct {
		Mode             string   `json:"mode"`
		EnforceHarnesses []string `json:"enforce_harnesses"`
		Acceptance       struct {
			PerSkill            map[string]any `json:"per_skill"`
			MaxMedianFinalWords int            `json:"max_median_final_words"`
			MaxEmDashRate       float64        `json:"max_em_dash_rate"`
		} `json:"acceptance"`
	}
	if err := json.Unmarshal([]byte(read(t, repo, "evals/studies/trigger-recall/gates.json")), &gates); err != nil {
		t.Fatal(err)
	}
	if gates.Mode != "enforce" || strings.Join(gates.EnforceHarnesses, ",") != "claude" || len(gates.Acceptance.PerSkill) != 0 || gates.Acceptance.MaxMedianFinalWords != 120 || gates.Acceptance.MaxEmDashRate != 0.1 {
		t.Errorf("unexpected trigger policy: %+v", gates)
	}
}

func TestRunnerSourcesExcludeCredentialCopying(t *testing.T) {
	repo := root(t)
	for _, rel := range []string{"evals/studies/installed-ab/run.go", "evals/studies/pr-replay/replay.go", "evals/studies/trigger-recall/run.go", "scripts/internal/maintain/install_smoke.go"} {
		body := read(t, repo, rel)
		for _, forbidden := range []string{".credentials.json", "auth.json", "copyCredential", "dangerously-skip-permissions"} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s contains %s", rel, forbidden)
			}
		}
	}
}

func TestEvalShellLaunchersContainNoPolicy(t *testing.T) {
	repo := root(t)
	paths := []string{"evals/run-all.sh", "evals/coverage-inventory.sh", "evals/check-portability-boundary.sh", "evals/studies/install-smoke/run-smoke.sh"}
	sort.Strings(paths)
	for _, rel := range paths {
		body := read(t, repo, rel)
		if len(strings.Split(strings.TrimSpace(body), "\n")) > 14 || !strings.Contains(body, "go run") {
			t.Errorf("%s is not a thin Go launcher", rel)
		}
	}
}
