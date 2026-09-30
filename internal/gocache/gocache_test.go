package gocache

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnsureTrustsAWritableGoDefault pins M4: Ensure must not
// force every contributor with a normal, writable GOCACHE onto a cold cache
// under TMPDIR. It should only fall back when go's own default is unset or
// unwritable.
func TestEnsureTrustsAWritableGoDefault(t *testing.T) {
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
	if want == "" || !Writable(want) {
		t.Skip("go's default GOCACHE is not writable in this environment")
	}
	if err := Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got := os.Getenv("GOCACHE"); got != "" {
		t.Fatalf("Ensure overrode a writable default GOCACHE (go's own default is %q): got %q", want, got)
	}
}

func TestWritableAcceptsAWritableDirectory(t *testing.T) {
	if !Writable(t.TempDir()) {
		t.Fatal("expected a writable temp directory to be accepted")
	}
}

func TestWritableRejectsAnUnwritableParent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits are not enforced for root")
	}
	parent := t.TempDir()
	locked := filepath.Join(parent, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o700) })
	if Writable(filepath.Join(locked, "gocache")) {
		t.Fatal("expected an unwritable parent directory to be rejected")
	}
}
