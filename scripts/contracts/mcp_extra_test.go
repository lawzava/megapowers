package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPSetupIsRepositoryExtra(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "plugins/megapowers/skills/mcp-setup")); !os.IsNotExist(err) {
		t.Error("mcp-setup must not exist under shipped skills")
	}
	err := filepath.WalkDir(filepath.Join(root, "plugins/megapowers"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(rel), "mcp-setup") || strings.Contains(strings.ToLower(string(body)), "mcp-setup") {
			t.Errorf("repository extra appears in shipped plugin: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	body := normalized(read(t, root, "extras/skills/mcp-setup/SKILL.md"))
	for _, fact := range requiredSkillFacts["mcp-setup"] {
		if !strings.Contains(body, strings.ToLower(fact)) {
			t.Errorf("extra missing required fact %q", fact)
		}
	}
	for _, text := range forbiddenSkillText["mcp-setup"] {
		requireAbsent(t, body, text, "removed guidance")
	}
}
