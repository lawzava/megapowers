package maintain

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runRelease(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	usage := func() int {
		fmt.Fprintln(stderr, "usage: release.sh <X.Y.Z>")
		return 2
	}
	if len(args) != 1 || !validReleaseVersion(args[0]) {
		return usage()
	}
	version := args[0]
	for _, required := range []string{"git", "go"} {
		if !executable(required) {
			fmt.Fprintf(stderr, "release.sh: %s is required\n", required)
			return 2
		}
	}
	changelog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		fmt.Fprintln(stderr, "release.sh: cannot read CHANGELOG.md")
		return 2
	}
	wantedHeading := "## " + version + " - "
	if !bytes.Contains(changelog, []byte(wantedHeading)) {
		fmt.Fprintf(stderr, "release.sh: CHANGELOG.md has no '## %s - ' entry; write it first\n", version)
		return 2
	}
	latest := newestChangelogVersion(changelog)
	if latest != version {
		fmt.Fprintf(stderr, "release.sh: %s is not the newest CHANGELOG.md version (%s)\n", version, latest)
		return 2
	}
	for _, rel := range []string{
		"plugins/megapowers/.claude-plugin/plugin.json",
		"plugins/megapowers/.codex-plugin/plugin.json",
	} {
		declared, err := manifestVersion(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || declared != version {
			fmt.Fprintf(stderr, "release.sh: %s must already declare version %s\n", rel, version)
			return 2
		}
	}
	if _, err := output(ctx, root, nil, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		fmt.Fprintln(stderr, "release.sh: candidate is not a Git checkout")
		return 2
	}
	if err := exec.CommandContext(ctx, "git", "-C", root, "show-ref", "--verify", "--quiet", "refs/tags/v"+version).Run(); err == nil {
		fmt.Fprintf(stderr, "release.sh: tag v%s already exists\n", version)
		return 2
	} else if code := exitCode(err); code != 1 {
		fmt.Fprintln(stderr, "release.sh: cannot inspect existing tags")
		return 2
	}
	status, err := output(ctx, root, nil, "git", "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil || len(status) != 0 {
		fmt.Fprintln(stderr, "release.sh: candidate must be a clean HEAD before validation")
		return 2
	}
	flags, err := output(ctx, root, nil, "git", "ls-files", "-v")
	if err != nil {
		fmt.Fprintln(stderr, "release.sh: cannot inspect candidate index")
		return 2
	}
	for _, line := range strings.Split(string(flags), "\n") {
		if len(line) > 1 && line[1] == ' ' && (line[0] >= 'a' && line[0] <= 'z' || line[0] == 'S') {
			fmt.Fprintln(stderr, "release.sh: candidate index hides tracked paths with assume-unchanged or skip-worktree")
			return 2
		}
	}

	if err := command(ctx, root, io.Discard, stderr, nil, "go", "run", "./evals/studies/installed-ab", "--hash-plugin", "--repo", root); err != nil {
		fmt.Fprintln(stderr, "release.sh: candidate plugin tree cannot be verified")
		return 2
	}
	tmp, err := privateTempDir("megapowers-release")
	if err != nil {
		fmt.Fprintln(stderr, "release.sh: cannot create temporary result directory")
		return 2
	}
	defer os.RemoveAll(tmp)
	results := filepath.Join(tmp, "results.jsonl")
	gates := [][]string{
		{filepath.Join(root, "scripts", "validate.sh")},
		{filepath.Join(root, "evals", "run-all.sh"), "--json", results},
		{"go", "run", "./evals/score.go", "--strict", results},
		{filepath.Join(root, "scripts", "check-freshness.sh")},
	}
	for _, gate := range gates {
		if err := command(ctx, root, stdout, stderr, nil, gate[0], gate[1:]...); err != nil {
			return exitCode(err)
		}
	}
	fmt.Fprintf(stdout, "release.sh: validated already-stamped %s; no tag or publish action performed\n", version)
	return 0
}

func newestChangelogVersion(data []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "##" {
			return fields[1]
		}
	}
	return ""
}

func manifestVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var manifest struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", err
	}
	if manifest.Name != "megapowers" || manifest.Version == "" {
		return "", fmt.Errorf("name or version is invalid")
	}
	return manifest.Version, nil
}
