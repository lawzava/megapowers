package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(runMemIndex())
}

func runMemIndex() int {
	base := os.Getenv("MEGAPOWERS_MEMORY_DIR")
	if base == "" {
		base = ".megapowers/memory"
	}
	st, err := os.Stat(base)
	if err != nil || !st.IsDir() {
		fmt.Fprintf(os.Stderr, "mem-index: no memory dir at %s\n", base)
		return 2
	}
	idx := filepath.Join(base, "INDEX.md")
	tmp, err := os.CreateTemp(os.Getenv("TMPDIR"), "memidx.")
	if err != nil {
		tmp, err = os.CreateTemp("", "memidx.")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem-index: %v\n", err)
		return 2
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString("# Project Memory Index\n\n"); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "mem-index: %v\n", err)
		return 2
	}
	entries, err := filepath.Glob(filepath.Join(base, "*.md"))
	if err != nil {
		tmp.Close()
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "mem-index: %v\n", err)
		return 2
	}
	n := 0
	for _, f := range entries {
		bn := filepath.Base(f)
		if bn == "INDEX.md" {
			continue
		}
		fm, ok := extractFrontmatter(f)
		if !ok {
			fmt.Fprintf(os.Stderr, "mem-index: skipping %s (no valid frontmatter)\n", bn)
			continue
		}
		slug := strings.TrimSuffix(bn, ".md")
		title := fmGet(fm, "title")
		if title == "" {
			title = slug
		}
		hook := fmGet(fm, "hook")
		dest := slug + ".md"
		if strings.Contains(slug, " ") {
			dest = "<" + dest + ">"
		}
		if _, err := fmt.Fprintf(tmp, "- [%s](%s) — %s\n", title, dest, hook); err != nil {
			tmp.Close()
			os.Remove(tmpName)
			fmt.Fprintf(os.Stderr, "mem-index: %v\n", err)
			return 2
		}
		n++
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "mem-index: %v\n", err)
		return 2
	}
	if err := os.Rename(tmpName, idx); err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "mem-index: %v\n", err)
		return 2
	}
	fmt.Printf("mem-index: %s rebuilt (%d memories)\n", idx, n)
	return 0
}

func extractFrontmatter(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() || sc.Text() != "---" {
		return "", false
	}
	var b strings.Builder
	closed := false
	for sc.Scan() {
		if sc.Text() == "---" {
			closed = true
			break
		}
		b.WriteString(sc.Text())
		b.WriteByte('\n')
	}
	if !closed {
		return "", false
	}
	return b.String(), true
}

func fmGet(fm, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(fm, "\n") {
		if !strings.HasPrefix(line, prefix) && !strings.HasPrefix(strings.TrimLeft(line, " \t"), prefix) {
			continue
		}
		v := strings.TrimSpace(line[strings.Index(line, ":")+1:])
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			v = v[1 : len(v)-1]
			v = strings.ReplaceAll(v, `\"`, `"`)
			v = strings.ReplaceAll(v, `\\`, `\`)
		}
		return v
	}
	return ""
}
