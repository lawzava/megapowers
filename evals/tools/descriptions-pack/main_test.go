package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		tree[filepath.ToSlash(rel)] = string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

const alphaFrontmatter = "---\nname: alpha\ndescription: \"Use when: testing a pack. Covers alpha.\"\n---\n"

func TestWritesFrontmatterAndOneLineBodyOnly(t *testing.T) {
	source := filepath.Join(t.TempDir(), "skills")
	writeTree(t, source, map[string]string{
		"alpha/SKILL.md":           alphaFrontmatter + "\n# Alpha\n\nLong body.\n",
		"alpha/references/deep.md": "reference\n",
		"alpha/agents/openai.yaml": "policy: explicit\n",
		"beta/SKILL.md":            "---\nname: beta\ndescription: Use for beta.\n---\nBody.\n",
		"beta/assets/card.md":      "asset\n",
		"catalog.json":             "{}\n",
		"index.json":               "{}\n",
	})
	before := readTree(t, source)
	out := filepath.Join(t.TempDir(), "pack", "skills")
	names, err := writePack(source, out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "alpha,beta" {
		t.Fatalf("names = %v", names)
	}
	got := readTree(t, out)
	want := map[string]string{
		"alpha/SKILL.md": alphaFrontmatter + "See the description.\n",
		"beta/SKILL.md":  "---\nname: beta\ndescription: Use for beta.\n---\nSee the description.\n",
	}
	if len(got) != len(want) {
		t.Fatalf("pack files = %v, want only SKILL.md files", got)
	}
	for name, body := range want {
		if got[name] != body {
			t.Errorf("%s = %q, want %q", name, got[name], body)
		}
	}
	after := readTree(t, source)
	if len(after) != len(before) {
		t.Fatal("source tree changed")
	}
	for name, body := range before {
		if after[name] != body {
			t.Fatalf("source file %s changed", name)
		}
	}
}

func TestRefusesUnsafeTargets(t *testing.T) {
	source := filepath.Join(t.TempDir(), "skills")
	writeTree(t, source, map[string]string{"alpha/SKILL.md": alphaFrontmatter + "Body.\n"})
	occupied := t.TempDir()
	writeTree(t, occupied, map[string]string{"keep.txt": "x\n"})
	for name, out := range map[string]string{
		"same as source":   source,
		"inside source":    filepath.Join(source, "pack"),
		"non-empty target": occupied,
		"source inside":    filepath.Dir(source),
	} {
		if _, err := writePack(source, out); err == nil {
			t.Errorf("%s: accepted %s", name, out)
		}
	}
	if _, err := os.Stat(filepath.Join(source, "pack")); !os.IsNotExist(err) {
		t.Fatalf("wrote inside the source tree: %v", err)
	}
}

// A target beside the skills directory is still inside the shipped plugin.
func TestRefusesTargetsInsidePluginRoot(t *testing.T) {
	plugin := filepath.Join(t.TempDir(), "plugins", "megapowers")
	writeTree(t, plugin, map[string]string{
		".claude-plugin/plugin.json": "{}\n",
		"README.md":                  "plugin\n",
		"skills/alpha/SKILL.md":      alphaFrontmatter + "Body.\n",
	})
	source := filepath.Join(plugin, "skills")
	alias := filepath.Join(t.TempDir(), "plugin-alias")
	if err := os.Symlink(plugin, alias); err != nil {
		t.Fatal(err)
	}
	before := readTree(t, plugin)
	for name, out := range map[string]string{
		"sibling of skills":        filepath.Join(plugin, "generated"),
		"nested sibling":           filepath.Join(plugin, "generated", "skills"),
		"plugin manifest folder":   filepath.Join(plugin, ".claude-plugin", "pack"),
		"through a symlink alias":  filepath.Join(alias, "generated"),
		"source through the alias": filepath.Join(alias, "skills", "pack"),
	} {
		if _, err := writePack(source, out); err == nil {
			t.Errorf("%s: accepted %s", name, out)
		}
	}
	if after := readTree(t, plugin); len(after) != len(before) {
		t.Fatalf("plugin tree changed: %v", after)
	}
	// Plain sources without a plugin manifest protect their parent directory.
	loose := filepath.Join(t.TempDir(), "skills")
	writeTree(t, loose, map[string]string{"alpha/SKILL.md": alphaFrontmatter + "Body.\n"})
	if _, err := writePack(loose, filepath.Join(filepath.Dir(loose), "pack")); err == nil {
		t.Error("accepted a sibling of a manifest-less source")
	}
}

func TestRejectsUnreadableSkills(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"no frontmatter":       {"alpha/SKILL.md": "# Alpha\n"},
		"unclosed frontmatter": {"alpha/SKILL.md": "---\nname: alpha\n"},
		"directory without":    {"alpha/notes.md": "x\n"},
		"no skills":            {"catalog.json": "{}\n"},
	} {
		source := filepath.Join(t.TempDir(), "skills")
		writeTree(t, source, files)
		if _, err := writePack(source, filepath.Join(t.TempDir(), "out")); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestShippedSkillsKeepTheirFrontmatter(t *testing.T) {
	source, err := filepath.Abs(filepath.Join("..", "..", "..", "plugins", "megapowers", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	before := readTree(t, source)
	out := filepath.Join(t.TempDir(), "skills")
	names, err := writePack(source, out)
	if err != nil {
		t.Fatal(err)
	}
	got := readTree(t, out)
	if len(names) == 0 || len(got) != len(names) {
		t.Fatalf("names %v, files %d", names, len(got))
	}
	for _, name := range names {
		original := before[name+"/SKILL.md"]
		end := strings.Index(original[4:], "\n---\n")
		if !strings.HasPrefix(original, "---\n") || end < 0 {
			t.Fatalf("%s has no frontmatter", name)
		}
		frontmatter := original[:4+end+5]
		if got[name+"/SKILL.md"] != frontmatter+"See the description.\n" {
			t.Errorf("%s pack copy = %q", name, got[name+"/SKILL.md"])
		}
	}
	after := readTree(t, source)
	for name, body := range before {
		if after[name] != body {
			t.Fatalf("plugin file %s changed", name)
		}
	}
}
