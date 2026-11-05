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

	writeFixtureFile(t, clean, "plugins/megapowers/skills/x/SKILL.md", "---\n---\n")
	if err := checkPluginFolderHoldsNoTests(clean); err != nil {
		t.Fatalf("clean plugin folder rejected: %v", err)
	}
	for _, c := range []struct{ rel, offender string }{
		{"plugins/megapowers/skills/x/scripts/runner_test.go", "plugins/megapowers/skills/x/scripts/runner_test.go"},
		{"plugins/megapowers/skills/x/testdata/case.txt", "plugins/megapowers/skills/x/testdata"},
		{"plugins/megapowers/evals/case/case.yaml", "plugins/megapowers/evals"},
	} {
		root := t.TempDir()

		writeFixtureFile(t, root, c.rel, "x\n")
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

func TestPluginFolderRejectsExecutableCode(t *testing.T) {
	for _, name := range []string{"tool.go", "tool.sh", "tool.cmd", "tool.ps1", "tool.py", "tool.js", "tool.ts", "hooks/hook.json", ".mcp.json"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFixtureFile(t, root, "plugins/megapowers/"+name, "{}\n")
			if err := checkPluginFolderHoldsNoTests(root); err == nil {
				t.Fatalf("accepted executable code or integration: %s", name)
			}
		})
	}
}

func TestRepositoryPluginContainsNoExecutableCode(t *testing.T) {
	if err := checkPluginFolderHoldsNoTests("../../.."); err != nil {
		t.Fatal(err)
	}
}

func TestPluginFolderRejectsIntegrationManifestEntries(t *testing.T) {
	for _, key := range []string{"hooks", "mcpServers"} {
		root := t.TempDir()
		writeFixtureFile(t, root, "plugins/megapowers/.claude-plugin/plugin.json", `{"`+key+`":{}}`)
		if err := checkPluginFolderHoldsNoTests(root); err == nil {
			t.Errorf("accepted manifest entry %s", key)
		}
	}
}
