package maintain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// hookTestDir holds the hook tests. The plugin directory ships to users and
	// is scanned by marketplace review, so test inputs live outside it. The
	// leading underscore hides the directory from ./... patterns: the tests are
	// package main and need the hook sources, which only an overlay provides.
	hookTestDir    = "tests/_hooks"
	hookPackageDir = "plugins/megapowers/hooks"
)

// writeHookTestOverlay writes a go build overlay into dir that places each
// moved hook test at its virtual path inside the hooks package. Absolute
// paths keep it valid whatever the working directory, and the file is
// generated per run, never committed.
func writeHookTestOverlay(root, dir string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	testDir := filepath.Join(absRoot, filepath.FromSlash(hookTestDir))
	entries, err := os.ReadDir(testDir)
	if err != nil {
		return "", fmt.Errorf("hook tests: %w", err)
	}
	replace := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		virtual := filepath.Join(absRoot, filepath.FromSlash(hookPackageDir), name)
		replace[virtual] = filepath.Join(testDir, name)
	}
	if len(replace) == 0 {
		return "", fmt.Errorf("hook tests: no *_test.go files in %s", hookTestDir)
	}
	data, err := json.MarshalIndent(map[string]map[string]string{"Replace": replace}, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "hook-tests-overlay.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// withHookTestOverlay runs fn with the path of a fresh overlay file.
func withHookTestOverlay(root string, fn func(overlay string) int, stderr io.Writer) int {
	dir, err := privateTempDir("megapowers-hook-overlay")
	if err != nil {
		fmt.Fprintf(stderr, "hook tests: %v\n", err)
		return 2
	}
	defer os.RemoveAll(dir)
	overlay, err := writeHookTestOverlay(root, dir)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	return fn(overlay)
}

// runTestHooks runs the hook package tests with the overlay. Arguments are
// go test flags and are placed before the package.
func runTestHooks(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	return withHookTestOverlay(root, func(overlay string) int {
		argv := append([]string{"test", "-overlay", overlay}, args...)
		argv = append(argv, "./"+hookPackageDir)
		if err := command(ctx, root, stdout, stderr, nil, "go", argv...); err != nil {
			return max(exitCode(err), 1)
		}
		return 0
	}, stderr)
}

// checkPluginFolderHoldsNoTests fails when the shipped plugin directory
// contains a test, test data, or an eval case. Those files belong under
// tests/ or evals/ and must not reach plugin installs or marketplace scans.
func checkPluginFolderHoldsNoTests(root string) error {
	pluginRoot := filepath.Join(root, "plugins", "megapowers")
	var found []string
	err := filepath.WalkDir(pluginRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if entry.IsDir() {
			if name := entry.Name(); name == "testdata" || name == "evals" {
				found = append(found, filepath.ToSlash(rel)+"/")
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(found) == 0 {
		return nil
	}
	sort.Strings(found)
	return fmt.Errorf("plugin folder holds files that must live outside it (tests under %s, evals under evals/):\n  %s",
		hookTestDir, strings.Join(found, "\n  "))
}
