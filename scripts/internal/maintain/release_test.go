package maintain

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("MEGAPOWERS_MAINTAIN_FAKE") == "1" {
		os.Exit(runFakeMaintainerProcess())
	}
	os.Exit(m.Run())
}

func runFakeMaintainerProcess() int {
	log := os.Getenv("MEGAPOWERS_MAINTAIN_GATE_LOG")
	appendLog := func(line string) {
		file, err := os.OpenFile(log, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
		if err == nil {
			_, _ = file.WriteString(line + "\n")
			_ = file.Close()
		}
	}
	switch filepath.Base(os.Args[0]) {
	case "validate.sh":
		appendLog("validate")
		if os.Getenv("MEGAPOWERS_MAINTAIN_FAIL_VALIDATE") == "1" {
			return 1
		}
	case "run-all.sh":
		appendLog("deterministic")
		for i := 1; i+1 < len(os.Args); i++ {
			if os.Args[i] == "--json" {
				_ = os.WriteFile(os.Args[i+1], nil, 0o600)
			}
		}
	case "check-freshness.sh":
		appendLog("fresh")
	case "go":
		joined := strings.Join(os.Args[1:], " ")
		if strings.Contains(joined, "installed-ab") {
			appendLog("hash")
		} else if strings.Contains(joined, "score.go") {
			appendLog("score")
			if os.Getenv("MEGAPOWERS_MAINTAIN_FAIL_SCORE") == "1" {
				return 1
			}
		}
	}
	return 0
}

func TestReleasePreflight(t *testing.T) {
	if code := runRelease(context.Background(), t.TempDir(), []string{"v1.2.3"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 2 {
		t.Fatalf("invalid version exit = %d", code)
	}

	t.Run("valid candidate runs gates in order without mutation", func(t *testing.T) {
		repo, log := newReleaseFixture(t)
		before := manifestBytes(t, repo)
		var stdout, stderr bytes.Buffer
		if code := runRelease(context.Background(), repo, []string{"1.2.3"}, &stdout, &stderr); code != 0 {
			t.Fatalf("release failed (%d): %s", code, stderr.String())
		}
		if got := strings.TrimSpace(string(mustRead(t, log))); got != "hash\nvalidate\ndeterministic\nscore\nfresh" {
			t.Fatalf("gate order = %q", got)
		}
		if !bytes.Equal(before, manifestBytes(t, repo)) {
			t.Fatal("release validation mutated manifests")
		}
		if tags, _ := gitOutput(repo, "tag", "--list"); strings.TrimSpace(tags) != "" {
			t.Fatalf("release validation created tag %q", tags)
		}
	})

	tests := []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{"unstamped", func(t *testing.T, repo string) {
			os.WriteFile(filepath.Join(repo, "plugins/megapowers/.claude-plugin/plugin.json"), []byte(`{"name":"megapowers","version":"0.0.0"}`), 0o644)
		}},
		{"non-latest changelog", func(t *testing.T, repo string) {
			os.WriteFile(filepath.Join(repo, "CHANGELOG.md"), []byte("# Changelog\n\n## 1.2.4 - newer\n## 1.2.3 - test\n"), 0o644)
		}},
		{"existing tag", func(t *testing.T, repo string) { gitRun(t, repo, "tag", "v1.2.3") }},
		{"dirty candidate", func(t *testing.T, repo string) {
			os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("dirty\n"), 0o644)
		}},
		{"hidden index", func(t *testing.T, repo string) { gitRun(t, repo, "update-index", "--assume-unchanged", "CHANGELOG.md") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo, log := newReleaseFixture(t)
			test.mutate(t, repo)
			var stdout, stderr bytes.Buffer
			if code := runRelease(context.Background(), repo, []string{"1.2.3"}, &stdout, &stderr); code == 0 {
				t.Fatalf("invalid candidate accepted: %s", stdout.String())
			}
			if data := mustRead(t, log); len(data) != 0 {
				t.Fatalf("gates ran before candidate acceptance: %s", data)
			}
		})
	}
}

func newReleaseFixture(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	for _, rel := range []string{"scripts", "evals", "plugins/megapowers/.claude-plugin", "plugins/megapowers/.codex-plugin"} {
		if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(repo, "CHANGELOG.md"), []byte("# Changelog\n\n## 1.2.3 - test\n"), 0o644)
	manifest := []byte(`{"name":"megapowers","version":"1.2.3"}`)
	os.WriteFile(filepath.Join(repo, "plugins/megapowers/.claude-plugin/plugin.json"), manifest, 0o644)
	os.WriteFile(filepath.Join(repo, "plugins/megapowers/.codex-plugin/plugin.json"), manifest, 0o644)
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data := mustRead(t, testBinary)
	for _, rel := range []string{"scripts/validate.sh", "evals/run-all.sh", "scripts/check-freshness.sh"} {
		if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(rel)), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fakeBin := filepath.Join(t.TempDir(), "bin")
	os.Mkdir(fakeBin, 0o755)
	os.WriteFile(filepath.Join(fakeBin, "go"), data, 0o755)
	log := filepath.Join(repo, "gate.log")
	os.WriteFile(log, nil, 0o600)
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

func manifestBytes(t *testing.T, repo string) []byte {
	t.Helper()
	return mustRead(t, filepath.Join(repo, "plugins/megapowers/.claude-plugin/plugin.json"))
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func gitRun(t *testing.T, repo string, args ...string) {
	t.Helper()
	if output, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func gitOutput(repo string, args ...string) (string, error) {
	output, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
	return string(output), err
}
