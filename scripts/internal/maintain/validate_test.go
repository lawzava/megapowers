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

func newValidateFixture(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	dirs := []string{
		".claude-plugin",
		".agents/plugins",
		"plugins/megapowers/.claude-plugin",
		"plugins/megapowers/.codex-plugin",
		"plugins/megapowers/hooks",
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
