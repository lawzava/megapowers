package maintain

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var launcherKeyPattern = regexp.MustCompile(`megapowers-hook-[0-9a-f]+-`)

// runHookHash checks, or with --write rewrites, the runner-source hash that
// run-hook.cmd embeds in its cache key. The key must change whenever a runner
// source changes so installed launchers rebuild instead of reusing a stale binary.
func runHookHash(root string, args []string, stdout, stderr io.Writer) int {
	write := false
	for _, arg := range args {
		switch arg {
		case "--write", "-write":
			write = true
		default:
			fmt.Fprintln(stderr, "usage: maintainer hook-hash [--write]")
			return 2
		}
	}
	dir := filepath.Join(root, "plugins", "megapowers", "hooks")
	want, err := hookSourceHash(dir)
	if err != nil {
		fmt.Fprintf(stderr, "hook-hash: %v\n", err)
		return 2
	}
	path := filepath.Join(dir, "run-hook.cmd")
	launcher, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "hook-hash: %v\n", err)
		return 2
	}
	keys := launcherKeyPattern.FindAllString(string(launcher), -1)
	if len(keys) != 2 {
		fmt.Fprintf(stderr, "hook-hash: run-hook.cmd has %d cache keys, want 2\n", len(keys))
		return 2
	}
	current := "megapowers-hook-" + want + "-"
	stale := keys[0] != current || keys[1] != current
	if !stale {
		fmt.Fprintf(stdout, "hook-hash: run-hook.cmd matches %s\n", want)
		return 0
	}
	if !write {
		fmt.Fprintf(stderr, "hook-hash: run-hook.cmd cache key is stale; want %s\nfix: go run ./scripts/cmd/maintainer hook-hash --write\n", want)
		return 1
	}
	updated := launcherKeyPattern.ReplaceAllLiteralString(string(launcher), current)
	if err := os.WriteFile(path, []byte(updated), 0o755); err != nil {
		fmt.Fprintf(stderr, "hook-hash: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "hook-hash: updated run-hook.cmd to %s\n", want)
	return 0
}

// hookSourceHash hashes every non-test Go source in name order, matching the
// launcher's compile list (hooks tests enforce that the two lists agree).
func hookSourceHash(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(hash, "%s\x00", name)
		hash.Write(content)
		count++
	}
	if count == 0 {
		return "", fmt.Errorf("no runner sources in %s", dir)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))[:16], nil
}
