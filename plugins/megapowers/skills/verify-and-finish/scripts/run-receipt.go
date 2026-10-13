// Command run-receipt runs one check, keeps its full combined output in a
// private log, and prints a short tail plus a one-line receipt with the real
// exit status and the git identity of the working tree.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	exitToolError     = 125
	exitNotRunnable   = 126
	exitNotFound      = 127
	defaultTail       = 40
	maxTailBytes      = 1 << 20
	maxUntrackedBytes = 16 << 20
)

const usage = `Usage: go run run-receipt.go [--dir DIR] [--tail N] [--shell] [--reuse] -- <command> [args...]

Runs the command, writes its combined stdout and stderr to a private log,
prints the last N lines after it exits, then one final line:

  receipt: exit=<code> duration=<ms>ms head=<sha|none> tree=<clean|dirty:<sha>|none> log=<path> json=<path>

Options:
  --dir DIR   receipt directory (default $TMPDIR/megapowers-receipts)
  --tail N    output lines to print after the run (default 40)
  --shell     run the single argument after -- through sh -c; the exit
              status is then the shell's, so a failing pipeline stage counts
              only under the shell's own rules (not supported on Windows)
  --reuse     if DIR already holds a passing receipt for the same command,
              working directory, commit, and uncommitted-tree digest, print
              that receipt instead of running the check again
  --help      print this help

Exit status is the command's; 127 if it cannot be found, 126 if it cannot
run, 125 for a tool error.
`

type receipt struct {
	SchemaVersion      string   `json:"schema_version"`
	Argv               []string `json:"argv"`
	Shell              bool     `json:"shell"`
	Cwd                string   `json:"cwd"`
	StartedAt          string   `json:"started_at"`
	DurationMS         int64    `json:"duration_ms"`
	ExitCode           int      `json:"exit_code"`
	Signal             string   `json:"signal,omitempty"`
	GitHead            string   `json:"git_head"`
	TreeState          string   `json:"tree_state"`
	TreeDigest         string   `json:"tree_digest"`
	LogPath            string   `json:"log_path"`
	LogBytes           int64    `json:"log_bytes"`
	TruncatedTailLines int      `json:"truncated_tail_lines"`
}

type toolError struct{ msg string }

func (e toolError) Error() string { return e.msg }

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("run-receipt", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dir := flags.String("dir", "", "receipt directory")
	tail := flags.Int("tail", defaultTail, "tail lines")
	shell := flags.Bool("shell", false, "run through the shell")
	reuse := flags.Bool("reuse", false, "reuse a passing receipt for the same tree")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		return fail(stderr, err.Error())
	}
	command := flags.Args()
	switch {
	case len(command) == 0:
		return fail(stderr, "no command given after --")
	case *tail < 0:
		return fail(stderr, "--tail must be zero or more")
	case *shell && runtime.GOOS == "windows":
		// Go quotes arguments for CommandLineToArgvW, which cmd /c does not parse.
		return fail(stderr, "--shell is not supported on Windows; pass the command as argv after --")
	case *shell && len(command) != 1:
		return fail(stderr, "--shell takes exactly one argument after --; quote the whole command line")
	}

	code, err := execute(command, *shell, *reuse, *dir, *tail, stdout)
	if err != nil {
		var tool toolError
		if errors.As(err, &tool) {
			return fail(stderr, tool.msg)
		}
		return fail(stderr, err.Error())
	}
	return code
}

func fail(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "run-receipt: %s\n%s", msg, usage)
	return exitToolError
}

func execute(command []string, shell, reuse bool, dir string, tail int, stdout io.Writer) (int, error) {
	if dir == "" {
		base := os.Getenv("TMPDIR")
		if base == "" {
			base = os.TempDir()
		}
		dir = filepath.Join(base, "megapowers-receipts")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return 0, toolError{fmt.Sprintf("resolve --dir: %v", err)}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, toolError{fmt.Sprintf("create receipt directory: %v", err)}
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return 0, toolError{fmt.Sprintf("inspect receipt directory: %v", err)}
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return 0, toolError{fmt.Sprintf("receipt directory %s is a symlink or not a directory", dir)}
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return 0, toolError{fmt.Sprintf("restrict receipt directory to 0700: %v", err)}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return 0, toolError{fmt.Sprintf("read working directory: %v", err)}
	}

	// Record the tree the check ran against before the command can change it.
	head, treeState, treeDigest := gitIdentity()
	if reuse && treeState != "none" {
		if prior, jsonPath, ok := findReusable(dir, command, shell, cwd, head, treeState, treeDigest); ok {
			lines, _, err := readTail(prior.LogPath, tail)
			if err != nil {
				return 0, toolError{fmt.Sprintf("read reused log: %v", err)}
			}
			for _, line := range lines {
				fmt.Fprintln(stdout, line)
			}
			fmt.Fprintf(stdout, "run-receipt: reused the passing receipt from %s; the commit and uncommitted tree are unchanged\n", prior.StartedAt)
			fmt.Fprintf(stdout, "receipt: exit=0 duration=%dms head=%s tree=%s log=%s json=%s\n",
				prior.DurationMS, short(head), treeLabel(treeState, treeDigest), prior.LogPath, jsonPath)
			return 0, nil
		}
	}

	started := time.Now().UTC()
	stem := filepath.Join(dir, fmt.Sprintf("%s-%d", started.Format("20060102T150405Z"), os.Getpid()))
	logPath, jsonPath := stem+".log", stem+".json"
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, toolError{fmt.Sprintf("create log: %v", err)}
	}

	argv := command
	if shell {
		argv = []string{"sh", "-c", command[0]}
	}
	code, sig := runCommand(argv, logFile)
	duration := time.Since(started)
	if err := logFile.Close(); err != nil {
		return 0, toolError{fmt.Sprintf("close log: %v", err)}
	}

	lines, logBytes, err := readTail(logPath, tail)
	if err != nil {
		return 0, toolError{fmt.Sprintf("read log: %v", err)}
	}
	total, err := countLines(logPath)
	if err != nil {
		return 0, toolError{fmt.Sprintf("read log: %v", err)}
	}

	record := receipt{
		SchemaVersion:      "1",
		Argv:               command,
		Shell:              shell,
		Cwd:                cwd,
		StartedAt:          started.Format(time.RFC3339),
		DurationMS:         duration.Milliseconds(),
		ExitCode:           code,
		Signal:             sig,
		GitHead:            head,
		TreeState:          treeState,
		TreeDigest:         treeDigest,
		LogPath:            logPath,
		LogBytes:           logBytes,
		TruncatedTailLines: max(total-len(lines), 0),
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return 0, toolError{fmt.Sprintf("encode receipt: %v", err)}
	}
	if err := writePrivate(jsonPath, append(data, '\n')); err != nil {
		return 0, toolError{fmt.Sprintf("write receipt: %v", err)}
	}

	for _, line := range lines {
		fmt.Fprintln(stdout, line)
	}
	fmt.Fprintf(stdout, "receipt: exit=%d duration=%dms head=%s tree=%s log=%s json=%s\n",
		code, record.DurationMS, short(head), treeLabel(treeState, treeDigest), logPath, jsonPath)
	return code, nil
}

// findReusable returns the newest passing receipt in dir whose command,
// working directory, commit, and tree digest all match, and whose log still
// exists. Only repository state is compared: a check that depends on the
// network, environment, or files outside the repository must not use --reuse.
func findReusable(dir string, command []string, shell bool, cwd, head, treeState, treeDigest string) (receipt, string, bool) {
	// Read the directory itself: a glob would treat brackets in dir as a pattern.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return receipt{}, "", false
	}
	var matches []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".json") {
			matches = append(matches, filepath.Join(dir, entry.Name()))
		}
	}
	// Names start with a UTC timestamp, so reverse order is newest first.
	for i := len(matches) - 1; i >= 0; i-- {
		data, err := os.ReadFile(matches[i])
		if err != nil {
			continue
		}
		var prior receipt
		if json.Unmarshal(data, &prior) != nil {
			continue
		}
		if prior.ExitCode != 0 || prior.Signal != "" || prior.Shell != shell || prior.Cwd != cwd ||
			prior.GitHead != head || prior.TreeState != treeState || prior.TreeDigest != treeDigest ||
			strings.Join(prior.Argv, "\x00") != strings.Join(command, "\x00") {
			continue
		}
		if _, err := os.Stat(prior.LogPath); err != nil {
			continue
		}
		return prior, matches[i], true
	}
	return receipt{}, "", false
}

// runCommand runs argv with stdout and stderr on the same file, so the log
// keeps their relative order. It returns the exit code and any signal name.
func runCommand(argv []string, log *os.File) (int, string) {
	path, err := exec.LookPath(argv[0])
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			fmt.Fprintf(log, "run-receipt: command not found: %s\n", argv[0])
			return exitNotFound, ""
		}
		fmt.Fprintf(log, "run-receipt: cannot run %s: %v\n", argv[0], err)
		return exitNotRunnable, ""
	}
	cmd := exec.Command(path, argv[1:]...)
	cmd.Args[0] = argv[0]
	cmd.Stdin = nil
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(log, "run-receipt: cannot run %s: %v\n", argv[0], err)
		return exitNotRunnable, ""
	}

	// Forward an interrupt so the receipt is still written when the run is cancelled.
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-interrupts:
			if cmd.Process.Signal(sig) != nil {
				_ = cmd.Process.Kill()
			}
		case <-done:
		}
	}()
	waitErr := cmd.Wait()
	close(done)
	signal.Stop(interrupts)

	state := cmd.ProcessState
	if state == nil {
		fmt.Fprintf(log, "run-receipt: wait for %s: %v\n", argv[0], waitErr)
		return exitToolError, ""
	}
	if code := state.ExitCode(); code >= 0 {
		return code, ""
	}
	if status, ok := state.Sys().(interface {
		Signaled() bool
		Signal() syscall.Signal
	}); ok && status.Signaled() {
		return 128 + int(status.Signal()), status.Signal().String()
	}
	return exitToolError, ""
}

// gitIdentity reports HEAD and a digest of uncommitted state in the current
// directory, or "none" when git or a commit is unavailable.
func gitIdentity() (head, state, digest string) {
	out, err := gitOutput("rev-parse", "--verify", "HEAD")
	if err != nil {
		return "none", "none", ""
	}
	head = strings.TrimSpace(string(out))
	status, err := gitOutput("status", "--porcelain=v1", "-z")
	if err != nil {
		return head, "none", ""
	}
	diff, err := gitOutput("diff", "--binary", "HEAD")
	if err != nil {
		return head, "none", ""
	}
	if len(status) == 0 && len(diff) == 0 {
		return head, "clean", ""
	}
	top, err := gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return head, "none", ""
	}
	// Trim only the line ending: a path may end with a space.
	root := strings.TrimRight(string(top), "\r\n")
	untracked, err := gitOutput("-C", root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return head, "none", ""
	}
	hash := sha256.New()
	hash.Write(status)
	hash.Write([]byte{0})
	hash.Write(diff)
	for _, rel := range strings.Split(string(untracked), "\x00") {
		if rel != "" {
			hashUntracked(hash, root, rel)
		}
	}
	return head, "dirty", hex.EncodeToString(hash.Sum(nil))
}

// hashUntracked adds an untracked path and its content to the digest. Files
// over maxUntrackedBytes contribute their path and size only.
func hashUntracked(hash io.Writer, root, rel string) {
	fmt.Fprintf(hash, "\x00untracked\x00%s\x00", rel)
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(path)
	switch {
	case err != nil:
		fmt.Fprintf(hash, "error:%v", err)
	case info.Mode()&fs.ModeSymlink != 0:
		target, _ := os.Readlink(path)
		fmt.Fprintf(hash, "symlink:%s", target)
	case !info.Mode().IsRegular():
		fmt.Fprintf(hash, "mode:%v", info.Mode().Type())
	case info.Size() > maxUntrackedBytes:
		fmt.Fprintf(hash, "size:%d", info.Size())
	default:
		file, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(hash, "error:%v", err)
			return
		}
		defer file.Close()
		fmt.Fprintf(hash, "content:%d:", info.Size())
		if _, err := io.Copy(hash, io.LimitReader(file, maxUntrackedBytes)); err != nil {
			fmt.Fprintf(hash, "error:%v", err)
		}
	}
}

func gitOutput(args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"--no-pager"}, args...)...)
	cmd.Stdin = nil
	cmd.Stderr = io.Discard
	return cmd.Output()
}

func short(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func treeLabel(state, digest string) string {
	if state == "dirty" {
		return "dirty:" + short(digest)
	}
	return state
}

// readTail returns the last n lines of the file without loading more than
// maxTailBytes, plus the file size.
func readTail(path string, n int) ([]string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	size := info.Size()
	if n == 0 || size == 0 {
		return nil, size, nil
	}
	start := max(size-maxTailBytes, 0)
	buf := make([]byte, size-start)
	if _, err := file.ReadAt(buf, start); err != nil && !errors.Is(err, io.EOF) {
		return nil, size, err
	}
	buf = bytes.TrimSuffix(buf, []byte("\n"))
	lines := strings.Split(string(buf), "\n")
	if start > 0 && len(lines) > 1 {
		lines = lines[1:] // first line may be partial
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines, size, nil
}

// countLines counts newline-terminated lines plus a final unterminated one.
func countLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	buf := make([]byte, 64*1024)
	count, last := 0, byte('\n')
	for {
		read, err := file.Read(buf)
		if read > 0 {
			count += bytes.Count(buf[:read], []byte("\n"))
			last = buf[read-1]
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	if last != '\n' {
		count++
	}
	return count, nil
}

func writePrivate(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
