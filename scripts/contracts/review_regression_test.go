package contracts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The provider is a local Go fixture. No test invokes a model or network client.
const reviewProviderSource = `package main
import("fmt"; "os"; "io"; "path/filepath"; "strings"; "strconv"; "time")
const record = RECORD_DIRECTORY
func write(name, value string) { if err := os.WriteFile(filepath.Join(record,name), []byte(value),0600); err != nil { panic(err) } }
func main() {
 args:=os.Args[1:]; mode:="ok"; delivery:="stdin"
 input,_:=io.ReadAll(os.Stdin)
 write("env",strings.Join(os.Environ(),"\n")); write("args",strings.Join(args,"\n")); path,_:=os.Executable(); write("path",path)
 f,_:=os.OpenFile(filepath.Join(record,"calls"),os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600); fmt.Fprintln(f,"call"); f.Close()
 for i:=0;i<len(args);i++ { switch args[i] {
 case "--mode": i++; mode=args[i]
 case "--prompt-file": i++; input,_=os.ReadFile(args[i]); info,_:=os.Stat(args[i]); write("prompt-mode",fmt.Sprintf("%o",info.Mode().Perm())); write("prompt-dir",filepath.Dir(args[i])); delivery="file"
 case "--scratch": i++; write("scratch",args[i])
 } }
 write("input",string(input)); write("delivery",delivery)
 switch mode {
 case "fail": fmt.Fprintln(os.Stderr,"\x1b[31mprovider authentication failed\x1b[0m"); fmt.Fprintln(os.Stderr,"api_key = sk-abcdefghijklmnopqrstuvwxyz123456\nOAUTH_TOKEN=eyJhbGciOiJIUzI1NiJ9.sensitive.signature\naccount=user@example.com org=org_12345 url=https://example.invalid/?token=secret\ntenant=internal-customer"); fmt.Fprint(os.Stderr,strings.Repeat("x",6000)); os.Exit(23)
 case "empty": fmt.Fprintln(os.Stderr,"401 OAuth access token has expired. Please log in again."); return
 case "limit": fmt.Println("You've reached your usage limit"); os.Exit(1)
 case "stall": time.Sleep(30*time.Second)
case "overflow": for i:=0;i<64;i++ { if _,err:=fmt.Print(strings.Repeat("x",1<<20));err!=nil{return} }; return
 case "later": data,_:=os.ReadFile(filepath.Join(record,"count")); n,_:=strconv.Atoi(string(data)); n++; write("count",strconv.Itoa(n)); if n>=3 { fmt.Fprintln(os.Stderr,"rate limit exceeded"); os.Exit(1) }
 }
 fmt.Printf("reviewed by fake provider via %s\n",delivery)
}
`

type reviewFixture struct {
	t                                                     *testing.T
	root, repo, record, binary, tool, command, base, head string
}

func newReviewFixture(t *testing.T) *reviewFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("review process isolation is Unix-only; Windows hook tests run separately")
	}
	f := &reviewFixture{t: t, root: t.TempDir()}
	f.repo, f.record = filepath.Join(f.root, "repo"), filepath.Join(f.root, "record")
	for _, dir := range []string{f.repo, f.record, filepath.Join(f.root, "bin")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	f.binary, f.tool = filepath.Join(f.root, "bin", "fake-reviewer"), filepath.Join(f.root, "review-tool")
	providerFile := filepath.Join(f.root, "provider.go")
	f.write(providerFile, strings.Replace(reviewProviderSource, "RECORD_DIRECTORY", strconv.Quote(f.record), 1))
	f.exec(f.root, "go", "build", "-o", f.binary, providerFile)
	f.exec(repoRoot(t), "go", "build", "-o", f.tool, filepath.Join(repoRoot(t), "plugins/megapowers/skills/independent-review/scripts/megapowers-review.go"))
	t.Setenv("PATH", filepath.Dir(f.binary)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MEGAPOWERS_TEST_LEAK", "do-not-forward")
	t.Setenv("FAKE_API_KEY", "test-credential")
	t.Setenv("FAKE_HOME", filepath.Join(f.root, "fake-home"))
	t.Setenv("FAKE_CONFIG_DIR", filepath.Join(f.root, "fake-home", "config"))
	f.command = "fake-reviewer --mode ok"
	f.exec(f.repo, "git", "init", "-q")
	f.exec(f.repo, "git", "config", "user.name", "Review Test")
	f.exec(f.repo, "git", "config", "user.email", "review@example.invalid")
	f.write(filepath.Join(f.repo, "app.go"), "package example\nfunc Value() int { return 1 }\n")
	f.base = f.commit("initial", "app.go")
	f.write(filepath.Join(f.repo, "app.go"), "package example\nfunc Value() int { return 2 }\n")
	f.head = f.commit("changed", "app.go")
	return f
}

func (f *reviewFixture) write(path, content string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		f.t.Fatal(err)
	}
}

func (f *reviewFixture) exec(dir, name string, args ...string) string {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.WaitDelay = 2 * time.Second
	data, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("%s %v: %v\n%s", name, args, err, data)
	}
	return strings.TrimSpace(string(data))
}

func (f *reviewFixture) commit(message string, paths ...string) string {
	f.t.Helper()
	f.exec(f.repo, "git", append([]string{"add", "--"}, paths...)...)
	f.exec(f.repo, "git", "-c", "commit.gpgsign=false", "commit", "-qm", message)
	return f.exec(f.repo, "git", "rev-parse", "HEAD")
}

func (f *reviewFixture) call(wantError string, args ...string) string {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.tool, args...)
	cmd.Dir = f.repo
	cmd.WaitDelay = 2 * time.Second
	data, err := cmd.CombinedOutput()
	output := string(data)
	diagnostic := output
	if len(diagnostic) > 4096 {
		diagnostic = diagnostic[:4096] + "\n[test diagnostic truncated]"
	}
	if wantError == "" {
		if err != nil {
			f.t.Fatalf("review %v: %v\n%s", args, err, diagnostic)
		}
	} else if err == nil || !strings.Contains(strings.ToLower(output), strings.ToLower(wantError)) {
		f.t.Fatalf("review %v: wanted %q, got %v\n%s", args, wantError, err, diagnostic)
	}
	return output
}

func (f *reviewFixture) inspect(command string, input ...string) map[string]any {
	f.t.Helper()
	args := append([]string{"inspect", "--provider", "vendor-a", "--provider-command", command}, input...)
	var value map[string]any
	if err := json.Unmarshal([]byte(f.call("", args...)), &value); err != nil {
		f.t.Fatal(err)
	}
	return value
}

func (f *reviewFixture) review(token, command string, input ...string) []string {
	return append([]string{"review", "--provider", "vendor-a", "--author", "vendor-b", "--provider-command", command, "--approve-external", token}, input...)
}

func (f *reviewFixture) recordText(name string) string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.record, name))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func (f *reviewFixture) clearCalls() {
	f.t.Helper()
	for _, name := range []string{"calls", "env", "input", "count"} {
		if err := os.Remove(filepath.Join(f.record, name)); err != nil && !os.IsNotExist(err) {
			f.t.Fatal(err)
		}
	}
}

func (f *reviewFixture) noCalls() {
	f.t.Helper()
	for _, name := range []string{"calls", "env", "input"} {
		if _, err := os.Stat(filepath.Join(f.record, name)); !os.IsNotExist(err) {
			f.t.Fatalf("provider ran before rejection: %s", name)
		}
	}
}

func reviewFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func assertReviewNoReceipts(t *testing.T, root string) {
	t.Helper()
	for _, file := range reviewFiles(t, root) {
		if filepath.Base(file) == "receipt.json" {
			t.Fatalf("failed review retained receipt: %s", file)
		}
	}
}

func TestReviewInputAndApprovalBoundaries(t *testing.T) {
	f := newReviewFixture(t)
	for _, tc := range []struct {
		name, needle string
		args         []string
	}{
		{"no input", "exactly one input mode", []string{"inspect", "--provider", "vendor-a", "--provider-command", f.command}},
		{"two inputs", "exactly one input mode", []string{"inspect", "--file", "app.go", "--base", f.base, "--head", f.head, "--provider", "vendor-a", "--provider-command", f.command}},
		{"partial range", "base and --head", []string{"inspect", "--base", f.base, "--provider", "vendor-a", "--provider-command", f.command}},
		{"no provider", "--provider", []string{"inspect", "--file", "app.go", "--provider-command", f.command}},
		{"no command", "--provider-command", []string{"inspect", "--file", "app.go", "--provider", "vendor-a"}},
		{"no review command", "--provider-command", []string{"review", "--file", "app.go", "--provider", "vendor-a", "--author", "vendor-b", "--approve-external", "invalid"}},
		{"no author", "--author", []string{"review", "--file", "app.go", "--provider", "vendor-a", "--provider-command", f.command, "--approve-external", "invalid"}},
		{"same family", "must differ", []string{"review", "--file", "app.go", "--provider", "vendor-a", "--author", "vendor-a", "--provider-command", f.command, "--approve-external", "invalid"}},
		{"no approval", "approve-external", []string{"review", "--file", "app.go", "--provider", "vendor-a", "--author", "vendor-b", "--provider-command", f.command}},
	} {
		t.Run(tc.name, func(t *testing.T) { f.call(tc.needle, tc.args...) })
	}
	for _, template := range []string{"fake-reviewer | cat", "fake-reviewer; id", "fake-reviewer & true", "fake-reviewer > out", "fake-reviewer < in", "fake-reviewer $(id)", "fake-reviewer `id`"} {
		f.call("shell", "inspect", "--file", "app.go", "--provider", "vendor-a", "--provider-command", template)
	}
	for template, needle := range map[string]string{"fake-reviewer 'open": "quote", "   ": "provider-command", "no-such-reviewer-binary --x": "not installed"} {
		f.call(needle, "inspect", "--file", "app.go", "--provider", "vendor-a", "--provider-command", template)
	}
	for _, tc := range []struct{ name, content, needle string }{
		{".env", "AWS_SECRET_ACCESS_KEY=not-part-of-the-commit\n", "secret-like path"},
		{"config.txt", "api_key = \"sk-abcdefghijklmnopqrstuvwxyz123456\"\n", "likely secret content"},
		{"binary.dat", "text\x00binary\n", "binary"},
		{"large.txt", strings.Repeat("a", 1100000), "size limit"},
	} {
		f.write(filepath.Join(f.repo, tc.name), tc.content)
		f.call(tc.needle, "inspect", "--file", tc.name, "--provider", "vendor-a", "--provider-command", f.command)
	}
	if err := os.Symlink("app.go", filepath.Join(f.repo, "link.go")); err != nil {
		t.Fatal(err)
	}
	f.call("symlink", "inspect", "--file", "link.go", "--provider", "vendor-a", "--provider-command", f.command)
	rangeInfo := f.inspect(f.command, "--base", f.base, "--head", f.head)
	rangeBytes, _ := json.Marshal(rangeInfo)
	for _, needle := range []string{f.base, f.head, "app.go"} {
		if !bytes.Contains(rangeBytes, []byte(needle)) {
			t.Fatalf("range disclosure missing %s", needle)
		}
	}
	local := filepath.Join(f.repo, "local-bin", "fake-reviewer")
	binary, err := os.ReadFile(f.binary)
	if err != nil {
		t.Fatal(err)
	}
	f.write(local, string(binary))
	if err := os.Chmod(local, 0700); err != nil {
		t.Fatal(err)
	}
	for _, template := range []string{strconv.Quote(local), "./local-bin/fake-reviewer"} {
		f.call("inside the repository", "inspect", "--file", "app.go", "--provider", "vendor-a", "--provider-command", template)
	}
	path := os.Getenv("PATH")
	t.Setenv("PATH", filepath.Dir(local)+string(os.PathListSeparator)+path)
	f.call("inside the repository", "inspect", "--file", "app.go", "--provider", "vendor-a", "--provider-command", f.command)
	t.Setenv("PATH", path+string(os.PathListSeparator)+filepath.Dir(local))
	info := f.inspect(f.command, "--file", "app.go")
	token, _ := info["approval_token"].(string)
	for _, key := range []string{"approval_token", "binary_sha256", "provider", "provider_command", "binary_path"} {
		if info[key] == nil || info[key] == "" {
			t.Errorf("disclosure missing %s", key)
		}
	}
	chunks := info["chunks"].([]any)
	if len(chunks) != 1 || len(chunks[0].(map[string]any)["package_sha256"].(string)) != 64 {
		t.Fatal("missing chunk package hash")
	}
	if info["provider"] != "vendor-a" || info["provider_command"] != f.command || info["binary_path"] != f.binary {
		t.Fatalf("incorrect disclosure: %v", info)
	}
	original, err := os.ReadFile(filepath.Join(f.repo, "app.go"))
	if err != nil {
		t.Fatal(err)
	}
	f.write(filepath.Join(f.repo, "app.go"), string(original)+"\n// changed after approval\n")
	f.clearCalls()
	f.call("approval token does not match", f.review(token, f.command, "--file", "app.go")...)
	f.noCalls()
	f.write(filepath.Join(f.repo, "app.go"), string(original))
	f.call("approval token does not match", f.review(token, f.command+" --unapproved", "--file", "app.go")...)
	f.noCalls()
	changedLabel := f.review(token, f.command, "--file", "app.go")
	changedLabel[2] = "vendor-c"
	f.call("approval token does not match", changedLabel...)
	f.noCalls()
	f.write(f.binary, string(binary)+"changed bytes")
	if err := os.Chmod(f.binary, 0700); err != nil {
		t.Fatal(err)
	}
	f.call("approval token does not match", f.review(token, f.command, "--file", "app.go")...)
	f.noCalls()
}

func TestReviewDeliveryReceiptsAndEnvironment(t *testing.T) {
	f := newReviewFixture(t)
	localPath := filepath.Join(f.repo, "local-bin")
	if err := os.Mkdir(localPath, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+localPath)
	token := f.inspect(f.command, "--file", "app.go")["approval_token"].(string)
	f.call("already exist", f.review(token, f.command, "--file", "app.go", "--out", filepath.Join(f.root, "missing"))...)
	before, _ := os.Stat(f.repo)
	f.call("outside the repository", f.review(token, f.command, "--file", "app.go", "--out", f.repo, "--retain-transcript")...)
	after, _ := os.Stat(f.repo)
	if before.Mode() != after.Mode() {
		t.Fatal("rejected destination changed repository mode")
	}
	out := filepath.Join(f.root, "out")
	if err := os.Mkdir(out, 0700); err != nil {
		t.Fatal(err)
	}
	result := f.call("", f.review(token, f.command, "--file", "app.go", "--out", out)...)
	for _, needle := range []string{"provider=vendor-a", "command=", "files=1", "app.go", "reviewed by fake provider"} {
		if !strings.Contains(result, needle) {
			t.Errorf("missing disclosure/output %q", needle)
		}
	}
	if f.recordText("delivery") != "stdin" || !strings.Contains(f.recordText("input"), "review-package") || f.recordText("args") != "--mode\nok" {
		t.Fatal("stdin or argument delivery changed")
	}
	if f.recordText("path") == f.binary {
		t.Fatal("provider was not executed from a verified private copy")
	}
	files := reviewFiles(t, out)
	if len(files) != 2 {
		t.Fatalf("default receipt retained %d files", len(files))
	}
	for _, file := range files {
		info, _ := os.Stat(file)
		dir, _ := os.Stat(filepath.Dir(file))
		if info.Mode().Perm() != 0600 || dir.Mode().Perm() != 0700 {
			t.Fatalf("nonprivate receipt %s", file)
		}
		data, _ := os.ReadFile(file)
		for _, needle := range []string{"vendor-a", "vendor-b", f.command, "binary_sha256"} {
			if !bytes.Contains(data, []byte(needle)) {
				t.Errorf("receipt %s missing %q", file, needle)
			}
		}
	}
	env := f.recordText("env")
	if strings.Contains(env, localPath) {
		t.Fatal("provider PATH retained a repository-local directory")
	}
	for _, forbidden := range []string{"MEGAPOWERS_TEST_LEAK=", "FAKE_API_KEY="} {
		if strings.Contains(env, forbidden) {
			t.Fatal("ambient variable reached provider")
		}
	}
	for _, required := range []string{"HOME=", "PATH="} {
		if !strings.Contains(env, required) {
			t.Errorf("missing allowed environment %s", required)
		}
	}
	args := f.review(token, f.command, "--file", "app.go", "--out", out, "--provider-env", "FAKE_API_KEY", "--provider-env", "FAKE_HOME", "--provider-env", "FAKE_CONFIG_DIR")
	f.call("", args...)
	env = f.recordText("env")
	for _, required := range []string{"FAKE_API_KEY=test-credential", "FAKE_HOME=" + os.Getenv("FAKE_HOME"), "FAKE_CONFIG_DIR=" + os.Getenv("FAKE_CONFIG_DIR")} {
		if !strings.Contains(env, required) {
			t.Errorf("explicit environment not passed: %s", required)
		}
	}
	f.call("provider-env", f.review(token, f.command, "--file", "app.go", "--out", out, "--provider-env", "FAKE=VALUE")...)
	t.Setenv("FAKE_HOME", filepath.Join(f.repo, "inside-home"))
	t.Setenv("FAKE_CONFIG_DIR", filepath.Join(f.repo, "inside-home", "config"))
	f.call("", args...)
	env = f.recordText("env")
	if strings.Contains(env, "FAKE_HOME=") || strings.Contains(env, "FAKE_CONFIG_DIR=") || !strings.Contains(env, "FAKE_API_KEY=test-credential") {
		t.Fatal("repository-local config environment was not removed")
	}
	fileCommand := `fake-reviewer --prompt-file {prompt_file} --scratch {scratch_dir} --note 'hello world' --pipe 'a|b' --sub "$(id)" back\slash`
	fileToken := f.inspect(fileCommand, "--file", "app.go")["approval_token"].(string)
	f.call("", f.review(fileToken, fileCommand, "--file", "app.go", "--out", out)...)
	if f.recordText("delivery") != "file" || f.recordText("prompt-mode") != "600" || !strings.Contains(f.recordText("input"), "review-package") {
		t.Fatal("private prompt-file delivery failed")
	}
	scratch := f.recordText("scratch")
	if scratch != f.recordText("prompt-dir") || strings.HasPrefix(scratch, f.repo+string(os.PathSeparator)) {
		t.Fatal("scratch boundary failed")
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Fatal("scratch was not removed")
	}
	for _, needle := range []string{"hello world", "a|b", "$(id)", "backslash"} {
		if !strings.Contains(f.recordText("args"), needle) {
			t.Errorf("quoted argument lost: %s", needle)
		}
	}
	retained := filepath.Join(f.root, "retained")
	if err := os.Mkdir(retained, 0700); err != nil {
		t.Fatal(err)
	}
	f.call("", f.review(token, f.command, "--file", "app.go", "--out", retained, "--retain-transcript")...)
	var names []string
	for _, file := range reviewFiles(t, retained) {
		info, _ := os.Stat(file)
		if info.Mode().Perm() != 0600 {
			t.Fatal("transcript is not private")
		}
		names = append(names, filepath.Base(file))
		if filepath.Base(file) == "receipt.json" && strings.Contains(file, "chunk-01") {
			data, _ := os.ReadFile(file)
			for _, needle := range []string{`"transcript_retained": true`, `"prompt_sha256"`, `"advisory": true`} {
				if !bytes.Contains(data, []byte(needle)) {
					t.Errorf("transcript receipt missing %s", needle)
				}
			}
		}
	}
	for _, name := range []string{"prompt.txt", "provider.stdout", "provider.stderr"} {
		if !strings.Contains(strings.Join(names, "\n"), name) {
			t.Errorf("missing opted-in transcript %s", name)
		}
	}
}

func TestReviewFailuresRemainPrivateAndBounded(t *testing.T) {
	f := newReviewFixture(t)
	for _, tc := range []struct{ mode, needle string }{{"fail", "provider exited"}, {"empty", "authentication failed; verify provider login or API credentials"}, {"limit", "preflight"}, {"stall", "timeout"}, {"overflow", "preflight"}} {
		t.Run(tc.mode, func(t *testing.T) {
			parent := f.t
			f.t = t
			defer func() { f.t = parent }()
			command := "fake-reviewer --mode " + tc.mode
			token := f.inspect(command, "--file", "app.go")["approval_token"].(string)
			out := filepath.Join(f.root, "failure-"+tc.mode)
			if err := os.Mkdir(out, 0700); err != nil {
				t.Fatal(err)
			}
			args := f.review(token, command, "--file", "app.go", "--out", out, "--preflight-timeout", "1s")
			started := time.Now()
			result := f.call(tc.needle, args...)
			if time.Since(started) > 15*time.Second {
				t.Fatal("failure exceeded deadline")
			}
			assertReviewNoReceipts(t, out)
			if len(result) > 4096 {
				t.Fatal("failure output was not bounded")
			}
			for _, secret := range []string{"sk-abcdefghijklmnopqrstuvwxyz123456", "eyJhbGciOiJIUzI1NiJ9", "user@example.com", "org_12345", "example.invalid", "internal-customer", "\x1b"} {
				if strings.Contains(result, secret) {
					t.Errorf("diagnostic leaked fixture marker %q", secret)
				}
			}
		})
	}
	readOnly := filepath.Join(f.root, "readonly")
	if err := os.Mkdir(readOnly, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })
	token := f.inspect(f.command, "--file", "app.go")["approval_token"].(string)
	f.clearCalls()
	f.call("receipt", f.review(token, f.command, "--file", "app.go", "--out", readOnly)...)
	f.noCalls()
}

func TestReviewRangesAndChunking(t *testing.T) {
	f := newReviewFixture(t)
	if err := os.Symlink("app.go", filepath.Join(f.repo, "link.go")); err != nil {
		t.Fatal(err)
	}
	linkHead := f.commit("symlink", "link.go")
	f.call("symlink", "inspect", "--base", f.head, "--head", linkHead, "--provider", "vendor-a", "--provider-command", f.command)
	f.exec(f.repo, "git", "rm", "-q", "link.go")
	f.exec(f.repo, "git", "-c", "commit.gpgsign=false", "commit", "-qm", "remove symlink")
	subBase := f.exec(f.repo, "git", "rev-parse", "HEAD")
	f.exec(f.repo, "git", "update-index", "--add", "--cacheinfo", "160000,"+f.head+",vendor/module")
	f.exec(f.repo, "git", "-c", "commit.gpgsign=false", "commit", "-qm", "gitlink")
	subHead := f.exec(f.repo, "git", "rev-parse", "HEAD")
	f.call("submodule", "inspect", "--base", subBase, "--head", subHead, "--provider", "vendor-a", "--provider-command", f.command)
	f.write(filepath.Join(f.repo, "empty.txt"), "")
	emptyHead := f.commit("empty", "empty.txt")
	empty := f.inspect(f.command, "--base", subHead, "--head", emptyHead)
	data, _ := json.Marshal(empty)
	if !bytes.Contains(data, []byte("empty.txt")) || !bytes.Contains(data, []byte("e3b0c44298fc1c149afbf4c8996fb924")) {
		t.Fatal("empty range file lost its hash")
	}
	base := emptyHead
	for _, dir := range []string{"alpha", "beta", "gamma"} {
		for n := 1; n <= 60; n++ {
			f.write(filepath.Join(f.repo, dir, fmt.Sprintf("f%03d.txt", n)), fmt.Sprintf("file %s %d\n", dir, n))
		}
	}
	paths := []string{"alpha", "beta", "gamma"}
	for n := 1; n <= 20; n++ {
		name := fmt.Sprintf("root%02d.txt", n)
		f.write(filepath.Join(f.repo, name), fmt.Sprintf("root %d\n", n))
		paths = append(paths, name)
	}
	head := f.commit("200 files", paths...)
	info := f.inspect(f.command, "--base", base, "--head", head)
	token := info["approval_token"].(string)
	chunks := info["chunks"].([]any)
	if info["file_count"] != float64(200) || len(chunks) != 2 || info["chunk_count"] != float64(2) {
		t.Fatalf("bad chunk count: %v", info)
	}
	first, second := chunks[0].(map[string]any), chunks[1].(map[string]any)
	if first["file_count"] != float64(80) || second["file_count"] != float64(120) || first["package_sha256"] == second["package_sha256"] {
		t.Fatal("directory grouping or chunk hashes failed")
	}
	p1, p2 := first["paths"].([]any), second["paths"].([]any)
	for value, prefix := range map[string]string{p1[0].(string): "root", p1[len(p1)-1].(string): "alpha/", p2[0].(string): "beta/", p2[len(p2)-1].(string): "gamma/"} {
		if !strings.HasPrefix(value, prefix) {
			t.Errorf("chunk path %s not grouped as %s", value, prefix)
		}
	}
	if f.inspect(f.command, "--base", base, "--head", head)["approval_token"] != token {
		t.Fatal("chunk approval is nondeterministic")
	}
	f.call("chunk ceiling", "inspect", "--base", base, "--head", head, "--provider", "vendor-a", "--provider-command", f.command, "--max-files-per-chunk", "10")
	f.write(filepath.Join(f.repo, "beta/f001.txt"), "changed\n")
	changed := f.commit("changed chunk", "beta/f001.txt")
	f.clearCalls()
	f.call("approval token does not match", f.review(token, f.command, "--base", base, "--head", changed)...)
	f.noCalls()
	out := filepath.Join(f.root, "chunks")
	if err := os.Mkdir(out, 0700); err != nil {
		t.Fatal(err)
	}
	result := f.call("", f.review(token, f.command, "--base", base, "--head", head, "--out", out)...)
	if strings.Count(result, "reviewed by fake provider") != 2 || strings.Count(f.recordText("calls"), "call") != 3 {
		t.Fatal("chunk dispatch count failed")
	}
	if len(reviewFiles(t, out)) != 3 {
		t.Fatal("expected index plus one receipt per chunk")
	}
	for _, file := range reviewFiles(t, out) {
		if strings.Contains(file, "chunk-02") {
			var receipt struct {
				Chunk  struct{ Index, Count int }
				Source struct{ Files []any }
			}
			raw, _ := os.ReadFile(file)
			if err := json.Unmarshal(raw, &receipt); err != nil {
				t.Fatal(err)
			}
			if receipt.Chunk.Index != 2 || receipt.Chunk.Count != 2 || len(receipt.Source.Files) != 120 {
				t.Fatal("second chunk receipt is incomplete")
			}
		}
	}
	command := "fake-reviewer --mode later"
	failureToken := f.inspect(command, "--base", base, "--head", head)["approval_token"].(string)
	f.clearCalls()
	failureOut := filepath.Join(f.root, "chunk-failure")
	if err := os.Mkdir(failureOut, 0700); err != nil {
		t.Fatal(err)
	}
	result = f.call("chunk 2 of 2", f.review(failureToken, command, "--base", base, "--head", head, "--out", failureOut)...)
	if !strings.Contains(result, "completed chunks: 1") {
		t.Fatal("partial completion not reported")
	}
	assertReviewNoReceipts(t, failureOut)
	for n := 1; n <= 6; n++ {
		f.write(filepath.Join(f.repo, "bulk", fmt.Sprintf("b%d.txt", n)), strings.Repeat("a", 300000))
	}
	bytesHead := f.commit("bulk files", "bulk")
	bulk := f.inspect(f.command, "--base", changed, "--head", bytesHead)
	count := 0
	for _, raw := range bulk["chunks"].([]any) {
		chunk := raw.(map[string]any)
		if chunk["byte_count"].(float64) > 1048576 {
			t.Fatal("oversized chunk")
		}
		count += int(chunk["file_count"].(float64))
	}
	if count != 6 || bulk["chunk_count"].(float64) < 2 {
		t.Fatal("byte chunks lost files")
	}
	f.write(filepath.Join(f.repo, "bulk/huge.txt"), strings.Repeat("b", 600000))
	hugeHead := f.commit("huge file", "bulk/huge.txt")
	f.call("size limit", "inspect", "--base", bytesHead, "--head", hugeHead, "--provider", "vendor-a", "--provider-command", f.command)
}
