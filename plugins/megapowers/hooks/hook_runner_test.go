package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRunDenyDestructive(t *testing.T) {
	t.Parallel()

	input := `{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`
	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"deny-destructive"}, func(string) string { return "" }, strings.NewReader(input), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runHook returned %d: %s", rc, stderr.String())
	}

	var output struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode hook output: %v; output=%q", err, stdout.String())
	}
	if output.HookSpecificOutput.HookEventName != "PreToolUse" || output.HookSpecificOutput.PermissionDecision != "deny" || output.HookSpecificOutput.PermissionDecisionReason == "" {
		t.Fatalf("unexpected hook output: %+v", output.HookSpecificOutput)
	}
}

func TestRunDenyDestructiveSilentAllow(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"deny-destructive"}, func(string) string { return "" }, strings.NewReader(`{"tool_input":{"command":"git status"}}`), &stdout, &stderr)
	if rc != 0 || stdout.Len() != 0 {
		t.Fatalf("runHook rc=%d stdout=%q stderr=%q, want silent success", rc, stdout.String(), stderr.String())
	}
}

func TestRunDenyDestructiveRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"deny-destructive"}, func(string) string { return "" }, strings.NewReader(`{}`), &stdout, &stderr)
	if rc == 0 || !strings.Contains(stderr.String(), "cannot evaluate command input") {
		t.Fatalf("runHook rc=%d stderr=%q, want visible input error", rc, stderr.String())
	}
}

func TestRunDenyDestructiveRejectsOversizedInput(t *testing.T) {
	t.Parallel()

	input := `{"padding":"` + strings.Repeat("x", maxHookInputBytes) + `","tool_input":{"command":"true"}}`
	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"deny-destructive"}, func(string) string { return "" }, strings.NewReader(input), &stdout, &stderr)
	if rc == 0 || !strings.Contains(stderr.String(), "cannot evaluate command input") {
		t.Fatalf("runHook rc=%d stderr=%q, want visible bounded-input error", rc, stderr.String())
	}
}

func TestRunDenyDestructiveAcceptsFullCodexEvent(t *testing.T) {
	t.Parallel()

	input := `{
		"session_id":"11111111-1111-1111-1111-111111111111",
		"turn_id":"turn-abc",
		"transcript_path":"/tmp/transcript.jsonl",
		"cwd":"/work/repo",
		"hook_event_name":"PreToolUse",
		"model":"gpt-5.5",
		"permission_mode":"default",
		"tool_name":"PowerShell",
		"tool_use_id":"call-xyz",
		"tool_input":{"command":"Remove-Item -Recurse C:\\"}
	}`
	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"deny-destructive"}, func(string) string { return "" }, strings.NewReader(input), &stdout, &stderr)
	if rc != 0 || !strings.Contains(stdout.String(), `"permissionDecision":"deny"`) {
		t.Fatalf("runHook rc=%d stdout=%q stderr=%q, want Codex deny", rc, stdout.String(), stderr.String())
	}
}

func TestRunHookRejectsUnknownHook(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"unknown"}, func(string) string { return "" }, strings.NewReader(`{}`), &stdout, &stderr)
	if rc == 0 || !strings.Contains(stderr.String(), "cannot run unknown hook") {
		t.Fatalf("runHook rc=%d stderr=%q, want visible unknown-hook error", rc, stderr.String())
	}
}

func TestRequireGo125(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		version string
		ok      bool
	}{
		{version: "go1.23.9"},
		{version: "go1.24"},
		{version: "go1.25.1", ok: true},
		{version: "devel go1.26-abcdef", ok: true},
		{version: "unknown"},
	} {
		t.Run(tt.version, func(t *testing.T) {
			err := requireGo125(tt.version)
			if (err == nil) != tt.ok {
				t.Fatalf("requireGo125(%q) error = %v, want ok=%v", tt.version, err, tt.ok)
			}
		})
	}
}

func TestLauncherBuildsOnceAndExecutesCachedRunner(t *testing.T) {
	cache := t.TempDir()
	launcher := filepath.Join(packageDir(t), "run-hook.cmd")
	input := `{"tool_input":{"command":"rm -rf /"}}`

	run := func() string {
		t.Helper()
		cmd := exec.Command("bash", launcher, "deny-destructive")
		cmd.Stdin = strings.NewReader(input)
		cmd.Env = append(os.Environ(),
			"MEGAPOWERS_HOOK_CACHE="+cache,
			"HOME=/home/tester",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run launcher: %v\n%s", err, output)
		}
		return string(output)
	}

	if output := run(); !strings.Contains(output, `"permissionDecision":"deny"`) {
		t.Fatalf("first launcher output = %q", output)
	}
	binaries, err := filepath.Glob(filepath.Join(cache, "megapowers-hooks", "megapowers-hook-*"))
	if err != nil || len(binaries) != 1 {
		t.Fatalf("cached binaries = %v, error = %v, want one", binaries, err)
	}
	info, err := os.Stat(binaries[0])
	if err != nil {
		t.Fatal(err)
	}
	firstModTime := info.ModTime()
	time.Sleep(10 * time.Millisecond)
	if output := run(); !strings.Contains(output, `"permissionDecision":"deny"`) {
		t.Fatalf("second launcher output = %q", output)
	}
	info, err = os.Stat(binaries[0])
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(firstModTime) {
		t.Fatalf("cached runner was rebuilt: first mtime %s, second %s", firstModTime, info.ModTime())
	}
}

func TestLauncherUsesPrivateHomeCacheAndPlatformKey(t *testing.T) {
	home := t.TempDir()
	cmd := exec.Command("bash", filepath.Join(packageDir(t), "run-hook.cmd"), "deny-destructive")
	cmd.Stdin = strings.NewReader(`{"tool_input":{"command":"true"}}`)
	cmd.Env = append(environmentWithout("MEGAPOWERS_HOOK_CACHE", "XDG_CACHE_HOME", "HOME"), "HOME="+home)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run launcher: %v\n%s", err, output)
	}
	cacheDir := filepath.Join(home, ".cache", "megapowers-hooks")
	info, err := os.Stat(cacheDir)
	if err != nil {
		t.Fatalf("stat private cache: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("cache permissions = %04o, want 0700", info.Mode().Perm())
	}
	binaries, err := filepath.Glob(filepath.Join(cacheDir, "megapowers-hook-*-*-*"))
	if err != nil || len(binaries) != 1 {
		t.Fatalf("platform-keyed binaries = %v, error = %v, want one", binaries, err)
	}
}

func TestLauncherDefaultsGoCacheUnderPrivateHookCache(t *testing.T) {
	cache := t.TempDir()
	fakeHome := filepath.Join(t.TempDir(), "home-file")
	if err := os.WriteFile(fakeHome, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", filepath.Join(packageDir(t), "run-hook.cmd"), "deny-destructive")
	cmd.Stdin = strings.NewReader(`{"tool_input":{"command":"true"}}`)
	cmd.Env = append(
		environmentWithout("GOCACHE", "XDG_CACHE_HOME", "HOME", "MEGAPOWERS_HOOK_CACHE"),
		"HOME="+fakeHome,
		"MEGAPOWERS_HOOK_CACHE="+cache,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cold launcher with private hook cache: %v\n%s", err, output)
	}
	goCache := filepath.Join(cache, "megapowers-hooks", "go-build")
	if info, err := os.Stat(goCache); err != nil || !info.IsDir() {
		t.Fatalf("private Go cache missing at %s: info=%v err=%v", goCache, info, err)
	}
}

func TestLauncherPreservesCallerGoCache(t *testing.T) {
	cache := t.TempDir()
	goCache := t.TempDir()
	cmd := exec.Command("bash", filepath.Join(packageDir(t), "run-hook.cmd"), "deny-destructive")
	cmd.Stdin = strings.NewReader(`{"tool_input":{"command":"true"}}`)
	cmd.Env = append(
		environmentWithout("GOCACHE", "MEGAPOWERS_HOOK_CACHE"),
		"GOCACHE="+goCache,
		"MEGAPOWERS_HOOK_CACHE="+cache,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cold launcher with caller Go cache: %v\n%s", err, output)
	}
	entries, err := os.ReadDir(goCache)
	if err != nil || len(entries) == 0 {
		t.Fatalf("caller Go cache was not used: entries=%d err=%v", len(entries), err)
	}
	defaultCache := filepath.Join(cache, "megapowers-hooks", "go-build")
	if _, err := os.Stat(defaultCache); !os.IsNotExist(err) {
		t.Fatalf("launcher created default Go cache despite caller GOCACHE: %v", err)
	}
}

func TestLauncherRejectsSymlinkCache(t *testing.T) {
	base := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(base, "megapowers-hooks")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", filepath.Join(packageDir(t), "run-hook.cmd"), "deny-destructive")
	cmd.Stdin = strings.NewReader(`{"tool_input":{"command":"true"}}`)
	cmd.Env = append(os.Environ(), "MEGAPOWERS_HOOK_CACHE="+base)
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "refusing symlink hook cache") {
		t.Fatalf("launcher error = %v, output = %q, want symlink rejection", err, output)
	}
}

func TestLauncherRejectsSymlinkRunner(t *testing.T) {
	base := t.TempDir()
	cacheDir := filepath.Join(base, "megapowers-hooks")
	if err := os.Mkdir(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	platformOS := commandOutput(t, "uname", "-s")
	platformArch := commandOutput(t, "uname", "-m")
	runner := filepath.Join(cacheDir, "megapowers-hook-"+runnerSourceHash(t)+"-"+platformOS+"-"+platformArch)
	if err := os.Symlink(filepath.Join(t.TempDir(), "attacker-runner"), runner); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", filepath.Join(packageDir(t), "run-hook.cmd"), "deny-destructive")
	cmd.Stdin = strings.NewReader(`{"tool_input":{"command":"true"}}`)
	cmd.Env = append(os.Environ(), "MEGAPOWERS_HOOK_CACHE="+base)
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "refusing symlink cached runner") {
		t.Fatalf("launcher error = %v, output = %q, want runner symlink rejection", err, output)
	}
}

func TestLauncherHandlesPluginPathWithSpaces(t *testing.T) {
	pluginRoot := filepath.Dir(packageDir(t))
	linkedRoot := filepath.Join(t.TempDir(), "plugin root")
	if err := os.Symlink(pluginRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", filepath.Join(linkedRoot, "hooks", "run-hook.cmd"), "output-style")
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"SessionStart","source":"startup"}`)
	cmd.Env = append(os.Environ(),
		"MEGAPOWERS_HOOK_CACHE="+t.TempDir(),
		"MEGAPOWERS_HARNESS=codex",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run launcher through spaced path: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "ASD-STE100-inspired principles") {
		t.Fatalf("launcher output omits shared style: %q", output)
	}
}

func TestHookManifestUsesGoRunnerWithoutStopGate(t *testing.T) {
	manifest, err := os.ReadFile(filepath.Join(packageDir(t), "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(manifest, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, exists := parsed.Hooks["Stop"]; exists {
		t.Fatal("hooks manifest must not add a universal Stop gate")
	}
	session := parsed.Hooks["SessionStart"]
	if len(session) != 1 || len(session[0].Hooks) != 1 || !strings.HasSuffix(session[0].Hooks[0].Command, "run-hook.cmd session-start") || session[0].Hooks[0].Timeout < 30 {
		t.Fatalf("unexpected SessionStart hook: %+v", session)
	}
	preTool := parsed.Hooks["PreToolUse"]
	if len(preTool) != 1 || preTool[0].Matcher != "Bash|PowerShell" || len(preTool[0].Hooks) != 1 || !strings.HasSuffix(preTool[0].Hooks[0].Command, "run-hook.cmd deny-destructive") || preTool[0].Hooks[0].Timeout < 30 {
		t.Fatalf("unexpected PreToolUse hook: %+v", preTool)
	}
}

func TestLauncherCacheKeyMatchesRunnerSources(t *testing.T) {
	t.Parallel()

	want := runnerSourceHash(t)
	launcher, err := os.ReadFile(filepath.Join(packageDir(t), "run-hook.cmd"))
	if err != nil {
		t.Fatal(err)
	}
	matches := regexp.MustCompile(`megapowers-hook-([0-9a-f]+)`).FindAllStringSubmatch(string(launcher), -1)
	if len(matches) != 2 {
		t.Fatalf("launcher cache-key occurrences = %d, want 2", len(matches))
	}
	for _, match := range matches {
		if match[1] != want {
			t.Fatalf("launcher cache key = %s, want source hash %s", match[1], want)
		}
	}
}

func runnerSourceHash(t *testing.T) string {
	t.Helper()
	hash := sha256.New()
	for _, name := range []string{"deny_destructive.go", "hook_runner.go", "output_style.go"} {
		content, err := os.ReadFile(filepath.Join(packageDir(t), name))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(hash, "%s\x00", name)
		hash.Write(content)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))[:16]
}

func TestLauncherFailsClearlyWithoutGoOrCache(t *testing.T) {
	binDir := t.TempDir()
	for _, name := range []string{"chmod", "dirname", "mkdir", "uname"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(path, filepath.Join(binDir, name)); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("/bin/bash", filepath.Join(packageDir(t), "run-hook.cmd"), "deny-destructive")
	cmd.Stdin = strings.NewReader(`{"tool_input":{"command":"true"}}`)
	cmd.Env = []string{
		"PATH=" + binDir,
		"MEGAPOWERS_HOOK_CACHE=" + t.TempDir(),
	}
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "Go 1.25 or newer is required") {
		t.Fatalf("launcher error = %v, output = %q, want explicit Go requirement", err, output)
	}
}

func commandOutput(t *testing.T, name string, args ...string) string {
	t.Helper()
	output, err := exec.Command(name, args...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}

func packageDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func environmentWithout(keys ...string) []string {
	blocked := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		blocked[key] = struct{}{}
	}
	result := make([]string, 0, len(os.Environ()))
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, skip := blocked[key]; !skip {
			result = append(result, item)
		}
	}
	return result
}
