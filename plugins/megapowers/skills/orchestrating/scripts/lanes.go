package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	exitOK       = 0
	exitMismatch = 3
	exitError    = 125
	baseFile     = "megapowers-lane-base"
)

const usage = `lanes: isolated worktrees for parallel writing lanes

Usage:
  go run lanes.go <command> [--repo DIR] [--root DIR] [flags]

Commands:
  add     --name NAME [--base REF]   detached worktree at REF (default HEAD)
  list                               lanes with base and changed-file count
  diff    --name NAME                binary patch of the lane against its base
  check                              per lane: clean, applied, or conflicts; files
                                     touched by more than one lane
  apply   --name NAME                apply the lane patch to the lead worktree
  remove  --name NAME [--discard]    remove the worktree; refuses unapplied work
                                     and ignored files unless --discard

Lanes never touch the lead branch. Integration is a patch the lead applies,
so the lead stays the single writer and lanes need no commit signing. Lane
commits are allowed and are included in the patch.

The root defaults to $TMPDIR/megapowers-lanes/<repo>-<hash>. Names match
[a-z0-9][a-z0-9-]{0,62}.

Exit codes: 0 ok, 3 overlap, conflict, or unapplied work, 125 tool or usage
error.
`

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type options struct {
	command, repo, root, name, base string
	discard                         bool
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
	if isHelp(args[0]) {
		fmt.Print(usage)
		return exitOK
	}
	opt, err := parse(args)
	if err != nil {
		return report(err)
	}
	var code int
	switch opt.command {
	case "add":
		code, err = cmdAdd(opt)
	case "list":
		code, err = cmdList(opt)
	case "diff":
		code, err = cmdDiff(opt)
	case "check":
		code, err = cmdCheck(opt)
	case "apply":
		code, err = cmdApply(opt)
	case "remove":
		code, err = cmdRemove(opt)
	}
	if err != nil {
		return report(err)
	}
	return code
}

func report(err error) int {
	fmt.Fprintf(os.Stderr, "lanes: %v\nresult: error (exit %d)\n", err, exitError)
	return exitError
}

func isHelp(arg string) bool { return arg == "--help" || arg == "-h" || arg == "help" }

var allowed = map[string][]string{
	"add":    {"--repo", "--root", "--name", "--base"},
	"list":   {"--repo", "--root"},
	"diff":   {"--repo", "--root", "--name"},
	"check":  {"--repo", "--root"},
	"apply":  {"--repo", "--root", "--name"},
	"remove": {"--repo", "--root", "--name", "--discard"},
}

func parse(args []string) (options, error) {
	opt := options{command: args[0], repo: ".", base: "HEAD"}
	flags, ok := allowed[opt.command]
	if !ok {
		return opt, fail("unknown command: %s (see --help)", opt.command)
	}
	seen := map[string]bool{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if !contains(flags, arg) {
			return opt, fail("%s: unknown arg: %s", opt.command, arg)
		}
		if seen[arg] {
			return opt, fail("%s may be provided only once", arg)
		}
		seen[arg] = true
		if arg == "--discard" {
			opt.discard = true
			continue
		}
		if i+1 >= len(args) {
			return opt, fail("%s requires a value", arg)
		}
		i++
		switch arg {
		case "--repo":
			opt.repo = args[i]
		case "--root":
			opt.root = args[i]
		case "--name":
			opt.name = args[i]
		case "--base":
			opt.base = args[i]
		}
	}
	if contains(flags, "--name") && !namePattern.MatchString(opt.name) {
		return opt, fail("--name must match [a-z0-9][a-z0-9-]{0,62}")
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

func git(dir string, stdin []byte, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	if err := cmd.Run(); err != nil {
		return stdout.String(), fail("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

type repoState struct {
	lead string // lead worktree root
	root string // directory that holds lane worktrees
}

func inspect(opt options) (repoState, error) {
	var st repoState
	top, err := git(opt.repo, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return st, fail("not inside a git worktree: %s", opt.repo)
	}
	st.lead = filepath.Clean(strings.TrimSpace(top))
	if opt.root != "" {
		abs, err := filepath.Abs(opt.root)
		if err != nil {
			return st, fail("cannot resolve --root %s", opt.root)
		}
		st.root = abs
		return st, nil
	}
	common, err := git(st.lead, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return st, err
	}
	// Key the default root on the shared repository so every worktree of one
	// checkout sees the same lanes, and two clones never collide.
	sum := sha256.Sum256([]byte(filepath.Clean(strings.TrimSpace(common))))
	st.root = filepath.Join(os.TempDir(), "megapowers-lanes", filepath.Base(st.lead)+"-"+hex.EncodeToString(sum[:4]))
	return st, nil
}

type lane struct {
	name, path, base string
}

// lanes lists worktrees registered with Git that live directly under the root
// and carry a recorded base, so unrelated worktrees are never touched.
func lanes(st repoState) ([]lane, error) {
	out, err := git(st.lead, nil, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	root, err := filepath.EvalSymlinks(st.root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fail("cannot resolve %s", st.root)
	}
	var found []lane
	for _, line := range strings.Split(out, "\n") {
		path, ok := strings.CutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || filepath.Dir(resolved) != root {
			continue
		}
		gitDir, err := git(resolved, nil, "rev-parse", "--absolute-git-dir")
		if err != nil {
			continue
		}
		base, err := os.ReadFile(filepath.Join(strings.TrimSpace(gitDir), baseFile))
		if err != nil {
			continue
		}
		found = append(found, lane{name: filepath.Base(resolved), path: resolved, base: strings.TrimSpace(string(base))})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].name < found[j].name })
	return found, nil
}

func findLane(st repoState, name string) (lane, error) {
	all, err := lanes(st)
	if err != nil {
		return lane{}, err
	}
	for _, l := range all {
		if l.name == name {
			return l, nil
		}
	}
	return lane{}, fail("lane %q not found under %s", name, st.root)
}

func cmdAdd(opt options) (int, error) {
	st, err := inspect(opt)
	if err != nil {
		return exitError, err
	}
	base, err := git(st.lead, nil, "rev-parse", "--verify", "-q", opt.base+"^{commit}")
	if err != nil {
		return exitError, fail("--base %s is not a commit", opt.base)
	}
	base = strings.TrimSpace(base)
	path := filepath.Join(st.root, opt.name)
	if _, err := os.Lstat(path); err == nil {
		return exitError, fail("lane %q already exists at %s", opt.name, path)
	}
	if err := os.MkdirAll(st.root, 0o700); err != nil {
		return exitError, fail("cannot create %s: %v", st.root, err)
	}
	if _, err := git(st.lead, nil, "worktree", "add", "--detach", "-q", path, base); err != nil {
		return exitError, err
	}
	gitDir, err := git(path, nil, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return exitError, err
	}
	if err := os.WriteFile(filepath.Join(strings.TrimSpace(gitDir), baseFile), []byte(base+"\n"), 0o600); err != nil {
		return exitError, fail("cannot record lane base: %v", err)
	}
	fmt.Printf("lane %s: path %s base %s\n", opt.name, path, base)
	if status, err := git(st.lead, nil, "status", "--porcelain"); err == nil && strings.TrimSpace(status) != "" {
		fmt.Printf("note: the lead worktree has uncommitted changes; lane %s starts from %s without them\n", opt.name, base)
	}
	fmt.Printf("result: ok (exit %d)\n", exitOK)
	return exitOK, nil
}

// patch returns the lane's changes against its base: commits, staged and
// unstaged edits, and untracked files that are not ignored.
func patch(l lane) ([]byte, error) {
	if _, err := git(l.path, nil, "add", "--all", "--intent-to-add"); err != nil {
		return nil, err
	}
	// Pin the a/ and b/ prefixes so diff.noprefix or mnemonicPrefix in user
	// config cannot produce a patch that git apply rejects.
	out, err := git(l.path, nil, "diff", "--binary", "--no-color", "--no-ext-diff", "--no-textconv", "--src-prefix=a/", "--dst-prefix=b/", l.base)
	return []byte(out), err
}

// touched lists every path the lane changes against its base. It asks Git
// with -z instead of parsing patch headers, which Git quotes for names such
// as café.txt; --no-renames reports both sides of a rename.
func touched(l lane) []string {
	out, err := git(l.path, nil, "diff", "--no-renames", "--name-only", "-z", l.base)
	if err != nil {
		return nil
	}
	var files []string
	for _, name := range strings.Split(out, "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files
}

// ignored lists files Git ignores in the lane. They are not in the patch, so
// removing the worktree would delete their only copy.
func ignored(l lane) ([]string, error) {
	// Explicit flags override status.showUntrackedFiles, which can hide
	// ignored files too.
	out, err := git(l.path, nil, "status", "--porcelain", "-z", "--ignored=matching", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range strings.Split(out, "\x00") {
		if name, ok := strings.CutPrefix(entry, "!! "); ok {
			files = append(files, name)
		}
	}
	return files, nil
}

// state reports whether a lane patch applies to the lead worktree, is already
// present there, or conflicts.
func state(st repoState, diff []byte) string {
	if len(bytes.TrimSpace(diff)) == 0 {
		return "empty"
	}
	if _, err := git(st.lead, diff, "apply", "--check", "--binary", "-"); err == nil {
		return "clean"
	}
	if _, err := git(st.lead, diff, "apply", "--check", "--binary", "--reverse", "-"); err == nil {
		return "applied"
	}
	return "conflicts"
}

func cmdList(opt options) (int, error) {
	st, err := inspect(opt)
	if err != nil {
		return exitError, err
	}
	all, err := lanes(st)
	if err != nil {
		return exitError, err
	}
	fmt.Printf("lanes: %d under %s\n", len(all), st.root)
	for _, l := range all {
		// patch also marks untracked files intent-to-add, so touched sees them.
		_, err := patch(l)
		if err != nil {
			return exitError, err
		}
		fmt.Printf("- %s: base %s, %d files changed, %s\n", l.name, l.base, len(touched(l)), l.path)
	}
	fmt.Printf("result: ok (exit %d)\n", exitOK)
	return exitOK, nil
}

func cmdDiff(opt options) (int, error) {
	st, err := inspect(opt)
	if err != nil {
		return exitError, err
	}
	l, err := findLane(st, opt.name)
	if err != nil {
		return exitError, err
	}
	diff, err := patch(l)
	if err != nil {
		return exitError, err
	}
	os.Stdout.Write(diff)
	return exitOK, nil
}

func cmdCheck(opt options) (int, error) {
	st, err := inspect(opt)
	if err != nil {
		return exitError, err
	}
	all, err := lanes(st)
	if err != nil {
		return exitError, err
	}
	code := exitOK
	owners := map[string][]string{}
	for _, l := range all {
		diff, err := patch(l)
		if err != nil {
			return exitError, err
		}
		files := touched(l)
		s := state(st, diff)
		if s == "conflicts" {
			code = exitMismatch
		}
		fmt.Printf("%s: %s (%s)\n", l.name, s, strings.Join(files, ", "))
		// An applied lane no longer owns anything still in flight.
		if s == "applied" {
			continue
		}
		for _, file := range files {
			owners[file] = append(owners[file], l.name)
		}
	}
	var overlap []string
	for file, names := range owners {
		if len(names) > 1 {
			overlap = append(overlap, fmt.Sprintf("%s (%s)", file, strings.Join(names, ", ")))
		}
	}
	sort.Strings(overlap)
	if len(overlap) == 0 {
		fmt.Println("overlap: none")
	} else {
		code = exitMismatch
		fmt.Println("overlap: " + strings.Join(overlap, "; "))
	}
	if code == exitOK {
		fmt.Printf("result: ok (exit %d)\n", exitOK)
	} else {
		fmt.Printf("result: mismatch (exit %d); reassign overlapping files to one lane and rebase conflicting lanes before applying\n", exitMismatch)
	}
	return code, nil
}

func cmdApply(opt options) (int, error) {
	st, err := inspect(opt)
	if err != nil {
		return exitError, err
	}
	l, err := findLane(st, opt.name)
	if err != nil {
		return exitError, err
	}
	diff, err := patch(l)
	if err != nil {
		return exitError, err
	}
	switch s := state(st, diff); s {
	case "empty", "applied":
		fmt.Printf("lane %s: %s; nothing to apply\n", l.name, s)
		fmt.Printf("result: ok (exit %d)\n", exitOK)
		return exitOK, nil
	case "conflicts":
		fmt.Printf("lane %s: conflicts with the lead worktree (%s)\n", l.name, strings.Join(touched(l), ", "))
		fmt.Printf("result: mismatch (exit %d); resolve in the lane or apply by hand\n", exitMismatch)
		return exitMismatch, nil
	}
	if _, err := git(st.lead, diff, "apply", "--binary", "-"); err != nil {
		return exitError, err
	}
	fmt.Printf("lane %s: applied to %s (%s); nothing was committed\n", l.name, st.lead, strings.Join(touched(l), ", "))
	fmt.Printf("result: ok (exit %d)\n", exitOK)
	return exitOK, nil
}

func cmdRemove(opt options) (int, error) {
	st, err := inspect(opt)
	if err != nil {
		return exitError, err
	}
	l, err := findLane(st, opt.name)
	if err != nil {
		return exitError, err
	}
	diff, err := patch(l)
	if err != nil {
		return exitError, err
	}
	if s := state(st, diff); !opt.discard && s != "empty" && s != "applied" {
		fmt.Printf("lane %s: unapplied changes (%s)\n", l.name, strings.Join(touched(l), ", "))
		fmt.Printf("result: mismatch (exit %d); apply the lane, or pass --discard to drop its work\n", exitMismatch)
		return exitMismatch, nil
	}
	if !opt.discard {
		kept, err := ignored(l)
		if err != nil {
			return exitError, err
		}
		if len(kept) > 0 {
			fmt.Printf("lane %s: ignored files would be deleted (%s)\n", l.name, strings.Join(kept, ", "))
			fmt.Printf("result: mismatch (exit %d); copy what you need, or pass --discard to drop them\n", exitMismatch)
			return exitMismatch, nil
		}
	}
	if _, err := git(st.lead, nil, "worktree", "remove", "--force", l.path); err != nil {
		return exitError, err
	}
	// Drop the root and its shared parent once empty; os.Remove refuses a
	// non-empty directory, so other lanes and repositories are untouched.
	if os.Remove(st.root) == nil {
		_ = os.Remove(filepath.Dir(st.root))
	}
	fmt.Printf("lane %s: removed %s\n", l.name, l.path)
	fmt.Printf("result: ok (exit %d)\n", exitOK)
	return exitOK, nil
}
