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

// Exercise the public command and canonical validator, including stale bytes,
// missing/extra files, and hidden local artifacts that must not be published.
func TestOpenCodeIndexValidation(t *testing.T) {
	repo, _ := newValidateFixture(t)
	writeFixtureFile(t, repo, "plugins/megapowers/skills/alpha/references/a.md", "Reference\n")
	writeFixtureFile(t, repo, "plugins/megapowers/skills/alpha/.claude/local.md", "Local\n")
	var out, errOut bytes.Buffer
	if code := Main(context.Background(), repo, []string{"opencode-index"}, &out, &errOut); code != 0 {
		t.Fatalf("generate: exit %d: %s", code, &errOut)
	}
	index := filepath.Join(repo, "plugins/megapowers/skills/index.json")
	data := mustRead(t, index)
	var got struct {
		Skills []struct {
			Name, Version string
			Files         []string
		}
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Skills) != 1 || got.Skills[0].Name != "alpha" || got.Skills[0].Version != "1.2.3" || strings.Join(got.Skills[0].Files, ",") != "SKILL.md,references/a.md" {
		t.Fatalf("unexpected index: %s", data)
	}
	if code := Main(context.Background(), repo, []string{"opencode-index"}, &out, &errOut); code != 0 || !bytes.Equal(data, mustRead(t, index)) {
		t.Fatalf("regeneration must be deterministic: exit %d: %s", code, &errOut)
	}
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"current", func() {}},
		{"missing", func() { os.Remove(index) }},
		{"stale", func() { os.WriteFile(index, bytes.ReplaceAll(data, []byte("1.2.3"), []byte("1.2.2")), 0600) }},
		{"extra", func() { writeFixtureFile(t, repo, "plugins/megapowers/skills/alpha/extra.md", "Extra") }},
		{"removed", func() { os.Remove(filepath.Join(repo, "plugins/megapowers/skills/alpha/references/a.md")) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(index, data, 0600); err != nil {
				t.Fatal(err)
			}
			tc.change()
			defer os.Remove(filepath.Join(repo, "plugins/megapowers/skills/alpha/extra.md"))
			out.Reset()
			errOut.Reset()
			code := runValidate(context.Background(), repo, nil, &out, &errOut)
			if tc.name == "current" {
				if code != 0 {
					t.Fatalf("current index rejected: %s", &errOut)
				}
			} else if code == 0 || !strings.Contains(errOut.String(), "OpenCode skill index") {
				t.Fatalf("stale index accepted or wrong error: exit %d: %s", code, &errOut)
			}
		})
	}
}

func TestOpenCodeIndexRejectsUnsafeResources(t *testing.T) {
	for _, name := range []string{"references/question?.md", "references/fragment#.md", "references/encoded%2f.md", "references/back\\slash.md", "references/link.md"} {
		t.Run(name, func(t *testing.T) {
			repo, _ := newValidateFixture(t)
			path := filepath.Join(repo, "plugins/megapowers/skills/alpha", name)
			if strings.HasSuffix(name, "link.md") {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("../SKILL.md", path); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFixtureFile(t, repo, "plugins/megapowers/skills/alpha/"+name, "Text")
			}
			var out, errOut bytes.Buffer
			if code := Main(context.Background(), repo, []string{"opencode-index"}, &out, &errOut); code == 0 {
				t.Fatalf("accepted unsafe resource: %s", name)
			}
		})
	}
}

func TestOpenCodeIndexRejectsInvalidSkillDirectories(t *testing.T) {
	for _, name := range []string{"symlink", "bad id", "missing-entrypoint"} {
		t.Run(name, func(t *testing.T) {
			repo, _ := newValidateFixture(t)
			base := filepath.Join(repo, "plugins/megapowers/skills")
			if name == "symlink" {
				if err := os.Symlink("alpha", filepath.Join(base, name)); err != nil {
					t.Fatal(err)
				}
			} else if name == "bad id" {
				writeFixtureFile(t, repo, "plugins/megapowers/skills/bad id/SKILL.md", "Text")
			} else if err := os.Mkdir(filepath.Join(base, name), 0o755); err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			if code := Main(context.Background(), repo, []string{"opencode-index"}, &out, &errOut); code == 0 {
				t.Fatalf("accepted invalid skill directory: %s", name)
			}
		})
	}
}
