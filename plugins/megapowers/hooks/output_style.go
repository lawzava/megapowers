package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const skillLoadingReminder = `
# Skills

The available-skills catalog lists trigger descriptions, not skill content.
When a task matches a skill's description, load that skill's SKILL.md with
the skills tool or a direct file read before you act on the task, and follow
what it says. Do not claim a skill without loading it.
`

func outputStyleEnabled(getenv func(string) string) bool {
	if getenv("MEGAPOWERS_OUTPUT_STYLE") == "off" {
		return false
	}
	switch getenv("MEGAPOWERS_HARNESS") {
	case "codex":
		return true
	case "claude":
		return false
	default:
		return getenv("PLUGIN_ROOT") != ""
	}
}

func writeOutputStyle(path string, output io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	frontmatter := false
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first && line == "---" {
			frontmatter = true
			first = false
			continue
		}
		first = false
		if frontmatter && line == "---" {
			frontmatter = false
			continue
		}
		if !frontmatter {
			if _, err := fmt.Fprintln(output, line); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func outputStylePath(getenv func(string) string) string {
	return filepath.Join(getenv("MEGAPOWERS_PLUGIN_ROOT"), "output-styles", "megapowers.md")
}
