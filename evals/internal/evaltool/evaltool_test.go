package evaltool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPortableSkillBody(t *testing.T) {
	for _, body := range []string{
		"Use the configured harness.",
		"A codexical example is not a harness name.",
		"Claudean prose is not a harness name.",
	} {
		if hits := portabilityHits(body); len(hits) != 0 {
			t.Errorf("portable body %q matched %v", body, hits)
		}
	}
	for _, body := range []string{"Use Codex.", "Ask claude to review.", "model gpt-5.4", "fork_turns: all"} {
		if hits := portabilityHits(body); len(hits) == 0 {
			t.Errorf("nonportable body %q produced no hits", body)
		}
	}
}

func TestCoverageInventoryDiscoversSkillsDynamically(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"zeta", "alpha"} {
		path := filepath.Join(root, "plugins", "megapowers", "skills", name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	coverage := filepath.Join(root, "evals", "studies", "coverage.tsv")
	if err := os.MkdirAll(filepath.Dir(coverage), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(coverage, []byte("case\talpha\tbehavioral\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := writeCoverageInventory(&out, root); err != nil {
		t.Fatal(err)
	}
	want := "2 shipped skills"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("inventory missing %q:\n%s", want, out.String())
	}
	alpha, zeta := strings.Index(out.String(), "| alpha | 1 | study-declared |"), strings.Index(out.String(), "| zeta | 0 | none |")
	if alpha < 0 || zeta < 0 {
		t.Fatalf("inventory lacks an expected row:\n%s", out.String())
	}
	if alpha > zeta {
		t.Fatalf("skills not sorted:\n%s", out.String())
	}
}

func TestRunAllRejectsInvalidTimeout(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "abc"} {
		if _, err := positiveSeconds(value); err == nil {
			t.Errorf("positiveSeconds(%q) accepted", value)
		}
	}
	if got, err := positiveSeconds("7"); err != nil || got.Seconds() != 7 {
		t.Fatalf("positiveSeconds(7) = %v, %v", got, err)
	}
}

// The installed-ab selftest builds and runs Go fixtures with a cold build
// cache. Measured cold on one CPU it takes about 70 s, and the attest job runs
// it on a shared 4-vCPU runner next to other test packages. A 60 s per-selftest
// limit turned that into a "timeout" harness error on release v0.42.5. The
// limit stays a hard ceiling, but it must leave room for a loaded runner.
func TestDefaultSelftestLimitCoversColdLoadedRunner(t *testing.T) {
	const measuredColdSingleCPU = 70 * time.Second
	if defaultSelftestLimit < 3*measuredColdSingleCPU {
		t.Fatalf("defaultSelftestLimit = %s, want at least %s", defaultSelftestLimit, 3*measuredColdSingleCPU)
	}
}
