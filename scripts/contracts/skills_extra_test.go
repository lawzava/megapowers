package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemovedSkillsAreRepositoryExtras(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"mcp-setup", "megapowers-doctor", "upgrading-megapowers"} {
		if _, err := os.Stat(filepath.Join(root, "plugins/megapowers/skills", name)); !os.IsNotExist(err) {
			t.Errorf("%s must not ship", name)
		}
		body := normalized(read(t, root, "extras/skills/"+name+"/SKILL.md"))
		for _, fact := range requiredSkillFacts[name] {
			requireContains(t, body, strings.ToLower(fact), name)
		}
		for _, text := range forbiddenSkillText[name] {
			requireAbsent(t, body, text, name)
		}
	}
}
