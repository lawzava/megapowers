package contracts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const doctorSkillRel = "plugins/megapowers/skills/megapowers-doctor"

// The directory edition ships no hooks/ directory, so the doctor must branch
// on the runner file instead of calling a path that does not exist.
func TestDoctorSkillHasDirectoryEditionBranch(t *testing.T) {
	root := repoRoot(t)
	skill := read(t, root, doctorSkillRel+"/SKILL.md")
	text := normalized(skill)
	for _, fact := range []string{
		"hooks/run-hook.cmd` is missing",
		"directory edition",
		"references/directory-edition.md",
	} {
		if !strings.Contains(text, strings.ToLower(fact)) {
			t.Errorf("doctor SKILL.md missing %q", fact)
		}
	}
	// The runner command stays the unchanged step for the full plugin.
	if !strings.Contains(skill, `"<plugin root>/hooks/run-hook.cmd" doctor`) {
		t.Error("full-plugin runner command no longer appears")
	}
	// The branch must come before the runner command.
	if strings.Index(skill, "is missing") > strings.Index(skill, `run-hook.cmd" doctor`) {
		t.Error("missing-runner branch must precede the runner command")
	}

	ref := normalized(read(t, root, doctorSkillRel+"/references/directory-edition.md"))
	for _, fact := range []string{
		"directory edition",
		"no hooks, no output style, no destructive-command guard, no startup reminder",
		"not as a fault",
		".codex-plugin/plugin.json",
		".claude-plugin/plugin.json",
		"sibling skills",
		"skill.md",
		"only if the harness has a shell",
		"never claim a check ran that did not",
		"docs/install.md",
		"https://github.com/lawzava/megapowers",
		"claude code and codex",
	} {
		if !strings.Contains(ref, fact) {
			t.Errorf("directory-edition.md missing %q", fact)
		}
	}
	fixes := normalized(read(t, root, doctorSkillRel+"/references/fixes.md"))
	if !strings.Contains(fixes, "directory edition") {
		t.Error("fixes.md should name the directory-edition result")
	}
}

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
	if _, err := os.Stat(filepath.Join(plugin, "hooks", "run-hook.cmd")); err == nil {
		t.Fatal("fixture must have no runner")
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
