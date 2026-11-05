package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// Doctor reads only the supplied plugin tree and writes its report to stdout.
// os.Root also prevents symlinks in that tree from reaching outside it.
func runDoctor(root, harness string, output io.Writer) error {
	var lines []string
	add := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	add("megapowers doctor")
	if root == "" {
		add("WARN: plugin root unknown: pass --root with the installed plugin directory")
	} else {
		add("plugin root: %s", root)
		tree, err := os.OpenRoot(root)
		if err != nil {
			add("WARN: plugin root unreadable: %v", err)
		} else {
			defer tree.Close()
			doctorTree(tree, add)
		}
	}
	if harness != "" {
		add("harness: %s (explicit)", harness)
	}
	_, err := io.WriteString(output, strings.Join(lines, "\n")+"\n")
	return err
}

func manifestVersion(tree *os.Root, path string) (string, error) {
	data, err := tree.ReadFile(path)
	if err != nil {
		return "", err
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", err
	}
	if manifest.Version == "" {
		return "", fmt.Errorf("no version field")
	}
	return manifest.Version, nil
}

func doctorTree(tree *os.Root, add func(string, ...any)) {
	claude, claudeErr := manifestVersion(tree, ".claude-plugin/plugin.json")
	codex, codexErr := manifestVersion(tree, ".codex-plugin/plugin.json")
	switch {
	case claudeErr == nil:
		add("plugin version: %s (.claude-plugin/plugin.json)", claude)
		if codexErr == nil && codex != claude {
			add("WARN: .codex-plugin/plugin.json version %s differs from %s", codex, claude)
		}
	case codexErr == nil:
		add("plugin version: %s (.codex-plugin/plugin.json)", codex)
	default:
		add("WARN: no readable plugin manifest: %v", claudeErr)
	}
	// The directory edition omits the catalog; its absence is expected.
	data, err := tree.ReadFile("skills/catalog.json")
	if err == nil {
		var catalog struct {
			Skills []struct {
				Name string `json:"name"`
			} `json:"skills"`
		}
		if err := json.Unmarshal(data, &catalog); err != nil {
			add("WARN: skill catalog is not valid JSON: %v", err)
		} else {
			add("skill catalog: %d skills", len(catalog.Skills))
			for _, skill := range catalog.Skills {
				if _, err := tree.ReadFile("skills/" + skill.Name + "/SKILL.md"); err != nil {
					add("WARN: skill %s body unreadable: %v", skill.Name, err)
				}
			}
		}
	} else if !os.IsNotExist(err) {
		add("WARN: skill catalog unreadable: %v", err)
	}
	const template = "skills/orchestrating/assets/agent-capabilities.md"
	if _, err := tree.ReadFile(template); err == nil {
		add("agent registry template: %s", template)
	} else if !os.IsNotExist(err) {
		add("WARN: agent registry template unreadable: %v", err)
	}
}

func main() {
	root := flag.String("root", "", "installed plugin root")
	harness := flag.String("harness", "", "harness: claude or codex (optional label)")
	flag.Parse()
	if *harness != "" && *harness != "claude" && *harness != "codex" {
		fmt.Fprintln(os.Stderr, "doctor: --harness must be claude or codex")
		os.Exit(2)
	}
	if *root == "" {
		for _, key := range []string{"MEGAPOWERS_PLUGIN_ROOT", "PLUGIN_ROOT", "CLAUDE_PLUGIN_ROOT"} {
			if value := os.Getenv(key); value != "" {
				*root = value
				break
			}
		}
	}
	if err := runDoctor(*root, *harness, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "megapowers doctor: cannot write report")
		os.Exit(1)
	}
}
