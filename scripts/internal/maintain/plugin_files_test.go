package maintain

import (
	"bytes"
	"context"
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

func TestPluginFolderMustHoldOnlyShippedFiles(t *testing.T) {
	clean := t.TempDir()
	writeFixtureFile(t, clean, "plugins/megapowers/skills/index.json", "{\"skills\":[]}\n")
	writeFixtureFile(t, clean, "plugins/megapowers/skills/x/SKILL.md", "---\n---\n")
	if err := checkPluginFolderHoldsNoTests(clean); err != nil {
		t.Fatalf("clean plugin folder rejected: %v", err)
	}
	for _, c := range []struct{ rel, offender string }{
		{"plugins/megapowers/skills/x/scripts/runner.go", "runner.go"},
		{"plugins/megapowers/hooks/hooks.json", "hooks"},
		{"plugins/megapowers/.mcp.json", ".mcp.json"},
		{"plugins/megapowers/.claude-plugin/plugin.json", "plugin.json:mcpServers"},
		{"plugins/megapowers/skills/x/scripts/runner_test.go", "plugins/megapowers/skills/x/scripts/runner_test.go"},
		{"plugins/megapowers/skills/x/testdata/case.txt", "plugins/megapowers/skills/x/testdata"},
		{"plugins/megapowers/evals/case/case.yaml", "plugins/megapowers/evals"},
	} {
		root := t.TempDir()
		body := "x\n"
		if strings.HasSuffix(c.rel, "plugin.json") {
			body = `{"mcpServers":{}}`
		}
		writeFixtureFile(t, root, c.rel, body)
		err := checkPluginFolderHoldsNoTests(root)
		if err == nil {
			t.Errorf("%s accepted in plugin folder", c.rel)
		} else if !strings.Contains(err.Error(), c.offender) {
			t.Errorf("error for %s does not name %s: %v", c.rel, c.offender, err)
		}
	}
}

func TestValidateFailsWhenPluginFolderHoldsATest(t *testing.T) {
	repo, _ := newValidateFixture(t)
	writeFixtureFile(t, repo, "plugins/megapowers/skills/x/scripts/stray_test.go", "package main\n")
	var stdout, stderr bytes.Buffer
	if code := runValidate(context.Background(), repo, nil, &stdout, &stderr); code == 0 {
		t.Fatalf("validate passed with a test inside the plugin folder\nstdout:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "stray_test.go") {
		t.Fatalf("failure does not name the stray file\nstderr:\n%s", stderr.String())
	}
}

func TestPluginFolderRejectsExecutableAndSymlinkMetadata(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		root := t.TempDir()
		rel := "plugins/megapowers/skills/x/SKILL.md"
		writeFixtureFile(t, root, rel, "Text\n")
		path := filepath.Join(root, rel)
		if symlink {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("missing.md", path); err != nil {
				t.Fatal(err)
			}
		} else if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := checkPluginFolderHoldsNoTests(root); err == nil {
			t.Errorf("accepted executable or symlink metadata (symlink=%v)", symlink)
		}
	}
	for _, key := range []string{"hooks", "mcpServers"} {
		root := t.TempDir()
		writeFixtureFile(t, root, "plugins/megapowers/.codex-plugin/plugin.json", `{"`+key+`":{}}`)
		if err := checkPluginFolderHoldsNoTests(root); err == nil {
			t.Errorf("accepted integration manifest entry %s", key)
		}
	}
}
