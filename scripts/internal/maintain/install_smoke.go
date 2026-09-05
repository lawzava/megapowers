package maintain

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const researchQuote = "A research conclusion is not authority to implement or publish."

type smokeOptions struct {
	out, repo, source, ref, version string
	harnesses                       []string
	failOnSkip                      bool
}

type smokeRecorder struct {
	file   *os.File
	stdout io.Writer
}

func (r *smokeRecorder) note(harness, status, message string) {
	line := fmt.Sprintf("%s\t%s\t%s\n", harness, status, message)
	fmt.Fprint(r.stdout, line)
	fmt.Fprint(r.file, line)
}

func runInstallSmoke(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--selftest" {
		return installSmokeSelftest(stdout)
	}
	opts, err := parseSmokeOptions(root, args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := os.MkdirAll(opts.out, 0o755); err != nil {
		fmt.Fprintln(stderr, "install smoke: cannot create output directory")
		return 2
	}
	absoluteOut, err := filepath.Abs(opts.out)
	if err != nil {
		fmt.Fprintln(stderr, "install smoke: cannot resolve output directory")
		return 2
	}
	opts.out = absoluteOut
	resultsPath := filepath.Join(opts.out, "results.tsv")
	results, err := os.Create(resultsPath)
	if err != nil {
		fmt.Fprintln(stderr, "install smoke: cannot create results.tsv")
		return 2
	}
	defer results.Close()
	recorder := &smokeRecorder{file: results, stdout: stdout}

	if opts.source != "" {
		fetched, cleanup, fetchErr := fetchReleaseSource(ctx, opts, recorder)
		err = fetchErr
		if err != nil {
			return 1
		}
		defer cleanup()
		opts.repo = fetched
	}

	version, err := verifySmokeSource(opts.repo, opts.version)
	if err != nil {
		recorder.note("source", "FAIL", err.Error())
		return 1
	}
	for _, harness := range opts.harnesses {
		switch harness {
		case "claude":
			smokeClaude(ctx, opts, version, recorder)
		case "codex":
			smokeCodex(ctx, opts, version, recorder)
		}
	}
	if err := results.Sync(); err != nil {
		fmt.Fprintln(stderr, "install smoke: cannot persist results")
		return 1
	}
	data, err := os.ReadFile(resultsPath)
	if err != nil {
		fmt.Fprintln(stderr, "install smoke: cannot read results")
		return 1
	}
	fmt.Fprintln(stdout, "\n== install-smoke summary ==")
	fmt.Fprint(stdout, string(data))
	if resultsOK(data, opts.failOnSkip) {
		return 0
	}
	fmt.Fprintln(stderr, "install smoke failed: FAIL, strict SKIP, or no PASS result")
	return 1
}

func parseSmokeOptions(root string, args []string) (smokeOptions, error) {
	opts := smokeOptions{repo: root, harnesses: []string{"claude", "codex"}}
	for len(args) > 0 {
		switch args[0] {
		case "--fail-on-skip":
			opts.failOnSkip = true
			args = args[1:]
		case "--out", "--repo", "--source", "--ref", "--version", "--harnesses":
			if len(args) < 2 {
				return opts, fmt.Errorf("%s requires a value", args[0])
			}
			flag, value := args[0], args[1]
			switch flag {
			case "--out":
				opts.out = value
			case "--repo":
				opts.repo = value
			case "--source":
				opts.source = value
			case "--ref":
				opts.ref = value
			case "--version":
				opts.version = value
			case "--harnesses":
				opts.harnesses = strings.Split(value, ",")
			}
			args = args[2:]
		default:
			return opts, fmt.Errorf("unknown arg: %s", args[0])
		}
	}
	if opts.out == "" {
		return opts, errors.New("usage: run-smoke.sh --out DIR [--harnesses claude,codex] [--repo DIR | --source OWNER/REPO --ref TAG --version VERSION]")
	}
	if len(opts.harnesses) == 0 {
		return opts, errors.New("at least one harness is required")
	}
	for _, harness := range opts.harnesses {
		if harness != "claude" && harness != "codex" {
			return opts, fmt.Errorf("unsupported harness: %s", harness)
		}
	}
	if opts.source != "" {
		if opts.repo != root {
			return opts, errors.New("--repo and --source are mutually exclusive")
		}
		if opts.ref == "" || opts.version == "" {
			return opts, errors.New("--source requires --ref and --version")
		}
		opts.failOnSkip = true
	} else if opts.ref != "" || opts.version != "" {
		return opts, errors.New("--ref and --version require --source")
	}
	return opts, nil
}

func installSmokeSelftest(stdout io.Writer) int {
	tmp, err := privateTempDir("megapowers-install-selftest")
	if err != nil {
		fmt.Fprintln(stdout, "install-smoke selftest: FAIL")
		return 1
	}
	defer os.RemoveAll(tmp)
	failed := false
	check := func(ok bool, success, failure string) {
		if ok {
			fmt.Fprintln(stdout, "ok  ", success)
		} else {
			fmt.Fprintln(stdout, "FAIL", failure)
			failed = true
		}
	}
	quoteOK := func(data []byte) bool {
		return bytes.Contains(bytes.ReplaceAll(data, []byte{'\n'}, []byte{' '}), []byte(researchQuote))
	}
	check(quoteOK([]byte(researchQuote+"\n")), "verbatim sentence matches", "verbatim sentence not matched")
	check(!quoteOK([]byte("A Research conclusion is not authority to implement or publish.")), "case change rejected", "case change matched")
	check(!quoteOK([]byte("Research does not automatically allow a change.")), "generic phrasing rejected", "generic phrasing matched")
	source, installed := filepath.Join(tmp, "source"), filepath.Join(tmp, "installed")
	os.Mkdir(source, 0o755)
	os.Mkdir(installed, 0o755)
	os.WriteFile(filepath.Join(source, "skill"), []byte(researchQuote+"\n"), 0o644)
	os.WriteFile(filepath.Join(installed, "skill"), []byte(researchQuote+"\n"), 0o644)
	check(compareTrees(source, installed) == nil, "identical installed tree accepted", "identical installed tree rejected")
	os.WriteFile(filepath.Join(installed, "skill"), []byte("mutation\n"), 0o644)
	check(compareTrees(source, installed) != nil, "mutated installed tree rejected", "mutated installed tree accepted")
	os.WriteFile(filepath.Join(installed, "skill"), []byte(researchQuote+"\n"), 0o600)
	os.Chmod(filepath.Join(installed, "skill"), 0o600)
	check(compareTrees(source, installed) != nil, "installed mode change rejected", "installed mode change accepted")
	check(!resultsOK([]byte("claude\tSKIP\tunavailable\n"), false), "all-SKIP results rejected", "all-SKIP results accepted")
	mixed := []byte("claude\tPASS\tinstalled\ncodex\tSKIP\tunavailable\n")
	check(resultsOK(mixed, false), "optional SKIP accepted with a PASS", "optional SKIP rejected")
	check(!resultsOK(mixed, true), "strict SKIP rejected", "strict SKIP accepted")
	if failed {
		fmt.Fprintln(stdout, "install-smoke selftest: FAIL")
		return 1
	}
	fmt.Fprintln(stdout, "install-smoke selftest: PASS")
	return 0
}

func fetchReleaseSource(ctx context.Context, opts smokeOptions, recorder *smokeRecorder) (string, func(), error) {
	tmp, err := privateTempDir("megapowers-install-fetch")
	if err != nil {
		recorder.note("source", "FAIL", "cannot create private fetch directory")
		return "", func() {}, err
	}
	cleanup := func() { os.RemoveAll(tmp) }
	remote := opts.source
	if !strings.Contains(remote, "://") && !strings.HasPrefix(remote, "git@") {
		remote = "https://github.com/" + remote + ".git"
	}
	fetchCtx, cancel := deadline(ctx, 5*time.Minute)
	defer cancel()
	log, err := os.Create(filepath.Join(opts.out, "fetch.log"))
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	err = command(fetchCtx, opts.repo, log, log, nil, "git", "clone", "--quiet", "--depth", "1", "--branch", opts.ref, remote, filepath.Join(tmp, "repo"))
	log.Close()
	if err != nil {
		recorder.note("source", "FAIL", fmt.Sprintf("fetch exact ref %s@%s, see fetch.log", opts.source, opts.ref))
		cleanup()
		return "", func() {}, err
	}
	repo := filepath.Join(tmp, "repo")
	shaBytes, err := output(ctx, repo, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	sha := strings.TrimSpace(string(shaBytes))
	tags, err := output(ctx, repo, nil, "git", "tag", "--points-at", "HEAD")
	if err != nil || !linePresent(tags, opts.ref) {
		recorder.note("source", "FAIL", fmt.Sprintf("fetched HEAD %s is not exact tag %s", sha, opts.ref))
		cleanup()
		return "", func() {}, errors.New("fetched ref is not an exact tag")
	}
	metadata := map[string]string{"source": opts.source, "ref": opts.ref, "version": opts.version, "sha": sha, "mode": "exact-remote-ref"}
	data, _ := json.MarshalIndent(metadata, "", "  ")
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(opts.out, "source.json"), data, 0o644); err != nil {
		cleanup()
		return "", func() {}, err
	}
	recorder.note("source", "PASS", fmt.Sprintf("fetched %s@%s at %s", opts.source, opts.ref, sha))
	return repo, cleanup, nil
}

func linePresent(data []byte, want string) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

func verifySmokeSource(repo, requestedVersion string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(repo, "plugins"))
	if err != nil {
		return "", errors.New("checkout must contain exactly the megapowers plugin")
	}
	var plugins []string
	for _, entry := range entries {
		if entry.IsDir() {
			plugins = append(plugins, entry.Name())
		}
	}
	if len(plugins) != 1 || plugins[0] != "megapowers" {
		return "", errors.New("checkout must contain exactly the megapowers plugin")
	}
	claudeVersion, err := manifestVersion(filepath.Join(repo, "plugins/megapowers/.claude-plugin/plugin.json"))
	if err != nil {
		return "", errors.New("claude manifest has no version")
	}
	codexVersion, err := manifestVersion(filepath.Join(repo, "plugins/megapowers/.codex-plugin/plugin.json"))
	if err != nil {
		return "", errors.New("codex manifest has no version")
	}
	if claudeVersion != codexVersion {
		return "", errors.New("source manifest versions differ")
	}
	expected := claudeVersion
	if requestedVersion != "" {
		expected = requestedVersion
	}
	if claudeVersion != expected {
		return "", fmt.Errorf("source manifest version is not %s", expected)
	}
	return expected, nil
}

func smokeClaude(ctx context.Context, opts smokeOptions, version string, recorder *smokeRecorder) {
	if !executable("claude") {
		recorder.note("claude", "SKIP", "claude CLI not installed")
		return
	}
	home, err := privateTempDir("megapowers-claude-home")
	if err != nil {
		recorder.note("claude", "FAIL", "cannot create fresh home")
		return
	}
	defer os.RemoveAll(home)
	env := []string{"CLAUDE_CONFIG_DIR=" + home}
	if !runLogged(ctx, 5*time.Minute, opts.repo, opts.out, "claude-marketplace", env, "claude", "plugin", "marketplace", "add", opts.repo) {
		recorder.note("claude", "FAIL", "marketplace add, see claude-marketplace.log")
		return
	}
	recorder.note("claude", "PASS", "marketplace add (local path)")
	if !runLogged(ctx, 5*time.Minute, opts.repo, opts.out, "claude-install", env, "claude", "plugin", "install", "megapowers@megapowers") {
		recorder.note("claude", "FAIL", "plugin install, see claude-install.log")
		return
	}
	recorder.note("claude", "PASS", "plugin install megapowers@megapowers")
	data, ok := runJSON(ctx, 2*time.Minute, opts.repo, opts.out, "claude-list", env, "claude", "plugin", "list", "--json")
	if !ok {
		recorder.note("claude", "FAIL", "plugin list JSON, see claude-list.err")
		return
	}
	var installed []struct {
		ID, Version, InstallPath string
		Enabled                  bool
	}
	if json.Unmarshal(data, &installed) != nil {
		recorder.note("claude", "FAIL", "installed plugin missing from registration JSON")
		return
	}
	for _, plugin := range installed {
		if plugin.ID == "megapowers@megapowers" && plugin.Version == version && plugin.Enabled {
			verifyInstalledBytes(ctx, opts.repo, "claude", home, plugin.InstallPath, version, recorder)
			return
		}
	}
	recorder.note("claude", "FAIL", "installed plugin missing from registration JSON")
}

func smokeCodex(ctx context.Context, opts smokeOptions, version string, recorder *smokeRecorder) {
	if !executable("codex") {
		recorder.note("codex", "SKIP", "codex CLI not installed")
		return
	}
	home, err := privateTempDir("megapowers-codex-home")
	if err != nil {
		recorder.note("codex", "FAIL", "cannot create fresh home")
		return
	}
	defer os.RemoveAll(home)
	env := []string{"CODEX_HOME=" + home}
	if !runLogged(ctx, 5*time.Minute, opts.repo, opts.out, "codex-marketplace", env, "codex", "plugin", "marketplace", "add", opts.repo, "--json") {
		recorder.note("codex", "FAIL", "marketplace add, see codex-marketplace.err")
		return
	}
	recorder.note("codex", "PASS", "marketplace add (local path)")
	data, ok := runJSON(ctx, 5*time.Minute, opts.repo, opts.out, "codex-install", env, "codex", "plugin", "add", "megapowers@megapowers", "--json")
	if !ok {
		recorder.note("codex", "FAIL", "plugin add, see codex-install.err")
		return
	}
	var added struct {
		PluginID, Version, InstalledPath string
	}
	if json.Unmarshal(data, &added) != nil || added.PluginID != "megapowers@megapowers" || added.Version != version || added.InstalledPath == "" {
		recorder.note("codex", "FAIL", "plugin add JSON missing installedPath or expected version")
		return
	}
	listData, ok := runJSON(ctx, 2*time.Minute, opts.repo, opts.out, "codex-list", env, "codex", "plugin", "list", "--json")
	if !ok || !codexRegistrationPresent(listData, version) {
		recorder.note("codex", "FAIL", "installed plugin missing from registration JSON")
		return
	}
	if !verifyInstalledBytes(ctx, opts.repo, "codex", home, added.InstalledPath, version, recorder) {
		return
	}
	policyExposed, err := verifyCodexSkillPolicy(ctx, home, added.InstalledPath)
	if err != nil {
		recorder.note("codex", "FAIL", "skills/list policy check: "+err.Error())
		return
	}
	if policyExposed {
		recorder.note("codex", "PASS", "skills/list exposes installed enabled memory-hygiene and writing-agent-instructions with explicit-only memory policy")
	} else {
		recorder.note("codex", "PASS", "skills/list exposes installed enabled memory-hygiene and writing-agent-instructions; installed memory policy is explicit-only; API omits effective invocation policy")
	}
}

func runLogged(parent context.Context, limit time.Duration, root, out, stem string, env []string, name string, args ...string) bool {
	ctx, cancel := deadline(parent, limit)
	defer cancel()
	stdout, err := os.Create(filepath.Join(out, stem+".log"))
	if err != nil {
		return false
	}
	defer stdout.Close()
	return command(ctx, root, stdout, stdout, env, name, args...) == nil
}

func runJSON(parent context.Context, limit time.Duration, root, out, stem string, env []string, name string, args ...string) ([]byte, bool) {
	ctx, cancel := deadline(parent, limit)
	defer cancel()
	stdout := cappedBuffer{limit: 10 << 20}
	stderr, err := os.Create(filepath.Join(out, stem+".err"))
	if err != nil {
		return nil, false
	}
	defer stderr.Close()
	if command(ctx, root, &stdout, stderr, env, name, args...) != nil {
		return nil, false
	}
	if stdout.overflow {
		return nil, false
	}
	data := stdout.Bytes()
	if err := os.WriteFile(filepath.Join(out, stem+".json"), data, 0o644); err != nil {
		return nil, false
	}
	return data, true
}

func codexRegistrationPresent(data []byte, version string) bool {
	var list struct {
		Installed []struct {
			PluginID, Version  string
			Installed, Enabled bool
		} `json:"installed"`
	}
	if json.Unmarshal(data, &list) != nil {
		return false
	}
	for _, plugin := range list.Installed {
		if plugin.PluginID == "megapowers@megapowers" && plugin.Version == version && plugin.Installed && plugin.Enabled {
			return true
		}
	}
	return false
}

func verifyInstalledBytes(ctx context.Context, repo, harness, home, installedPath, version string, recorder *smokeRecorder) bool {
	absoluteHome, err := filepath.Abs(home)
	if err != nil {
		recorder.note(harness, "FAIL", "cannot resolve fresh home")
		return false
	}
	absoluteInstall, err := filepath.Abs(installedPath)
	cacheRoot := filepath.Join(absoluteHome, "plugins", "cache") + string(os.PathSeparator)
	if err != nil || !strings.HasPrefix(absoluteInstall+string(os.PathSeparator), cacheRoot) {
		recorder.note(harness, "FAIL", "reported install path is outside fresh home")
		return false
	}
	for _, manifest := range []string{".claude-plugin/plugin.json", ".codex-plugin/plugin.json"} {
		declared, err := manifestVersion(filepath.Join(absoluteInstall, filepath.FromSlash(manifest)))
		if err != nil || declared != version {
			recorder.note(harness, "FAIL", "cached manifest or version differs")
			return false
		}
	}
	skill := filepath.Join(absoluteInstall, "skills/evidence-research/SKILL.md")
	sourceSkill := filepath.Join(repo, "plugins/megapowers/skills/evidence-research/SKILL.md")
	a, errA := os.ReadFile(sourceSkill)
	b, errB := os.ReadFile(skill)
	if errA != nil || errB != nil || !bytes.Equal(a, b) || !bytes.Contains(bytes.ReplaceAll(b, []byte{'\n'}, []byte{' '}), []byte(researchQuote)) {
		recorder.note(harness, "FAIL", "cached evidence-research bytes differ")
		return false
	}
	if err := compareTrees(filepath.Join(repo, "plugins/megapowers"), absoluteInstall); err != nil {
		recorder.note(harness, "FAIL", "cached plugin tree differs in paths, modes, or bytes")
		return false
	}
	if !hookCommandsUseWrapper(filepath.Join(absoluteInstall, "hooks/hooks.json")) {
		recorder.note(harness, "FAIL", "cached hook commands do not use the shared wrapper")
		return false
	}
	if err := verifyInstalledHookRuntime(ctx, absoluteHome, absoluteInstall); err != nil {
		recorder.note(harness, "FAIL", "cached hook runtime: "+err.Error())
		return false
	}
	recorder.note(harness, "PASS", fmt.Sprintf("registered megapowers %s with exact cached tree and executable hooks", version))
	return true
}

func verifyInstalledHookRuntime(parent context.Context, home, installedRoot string) error {
	launcher := filepath.Join(installedRoot, "hooks", "run-hook.cmd")
	cacheBase := filepath.Join(home, "hook-cache")
	goCache := filepath.Join(home, "go-cache")
	for _, dir := range []string{cacheBase, goCache} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return errors.New("cannot create private hook cache")
		}
	}
	env := mergedEnv([]string{
		"HOME=" + home,
		"MEGAPOWERS_HOOK_CACHE=" + cacheBase,
		"GOCACHE=" + goCache,
		"MEGAPOWERS_OUTPUT_STYLE=off",
	})
	run := func(hook, input string, overrides ...string) ([]byte, []byte, error) {
		ctx, cancel := deadline(parent, 2*time.Minute)
		defer cancel()
		var name string
		var args []string
		if runtime.GOOS == "windows" {
			name, args = "cmd.exe", []string{"/c", launcher, hook}
		} else {
			name, args = "bash", []string{launcher, hook}
		}
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = installedRoot
		cmd.Env = append(append([]string(nil), env...), overrides...)
		cmd.Stdin = strings.NewReader(input)
		stdout := cappedBuffer{limit: 1 << 20}
		stderr := cappedBuffer{limit: 1 << 20}
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.WaitDelay = 5 * time.Second
		err := cmd.Run()
		if ctx.Err() != nil {
			return nil, nil, errors.New("hook execution timed out")
		}
		if stdout.overflow || stderr.overflow {
			return nil, nil, errors.New("hook output exceeded 1 MiB")
		}
		return stdout.Bytes(), stderr.Bytes(), err
	}

	safe := `{"tool_name":"Bash","tool_input":{"command":"git status --short"}}`
	stdout, stderr, err := run("deny-destructive", safe)
	if err != nil || len(stdout) != 0 || len(stderr) != 0 {
		return errors.New("safe command did not produce silent success")
	}
	runners, err := filepath.Glob(filepath.Join(cacheBase, "megapowers-hooks", "megapowers-hook-*"))
	if err != nil || len(runners) != 1 {
		return errors.New("cold hook call did not create exactly one cached runner")
	}
	info, err := os.Lstat(runners[0])
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o111 == 0 {
		return errors.New("cached hook runner is not a regular executable")
	}
	firstBytes, err := os.ReadFile(runners[0])
	if err != nil {
		return errors.New("cannot read cached hook runner")
	}
	firstHash := sha256.Sum256(firstBytes)
	firstModTime := info.ModTime()

	destructive := `{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`
	stdout, stderr, err = run("deny-destructive", destructive)
	if err != nil || len(stderr) != 0 {
		return errors.New("destructive command probe failed")
	}
	var decision struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if json.Unmarshal(stdout, &decision) != nil ||
		decision.HookSpecificOutput.HookEventName != "PreToolUse" ||
		decision.HookSpecificOutput.PermissionDecision != "deny" ||
		decision.HookSpecificOutput.PermissionDecisionReason == "" {
		return errors.New("destructive command probe was not denied")
	}
	stdout, stderr, err = run("output-style", `{}`)
	if err != nil || len(stdout) != 0 || len(stderr) != 0 {
		return errors.New("output-style opt-out did not produce silent success")
	}
	stdout, stderr, err = run("session-start", `{"hook_event_name":"SessionStart","source":"startup"}`)
	if err != nil || len(stderr) != 0 || !bytes.Contains(stdout, []byte("Do not claim a skill without loading it.")) || bytes.Contains(stdout, []byte("ASD-STE100-inspired")) {
		return errors.New("session-start must retain workflow guidance with style disabled")
	}
	stdout, stderr, err = run("session-start", `{}`, "MEGAPOWERS_HARNESS=codex", "MEGAPOWERS_OUTPUT_STYLE=")
	if err != nil || len(stderr) != 0 || bytes.Count(stdout, []byte("Do not claim a skill without loading it.")) != 1 || !bytes.Contains(stdout, []byte("ASD-STE100-inspired")) {
		return errors.New("installed Codex session-start must load style and one workflow reminder")
	}
	info, err = os.Lstat(runners[0])
	if err != nil || !info.ModTime().Equal(firstModTime) {
		return errors.New("warm hook call rebuilt the cached runner")
	}
	warmBytes, err := os.ReadFile(runners[0])
	if err != nil || sha256.Sum256(warmBytes) != firstHash {
		return errors.New("warm hook call changed cached runner bytes")
	}
	return nil
}

func hookCommandsUseWrapper(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var value any
	if json.Unmarshal(data, &value) != nil {
		return false
	}
	var commands []string
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "command" {
					if command, ok := child.(string); ok {
						commands = append(commands, command)
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	if len(commands) != 2 {
		return false
	}
	for _, cmd := range commands {
		if !strings.HasPrefix(cmd, `"${CLAUDE_PLUGIN_ROOT}"/hooks/run-hook.cmd `) {
			return false
		}
	}
	return true
}

func verifyCodexSkillPolicy(parent context.Context, home, installedRoot string) (bool, error) {
	if err := verifyDeclaredExplicitOnly(filepath.Join(installedRoot, "skills", "memory-hygiene", "agents", "openai.yaml")); err != nil {
		return false, err
	}
	project, err := os.MkdirTemp(home, "skills-list-project-")
	if err != nil {
		return false, errors.New("cannot create isolated skills/list project")
	}
	defer os.RemoveAll(project)
	ctx, cancel := deadline(parent, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "codex", "app-server", "--stdio")
	cmd.Dir = project
	cmd.Env = mergedEnv([]string{"CODEX_HOME=" + home})
	cmd.WaitDelay = 5 * time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	stderr := cappedBuffer{limit: 1 << 20}
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return false, err
	}
	defer func() {
		stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	encoder := json.NewEncoder(stdin)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	send := func(id int, method string, params map[string]any) (map[string]any, error) {
		if err := encoder.Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			return nil, err
		}
		for scanner.Scan() {
			var response map[string]any
			if json.Unmarshal(scanner.Bytes(), &response) != nil {
				continue
			}
			responseID, ok := response["id"].(float64)
			if !ok || int(responseID) != id {
				continue
			}
			if protocolErr, ok := response["error"]; ok {
				return nil, fmt.Errorf("app-server rejected %s: %v", method, protocolErr)
			}
			result, ok := response["result"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("app-server %s response lacks result", method)
			}
			return result, nil
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("app-server closed during %s: %s", method, strings.TrimSpace(stderr.String()))
	}
	if _, err := send(1, "initialize", map[string]any{
		"clientInfo":   map[string]any{"name": "megapowers-install-smoke", "title": "Megapowers install smoke", "version": "1"},
		"capabilities": map[string]any{"experimentalApi": true},
	}); err != nil {
		return false, err
	}
	if err := encoder.Encode(map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
		return false, err
	}
	result, err := send(2, "skills/list", map[string]any{"cwds": []string{project}, "forceReload": true})
	if err != nil {
		return false, err
	}
	data, ok := result["data"].([]any)
	if !ok {
		return false, errors.New("skills/list response lacks data")
	}
	return verifyMemorySkillEntries(data, installedRoot)
}

func verifyMemorySkillEntries(data []any, installedRoot string) (bool, error) {
	memory, err := installedSkillEntry(data, installedRoot, "memory-hygiene")
	if err != nil {
		return false, err
	}
	if _, err := installedSkillEntry(data, installedRoot, "writing-agent-instructions"); err != nil {
		return false, err
	}
	policyValue, present := memory["policy"]
	if !present {
		return false, nil
	}
	policy, ok := policyValue.(map[string]any)
	if !ok {
		return false, errors.New("memory-hygiene policy reported by skills/list is malformed")
	}
	implicit, ok := policy["allowImplicitInvocation"].(bool)
	if !ok || implicit {
		return false, errors.New("memory-hygiene policy does not explicitly deny implicit invocation")
	}
	return true, nil
}

func installedSkillEntry(data []any, installedRoot, skillName string) (map[string]any, error) {
	expectedPath, err := filepath.Abs(filepath.Join(installedRoot, "skills", skillName, "SKILL.md"))
	if err != nil {
		return nil, errors.New("cannot resolve installed plugin")
	}
	var matches []map[string]any
	for _, rawEntry := range data {
		entry, _ := rawEntry.(map[string]any)
		skills, _ := entry["skills"].([]any)
		for _, rawSkill := range skills {
			skill, _ := rawSkill.(map[string]any)
			name, _ := skill["name"].(string)
			if name != skillName && name != "megapowers:"+skillName {
				continue
			}
			path, _ := skill["path"].(string)
			absolutePath, pathErr := filepath.Abs(path)
			if pathErr != nil || absolutePath != expectedPath {
				continue
			}
			matches = append(matches, skill)
		}
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("installed %s match count is %d, want 1", skillName, len(matches))
	}
	skill := matches[0]
	if enabled, ok := skill["enabled"].(bool); !ok || !enabled {
		return nil, fmt.Errorf("installed %s is not enabled", skillName)
	}
	for _, field := range []string{"explicitlyAvailable", "availableForExplicitInvocation"} {
		value, present := skill[field]
		if !present {
			continue
		}
		available, ok := value.(bool)
		if !ok || !available {
			return nil, fmt.Errorf("%s %s is not true", skillName, field)
		}
	}
	return skill, nil
}

func verifyDeclaredExplicitOnly(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.New("installed memory-hygiene openai.yaml is missing")
	}
	const canonical = "policy:\n  allow_implicit_invocation: false\n"
	if string(data) != canonical {
		return errors.New("installed memory-hygiene openai.yaml does not declare allow_implicit_invocation: false")
	}
	return nil
}

func runCodexInstallSmoke(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: codex-install-smoke.sh")
		return 2
	}
	if !executable("codex") {
		fmt.Fprintln(stderr, "codex install smoke: codex CLI not installed")
		return 1
	}
	minimum := os.Getenv("CODEX_MIN_VERSION")
	if minimum == "" {
		minimum = "0.152.0"
	}
	versionOutput, err := output(ctx, root, nil, "codex", "--version")
	if err != nil {
		fmt.Fprintln(stderr, "codex install smoke: cannot read the codex CLI version")
		return 1
	}
	fields := strings.Fields(string(versionOutput))
	if len(fields) == 0 || !versionAtLeast(fields[len(fields)-1], minimum) {
		got := "unknown"
		if len(fields) > 0 {
			got = fields[len(fields)-1]
		}
		fmt.Fprintf(stderr, "codex install smoke: codex CLI %s is older than the verified minimum %s\n", got, minimum)
		return 1
	}
	version := fields[len(fields)-1]
	fmt.Fprintf(stdout, "codex install smoke: codex-cli %s meets the minimum %s\n", version, minimum)
	out, err := privateTempDir("megapowers-codex-smoke")
	if err != nil {
		fmt.Fprintln(stderr, "codex install smoke: cannot create the smoke scratch directory")
		return 1
	}
	defer os.RemoveAll(out)
	args = []string{"--harnesses", "codex", "--fail-on-skip", "--out", out}
	if code := runInstallSmoke(ctx, root, args, stdout, stderr); code != 0 {
		fmt.Fprintln(stderr, "codex install smoke: install-smoke runner reported a failed or skipped check")
		return 1
	}
	return 0
}
