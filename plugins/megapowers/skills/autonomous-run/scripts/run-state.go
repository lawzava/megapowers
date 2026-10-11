package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	exitOK       = 0
	exitMismatch = 3
	exitError    = 125
	maxTextRunes = 2000
	tailLines    = 5
	excludeEntry = ".megapowers/"
)

const usage = `run-state: deterministic state for megapowers autonomous runs

Usage:
  go run run-state.go <command> [--repo DIR] --id ID [flags]

Commands:
  init        --id ID --objective TEXT
  checkpoint  --id ID --milestone TEXT --next TEXT [--blocker TEXT ...] [--evidence TEXT ...]
  journal     --id ID --event KIND --evidence TEXT [--decision TEXT --why TEXT]
  resume      --id ID

State lives in <worktree>/.megapowers/run/<id>/: charter.md, checkpoint.md,
journal.jsonl (append-only), handoff.md. IDs match [a-z0-9][a-z0-9-]{0,62}.

Exit codes: 0 ok, 3 resume found a workspace, branch, worktree, or HEAD
mismatch, 125 tool or usage error.
`

var (
	idPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	eventPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	headerLine   = regexp.MustCompile(`^- (Workspace|Branch|Worktree|HEAD): (.*)$`)
)

type options struct {
	repo      string
	id        string
	objective string
	milestone string
	next      string
	event     string
	evidence  []string
	blockers  []string
	decision  string
	why       string
	seen      map[string]bool
}

type gitState struct {
	workspace string
	worktree  string
	branch    string
	head      string
	dirty     bool
	exclude   string
}

type journalRecord struct {
	TS        string `json:"ts"`
	Event     string `json:"event"`
	Evidence  string `json:"evidence"`
	Decision  string `json:"decision,omitempty"`
	Why       string `json:"why,omitempty"`
	Head      string `json:"head"`
	Branch    string `json:"branch,omitempty"`
	Worktree  string `json:"worktree,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Next      string `json:"next,omitempty"`
}

type toolError struct{ msg string }

func (e toolError) Error() string { return e.msg }

func fail(format string, args ...any) error { return toolError{fmt.Sprintf(format, args...)} }

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return exitError
	}
	command := args[0]
	if isHelp(command) {
		fmt.Print(usage)
		return exitOK
	}
	opt, err := parseOptions(command, args[1:])
	if errors.Is(err, errHelp) {
		fmt.Print(usage)
		return exitOK
	}
	if err != nil {
		return report(err)
	}
	switch command {
	case "init":
		err = cmdInit(opt)
	case "checkpoint":
		err = cmdCheckpoint(opt)
	case "journal":
		err = cmdJournal(opt)
	case "resume":
		var code int
		code, err = cmdResume(opt)
		if err == nil {
			return code
		}
	}
	if err != nil {
		return report(err)
	}
	return exitOK
}

func report(err error) int {
	fmt.Fprintf(os.Stderr, "run-state: %v\n", err)
	return exitError
}

var allowedFlags = map[string][]string{
	"init":       {"--repo", "--id", "--objective"},
	"checkpoint": {"--repo", "--id", "--milestone", "--next", "--blocker", "--evidence"},
	"journal":    {"--repo", "--id", "--event", "--evidence", "--decision", "--why"},
	"resume":     {"--repo", "--id"},
}

func parseOptions(command string, args []string) (options, error) {
	opt := options{repo: ".", seen: map[string]bool{}}
	flags, ok := allowedFlags[command]
	if !ok {
		return opt, fail("unknown command: %s (see --help)", command)
	}
	repeatable := command == "checkpoint"
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if isHelp(arg) {
			return opt, errHelp
		}
		if !contains(flags, arg) {
			return opt, fail("%s: unknown arg: %s", command, arg)
		}
		multi := repeatable && (arg == "--blocker" || arg == "--evidence")
		if opt.seen[arg] && !multi {
			return opt, fail("%s may be provided only once", arg)
		}
		opt.seen[arg] = true
		if i+1 >= len(args) {
			return opt, fail("%s requires a value", arg)
		}
		i++
		value := args[i]
		switch arg {
		case "--repo":
			opt.repo = value
		case "--id":
			opt.id = value
		case "--objective":
			opt.objective = clean(value)
		case "--milestone":
			opt.milestone = clean(value)
		case "--next":
			opt.next = clean(value)
		case "--event":
			opt.event = value
		case "--evidence":
			opt.evidence = append(opt.evidence, clean(value))
		case "--blocker":
			opt.blockers = append(opt.blockers, clean(value))
		case "--decision":
			opt.decision = clean(value)
		case "--why":
			opt.why = clean(value)
		}
	}
	if !idPattern.MatchString(opt.id) {
		return opt, fail("--id must match [a-z0-9][a-z0-9-]{0,62}")
	}
	if strings.TrimSpace(opt.repo) == "" {
		return opt, fail("--repo must not be empty")
	}
	required := map[string]string{}
	switch command {
	case "init":
		required["--objective"] = opt.objective
	case "checkpoint":
		required["--milestone"] = opt.milestone
		required["--next"] = opt.next
		for _, value := range append(append([]string{}, opt.evidence...), opt.blockers...) {
			if value == "" {
				return opt, fail("--evidence and --blocker values must not be empty")
			}
		}
	case "journal":
		if !eventPattern.MatchString(opt.event) {
			return opt, fail("--event must match [a-z0-9][a-z0-9_-]{0,31}")
		}
		if len(opt.evidence) == 0 || opt.evidence[0] == "" {
			return opt, fail("--evidence is required")
		}
		if (opt.decision == "") != (opt.why == "") {
			return opt, fail("--decision and --why must be given together")
		}
	}
	for flag, value := range required {
		if value == "" {
			return opt, fail("%s is required", flag)
		}
	}
	return opt, nil
}

var errHelp = errors.New("help requested")

func isHelp(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "help"
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// clean folds control characters (including newlines) into spaces, trims, and
// caps the result so every record stays one bounded line.
func clean(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	if runes := []rune(value); len(runes) > maxTextRunes {
		value = string(runes[:maxTextRunes])
	}
	return value
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), err
}

func inspect(repo string) (gitState, error) {
	var st gitState
	top, err := git(repo, "rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return st, fail("not inside a git worktree: %s", repo)
	}
	st.worktree = filepath.Clean(top)
	common, err := git(st.worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || common == "" {
		return st, fail("cannot resolve git common directory")
	}
	common = filepath.Clean(common)
	if filepath.Base(common) == ".git" {
		st.workspace = filepath.Dir(common)
	} else {
		st.workspace = common
	}
	exclude, err := git(st.worktree, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude")
	if err != nil || exclude == "" {
		return st, fail("cannot resolve info/exclude")
	}
	st.exclude = exclude
	if branch, err := git(st.worktree, "symbolic-ref", "--short", "-q", "HEAD"); err == nil && branch != "" {
		st.branch = branch
	} else {
		st.branch = "(detached)"
	}
	if head, err := git(st.worktree, "rev-parse", "--verify", "-q", "HEAD"); err == nil && head != "" {
		st.head = head
	} else {
		st.head = "(unborn)"
	}
	status, err := git(st.worktree, "status", "--porcelain")
	if err != nil {
		return st, fail("git status failed")
	}
	st.dirty = status != ""
	return st, nil
}

func runDir(st gitState, id string) string {
	return filepath.Join(st.worktree, ".megapowers", "run", id)
}

// ensureDir creates dir with 0700 and refuses symlinks along the run path.
func ensureDir(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fail("%s is not a plain directory", path)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fail("cannot inspect %s", path)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		return fail("cannot create %s: %v", path, err)
	}
	return os.Chmod(path, 0o700)
}

func existingRun(st gitState, id string) (string, error) {
	dir := runDir(st, id)
	for _, path := range []string{filepath.Dir(filepath.Dir(dir)), filepath.Dir(dir), dir} {
		info, err := os.Lstat(path)
		if err != nil {
			return "", fail("run %q not found under %s", id, filepath.Dir(dir))
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fail("%s is not a plain directory", path)
		}
	}
	return dir, nil
}

func refuseSymlink(path string) error {
	if info, err := os.Lstat(path); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return fail("%s is not a regular file", path)
	}
	return nil
}

func ensureExcluded(st gitState) error {
	cmd := exec.Command("git", "-C", st.worktree, "check-ignore", "-q", ".megapowers/x")
	err := cmd.Run()
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return fail("git check-ignore failed")
	}
	if err := os.MkdirAll(filepath.Dir(st.exclude), 0o755); err != nil {
		return fail("cannot create %s", filepath.Dir(st.exclude))
	}
	existing, err := os.ReadFile(st.exclude)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fail("cannot read %s", st.exclude)
	}
	line := excludeEntry + "\n"
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		line = "\n" + line
	}
	file, err := os.OpenFile(st.exclude, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return fail("cannot open %s", st.exclude)
	}
	defer file.Close()
	if _, err := file.WriteString(line); err != nil {
		return fail("cannot write %s", st.exclude)
	}
	return nil
}

func writeExclusive(path, body string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fail("cannot create %s: %v", path, err)
	}
	if _, err := file.WriteString(body); err != nil {
		file.Close()
		return fail("cannot write %s", path)
	}
	if err := file.Close(); err != nil {
		return fail("cannot write %s", path)
	}
	return os.Chmod(path, 0o600)
}

// writeReplace atomically replaces path with body at mode 0600.
func writeReplace(path, body string) error {
	if err := refuseSymlink(path); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fail("cannot create temp file for %s", path)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fail("cannot chmod %s", name)
	}
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return fail("cannot write %s", path)
	}
	if err := tmp.Close(); err != nil {
		return fail("cannot write %s", path)
	}
	if err := os.Rename(name, path); err != nil {
		return fail("cannot replace %s", path)
	}
	return nil
}

func appendJournal(dir string, rec journalRecord) error {
	path := filepath.Join(dir, "journal.jsonl")
	if err := refuseSymlink(path); err != nil {
		return err
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return fail("cannot encode journal record")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return fail("cannot open %s", path)
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return fail("cannot append %s", path)
	}
	return file.Chmod(0o600)
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func cmdInit(opt options) error {
	st, err := inspect(opt.repo)
	if err != nil {
		return err
	}
	dir := runDir(st, opt.id)
	if _, err := os.Lstat(dir); err == nil {
		return fail("run %q already exists at %s", opt.id, dir)
	}
	if err := ensureExcluded(st); err != nil {
		return err
	}
	for _, path := range []string{filepath.Dir(filepath.Dir(dir)), filepath.Dir(dir)} {
		if err := ensureDir(path); err != nil {
			return err
		}
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fail("run %q already exists at %s", opt.id, dir)
		}
		return fail("cannot create %s: %v", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fail("cannot chmod %s", dir)
	}
	ts := now()
	charter := fmt.Sprintf("# Charter: %s\n\nCreated: %s\n\n## Objective\n\n%s\n\n## Done criteria\n\n- (fill in: each criterion with its oracle)\n\n## Scope\n\n- (fill in)\n\n## Authority\n\n- (fill in: approved effects and their targets)\n\n## Cap\n\n- (fill in: time, cost, or attempt limit)\n", opt.id, ts, opt.objective)
	if err := writeExclusive(filepath.Join(dir, "charter.md"), charter); err != nil {
		return err
	}
	rec := journalRecord{TS: ts, Event: "init", Evidence: clean("objective: " + opt.objective), Head: st.head, Branch: st.branch, Worktree: st.worktree, Workspace: st.workspace}
	if err := appendJournal(dir, rec); err != nil {
		return err
	}
	fmt.Printf("run-state: initialized %s\n", dir)
	return nil
}

func bulletList(items []string) string {
	if len(items) == 0 {
		return "- none\n"
	}
	var b strings.Builder
	for _, item := range items {
		b.WriteString("- " + item + "\n")
	}
	return b.String()
}

func cmdCheckpoint(opt options) error {
	st, err := inspect(opt.repo)
	if err != nil {
		return err
	}
	dir, err := existingRun(st, opt.id)
	if err != nil {
		return err
	}
	ts := now()
	dirty := "no"
	if st.dirty {
		dirty = "yes"
	}
	checkpointPath := filepath.Join(dir, "checkpoint.md")
	charterPath := filepath.Join(dir, "charter.md")
	checkpoint := fmt.Sprintf("# Checkpoint: %s\n\n- Time: %s\n- Workspace: %s\n- Branch: %s\n- Worktree: %s\n- HEAD: %s\n- Dirty: %s\n- Milestone: %s\n\n## Evidence\n\n%s\n## Blockers\n\n%s\n## Next safe action\n\n%s\n",
		opt.id, ts, st.workspace, st.branch, st.worktree, st.head, dirty, opt.milestone, bulletList(opt.evidence), bulletList(opt.blockers), opt.next)
	if err := writeReplace(checkpointPath, checkpoint); err != nil {
		return err
	}
	handoff := fmt.Sprintf("# Handoff: %s\n\nResume the approved autonomous run %q in %s.\n\n1. Read the repository instructions and any native goal state.\n2. Read the charter: %s\n3. Read the latest checkpoint (%s): %s\n4. Run `run-state resume --id %s` from the autonomous-run skill scripts. On any mismatch, stop affected work and reconcile first.\n5. Next safe action: %s\n",
		opt.id, opt.id, st.worktree, charterPath, ts, checkpointPath, opt.id, opt.next)
	if err := writeReplace(filepath.Join(dir, "handoff.md"), handoff); err != nil {
		return err
	}
	evidence := "milestone: " + opt.milestone
	if len(opt.evidence) > 0 {
		evidence += "; evidence: " + strings.Join(opt.evidence, " | ")
	}
	rec := journalRecord{TS: ts, Event: "checkpoint", Evidence: clean(evidence), Head: st.head, Branch: st.branch, Worktree: st.worktree, Workspace: st.workspace, Next: opt.next}
	if err := appendJournal(dir, rec); err != nil {
		return err
	}
	fmt.Printf("run-state: checkpoint written to %s\n", checkpointPath)
	return nil
}

func cmdJournal(opt options) error {
	st, err := inspect(opt.repo)
	if err != nil {
		return err
	}
	dir, err := existingRun(st, opt.id)
	if err != nil {
		return err
	}
	rec := journalRecord{TS: now(), Event: opt.event, Evidence: opt.evidence[0], Decision: opt.decision, Why: opt.why, Head: st.head}
	if err := appendJournal(dir, rec); err != nil {
		return err
	}
	fmt.Printf("run-state: journal %s appended\n", opt.event)
	return nil
}

type recorded struct {
	fields   map[string]string
	blockers []string
	next     string
}

func parseCheckpoint(path string) (recorded, error) {
	rec := recorded{fields: map[string]string{}}
	if err := refuseSymlink(path); err != nil {
		return rec, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return rec, fail("no checkpoint at %s; run checkpoint first", path)
	}
	section := ""
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## ") {
			section = strings.TrimPrefix(line, "## ")
			continue
		}
		switch section {
		case "":
			if m := headerLine.FindStringSubmatch(line); m != nil {
				rec.fields[m[1]] = m[2]
			}
		case "Blockers":
			if item := strings.TrimPrefix(line, "- "); item != line && item != "none" {
				rec.blockers = append(rec.blockers, item)
			}
		case "Next safe action":
			if rec.next == "" && strings.TrimSpace(line) != "" {
				rec.next = strings.TrimSpace(line)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return rec, fail("cannot read %s", path)
	}
	for _, key := range []string{"Workspace", "Branch", "Worktree", "HEAD"} {
		if _, ok := rec.fields[key]; !ok {
			return rec, fail("checkpoint %s lacks %s; write a fresh checkpoint", path, key)
		}
	}
	return rec, nil
}

func journalTail(path string, n int) ([]string, error) {
	if err := refuseSymlink(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fail("cannot read %s", path)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

func cmdResume(opt options) (int, error) {
	st, err := inspect(opt.repo)
	if err != nil {
		return exitError, err
	}
	dir, err := existingRun(st, opt.id)
	if err != nil {
		return exitError, err
	}
	rec, err := parseCheckpoint(filepath.Join(dir, "checkpoint.md"))
	if err != nil {
		return exitError, err
	}
	tail, err := journalTail(filepath.Join(dir, "journal.jsonl"), tailLines)
	if err != nil {
		return exitError, err
	}
	fmt.Printf("run-state: resume %s\n", opt.id)
	mismatch := false
	for _, check := range []struct{ label, key, live string }{
		{"workspace", "Workspace", st.workspace},
		{"branch", "Branch", st.branch},
		{"worktree", "Worktree", st.worktree},
		{"head", "HEAD", st.head},
	} {
		if want := rec.fields[check.key]; want == check.live {
			fmt.Printf("%s: match\n", check.label)
		} else {
			mismatch = true
			fmt.Printf("%s: mismatch: recorded %s, live %s\n", check.label, want, check.live)
		}
	}
	fmt.Printf("journal tail (last %d):\n", len(tail))
	for _, line := range tail {
		fmt.Println("  " + line)
	}
	fmt.Printf("next safe action: %s\n", rec.next)
	if len(rec.blockers) == 0 {
		fmt.Println("blockers: none")
	} else {
		fmt.Println("blockers:")
		for _, blocker := range rec.blockers {
			fmt.Println("- " + blocker)
		}
	}
	if mismatch {
		fmt.Println("result: mismatch; stop affected work and reconcile before continuing")
		return exitMismatch, nil
	}
	fmt.Println("result: match")
	return exitOK, nil
}
