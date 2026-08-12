package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(runMemRecall(os.Args[1:]))
}

func runMemRecall(args []string) int {
	base := os.Getenv("MEGAPOWERS_MEMORY_DIR")
	if base == "" {
		base = ".megapowers/memory"
	}
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: mem-recall <query>")
		return 2
	}
	q := args[0]
	st, err := os.Stat(base)
	if err != nil || !st.IsDir() {
		fmt.Println("(no project memory yet)")
		return 0
	}
	entries, err := filepath.Glob(filepath.Join(base, "*.md"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem-recall: %v\n", err)
		return 2
	}
	ql := strings.ToLower(q)
	matched := false
	for _, f := range entries {
		bn := filepath.Base(f)
		if bn == "INDEX.md" {
			continue
		}
		fm, _ := extractFrontmatter(f)
		title := fmGet(fm, "title")
		hook := fmGet(fm, "hook")
		slug := strings.TrimSuffix(bn, ".md")
		hay := strings.ToLower(title + " " + hook + " " + slug)
		body, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if strings.Contains(hay, ql) || strings.Contains(strings.ToLower(string(body)), ql) {
			fmt.Printf("- %s (%s) — %s\n", title, bn, hook)
			matched = true
		}
	}
	if !matched {
		fmt.Printf("(no memory matches '%s')\n", q)
	}
	return 0
}

func extractFrontmatter(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", false
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return "", false
	}
	rest := text[4:]
	if strings.HasPrefix(text, "---\r\n") {
		rest = text[5:]
	}
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", false
	}
	return rest[:idx+1], true
}

func fmGet(fm, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(fm, "\n") {
		trim := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trim, prefix) {
			continue
		}
		v := strings.TrimSpace(trim[len(prefix):])
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			v = v[1 : len(v)-1]
			v = strings.ReplaceAll(v, `\"`, `"`)
			v = strings.ReplaceAll(v, `\\`, `\`)
		}
		return v
	}
	return ""
}
