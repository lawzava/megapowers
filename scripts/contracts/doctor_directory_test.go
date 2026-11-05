package contracts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const doctorSkillRel = "plugins/megapowers/skills/megapowers-doctor"

// Simulate a directory-edition plugin root (no hooks/, no output-styles/) and
// check that the skill's links resolve there and that the root two levels up
// holds the files the directory-edition branch reads.
func TestDoctorSkillReferencesResolveWithoutHooks(t *testing.T) {
	root := repoRoot(t)
	plugin := t.TempDir()
	dst := filepath.Join(plugin, "skills", "megapowers-doctor")
	if err := os.CopyFS(dst, os.DirFS(filepath.Join(root, doctorSkillRel))); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(plugin, ".codex-plugin", "plugin.json")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`{"name":"megapowers","version":"0.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := filepath.Clean(filepath.Join(dst, "..", "..")); got != filepath.Clean(plugin) {
		t.Fatalf("plugin root = %s, want %s", got, plugin)
	}
	if _, err := os.Stat(filepath.Join(plugin, ".codex-plugin", "plugin.json")); err != nil {
		t.Fatalf("version file unreachable from plugin root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Fatalf("skill is not listable as a sibling: %v", err)
	}
	for _, rel := range []string{"SKILL.md", "references/directory-edition.md", "references/fixes.md"} {
		body, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		links := regexp.MustCompile(`\]\(([^)#]+)`).FindAllStringSubmatch(string(body), -1)
		for _, m := range links {
			if strings.Contains(m[1], "://") {
				continue
			}
			target := filepath.Join(dst, filepath.Dir(filepath.FromSlash(rel)), filepath.FromSlash(m[1]))
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s links to %s, which does not resolve in a directory-edition root", rel, m[1])
			}
		}
	}
	refs := regexp.MustCompile(`references/[a-z-]+\.md`).FindAllString(read(t, root, doctorSkillRel+"/SKILL.md"), -1)
	if len(refs) < 2 {
		t.Fatalf("expected the skill to name its references, got %v", refs)
	}
	for _, r := range refs {
		if _, err := os.Stat(filepath.Join(dst, r)); err != nil {
			t.Errorf("reference %s missing: %v", r, err)
		}
	}
}
