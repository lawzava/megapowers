package evaltool

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	if strings.Index(out.String(), "| alpha | 1 | study-declared |") > strings.Index(out.String(), "| zeta | 0 | none |") {
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

// TestEnsureGoCacheTrustsAWritableGoDefault pins M4: ensureGoCache must not
// force every contributor with a normal, writable GOCACHE onto a cold cache
// under TMPDIR. It should only fall back when go's own default is unset or
// unwritable.
func TestEnsureGoCacheTrustsAWritableGoDefault(t *testing.T) {
	original, had := os.LookupEnv("GOCACHE")
	if err := os.Unsetenv("GOCACHE"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv("GOCACHE", original)
		} else {
			os.Unsetenv("GOCACHE")
		}
	})
	out, err := exec.Command("go", "env", "GOCACHE").Output()
	if err != nil {
		t.Skipf("cannot determine go's default GOCACHE: %v", err)
	}
	want := strings.TrimSpace(string(out))
	if want == "" || !goCacheWritable(want) {
		t.Skip("go's default GOCACHE is not writable in this environment")
	}
	if err := ensureGoCache(); err != nil {
		t.Fatalf("ensureGoCache: %v", err)
	}
	if got := os.Getenv("GOCACHE"); got != "" {
		t.Fatalf("ensureGoCache overrode a writable default GOCACHE (go's own default is %q): got %q", want, got)
	}
}

func TestGoCacheWritableAcceptsAWritableDirectory(t *testing.T) {
	if !goCacheWritable(t.TempDir()) {
		t.Fatal("expected a writable temp directory to be accepted")
	}
}

func TestGoCacheWritableRejectsAnUnwritableParent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits are not enforced for root")
	}
	parent := t.TempDir()
	locked := filepath.Join(parent, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o700) })
	if goCacheWritable(filepath.Join(locked, "gocache")) {
		t.Fatal("expected an unwritable parent directory to be rejected")
	}
}
