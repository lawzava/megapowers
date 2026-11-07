package maintain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShippedSkillFrontmatterContract(t *testing.T) {
	for _, tc := range []struct {
		name, fields, code string
	}{
		{"minimal", "name: beta\ndescription: Use when testing.\n", ""},
		{"explicit only", "name: beta\ndescription: Use when testing.\ndisable-model-invocation: true\n", ""},
		{"quoted text", "name: 'beta'\ndescription: \"Use when testing: quotes.\"\n", ""},
		{"quoted null text", "name: beta\ndescription: \"null\"\n", ""},
		{"folded text", "name: beta\ndescription: >\n  Use when\n  testing.\n", ""},
		{"metadata map", "name: beta\ndescription: d\nmetadata:\n  short-description: s\n", "skill_frontmatter_key_unsupported"},
		{"unknown scalar", "name: beta\ndescription: d\ntags: testing\n", "skill_frontmatter_key_unsupported"},
		{"legacy trigger", "name: beta\ndescription: d\nwhen_to_use: testing\n", "skill_frontmatter_key_unsupported"},
		{"name map", "name:\n  nested: beta\ndescription: d\n", "skill_frontmatter_value_wrong_type"},
		{"description list", "name: beta\ndescription:\n  - testing\n", "skill_frontmatter_value_wrong_type"},
		{"policy flow list", "name: beta\ndescription: d\ndisable-model-invocation: [true]\n", "skill_frontmatter_value_wrong_type"},
		{"null value", "name: beta\ndescription: d\ndisable-model-invocation: null\n", "skill_frontmatter_value_wrong_type"},
		{"empty value", "name: beta\ndescription: d\ndisable-model-invocation:\n", "skill_frontmatter_value_wrong_type"},
		{"tilde null", "name: beta\ndescription: d\ndisable-model-invocation: ~\n", "skill_frontmatter_value_wrong_type"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := "---\n" + tc.fields + "---\nbody\n"
			root := t.TempDir()
			file := filepath.Join(root, "plugins/megapowers/skills/beta/SKILL.md")
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := checkCatalogBudgets(root)
			pkg := fixturePackage(t)
			setFile(pkg, "skills/beta/SKILL.md", text)
			report := validateDirectoryPackage(pkg)
			if tc.code == "" {
				if err != nil || len(report.Errors) != 0 {
					t.Fatalf("valid frontmatter: catalog=%v, directory=%v", err, report.Errors)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Errorf("catalog error = %v, want %s", err, tc.code)
			}
			requireCode(t, report, tc.code)
		})
	}
}
