package contracts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const runReceiptTool = "plugins/megapowers/skills/verify-and-finish/scripts/run-receipt.go"

var receiptLinePattern = regexp.MustCompile(`(?m)^receipt: exit=(-?\d+) duration=\d+ms head=([0-9a-f]{12}|none) tree=(clean|dirty:[0-9a-f]{12}|none) log=(\S+\.log) json=(\S+\.json)$`)

type receiptResult struct {
	output string
	code   int
	exit   string
	head   string
	tree   string
	log    string
	json   string
}

// runReceiptIn runs the tool binary with dir as its working directory. The
// binary keeps the child's exit status; `go run` reports any failure as 1.
func runReceiptIn(t *testing.T, bin, dir string, env []string, args ...string) receiptResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	result := receiptResult{output: string(out)}
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run-receipt %v: %v", args, err)
		}
		result.code = exit.ExitCode()
	}
	if match := receiptLinePattern.FindStringSubmatch(result.output); match != nil {
		result.exit, result.head, result.tree, result.log, result.json = match[1], match[2], match[3], match[4], match[5]
	}
	return result
}

func requireReceipt(t *testing.T, result receiptResult, code int, label string) {
	t.Helper()
	if result.code != code {
		t.Fatalf("%s: tool exit %d, want %d\n%s", label, result.code, code, result.output)
	}
	if result.log == "" {
		t.Fatalf("%s: no receipt line\n%s", label, result.output)
	}
	lines := strings.Split(strings.TrimRight(result.output, "\n"), "\n")
	if !strings.HasPrefix(lines[len(lines)-1], "receipt: ") {
		t.Errorf("%s: receipt is not the final line\n%s", label, result.output)
	}
	if want := strconv.Itoa(code); result.exit != want {
		t.Errorf("%s: receipt exit=%s, want %s", label, result.exit, want)
	}
}

func shellExit(t *testing.T, script string) int {
	t.Helper()
	err := exec.Command("sh", "-c", script).Run()
	if err == nil {
		return 0
	}
	exit, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("sh -c %q: %v", script, err)
	}
	return exit.ExitCode()
}

func TestRunReceiptContract(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("contract fixtures use sh")
	}
	tmp := t.TempDir()
	outside := filepath.Join(tmp, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	// Keep git from discovering any repository above the fixture directories.
	env := []string{"GIT_CEILING_DIRECTORIES=" + tmp, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
	receipts := filepath.Join(tmp, "receipts")
	root := repoRoot(t)
	tool := "./" + runReceiptTool
	bin := filepath.Join(tmp, "bin", "run-receipt")
	if output, code := run(t, root, nil, "go", "build", "-o", bin, tool); code != 0 {
		t.Fatalf("build run-receipt (%d): %s", code, output)
	}

	t.Run("go run keeps the receipt line", func(t *testing.T) {
		output, code := run(t, root, nil, "go", "run", tool, "--dir", receipts, "--", "sh", "-c", "echo via-go-run; exit 7")
		if code == 0 || !strings.Contains(output, "via-go-run\n") || !regexp.MustCompile(`(?m)^receipt: exit=7 `).MatchString(output) {
			t.Errorf("go run (%d) lost the output or exit status: %s", code, output)
		}
	})

	t.Run("help", func(t *testing.T) {
		result := runReceiptIn(t, bin, outside, env, "--help")
		if result.code != 0 || !strings.Contains(result.output, "Usage:") || !strings.Contains(result.output, "--shell") {
			t.Errorf("--help (%d): %s", result.code, result.output)
		}
	})

	t.Run("passing command", func(t *testing.T) {
		result := runReceiptIn(t, bin, outside, env, "--dir", receipts, "--", "sh", "-c", "echo hello-receipt")
		requireReceipt(t, result, 0, "passing")
		if !strings.Contains(result.output, "hello-receipt\n") {
			t.Errorf("tail missing command output: %s", result.output)
		}
		if filepath.Dir(result.log) != receipts || filepath.Dir(result.json) != receipts {
			t.Errorf("receipt files outside --dir: %s %s", result.log, result.json)
		}
		if !regexp.MustCompile(`^\d{8}T\d{6}Z-\d+\.log$`).MatchString(filepath.Base(result.log)) {
			t.Errorf("log name %q does not match <UTC stamp>-<pid>.log", filepath.Base(result.log))
		}
		if strings.TrimSuffix(result.log, ".log")+".json" != result.json {
			t.Errorf("json %q is not next to log %q", result.json, result.log)
		}
		info, err := os.Stat(receipts)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Errorf("receipt dir mode %v, want 0700", info.Mode().Perm())
		}
		for _, path := range []string{result.log, result.json} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Errorf("%s mode %v, want 0600", path, info.Mode().Perm())
			}
		}
	})

	t.Run("failing command keeps exit code and ordered output", func(t *testing.T) {
		result := runReceiptIn(t, bin, outside, env, "--dir", receipts, "--", "sh", "-c", "echo out1; echo err1 >&2; echo out2; exit 7")
		requireReceipt(t, result, 7, "failing")
		log := readBytes(t, result.log)
		if string(log) != "out1\nerr1\nout2\n" {
			t.Errorf("log did not keep combined output in order: %q", log)
		}
	})

	t.Run("shell pipeline reports the shell exit", func(t *testing.T) {
		for _, script := range []string{"false | cat", "echo piped | grep -q absent", "exit 3"} {
			want := shellExit(t, script)
			result := runReceiptIn(t, bin, outside, env, "--dir", receipts, "--shell", "--", script)
			requireReceipt(t, result, want, script)
		}
		var receipt map[string]any
		result := runReceiptIn(t, bin, outside, env, "--dir", receipts, "--shell", "--", "false | cat")
		if err := json.Unmarshal(readBytes(t, result.json), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt["shell"] != true {
			t.Errorf("shell receipt did not record shell=true: %v", receipt["shell"])
		}
		bad := runReceiptIn(t, bin, outside, env, "--dir", receipts, "--shell", "--", "echo", "two")
		if bad.code != 125 {
			t.Errorf("--shell with two arguments exit %d, want 125: %s", bad.code, bad.output)
		}
	})

	t.Run("log keeps full output beyond the tail", func(t *testing.T) {
		script := "i=1; while [ $i -le 100 ]; do echo line-$i; i=$((i+1)); done"
		result := runReceiptIn(t, bin, outside, env, "--dir", receipts, "--tail", "3", "--", "sh", "-c", script)
		requireReceipt(t, result, 0, "tail")
		for _, line := range []string{"line-98\n", "line-99\n", "line-100\n"} {
			if !strings.Contains(result.output, line) {
				t.Errorf("tail missing %q: %s", line, result.output)
			}
		}
		if strings.Contains(result.output, "line-97\n") || strings.Contains(result.output, "line-1\n") {
			t.Errorf("tail printed more than 3 lines: %s", result.output)
		}
		log := string(readBytes(t, result.log))
		if !strings.HasPrefix(log, "line-1\nline-2\n") || !strings.HasSuffix(log, "line-100\n") || strings.Count(log, "\n") != 100 {
			t.Errorf("log does not hold the full output: %d lines", strings.Count(log, "\n"))
		}

		var receipt map[string]any
		if err := json.Unmarshal(readBytes(t, result.json), &receipt); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"schema_version", "argv", "shell", "cwd", "started_at", "duration_ms", "exit_code", "git_head", "tree_state", "tree_digest", "log_path", "log_bytes", "truncated_tail_lines"} {
			if _, ok := receipt[field]; !ok {
				t.Errorf("json receipt missing %q", field)
			}
		}
		checks := map[string]any{
			"schema_version":       "1",
			"shell":                false,
			"exit_code":            float64(0),
			"log_path":             result.log,
			"log_bytes":            float64(len(log)),
			"truncated_tail_lines": float64(97),
			"git_head":             "none",
			"tree_state":           "none",
		}
		for field, want := range checks {
			if receipt[field] != want {
				t.Errorf("json %s = %v, want %v", field, receipt[field], want)
			}
		}
		argv, _ := receipt["argv"].([]any)
		if len(argv) != 3 || argv[0] != "sh" || argv[2] != script {
			t.Errorf("json argv = %v", receipt["argv"])
		}
		if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`).MatchString(receipt["started_at"].(string)) {
			t.Errorf("started_at %v is not RFC3339 UTC", receipt["started_at"])
		}
		if cwd, _ := receipt["cwd"].(string); filepath.Clean(cwd) != outside {
			if resolved, _ := filepath.EvalSymlinks(outside); filepath.Clean(cwd) != resolved {
				t.Errorf("json cwd %q, want %q", cwd, outside)
			}
		}
	})

	t.Run("git identity inside and outside a repository", func(t *testing.T) {
		outsideResult := runReceiptIn(t, bin, outside, env, "--dir", receipts, "--", "sh", "-c", "true")
		requireReceipt(t, outsideResult, 0, "outside")
		if outsideResult.head != "none" || outsideResult.tree != "none" {
			t.Errorf("outside repo head=%s tree=%s, want none", outsideResult.head, outsideResult.tree)
		}

		repo := filepath.Join(tmp, "repo")
		if err := os.MkdirAll(repo, 0o755); err != nil {
			t.Fatal(err)
		}
		gitCmd := func(args ...string) string {
			t.Helper()
			cmd := exec.Command("git", append([]string{"-c", "user.name=Receipt Test", "-c", "user.email=receipt@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
			cmd.Dir = repo
			cmd.Env = append(os.Environ(), env...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
			return strings.TrimSpace(string(out))
		}
		gitCmd("init", "-q")
		if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("one\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitCmd("add", "file.txt")
		gitCmd("commit", "-q", "-m", "initial")
		head := gitCmd("rev-parse", "HEAD")

		clean := runReceiptIn(t, bin, repo, env, "--dir", receipts, "--", "sh", "-c", "true")
		requireReceipt(t, clean, 0, "clean repo")
		if clean.head != head[:12] || clean.tree != "clean" {
			t.Errorf("clean repo head=%s tree=%s, want %s clean", clean.head, clean.tree, head[:12])
		}

		if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("two\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		dirty := runReceiptIn(t, bin, repo, env, "--dir", receipts, "--", "sh", "-c", "true")
		requireReceipt(t, dirty, 0, "dirty repo")
		if dirty.head != head[:12] || !strings.HasPrefix(dirty.tree, "dirty:") {
			t.Errorf("dirty repo head=%s tree=%s", dirty.head, dirty.tree)
		}
		if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("three\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		changed := runReceiptIn(t, bin, repo, env, "--dir", receipts, "--", "sh", "-c", "true")
		if changed.tree == dirty.tree || !strings.HasPrefix(changed.tree, "dirty:") {
			t.Errorf("different dirty content kept digest %s", changed.tree)
		}
		var receipt map[string]any
		if err := json.Unmarshal(readBytes(t, dirty.json), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt["git_head"] != head || receipt["tree_state"] != "dirty" {
			t.Errorf("json git_head=%v tree_state=%v", receipt["git_head"], receipt["tree_state"])
		}
		if digest, _ := receipt["tree_digest"].(string); len(digest) != 64 || !strings.HasPrefix(digest, strings.TrimPrefix(dirty.tree, "dirty:")) {
			t.Errorf("json tree_digest %q does not extend %s", digest, dirty.tree)
		}
	})

	t.Run("missing command exits 127", func(t *testing.T) {
		result := runReceiptIn(t, bin, outside, env, "--dir", receipts, "--", "megapowers-no-such-command-for-receipt")
		requireReceipt(t, result, 127, "missing command")
	})

	t.Run("tool errors exit 125", func(t *testing.T) {
		blocker := filepath.Join(tmp, "blocker")
		if err := os.WriteFile(blocker, []byte("file\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cases := map[string][]string{
			"bad flag":      {"--no-such-flag", "--", "true"},
			"no command":    {"--dir", receipts},
			"negative tail": {"--tail", "-1", "--", "true"},
			"dir is a file": {"--dir", filepath.Join(blocker, "sub"), "--", "true"},
		}
		for name, args := range cases {
			result := runReceiptIn(t, bin, outside, env, args...)
			if result.code != 125 {
				t.Errorf("%s: exit %d, want 125: %s", name, result.code, result.output)
			}
		}
	})

	t.Run("default dir honors TMPDIR", func(t *testing.T) {
		base := filepath.Join(tmp, "tmpdir")
		if err := os.MkdirAll(base, 0o700); err != nil {
			t.Fatal(err)
		}
		result := runReceiptIn(t, bin, outside, append(append([]string{}, env...), "TMPDIR="+base), "--", "sh", "-c", "true")
		requireReceipt(t, result, 0, "default dir")
		if filepath.Dir(result.log) != filepath.Join(base, "megapowers-receipts") {
			t.Errorf("default log %q not under TMPDIR/megapowers-receipts", result.log)
		}
	})
}
