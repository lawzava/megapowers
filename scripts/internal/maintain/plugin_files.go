package maintain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// checkPluginFolderHoldsNoTests fails when the shipped plugin directory
// contains executable code, hooks, MCP configuration, tests or eval data.
// Helpers belong in extras; tests and evals stay in repository tooling.
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
			if name := entry.Name(); name == "testdata" || name == "evals" || name == "hooks" {
				found = append(found, filepath.ToSlash(rel)+"/")
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		// Match the passive file types accepted by the probe's package oracle.
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		passive := ext == ".md" || ext == ".json" || ext == ".svg" || ext == ".yaml"
		if !passive || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 != 0 || entry.Name() == ".mcp.json" || entry.Name() == "hooks.json" {
			found = append(found, filepath.ToSlash(rel))
		}
		if entry.Name() == "plugin.json" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var manifest map[string]json.RawMessage
			if err := json.Unmarshal(data, &manifest); err != nil {
				return err
			}
			for _, key := range []string{"hooks", "mcpServers"} {
				if _, exists := manifest[key]; exists {
					found = append(found, filepath.ToSlash(rel)+":"+key)
				}
			}
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
	return fmt.Errorf("plugin folder holds code, integrations or test data that must live outside it:\n  %s", strings.Join(found, "\n  "))
}
