package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var slugRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func memAddUsage() {
	fmt.Fprintln(os.Stderr, "usage: mem-add <slug> --title T --hook H [--type decision|constraint|preference|gotcha|reference] [--update]  (body on stdin)")
}

func yamlEscape(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, s)
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func main() {
	os.Exit(runMemAdd(os.Args[1:]))
}

func runMemAdd(args []string) int {
	base := os.Getenv("MEGAPOWERS_MEMORY_DIR")
	if base == "" {
		base = ".megapowers/memory"
	}
	slug, title, hook, typ := "", "", "", "reference"
	update := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		need := func() (string, bool) {
			if i+1 >= len(args) {
				memAddUsage()
				return "", false
			}
			i++
			return args[i], true
		}
		switch a {
		case "--title":
			v, ok := need()
			if !ok {
				return 2
			}
			title = v
		case "--hook":
			v, ok := need()
			if !ok {
				return 2
			}
			hook = v
		case "--type":
			v, ok := need()
			if !ok {
				return 2
			}
			typ = v
		case "--update":
			update = true
		case "-h", "--help":
			memAddUsage()
			return 0
		default:
			if strings.HasPrefix(a, "-") {
				memAddUsage()
				return 2
			}
			slug = a
		}
	}
	if slug == "" || title == "" || hook == "" {
		memAddUsage()
		return 2
	}
	if !slugRE.MatchString(slug) {
		fmt.Fprintln(os.Stderr, "mem-add: slug must be lowercase-kebab, e.g. my-note (a-z 0-9, single hyphens, no leading/trailing/double hyphen)")
		return 2
	}
	switch typ {
	case "decision", "constraint", "preference", "gotcha", "reference":
	default:
		fmt.Fprintf(os.Stderr, "mem-add: bad --type '%s'\n", typ)
		return 2
	}
	title = yamlEscape(title)
	hook = yamlEscape(hook)
	if err := os.MkdirAll(base, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mem-add: %v\n", err)
		return 2
	}
	f := filepath.Join(base, slug+".md")
	if _, err := os.Stat(f); err == nil && !update {
		fmt.Fprintf(os.Stderr, "mem-add: %s already exists — pass --update to overwrite, or pick a new slug.\n", f)
		return 3
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem-add: %v\n", err)
		return 2
	}
	body := strings.TrimRight(string(raw), "\n")
	tmp, err := os.CreateTemp(base, ".mem.")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem-add: %v\n", err)
		return 2
	}
	tmpName := tmp.Name()
	content := fmt.Sprintf("---\nname: %s\ntitle: \"%s\"\nhook: \"%s\"\ntype: %s\n---\n%s\n", slug, title, hook, typ, body)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "mem-add: %v\n", err)
		return 2
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "mem-add: %v\n", err)
		return 2
	}
	if err := os.Rename(tmpName, f); err != nil {
		os.Remove(tmpName)
		fmt.Fprintf(os.Stderr, "mem-add: %v\n", err)
		return 2
	}
	here := os.Getenv("MEGAPOWERS_SCRIPT_DIR")
	if here == "" {
		here = "."
	}
	cmd := exec.Command(filepath.Join(here, "mem-index"))
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mem-add: mem-index: %v\n", err)
		return 2
	}
	fmt.Printf("mem-add: wrote %s (index updated)\n", f)
	return 0
}
