// replay.go runs correctness-oracle PR replays without exposing the gold change.
package main

import (
	"bytes"
	"context"
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
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,255}$`)

type casesFile struct {
	SchemaVersion string       `json:"schema_version"`
	Cases         []replayCase `json:"cases"`
}

type replayCase struct {
	ID           string       `json:"id"`
	Enabled      bool         `json:"enabled"`
	RepositoryID string       `json:"repository_id"`
	Repository   string       `json:"repository"`
	Base         string       `json:"base"`
	Head         string       `json:"head"`
	Task         string       `json:"task"`
	Oracle       oracleConfig `json:"oracle"`
}

type oracleConfig struct {
	Files   []string `json:"files"`
	Command []string `json:"command"`
}

type runOptions struct {
	Harness    string
	Model      string
	Effort     string
	PluginRepo string
	Out        string
	TempRoot   string
	Timestamp  time.Time
	Selftest   bool
	Broker     string
	BrokerHash string
}

type actorRequest struct {
	CaseID     string
	Task       string
	Harness    string
	Model      string
	Effort     string
	Project    string
	PluginRepo string
}

type actorResult struct {
	Response   string
	Trace      []byte
	Inventory  []string
	CLIVersion string
	RC         int
	Duration   time.Duration
}

type actor interface {
	Run(context.Context, actorRequest) (actorResult, error)
}

type harnessIdentity struct {
	Name       string `json:"name"`
	CLIVersion string `json:"cli_version"`
	Model      string `json:"model"`
	Effort     string `json:"effort"`
}

type sourceIdentity struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
}

type environment struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Sandbox string `json:"sandbox"`
	Locale  string `json:"locale"`
}

type resultRow struct {
	SchemaVersion string             `json:"schema_version"`
	Study         string             `json:"study"`
	EvidenceClass string             `json:"evidence_class"`
	CaseID        string             `json:"case_id"`
	RunID         string             `json:"run_id"`
	BlockID       string             `json:"block_id"`
	Arm           string             `json:"arm"`
	Harness       harnessIdentity    `json:"harness"`
	Source        sourceIdentity     `json:"source"`
	PromptHash    string             `json:"prompt_hash"`
	FixtureHash   string             `json:"fixture_hash"`
	PluginHash    string             `json:"plugin_hash"`
	Status        string             `json:"status"`
	RC            int                `json:"rc"`
	DurationMS    int64              `json:"duration_ms"`
	Verdict       string             `json:"verdict"`
	Metrics       map[string]float64 `json:"metrics"`
	Artifacts     map[string]string  `json:"artifacts"`
	Environment   environment        `json:"environment"`
	Timestamp     string             `json:"timestamp"`
}

type caseManifest struct {
	CaseID              string   `json:"case_id"`
	Repository          string   `json:"repository_id"`
	Base                string   `json:"base"`
	Head                string   `json:"head"`
	PromptHash          string   `json:"prompt_hash"`
	FixtureHash         string   `json:"fixture_hash"`
	OracleHash          string   `json:"oracle_hash"`
	PluginHash          string   `json:"plugin_hash"`
	PluginInventory     []string `json:"plugin_inventory"`
	PluginInventoryHash string   `json:"plugin_inventory_hash"`
	Evidence            string   `json:"evidence"`
}

type publishManifest struct {
	SchemaVersion string         `json:"schema_version"`
	Study         string         `json:"study"`
	Evidence      string         `json:"evidence"`
	Harness       string         `json:"harness"`
	Model         string         `json:"model"`
	Effort        string         `json:"effort"`
	BrokerHash    string         `json:"broker_hash"`
	Cases         []caseManifest `json:"cases"`
}

func main() {
	var selftest, validate, run, credentialed bool
	var casesPath, harness, model, effort, out, repo, broker, brokerPin string
	flag.BoolVar(&selftest, "selftest", false, "run local synthetic contracts")
	flag.BoolVar(&validate, "validate-config", false, "validate replay metadata")
	flag.BoolVar(&run, "run", false, "run credentialed replays")
	flag.BoolVar(&credentialed, "credentialed", false, "acknowledge use of real harness credentials")
	flag.StringVar(&casesPath, "cases", "", "replay case catalog")
	flag.StringVar(&harness, "harness", "", "claude or codex")
	flag.StringVar(&model, "model", "", "exact model identity")
	flag.StringVar(&effort, "effort", "high", "exact effort identity")
	flag.StringVar(&out, "out", "", "result directory")
	flag.StringVar(&repo, "repo", "", "megapowers checkout")
	flag.StringVar(&broker, "sandbox-broker", "", "absolute path to a trusted OS-isolation broker")
	flag.StringVar(&brokerPin, "broker-sha256", "", "pinned sha256 of the trusted broker")
	flag.Parse()

	modes := 0
	for _, enabled := range []bool{selftest, validate, run} {
		if enabled {
			modes++
		}
	}
	if modes != 1 {
		fatal(errors.New("choose exactly one of --selftest, --validate-config, or --run"))
	}
	if run && credentialed {
		fatal(errors.New("credentialed PR replay is disabled: the sandbox broker schema-2 upgrade is required before credentialed runs"))
	}
	if selftest {
		if err := runSelftest(); err != nil {
			fatal(err)
		}
		return
	}
	pluginRepo, err := locateRoot(repo)
	if err != nil {
		fatal(err)
	}
	if casesPath == "" {
		casesPath = filepath.Join(pluginRepo, "evals", "studies", "pr-replay", "cases.json")
	}
	cases, err := loadCases(casesPath)
	if err != nil {
		fatal(err)
	}
	if validate {
		fmt.Printf("pr-replay config: %d cases valid\n", len(cases.Cases))
		return
	}
	if !credentialed {
		fatal(errors.New("--run requires --credentialed; selftests never substitute for real actors"))
	}
	if harness != "claude" && harness != "codex" {
		fatal(errors.New("--harness must be claude or codex"))
	}
	if model == "" || out == "" {
		fatal(errors.New("--run requires --model and --out"))
	}
	brokerHash, err := validateBroker(broker, brokerPin, pluginRepo, out)
	if err != nil {
		fatal(err)
	}
	if countEnabled(cases) == 0 {
		fatal(errors.New("no enabled replay cases"))
	}
	opts := runOptions{Harness: harness, Model: model, Effort: effort, PluginRepo: pluginRepo, Out: out, Timestamp: time.Now().UTC(), Broker: broker, BrokerHash: brokerHash}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if _, _, err := executeReplays(ctx, cases, opts, brokerActor{Path: broker, ExpectedHash: brokerHash}); err != nil {
		fatal(err)
	}
	fmt.Printf("report-only replay rows written to %s\n", filepath.Join(out, "publish"))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "pr-replay:", err)
	os.Exit(1)
}

func locateRoot(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for current := wd; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, "plugins", "megapowers")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("could not locate megapowers checkout")
		}
	}
}

func loadCases(path string) (casesFile, error) {
	var cases casesFile
	f, err := os.Open(path)
	if err != nil {
		return cases, err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cases); err != nil {
		return cases, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return cases, errors.New("trailing JSON data")
	}
	return cases, validateCases(cases)
}

func validateCases(cases casesFile) error {
	if cases.SchemaVersion != "1" {
		return errors.New("schema_version must be 1")
	}
	seen := map[string]bool{}
	for _, c := range cases.Cases {
		if !identifierPattern.MatchString(c.ID) || seen[c.ID] {
			return fmt.Errorf("case id %q is empty or duplicated", c.ID)
		}
		seen[c.ID] = true
		if !identifierPattern.MatchString(c.RepositoryID) || c.Repository == "" || c.Task == "" {
			return fmt.Errorf("case %s is missing repository or task metadata", c.ID)
		}
		if !commitPattern.MatchString(c.Base) || !commitPattern.MatchString(c.Head) || c.Base == c.Head {
			return fmt.Errorf("case %s base/head must be distinct full lowercase commit IDs", c.ID)
		}
		if len(c.Oracle.Files) == 0 || len(c.Oracle.Command) == 0 {
			return fmt.Errorf("case %s has no correctness oracle", c.ID)
		}
		for _, path := range c.Oracle.Files {
			if err := safeRelative(path); err != nil {
				return fmt.Errorf("case %s oracle: %w", c.ID, err)
			}
		}
	}
	return nil
}

func countEnabled(cases casesFile) int {
	count := 0
	for _, c := range cases.Cases {
		if c.Enabled {
			count++
		}
	}
	return count
}

func safeRelative(name string) error {
	clean := filepath.Clean(name)
	if name == "" || filepath.IsAbs(name) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe path %q", name)
	}
	return nil
}

func validateBroker(path, pin, repo, out string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", errors.New("--run requires an absolute --sandbox-broker path")
	}
	path = filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve sandbox broker: %w", err)
	}
	resolved = filepath.Clean(resolved)
	if resolved != path {
		return "", errors.New("sandbox broker path must be canonical and contain no symlinks")
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", fmt.Errorf("sandbox broker: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", errors.New("sandbox broker must be a regular executable")
	}
	for _, visible := range []string{repo, out} {
		if visible == "" {
			continue
		}
		physical, err := canonicalProspectivePath(visible)
		if err != nil {
			return "", fmt.Errorf("resolve actor-visible path: %w", err)
		}
		if pathsOverlap(resolved, physical) {
			return "", errors.New("sandbox broker must be outside actor-visible and output filesystems")
		}
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		return "", err
	}
	actual := hashBytes(content)
	if pin == "" || pin != actual {
		return "", fmt.Errorf("--broker-sha256 must pin the trusted broker as %s", actual)
	}
	return actual, nil
}

func canonicalProspectivePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(absolute)
	missing := make([]string, 0)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for _, part := range missing {
				resolved = filepath.Join(resolved, part)
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append([]string{filepath.Base(current)}, missing...)
		current = parent
	}
}

func pathsOverlap(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	return a == b || strings.HasPrefix(a, b+string(filepath.Separator)) || strings.HasPrefix(b, a+string(filepath.Separator))
}

func executeReplays(ctx context.Context, cases casesFile, opts runOptions, subject actor) ([]resultRow, publishManifest, error) {
	parent := opts.TempRoot
	if parent == "" {
		parent = os.Getenv("TMPDIR")
	}
	if parent == "" {
		parent = os.TempDir()
	}
	oldUmask := syscall.Umask(0o077)
	root, err := os.MkdirTemp(parent, "megapowers-pr-replay-*")
	syscall.Umask(oldUmask)
	if err != nil {
		return nil, publishManifest{}, err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		os.RemoveAll(root)
		return nil, publishManifest{}, err
	}
	defer os.RemoveAll(root)

	pluginHash, err := hashTree(filepath.Join(opts.PluginRepo, "plugins", "megapowers"))
	if err != nil {
		return nil, publishManifest{}, err
	}
	cliVersion := "selftest"
	brokerHash := opts.BrokerHash
	if opts.Selftest {
		brokerHash = hashBytes([]byte("in-process-selftest-fake"))
	}
	manifest := publishManifest{SchemaVersion: "1", Study: "pr-replay", Evidence: evidenceLabel(opts.Selftest), Harness: opts.Harness, Model: opts.Model, Effort: opts.Effort, BrokerHash: brokerHash}
	var rows []resultRow
	for _, c := range cases.Cases {
		if !c.Enabled {
			continue
		}
		row, item, err := executeCase(ctx, root, c, opts, subject, pluginHash, cliVersion)
		if row.SchemaVersion != "" {
			rows = append(rows, row)
		}
		if item.CaseID != "" {
			manifest.Cases = append(manifest.Cases, item)
		}
		if err != nil {
			_ = writePublish(opts.Out, rows, manifest)
			return rows, manifest, err
		}
	}
	if len(rows) == 0 {
		return nil, manifest, errors.New("no enabled replay cases")
	}
	if err := writePublish(opts.Out, rows, manifest); err != nil {
		return rows, manifest, err
	}
	return rows, manifest, nil
}

func executeCase(ctx context.Context, tempRoot string, c replayCase, opts runOptions, subject actor, pluginHash, cliVersion string) (resultRow, caseManifest, error) {
	caseRoot := filepath.Join(tempRoot, c.ID)
	source, err := prepareSource(ctx, caseRoot, c.Repository)
	if err != nil {
		return resultRow{}, caseManifest{}, fmt.Errorf("%s source: %w", c.ID, err)
	}
	if err := validateCommit(source, c.Base); err != nil {
		return resultRow{}, caseManifest{}, fmt.Errorf("%s base: %w", c.ID, err)
	}
	if err := validateCommit(source, c.Head); err != nil {
		return resultRow{}, caseManifest{}, fmt.Errorf("%s head: %w", c.ID, err)
	}
	for _, path := range c.Oracle.Files {
		if !gitObjectExists(source, c.Head+":"+path) {
			return resultRow{}, caseManifest{}, fmt.Errorf("%s missing oracle file %s at head", c.ID, path)
		}
	}
	baseline := filepath.Join(caseRoot, "oracle-baseline")
	if err := materializeTree(source, c.Base, baseline); err != nil {
		return resultRow{}, caseManifest{}, err
	}
	if err := overlayFiles(source, c.Head, c.Oracle.Files, baseline); err != nil {
		return resultRow{}, caseManifest{}, err
	}
	baseRC, _, err := runOracle(ctx, baseline, c.Oracle.Command)
	if err != nil {
		return resultRow{}, caseManifest{}, err
	}
	if baseRC == 0 {
		return resultRow{}, caseManifest{}, fmt.Errorf("%s correctness oracle is already green at base", c.ID)
	}

	actorRepo := filepath.Join(caseRoot, "actor")
	if err := materializeTree(source, c.Base, actorRepo); err != nil {
		return resultRow{}, caseManifest{}, err
	}
	if err := initializeActorRepository(actorRepo); err != nil {
		return resultRow{}, caseManifest{}, err
	}
	fixtureHash, err := hashTreeExcludingGit(actorRepo)
	if err != nil {
		return resultRow{}, caseManifest{}, err
	}
	request := actorRequest{CaseID: c.ID, Task: c.Task, Harness: opts.Harness, Model: opts.Model, Effort: opts.Effort, Project: actorRepo, PluginRepo: filepath.Join(opts.PluginRepo, "plugins", "megapowers")}
	result, actorErr := subject.Run(ctx, request)
	if result.CLIVersion != "" {
		cliVersion = result.CLIVersion
	}
	row := baseRow(c, opts, cliVersion, fixtureHash, pluginHash)
	row.DurationMS = max(result.Duration.Milliseconds(), 0)
	row.RC = result.RC
	row.Artifacts = map[string]string{"response": hashBytes([]byte(result.Response)), "trace": hashBytes(result.Trace)}
	inventory := cleanInventory(result.Inventory)
	item := caseManifest{CaseID: c.ID, Repository: portableIdentifier(c.RepositoryID), Base: c.Base, Head: c.Head, PromptHash: hashBytes([]byte(c.Task)), FixtureHash: fixtureHash, OracleHash: hashOracle(c.Oracle), PluginHash: pluginHash, PluginInventory: inventory, PluginInventoryHash: hashInventory(inventory), Evidence: evidenceLabel(opts.Selftest)}
	if actorErr != nil || result.RC != 0 {
		row.Status = "harness_error"
		row.Verdict = "harness_error"
		row.Metrics = map[string]float64{"task_success": 0, "report_only": 1}
		if actorErr != nil {
			return row, item, fmt.Errorf("%s actor error: %w", c.ID, actorErr)
		}
		return row, item, fmt.Errorf("%s actor exited %d", c.ID, result.RC)
	}

	actorPaths, patch, err := actorChanges(actorRepo)
	if err != nil {
		return row, item, err
	}
	goldPaths, err := gitLines(source, "diff", "--name-only", c.Base, c.Head)
	if err != nil {
		return row, item, err
	}
	overlap := intersectionCount(actorPaths, goldPaths)
	if err := overlayFiles(source, c.Head, c.Oracle.Files, actorRepo); err != nil {
		return row, item, err
	}
	oracleRC, oracleOutput, err := runOracle(ctx, actorRepo, c.Oracle.Command)
	if err != nil {
		return row, item, err
	}
	ratio := 0.0
	if len(actorPaths) > 0 {
		ratio = float64(overlap) / float64(len(actorPaths))
	}
	pass := oracleRC == 0
	row.Status = "completed"
	row.RC = oracleRC
	row.Verdict = map[bool]string{true: "pass", false: "fail"}[pass]
	row.Metrics = map[string]float64{
		"task_success":            boolMetric(pass),
		"oracle_pass":             boolMetric(pass),
		"changed_file_count":      float64(len(actorPaths)),
		"gold_file_overlap_count": float64(overlap),
		"gold_file_overlap_ratio": ratio,
		"report_only":             1,
	}
	row.Artifacts["actor_patch"] = hashBytes(patch)
	row.Artifacts["oracle_output"] = hashBytes(oracleOutput)
	return row, item, nil
}

func prepareSource(ctx context.Context, caseRoot, repository string) (string, error) {
	if info, err := os.Stat(repository); err == nil && info.IsDir() {
		return filepath.Abs(repository)
	}
	source := filepath.Join(caseRoot, "source-mirror")
	if err := os.MkdirAll(caseRoot, 0o700); err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, "git", "clone", "--mirror", "--", repository, source)
	isolateChild(command)
	var output boundedOutput
	output.limit = oracleCaptureLimit
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git clone failed: %w: %s", err, strings.TrimSpace(string(output.Bytes())))
	}
	return source, nil
}

func validateCommit(repo, revision string) error {
	if !commitPattern.MatchString(revision) {
		return errors.New("revision is not an immutable full commit ID")
	}
	command := exec.Command("git", "-C", repo, "cat-file", "-e", revision+"^{commit}")
	if err := command.Run(); err != nil {
		return errors.New("commit object does not exist")
	}
	return nil
}

func gitObjectExists(repo, object string) bool {
	return exec.Command("git", "-C", repo, "cat-file", "-e", object).Run() == nil
}

func materializeTree(repo, revision, destination string) error {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	paths, err := gitNull(repo, "ls-tree", "-r", "-z", "--name-only", revision)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := safeRelative(path); err != nil {
			return err
		}
		content, err := gitBytes(repo, "show", revision+":"+path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func overlayFiles(repo, revision string, paths []string, destination string) error {
	for _, path := range paths {
		content, err := gitBytes(repo, "show", revision+":"+path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func initializeActorRepository(repo string) error {
	commands := [][]string{{"git", "init", "-q"}, {"git", "config", "user.name", "Megapowers Replay"}, {"git", "config", "user.email", "replay@example.invalid"}, {"git", "config", "commit.gpgsign", "false"}, {"git", "add", "."}, {"git", "commit", "-q", "-m", "baseline"}}
	for _, argv := range commands {
		command := exec.Command(argv[0], argv[1:]...)
		command.Dir = repo
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("actor repository setup failed: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func actorChanges(repo string) ([]string, []byte, error) {
	status, err := gitBytes(repo, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	for _, record := range bytes.Split(status, []byte{0}) {
		if len(record) < 4 {
			continue
		}
		path := string(record[3:])
		if arrow := strings.LastIndex(path, " -> "); arrow >= 0 {
			path = path[arrow+4:]
		}
		seen[filepath.ToSlash(path)] = true
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	tracked, err := gitBytes(repo, "diff", "--binary", "HEAD")
	if err != nil {
		return nil, nil, err
	}
	var patch bytes.Buffer
	patch.Write(tracked)
	for _, path := range paths {
		if gitObjectExists(repo, "HEAD:"+path) {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
		if readErr == nil {
			fmt.Fprintf(&patch, "untracked:%s\n", path)
			patch.Write(content)
		}
	}
	return paths, patch.Bytes(), nil
}

const (
	oracleCaptureLimit     = 10 << 20
	oracleTruncationNotice = "\n[megapowers: oracle output truncated at the 10485760-byte capture limit]\n"
	subprocessWaitDelay    = 5 * time.Second
)

type boundedOutput struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *boundedOutput) Write(content []byte) (int, error) {
	original := len(content)
	if room := b.limit - b.buffer.Len(); room > 0 {
		if original > room {
			content = content[:room]
			b.truncated = true
		}
		_, _ = b.buffer.Write(content)
	} else if original > 0 {
		b.truncated = true
	}
	return original, nil
}

func (b *boundedOutput) Bytes() []byte {
	if !b.truncated {
		return b.buffer.Bytes()
	}
	return append(b.buffer.Bytes(), oracleTruncationNotice...)
}

func isolateChild(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	command.WaitDelay = subprocessWaitDelay
}

func runOracle(ctx context.Context, dir string, argv []string) (int, []byte, error) {
	if len(argv) == 0 {
		return 0, nil, errors.New("oracle command is empty")
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	isolateChild(command)
	var output boundedOutput
	output.limit = oracleCaptureLimit
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	if err == nil {
		return 0, output.Bytes(), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), output.Bytes(), nil
	}
	return 0, output.Bytes(), fmt.Errorf("could not execute oracle: %w", err)
}

func intersectionCount(a, b []string) int {
	set := map[string]bool{}
	for _, value := range b {
		set[value] = true
	}
	count := 0
	for _, value := range a {
		if set[value] {
			count++
		}
	}
	return count
}

func baseRow(c replayCase, opts runOptions, cliVersion, fixtureHash, pluginHash string) resultRow {
	timestamp := opts.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	return resultRow{
		SchemaVersion: "1", Study: "pr-replay", EvidenceClass: "regression", CaseID: c.ID,
		RunID: fmt.Sprintf("%s-%s", c.ID, timestamp.Format("20060102T150405Z")), BlockID: c.ID, Arm: "regression",
		Harness:    harnessIdentity{Name: portableIdentifier(opts.Harness), CLIVersion: portableIdentifier(cliVersion), Model: portableIdentifier(opts.Model), Effort: portableIdentifier(opts.Effort)},
		Source:     sourceIdentity{Repository: portableIdentifier(c.RepositoryID), Revision: c.Head},
		PromptHash: hashBytes([]byte(c.Task)), FixtureHash: fixtureHash, PluginHash: pluginHash,
		Environment: environment{OS: runtime.GOOS, Arch: runtime.GOARCH, Sandbox: "workspace-write", Locale: portableIdentifier(locale())},
		Timestamp:   timestamp.Format(time.RFC3339), Metrics: map[string]float64{}, Artifacts: map[string]string{},
	}
}

func evidenceLabel(selftest bool) string {
	if selftest {
		return "selftest-only-not-behavioral-evidence"
	}
	return "credentialed-report-only"
}

func hashOracle(oracle oracleConfig) string {
	content, _ := json.Marshal(oracle)
	return hashBytes(content)
}

func hashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hashTree(root string) (string, error)             { return hashTreeWithFilter(root, false) }
func hashTreeExcludingGit(root string) (string, error) { return hashTreeWithFilter(root, true) }

func hashTreeWithFilter(root string, excludeGit bool) (string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if excludeGit && entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%d:%s:%d:", len(path), path, len(content))
		h.Write(content)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func gitBytes(repo string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func gitLines(repo string, args ...string) ([]string, error) {
	output, err := gitBytes(repo, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			lines = append(lines, filepath.ToSlash(line))
		}
	}
	return lines, nil
}

func gitNull(repo string, args ...string) ([]string, error) {
	output, err := gitBytes(repo, args...)
	if err != nil {
		return nil, err
	}
	var values []string
	for _, value := range bytes.Split(output, []byte{0}) {
		if len(value) > 0 {
			values = append(values, string(value))
		}
	}
	return values, nil
}

func writePublish(out string, rows []resultRow, manifest publishManifest) error {
	if out == "" {
		return nil
	}
	publish := filepath.Join(out, "publish")
	if err := os.MkdirAll(publish, 0o755); err != nil {
		return err
	}
	var rowsJSON bytes.Buffer
	encoder := json.NewEncoder(&rowsJSON)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return err
		}
	}
	if err := atomicWrite(filepath.Join(publish, "results.jsonl"), rowsJSON.Bytes()); err != nil {
		return err
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(publish, "manifest.json"), append(manifestJSON, '\n'))
}

func atomicWrite(path string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

type brokerActor struct {
	Path         string
	ExpectedHash string
}

func stageVerifiedBroker(sourcePath, expectedHash string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "megapowers-verified-broker-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_ = os.Chmod(dir, 0o700)
		_ = os.RemoveAll(dir)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		cleanup()
		return "", nil, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	destination := filepath.Join(dir, "broker")
	target, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o500)
	if err != nil {
		source.Close()
		cleanup()
		return "", nil, err
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(target, hasher), source)
	targetCloseErr := target.Close()
	sourceCloseErr := source.Close()
	if copyErr != nil || targetCloseErr != nil || sourceCloseErr != nil {
		cleanup()
		return "", nil, errors.New("copy trusted sandbox broker")
	}
	actual := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if actual != expectedHash {
		cleanup()
		return "", nil, errors.New("sandbox broker changed before its verified execution copy was created")
	}
	if err := os.Chmod(destination, 0o500); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		cleanup()
		return "", nil, err
	}
	return destination, cleanup, nil
}

func brokerCommand(ctx context.Context, stagedPath string) *exec.Cmd {
	command := exec.CommandContext(ctx, stagedPath)
	command.Dir = filepath.Dir(stagedPath)
	isolateChild(command)
	return command
}

type brokerRequest struct {
	SchemaVersion  string   `json:"schema_version"`
	Harness        string   `json:"harness"`
	Model          string   `json:"model"`
	Effort         string   `json:"effort"`
	Arm            string   `json:"arm"`
	Task           string   `json:"task"`
	Project        string   `json:"project"`
	PluginRepo     string   `json:"plugin_repo"`
	TaskReadRoots  []string `json:"task_read_roots"`
	TaskWriteRoots []string `json:"task_write_roots"`
}

type isolationAttestation struct {
	Boundary                    string   `json:"boundary"`
	CredentialsReadableByActor  *bool    `json:"credentials_readable_by_actor"`
	SiblingStateReadableByActor *bool    `json:"sibling_state_readable_by_actor"`
	TaskReadRoots               []string `json:"task_read_roots"`
	TaskWriteRoots              []string `json:"task_write_roots"`
}

type brokerResponse struct {
	SchemaVersion   string               `json:"schema_version"`
	CLIVersion      string               `json:"cli_version"`
	Response        string               `json:"response"`
	Trace           string               `json:"trace"`
	PluginInventory []string             `json:"plugin_inventory"`
	RC              int                  `json:"rc"`
	DurationMS      int64                `json:"duration_ms"`
	Isolation       isolationAttestation `json:"isolation"`
}

func (b brokerActor) Run(ctx context.Context, request actorRequest) (actorResult, error) {
	stagedBroker, cleanup, err := stageVerifiedBroker(b.Path, b.ExpectedHash)
	if err != nil {
		return actorResult{RC: 125}, err
	}
	defer cleanup()
	roots := []string{request.Project, request.PluginRepo}
	payload := brokerRequest{SchemaVersion: "1", Harness: request.Harness, Model: request.Model, Effort: request.Effort, Arm: "treatment", Task: request.Task, Project: request.Project, PluginRepo: request.PluginRepo, TaskReadRoots: roots, TaskWriteRoots: []string{request.Project}}
	input, err := json.Marshal(payload)
	if err != nil {
		return actorResult{RC: 125}, err
	}
	command := brokerCommand(ctx, stagedBroker)
	command.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return actorResult{RC: 125}, fmt.Errorf("sandbox broker failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var response brokerResponse
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return actorResult{RC: 125}, fmt.Errorf("sandbox broker response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return actorResult{RC: 125}, errors.New("sandbox broker response has trailing data")
	}
	if err := validateIsolation(response, roots); err != nil {
		return actorResult{RC: 125}, err
	}
	return actorResult{Response: response.Response, Trace: []byte(response.Trace), Inventory: response.PluginInventory, CLIVersion: response.CLIVersion, RC: response.RC, Duration: time.Duration(response.DurationMS) * time.Millisecond}, nil
}

func validateIsolation(response brokerResponse, expectedRoots []string) error {
	if response.SchemaVersion != "1" || response.CLIVersion == "" {
		return errors.New("sandbox broker response is incomplete")
	}
	allowedBoundaries := map[string]bool{"bwrap": true, "container": true, "seatbelt": true, "sandbox-exec": true, "appcontainer": true}
	if !allowedBoundaries[response.Isolation.Boundary] || response.Isolation.CredentialsReadableByActor == nil || *response.Isolation.CredentialsReadableByActor || response.Isolation.SiblingStateReadableByActor == nil || *response.Isolation.SiblingStateReadableByActor {
		return errors.New("sandbox broker did not attest a credential-safe sibling-isolating OS boundary")
	}
	if !samePaths(response.Isolation.TaskReadRoots, expectedRoots) {
		return errors.New("sandbox broker attested unexpected actor read roots")
	}
	if !samePaths(response.Isolation.TaskWriteRoots, expectedRoots[:1]) {
		return errors.New("sandbox broker must limit actor writes to the exported base project")
	}
	if len(response.PluginInventory) != 1 || response.PluginInventory[0] != "megapowers" {
		return errors.New("sandbox broker inventory is not exactly megapowers")
	}
	return nil
}

func samePaths(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	left := append([]string(nil), a...)
	right := append([]string(nil), b...)
	for i := range left {
		left[i] = filepath.Clean(left[i])
	}
	for i := range right {
		right[i] = filepath.Clean(right[i])
	}
	sort.Strings(left)
	sort.Strings(right)
	return strings.Join(left, "\x00") == strings.Join(right, "\x00")
}

func cleanInventory(input []string) []string {
	seen := map[string]bool{}
	var output []string
	for _, name := range input {
		if name == "megapowers" && !seen[name] {
			seen[name] = true
			output = append(output, name)
		}
	}
	sort.Strings(output)
	return output
}

func hashInventory(names []string) string {
	canonical := append([]string(nil), names...)
	sort.Strings(canonical)
	content, _ := json.Marshal(canonical)
	return hashBytes(append(content, '\n'))
}

func portableIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var output strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._:/-", r) {
			output.WriteRune(r)
		} else {
			output.WriteByte('-')
		}
		if output.Len() == 256 {
			break
		}
	}
	return strings.Trim(output.String(), "-")
}

func locale() string {
	for _, key := range []string{"LC_ALL", "LANG"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return "C.UTF-8"
}

func boolMetric(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func boolPointer(value bool) *bool { return &value }

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

type fakeActor struct {
	Head       string
	Correct    bool
	GoldHidden bool
	Fail       bool
}

func (f *fakeActor) Run(_ context.Context, request actorRequest) (actorResult, error) {
	if gitObjectExists(request.Project, f.Head+"^{commit}") {
		return actorResult{RC: 125}, errors.New("gold head object is visible to actor")
	}
	f.GoldHidden = true
	if f.Fail {
		return actorResult{RC: 41}, errors.New("synthetic actor failure")
	}
	content := "package calculator\n\nfunc Add(a, b int) int { return a + b }\nfunc Multiply(a, b int) int { return a + b }\n"
	if f.Correct {
		content = "package calculator\n\nfunc Add(a, b int) int { return a + b }\nfunc Multiply(a, b int) int { return a * b }\n"
	}
	if err := os.WriteFile(filepath.Join(request.Project, "calculator.go"), []byte(content), 0o600); err != nil {
		return actorResult{RC: 125}, err
	}
	return actorResult{Response: "implemented", Trace: []byte("synthetic trace"), Inventory: []string{"megapowers"}, CLIVersion: "selftest", RC: 0, Duration: time.Millisecond}, nil
}

func runSelftest() error {
	root, err := locateRoot("")
	if err != nil {
		return err
	}
	parent, err := os.MkdirTemp("", "pr-replay-selftest-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(parent)
	source, base, head, greenHead, err := makeSyntheticRepository(parent)
	if err != nil {
		return err
	}
	valid := replayCase{ID: "multiply", Enabled: true, RepositoryID: "synthetic/calculator", Repository: source, Base: base, Head: head, Task: "Add Multiply(a, b int) int.", Oracle: oracleConfig{Files: []string{"hidden_test.go"}, Command: []string{"go", "test", "./..."}}}
	_, brokerErr := validateBroker("", "", root, filepath.Join(parent, "output"))
	brokerRequired := brokerErr != nil
	printCheck("live runs require isolated broker", brokerRequired)
	if !brokerRequired {
		return errors.New("live replay accepted no isolation broker")
	}
	brokerTarget := filepath.Join(parent, "trusted-broker")
	brokerLink := filepath.Join(parent, "trusted-broker-link")
	brokerContent := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(brokerTarget, brokerContent, 0o700); err != nil {
		return err
	}
	if err := os.Symlink(brokerTarget, brokerLink); err != nil {
		return err
	}
	_, brokerLinkErr := validateBroker(brokerLink, hashBytes(brokerContent), root, filepath.Join(parent, "output"))
	brokerLinkRejected := brokerLinkErr != nil
	printCheck("symlinked broker path rejected", brokerLinkRejected)
	if !brokerLinkRejected {
		return errors.New("symlinked sandbox broker was accepted")
	}
	stagedBroker, cleanupBroker, stageErr := stageVerifiedBroker(brokerTarget, hashBytes(brokerContent))
	if stageErr != nil {
		return stageErr
	}
	defer cleanupBroker()
	if err := os.WriteFile(brokerTarget, []byte("#!/bin/sh\nexit 99\n"), 0o700); err != nil {
		return err
	}
	stagedContent, err := os.ReadFile(stagedBroker)
	if err != nil {
		return err
	}
	stagedCopyBound := stagedBroker != brokerTarget && hashBytes(stagedContent) == hashBytes(brokerContent)
	printCheck("broker executes from a verified private copy", stagedCopyBound)
	if !stagedCopyBound {
		return errors.New("staged broker did not retain approved bytes")
	}
	privateBrokerCWD := brokerCommand(context.Background(), stagedBroker).Dir == filepath.Dir(stagedBroker)
	printCheck("broker working directory is the verified private directory", privateBrokerCWD)
	if !privateBrokerCWD {
		return errors.New("sandbox broker retained a mutable original working directory")
	}
	attestationRoots := []string{filepath.Join(parent, "actor"), filepath.Join(root, "plugins", "megapowers")}
	validAttestation := brokerResponse{SchemaVersion: "1", CLIVersion: "selftest", PluginInventory: []string{"megapowers"}, Isolation: isolationAttestation{Boundary: "bwrap", CredentialsReadableByActor: boolPointer(false), SiblingStateReadableByActor: boolPointer(false), TaskReadRoots: attestationRoots, TaskWriteRoots: attestationRoots[:1]}}
	credentialLeak := validAttestation
	credentialLeak.Isolation.CredentialsReadableByActor = boolPointer(true)
	goldLeak := validAttestation
	goldLeak.Isolation.SiblingStateReadableByActor = boolPointer(true)
	missingAttestation := validAttestation
	missingAttestation.Isolation.SiblingStateReadableByActor = nil
	leaksRejected := validateIsolation(credentialLeak, attestationRoots) != nil && validateIsolation(goldLeak, attestationRoots) != nil && validateIsolation(missingAttestation, attestationRoots) != nil
	printCheck("isolation attestation rejects gold and credential access", leaksRejected)
	if !leaksRejected {
		return errors.New("unsafe replay isolation attestation was accepted")
	}

	mutable := valid
	mutable.Base = "main"
	mutableRejected := validateCases(casesFile{SchemaVersion: "1", Cases: []replayCase{mutable}}) != nil
	printCheck("mutable refs rejected", mutableRejected)
	if !mutableRejected {
		return errors.New("mutable ref was accepted")
	}
	missing := valid
	missing.Oracle = oracleConfig{}
	missingRejected := validateCases(casesFile{SchemaVersion: "1", Cases: []replayCase{missing}}) != nil
	printCheck("missing correctness oracle rejected", missingRejected)
	if !missingRejected {
		return errors.New("missing oracle was accepted")
	}
	alreadyGreen := valid
	alreadyGreen.ID = "already-green"
	alreadyGreen.Head = greenHead
	alreadyGreen.Oracle.Files = []string{"green_test.go"}
	outGreen := filepath.Join(parent, "green-output")
	greenOpts := runOptions{Harness: "codex", Model: "fake-selftest", Effort: "test", PluginRepo: root, Out: outGreen, TempRoot: parent, Timestamp: time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC), Selftest: true}
	_, _, greenErr := executeReplays(context.Background(), casesFile{SchemaVersion: "1", Cases: []replayCase{alreadyGreen}}, greenOpts, &fakeActor{Head: greenHead, Correct: true})
	greenRejected := greenErr != nil && strings.Contains(greenErr.Error(), "already green")
	printCheck("already-green correctness oracle rejected", greenRejected)
	if !greenRejected {
		return errors.New("already-green oracle was accepted")
	}

	out := filepath.Join(parent, "success-output")
	opts := greenOpts
	opts.Out = out
	correct := &fakeActor{Head: head, Correct: true}
	rows, _, err := executeReplays(context.Background(), casesFile{SchemaVersion: "1", Cases: []replayCase{valid}}, opts, correct)
	if err != nil {
		return err
	}
	goldHidden := correct.GoldHidden && len(rows) == 1 && rows[0].Verdict == "pass"
	printCheck("gold change unavailable to actor", goldHidden)
	if !goldHidden {
		return errors.New("gold change was visible or correct replay failed")
	}

	wrongOut := filepath.Join(parent, "wrong-output")
	wrongOpts := opts
	wrongOpts.Out = wrongOut
	wrong := &fakeActor{Head: head, Correct: false}
	wrongRows, _, err := executeReplays(context.Background(), casesFile{SchemaVersion: "1", Cases: []replayCase{valid}}, wrongOpts, wrong)
	if err != nil {
		return err
	}
	overlapDiagnostic := len(wrongRows) == 1 && wrongRows[0].Metrics["gold_file_overlap_count"] > 0 && wrongRows[0].Verdict == "fail" && wrongRows[0].Metrics["task_success"] == 0
	printCheck("file overlap remains diagnostic", overlapDiagnostic)
	if !overlapDiagnostic {
		return errors.New("file overlap changed correctness verdict")
	}
	schemaOK := strictScore(root, filepath.Join(out, "publish", "results.jsonl")) == nil && strictScore(root, filepath.Join(wrongOut, "publish", "results.jsonl")) == nil
	printCheck("schema rows pass strict scorer", schemaOK)
	if !schemaOK {
		return errors.New("selftest rows failed strict scorer")
	}

	leftovers, _ := filepath.Glob(filepath.Join(parent, "megapowers-pr-replay-*"))
	sanitized := len(leftovers) == 0 && publishFilesOnly(out) && publishFilesOnly(wrongOut) && publishContainsNo([]string{out, wrongOut}, []string{source, "actor-final", "credentials", "synthetic trace", valid.Task})
	printCheck("publish bundle contains sanitized files only", sanitized)
	if !sanitized {
		return errors.New("publish bundle is not sanitized or temporary state leaked")
	}
	fmt.Println("pr-replay selftest: PASS")
	return nil
}

func makeSyntheticRepository(parent string) (string, string, string, string, error) {
	repo := filepath.Join(parent, "synthetic-source")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		return "", "", "", "", err
	}
	commands := [][]string{{"git", "init", "-q"}, {"git", "config", "user.name", "Replay Fixture"}, {"git", "config", "user.email", "replay@example.invalid"}, {"git", "config", "commit.gpgsign", "false"}}
	for _, argv := range commands {
		if err := runIn(repo, argv...); err != nil {
			return "", "", "", "", err
		}
	}
	baseFiles := map[string]string{"go.mod": "module example.com/replay\n\ngo 1.24\n", "calculator.go": "package calculator\n\nfunc Add(a, b int) int { return a + b }\n"}
	if err := writeFiles(repo, baseFiles); err != nil {
		return "", "", "", "", err
	}
	if err := runIn(repo, "git", "add", "."); err != nil {
		return "", "", "", "", err
	}
	if err := runIn(repo, "git", "commit", "-q", "-m", "base"); err != nil {
		return "", "", "", "", err
	}
	base := strings.TrimSpace(string(mustGit(repo, "rev-parse", "HEAD")))
	headFiles := map[string]string{
		"calculator.go":  "package calculator\n\nfunc Add(a, b int) int { return a + b }\nfunc Multiply(a, b int) int { return a * b }\n",
		"hidden_test.go": "package calculator\n\nimport \"testing\"\n\nfunc TestMultiply(t *testing.T) { if Multiply(3, 4) != 12 { t.Fatal(\"bad product\") } }\n",
	}
	if err := writeFiles(repo, headFiles); err != nil {
		return "", "", "", "", err
	}
	if err := runIn(repo, "git", "add", "."); err != nil {
		return "", "", "", "", err
	}
	if err := runIn(repo, "git", "commit", "-q", "-m", "gold"); err != nil {
		return "", "", "", "", err
	}
	head := strings.TrimSpace(string(mustGit(repo, "rev-parse", "HEAD")))
	if err := runIn(repo, "git", "checkout", "-q", base); err != nil {
		return "", "", "", "", err
	}
	greenTest := "package calculator\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { if Add(2, 3) != 5 { t.Fatal(\"bad sum\") } }\n"
	if err := os.WriteFile(filepath.Join(repo, "green_test.go"), []byte(greenTest), 0o600); err != nil {
		return "", "", "", "", err
	}
	if err := runIn(repo, "git", "add", "."); err != nil {
		return "", "", "", "", err
	}
	if err := runIn(repo, "git", "commit", "-q", "-m", "already green oracle"); err != nil {
		return "", "", "", "", err
	}
	greenHead := strings.TrimSpace(string(mustGit(repo, "rev-parse", "HEAD")))
	return repo, base, head, greenHead, nil
}

func writeFiles(root string, files map[string]string) error {
	for path, content := range files {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func runIn(dir string, argv ...string) error {
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("%s failed: %w: %s", strings.Join(argv, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func mustGit(repo string, args ...string) []byte {
	output, err := gitBytes(repo, args...)
	if err != nil {
		panic(err)
	}
	return output
}

func publishFilesOnly(out string) bool {
	var files []string
	err := filepath.WalkDir(out, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			rel, _ := filepath.Rel(out, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return err == nil && len(files) == 2 && files[0] == "publish/manifest.json" && files[1] == "publish/results.jsonl"
}

func strictScore(root, rows string) error {
	command := exec.Command("go", "run", filepath.Join(root, "evals", "score.go"), "--strict", rows)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("strict scorer failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func publishContainsNo(outputs, banned []string) bool {
	for _, out := range outputs {
		err := filepath.WalkDir(filepath.Join(out, "publish"), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, value := range banned {
				if value != "" && bytes.Contains(content, []byte(value)) {
					return fmt.Errorf("banned publish content")
				}
			}
			return nil
		})
		if err != nil {
			return false
		}
	}
	return true
}

func printCheck(description string, ok bool) {
	if ok {
		fmt.Println("ok  ", description)
	} else {
		fmt.Println("FAIL", description)
	}
}
