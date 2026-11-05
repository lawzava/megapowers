package maintain

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
		"tests/", strings.Join(found, "\n  "))
}
