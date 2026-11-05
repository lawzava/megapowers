package maintain

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixtureFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHookTestOverlayMapsEachMovedTest(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, root, "plugins/megapowers/hooks/runner.go", "package main\n")
	writeFixtureFile(t, root, "tests/_hooks/a_test.go", "package main\n")
	writeFixtureFile(t, root, "tests/_hooks/b_test.go", "package main\n")
	writeFixtureFile(t, root, "tests/_hooks/notes.md", "not a test\n")

	path, err := writeHookTestOverlay(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var overlay struct{ Replace map[string]string }
	if err := json.Unmarshal(mustRead(t, path), &overlay); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for _, name := range []string{"a_test.go", "b_test.go"} {
		want[filepath.Join(root, "plugins", "megapowers", "hooks", name)] = filepath.Join(root, "tests", "_hooks", name)
	}
	if len(overlay.Replace) != len(want) {
		t.Fatalf("overlay = %v, want %v", overlay.Replace, want)
	}
	for virtual, real := range want {
		if overlay.Replace[virtual] != real {
			t.Errorf("overlay[%s] = %q, want %q", virtual, overlay.Replace[virtual], real)
		}
	}
}

func TestHookTestOverlayRequiresMovedTests(t *testing.T) {
	root := t.TempDir()
	if _, err := writeHookTestOverlay(root, t.TempDir()); err == nil {
		t.Fatal("overlay accepted a repository without tests/_hooks")
	}
	writeFixtureFile(t, root, "tests/_hooks/notes.md", "no tests here\n")
	if _, err := writeHookTestOverlay(root, t.TempDir()); err == nil {
		t.Fatal("overlay accepted a tests/_hooks directory without *_test.go files")
	}
}

// The hooks are package main, so their tests need package internals. The
// overlay must compile a test kept outside the plugin folder with them, while
// a plain go test run must ignore the moved tests.
func TestRunTestHooksCompilesMovedTestsWithPackageInternals(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", "module example.com/fixture\n\ngo 1.25.0\n")
	writeFixtureFile(t, root, "plugins/megapowers/hooks/main.go", "package main\n\nfunc helper() int { return 7 }\n\nfunc main() {}\n")
	writeFixtureFile(t, root, "tests/_hooks/helper_test.go", "package main\n\nimport \"testing\"\n\nfunc TestHelper(t *testing.T) {\n\tif helper() != 7 {\n\t\tt.Fatal(\"helper\")\n\t}\n}\n")

	var stdout, stderr bytes.Buffer
	if code := runTestHooks(context.Background(), root, []string{"-count=1", "-v"}, &stdout, &stderr); code != 0 {
		t.Fatalf("test-hooks exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "--- PASS: TestHelper") {
		t.Fatalf("overlay run did not execute the moved test\nstdout:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if err := command(context.Background(), root, &stdout, &stderr, nil, "go", "test", "./...", "-count=1", "-v"); err != nil {
		t.Fatalf("plain go test must ignore tests/_hooks: %v\n%s%s", err, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "TestHelper") {
		t.Fatalf("plain go test ran the moved test\n%s", stdout.String())
	}
}

func TestRunTestHooksPropagatesFailure(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", "module example.com/fixture\n\ngo 1.25.0\n")
	writeFixtureFile(t, root, "plugins/megapowers/hooks/main.go", "package main\n\nfunc main() {}\n")
	writeFixtureFile(t, root, "tests/_hooks/broken_test.go", "package main\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) { t.Fatal(\"boom\") }\n")
	var stdout, stderr bytes.Buffer
	if code := runTestHooks(context.Background(), root, []string{"-count=1"}, &stdout, &stderr); code == 0 {
		t.Fatalf("test-hooks passed a failing test\nstdout:\n%s", stdout.String())
	}
}

func TestPluginFolderMustHoldOnlyShippedFiles(t *testing.T) {
	clean := t.TempDir()
	writeFixtureFile(t, clean, "plugins/megapowers/hooks/runner.go", "package main\n")
	writeFixtureFile(t, clean, "plugins/megapowers/skills/x/SKILL.md", "---\n---\n")
	if err := checkPluginFolderHoldsNoTests(clean); err != nil {
		t.Fatalf("clean plugin folder rejected: %v", err)
	}
	for _, c := range []struct{ rel, offender string }{
		{"plugins/megapowers/hooks/runner_test.go", "plugins/megapowers/hooks/runner_test.go"},
		{"plugins/megapowers/skills/x/testdata/case.txt", "plugins/megapowers/skills/x/testdata"},
		{"plugins/megapowers/evals/case/case.yaml", "plugins/megapowers/evals"},
	} {
		root := t.TempDir()
		writeFixtureFile(t, root, "plugins/megapowers/hooks/runner.go", "package main\n")
		writeFixtureFile(t, root, c.rel, "x\n")
		err := checkPluginFolderHoldsNoTests(root)
		if err == nil {
			t.Errorf("%s accepted in plugin folder", c.rel)
		} else if !strings.Contains(err.Error(), c.offender) {
			t.Errorf("error for %s does not name %s: %v", c.rel, c.offender, err)
		}
	}
}

func TestValidatePassesOverlayToVetAndTests(t *testing.T) {
	repo, log := newValidateFixture(t)
	var stdout, stderr bytes.Buffer
	runValidate(context.Background(), repo, nil, &stdout, &stderr)
	gateLog := string(mustRead(t, log))
	for _, want := range []string{"vet overlay", "gotest overlay"} {
		if !strings.Contains(gateLog, want) {
			t.Errorf("validate did not run %q\ngate log:\n%s\nstderr:\n%s", want, gateLog, stderr.String())
		}
	}
}

func TestValidateFailsWhenPluginFolderHoldsATest(t *testing.T) {
	repo, _ := newValidateFixture(t)
	writeFixtureFile(t, repo, "plugins/megapowers/hooks/stray_test.go", "package main\n")
	var stdout, stderr bytes.Buffer
	if code := runValidate(context.Background(), repo, nil, &stdout, &stderr); code == 0 {
		t.Fatalf("validate passed with a test inside the plugin folder\nstdout:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "stray_test.go") {
		t.Fatalf("failure does not name the stray file\nstderr:\n%s", stderr.String())
	}
}
