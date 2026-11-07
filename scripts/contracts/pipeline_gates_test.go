package contracts

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPipelineGuidanceContracts(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"independent-review/SKILL.md",
		"independent-review/references/correction-rounds.md",
		"orchestrating/references/review-contract.md",
	} {
		text := normalized(read(t, root, "plugins/megapowers/skills/"+rel))
		for _, fact := range []string{"verdict", "scope.creep", "scope.shrink", "blocker", "major", "nit", "file", "line", "claim", "evidence", "lead computes", "regardless", "nits never fail"} {
			if !strings.Contains(text, fact) {
				t.Errorf("%s missing %q", rel, fact)
			}
		}
	}
	entry := normalized(read(t, root, "plugins/megapowers/skills/independent-review/SKILL.md"))
	for _, fact := range []string{"native subagents", "another model", "guidance only", "secrets", "disclosure scope", "user's global instructions"} {
		if !strings.Contains(entry, fact) {
			t.Errorf("native review boundary missing %q", fact)
		}
	}
	orchestration := normalized(read(t, root, "plugins/megapowers/skills/orchestrating/SKILL.md"))
	for _, fact := range []string{"automatic kickback before review", "references/fences.md", "references/review-contract.md", "fresh agent"} {
		if !strings.Contains(orchestration, fact) {
			t.Errorf("orchestration missing %q", fact)
		}
	}
	planning := normalized(read(t, root, "plugins/megapowers/skills/design-and-plan/references/task-contracts.md"))
	for _, fact := range []string{"out-of-scope", "repo-relative command per acceptance item", "more than two", "overlapping fences", "uncheckable", "merge alone", "no producer", "*", "dir/"} {
		if !strings.Contains(planning, fact) {
			t.Errorf("planning missing %q", fact)
		}
	}
}

// Copy the skill collection, not a flattened SKILL.md fork, into a disposable
// OpenCode source layout. Dependencies remain siblings; no live config is used.
func TestCopiedOpenCodeSkillsResolveResources(t *testing.T) {
	source := filepath.Join(repoRoot(t), "plugins/megapowers/skills")
	dest := filepath.Join(t.TempDir(), ".opencode/skills")
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	links := regexp.MustCompile(`\]\(([^)]+)\)`)
	for _, name := range []string{"orchestrating", "independent-review", "autonomous-run"} {
		err := filepath.WalkDir(filepath.Join(dest, name), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, link := range links.FindAllStringSubmatch(string(body), -1) {
				target := strings.SplitN(link[1], "#", 2)[0]
				if target == "" || strings.Contains(target, "://") {
					continue
				}
				resolved := filepath.Join(filepath.Dir(path), target)
				if _, err := os.Stat(resolved); err != nil {
					t.Errorf("copied link %s -> %s: %v", path, target, err)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, ref := range []string{"orchestrating/references/native-dispatch.md", "orchestrating/references/fences.md", "autonomous-run/references/run-state.md"} {
		if info, err := os.Stat(filepath.Join(dest, ref)); err != nil || !info.Mode().IsRegular() {
			t.Errorf("copied manual procedure %s unavailable: %v", ref, err)
		}
	}
}
