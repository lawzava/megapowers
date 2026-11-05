package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewIsGuidanceOnly(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "plugins/megapowers/skills/independent-review")
	entries, err := os.ReadDir(filepath.Join(dir, "scripts"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("independent review must not ship a provider launcher")
	}
	body := normalized(read(t, root, "plugins/megapowers/skills/independent-review/SKILL.md"))
	for _, fact := range []string{"another model", "claude.md", "agents.md", "preferred", "optional", "installed agent clis", "native review", "secrets", "disclosure scope", "round cap"} {
		if !strings.Contains(body, fact) {
			t.Errorf("review guidance missing %q", fact)
		}
	}
}
