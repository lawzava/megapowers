package maintain

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Main runs the maintainer command and returns its process exit status.
func Main(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if err := ensureGoCache(); err != nil {
		fmt.Fprintln(stderr, "maintainer: no writable Go cache")
		return 2
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: maintainer <validate|release|install-smoke|codex-install-smoke> [args]")
		return 2
	}
	switch args[0] {
	case "validate":
		return runValidate(ctx, root, args[1:], stdout, stderr)
	case "release":
		return runRelease(ctx, root, args[1:], stdout, stderr)
	case "install-smoke":
		return runInstallSmoke(ctx, root, args[1:], stdout, stderr)
	case "codex-install-smoke":
		return runCodexInstallSmoke(ctx, root, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "maintainer: unknown command: %s\n", args[0])
		return 2
	}
}

// ensureGoCache trusts go's own default GOCACHE (normally a writable
// per-user directory) and leaves it alone. It only falls back to a
// TMPDIR-based cache when GOCACHE is unset and go's default is missing or
// unwritable, instead of unconditionally forcing every contributor onto a
// cold cache.
func ensureGoCache() error {
	if os.Getenv("GOCACHE") != "" {
		return nil
	}
	if out, err := exec.Command("go", "env", "GOCACHE").Output(); err == nil {
		if dir := strings.TrimSpace(string(out)); goCacheWritable(dir) {
			return nil
		}
	}
	base := os.Getenv("TMPDIR")
	if base == "" {
		base = os.TempDir()
	}
	cache := filepath.Join(base, "megapowers-gocache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return err
	}
	return os.Setenv("GOCACHE", cache)
}

// goCacheWritable reports whether dir exists (or can be created) and a file
// can actually be written inside it.
func goCacheWritable(dir string) bool {
	if dir == "" {
		return false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	probe, err := os.CreateTemp(dir, ".gocache-write-check-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return true
}
