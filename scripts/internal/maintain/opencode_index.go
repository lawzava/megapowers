package maintain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const openCodeIndexPath = "plugins/megapowers/skills/index.json"

type openCodeSkill struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Files   []string `json:"files"`
}

// Derive the catalog from files, not the status inventory. A missing inventory
// entry must not hide a shipped skill or its supporting resources.
func buildOpenCodeIndex(root string) ([]byte, error) {
	version, err := manifestVersion(filepath.Join(root, directoryPluginDir, ".claude-plugin/plugin.json"))
	if err != nil {
		return nil, err
	}
	skillsRoot := filepath.Join(root, directoryPluginDir, "skills")
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return nil, err
	}
	index := struct {
		Skills []openCodeSkill `json:"skills"`
	}{Skills: []openCodeSkill{}}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("catalog skill directory is a symlink: %s", entry.Name())
		}
		if !entry.IsDir() {
			continue // root metadata, including catalog.json and index.json
		}
		if !pluginNamePattern.MatchString(entry.Name()) {
			return nil, fmt.Errorf("unsafe skill ID %q", entry.Name())
		}
		skill := openCodeSkill{Name: entry.Name(), Version: version, Files: []string{}}
		base := filepath.Join(skillsRoot, entry.Name())
		err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if url.PathEscape(d.Name()) != d.Name() || strings.ContainsAny(d.Name(), "\\:") {
				return fmt.Errorf("unsafe catalog path %s", p)
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("catalog file is not regular: %s", p)
			}
			rel, err := filepath.Rel(base, p)
			if err == nil {
				skill.Files = append(skill.Files, filepath.ToSlash(rel))
			}
			return err
		})
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(base, "SKILL.md")); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		index.Skills = append(index.Skills, skill)
	}
	data, err := json.MarshalIndent(index, "", "  ")
	return append(data, '\n'), err
}

func checkOpenCodeIndex(root string) error {
	want, err := buildOpenCodeIndex(root)
	if err != nil {
		return err
	}
	got, err := os.ReadFile(filepath.Join(root, openCodeIndexPath))
	if err != nil || !bytes.Equal(got, want) {
		return fmt.Errorf("%s is missing or stale; run go run ./scripts/cmd/maintainer opencode-index", openCodeIndexPath)
	}
	return nil
}

func runOpenCodeIndex(root string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: maintainer opencode-index")
		return 2
	}
	data, err := buildOpenCodeIndex(root)
	if err == nil {
		err = os.WriteFile(filepath.Join(root, openCodeIndexPath), data, 0o644)
	}
	if err != nil {
		fmt.Fprintf(stderr, "opencode-index: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "updated "+openCodeIndexPath)
	return 0
}
