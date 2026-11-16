// Command descriptions-pack writes a descriptions-only copy of a skill tree.
// Each skill keeps its SKILL.md frontmatter byte for byte; the body becomes
// the single line "See the description.". References, assets, agents and
// root catalog files are not copied. The installed-ab runner loads the result
// as a pinned --skill-pack arm, which isolates what descriptions alone do.
//
// The source tree is only read. The target must be absent or empty, must not
// contain the source, and must lie outside the plugin root that holds the
// source (the nearest ancestor with .claude-plugin or .codex-plugin, else the
// source's parent). Symlinks in either path are resolved first.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const body = "See the description.\n"

func main() {
	source := flag.String("source", "", "skill tree to read, such as plugins/megapowers/skills")
	out := flag.String("out", "", "absent or empty directory to write the pack into")
	flag.Parse()
	if *source == "" || *out == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: descriptions-pack --source SKILLS_DIR --out EMPTY_DIR")
		os.Exit(2)
	}
	names, err := writePack(*source, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "descriptions-pack:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d descriptions-only skills to %s\n", len(names), *out)
}

// writePack returns the sorted skill directory names it wrote.
func writePack(source, out string) ([]string, error) {
	src, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	if src, err = filepath.EvalSymlinks(src); err != nil {
		return nil, err
	}
	dst, err := resolveTarget(out)
	if err != nil {
		return nil, err
	}
	root := protectedRoot(src)
	if within(dst, root) || within(src, dst) {
		return nil, fmt.Errorf("target %s is inside %s, which holds the source skills, or contains the source", out, root)
	}
	if entries, err := os.ReadDir(dst); err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("target %s is not empty", out)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, err
	}
	frontmatters := map[string][]byte{}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // root catalogs such as catalog.json describe files the pack omits
		}
		name := entry.Name()
		content, err := os.ReadFile(filepath.Join(src, name, "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("skill directory %s: %w", name, err)
		}
		frontmatter, err := frontmatterOf(content)
		if err != nil {
			return nil, fmt.Errorf("%s/SKILL.md: %w", name, err)
		}
		frontmatters[name] = frontmatter
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no skill directories in %s", source)
	}
	sort.Strings(names)
	for _, name := range names {
		dir := filepath.Join(dst, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		content := append(append([]byte(nil), frontmatters[name]...), body...)
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), content, 0o644); err != nil {
			return nil, err
		}
	}
	return names, nil
}

// frontmatterOf returns the opening "---" line through the closing "---" line.
func frontmatterOf(content []byte) ([]byte, error) {
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return nil, errors.New("missing frontmatter")
	}
	end := bytes.Index(content[4:], []byte("\n---\n"))
	if end < 0 {
		return nil, errors.New("unclosed frontmatter")
	}
	return content[:4+end+5], nil
}

// protectedRoot is the nearest ancestor of the resolved source that carries a
// plugin manifest directory, such as plugins/megapowers. Without one it is the
// source's parent, so a sibling of the skills directory is never a target.
func protectedRoot(src string) string {
	for dir := filepath.Dir(src); ; dir = filepath.Dir(dir) {
		for _, manifest := range []string{".claude-plugin", ".codex-plugin"} {
			if info, err := os.Stat(filepath.Join(dir, manifest)); err == nil && info.IsDir() {
				return dir
			}
		}
		if filepath.Dir(dir) == dir {
			return filepath.Dir(src)
		}
	}
}

// resolveTarget makes out absolute and resolves symlinks in its existing part.
func resolveTarget(out string) (string, error) {
	abs, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	existing, rest := abs, ""
	for {
		resolved, err := filepath.EvalSymlinks(existing)
		if err == nil {
			return filepath.Join(resolved, rest), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(existing), rest)
		existing = parent
	}
}

func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
