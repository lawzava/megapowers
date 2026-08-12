package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	reFetch = regexp.MustCompile(`(?i)(^|[^[:alnum:]_])(curl|wget|fetch)([^[:alnum:]_]|$)`)
	reHTTP  = regexp.MustCompile(`(?i)https?://`)
	reB64   = regexp.MustCompile(`(?i)base64([[:space:]]+[^|]*)?(-d|-di|--decode)[^|]*\|[[:space:]]*(env[[:space:]]+)?([^[:space:]]*/)?(sh|bash|zsh|dash|ksh|python[0-9.]*|node|perl|ruby)([[:space:]]|$)`)
	reEval  = regexp.MustCompile("(?i)eval[^#]*(\\$\\(|`)[^)]*(curl|wget|fetch)")
	reSafe  = regexp.MustCompile(`(?i)ignore (all |the )?(previous|prior) (instruction|message|context)|disregard (all |the )?(previous|prior|the above)|disable (the )?(sandbox|safety|guardrail|security)|bypass (the )?permission|bypass permissions|turn off (the )?(sandbox|safety)`)
	reSkill = regexp.MustCompile(`^plugins/[^/]+/skills/[^/]+/SKILL\.md$`)
)

var bidiRunes = []rune{
	'\u202A', '\u202B', '\u202C', '\u202D', '\u202E',
	'\u2066', '\u2067', '\u2068', '\u2069',
	'\u200E', '\u200F',
	'\u061C',
}

func main() {
	os.Exit(runSecurityLint(os.Args[1:]))
}

func runSecurityLint(args []string) int {
	root := os.Getenv("MEGAPOWERS_ROOT")
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "security-lint: could not determine repo root")
			return 2
		}
		root = wd
	}
	allowFile := filepath.Join(root, "scripts/security-lint.allowlist")
	allow, err := loadAllowlist(allowFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "security-lint: %v\n", err)
		return 2
	}
	for entry := range allow {
		if reSkill.MatchString(entry) {
			fmt.Fprintf(os.Stderr, "security-lint: disallowed allowlist entry: %s (a shipped skill may never be allowlisted; fix the skill, not the allowlist).\n", entry)
			return 1
		}
	}

	var files []string
	if len(args) > 0 {
		files, err = listArgsScope(args)
		if err != nil {
			fmt.Fprintln(os.Stderr, "security-lint: input discovery failed")
			return 2
		}
	} else {
		files, err = listDefaultScope(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, "security-lint: default-scope discovery failed")
			return 2
		}
	}

	seen := map[string]bool{}
	var uniq []string
	for _, f := range files {
		if seen[f] {
			continue
		}
		seen[f] = true
		uniq = append(uniq, f)
	}
	sort.Strings(uniq)

	hits := 0
	type finding struct {
		rel, ln, msg string
	}
	emitAll := func(group []finding) {
		for _, h := range group {
			fmt.Printf("%s:%s: %s\n", h.rel, h.ln, h.msg)
			hits++
		}
	}

	for _, f := range uniq {
		st, err := os.Stat(f)
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		rel := relpath(root, f)
		if allow[rel] {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "security-lint: unreadable input: %s\n", rel)
			return 2
		}
		if len(data) == 0 || !isText(data) {
			continue
		}
		var fetchHits, b64Hits, evalHits, safeHits, bidiHits []finding
		for _, rec := range logicalLines(string(data)) {
			lower := strings.ToLower(rec.text)
			if reFetch.MatchString(lower) && reHTTP.MatchString(lower) {
				fetchHits = append(fetchHits, finding{rel, rec.n, "fetch of remote content in executable context"})
			}
			if reB64.MatchString(lower) {
				b64Hits = append(b64Hits, finding{rel, rec.n, "base64-decoded blob piped into a shell"})
			}
			if reEval.MatchString(lower) {
				evalHits = append(evalHits, finding{rel, rec.n, "eval of fetched remote content"})
			}
			if reSafe.MatchString(lower) {
				safeHits = append(safeHits, finding{rel, rec.n, "instruction to disable a safety mechanism"})
			}
			if hasBidi(rec.text) {
				bidiHits = append(bidiHits, finding{rel, rec.n, "unicode direction-override / bidi control character"})
			}
		}
		emitAll(fetchHits)
		emitAll(b64Hits)
		emitAll(evalHits)
		emitAll(safeHits)
		emitAll(bidiHits)
	}

	if hits > 0 {
		fmt.Fprintf(os.Stderr, "security-lint: %d finding(s)\n", hits)
		return 1
	}
	fmt.Fprintln(os.Stderr, "security-lint: clean")
	return 0
}

func loadAllowlist(path string) (map[string]bool, error) {
	out := map[string]bool{}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line != "" {
			out[line] = true
		}
	}
	return out, sc.Err()
}

func listDefaultScope(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(filepath.Join(root, "plugins"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "SKILL.md" {
			files = append(files, path)
		}
		if strings.Contains(path, string(filepath.Separator)+"hooks"+string(filepath.Separator)) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(filepath.Join(root, "templates"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func listArgsScope(args []string) ([]string, error) {
	var files []string
	for _, p := range args {
		st, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "security-lint: input is missing or unsupported: %s\n", p)
			return nil, err
		}
		if st.IsDir() {
			err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() {
					files = append(files, path)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			continue
		}
		if st.Mode().IsRegular() {
			files = append(files, p)
			continue
		}
		fmt.Fprintf(os.Stderr, "security-lint: input is missing or unsupported: %s\n", p)
		return nil, fmt.Errorf("unsupported")
	}
	return files, nil
}

func relpath(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return rel
}

type rec struct {
	n    string
	text string
}

func logicalLines(s string) []rec {
	raw := strings.Split(s, "\n")
	var out []rec
	var buf string
	start := 0
	for i, line := range raw {
		if start == 0 {
			start = i + 1
		}
		if strings.HasSuffix(line, "\\") {
			buf += strings.TrimSuffix(line, "\\")
			continue
		}
		buf += line
		out = append(out, rec{n: fmt.Sprintf("%d", start), text: buf})
		buf = ""
		start = 0
	}
	if buf != "" {
		out = append(out, rec{n: fmt.Sprintf("%d", start), text: buf})
	}
	return out
}

func isText(data []byte) bool {
	if !utf8.Valid(data) {
		// still scan if mostly text; skip only if NUL present
	}
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}

func hasBidi(s string) bool {
	for _, r := range s {
		for _, b := range bidiRunes {
			if r == b {
				return true
			}
		}
	}
	return false
}
