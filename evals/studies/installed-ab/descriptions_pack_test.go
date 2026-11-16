//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// The descriptions-only arm keeps every skill description and drops bodies.
// The generated pack must load through the same pinned-pack path as any
// third-party pack and expose exactly the shipped skill names.
func TestDescriptionsOnlyPackLoadsAsPinnedPack(t *testing.T) {
	repo, err := locateRoot("")
	if err != nil {
		t.Fatal(err)
	}
	checkout := t.TempDir()
	dir := filepath.Join(checkout, "skills")
	tool := exec.Command("go", "run", "./evals/tools/descriptions-pack", "--source", filepath.Join(repo, "plugins", "megapowers", "skills"), "--out", dir)
	tool.Dir = repo
	if data, err := tool.CombinedOutput(); err != nil {
		t.Fatalf("descriptions-pack: %v: %s", err, data)
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "https://example.invalid/megapowers-descriptions"}, {"add", "skills"}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "descriptions-only pack"}} {
		cmd := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null", "-C", checkout}, args...)...)
		if data, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, data)
		}
	}
	pack, err := loadSkillPack("megapowers-descriptions="+dir, gitOutput(checkout, "rev-parse", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(publicMegapowersSkillNames[:])
	slices.Sort(want)
	if !reflect.DeepEqual(pack.Identity.Skills, want) {
		t.Fatalf("pack skills = %v, want %v", pack.Identity.Skills, want)
	}
	if err := skillPackConfigSelftest(pack, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, want[0]))
	if err != nil || len(entries) != 1 || entries[0].Name() != "SKILL.md" {
		t.Fatalf("pack skill directory holds more than SKILL.md: %v %v", entries, err)
	}
}
