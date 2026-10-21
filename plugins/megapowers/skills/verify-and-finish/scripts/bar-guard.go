//go:build ignore

// Command bar-guard reports diff-scoped changes that weaken verification.
package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const usage = `Usage: go run bar-guard.go [--base REF | --diff FILE] [--json]

Checks staged, unstaged, and untracked changes from the work tree's root.
--base REF    compare with the merge base of REF and HEAD (default HEAD);
              --base origin/main uses the merge base with origin/main
--diff FILE   read a unified diff instead; no work tree is required
--json        emit an array of findings instead of lines and a summary
--help        print this help

Findings include skipped/focused tests, deleted tests, a net loss of assertion
lines, suppressions, empty error handlers, and loosened numeric thresholds.
Thresholds pair unique, identical scalar assignment keys within one hunk,
with matching units: lower threshold/coverage/min, higher max warnings/issues,
limit/timeout. Unparseable numeric scalars are reported as unverifiable.
Expressions and ambiguous duplicate keys are not compared.
A same-line bar-guard:allow <reason> marks a finding waived but still prints it.
Exit status: 0 clean or all waived, 1 findings, 2 could not run.
Go run maps nonzero program exits to 1; build the file to preserve exit 2.
`

type finding struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	Waived bool   `json:"waived"`
	Reason string `json:"reason,omitempty"`
}

type sourceLine struct {
	number int
	text   string
}

type hunk struct{ added, removed, before, after []sourceLine }

type fileChange struct {
	path, oldPath string
	deleted       bool
	hunks         []*hunk
	before, after []sourceLine
	metadata      bool
	textHeaders   bool
	oldMode       bool
	renameFrom    bool
	copyFrom      bool
	index         string
	emptyFile     bool
	binary        bool
	contentChange bool
}

var (
	skipPattern         = regexp.MustCompile(`\bt\.Skip(f|Now)?\s*\(|\.(skip|only)\s*\(|\bit\.todo\s*\(|\b(xit|xdescribe)\s*\(|\bpytest\.mark\.(skip|skipif|xfail)\b|@unittest\.skip\b|\bpytest\.skip\s*\(|#\[\s*ignore\s*\]`)
	suppressionPattern  = regexp.MustCompile(`\bnolint\b|//\s*lint:ignore\b|#\s*nosec\b|\bnosemgrep\b|eslint-disable|@SuppressWarnings\b|#\s*pylint:\s*disable\b|#\s*pyright:\s*ignore\b|@ts-ignore\b|@ts-expect-error\b|#\s*noqa\b|#\s*type:\s*ignore\b|gitleaks:allow|#\[\s*allow\s*\(|//\s*@ts-nocheck\b|istanbul ignore|pragma:\s*no cover`)
	emptyPattern        = regexp.MustCompile(`\bcatch\s*(\([^)]*\)\s*)?\{\s*\}|\bexcept\b[^:]*:\s*pass\s*(#.*)?$|\bif\s+err\s*!=\s*nil\s*\{\s*\}`)
	handlerPattern      = regexp.MustCompile(`\bcatch\s*(\([^)]*\)\s*)?\{\s*(?://.*)?$|^\s*except\b[^:]*:\s*(?:#.*)?$|^\s*if\s+err\s*!=\s*nil\s*\{\s*(?://.*)?$`)
	assertionPattern    = regexp.MustCompile(`\bassert\w*|\bexpect\s*\(|\brequire\.|\bt\.(Error|Fatal|Fail)\w*\s*\(|\bshould\w*`)
	testPattern         = regexp.MustCompile(`^\s*func\s+(Test\w*)\s*\(\s*\w+\s+\*testing\.T\s*\)|^\s*(?:async\s+)?def\s+(test_\w+)\s*\(|^\s*(?:it|test)\s*\(\s*['"\x60]([^'"\x60]+)['"\x60]\s*,`)
	rustTestPattern     = regexp.MustCompile(`#\[\s*(?:cfg\s*\(\s*test\s*\)|test)\s*\]`)
	rustFunctionPattern = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?fn\s+(\w+)\s*\(`)
	waiverPattern       = regexp.MustCompile(`\bbar-guard:allow[ \t]+(\S.*)`)
	hunkPattern         = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@.*$`)
	assignmentPattern   = regexp.MustCompile(`^\s*(?:-\s+)?["']?([A-Za-z_][A-Za-z0-9_.-]*)["']?\s*[:=]\s*["']?([+-]?(?:\d+(?:\.\d+)?|\.\d+))([A-Za-z%]*)["']?\s*[,;]?\s*(?:(?://|#).*)?$`)
	minimumPattern      = regexp.MustCompile(`(?i)threshold|coverage|min`)
	maximumPattern      = regexp.MustCompile(`(?i)max_?(warnings|issues)|limit|timeout`)
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("bar-guard", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	base := flags.String("base", "HEAD", "base ref")
	diff := flags.String("diff", "", "unified diff file")
	asJSON := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		return fail(stderr, err)
	}
	baseSet, diffSet := false, false
	flags.Visit(func(f *flag.Flag) {
		baseSet = baseSet || f.Name == "base"
		diffSet = diffSet || f.Name == "diff"
	})
	if len(flags.Args()) != 0 || baseSet && diffSet || *base == "" || diffSet && *diff == "" {
		return fail(stderr, errors.New("use --base REF or --diff FILE, without positional arguments"))
	}

	var views [][]*fileChange
	var err error
	if diffSet {
		var data []byte
		data, err = os.ReadFile(*diff)
		if err == nil {
			var files []*fileChange
			files, err = parseDiff(data)
			views = append(views, files)
		}
	} else {
		views, err = workingChanges(*base)
	}
	if err != nil {
		return fail(stderr, err)
	}
	findings := make([]finding, 0)
	seen := map[finding]bool{}
	for _, files := range views {
		for _, f := range inspect(files) {
			if !seen[f] {
				findings = append(findings, f)
				seen[f] = true
			}
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Text < b.Text
	})
	unwaived, waived := 0, 0
	for _, f := range findings {
		if f.Waived {
			waived++
		} else {
			unwaived++
		}
	}
	if *asJSON {
		err = json.NewEncoder(stdout).Encode(findings)
	} else {
		var output strings.Builder
		for _, f := range findings {
			kind, name := f.Kind, f.Path
			if f.Waived {
				kind += " (waived)"
			}
			if strings.ContainsAny(name, "\r\n\t") {
				name = strconv.Quote(name)
			}
			fmt.Fprintf(&output, "%s:%d: %s: %s\n", name, f.Line, kind, f.Text)
		}
		fmt.Fprintf(&output, "bar-guard: %d findings (%d waived)\n", unwaived, waived)
		_, err = io.WriteString(stdout, output.String())
	}
	if err != nil {
		return fail(stderr, err)
	}
	if unwaived > 0 {
		return 1
	}
	return 0
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "bar-guard: %v\n", err)
	return 2
}

func gitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"--no-pager"}, args...)...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func workingChanges(base string) ([][]*fileChange, error) {
	top, err := gitOutput("", "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not inside a git work tree: %w", err)
	}
	root := strings.TrimRight(string(top), "\r\n")
	ref, err := gitOutput(root, "rev-parse", "--verify", "--end-of-options", base+"^{commit}")
	if err != nil {
		return nil, err
	}
	ancestor, err := gitOutput(root, "merge-base", strings.TrimSpace(string(ref)), "HEAD")
	if err != nil {
		return nil, err
	}
	var views [][]*fileChange
	for _, cached := range []bool{false, true} {
		args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-renames", "--no-relative", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
		if cached {
			args = append(args, "--cached")
		}
		args = append(args, strings.TrimSpace(string(ancestor)), "--")
		data, err := gitOutput(root, args...)
		if err != nil {
			return nil, err
		}
		files, err := parseDiff(data)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if file.deleted || !handlerFile(file.path) || file.oldPath == "/dev/null" || len(file.hunks) == 0 {
				continue
			}
			before, err := gitOutput(root, "show", strings.TrimSpace(string(ancestor))+":"+file.oldPath)
			if err != nil {
				return nil, err
			}
			var after []byte
			if cached {
				after, err = gitOutput(root, "show", ":"+file.path)
			} else {
				after, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(file.path)))
			}
			if err != nil {
				return nil, err
			}
			file.before, file.after = sourceLines(before), sourceLines(after)
		}
		views = append(views, files)
	}
	untracked, err := gitOutput(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Split(string(untracked), "\x00") {
		if name == "" {
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Lstat(full)
		if err != nil {
			return nil, err
		}
		var content []byte
		switch {
		case info.Mode().IsRegular():
			content, err = os.ReadFile(full)
		case info.Mode()&os.ModeSymlink != 0:
			var target string
			target, err = os.Readlink(full)
			content = []byte(target)
		default:
			return nil, fmt.Errorf("cannot read untracked file %s", name)
		}
		if err != nil {
			return nil, err
		}
		if bytes.ContainsRune(content, 0) {
			continue // Match git's treatment of binary content.
		}
		h := &hunk{added: sourceLines(content)}
		views[0] = append(views[0], &fileChange{path: name, hunks: []*hunk{h}})
	}
	return views, nil
}

func sourceLines(data []byte) []sourceLine {
	var lines []sourceLine
	if len(data) > 0 {
		for i, text := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
			lines = append(lines, sourceLine{i + 1, strings.TrimSuffix(text, "\r")})
		}
	}
	return lines
}

// Hunk counts distinguish headers from content such as an added "++value".
// Incomplete or unfamiliar patches fail closed rather than report clean.
func parseDiff(data []byte) ([]*fileChange, error) {
	var files []*fileChange
	var file *fileChange
	var current *hunk
	oldLine, newLine, oldLeft, newLeft := 0, 0, 0, 0
	pendingOld, headers := false, false
	for i, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSuffix(raw, "\r")
		bad := func() ([]*fileChange, error) {
			return nil, fmt.Errorf("invalid unified diff at line %d", i+1)
		}
		if line == "\\ No newline at end of file" && current != nil {
			continue
		}
		if oldLeft > 0 || newLeft > 0 {
			if line == "" {
				return bad()
			}
			switch line[0] {
			case '-':
				current.removed = append(current.removed, sourceLine{oldLine, line[1:]})
				current.before = append(current.before, sourceLine{oldLine, line[1:]})
				oldLine++
				oldLeft--
			case '+':
				current.added = append(current.added, sourceLine{newLine, line[1:]})
				current.after = append(current.after, sourceLine{newLine, line[1:]})
				newLine++
				newLeft--
			case ' ':
				current.before = append(current.before, sourceLine{oldLine, line[1:]})
				current.after = append(current.after, sourceLine{newLine, line[1:]})
				oldLine++
				newLine++
				oldLeft--
				newLeft--
			default:
				return bad()
			}
			if oldLeft < 0 || newLeft < 0 {
				return bad()
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "diff --git "):
			if pendingOld {
				return bad()
			}
			old, new, err := diffPaths(strings.TrimPrefix(line, "diff --git "))
			if err != nil {
				return bad()
			}
			file = &fileChange{oldPath: old, path: new}
			files = append(files, file)
			headers, current = false, nil
		case strings.HasPrefix(line, "--- "):
			if pendingOld {
				return bad()
			}
			if file == nil || headers {
				file = &fileChange{}
				files = append(files, file)
			}
			var err error
			file.oldPath, err = headerPath(line[4:])
			if err != nil {
				return bad()
			}
			pendingOld, headers, current = true, false, nil
		case strings.HasPrefix(line, "+++ "):
			if !pendingOld {
				return bad()
			}
			name, err := headerPath(line[4:])
			if err != nil {
				return bad()
			}
			file.deleted = name == "/dev/null"
			file.path = name
			if file.deleted {
				file.path = file.oldPath
			}
			pendingOld, headers = false, true
			file.textHeaders = true
		case strings.HasPrefix(line, "@@"):
			match := hunkPattern.FindStringSubmatch(line)
			if !headers || match == nil {
				return bad()
			}
			values := make([]int, 4)
			for j, s := range match[1:] {
				if s == "" {
					s = "1"
				}
				var err error
				values[j], err = strconv.Atoi(s)
				if err != nil {
					return bad()
				}
			}
			oldLine, oldLeft, newLine, newLeft = values[0], values[1], values[2], values[3]
			current = &hunk{}
			file.hunks = append(file.hunks, current)
		case strings.HasPrefix(line, "deleted file mode ") && file != nil:
			file.deleted = true
			file.path = file.oldPath
			file.metadata = true
			file.emptyFile = true
		case strings.HasPrefix(line, "new file mode ") && file != nil:
			file.metadata = true
			file.emptyFile = true
		case strings.HasPrefix(line, "Binary files ") && file != nil:
			file.metadata = true
			file.binary = true
		case strings.HasPrefix(line, "old mode ") && file != nil:
			file.oldMode = true
		case strings.HasPrefix(line, "new mode ") && file != nil:
			file.metadata = file.metadata || file.oldMode
		case strings.HasPrefix(line, "rename from ") && file != nil:
			file.renameFrom = true
		case strings.HasPrefix(line, "rename to ") && file != nil:
			file.metadata = file.metadata || file.renameFrom
		case strings.HasPrefix(line, "copy from ") && file != nil:
			file.copyFrom = true
		case strings.HasPrefix(line, "copy to ") && file != nil:
			file.metadata = file.metadata || file.copyFrom
		case line == "":
		case strings.HasPrefix(line, "index ") && file != nil:
			file.index = strings.TrimPrefix(line, "index ")
		case file != nil && (strings.HasPrefix(line, "similarity index ") || strings.HasPrefix(line, "dissimilarity index ")):
			file.contentChange = line != "similarity index 100%"
		default:
			return bad()
		}
	}
	if pendingOld || oldLeft != 0 || newLeft != 0 {
		return nil, errors.New("incomplete unified diff")
	}
	for _, file := range files {
		metadata := file.metadata && file.index == "" && !file.contentChange
		if file.emptyFile {
			metadata = emptyFileIndex(file.index, file.deleted)
		}
		if file.binary {
			metadata = true
		}
		if len(file.hunks) == 0 && (file.textHeaders || !metadata) {
			return nil, errors.New("incomplete unified diff: file header without a hunk")
		}
	}
	return files, nil
}

func emptyFileIndex(index string, deleted bool) bool {
	fields := strings.Fields(index)
	if len(fields) == 0 {
		return false
	}
	old, new, ok := strings.Cut(fields[0], "..")
	if deleted {
		old, new = new, old
	}
	if !ok || len(old) < 4 || len(new) < 4 || strings.Trim(old, "0") != "" {
		return false
	}
	for _, hash := range []string{
		fmt.Sprintf("%x", sha1.Sum([]byte("blob 0\x00"))),
		fmt.Sprintf("%x", sha256.Sum256([]byte("blob 0\x00"))),
	} {
		if strings.HasPrefix(hash, new) {
			return true
		}
	}
	return false
}

func headerPath(value string) (string, error) {
	name, _, _ := strings.Cut(value, "\t") // Traditional diffs append a timestamp.
	if strings.HasPrefix(name, `"`) {
		var err error
		name, err = strconv.Unquote(name)
		if err != nil {
			return "", err
		}
	}
	if name == "" {
		return "", errors.New("empty diff path")
	}
	if strings.HasPrefix(name, "a/") || strings.HasPrefix(name, "b/") {
		name = name[2:]
	}
	return name, nil
}

func diffPaths(value string) (string, string, error) {
	// Git leaves spaces unquoted. Try each separator, preferring equal paths
	// (renames are disabled for working-tree diffs), including empty deletions.
	for i := 1; i < len(value); i++ {
		if value[i] != ' ' {
			continue
		}
		old, errOld := headerPath(value[:i])
		new, errNew := headerPath(value[i+1:])
		if errOld == nil && errNew == nil && old == new {
			return old, new, nil
		}
	}
	separator := strings.LastIndex(value, " b/")
	if strings.HasPrefix(value, `"`) {
		separator = strings.Index(value, `" "`) + 1
	}
	if separator <= 0 {
		return "", "", errors.New("invalid git diff paths")
	}
	old, err := headerPath(value[:separator])
	if err != nil {
		return "", "", err
	}
	new, err := headerPath(value[separator+1:])
	return old, new, err
}

func isTest(name string) bool {
	base := path.Base(name)
	if strings.HasSuffix(base, "_test.go") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") ||
		strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py") || strings.HasSuffix(base, "_test.py") {
		return true
	}
	for _, part := range strings.Split(path.Dir(name), "/") {
		if part == "test" || part == "tests" || part == "__tests__" {
			return true
		}
	}
	return strings.HasSuffix(base, "Test.java") || strings.HasSuffix(base, "Tests.java")
}

func testChange(file *fileChange) bool {
	if isTest(file.path) {
		return true
	}
	if path.Ext(file.path) == ".rs" {
		for _, h := range file.hunks {
			// The hunk views include unchanged context, where an existing
			// #[test] attribute usually sits.
			for _, lines := range [][]sourceLine{h.removed, h.added, h.before, h.after} {
				for _, line := range lines {
					if rustTestPattern.MatchString(line.text) {
						return true
					}
				}
			}
		}
	}
	return false
}

func testDeclarations(file *fileChange, added bool) map[string]sourceLine {
	names := map[string]sourceLine{}
	capture := 0
	switch path.Ext(file.path) {
	case ".go":
		capture = 1
	case ".py":
		capture = 2
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
		capture = 3
	}
	for _, h := range file.hunks {
		lines, changed := h.before, h.removed
		if added {
			lines, changed = h.after, h.added
		}
		if len(lines) == 0 {
			lines = changed
		}
		modified := map[int]bool{}
		for _, line := range changed {
			modified[line.number] = true
		}
		var attribute *sourceLine
		for _, line := range lines {
			if path.Ext(file.path) == ".rs" {
				text := strings.TrimSpace(line.text)
				if text == "#[test]" {
					copy := line
					attribute = &copy
					continue
				}
				if attribute != nil {
					if match := rustFunctionPattern.FindStringSubmatch(line.text); match != nil {
						if modified[line.number] || modified[attribute.number] {
							names["rust:"+match[1]] = *attribute
						}
					}
					if text != "" && !strings.HasPrefix(text, "#[") {
						attribute = nil
					}
				}
				continue
			}
			if !modified[line.number] {
				continue
			}
			if match := testPattern.FindStringSubmatch(line.text); match != nil && capture > 0 && match[capture] != "" {
				names[fmt.Sprintf("%d:%s", capture, match[capture])] = line
			}
		}
	}
	return names
}

func handlerFile(name string) bool {
	switch path.Ext(name) {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py":
		return true
	}
	return false
}

type errorBlock struct {
	header sourceLine
	body   []sourceLine
	closed bool
}

func errorBlocks(lines []sourceLine) map[string][]errorBlock {
	blocks := map[string][]errorBlock{}
	for i, line := range lines {
		header := handlerPattern.FindString(line.text)
		if header == "" {
			continue
		}
		block := errorBlock{header: line}
		python := strings.HasPrefix(strings.TrimSpace(line.text), "except")
		indent := len(line.text) - len(strings.TrimLeft(line.text, " \t"))
		for _, body := range lines[i+1:] {
			text := strings.TrimSpace(body.text)
			if text == "" {
				continue
			}
			bodyIndent := len(body.text) - len(strings.TrimLeft(body.text, " \t"))
			if python && bodyIndent <= indent || !python && (text == "}" || text == "};") && bodyIndent <= indent {
				block.closed = true
				break
			}
			block.body = append(block.body, body)
		}
		block.closed = block.closed || python
		key, _, _ := strings.Cut(header, "//")
		key, _, _ = strings.Cut(key, "#")
		key = strings.TrimSpace(key)
		blocks[key] = append(blocks[key], block)
	}
	return blocks
}

func emptiedHandlers(file *fileChange) []sourceLine {
	if !handlerFile(file.path) {
		return nil
	}
	removed := map[int]bool{}
	for _, h := range file.hunks {
		for _, line := range h.removed {
			removed[line.number] = true
		}
	}
	views := [][2][]sourceLine{{file.before, file.after}}
	if file.before == nil {
		views = nil
		for _, h := range file.hunks {
			views = append(views, [2][]sourceLine{h.before, h.after})
		}
	}
	var lines []sourceLine
	for _, view := range views {
		before, after := errorBlocks(view[0]), errorBlocks(view[1])
		for key, old := range before {
			current := after[key]
			if len(old) != len(current) {
				continue
			}
			for i, block := range old {
				if block.closed && len(block.body) > 0 && allRemoved(block.body, removed) && current[i].closed && len(current[i].body) == 0 {
					lines = append(lines, current[i].header)
				}
			}
		}
	}
	return lines
}

func inspect(files []*fileChange) []finding {
	findings := make([]finding, 0)
	addedTests := map[string]bool{}
	for _, file := range files {
		if testChange(file) {
			for name := range testDeclarations(file, true) {
				addedTests[name] = true
			}
		}
	}
	add := func(file, kind string, line sourceLine) {
		text := []rune(strings.TrimSpace(line.text))
		f := finding{Path: file, Line: line.number, Kind: kind, Text: string(text[:min(120, len(text))])}
		if match := waiverPattern.FindStringSubmatch(line.text); match != nil {
			f.Waived, f.Reason = true, strings.TrimSpace(match[1])
		}
		findings = append(findings, f)
	}
	for _, file := range files {
		if file.deleted {
			if testChange(file) {
				line := sourceLine{1, "file deleted"}
				if len(file.hunks) > 0 && len(file.hunks[0].removed) > 0 {
					line = file.hunks[0].removed[0]
				}
				add(file.path, "deleted-test-file", line)
			}
			continue
		}
		var removedAssertions []sourceLine
		addedAssertions := 0
		for _, h := range file.hunks {
			for _, line := range h.added {
				for _, check := range []struct {
					kind    string
					pattern *regexp.Regexp
				}{
					{"skipped-test", skipPattern},
					{"suppression", suppressionPattern},
					{"empty-error-handler", emptyPattern},
				} {
					if check.pattern.MatchString(line.text) {
						add(file.path, check.kind, line)
					}
				}
				if assertionPattern.MatchString(line.text) {
					addedAssertions++
				}
			}
			for _, line := range h.removed {
				if assertionPattern.MatchString(line.text) {
					removedAssertions = append(removedAssertions, line)
				}
			}
			for _, line := range loosenedThresholds(h) {
				add(file.path, "threshold-loosened", line)
			}
			for _, line := range unverifiableThresholds(h) {
				add(file.path, "threshold-changed", line)
			}
		}
		for _, line := range emptiedHandlers(file) {
			add(file.path, "empty-error-handler", line)
		}
		if testChange(file) {
			for name, line := range testDeclarations(file, false) {
				if !addedTests[name] {
					add(file.path, "test-removed", line)
				}
			}
			for _, line := range removedAssertions[:max(0, len(removedAssertions)-addedAssertions)] {
				add(file.path, "assertion-removed", line)
			}
		}
	}
	return findings
}

type threshold struct {
	line  sourceLine
	value float64
	unit  string
	max   bool
	valid bool
}

func thresholds(lines []sourceLine) map[string][]threshold {
	result := map[string][]threshold{}
	for _, line := range lines {
		match := assignmentPattern.FindStringSubmatch(line.text)
		if match == nil {
			continue
		}
		key := match[1]
		maximum := maximumPattern.MatchString(key)
		if !maximum && !minimumPattern.MatchString(key) {
			continue
		}
		value, err := strconv.ParseFloat(match[2], 64)
		result[key] = append(result[key], threshold{line, value, match[3], maximum, err == nil})
	}
	return result
}

func loosenedThresholds(h *hunk) []sourceLine {
	before, after := thresholds(h.removed), thresholds(h.added)
	var lines []sourceLine
	for key, old := range before {
		added := after[key]
		if len(old) != 1 || len(added) != 1 || old[0].unit != added[0].unit {
			continue
		}
		a, b := old[0], added[0]
		if a.valid && b.valid && (a.max && b.value > a.value || !a.max && b.value < a.value) {
			lines = append(lines, b.line)
		}
	}
	return lines
}

func unverifiableThresholds(h *hunk) []sourceLine {
	var lines []sourceLine
	for _, changed := range [][]sourceLine{h.removed, h.added} {
		for _, values := range thresholds(changed) {
			for _, value := range values {
				if !value.valid {
					line := value.line
					line.text = "unverifiable: " + line.text
					lines = append(lines, line)
				}
			}
		}
	}
	return lines
}

func allRemoved(lines []sourceLine, removed map[int]bool) bool {
	for _, line := range lines {
		if !removed[line.number] {
			return false
		}
	}
	return true
}
