package maintain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type validation struct {
	name    string
	limit   time.Duration
	command []string
}

func runValidate(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: validate.sh")
		return 2
	}
	for _, required := range []string{"go", "git", "bash", "gofmt"} {
		if !executable(required) {
			fmt.Fprintf(stderr, "validate: required command not found: %s\n", required)
			return 2
		}
	}
	pass, fail := 0, 0
	run := func(check validation) {
		checkCtx, cancel := deadline(ctx, check.limit)
		defer cancel()
		log := cappedBuffer{limit: 10 << 20}
		err := command(checkCtx, root, &log, &log, nil, check.command[0], check.command[1:]...)
		if err == nil {
			fmt.Fprintf(stdout, "  PASS %s\n", check.name)
			pass++
			return
		}
		if checkCtx.Err() != nil {
			fmt.Fprintf(stderr, "  FAIL %s (timed out after %s)\n", check.name, check.limit)
		} else {
			fmt.Fprintf(stderr, "  FAIL %s (exit %d)\n", check.name, exitCode(err))
		}
		lines := strings.Split(log.String(), "\n")
		if len(lines) > 200 {
			lines = lines[:200]
		}
		fmt.Fprintln(stderr, strings.Join(lines, "\n"))
		fail++
	}
	if err := validateManifestContract(root); err != nil {
		fmt.Fprintf(stderr, "  FAIL marketplace and manifest contract (exit 1)\n%s\n", err)
		fail++
	} else {
		fmt.Fprintln(stdout, "  PASS marketplace and manifest contract")
		pass++
	}

	shells, err := discoverShellFiles(root)
	if err != nil || len(shells) == 0 {
		fmt.Fprintln(stderr, "  FAIL shell syntax (input discovery)")
		fail++
	} else {
		syntaxCtx, cancel := deadline(ctx, 30*time.Second)
		syntaxErr := checkShellSyntax(syntaxCtx, root, shells)
		cancel()
		if syntaxErr != nil {
			fmt.Fprintf(stderr, "  FAIL shell syntax\n%s\n", syntaxErr)
			fail++
		} else {
			fmt.Fprintln(stdout, "  PASS shell syntax")
			pass++
		}
		if executableErr := checkExecutableFiles(root, shells); executableErr != nil {
			fmt.Fprintf(stderr, "  FAIL shell entrypoints executable\n%s\n", executableErr)
			fail++
		} else {
			fmt.Fprintln(stdout, "  PASS shell entrypoints executable")
			pass++
		}
		if executable("shellcheck") {
			var regularShells, commandShims []string
			for _, path := range shells {
				if strings.HasSuffix(path, ".cmd") {
					commandShims = append(commandShims, path)
				} else {
					regularShells = append(regularShells, path)
				}
			}
			if len(regularShells) > 0 {
				run(validation{"shell lint", 60 * time.Second, append([]string{"shellcheck", "--severity=warning"}, regularShells...)})
			}
			if len(commandShims) > 0 {
				run(validation{"polyglot command shim lint", 60 * time.Second, append([]string{"shellcheck", "--shell=sh", "--severity=warning"}, commandShims...)})
			}
		} else {
			fmt.Fprintln(stderr, "SKIPPED: shell lint (shellcheck not installed)")
		}
	}

	goFiles, err := discoverGoFiles(root)
	if err != nil || len(goFiles) == 0 {
		fmt.Fprintln(stderr, "  FAIL Go formatting (input discovery)")
		fail++
	} else {
		formatCtx, cancel := deadline(ctx, 30*time.Second)
		var unformatted bytes.Buffer
		formatErr := command(formatCtx, root, &unformatted, &unformatted, nil, "gofmt", append([]string{"-l"}, goFiles...)...)
		cancel()
		if formatErr != nil || unformatted.Len() != 0 {
			fmt.Fprintf(stderr, "  FAIL Go formatting (exit %d)\n%s", exitCode(formatErr), unformatted.String())
			fail++
		} else {
			fmt.Fprintln(stdout, "  PASS Go formatting")
			pass++
		}
	}
	if err := checkPluginFolderHoldsNoTests(root); err != nil {
		fmt.Fprintf(stderr, "  FAIL plugin folder holds only shipped files (exit 1)\n%s\n", err)
		fail++
	} else {
		fmt.Fprintln(stdout, "  PASS plugin folder holds only shipped files")
		pass++
	}
	// Hook tests live outside the plugin folder and join the hooks package
	// through a build overlay, so vet and test both need it.
	overlayDir, overlayErr := privateTempDir("megapowers-validate-overlay")
	overlay := ""
	if overlayErr == nil {
		defer os.RemoveAll(overlayDir)
		overlay, overlayErr = writeHookTestOverlay(root, overlayDir)
	}
	if overlayErr != nil {
		fmt.Fprintf(stderr, "  FAIL hook test overlay (exit 1)\n%v\n", overlayErr)
		fail++
	}
	if overlayErr == nil {
		run(validation{"Go vet", 2 * time.Minute, []string{"go", "vet", "-overlay", overlay, "./..."}})
	}
	run(validation{"full-tree security lint", 60 * time.Second, []string{"go", "run", "./scripts/cmd/security-lint"}})
	// Freshness (docs/harness-support.md review age) is deliberately not part
	// of this gate: it ages on a calendar, not on content, and would turn
	// every PR/CI run red on a schedule unrelated to the change under review.
	// It stays enforced in the scheduled freshness workflow
	// (.github/workflows/freshness.yml) and in the release gate (runRelease's
	// gates list, scripts/check-freshness.sh).
	if overlayErr == nil {
		if runGoTests(ctx, root, overlay, stdout, stderr) {
			pass++
		} else {
			fail++
		}
	}
	if executable("claude") {
		run(validation{"Claude marketplace strict validation", 90 * time.Second, []string{"claude", "plugin", "validate", "--strict", ".claude-plugin/marketplace.json"}})
		run(validation{"Claude plugin strict validation", 90 * time.Second, []string{"claude", "plugin", "validate", "--strict", "plugins/megapowers"}})
	} else {
		fmt.Fprintln(stderr, "SKIPPED: Claude marketplace strict validation (claude not installed)")
		fmt.Fprintln(stderr, "SKIPPED: Claude plugin strict validation (claude not installed)")
	}
	fmt.Fprintf(stdout, "\n%d passed, %d failed\n", pass, fail)
	if fail != 0 {
		return 1
	}
	return 0
}

func checkShellSyntax(ctx context.Context, root string, files []string) error {
	for _, path := range files {
		log := cappedBuffer{limit: 1 << 20}
		if err := command(ctx, root, &log, &log, nil, "bash", "-n", path); err != nil {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			return fmt.Errorf("%s: %w\n%s", rel, err, strings.TrimSpace(log.String()))
		}
	}
	return nil
}

func checkExecutableFiles(root string, files []string) error {
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			return fmt.Errorf("not executable: %s", rel)
		}
	}
	return nil
}

func validateManifestContract(root string) error {
	type plugin struct {
		Name   string          `json:"name"`
		Source json.RawMessage `json:"source"`
	}
	type marketplace struct {
		Name    string   `json:"name"`
		Plugins []plugin `json:"plugins"`
	}
	readMarketplace := func(rel string) (marketplace, error) {
		var value marketplace
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil {
			err = json.Unmarshal(data, &value)
		}
		return value, err
	}
	claude, err := readMarketplace(".claude-plugin/marketplace.json")
	if err != nil || claude.Name != "megapowers" || len(claude.Plugins) != 1 || claude.Plugins[0].Name != "megapowers" {
		return fmt.Errorf("invalid Claude marketplace")
	}
	var claudeSource string
	if json.Unmarshal(claude.Plugins[0].Source, &claudeSource) != nil || claudeSource != "./plugins/megapowers" {
		return fmt.Errorf("claude marketplace source is not ./plugins/megapowers")
	}
	codex, err := readMarketplace(".agents/plugins/marketplace.json")
	if err != nil || codex.Name != "megapowers" || len(codex.Plugins) != 1 || codex.Plugins[0].Name != "megapowers" {
		return fmt.Errorf("invalid Codex marketplace")
	}
	var codexSource struct {
		Source, Path string
	}
	if json.Unmarshal(codex.Plugins[0].Source, &codexSource) != nil || codexSource.Source != "local" || codexSource.Path != "./plugins/megapowers" {
		return fmt.Errorf("codex marketplace source is not local ./plugins/megapowers")
	}
	claudeVersion, err := manifestVersion(filepath.Join(root, "plugins/megapowers/.claude-plugin/plugin.json"))
	if err != nil {
		return err
	}
	codexVersion, err := manifestVersion(filepath.Join(root, "plugins/megapowers/.codex-plugin/plugin.json"))
	if err != nil || claudeVersion != codexVersion {
		return fmt.Errorf("plugin manifest versions differ")
	}
	changelog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil || !bytes.Contains(changelog, []byte("## "+claudeVersion+" - ")) {
		return fmt.Errorf("CHANGELOG.md has no entry for %s", claudeVersion)
	}
	tracked, err := output(context.Background(), root, nil, "git", "ls-files", "plugins/*")
	if err != nil {
		return err
	}
	plugins := map[string]bool{}
	for _, line := range strings.Split(string(tracked), "\n") {
		parts := strings.Split(line, "/")
		if len(parts) > 1 {
			plugins[parts[1]] = true
		}
	}
	if len(plugins) != 1 || !plugins["megapowers"] {
		return fmt.Errorf("tracked plugins differ from megapowers")
	}
	return nil
}

func discoverShellFiles(root string) ([]string, error) {
	var files []string
	for _, rel := range []string{"scripts", "evals", "plugins/megapowers/hooks"} {
		paths, err := regularFiles(filepath.Join(root, filepath.FromSlash(rel)), ".sh", ".bash", ".cmd")
		if err != nil {
			return nil, err
		}
		files = append(files, paths...)
	}
	sort.Strings(files)
	return files, nil
}

func discoverGoFiles(root string) ([]string, error) {
	var files []string
	for _, rel := range []string{"scripts", "evals", "internal", "plugins/megapowers", "tests"} {
		paths, err := regularFiles(filepath.Join(root, filepath.FromSlash(rel)), ".go")
		if err != nil {
			return nil, err
		}
		files = append(files, paths...)
	}
	sort.Strings(files)
	return files, nil
}

// runGoTests runs go test with -json so a skipped test is named instead of
// counted as passed. Inside the Claude sandbox, TestRunAllReporting skipped
// on a socket denial and hid a stale assertion behind "PASS Go tests".
func runGoTests(ctx context.Context, root, overlay string, stdout, stderr io.Writer) bool {
	checkCtx, cancel := deadline(ctx, 10*time.Minute)
	defer cancel()
	events := &testEvents{log: cappedBuffer{limit: 10 << 20}, output: map[string][]string{}, last: map[string]int{}}
	err := command(checkCtx, root, events, events, nil, "go", "test", "-overlay", overlay, "./...", "-count=1", "-json")
	events.flush()
	if err != nil {
		if checkCtx.Err() != nil {
			fmt.Fprintf(stderr, "  FAIL Go tests (timed out after %s)\n", 10*time.Minute)
		} else {
			fmt.Fprintf(stderr, "  FAIL Go tests (exit %d)\n", exitCode(err))
		}
		lines := strings.Split(events.log.String(), "\n")
		if len(lines) > 200 {
			lines = lines[:200]
		}
		fmt.Fprintln(stderr, strings.Join(lines, "\n"))
		return false
	}
	if len(events.skipped) == 0 {
		fmt.Fprintln(stdout, "  PASS Go tests")
		return true
	}
	fmt.Fprintf(stdout, "  PASS Go tests (%d skipped)\n", len(events.skipped))
	for _, name := range events.skipped {
		fmt.Fprintf(stdout, "    skipped: %s\n", name)
	}
	return true
}

// testEvents reads go test -json output line by line. exec.Cmd serializes
// writes when stdout and stderr share this writer. log keeps only output of
// tests and packages that failed, plus non-JSON text such as build errors,
// so a failure report is not filled with passing output.
type testEvents struct {
	pending []byte
	log     cappedBuffer
	output  map[string][]string
	last    map[string]int
	seq     int
	skipped []string
}

func (e *testEvents) Write(p []byte) (int, error) {
	e.pending = append(e.pending, p...)
	for {
		i := bytes.IndexByte(e.pending, '\n')
		if i < 0 {
			return len(p), nil
		}
		e.line(e.pending[:i])
		e.pending = e.pending[i+1:]
	}
}

// blockLines caps one test's output in the failure report, so a noisy test
// cannot push other diagnostics past the report's line limit.
const blockLines = 40

// writeBlock writes one test's buffered output, keeping its last lines,
// where a failure usually ends.
func (e *testEvents) writeBlock(key string) {
	lines := strings.SplitAfter(strings.Join(e.output[key], ""), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > blockLines {
		dropped := len(lines) - blockLines
		lines = append([]string{fmt.Sprintf("... %d earlier lines of this test omitted\n", dropped)}, lines[dropped:]...)
	}
	_, _ = e.log.Write([]byte(strings.Join(lines, "")))
	delete(e.output, key)
}

func (e *testEvents) flush() {
	if len(e.pending) > 0 {
		e.line(e.pending)
		e.pending = nil
	}
}

func (e *testEvents) line(raw []byte) {
	var event struct {
		Action, Package, Test, Output, ImportPath string
	}
	if json.Unmarshal(raw, &event) != nil || event.Action == "" {
		// Build failures and other stderr text are not JSON events.
		_, _ = e.log.Write(append(append([]byte{}, raw...), '\n'))
		return
	}
	key := event.Package + "\x00" + event.Test
	switch event.Action {
	case "output":
		e.output[key] = append(e.output[key], event.Output)
		e.seq++
		e.last[key] = e.seq
	case "skip":
		if event.Test != "" {
			e.skipped = append(e.skipped, event.Package+" "+event.Test)
		}
		delete(e.output, key)
	case "pass":
		delete(e.output, key)
	case "fail":
		if event.Test != "" {
			e.writeBlock(key)
			return
		}
		// A package failure also flushes output of tests that never got
		// their own fail event, such as a test that called os.Exit. The
		// test that wrote last is most likely the one that exited, so it
		// goes first and survives the report's line cap.
		prefix := event.Package + "\x00"
		var keys []string
		for k := range e.output {
			if strings.HasPrefix(k, prefix) {
				keys = append(keys, k)
			}
		}
		sort.Slice(keys, func(i, j int) bool { return e.last[keys[i]] > e.last[keys[j]] })
		for _, k := range keys {
			e.writeBlock(k)
		}
	case "build-output":
		// Compiler and linker diagnostics arrive as build events, not as
		// test output.
		_, _ = e.log.Write([]byte(event.Output))
	}
}
