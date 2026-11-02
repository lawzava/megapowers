package contracts

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func reviewCLI(t *testing.T, f *reviewFixture, dir string, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.tool, args...)
	cmd.Dir = dir
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return stdout.String(), stderr.String(), exit.ExitCode()
		}
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), 0
}

func TestReviewUsage(t *testing.T) {
	f := newReviewFixture(t)
	var usage string
	for _, args := range [][]string{
		{"--help"}, {"-h"}, {"help"},
		{"inspect", "--help"}, {"inspect", "-h"},
		{"review", "--help"}, {"review", "-h"},
		{"snapshot", "--help"}, {"snapshot", "-h"},
		{"delta", "--help"}, {"delta", "-h"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, code := reviewCLI(t, f, f.root, args...)
			if code != 0 || stderr != "" {
				t.Fatalf("help exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			for _, want := range []string{
				"inspect", "review", "snapshot", "delta", "-file", "-base", "-head",
				"-provider", "-provider-command", "-intent", "-author",
				"-out", "-approve-external", "-retain-transcript", "-max-files-per-chunk",
				"-preflight", "-preflight-timeout", "-since",
				"current directory", "Git work tree", "regular file", "$TMPDIR/<name>",
				"already exist", "symlink", "outside", "contain",
			} {
				if !strings.Contains(stdout, want) {
					t.Errorf("usage lacks %q:\n%s", want, stdout)
				}
			}
			if strings.Contains(stdout, "provider-env") {
				t.Errorf("usage still documents the removed -provider-env flag:\n%s", stdout)
			}
			if usage == "" {
				usage = stdout
			}
		})
	}
	stdout, stderr, code := reviewCLI(t, f, f.root)
	if code != 2 || stdout != "" || usage == "" || !strings.Contains(stderr, usage) {
		t.Errorf("no arguments: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	f.noCalls()
}

func TestReviewAnyFileUnderTMPDIR(t *testing.T) {
	f := newReviewFixture(t)
	scratch := filepath.Join(f.root, "scratch")
	if err := os.Mkdir(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", scratch)
	for _, name := range []string{"package.md", "notes.txt", "extensionless"} {
		path := filepath.Join(scratch, name)
		f.write(path, "Review the document for missing acceptance criteria.\n")
		info := f.inspect(f.command, "--file", path)
		paths, _ := json.Marshal(info["paths"])
		if string(paths) != `["$TMPDIR/`+name+`"]` {
			t.Fatalf("unexpected disclosure paths: %s", paths)
		}
		f.call("", f.review(info["approval_token"].(string), f.command, "--file", path)...)
		if !strings.Contains(f.recordText("input"), "missing acceptance criteria") {
			t.Fatal("temporary review package did not reach the reviewer")
		}
	}
	f.clearCalls()
	assertReviewFileGuards(t, f, scratch)
}

func assertReviewFileGuards(t *testing.T, f *reviewFixture, dir string) {
	t.Helper()
	for _, tc := range []struct{ name, content, needle string }{
		{".env", "plain text\n", "secret-like path"},
		{"secret.md", "-----BEGIN PRIVATE KEY-----\n", "likely secret content"},
		{"binary.md", "text\x00binary\n", "binary"},
		{"large.md", strings.Repeat("a", 512*1024+1), "size limit"},
		{"package-limit.md", strings.Repeat("\t", 512*1024), "package exceeds"},
	} {
		f.write(filepath.Join(dir, tc.name), tc.content)
		f.call(tc.needle, "inspect", "--file", filepath.Join(dir, tc.name), "--provider", "vendor-a", "--provider-command", f.command)
	}
	f.write(filepath.Join(dir, "regular.md"), "plain text\n")
	if err := os.Symlink("regular.md", filepath.Join(dir, "alias.md")); err != nil {
		t.Fatal(err)
	}
	f.call("symlink", "inspect", "--file", filepath.Join(dir, "alias.md"), "--provider", "vendor-a", "--provider-command", f.command)
	f.call("non-regular", "inspect", "--file", dir, "--provider", "vendor-a", "--provider-command", f.command)
	f.noCalls()
}

func TestReviewWithoutGit(t *testing.T) {
	f := newReviewFixture(t)
	t.Setenv("GIT_CEILING_DIRECTORIES", f.root)
	f.repo = filepath.Join(f.root, "documents")
	scratch := filepath.Join(f.root, "scratch")
	for _, dir := range []string{f.repo, scratch} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TMPDIR", scratch)
	f.write(filepath.Join(f.repo, "package.md"), "Review this document.\n")
	f.write(filepath.Join(scratch, "temporary.md"), "Review this temporary document.\n")
	for _, path := range []string{"package.md", filepath.Join(scratch, "temporary.md")} {
		info := f.inspect(f.command, "--file", path)
		result := f.call("", f.review(info["approval_token"].(string), f.command, "--file", path)...)
		_, receipt, ok := strings.Cut(result, "advisory receipt: ")
		if !ok || !strings.HasPrefix(strings.TrimSpace(receipt), scratch+string(filepath.Separator)) {
			t.Fatalf("default receipt must be under TMPDIR: %s", result)
		}
		f.call("", f.review(info["approval_token"].(string), f.command, "--file", path, "--out", scratch)...)
		if !strings.Contains(f.recordText("input"), "document") {
			t.Fatal("document did not reach the reviewer outside Git")
		}
	}
	f.clearCalls()
	assertReviewFileGuards(t, f, f.repo)
	outside := filepath.Join(f.root, "outside.md")
	f.write(outside, "plain text\n")
	result := f.call("$TMPDIR", "inspect", "--file", outside, "--provider", "vendor-a", "--provider-command", f.command)
	if !strings.Contains(result, "current directory") {
		t.Fatalf("file rejection lacks the non-Git root alternative: %s", result)
	}
	if err := os.Symlink(f.root, filepath.Join(f.repo, "escape")); err != nil {
		t.Fatal(err)
	}
	f.call("$TMPDIR", "inspect", "--file", "escape/outside.md", "--provider", "vendor-a", "--provider-command", f.command)
	for _, args := range [][]string{
		{"snapshot"},
		{"delta", "--since", f.head, "--out", filepath.Join(scratch, "delta.patch")},
		{"inspect", "--base", f.base, "--head", f.head, "--provider", "vendor-a", "--provider-command", f.command},
	} {
		result := f.call("Git work tree", args...)
		if !strings.Contains(result, "run from the repository") || !strings.Contains(result, "--file") {
			t.Errorf("Git requirement lacks an actionable fix: %s", result)
		}
	}
	if _, err := os.Stat(filepath.Join(f.repo, ".git")); !os.IsNotExist(err) {
		t.Fatalf("document review created Git metadata: %v", err)
	}
	info := f.inspect(f.command, "--file", "package.md")
	for _, out := range []string{f.repo, f.root} {
		result := f.call("--out", f.review(info["approval_token"].(string), f.command, "--file", "package.md", "--out", out)...)
		if !strings.Contains(result, "$TMPDIR/<name>") {
			t.Errorf("non-Git output rejection lacks a destination example: %s", result)
		}
	}
	t.Setenv("TMPDIR", f.root)
	info = f.inspect(f.command, "--file", "package.md")
	f.call("", f.review(info["approval_token"].(string), f.command, "--file", "package.md")...)
	f.clearCalls()
	t.Setenv("TMPDIR", f.repo)
	info = f.inspect(f.command, "--file", "package.md")
	f.call("--out", f.review(info["approval_token"].(string), f.command, "--file", "package.md")...)
	f.noCalls()
}

func TestReviewPathRuleHints(t *testing.T) {
	f := newReviewFixture(t)
	scratch := filepath.Join(f.root, "scratch")
	if err := os.Mkdir(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", scratch)
	link := filepath.Join(f.root, "linked-out")
	if err := os.Symlink(scratch, link); err != nil {
		t.Fatal(err)
	}
	readOnly := filepath.Join(scratch, "readonly")
	if err := os.Mkdir(readOnly, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })
	info := f.inspect(f.command, "--file", "app.go")
	f.write(filepath.Join(scratch, "regular.md"), "plain text\n")
	for _, path := range []string{filepath.Join(f.root, "missing.md"), filepath.Join(scratch, "alias.md"), scratch, filepath.Join(f.root, "outside.md"), filepath.Join(scratch, ".env")} {
		if path == filepath.Join(scratch, "alias.md") {
			if err := os.Symlink("regular.md", path); err != nil {
				t.Fatal(err)
			}
		} else if path != scratch && !strings.HasSuffix(path, "missing.md") {
			f.write(path, "plain text\n")
		}
		result := f.call("$TMPDIR", "inspect", "--file", path, "--provider", "vendor-a", "--provider-command", f.command)
		if !strings.Contains(result, "regular file") {
			t.Errorf("file rejection lacks an accepted alternative: %s", result)
		}
	}
	for _, out := range []string{"relative", filepath.Join(scratch, "missing"), f.repo, f.root, link, filepath.Join(scratch, "regular.md"), readOnly} {
		result := f.call("--out", f.review(info["approval_token"].(string), f.command, "--file", "app.go", "--out", out)...)
		if !strings.Contains(result, "$TMPDIR/<name>") {
			t.Errorf("output rejection lacks a destination example: %s", result)
		}
	}
	for _, out := range []string{filepath.Join(f.root, "outside.patch"), filepath.Join(scratch, "notes.md"), filepath.Join(scratch, "missing", "delta.patch")} {
		result := f.call("--out", "delta", "--since", f.head, "--out", out)
		if !strings.Contains(result, "$TMPDIR/<name>.patch") {
			t.Errorf("delta rejection lacks an accepted path: %s", result)
		}
	}
	f.noCalls()
}

func TestReviewRangePathRuleHints(t *testing.T) {
	f := newReviewFixture(t)
	if err := os.Symlink("app.go", filepath.Join(f.repo, "link.go")); err != nil {
		t.Fatal(err)
	}
	linkHead := f.commit("symlink", "link.go")
	f.exec(f.repo, "git", "update-index", "--add", "--cacheinfo", "160000,"+f.head+",module")
	f.exec(f.repo, "git", "-c", "commit.gpgsign=false", "commit", "-qm", "submodule")
	submoduleHead := f.exec(f.repo, "git", "rev-parse", "HEAD")
	f.write(filepath.Join(f.repo, ".env"), "plain text\n")
	secretHead := f.commit("secret path", ".env")
	for _, tc := range []struct{ base, head, needle string }{
		{f.head, linkHead, "symlink"},
		{linkHead, submoduleHead, "submodule"},
		{submoduleHead, secretHead, "secret-like path"},
	} {
		result := f.call(tc.needle, "inspect", "--base", tc.base, "--head", tc.head, "--provider", "vendor-a", "--provider-command", f.command)
		if !strings.Contains(result, "--file") || !strings.Contains(result, "$TMPDIR") {
			t.Errorf("range path rejection lacks an accepted alternative: %s", result)
		}
	}
	f.noCalls()
}

// Outside Git the current directory becomes the review root, so a home or
// filesystem-root launch would let any file there leave the machine.
func TestReviewWithoutGitRefusesHomeAsRoot(t *testing.T) {
	f := newReviewFixture(t)
	t.Setenv("GIT_CEILING_DIRECTORIES", f.root)
	f.repo = filepath.Join(f.root, "home")
	if err := os.Mkdir(f.repo, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", f.repo)
	f.write(filepath.Join(f.repo, "notes.md"), "Review this document.\n")
	f.call("home directory", "inspect", "--file", "notes.md", "--provider", "vendor-a", "--provider-command", f.command)
}
