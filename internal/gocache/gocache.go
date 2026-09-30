// Package gocache points go at a writable build cache for repository tools.
package gocache

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Ensure trusts go's own default GOCACHE (normally a writable per-user
// directory) and leaves it alone. It only falls back to a TMPDIR-based cache
// when GOCACHE is unset and go's default is missing or unwritable, instead of
// unconditionally forcing every contributor onto a cold cache.
func Ensure() error {
	if os.Getenv("GOCACHE") != "" {
		return nil
	}
	if out, err := exec.Command("go", "env", "GOCACHE").Output(); err == nil {
		if dir := strings.TrimSpace(string(out)); Writable(dir) {
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

// Writable reports whether dir exists (or can be created) and a file can
// actually be written inside it.
func Writable(dir string) bool {
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
