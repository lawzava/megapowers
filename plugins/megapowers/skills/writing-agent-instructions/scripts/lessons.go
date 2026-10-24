//go:build ignore

// lessons keeps the per-project promotion ledger of the learning loop: a
// lesson is added with evidence, proven, promoted with a read-back of its
// target, reviewed against a session-mine scorecard, and retired when it
// stops paying off. The store lives outside every worktree, like a holdout.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	exitOK       = 0
	exitAction   = 3
	exitError    = 125
	defaultAge   = 90
	schemaV1     = "1"
	dateLayout   = "2006-01-02"
	storeEnvName = "MEGAPOWERS_LESSONS_DIR"
)

const usage = `lessons: per-project lesson ledger for the megapowers learning loop

Usage:
  go run lessons.go <command> [--repo DIR] [--store DIR] [flags]

Commands:
  add      --id ID --class CLASS --lesson TEXT --surface check|instructions|memory|upstream
           --target PATH --evidence KIND --source TEXT [--source TEXT ...] [--max-age-days N]
  list     [--status candidate|proven|promoted|retired]
  prove    --id ID --fail-ref REF [--check-file PATH ...] -- COMMAND...
                                                  check lessons: fails at REF, passes now
  prove    --id ID --ab TEXT                       other lessons: cite the A/B result
  promote  --id ID [--anchor TEXT] [--today YYYY-MM-DD]
  review   --scorecard FILE [--today YYYY-MM-DD]
  retire   --id ID --why TEXT
  export   --id ID                                 sanitized upstream draft with a case

The store defaults to $MEGAPOWERS_LESSONS_DIR, else
${XDG_STATE_HOME:-~/.local/state}/megapowers/lessons, and must be outside the
repository. Evidence is direct-statement, direct-observation, source-backed, or
history-entry-only. CLASS is a session-mine correction class, "sleep", or
"gate:<skill>", so review can count recurrences.

prove copies the current version of every relative file named on the command,
and of each --check-file, into the old tree, so the check judges old code
rather than its own absence.

Exit codes: 0 ok, 3 action needed (proof failed, read-back missing, lesson
recurring or stale), 125 tool or usage error.
`

var (
	idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	surfaces  = []string{"check", "instructions", "memory", "upstream"}
	evidence  = []string{"direct-statement", "direct-observation", "source-backed", "history-entry-only"}
	statuses  = []string{"candidate", "proven", "promoted", "retired"}
)

type proof struct {
	Command  []string `json:"command,omitempty"`
	Overlay  []string `json:"overlay,omitempty"`
	FailRef  string   `json:"fail_ref,omitempty"`
	AB       string   `json:"ab,omitempty"`
	ProvedAt string   `json:"proved_at"`
}

type promotion struct {
	Anchor     string `json:"anchor,omitempty"`
	PromotedAt string `json:"promoted_at"`
}

type historyEntry struct {
	At     string `json:"at"`
	Action string `json:"action"`
	Note   string `json:"note,omitempty"`
}

type lesson struct {
	SchemaVersion string         `json:"schema_version"`
	ID            string         `json:"id"`
	Project       string         `json:"project"`
	Created       string         `json:"created"`
	Class         string         `json:"class"`
	Lesson        string         `json:"lesson"`
	Surface       string         `json:"surface"`
	Target        string         `json:"target"`
	Evidence      string         `json:"evidence"`
	Sources       []string       `json:"sources"`
	Status        string         `json:"status"`
	MaxAgeDays    int            `json:"max_age_days"`
	Proof         *proof         `json:"proof,omitempty"`
	Promotion     *promotion     `json:"promotion,omitempty"`
	RetiredWhy    string         `json:"retired_why,omitempty"`
	History       []historyEntry `json:"history"`
}

type options struct {
	command, repo, store, id, class, text, surface, target, evidence string
	sources, checkFiles                                              []string
	maxAge                                                           int
	status, failRef, ab, anchor, today, scorecard, why               string
	argv                                                             []string
}

type toolError struct{ msg string }

func (e toolError) Error() string { return e.msg }

func fail(format string, args ...any) error { return toolError{fmt.Sprintf(format, args...)} }

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return exitError
	}
	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Print(usage)
		return exitOK
	}
	opt, err := parse(args)
	if err != nil {
		return report(err)
	}
	commands := map[string]func(options) (int, error){
		"add": cmdAdd, "list": cmdList, "prove": cmdProve, "promote": cmdPromote,
		"review": cmdReview, "retire": cmdRetire, "export": cmdExport,
	}
	code, err := commands[opt.command](opt)
	if err != nil {
		return report(err)
	}
	return code
}

func report(err error) int {
	fmt.Fprintf(os.Stderr, "lessons: %v\nresult: error (exit %d)\n", err, exitError)
	return exitError
}

var allowed = map[string][]string{
	"add":     {"--id", "--class", "--lesson", "--surface", "--target", "--evidence", "--source", "--max-age-days"},
	"list":    {"--status"},
	"prove":   {"--id", "--fail-ref", "--ab", "--check-file"},
	"promote": {"--id", "--anchor", "--today"},
	"review":  {"--scorecard", "--today"},
	"retire":  {"--id", "--why"},
	"export":  {"--id"},
}

func parse(args []string) (options, error) {
	opt := options{command: args[0], repo: ".", maxAge: defaultAge}
	flags, ok := allowed[opt.command]
	if !ok {
		return opt, fail("unknown command: %s (see --help)", opt.command)
	}
	flags = append(flags, "--repo", "--store")
	seen := map[string]bool{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" && opt.command == "prove" {
			opt.argv = args[i+1:]
			break
		}
		if !contains(flags, arg) {
			return opt, fail("%s: unknown arg: %s", opt.command, arg)
		}
		if seen[arg] && arg != "--source" && arg != "--check-file" {
			return opt, fail("%s may be provided only once", arg)
		}
		seen[arg] = true
		if i+1 >= len(args) {
			return opt, fail("%s requires a value", arg)
		}
		i++
		value := args[i]
		switch arg {
		case "--repo":
			opt.repo = value
		case "--store":
			opt.store = value
		case "--id":
			opt.id = value
		case "--class":
			opt.class = value
		case "--lesson":
			opt.text = strings.TrimSpace(value)
		case "--surface":
			opt.surface = value
		case "--target":
			opt.target = value
		case "--evidence":
			opt.evidence = value
		case "--source":
			opt.sources = append(opt.sources, strings.TrimSpace(value))
		case "--max-age-days":
			if _, err := fmt.Sscanf(value, "%d", &opt.maxAge); err != nil || opt.maxAge < 1 {
				return opt, fail("--max-age-days must be a positive integer")
			}
		case "--status":
			opt.status = value
		case "--fail-ref":
			opt.failRef = value
		case "--check-file":
			opt.checkFiles = append(opt.checkFiles, value)
		case "--ab":
			opt.ab = strings.TrimSpace(value)
		case "--anchor":
			opt.anchor = value
		case "--today":
			if _, err := time.Parse(dateLayout, value); err != nil {
				return opt, fail("--today must use YYYY-MM-DD")
			}
			opt.today = value
		case "--scorecard":
			opt.scorecard = value
		case "--why":
			opt.why = strings.TrimSpace(value)
		}
	}
	if contains(allowed[opt.command], "--id") && !idPattern.MatchString(opt.id) {
		return opt, fail("--id must match [a-z0-9][a-z0-9-]{0,62}")
	}
	switch opt.command {
	case "add":
		for flag, value := range map[string]string{"--class": opt.class, "--lesson": opt.text, "--target": opt.target} {
			if value == "" {
				return opt, fail("%s is required", flag)
			}
		}
		if !contains(surfaces, opt.surface) {
			return opt, fail("--surface must be one of %s", strings.Join(surfaces, ", "))
		}
		if !contains(evidence, opt.evidence) {
			return opt, fail("--evidence must be one of %s; inferred, speculative, unknown, or contested claims are not lessons", strings.Join(evidence, ", "))
		}
		if len(opt.sources) == 0 || contains(opt.sources, "") {
			return opt, fail("--source is required and must not be empty")
		}
	case "list":
		if opt.status != "" && !contains(statuses, opt.status) {
			return opt, fail("--status must be one of %s", strings.Join(statuses, ", "))
		}
	case "prove":
		if (opt.ab == "") == (opt.failRef == "") {
			return opt, fail("prove needs either --fail-ref REF -- COMMAND or --ab TEXT")
		}
		if opt.failRef != "" && len(opt.argv) == 0 {
			return opt, fail("--fail-ref needs a check command after --")
		}
	case "review":
		if opt.scorecard == "" {
			return opt, fail("--scorecard is required")
		}
	case "retire":
		if opt.why == "" {
			return opt, fail("--why is required")
		}
	}
	return opt, nil
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fail("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

type project struct {
	top, common, dir string
}

func within(parent, path string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func resolve(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	// Resolve the longest existing prefix so a store that does not exist yet
	// still compares correctly against symlinked repository paths.
	rest := ""
	for {
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return filepath.Join(abs, rest)
		}
		rest = filepath.Join(filepath.Base(abs), rest)
		abs = parent
	}
}

func openProject(opt options) (project, error) {
	var p project
	top, err := git(opt.repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return p, fail("not inside a git worktree: %s", opt.repo)
	}
	p.top = filepath.Clean(top)
	common, err := git(p.top, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return p, err
	}
	p.common = filepath.Clean(common)
	store := opt.store
	if store == "" {
		store = os.Getenv(storeEnvName)
	}
	if store == "" {
		state := os.Getenv("XDG_STATE_HOME")
		if state == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return p, fail("cannot resolve a home directory; pass --store")
			}
			state = filepath.Join(home, ".local", "state")
		}
		store = filepath.Join(state, "megapowers", "lessons")
	}
	store = resolve(store)
	repoRoot := filepath.Dir(p.common)
	for _, root := range []string{resolve(p.top), resolve(repoRoot)} {
		if within(root, store) {
			return p, fail("the lesson store %s must be outside the repository %s, so implementing agents do not read it", store, root)
		}
	}
	sum := sha256.Sum256([]byte(p.common))
	p.dir = filepath.Join(store, filepath.Base(p.top)+"-"+hex.EncodeToString(sum[:4]))
	return p, nil
}

func (p project) path(id string) string { return filepath.Join(p.dir, id+".json") }

func load(p project, id string) (lesson, error) {
	var l lesson
	data, err := os.ReadFile(p.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return l, fail("lesson %q not found in %s", id, p.dir)
	}
	if err != nil {
		return l, fail("cannot read %s", p.path(id))
	}
	if err := json.Unmarshal(data, &l); err != nil || l.SchemaVersion != schemaV1 {
		return l, fail("%s is not a schema %s lesson", p.path(id), schemaV1)
	}
	return l, nil
}

func save(p project, l lesson, exclusive bool) error {
	if err := os.MkdirAll(p.dir, 0o700); err != nil {
		return fail("cannot create %s", p.dir)
	}
	body, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fail("cannot encode lesson")
	}
	path := p.path(l.ID)
	if exclusive {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			return fail("lesson %q already exists at %s", l.ID, path)
		}
		if err != nil {
			return fail("cannot create %s", path)
		}
		defer file.Close()
		_, err = file.Write(append(body, '\n'))
		return err
	}
	tmp, err := os.CreateTemp(p.dir, "."+l.ID+".tmp-*")
	if err != nil {
		return fail("cannot write %s", path)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(body, '\n')); err != nil {
		tmp.Close()
		return fail("cannot write %s", path)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fail("cannot write %s", path)
	}
	if err := tmp.Close(); err != nil {
		return fail("cannot write %s", path)
	}
	return os.Rename(tmp.Name(), path)
}

func today(opt options) string {
	if opt.today != "" {
		return opt.today
	}
	return time.Now().UTC().Format(dateLayout)
}

func stamp() string { return time.Now().UTC().Format(time.RFC3339) }

func cmdAdd(opt options) (int, error) {
	p, err := openProject(opt)
	if err != nil {
		return exitError, err
	}
	l := lesson{
		SchemaVersion: schemaV1, ID: opt.id, Project: p.top, Created: stamp(), Class: opt.class,
		Lesson: opt.text, Surface: opt.surface, Target: opt.target, Evidence: opt.evidence,
		Sources: opt.sources, Status: "candidate", MaxAgeDays: opt.maxAge,
		History: []historyEntry{{At: stamp(), Action: "add"}},
	}
	if err := save(p, l, true); err != nil {
		return exitError, err
	}
	fmt.Printf("lesson %s: candidate (%s, %d source(s)) in %s\n", l.ID, l.Surface, len(l.Sources), p.path(l.ID))
	fmt.Printf("result: ok (exit %d)\n", exitOK)
	return exitOK, nil
}

func all(p project) ([]lesson, error) {
	paths, _ := filepath.Glob(filepath.Join(p.dir, "*.json"))
	sort.Strings(paths)
	var out []lesson
	for _, path := range paths {
		l, err := load(p, strings.TrimSuffix(filepath.Base(path), ".json"))
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

func cmdList(opt options) (int, error) {
	p, err := openProject(opt)
	if err != nil {
		return exitError, err
	}
	ls, err := all(p)
	if err != nil {
		return exitError, err
	}
	shown := 0
	for _, l := range ls {
		if opt.status != "" && l.Status != opt.status {
			continue
		}
		shown++
		fmt.Printf("- %s [%s] %s -> %s (%s): %s\n", l.ID, l.Status, l.Surface, l.Target, l.Class, l.Lesson)
	}
	fmt.Printf("lessons: %d in %s\n", shown, p.dir)
	fmt.Printf("result: ok (exit %d)\n", exitOK)
	return exitOK, nil
}

// runCheck runs argv in dir and reports its exit status.
func runCheck(dir string, argv []string) (int, string, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, out.String(), nil
	case errors.As(err, &exit):
		return exit.ExitCode(), out.String(), nil
	default:
		return -1, out.String(), fail("cannot run %s: %v", argv[0], err)
	}
}

func cmdProve(opt options) (int, error) {
	p, err := openProject(opt)
	if err != nil {
		return exitError, err
	}
	l, err := load(p, opt.id)
	if err != nil {
		return exitError, err
	}
	if l.Status == "retired" {
		return exitError, fail("lesson %q is retired", l.ID)
	}
	if opt.ab != "" {
		if l.Surface == "check" {
			return exitError, fail("a check lesson is proven by --fail-ref and a command, not --ab")
		}
		l.Proof = &proof{AB: opt.ab, ProvedAt: stamp()}
	} else {
		if l.Surface != "check" {
			return exitError, fail("--fail-ref proves check lessons; cite an A/B result with --ab for %s lessons", l.Surface)
		}
		ref, err := git(p.top, "rev-parse", "--verify", "-q", opt.failRef+"^{commit}")
		if err != nil {
			return exitError, fail("--fail-ref %s is not a commit", opt.failRef)
		}
		scratch, err := os.MkdirTemp("", "lessons-prove-")
		if err != nil {
			return exitError, fail("cannot create a scratch directory")
		}
		defer os.RemoveAll(scratch)
		tree := filepath.Join(scratch, "tree")
		if _, err := git(p.top, "worktree", "add", "--detach", "-q", tree, ref); err != nil {
			return exitError, err
		}
		overlay, err := overlayChecks(p.top, tree, opt.argv, opt.checkFiles)
		if err != nil {
			git(p.top, "worktree", "remove", "--force", tree)
			return exitError, err
		}
		past, _, runErr := runCheck(tree, opt.argv)
		if _, err := git(p.top, "worktree", "remove", "--force", tree); err != nil {
			return exitError, err
		}
		if runErr != nil {
			return exitError, runErr
		}
		if past == 0 {
			fmt.Printf("lesson %s: the check passes at %s, where the mistake happened, so it cannot catch it\n", l.ID, ref[:12])
			fmt.Printf("result: action (exit %d); write a check that fails on the past instance\n", exitAction)
			return exitAction, nil
		}
		now, output, runErr := runCheck(p.top, opt.argv)
		if runErr != nil {
			return exitError, runErr
		}
		if now != 0 {
			fmt.Printf("lesson %s: the check fails on the current tree (exit %d):\n%s\n", l.ID, now, strings.TrimSpace(output))
			fmt.Printf("result: action (exit %d); fix the tree or the check before proving\n", exitAction)
			return exitAction, nil
		}
		l.Proof = &proof{Command: opt.argv, Overlay: overlay, FailRef: ref, ProvedAt: stamp()}
	}
	if l.Status == "candidate" {
		l.Status = "proven"
	}
	l.History = append(l.History, historyEntry{At: stamp(), Action: "prove"})
	if err := save(p, l, false); err != nil {
		return exitError, err
	}
	fmt.Printf("lesson %s: proven\n", l.ID)
	fmt.Printf("result: ok (exit %d)\n", exitOK)
	return exitOK, nil
}

// overlayChecks copies the current version of the check's own files into the
// old tree: every relative argument that names a regular file in the project,
// plus each --check-file. Without it, a check script added after the mistake
// is simply missing at the old commit, and its failure proves nothing.
func overlayChecks(top, tree string, argv, extra []string) ([]string, error) {
	var copied []string
	seen := map[string]bool{}
	for i, name := range append(append([]string{}, argv...), extra...) {
		explicit := i >= len(argv)
		if name == "" || strings.HasPrefix(name, "-") || filepath.IsAbs(name) {
			if explicit {
				return nil, fail("--check-file %q must be a relative path inside the project", name)
			}
			continue
		}
		clean := filepath.Clean(name)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			if explicit {
				return nil, fail("--check-file %q must stay inside the project", name)
			}
			continue
		}
		info, err := os.Lstat(filepath.Join(top, clean))
		if err != nil || !info.Mode().IsRegular() {
			if explicit {
				return nil, fail("--check-file %q is not a regular file in the project", name)
			}
			continue
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		data, err := os.ReadFile(filepath.Join(top, clean))
		if err != nil {
			return nil, fail("cannot read %s", clean)
		}
		dest := filepath.Join(tree, clean)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, fail("cannot prepare %s", dest)
		}
		if err := os.WriteFile(dest, data, info.Mode().Perm()); err != nil {
			return nil, fail("cannot write %s", dest)
		}
		copied = append(copied, filepath.ToSlash(clean))
	}
	return copied, nil
}

func cmdPromote(opt options) (int, error) {
	p, err := openProject(opt)
	if err != nil {
		return exitError, err
	}
	l, err := load(p, opt.id)
	if err != nil {
		return exitError, err
	}
	if l.Status == "retired" {
		return exitError, fail("lesson %q is retired", l.ID)
	}
	// A class counts after two occurrences; a proof can stand in for the
	// second one.
	if l.Proof == nil && len(l.Sources) < 2 {
		fmt.Printf("lesson %s: needs a proof or a second source before promotion\n", l.ID)
		fmt.Printf("result: action (exit %d)\n", exitAction)
		return exitAction, nil
	}
	if l.Surface == "check" {
		if l.Proof == nil || len(l.Proof.Command) == 0 {
			fmt.Printf("lesson %s: a check lesson needs a proof from prove --fail-ref\n", l.ID)
			fmt.Printf("result: action (exit %d)\n", exitAction)
			return exitAction, nil
		}
		code, output, err := runCheck(p.top, l.Proof.Command)
		if err != nil {
			return exitError, err
		}
		if code != 0 {
			fmt.Printf("lesson %s: the check fails on the current tree (exit %d):\n%s\n", l.ID, code, strings.TrimSpace(output))
			fmt.Printf("result: action (exit %d)\n", exitAction)
			return exitAction, nil
		}
	} else if l.Surface != "upstream" {
		if opt.anchor == "" {
			return exitError, fail("--anchor TEXT is required: the promoted text must read back from %s", l.Target)
		}
		target := l.Target
		if !filepath.IsAbs(target) {
			target = filepath.Join(p.top, target)
		}
		data, err := os.ReadFile(target)
		if err != nil || !strings.Contains(string(data), opt.anchor) {
			fmt.Printf("lesson %s: %s does not contain the anchor %q\n", l.ID, l.Target, opt.anchor)
			fmt.Printf("result: action (exit %d); write the lesson to its target, then promote\n", exitAction)
			return exitAction, nil
		}
	}
	l.Status = "promoted"
	l.Promotion = &promotion{Anchor: opt.anchor, PromotedAt: today(opt)}
	l.History = append(l.History, historyEntry{At: stamp(), Action: "promote"})
	if err := save(p, l, false); err != nil {
		return exitError, err
	}
	fmt.Printf("lesson %s: promoted to %s on %s\n", l.ID, l.Target, l.Promotion.PromotedAt)
	fmt.Printf("result: ok (exit %d)\n", exitOK)
	return exitOK, nil
}

type scorecardFile struct {
	Scope       string         `json:"scope"`
	Since       string         `json:"since"`
	Corrections map[string]int `json:"corrections"`
	Gates       map[string]int `json:"gates"`
	SleepCalls  int            `json:"sleep_calls"`
}

func recurrences(card scorecardFile, class string) (int, bool) {
	switch {
	case class == "sleep":
		return card.SleepCalls, true
	case strings.HasPrefix(class, "gate:"):
		return card.Gates[strings.TrimPrefix(class, "gate:")], true
	}
	count, ok := card.Corrections[class]
	return count, ok || card.Corrections != nil
}

// cmdReview judges promoted lessons against a scorecard whose window starts
// on or after the promotion: a recurring class means the lesson is not
// working; a quiet class past max_age_days is a retirement candidate.
func cmdReview(opt options) (int, error) {
	p, err := openProject(opt)
	if err != nil {
		return exitError, err
	}
	data, err := os.ReadFile(opt.scorecard)
	if err != nil {
		return exitError, fail("cannot read scorecard %s", opt.scorecard)
	}
	var card scorecardFile
	if err := json.Unmarshal(data, &card); err != nil {
		return exitError, fail("%s is not a session-mine scorecard JSON", opt.scorecard)
	}
	if resolve(card.Scope) != resolve(p.top) {
		return exitError, fail("scorecard scope %q is not this project %s; run session-mine scorecard --workspace %s", card.Scope, p.top, p.top)
	}
	since, err := time.Parse(dateLayout, card.Since)
	if err != nil {
		return exitError, fail("the scorecard needs a --since window to judge lessons")
	}
	now, _ := time.Parse(dateLayout, today(opt))
	ls, err := all(p)
	if err != nil {
		return exitError, err
	}
	code := exitOK
	for _, l := range ls {
		switch l.Status {
		case "retired":
			continue
		case "candidate", "proven":
			created, _ := time.Parse(time.RFC3339, l.Created)
			if now.Sub(created) > time.Duration(l.MaxAgeDays)*24*time.Hour {
				code = exitAction
				fmt.Printf("%s: expired %s; promote it or retire it\n", l.ID, l.Status)
			} else {
				fmt.Printf("%s: %s, not yet promoted\n", l.ID, l.Status)
			}
			continue
		}
		if l.Surface == "upstream" {
			fmt.Printf("%s: upstream draft; run export to share it\n", l.ID)
			continue
		}
		promoted, err := time.Parse(dateLayout, l.Promotion.PromotedAt)
		if err != nil {
			return exitError, fail("lesson %s has no promotion date", l.ID)
		}
		count, known := recurrences(card, l.Class)
		switch {
		case since.Before(promoted):
			fmt.Printf("%s: unknown; the scorecard window starts %s, before promotion on %s\n", l.ID, card.Since, l.Promotion.PromotedAt)
		case !known:
			fmt.Printf("%s: unknown; the scorecard has no %q signal\n", l.ID, l.Class)
		case count > 0:
			code = exitAction
			fmt.Printf("%s: recurring; %s seen %d time(s) since %s, so the lesson is not working\n", l.ID, l.Class, count, card.Since)
		case now.Sub(promoted) > time.Duration(l.MaxAgeDays)*24*time.Hour:
			code = exitAction
			fmt.Printf("%s: stale; %s quiet for over %d days; retire it, or delete its prose if a check covers it\n", l.ID, l.Class, l.MaxAgeDays)
		default:
			fmt.Printf("%s: holding; %s not seen since %s\n", l.ID, l.Class, card.Since)
		}
	}
	if code == exitOK {
		fmt.Printf("result: ok (exit %d)\n", exitOK)
	} else {
		fmt.Printf("result: action (exit %d); bring recurring, stale, and expired lessons to the user\n", exitAction)
	}
	return code, nil
}

func cmdRetire(opt options) (int, error) {
	p, err := openProject(opt)
	if err != nil {
		return exitError, err
	}
	l, err := load(p, opt.id)
	if err != nil {
		return exitError, err
	}
	l.Status, l.RetiredWhy = "retired", opt.why
	l.History = append(l.History, historyEntry{At: stamp(), Action: "retire", Note: opt.why})
	if err := save(p, l, false); err != nil {
		return exitError, err
	}
	fmt.Printf("lesson %s: retired (%s)\n", l.ID, opt.why)
	fmt.Printf("result: ok (exit %d)\n", exitOK)
	return exitOK, nil
}

// reAbsPath matches absolute filesystem paths of two or more segments.
var reAbsPath = regexp.MustCompile(`/[A-Za-z0-9._~@%+-]+(?:/[A-Za-z0-9._~@%+-]+)+`)

// scrub replaces absolute paths, which can name a client, a user, or a
// machine, before text leaves the machine.
func scrub(text string) string { return reAbsPath.ReplaceAllString(text, "<path>") }

// cmdExport prints what may leave the machine for the upstream loop: the
// lesson, its class and surface, the check command, and an installed-ab case
// draft. Project paths, sources, and commit IDs stay behind.
func cmdExport(opt options) (int, error) {
	p, err := openProject(opt)
	if err != nil {
		return exitError, err
	}
	l, err := load(p, opt.id)
	if err != nil {
		return exitError, err
	}
	draft := map[string]any{
		"schema_version": schemaV1,
		"id":             l.ID,
		"class":          scrub(l.Class),
		"lesson":         scrub(l.Lesson),
		"surface":        l.Surface,
		"status":         l.Status,
		"evidence":       l.Evidence,
		"occurrences":    len(l.Sources),
		"case": map[string]any{
			"id":   "lesson-" + l.ID,
			"kind": "workflow",
			"task": "(write a task that reproduces the mistake without naming the lesson)",
			"files": map[string]string{
				"(path)": "(minimal fixture that reproduces the mistake)",
			},
			"note": "add to evals/studies/installed-ab/cases.json with an oracleProofs entry before any A/B run",
		},
	}
	if l.Proof != nil && len(l.Proof.Command) > 0 {
		command := make([]string, len(l.Proof.Command))
		for i, arg := range l.Proof.Command {
			command[i] = scrub(arg)
		}
		draft["check"] = command
		draft["case"].(map[string]any)["oracle_command"] = command
	}
	// Keep <path> readable; the draft is for people, not an HTML page.
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(draft); err != nil {
		return exitError, fail("cannot encode export")
	}
	return exitOK, nil
}
