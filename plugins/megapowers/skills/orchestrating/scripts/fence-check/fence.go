// fence-check checks arbitrary Git worker checkouts without changing their index.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const usage = `fence-check: scope fences and lead-checkout drift

  check    --repo DIR --base REF --head REF_OR_CHECKOUT --fence PATTERN [--fence PATTERN ...]
  snapshot --repo DIR --out FILE
  leak     --repo DIR --since FILE

Fences are repo-relative exact files, dir/ prefixes, or patterns where * matches
any characters including /. All other characters are literal. Quote patterns.
Checkout checks include committed, staged, unstaged and non-ignored untracked
paths; revision checks compare only the two trees. Renames check both endpoints.
Record the immutable dispatch commit ID for --base; refs resolve in --repo at
check time. An ambiguous --head directory/ref needs an absolute path or tree ID.
snapshot saves HEAD, index and worktree trees to a new private file outside the
checkout. leak reports all path changes since that baseline, not their author.
Ignored untracked files, empty directories and Git metadata are not checked.
Sparse checkouts, unmerged indexes and submodules are unsupported (fail closed).
No real index, branch or worktree is changed. Snapshot trees write local Git
objects and may run configured Git clean filters; use only a trusted repository.
Exit codes: 0 pass, 3 violations, 125 usage or tool error. With go run a nonzero
code becomes 1; the final result line preserves the tool's code.
`

type patterns []string

func (p *patterns) String() string     { return strings.Join(*p, ", ") }
func (p *patterns) Set(s string) error { *p = append(*p, s); return nil }

type baseline struct {
	Version  int    `json:"version"`
	Repo     string `json:"repo"`
	Head     string `json:"head"`
	Index    string `json:"index"`
	Worktree string `json:"worktree"`
}

var objectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	code, err := execute(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fence-check: %v\nresult: error (exit 125)\n", err)
		return 125
	}
	return code
}

func execute(args []string) (int, error) {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "help") {
		fmt.Print(usage)
		return 0, nil
	}
	if len(args) == 0 {
		return 125, errors.New(usage)
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	repo := fs.String("repo", ".", "repository checkout")
	var base, head, out, since string
	var fences patterns
	switch args[0] {
	case "check":
		fs.StringVar(&base, "base", "", "base revision")
		fs.StringVar(&head, "head", "", "head revision or checkout path")
		fs.Var(&fences, "fence", "allowed path pattern (repeatable)")
	case "snapshot":
		fs.StringVar(&out, "out", "", "new baseline file outside checkout")
	case "leak":
		fs.StringVar(&since, "since", "", "pre-dispatch baseline file")
	default:
		return 125, fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
	if err := fs.Parse(args[1:]); err != nil {
		return 125, err
	}
	if fs.NArg() != 0 {
		return 125, errors.New("unexpected positional arguments")
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return 125, err
	}
	switch args[0] {
	case "check":
		if base == "" || head == "" || len(fences) == 0 {
			return 125, errors.New("check requires --base, --head and --fence")
		}
		var matchers []*regexp.Regexp
		for _, pattern := range fences {
			m, err := compileFence(pattern)
			if err != nil {
				return 125, err
			}
			matchers = append(matchers, m)
		}
		// Resolve symbolic refs once in the lead, never against a worker's HEAD.
		baseTree, err := tree(root, base)
		if err != nil {
			return 125, err
		}
		// An existing directory means a checkout, never a lane-root convention.
		if info, err := os.Stat(head); err == nil && info.IsDir() {
			if _, err := tree(root, head); err == nil {
				return 125, errors.New("ambiguous --head is both directory and revision; use an absolute checkout path or immutable revision ID")
			}
			worker, err := repoRoot(head)
			if err != nil {
				return 125, err
			}
			_, err = tree(worker, baseTree)
			if err != nil {
				return 125, err
			}
			state, err := capture(worker)
			if err != nil {
				return 125, err
			}
			paths, err := differences(worker, []string{baseTree, baseTree, baseTree}, []string{state.Head, state.Index, state.Worktree})
			if err != nil {
				return 125, err
			}
			return report("outside fence", outside(paths, matchers)), nil
		}
		h, err := tree(root, head)
		if err != nil {
			return 125, err
		}
		paths, err := differences(root, []string{baseTree}, []string{h})
		if err != nil {
			return 125, err
		}
		return report("outside fence", outside(paths, matchers)), nil
	case "snapshot":
		if out == "" {
			return 125, errors.New("snapshot requires --out")
		}
		abs, err := filepath.Abs(out)
		if err != nil {
			return 125, err
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
		if err != nil {
			return 125, err
		}
		abs = filepath.Join(parent, filepath.Base(abs))
		if rel, err := filepath.Rel(root, abs); err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return 125, errors.New("baseline must be outside the checkout")
		}
		state, err := capture(root)
		if err != nil {
			return 125, err
		}
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return 125, err
		}
		file, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return 125, err
		}
		_, writeErr := file.Write(append(data, '\n'))
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return 125, err
		}
		fmt.Printf("baseline: %s\nresult: pass (exit 0)\n", abs)
		return 0, nil
	case "leak":
		if since == "" {
			return 125, errors.New("leak requires --since")
		}
		data, err := os.ReadFile(since)
		if err != nil {
			return 125, err
		}
		var before baseline
		if err := json.Unmarshal(data, &before); err != nil {
			return 125, err
		}
		if before.Version != 1 || before.Repo != root || !objectID.MatchString(before.Head) || !objectID.MatchString(before.Index) || !objectID.MatchString(before.Worktree) {
			return 125, errors.New("invalid baseline or wrong checkout")
		}
		after, err := capture(root)
		if err != nil {
			return 125, err
		}
		paths, err := differences(root, []string{before.Head, before.Index, before.Worktree}, []string{after.Head, after.Index, after.Worktree})
		if err != nil {
			return 125, err
		}
		return report("lead drift", paths), nil
	}
	panic("unreachable")
}

func git(repo, index string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	// Hooks often export repository-local variables; -C does not override them.
	// Never let that ambient state redirect a check of another worktree.
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE", "GIT_PREFIX", "GIT_SHALLOW_FILE", "GIT_GRAFT_FILE", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT", "GIT_IMPLICIT_WORK_TREE", "GIT_NO_REPLACE_OBJECTS", "GIT_REPLACE_REF_BASE":
			continue
		}
		if strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	if index != "" {
		cmd.Env = append(cmd.Env, "GIT_INDEX_FILE="+index)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func repoRoot(dir string) (string, error) {
	out, err := git(dir, "", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(strings.TrimSpace(out))
}

func tree(repo, rev string) (string, error) {
	out, err := git(repo, "", "rev-parse", "--verify", "--end-of-options", rev+"^{tree}")
	return strings.TrimSpace(out), err
}

// capture uses disposable indexes for write-tree and add; even Git's cache-tree
// extension updates must not touch the user's real index.
func capture(repo string) (baseline, error) {
	s := baseline{Version: 1, Repo: repo}
	var err error
	if s.Head, err = tree(repo, "HEAD"); err != nil {
		return s, err
	}
	if sparse, _ := git(repo, "", "config", "--bool", "core.sparseCheckout"); strings.TrimSpace(sparse) == "true" {
		return s, errors.New("sparse checkouts are unsupported")
	}
	entries, err := git(repo, "", "ls-files", "--stage", "-z")
	if err != nil {
		return s, err
	}
	for _, entry := range strings.Split(entries, "\x00") {
		if strings.HasPrefix(entry, "160000 ") {
			return s, errors.New("submodules are unsupported; check each repository separately")
		}
	}
	dir, err := os.MkdirTemp("", "megapowers-fence-*")
	if err != nil {
		return s, err
	}
	defer os.RemoveAll(dir)
	index := filepath.Join(dir, "index")
	indexPath, err := git(repo, "", "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(strings.TrimSpace(indexPath))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return s, err
	}
	if err == nil {
		if err := os.WriteFile(index, data, 0600); err != nil {
			return s, err
		}
	} else if _, err := git(repo, index, "read-tree", "--empty"); err != nil {
		return s, err
	}
	if out, err := git(repo, index, "write-tree"); err != nil {
		return s, err
	} else {
		s.Index = strings.TrimSpace(out)
	}
	// Remove skip-worktree and assume-unchanged state by starting a new index.
	if err := os.Remove(index); err != nil {
		return s, err
	}
	if _, err := git(repo, index, "read-tree", s.Head); err != nil {
		return s, err
	}
	if _, err := git(repo, index, "add", "--all", "--", "."); err != nil {
		return s, err
	}
	out, err := git(repo, index, "write-tree")
	s.Worktree = strings.TrimSpace(out)
	return s, err
}

func differences(repo string, before, after []string) ([]string, error) {
	set := map[string]bool{}
	for i, b := range before {
		out, err := git(repo, "", "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", b, after[i], "--")
		if err != nil {
			return nil, err
		}
		for _, path := range strings.Split(out, "\x00") {
			if path != "" {
				set[path] = true
			}
		}
	}
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func compileFence(pattern string) (*regexp.Regexp, error) {
	invalid := pattern == "" || strings.HasPrefix(pattern, "/") || strings.ContainsAny(pattern, "\\\x00")
	for _, part := range strings.Split(strings.TrimSuffix(pattern, "/"), "/") {
		invalid = invalid || part == "" || part == "." || part == ".."
	}
	if invalid {
		return nil, fmt.Errorf("invalid repo-relative fence %q", pattern)
	}
	quoted := strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*")
	if strings.HasSuffix(pattern, "/") {
		quoted += ".+"
	}
	return regexp.Compile("(?s)^" + quoted + "$")
}

func outside(paths []string, matchers []*regexp.Regexp) []string {
	var violations []string
	for _, path := range paths {
		allowed := false
		for _, matcher := range matchers {
			allowed = allowed || matcher.MatchString(path)
		}
		if !allowed {
			violations = append(violations, path)
		}
	}
	return violations
}

func report(label string, paths []string) int {
	for _, path := range paths {
		fmt.Printf("%s: %q\n", label, path)
	}
	if len(paths) != 0 {
		fmt.Printf("result: fail (exit 3); %d paths\n", len(paths))
		return 3
	}
	fmt.Println("result: pass (exit 0); 0 paths")
	return 0
}
