package maintain

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateNeverRunsFreshnessGate pins H2: the freshness check must not be
// part of the canonical PR/CI validate path (scripts/internal/maintain
// validate.go), because docs/harness-support.md ages on a calendar and would
// turn every PR and CI run red regardless of content. Freshness stays
// enforced in the scheduled freshness workflow (.github/workflows/freshness.yml)
// and in the release gate (runRelease's gates list, covered by
// TestReleasePreflight's "fresh" log entry).
func TestValidateNeverRunsFreshnessGate(t *testing.T) {
	repo, log := newValidateFixture(t)
	var stdout, stderr bytes.Buffer
	runValidate(context.Background(), repo, nil, &stdout, &stderr)

	gateLog := string(mustRead(t, log))
	if strings.Contains(gateLog, "freshness-check") {
		t.Fatalf("validate invoked the freshness gate; it belongs only in freshness.yml and the release gate\ngate log:\n%s", gateLog)
	}
	combined := stdout.String() + stderr.String()
	if strings.Contains(strings.ToLower(combined), "freshness") {
		t.Fatalf("validate output mentions freshness; want no freshness step in the PR/CI validate path\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
}

// A skipped test is not a passed test. Inside the Claude sandbox,
// TestRunAllReporting skipped on a socket denial and hid a stale selftest
// count behind "PASS Go tests", so validate names every skipped test.
func TestValidateReportsSkippedGoTests(t *testing.T) {
	repo, _ := newValidateFixture(t)
	t.Setenv("MEGAPOWERS_MAINTAIN_GO_TEST_SKIP", "1")
	var stdout, stderr bytes.Buffer
	runValidate(context.Background(), repo, nil, &stdout, &stderr)
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "PASS Go tests (1 skipped)") || !strings.Contains(combined, "example.com/p TestSocket") {
		t.Fatalf("validate must name skipped tests\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
}

// A failure report shows the failing test's output, not the first lines of
// everything that passed.
func TestValidateShowsOnlyFailingGoTestOutput(t *testing.T) {
	repo, _ := newValidateFixture(t)
	t.Setenv("MEGAPOWERS_MAINTAIN_GO_TEST_FAIL", "1")
	var stdout, stderr bytes.Buffer
	if code := runValidate(context.Background(), repo, nil, &stdout, &stderr); code == 0 {
		t.Fatal("validate passed a failing go test run")
	}
	if !strings.Contains(stderr.String(), "broken_test.go:9: boom") || strings.Contains(stderr.String(), "noisy passing line") {
		t.Fatalf("failure report must hold only failing output\nstderr:\n%s", stderr.String())
	}
}

// An abrupt test exit and a build failure carry their diagnostics in events
// that are not test-level fail events.
func TestValidateKeepsDiagnosticsOfAbruptExitsAndBuildFailures(t *testing.T) {
	repo, _ := newValidateFixture(t)
	t.Setenv("MEGAPOWERS_MAINTAIN_GO_TEST_EXIT", "1")
	var stdout, stderr bytes.Buffer
	if code := runValidate(context.Background(), repo, nil, &stdout, &stderr); code == 0 {
		t.Fatal("validate passed a failing go test run")
	}
	for _, want := range []string{"root cause: exit", "q.go:3:1: syntax error"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("failure report lost %q\nstderr:\n%s", want, stderr.String())
		}
	}
}

func TestValidateCatalogBudgets(t *testing.T) {
	for _, tc := range []struct{ name, description, trigger, body, want string }{
		{"combined", strings.Repeat("x", 1000), strings.Repeat("y", 537), "Body", "1537 characters"},
		{"body", "Use for tests", "", strings.Repeat("x", 20001), "20001 bytes"},
		{"crlf-body", "Use for tests", "", strings.Repeat("x\r\n", 7000), "21000 bytes"},
		{"boundary", strings.Repeat("é", 1000), strings.Repeat("y", 536), strings.Repeat("x", 20000), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := newValidateFixture(t)
			path := filepath.Join(repo, "plugins/megapowers/skills/alpha/SKILL.md")
			if err := os.WriteFile(path, []byte("---\nname: alpha\ndescription: "+tc.description+"\nwhen_to_use: \""+tc.trigger+"\"\n---\n"+tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			code := runValidate(context.Background(), repo, nil, &out, &errOut)
			if tc.want != "" && (code == 0 || !strings.Contains(errOut.String(), tc.want)) {
				t.Fatalf("oversize catalog accepted: code %d\n%s\n%s", code, &out, &errOut)
			}
			if tc.want == "" && (code != 0 || !strings.Contains(out.String(), "total catalog characters: 1541")) {
				t.Fatalf("boundary/count: code %d\n%s\n%s", code, &out, &errOut)
			}
		})
	}
}

func newValidateFixture(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	dirs := []string{
		".claude-plugin",
		".agents/plugins",
		"plugins/megapowers/.claude-plugin",
		"plugins/megapowers/.codex-plugin",
		"plugins/megapowers/skills/alpha",
		"tests",
		"scripts",
		"evals",
		"internal",
	}
	for _, rel := range dirs {
		if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".claude-plugin/marketplace.json", `{"name":"megapowers","plugins":[{"name":"megapowers","source":"./plugins/megapowers"}]}`)
	write(".agents/plugins/marketplace.json", `{"name":"megapowers","plugins":[{"name":"megapowers","source":{"source":"local","path":"./plugins/megapowers"}}]}`)
	manifest := `{"name":"megapowers","version":"1.2.3"}`
	write("plugins/megapowers/.claude-plugin/plugin.json", manifest)
	write("plugins/megapowers/.codex-plugin/plugin.json", manifest)
	write("CHANGELOG.md", "# Changelog\n\n## 1.2.3 - test\n")
	write("plugins/megapowers/.claude-plugin/icon.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128"></svg>`)
	write("plugins/megapowers/skills/alpha/SKILL.md", "---\nname: alpha\ndescription: Use when testing.\n---\n\nBody.\n")
	if err := os.MkdirAll(filepath.Join(repo, "extras"), 0755); err != nil {
		t.Fatal(err)
	}
	write("scripts/dummy.go", "package dummy\n")
	write("scripts/dummy.sh", "#!/bin/sh\necho ok\n")
	if err := os.Chmod(filepath.Join(repo, "scripts/dummy.sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data := mustRead(t, testBinary)
	fakeBin := filepath.Join(t.TempDir(), "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go", "claude"} {
		if err := os.WriteFile(filepath.Join(fakeBin, name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	log := filepath.Join(repo, "gate.log")
	if err := os.WriteFile(log, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "init", "-q")
	gitRun(t, repo, "config", "user.name", "fixture")
	gitRun(t, repo, "config", "user.email", "fixture@example.invalid")
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MEGAPOWERS_MAINTAIN_FAKE", "1")
	t.Setenv("MEGAPOWERS_MAINTAIN_GATE_LOG", log)
	return repo, log
}

// A fresh checkout has no tests/ directory after the hook-only tests are removed.
func TestDiscoverGoFilesWithoutLegacyHookTests(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"scripts", "evals", "internal", "plugins/megapowers", "extras"} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "fixture.go"), []byte("package fixture\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := discoverGoFiles(root)
	if err != nil {
		t.Fatalf("Go discovery must work without legacy hook tests: %v", err)
	}
	if len(files) != 5 {
		t.Fatalf("discovered %d files, want 5", len(files))
	}
}
