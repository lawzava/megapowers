// run.go executes installed-plugin treatment/control studies in disposable homes.
// Selftests use an in-process fake actor and are never emitted as behavioral evidence.
package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
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

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)

type casesFile struct {
	SchemaVersion string      `json:"schema_version"`
	Cases         []studyCase `json:"cases"`
}

type studyCase struct {
	ID                string            `json:"id"`
	Kind              string            `json:"kind"`
	Task              string            `json:"task"`
	Files             map[string]string `json:"files"`
	RequiredFacts     []string          `json:"required_facts,omitempty"`
	ForbiddenFacts    []string          `json:"forbidden_facts,omitempty"`
	SeededDefects     []string          `json:"seeded_defects,omitempty"`
	ForbiddenPatterns []string          `json:"forbidden_patterns,omitempty"`
	OracleCommand     []string          `json:"oracle_command,omitempty"`
	ProtectedFiles    []string          `json:"protected_files,omitempty"`
	RequireNoop       bool              `json:"require_noop,omitempty"`
	ExpectedOutput    string            `json:"expected_output,omitempty"`
}

type gatesFile struct {
	SchemaVersion string `json:"schema_version"`
	Prose         struct {
		MinimumFactRetention    float64 `json:"minimum_fact_retention"`
		MaximumInventedFacts    int     `json:"maximum_invented_facts"`
		RequireNoopPreservation bool    `json:"require_noop_preservation"`
	} `json:"prose"`
	CodeQuality struct {
		MinimumSeededDefectReduction int `json:"minimum_seeded_defect_reduction"`
		MaximumConventionRegressions int `json:"maximum_convention_regressions"`
	} `json:"code_quality"`
	Acceptance struct {
		MinimumPairedRuns         int  `json:"minimum_paired_runs"`
		RequireAllTreatmentPasses bool `json:"require_all_treatment_passes"`
	} `json:"acceptance"`
	TDD struct {
		RequireTestBeforeImplementation bool `json:"require_test_before_implementation"`
		RequireRedBeforeImplementation  bool `json:"require_red_before_implementation"`
		RequirePassingOracle            bool `json:"require_passing_oracle"`
	} `json:"tdd"`
	AutonomyStatus struct {
		MinimumFactRetention float64 `json:"minimum_fact_retention"`
		MaximumInventedFacts int     `json:"maximum_invented_facts"`
		ReportOnly           bool    `json:"report_only"`
	} `json:"autonomy_status"`
}

type runOptions struct {
	Harness      string
	Model        string
	Effort       string
	Repo         string
	Out          string
	TempRoot     string
	Timestamp    time.Time
	Selftest     bool
	Broker       string
	BrokerHash   string
	PairedRuns   int
	ActorTimeout time.Duration
}

type actorRequest struct {
	Case       studyCase
	Arm        string
	Harness    string
	Model      string
	Effort     string
	Home       string
	Project    string
	PluginRoot string
	Timeout    time.Duration
}

type actorEvent struct {
	Kind string `json:"kind"`
	Path string `json:"path,omitempty"`
	RC   int    `json:"rc,omitempty"`
	Step int    `json:"step"`
}

type actorResult struct {
	Response   string
	Trace      []byte
	Events     []actorEvent
	Inventory  []string
	CLIVersion string
	Sandbox    string
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

type armManifest struct {
	CaseID        string   `json:"case_id"`
	BlockID       string   `json:"block_id"`
	Arm           string   `json:"arm"`
	PromptHash    string   `json:"prompt_hash"`
	FixtureHash   string   `json:"fixture_hash"`
	PluginHash    string   `json:"plugin_hash"`
	PluginNames   []string `json:"plugin_inventory"`
	InventoryHash string   `json:"plugin_inventory_hash"`
	EvidenceOnly  string   `json:"evidence"`
}

type caseAssessment struct {
	CaseID            string  `json:"case_id"`
	PairedRuns        int     `json:"paired_runs"`
	TreatmentPasses   int     `json:"treatment_passes"`
	ControlPasses     int     `json:"control_passes"`
	TreatmentPassRate float64 `json:"treatment_pass_rate"`
	ControlPassRate   float64 `json:"control_pass_rate"`
	ObservedLift      float64 `json:"observed_lift"`
	Accepted          bool    `json:"accepted"`
}

type studyAcceptance struct {
	Accepted                  bool             `json:"accepted"`
	MinimumPairedRuns         int              `json:"minimum_paired_runs"`
	RequireAllTreatmentPasses bool             `json:"require_all_treatment_passes"`
	Cases                     []caseAssessment `json:"cases"`
	Reasons                   []string         `json:"reasons"`
}

type publishManifest struct {
	SchemaVersion          string          `json:"schema_version"`
	Study                  string          `json:"study"`
	Evidence               string          `json:"evidence"`
	Harness                string          `json:"harness"`
	Model                  string          `json:"model"`
	Effort                 string          `json:"effort"`
	BrokerHash             string          `json:"broker_hash"`
	CaseCatalogHash        string          `json:"case_catalog_hash"`
	GatesHash              string          `json:"gates_hash"`
	TreatmentPluginHash    string          `json:"treatment_plugin_hash"`
	EmptyControlPluginHash string          `json:"empty_control_plugin_hash"`
	Acceptance             studyAcceptance `json:"acceptance"`
	Arms                   []armManifest   `json:"arms"`
}

func main() {
	var selftest, validate, run, hashPlugin, credentialed bool
	var casesPath, gatesPath, harness, model, effort, out, repo, broker, brokerPin string
	var pairedRuns int
	var actorTimeout time.Duration
	flag.BoolVar(&selftest, "selftest", false, "run credential-free runner contracts")
	flag.BoolVar(&validate, "validate-config", false, "validate cases and gates")
	flag.BoolVar(&run, "run", false, "run a real installed-plugin study")
	flag.BoolVar(&hashPlugin, "hash-plugin", false, "print the verified current plugin tree hash")
	flag.BoolVar(&credentialed, "credentialed", false, "acknowledge use of real harness credentials")
	flag.StringVar(&casesPath, "cases", "", "case catalog")
	flag.StringVar(&gatesPath, "gates", "", "gate catalog")
	flag.StringVar(&harness, "harness", "", "claude or codex")
	flag.StringVar(&model, "model", "", "exact model identity")
	flag.StringVar(&effort, "effort", "high", "exact effort identity")
	flag.StringVar(&out, "out", "", "result directory")
	flag.StringVar(&repo, "repo", "", "megapowers checkout, defaults to current checkout")
	flag.StringVar(&broker, "sandbox-broker", "", "absolute path to a trusted OS-isolation broker")
	flag.StringVar(&brokerPin, "broker-sha256", "", "pinned sha256 of the trusted broker")
	flag.IntVar(&pairedRuns, "paired-runs", 0, "paired runs per case, defaults to the configured minimum")
	flag.DurationVar(&actorTimeout, "actor-timeout", 20*time.Minute, "maximum time for one actor run")
	flag.Parse()

	modes := 0
	for _, enabled := range []bool{selftest, validate, run, hashPlugin} {
		if enabled {
			modes++
		}
	}
	if modes != 1 {
		fatal(errors.New("choose exactly one of --selftest, --validate-config, --hash-plugin, or --run"))
	}
	if selftest {
		if err := runSelftest(); err != nil {
			fatal(err)
		}
		return
	}
	root, err := locateRoot(repo)
	if err != nil {
		fatal(err)
	}
	if hashPlugin {
		hash, err := verifiedPluginHash(root)
		if err != nil {
			fatal(err)
		}
		fmt.Println(hash)
		return
	}
	if casesPath == "" {
		casesPath = filepath.Join(root, "evals", "studies", "installed-ab", "cases.json")
	}
	if gatesPath == "" {
		gatesPath = filepath.Join(root, "evals", "studies", "installed-ab", "gates.json")
	}
	if run {
		if err := verifyStudyConfiguration(root, casesPath, gatesPath); err != nil {
			fatal(err)
		}
	}
	cases, gates, err := loadConfiguration(casesPath, gatesPath)
	if err != nil {
		fatal(err)
	}
	if validate {
		fmt.Printf("installed-ab config: %d cases valid\n", len(cases.Cases))
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
	if actorTimeout < 10*time.Second || actorTimeout > 2*time.Hour {
		fatal(errors.New("--actor-timeout must be in [10s,2h]"))
	}
	brokerHash, err := validateBroker(broker, brokerPin, root, out)
	if err != nil {
		fatal(err)
	}
	if pairedRuns == 0 {
		pairedRuns = gates.Acceptance.MinimumPairedRuns
	}
	if pairedRuns < 1 {
		fatal(errors.New("--paired-runs must be positive"))
	}
	opts := runOptions{Harness: harness, Model: model, Effort: effort, Repo: root, Out: out, Timestamp: time.Now().UTC(), Broker: broker, BrokerHash: brokerHash, PairedRuns: pairedRuns, ActorTimeout: actorTimeout}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if _, _, err := executeStudy(ctx, cases, gates, opts, brokerActor{Path: broker, ExpectedHash: brokerHash}); err != nil {
		fatal(err)
	}
	fmt.Printf("behavioral rows written to %s; score separately with evals/score.go --strict\n", filepath.Join(out, "publish"))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "installed-ab:", err)
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

func decodeStrict(path string, target any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%s: trailing JSON data", path)
	}
	return nil
}

func loadConfiguration(casesPath, gatesPath string) (casesFile, gatesFile, error) {
	var cases casesFile
	var gates gatesFile
	if err := decodeStrict(casesPath, &cases); err != nil {
		return cases, gates, err
	}
	if err := decodeStrict(gatesPath, &gates); err != nil {
		return cases, gates, err
	}
	if err := validateConfiguration(cases, gates); err != nil {
		return cases, gates, err
	}
	return cases, gates, nil
}

func validateConfiguration(cases casesFile, gates gatesFile) error {
	if cases.SchemaVersion != "1" || gates.SchemaVersion != "1" {
		return errors.New("schema_version must be 1")
	}
	if len(cases.Cases) == 0 {
		return errors.New("at least one installed A/B case is required")
	}
	seen := map[string]bool{}
	for _, c := range cases.Cases {
		if !identifierPattern.MatchString(c.ID) || seen[c.ID] {
			return fmt.Errorf("case id %q is empty or duplicated", c.ID)
		}
		seen[c.ID] = true
		if c.Task == "" {
			return fmt.Errorf("case %s has no task", c.ID)
		}
		switch c.Kind {
		case "prose":
			if len(c.RequiredFacts) == 0 {
				return fmt.Errorf("prose case %s has no required facts", c.ID)
			}
			if c.RequireNoop && c.ExpectedOutput == "" {
				return fmt.Errorf("prose no-op case %s has no expected output", c.ID)
			}
		case "code_quality":
			if len(c.SeededDefects) == 0 || len(c.OracleCommand) == 0 {
				return fmt.Errorf("code-quality case %s needs seeded defects and oracle", c.ID)
			}
		case "tdd":
			if len(c.OracleCommand) == 0 || len(c.ProtectedFiles) == 0 {
				return fmt.Errorf("TDD case %s needs an oracle and protected public fixtures", c.ID)
			}
		case "autonomy_status":
			if len(c.RequiredFacts) == 0 {
				return fmt.Errorf("autonomy status case %s has no required facts", c.ID)
			}
		default:
			return fmt.Errorf("case %s has unsupported kind %q", c.ID, c.Kind)
		}
		for name := range c.Files {
			if err := safeRelative(name); err != nil {
				return fmt.Errorf("case %s: %w", c.ID, err)
			}
		}
		protected := map[string]bool{}
		for _, name := range c.ProtectedFiles {
			if err := safeRelative(name); err != nil {
				return fmt.Errorf("case %s protected file: %w", c.ID, err)
			}
			if _, ok := c.Files[name]; !ok {
				return fmt.Errorf("case %s protected file %q is absent from the fixture", c.ID, name)
			}
			if protected[name] {
				return fmt.Errorf("case %s protected file %q is duplicated", c.ID, name)
			}
			protected[name] = true
		}
	}
	if gates.Prose.MinimumFactRetention != 1 || gates.Prose.MaximumInventedFacts != 0 || !gates.Prose.RequireNoopPreservation {
		return errors.New("prose gate must require complete retention and zero inventions")
	}
	if gates.CodeQuality.MinimumSeededDefectReduction < 1 || gates.CodeQuality.MaximumConventionRegressions != 0 {
		return errors.New("code-quality gate must reduce defects without convention regressions")
	}
	if gates.Acceptance.MinimumPairedRuns < 2 || !gates.Acceptance.RequireAllTreatmentPasses {
		return errors.New("study acceptance criteria are incomplete")
	}
	if !gates.TDD.RequireTestBeforeImplementation || !gates.TDD.RequireRedBeforeImplementation || !gates.TDD.RequirePassingOracle {
		return errors.New("TDD gate must require test-first, observed red, and passing oracle")
	}
	if gates.AutonomyStatus.MinimumFactRetention != 1 || gates.AutonomyStatus.MaximumInventedFacts != 0 || !gates.AutonomyStatus.ReportOnly {
		return errors.New("autonomy status must remain a strict report-only gate")
	}
	return nil
}

func configurationHashes(cases casesFile, gates gatesFile) (string, string, error) {
	caseBytes, err := json.Marshal(cases)
	if err != nil {
		return "", "", err
	}
	gateBytes, err := json.Marshal(gates)
	if err != nil {
		return "", "", err
	}
	return hashBytes(caseBytes), hashBytes(gateBytes), nil
}

func safeRelative(name string) error {
	clean := filepath.Clean(name)
	if name == "" || filepath.IsAbs(name) || clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return fmt.Errorf("unsafe fixture path %q", name)
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

func executeStudy(ctx context.Context, cases casesFile, gates gatesFile, opts runOptions, subject actor) ([]resultRow, publishManifest, error) {
	parent := opts.TempRoot
	if parent == "" {
		parent = os.Getenv("TMPDIR")
	}
	if parent == "" {
		parent = os.TempDir()
	}
	oldUmask := setUmask077()
	root, err := os.MkdirTemp(parent, "megapowers-installed-ab-*")
	restoreUmask(oldUmask)
	if err != nil {
		return nil, publishManifest{}, err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		os.RemoveAll(root)
		return nil, publishManifest{}, err
	}
	defer removePrivateTree(root)

	revision := gitOutput(opts.Repo, "rev-parse", "HEAD")
	if revision == "" {
		revision = strings.Repeat("0", 40)
	}
	pluginHash := hashBytes([]byte("in-process-selftest-plugin"))
	actorPluginRoot := filepath.Join(opts.Repo, "plugins", "megapowers")
	if !opts.Selftest {
		pluginHash, err = verifiedPluginHash(opts.Repo)
		if err != nil {
			return nil, publishManifest{}, err
		}
		actorPluginRoot = filepath.Join(root, "verified-plugin")
		if err := stagePortablePluginTree(filepath.Join(opts.Repo, "plugins", "megapowers"), actorPluginRoot, pluginHash); err != nil {
			return nil, publishManifest{}, err
		}
	}
	controlHash := emptyControlPluginHash()
	cliVersion := "selftest"

	pairedRuns := opts.PairedRuns
	if pairedRuns == 0 {
		pairedRuns = 1
	}
	brokerHash := opts.BrokerHash
	if opts.Selftest {
		brokerHash = hashBytes([]byte("in-process-selftest-fake"))
	}
	caseCatalogHash, gatesHash, err := configurationHashes(cases, gates)
	if err != nil {
		return nil, publishManifest{}, err
	}
	manifest := publishManifest{SchemaVersion: "1", Study: "installed-plugin-ab", Evidence: evidenceLabel(opts.Selftest), Harness: opts.Harness, Model: opts.Model, Effort: opts.Effort, BrokerHash: brokerHash, CaseCatalogHash: caseCatalogHash, GatesHash: gatesHash, TreatmentPluginHash: pluginHash, EmptyControlPluginHash: controlHash}
	rows := make([]resultRow, 0, len(cases.Cases)*pairedRuns*2)
	for i, c := range cases.Cases {
		promptHash := hashBytes([]byte(c.Task))
		fixtureHash := hashFixture(c.Files)
		for repetition := 1; repetition <= pairedRuns; repetition++ {
			blockID := fmt.Sprintf("%s-%03d-%03d", c.ID, i+1, repetition)
			arms := []string{"treatment", "control"}
			if (i+repetition)%2 == 1 {
				arms = []string{"control", "treatment"}
			}
			for _, arm := range arms {
				armRoot := filepath.Join(root, blockID, arm)
				home := filepath.Join(armRoot, "home")
				project := filepath.Join(armRoot, "project")
				if err := os.MkdirAll(home, 0o700); err != nil {
					return rows, manifest, err
				}
				if err := os.MkdirAll(project, 0o700); err != nil {
					return rows, manifest, err
				}
				if err := materializeFixture(project, c.Files); err != nil {
					return rows, manifest, err
				}
				beforeDefects, err := countMarkers(project, c.SeededDefects)
				if err != nil {
					return rows, manifest, err
				}
				pluginRoot := ""
				armPluginHash := controlHash
				if arm == "treatment" {
					pluginRoot = actorPluginRoot
					armPluginHash = pluginHash
				}
				actorTimeout := opts.ActorTimeout
				if actorTimeout == 0 {
					actorTimeout = 20 * time.Minute
				}
				request := actorRequest{Case: c, Arm: arm, Harness: opts.Harness, Model: opts.Model, Effort: opts.Effort, Home: home, Project: project, PluginRoot: pluginRoot, Timeout: actorTimeout}
				actorCtx, cancelActor := context.WithTimeout(ctx, actorTimeout)
				result, actorErr := subject.Run(actorCtx, request)
				timedOut := errors.Is(actorCtx.Err(), context.DeadlineExceeded)
				cancelActor()
				if result.CLIVersion != "" {
					cliVersion = result.CLIVersion
				}
				row := baseRow(c, arm, blockID, opts, revision, cliVersion, promptHash, fixtureHash, armPluginHash)
				if result.Sandbox != "" {
					row.Environment.Sandbox = portableIdentifier(result.Sandbox)
				}
				row.DurationMS = max(result.Duration.Milliseconds(), 0)
				row.RC = result.RC
				row.Artifacts = map[string]string{"response": hashBytes([]byte(result.Response)), "trace": hashBytes(result.Trace)}
				if actorErr != nil || result.RC != 0 || timedOut {
					row.Status = "harness_error"
					if timedOut {
						row.Status = "timeout"
						row.RC = 124
					}
					row.Verdict = "harness_error"
					row.Metrics = map[string]float64{"task_success": 0}
					rows = append(rows, row)
					inventory := cleanInventory(result.Inventory)
					manifest.Arms = append(manifest.Arms, armManifest{CaseID: c.ID, BlockID: blockID, Arm: arm, PromptHash: promptHash, FixtureHash: fixtureHash, PluginHash: armPluginHash, PluginNames: inventory, InventoryHash: hashInventory(inventory), EvidenceOnly: evidenceLabel(opts.Selftest)})
					_ = writePublish(opts.Out, rows, manifest)
					if timedOut {
						return rows, manifest, fmt.Errorf("%s/%s actor timed out after %s", c.ID, arm, actorTimeout)
					}
					if actorErr != nil {
						return rows, manifest, fmt.Errorf("%s/%s actor error: %w", c.ID, arm, actorErr)
					}
					return rows, manifest, fmt.Errorf("%s/%s actor exited %d", c.ID, arm, result.RC)
				}
				metrics, verdict, err := evaluateCase(ctx, c, gates, project, beforeDefects, result)
				if err != nil {
					return rows, manifest, fmt.Errorf("%s/%s oracle: %w", c.ID, arm, err)
				}
				row.Metrics = metrics
				row.Status = "completed"
				row.Verdict = verdict
				rows = append(rows, row)
				inventory := cleanInventory(result.Inventory)
				manifest.Arms = append(manifest.Arms, armManifest{CaseID: c.ID, BlockID: blockID, Arm: arm, PromptHash: promptHash, FixtureHash: fixtureHash, PluginHash: armPluginHash, PluginNames: inventory, InventoryHash: hashInventory(inventory), EvidenceOnly: evidenceLabel(opts.Selftest)})
			}
		}
	}
	manifest.Acceptance = assessStudyAcceptance(rows, gates)
	if err := writePublish(opts.Out, rows, manifest); err != nil {
		return rows, manifest, err
	}
	return rows, manifest, nil
}

func emptyControlPluginHash() string { return hashBytes(nil) }

func hashInventory(names []string) string {
	canonical := append([]string(nil), names...)
	sort.Strings(canonical)
	content, _ := json.Marshal(canonical)
	return hashBytes(append(content, '\n'))
}

func assessStudyAcceptance(rows []resultRow, gates gatesFile) studyAcceptance {
	assessment := studyAcceptance{
		Accepted:                  true,
		MinimumPairedRuns:         gates.Acceptance.MinimumPairedRuns,
		RequireAllTreatmentPasses: gates.Acceptance.RequireAllTreatmentPasses,
	}
	type counts struct{ treatmentPass, treatmentTotal, controlPass, controlTotal int }
	byCase := map[string]*counts{}
	for _, row := range rows {
		if row.Status != "completed" || (row.Arm != "treatment" && row.Arm != "control") {
			continue
		}
		if row.Metrics["report_only"] == 1 {
			continue
		}
		if byCase[row.CaseID] == nil {
			byCase[row.CaseID] = &counts{}
		}
		cell := byCase[row.CaseID]
		if row.Arm == "treatment" {
			cell.treatmentTotal++
			if row.Verdict == "pass" {
				cell.treatmentPass++
			}
		} else {
			cell.controlTotal++
			if row.Verdict == "pass" {
				cell.controlPass++
			}
		}
	}
	caseIDs := make([]string, 0, len(byCase))
	for caseID := range byCase {
		caseIDs = append(caseIDs, caseID)
	}
	sort.Strings(caseIDs)
	for _, caseID := range caseIDs {
		cell := byCase[caseID]
		paired := minInt(cell.treatmentTotal, cell.controlTotal)
		treatmentRate := rate(cell.treatmentPass, cell.treatmentTotal)
		controlRate := rate(cell.controlPass, cell.controlTotal)
		observed := treatmentRate - controlRate
		accepted := paired >= gates.Acceptance.MinimumPairedRuns && cell.treatmentTotal == cell.controlTotal && cell.treatmentPass == cell.treatmentTotal
		assessment.Cases = append(assessment.Cases, caseAssessment{CaseID: caseID, PairedRuns: paired, TreatmentPasses: cell.treatmentPass, ControlPasses: cell.controlPass, TreatmentPassRate: treatmentRate, ControlPassRate: controlRate, ObservedLift: observed, Accepted: accepted})
		if paired < gates.Acceptance.MinimumPairedRuns || cell.treatmentTotal != cell.controlTotal {
			assessment.Reasons = append(assessment.Reasons, fmt.Sprintf("%s has %d balanced pairs, require %d", caseID, paired, gates.Acceptance.MinimumPairedRuns))
		}
		if cell.treatmentPass != cell.treatmentTotal {
			assessment.Reasons = append(assessment.Reasons, fmt.Sprintf("%s treatment passed %d/%d; require every treatment run to pass", caseID, cell.treatmentPass, cell.treatmentTotal))
		}
		assessment.Accepted = assessment.Accepted && accepted
	}
	if len(byCase) == 0 {
		assessment.Accepted = false
		assessment.Reasons = append(assessment.Reasons, "no completed treatment/control results")
	}
	return assessment
}

func rate(successes, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(successes) / float64(total)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func evidenceLabel(selftest bool) string {
	if selftest {
		return "selftest-only-not-behavioral-evidence"
	}
	return "credentialed-behavioral-run"
}

func baseRow(c studyCase, arm, block string, opts runOptions, revision, cliVersion, promptHash, fixtureHash, pluginHash string) resultRow {
	timestamp := opts.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	sandbox := "broker-isolated"
	if opts.Selftest {
		sandbox = "in-process-selftest"
	}
	return resultRow{
		SchemaVersion: "1", Study: "installed-plugin-ab", EvidenceClass: "behavioral", CaseID: c.ID,
		RunID: fmt.Sprintf("%s-%s-%s", block, arm, timestamp.Format("20060102T150405Z")), BlockID: block, Arm: arm,
		Harness:    harnessIdentity{Name: opts.Harness, CLIVersion: portableIdentifier(cliVersion), Model: portableIdentifier(opts.Model), Effort: portableIdentifier(opts.Effort)},
		Source:     sourceIdentity{Repository: "megapowers", Revision: portableIdentifier(revision)},
		PromptHash: promptHash, FixtureHash: fixtureHash, PluginHash: pluginHash,
		Environment: environment{OS: runtime.GOOS, Arch: runtime.GOARCH, Sandbox: sandbox, Locale: portableIdentifier(locale())},
		Timestamp:   timestamp.Format(time.RFC3339), Artifacts: map[string]string{}, Metrics: map[string]float64{},
	}
}

func evaluateCase(ctx context.Context, c studyCase, gates gatesFile, project string, beforeDefects int, result actorResult) (map[string]float64, string, error) {
	metrics := map[string]float64{}
	pass := false
	switch c.Kind {
	case "prose":
		retained, invented := factCounts(result.Response, c.RequiredFacts, c.ForbiddenFacts)
		retention := float64(retained) / float64(len(c.RequiredFacts))
		metrics["fact_retention"] = retention
		metrics["invented_facts"] = float64(invented)
		pass = retention >= gates.Prose.MinimumFactRetention && invented <= gates.Prose.MaximumInventedFacts
		if c.RequireNoop {
			unchanged := strings.TrimRight(result.Response, " \t\r\n") == strings.TrimRight(c.ExpectedOutput, " \t\r\n")
			metrics["noop_preservation"] = boolMetric(unchanged)
			pass = pass && (!gates.Prose.RequireNoopPreservation || unchanged)
		}
	case "code_quality":
		afterDefects, err := countMarkers(project, c.SeededDefects)
		if err != nil {
			return nil, "", err
		}
		regressions, err := countMarkers(project, c.ForbiddenPatterns)
		if err != nil {
			return nil, "", err
		}
		oracleRC, err := runOracle(ctx, project, c.OracleCommand)
		if err != nil {
			return nil, "", err
		}
		reduction := beforeDefects - afterDefects
		metrics["seeded_defect_reduction"] = float64(reduction)
		metrics["convention_regressions"] = float64(regressions)
		metrics["oracle_pass"] = boolMetric(oracleRC == 0)
		pass = oracleRC == 0 && reduction >= gates.CodeQuality.MinimumSeededDefectReduction && regressions <= gates.CodeQuality.MaximumConventionRegressions
	case "tdd":
		protectedIntact, err := protectedFilesIntact(project, c)
		if err != nil {
			return nil, "", err
		}
		oracleRC := 1
		if protectedIntact {
			oracleRC, err = runOracle(ctx, project, c.OracleCommand)
			if err != nil {
				return nil, "", err
			}
		}
		testFirst, redFirst := tddEvidence(result.Events)
		metrics["test_before_implementation"] = boolMetric(testFirst)
		metrics["red_before_implementation"] = boolMetric(redFirst)
		metrics["protected_fixture_intact"] = boolMetric(protectedIntact)
		metrics["oracle_pass"] = boolMetric(oracleRC == 0)
		pass = protectedIntact && (!gates.TDD.RequireTestBeforeImplementation || testFirst) && (!gates.TDD.RequireRedBeforeImplementation || redFirst) && (!gates.TDD.RequirePassingOracle || oracleRC == 0)
	case "autonomy_status":
		retained, invented := factCounts(result.Response, c.RequiredFacts, c.ForbiddenFacts)
		retention := float64(retained) / float64(len(c.RequiredFacts))
		metrics["fact_retention"] = retention
		metrics["invented_facts"] = float64(invented)
		metrics["report_only"] = 1
		pass = retention >= gates.AutonomyStatus.MinimumFactRetention && invented <= gates.AutonomyStatus.MaximumInventedFacts
	}
	metrics["task_success"] = boolMetric(pass)
	if pass {
		return metrics, "pass", nil
	}
	return metrics, "fail", nil
}

func factCounts(response string, required, forbidden []string) (int, int) {
	lower := strings.ToLower(response)
	retained := 0
	for _, fact := range required {
		if containsDelimitedFact(lower, strings.ToLower(fact)) {
			retained++
		}
	}
	invented := 0
	for _, fact := range forbidden {
		if containsDelimitedFact(lower, strings.ToLower(fact)) {
			invented++
		}
	}
	return retained, invented
}

func containsDelimitedFact(response, fact string) bool {
	if fact == "" {
		return false
	}
	for offset := 0; offset <= len(response)-len(fact); {
		index := strings.Index(response[offset:], fact)
		if index < 0 {
			return false
		}
		index += offset
		end := index + len(fact)
		beforeOK := index == 0 || !isASCIIWord(response[index-1])
		afterOK := end == len(response) || !isASCIIWord(response[end])
		if beforeOK && afterOK {
			return true
		}
		offset = index + 1
	}
	return false
}

func isASCIIWord(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9' || value == '_'
}

func runOracle(ctx context.Context, dir string, argv []string) (int, error) {
	if len(argv) == 0 {
		return 0, errors.New("oracle command is empty")
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 0, fmt.Errorf("could not execute oracle: %w", err)
}

func tddEvidence(events []actorEvent) (bool, bool) {
	firstTest, firstImpl, firstRed := -1, -1, -1
	for i, event := range events {
		step := event.Step
		if step == 0 {
			step = i + 1
		}
		switch event.Kind {
		case "write":
			if isTestPath(event.Path) && firstTest < 0 {
				firstTest = step
			}
			if !isTestPath(event.Path) && isImplementationPath(event.Path) && firstImpl < 0 {
				firstImpl = step
			}
		case "test":
			if event.RC != 0 && firstRed < 0 {
				firstRed = step
			}
		}
	}
	testFirst := firstTest >= 0 && firstImpl >= 0 && firstTest < firstImpl
	redFirst := testFirst && firstRed > firstTest && firstRed < firstImpl
	return testFirst, redFirst
}

func isTestPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.Contains(base, "test") || strings.HasSuffix(base, ".spec.ts") || strings.HasSuffix(base, ".test.ts")
}

func isImplementationPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".go" || ext == ".ts" || ext == ".js" || ext == ".py" || ext == ".rs"
}

func materializeFixture(root string, files map[string]string) error {
	for name, content := range files {
		if err := safeRelative(name); err != nil {
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func protectedFilesIntact(root string, c studyCase) (bool, error) {
	rooted, err := os.OpenRoot(root)
	if err != nil {
		return false, err
	}
	defer rooted.Close()
	for _, name := range c.ProtectedFiles {
		expected, ok := c.Files[name]
		if !ok {
			return false, fmt.Errorf("protected file %q is absent from fixture", name)
		}
		rootedName := filepath.FromSlash(name)
		info, err := rooted.Lstat(rootedName)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return false, nil
		}
		file, err := rooted.Open(rootedName)
		if err != nil {
			return false, err
		}
		content, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return false, readErr
		}
		if closeErr != nil {
			return false, closeErr
		}
		if !bytes.Equal(content, []byte(expected)) {
			return false, nil
		}
	}
	return true, nil
}

func countMarkers(root string, markers []string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			if filepath.Dir(path) == root && (entry.Name() == ".actor-home" || entry.Name() == ".actor-tmp" || entry.Name() == ".actor-cache") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, marker := range markers {
			count += bytes.Count(content, []byte(marker))
		}
		return nil
	})
	return count, err
}

func hashFixture(files map[string]string) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		fmt.Fprintf(h, "%d:%s:%d:", len(name), name, len(files[name]))
		h.Write([]byte(files[name]))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

type trackedPluginFile struct {
	mode   string
	object string
}

// verifiedPluginHash binds evidence to the exact regular files Git will ship.
// Ignored or untracked payloads, symlinks, special files, and worktree drift
// fail closed instead of entering an otherwise clean-looking comparison.
func verifiedPluginHash(repo string) (string, error) {
	const pluginPath = "plugins/megapowers"
	formatOutput, err := exec.Command("git", "-C", repo, "rev-parse", "--show-object-format").Output()
	if err != nil {
		return "", fmt.Errorf("read repository object format: %w", err)
	}
	objectFormat := strings.TrimSpace(string(formatOutput))
	if objectFormat != "sha1" && objectFormat != "sha256" {
		return "", fmt.Errorf("unsupported repository object format %q", objectFormat)
	}
	listing, err := exec.Command("git", "-C", repo, "ls-tree", "-r", "-z", "HEAD", "--", pluginPath).Output()
	if err != nil {
		return "", fmt.Errorf("list committed plugin tree: %w", err)
	}
	expected := make(map[string]trackedPluginFile)
	for _, raw := range bytes.Split(listing, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		parts := bytes.SplitN(raw, []byte{'\t'}, 2)
		if len(parts) != 2 {
			return "", errors.New("committed plugin tree has malformed metadata")
		}
		header := strings.Fields(string(parts[0]))
		name := filepath.ToSlash(string(parts[1]))
		if len(header) != 3 || header[1] != "blob" || (header[0] != "100644" && header[0] != "100755") {
			return "", fmt.Errorf("committed plugin entry %q is not a regular portable file", name)
		}
		expected[name] = trackedPluginFile{mode: header[0], object: header[2]}
	}
	if len(expected) == 0 {
		return "", errors.New("committed plugin tree is empty")
	}

	root := filepath.Join(repo, filepath.FromSlash(pluginPath))
	type hashedFile struct {
		name    string
		mode    string
		content []byte
	}
	files := make([]hashedFile, 0, len(expected))
	seen := make(map[string]bool, len(expected))
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		repoRelative, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		repoName := filepath.ToSlash(repoRelative)
		want, ok := expected[repoName]
		if !ok {
			return fmt.Errorf("plugin tree contains uncommitted path %q", repoName)
		}
		pathInfo, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !pathInfo.Mode().IsRegular() {
			return fmt.Errorf("plugin path %q is not a regular file", repoName)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		openedInfo, statErr := file.Stat()
		content, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if statErr != nil || readErr != nil || closeErr != nil {
			return fmt.Errorf("read plugin path %q", repoName)
		}
		if !openedInfo.Mode().IsRegular() || !os.SameFile(pathInfo, openedInfo) {
			return fmt.Errorf("plugin path %q changed while it was verified", repoName)
		}
		mode := "100644"
		if openedInfo.Mode().Perm()&0o111 != 0 {
			mode = "100755"
		}
		if mode != want.mode {
			return fmt.Errorf("plugin path %q mode differs from HEAD", repoName)
		}
		if gitBlobHash(content, objectFormat) != want.object {
			return fmt.Errorf("plugin path %q content differs from HEAD", repoName)
		}
		pluginRelative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, hashedFile{name: filepath.ToSlash(pluginRelative), mode: mode, content: content})
		seen[repoName] = true
		return nil
	})
	if err != nil {
		return "", err
	}
	for name := range expected {
		if !seen[name] {
			return "", fmt.Errorf("committed plugin path %q is missing", name)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	h := sha256.New()
	for _, file := range files {
		fmt.Fprintf(h, "%d:%s:%s:%d:", len(file.name), file.name, file.mode, len(file.content))
		h.Write(file.content)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func gitBlobHash(content []byte, format string) string {
	header := []byte(fmt.Sprintf("blob %d%c", len(content), 0))
	if format == "sha256" {
		h := sha256.New()
		h.Write(header)
		h.Write(content)
		return hex.EncodeToString(h.Sum(nil))
	}
	h := sha1.New()
	h.Write(header)
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

func verifyStudyConfiguration(repo, casesPath, gatesPath string) error {
	expectedCases := filepath.Join(repo, "evals", "studies", "installed-ab", "cases.json")
	expectedGates := filepath.Join(repo, "evals", "studies", "installed-ab", "gates.json")
	for _, pair := range []struct {
		got  string
		want string
	}{{casesPath, expectedCases}, {gatesPath, expectedGates}} {
		got, err := filepath.Abs(pair.got)
		if err != nil {
			return err
		}
		want, err := filepath.Abs(pair.want)
		if err != nil {
			return err
		}
		if filepath.Clean(got) != filepath.Clean(want) {
			return errors.New("real studies require the checkout's committed cases.json and gates.json")
		}
		if err := verifyRegularFileAtHEAD(repo, want); err != nil {
			return err
		}
	}
	return nil
}

func verifyRegularFileAtHEAD(repo, path string) error {
	relative, err := filepath.Rel(repo, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("study configuration path escapes the checkout")
	}
	repoName := filepath.ToSlash(relative)
	formatOutput, err := exec.Command("git", "-C", repo, "rev-parse", "--show-object-format").Output()
	if err != nil {
		return fmt.Errorf("read repository object format: %w", err)
	}
	objectFormat := strings.TrimSpace(string(formatOutput))
	if objectFormat != "sha1" && objectFormat != "sha256" {
		return fmt.Errorf("unsupported repository object format %q", objectFormat)
	}
	listing, err := exec.Command("git", "-C", repo, "ls-tree", "-z", "HEAD", "--", repoName).Output()
	if err != nil || len(listing) == 0 {
		return fmt.Errorf("study configuration %q is not committed at HEAD", repoName)
	}
	entry := bytes.TrimSuffix(listing, []byte{0})
	parts := bytes.SplitN(entry, []byte{'\t'}, 2)
	if len(parts) != 2 || filepath.ToSlash(string(parts[1])) != repoName {
		return fmt.Errorf("study configuration %q has malformed Git metadata", repoName)
	}
	header := strings.Fields(string(parts[0]))
	if len(header) != 3 || header[0] != "100644" || header[1] != "blob" {
		return fmt.Errorf("study configuration %q is not a regular non-executable file", repoName)
	}
	pathInfo, err := os.Lstat(path)
	if err != nil || !pathInfo.Mode().IsRegular() || pathInfo.Mode().Perm()&0o111 != 0 {
		return fmt.Errorf("study configuration %q is not a regular non-executable worktree file", repoName)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	openedInfo, statErr := file.Stat()
	content, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if statErr != nil || readErr != nil || closeErr != nil || !os.SameFile(pathInfo, openedInfo) {
		return fmt.Errorf("study configuration %q changed while it was verified", repoName)
	}
	if gitBlobHash(content, objectFormat) != header[2] {
		return fmt.Errorf("study configuration %q content differs from HEAD", repoName)
	}
	return nil
}

func hashPortablePluginTree(root string) (string, error) {
	type fileRecord struct {
		name    string
		mode    string
		content []byte
	}
	var files []fileRecord
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		pathInfo, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !pathInfo.Mode().IsRegular() {
			return fmt.Errorf("plugin path %q is not a regular file", path)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		openedInfo, statErr := file.Stat()
		content, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if statErr != nil || readErr != nil || closeErr != nil {
			return fmt.Errorf("read plugin path %q", path)
		}
		if !openedInfo.Mode().IsRegular() || !os.SameFile(pathInfo, openedInfo) {
			return fmt.Errorf("plugin path %q changed while it was hashed", path)
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		mode := "100644"
		if openedInfo.Mode().Perm()&0o111 != 0 {
			mode = "100755"
		}
		files = append(files, fileRecord{name: filepath.ToSlash(name), mode: mode, content: content})
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", errors.New("plugin tree is empty")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	h := sha256.New()
	for _, file := range files {
		fmt.Fprintf(h, "%d:%s:%s:%d:", len(file.name), file.name, file.mode, len(file.content))
		h.Write(file.content)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func stagePortablePluginTree(source, destination, expectedHash string) error {
	if err := os.Mkdir(destination, 0o700); err != nil {
		return fmt.Errorf("create verified plugin directory: %w", err)
	}
	directories := []string{destination}
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			if err := os.Mkdir(target, 0o700); err != nil {
				return err
			}
			directories = append(directories, target)
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("plugin path %q is not a regular file", path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o400)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o500
		}
		if err := os.WriteFile(target, content, mode); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	actualHash, err := hashPortablePluginTree(destination)
	if err != nil {
		return err
	}
	if actualHash != expectedHash {
		return errors.New("plugin tree changed before its verified execution copy was created")
	}
	sort.Slice(directories, func(i, j int) bool { return len(directories[i]) > len(directories[j]) })
	for _, directory := range directories {
		if err := os.Chmod(directory, 0o500); err != nil {
			return err
		}
	}
	return nil
}

func hashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func writePublish(out string, rows []resultRow, manifest publishManifest) error {
	if out == "" {
		return nil
	}
	publish := filepath.Join(out, "publish")
	if err := os.MkdirAll(publish, 0o755); err != nil {
		return err
	}
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return err
		}
	}
	if err := atomicWrite(filepath.Join(publish, "results.jsonl"), data.Bytes(), 0o644); err != nil {
		return err
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestBytes = append(manifestBytes, '\n')
	return atomicWrite(filepath.Join(publish, "manifest.json"), manifestBytes, 0o644)
}

func atomicWrite(path string, content []byte, mode fs.FileMode) error {
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
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func removePrivateTree(root string) {
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			_ = os.Chmod(path, 0o700)
		}
		return nil
	})
	_ = os.RemoveAll(root)
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
	PluginRepo     string   `json:"plugin_repo,omitempty"`
	TaskReadRoots  []string `json:"task_read_roots"`
	TaskWriteRoots []string `json:"task_write_roots"`
	TimeoutMS      int64    `json:"timeout_ms"`
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
	Events          []actorEvent         `json:"events"`
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
	payload, roots := makeBrokerRequest(request)
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
	if err := validateIsolation(response, roots, request.Arm); err != nil {
		return actorResult{RC: 125}, err
	}
	return actorResult{Response: response.Response, Trace: []byte(response.Trace), Events: response.Events, Inventory: response.PluginInventory, CLIVersion: response.CLIVersion, Sandbox: response.Isolation.Boundary, RC: response.RC, Duration: time.Duration(response.DurationMS) * time.Millisecond}, nil
}

func makeBrokerRequest(request actorRequest) (brokerRequest, []string) {
	pluginRepo := ""
	roots := []string{request.Project}
	if request.Arm == "treatment" {
		pluginRepo = request.PluginRoot
		roots = append(roots, request.PluginRoot)
	}
	brokerTimeout := request.Timeout - 5*time.Second
	if brokerTimeout <= 0 {
		brokerTimeout = request.Timeout
	}
	return brokerRequest{SchemaVersion: "1", Harness: request.Harness, Model: request.Model, Effort: request.Effort, Arm: request.Arm, Task: request.Case.Task, Project: request.Project, PluginRepo: pluginRepo, TaskReadRoots: roots, TaskWriteRoots: []string{request.Project}, TimeoutMS: brokerTimeout.Milliseconds()}, roots
}

func validateIsolation(response brokerResponse, expectedRoots []string, arm string) error {
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
		return errors.New("sandbox broker must limit actor writes to the current project")
	}
	if arm == "control" && (response.PluginInventory == nil || len(response.PluginInventory) != 0) {
		return errors.New("control broker inventory is not empty")
	}
	if arm == "treatment" && (len(response.PluginInventory) != 1 || response.PluginInventory[0] != "megapowers") {
		return errors.New("treatment broker inventory is not exactly megapowers")
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

func gitOutput(dir string, args ...string) string {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
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

// syscall.Umask is isolated to these tiny helpers so the rest of the runner
// remains straightforward and all created credential state starts private.
func setUmask077() int       { return syscall.Umask(0o077) }
func restoreUmask(value int) { syscall.Umask(value) }

type fakeActor struct {
	FailArm      string
	Homes        []string
	Projects     []string
	PluginRoots  []string
	ObservedMode []fs.FileMode
}

type blockingActor struct{}

func (blockingActor) Run(ctx context.Context, request actorRequest) (actorResult, error) {
	<-ctx.Done()
	return actorResult{RC: 124, Inventory: inventoryFor(request), Sandbox: "in-process-selftest"}, ctx.Err()
}

func (f *fakeActor) Run(_ context.Context, request actorRequest) (actorResult, error) {
	info, err := os.Stat(request.Home)
	if err != nil {
		return actorResult{RC: 125}, err
	}
	f.Homes = append(f.Homes, request.Home)
	f.Projects = append(f.Projects, request.Project)
	f.PluginRoots = append(f.PluginRoots, request.PluginRoot)
	f.ObservedMode = append(f.ObservedMode, info.Mode().Perm())
	if request.Arm == f.FailArm {
		return actorResult{RC: 42, Inventory: inventoryFor(request), Sandbox: "in-process-selftest"}, errors.New("synthetic actor failure")
	}
	result := actorResult{Inventory: inventoryFor(request), CLIVersion: "selftest", Sandbox: "in-process-selftest", Trace: []byte("synthetic trace"), RC: 0, Duration: time.Millisecond}
	if request.Arm == "control" {
		result.Response = "A generic response without the supplied facts."
		return result, nil
	}
	switch request.Case.Kind {
	case "prose":
		if request.Case.RequireNoop {
			result.Response = request.Case.ExpectedOutput + "\n"
		} else {
			result.Response = strings.Join(request.Case.RequiredFacts, ". ") + "."
		}
	case "code_quality":
		content := "package store\n\nimport \"os\"\n\nfunc Load(path string) (string, error) {\n\tdata, err := os.ReadFile(path)\n\tif err != nil { return \"\", err }\n\treturn string(data), nil\n}\n"
		if err := os.WriteFile(filepath.Join(request.Project, "store.go"), []byte(content), 0o600); err != nil {
			return actorResult{RC: 125}, err
		}
	case "tdd":
		test := "package calculator\n\nimport \"testing\"\n\nfunc TestMultiply(t *testing.T) { if Multiply(3, 4) != 12 { t.Fatal(\"bad product\") } }\n"
		impl := "package calculator\n\nfunc Add(a, b int) int { return a + b }\nfunc Multiply(a, b int) int { return a * b }\n"
		if err := os.WriteFile(filepath.Join(request.Project, "calculator_test.go"), []byte(test), 0o600); err != nil {
			return actorResult{RC: 125}, err
		}
		if err := os.WriteFile(filepath.Join(request.Project, "calculator.go"), []byte(impl), 0o600); err != nil {
			return actorResult{RC: 125}, err
		}
		result.Events = []actorEvent{{Kind: "write", Path: "calculator_test.go", Step: 1}, {Kind: "test", RC: 1, Step: 2}, {Kind: "write", Path: "calculator.go", Step: 3}, {Kind: "test", RC: 0, Step: 4}}
	case "autonomy_status":
		result.Response = strings.Join(request.Case.RequiredFacts, ". ") + "."
	}
	return result, nil
}

func inventoryFor(request actorRequest) []string {
	if request.Arm == "treatment" && request.PluginRoot != "" {
		return []string{"megapowers"}
	}
	return []string{}
}

func runSelftest() error {
	root, err := locateRoot("")
	if err != nil {
		return err
	}
	cases, gates, err := loadConfiguration(filepath.Join(root, "evals", "studies", "installed-ab", "cases.json"), filepath.Join(root, "evals", "studies", "installed-ab", "gates.json"))
	if err != nil {
		return err
	}
	parent, err := os.MkdirTemp("", "installed-ab-selftest-*")
	if err != nil {
		return err
	}
	defer removePrivateTree(parent)
	out := filepath.Join(parent, "success")
	fake := &fakeActor{}
	opts := runOptions{Harness: "codex", Model: "fake-selftest", Effort: "test", Repo: root, Out: out, TempRoot: parent, Timestamp: time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC), Selftest: true, PairedRuns: 1}
	rows, manifest, err := executeStudy(context.Background(), cases, gates, opts, fake)
	if err != nil {
		return err
	}
	private := true
	for _, mode := range fake.ObservedMode {
		private = private && mode == 0o700
	}
	isolated := len(fake.Homes) == len(cases.Cases)*2 && unique(fake.Homes) && unique(fake.Projects) && private
	printCheck("isolated private homes", isolated)
	if !isolated {
		return errors.New("selftest observed reused or non-private homes")
	}
	identical := pairedHashesMatch(rows)
	printCheck("identical treatment and control inputs", identical)
	if !identical {
		return errors.New("paired input hashes differ")
	}
	diagnosticWritten := len(rows) > 0 && !manifest.Acceptance.Accepted && len(manifest.Acceptance.Reasons) > 0
	printCheck("undersampled diagnostics still write results", diagnosticWritten)
	if !diagnosticWritten {
		return errors.New("undersampled diagnostic did not retain its negative study result")
	}
	boundaryRecorded := true
	for _, row := range rows {
		boundaryRecorded = boundaryRecorded && row.Environment.Sandbox == "in-process-selftest"
	}
	printCheck("rows record the actor sandbox boundary", boundaryRecorded)
	if !boundaryRecorded {
		return errors.New("result rows lost the actor sandbox boundary")
	}
	inventoryOK := inventoriesCorrect(manifest)
	printCheck("plugin inventory records empty control", inventoryOK)
	if !inventoryOK {
		return errors.New("plugin inventory is incorrect")
	}
	noopOK := metricFor(rows, "humanizing-prose-noop", "treatment", "noop_preservation") == 1
	printCheck("already-direct prose remains unchanged", noopOK)
	if !noopOK {
		return errors.New("already-direct prose was not preserved")
	}
	var prosePlan studyCase
	for _, c := range cases.Cases {
		if c.ID == "humanizing-prose-plan" {
			prosePlan = c
			break
		}
	}
	retained, invented := factCounts("Blocked because credentials are unavailable. Next, request a scoped token.", prosePlan.RequiredFacts, prosePlan.ForbiddenFacts)
	naturalRewriteOK := retained == len(prosePlan.RequiredFacts) && invented == 0
	printCheck("natural prose preserves atomic facts", naturalRewriteOK)
	if !naturalRewriteOK {
		return errors.New("natural prose was rejected by the fact oracle")
	}
	reversalRejected := true
	for _, reversed := range []string{
		"Unblocked because credentials are unavailable. Next, request a scoped token.",
		"Not blocked because credentials are unavailable. Do not request a scoped token.",
	} {
		retained, invented = factCounts(reversed, prosePlan.RequiredFacts, prosePlan.ForbiddenFacts)
		reversalRejected = reversalRejected && (retained < len(prosePlan.RequiredFacts) || invented > 0)
	}
	printCheck("fact reversals fail the prose oracle", reversalRejected)
	if !reversalRejected {
		return errors.New("fact reversal passed the prose oracle")
	}
	markerRoot := filepath.Join(parent, "marker-special-files")
	if err := os.MkdirAll(filepath.Join(markerRoot, ".actor-tmp"), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(markerRoot, "source.go"), []byte("seeded-defect\n"), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(markerRoot, ".actor-tmp", "runtime.log"), []byte("seeded-defect\n"), 0o600); err != nil {
		return err
	}
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(markerRoot); err != nil {
		return err
	}
	listener, listenErr := net.Listen("unix", filepath.Join(".actor-tmp", "runtime.sock"))
	restoreErr := os.Chdir(originalDir)
	if listenErr != nil {
		return listenErr
	}
	if restoreErr != nil {
		listener.Close()
		return restoreErr
	}
	markerCount, markerErr := countMarkers(markerRoot, []string{"seeded-defect"})
	if err := listener.Close(); err != nil {
		return err
	}
	markerScanSafe := markerErr == nil && markerCount == 1
	printCheck("marker scan ignores actor runtime state", markerScanSafe)
	if !markerScanSafe {
		return fmt.Errorf("marker scan included actor runtime state: count=%d err=%v", markerCount, markerErr)
	}
	autonomyOK := metricFor(rows, "autonomous-run-resume-status", "treatment", "report_only") == 1 && metricFor(rows, "autonomous-run-resume-status", "treatment", "fact_retention") == 1
	printCheck("autonomous status resumption stays report-only", autonomyOK)
	if !autonomyOK {
		return errors.New("autonomy status case is not report-only")
	}
	var autonomyCase studyCase
	for _, c := range cases.Cases {
		if c.ID == "autonomous-run-resume-status" {
			autonomyCase = c
			break
		}
	}
	completionClaimsRejected := true
	for _, claim := range []string{
		"TASK-2. Next: run tests/focused.sh. Work completed.",
		"TASK-2. Next: run tests/focused.sh. Completion is confirmed.",
	} {
		_, invented = factCounts(claim, autonomyCase.RequiredFacts, autonomyCase.ForbiddenFacts)
		completionClaimsRejected = completionClaimsRejected && invented > 0
	}
	printCheck("autonomy completion claims fail the oracle", completionClaimsRejected)
	if !completionClaimsRejected {
		return errors.New("autonomy completion claim passed the oracle")
	}
	sameBatchTestFirst, sameBatchRed := tddEvidence([]actorEvent{{Kind: "write", Path: "calculator_test.go", Step: 1}, {Kind: "write", Path: "calculator.go", Step: 1}, {Kind: "test", RC: 1, Step: 2}})
	simultaneousRejected := !sameBatchTestFirst && !sameBatchRed
	printCheck("simultaneous test and implementation edit rejected", simultaneousRejected)
	if !simultaneousRejected {
		return errors.New("simultaneous file changes passed TDD ordering")
	}
	var tddCase studyCase
	for _, c := range cases.Cases {
		if c.ID == "tdd-add-multiply" {
			tddCase = c
			break
		}
	}
	tddBaseline := filepath.Join(parent, "tdd-baseline")
	if err := os.MkdirAll(tddBaseline, 0o700); err != nil {
		return err
	}
	if err := materializeFixture(tddBaseline, tddCase.Files); err != nil {
		return err
	}
	baselineRC, err := runOracle(context.Background(), tddBaseline, tddCase.OracleCommand)
	if err != nil {
		return err
	}
	baselineRed := baselineRC != 0
	printCheck("unchanged TDD fixture fails its public oracle", baselineRed)
	if !baselineRed {
		return errors.New("unchanged TDD fixture passed its public oracle")
	}
	tamperCases := []struct {
		name   string
		mutate func(string) error
	}{
		{name: "rewrite", mutate: func(path string) error { return os.WriteFile(path, []byte("package calculator\n"), 0o600) }},
		{name: "delete", mutate: os.Remove},
		{name: "symlink", mutate: func(path string) error {
			if err := os.Remove(path); err != nil {
				return err
			}
			return os.Symlink("calculator.go", path)
		}},
		{name: "nonregular", mutate: func(path string) error {
			if err := os.Remove(path); err != nil {
				return err
			}
			return os.Mkdir(path, 0o700)
		}},
	}
	protectedTamperRejected := true
	for _, test := range tamperCases {
		fixtureRoot := filepath.Join(parent, "tdd-tamper-"+test.name)
		if err := os.MkdirAll(fixtureRoot, 0o700); err != nil {
			return err
		}
		if err := materializeFixture(fixtureRoot, tddCase.Files); err != nil {
			return err
		}
		if err := test.mutate(filepath.Join(fixtureRoot, "calculator_acceptance_test.go")); err != nil {
			return err
		}
		intact, err := protectedFilesIntact(fixtureRoot, tddCase)
		if err != nil {
			return err
		}
		protectedTamperRejected = protectedTamperRejected && !intact
	}
	printCheck("TDD oracle rejects protected fixture tampering", protectedTamperRejected)
	if !protectedTamperRejected {
		return errors.New("TDD oracle accepted protected fixture tampering")
	}
	_, brokerErr := validateBroker("", "", root, out)
	brokerRequired := brokerErr != nil
	printCheck("live runs require isolated broker", brokerRequired)
	if !brokerRequired {
		return errors.New("live run accepted no isolation broker")
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
	_, brokerLinkErr := validateBroker(brokerLink, hashBytes(brokerContent), root, out)
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
	pluginSource := filepath.Join(parent, "plugin-source")
	pluginStage := filepath.Join(parent, "plugin-stage")
	if err := os.Mkdir(pluginSource, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(pluginSource, "SKILL.md"), []byte("approved\n"), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(pluginSource, "hook"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		return err
	}
	approvedPluginHash, err := hashPortablePluginTree(pluginSource)
	if err != nil {
		return err
	}
	if err := stagePortablePluginTree(pluginSource, pluginStage, approvedPluginHash); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(pluginSource, "SKILL.md"), []byte("changed\n"), 0o600); err != nil {
		return err
	}
	stagedPluginHash, err := hashPortablePluginTree(pluginStage)
	if err != nil {
		return err
	}
	stagedSkill, err := os.Stat(filepath.Join(pluginStage, "SKILL.md"))
	if err != nil {
		return err
	}
	privatePluginCopy := stagedPluginHash == approvedPluginHash && stagedSkill.Mode().Perm() == 0o400
	printCheck("treatment uses a verified private plugin copy", privatePluginCopy)
	if !privatePluginCopy {
		return errors.New("staged plugin did not retain approved read-only bytes")
	}
	privateBrokerCWD := brokerCommand(context.Background(), stagedBroker).Dir == filepath.Dir(stagedBroker)
	printCheck("broker working directory is the verified private directory", privateBrokerCWD)
	if !privateBrokerCWD {
		return errors.New("sandbox broker retained a mutable original working directory")
	}
	brokerPayload, brokerRoots := makeBrokerRequest(actorRequest{Case: cases.Cases[0], Arm: "treatment", Harness: "codex", Model: "fake", Effort: "test", Project: filepath.Join(parent, "actor-project"), PluginRoot: filepath.Join(root, "plugins", "megapowers"), Timeout: time.Minute})
	repoExcluded := brokerPayload.PluginRepo == filepath.Join(root, "plugins", "megapowers") && brokerPayload.TimeoutMS == 55_000 && len(brokerRoots) == 2 && !samePaths(brokerRoots, []string{filepath.Join(parent, "actor-project"), root})
	printCheck("broker request excludes repository root", repoExcluded)
	if !repoExcluded {
		return errors.New("broker request exposed repository root")
	}
	validAttestation := brokerResponse{SchemaVersion: "1", CLIVersion: "selftest", PluginInventory: []string{"megapowers"}, Isolation: isolationAttestation{Boundary: "bwrap", CredentialsReadableByActor: boolPointer(false), SiblingStateReadableByActor: boolPointer(false), TaskReadRoots: brokerRoots, TaskWriteRoots: brokerRoots[:1]}}
	credentialLeak := validAttestation
	credentialLeak.Isolation.CredentialsReadableByActor = boolPointer(true)
	siblingLeak := validAttestation
	siblingLeak.Isolation.SiblingStateReadableByActor = boolPointer(true)
	missingAttestation := validAttestation
	missingAttestation.Isolation.CredentialsReadableByActor = nil
	leaksRejected := validateIsolation(credentialLeak, brokerRoots, "treatment") != nil && validateIsolation(siblingLeak, brokerRoots, "treatment") != nil && validateIsolation(missingAttestation, brokerRoots, "treatment") != nil
	printCheck("isolation attestation rejects credentials and siblings", leaksRejected)
	if !leaksRejected {
		return errors.New("unsafe isolation attestation was accepted")
	}
	insufficient := assessStudyAcceptance(syntheticGateRows(1, 0, 1), gates)
	insufficientRejected := !insufficient.Accepted
	printCheck("insufficient paired runs fail study acceptance", insufficientRejected)
	if !insufficientRejected {
		return errors.New("insufficient paired runs met study acceptance")
	}
	unreliable := assessStudyAcceptance(syntheticGateRows(9, 10, 10), gates)
	unreliableRejected := !unreliable.Accepted && unreliable.Cases[0].TreatmentPassRate == 0.9
	printCheck("treatment reliability fails study acceptance", unreliableRejected)
	if !unreliableRejected {
		return errors.New("imperfect treatment reliability met study acceptance")
	}
	diagnostic := assessStudyAcceptance(syntheticGateRows(10, 0, 10), gates)
	controlDiagnostic := diagnostic.Accepted && diagnostic.Cases[0].ControlPassRate == 0
	printCheck("control outcomes remain diagnostic", controlDiagnostic)
	if !controlDiagnostic {
		return errors.New("control outcome incorrectly blocked a reliable treatment")
	}
	perfect := assessStudyAcceptance(syntheticGateRows(10, 10, 10), gates)
	perfectAccepted := perfect.Accepted && perfect.Cases[0].TreatmentPassRate == 1
	printCheck("perfect treatment reliability clears study acceptance", perfectAccepted)
	if !perfectAccepted {
		return errors.New("perfect treatment reliability did not meet study acceptance")
	}
	hashesDiffer := manifest.TreatmentPluginHash != manifest.EmptyControlPluginHash && manifest.EmptyControlPluginHash == emptyControlPluginHash() && inventoriesCorrect(manifest)
	printCheck("treatment and empty-control hashes differ", hashesDiffer)
	if !hashesDiffer {
		return errors.New("treatment and control plugin evidence is ambiguous")
	}
	schemaOK := strictScore(root, filepath.Join(out, "publish", "results.jsonl")) == nil
	printCheck("schema rows pass strict scorer", schemaOK)
	if !schemaOK {
		return errors.New("selftest rows failed strict scorer")
	}
	leftovers, _ := filepath.Glob(filepath.Join(parent, "megapowers-installed-ab-*"))
	cleanupSuccess := len(leftovers) == 0
	printCheck("temporary state removed after success", cleanupSuccess)
	if !cleanupSuccess {
		return errors.New("success path leaked temporary state")
	}

	failing := &fakeActor{FailArm: "control"}
	failOut := filepath.Join(parent, "failure")
	failOpts := opts
	failOpts.Out = failOut
	failedRows, _, failErr := executeStudy(context.Background(), casesFile{SchemaVersion: "1", Cases: cases.Cases[:1]}, gates, failOpts, failing)
	leftovers, _ = filepath.Glob(filepath.Join(parent, "megapowers-installed-ab-*"))
	cleanupFailure := len(leftovers) == 0
	printCheck("temporary state removed after actor failure", cleanupFailure)
	failClosed := failErr != nil && len(failedRows) >= 1 && failedRows[len(failedRows)-1].Status == "harness_error" && failedRows[len(failedRows)-1].Verdict == "harness_error"
	printCheck("actor errors fail closed", failClosed)
	if !cleanupFailure || !failClosed {
		return errors.New("failure path did not clean up and fail closed")
	}

	timeoutOut := filepath.Join(parent, "timeout")
	timeoutOpts := opts
	timeoutOpts.Out = timeoutOut
	timeoutOpts.ActorTimeout = time.Millisecond
	timedRows, _, timeoutErr := executeStudy(context.Background(), casesFile{SchemaVersion: "1", Cases: cases.Cases[:1]}, gates, timeoutOpts, blockingActor{})
	leftovers, _ = filepath.Glob(filepath.Join(parent, "megapowers-installed-ab-*"))
	deadlineClosed := timeoutErr != nil && len(timedRows) == 1 && timedRows[0].Status == "timeout" && timedRows[0].Verdict == "harness_error" && timedRows[0].RC == 124 && len(leftovers) == 0
	printCheck("actor deadlines fail closed", deadlineClosed)
	if !deadlineClosed {
		return errors.New("actor deadline did not fail closed and clean up")
	}

	sanitized := publishFilesOnly(out) && publishFilesOnly(failOut) && publishContainsNo(parent, []string{"credentials", "actor-final", "transcript", "prompt.txt", "home"})
	printCheck("publish bundle contains sanitized files only", sanitized)
	if !sanitized {
		return errors.New("publish bundle contains non-sanitized files")
	}
	fmt.Println("installed-ab selftest: PASS")
	return nil
}

func unique(values []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func pairedHashesMatch(rows []resultRow) bool {
	byBlock := map[string][]resultRow{}
	for _, row := range rows {
		byBlock[row.BlockID] = append(byBlock[row.BlockID], row)
	}
	for _, pair := range byBlock {
		if len(pair) != 2 || pair[0].PromptHash != pair[1].PromptHash || pair[0].FixtureHash != pair[1].FixtureHash {
			return false
		}
	}
	return len(byBlock) > 0
}

func metricFor(rows []resultRow, caseID, arm, metric string) float64 {
	for _, row := range rows {
		if row.CaseID == caseID && row.Arm == arm {
			return row.Metrics[metric]
		}
	}
	return -1
}

func syntheticGateRows(treatmentPasses, controlPasses, total int) []resultRow {
	rows := make([]resultRow, 0, total*2)
	for i := 0; i < total; i++ {
		treatmentVerdict := "fail"
		if i < treatmentPasses {
			treatmentVerdict = "pass"
		}
		controlVerdict := "fail"
		if i < controlPasses {
			controlVerdict = "pass"
		}
		rows = append(rows,
			resultRow{CaseID: "synthetic", Arm: "treatment", Status: "completed", Verdict: treatmentVerdict},
			resultRow{CaseID: "synthetic", Arm: "control", Status: "completed", Verdict: controlVerdict},
		)
	}
	return rows
}

func inventoriesCorrect(manifest publishManifest) bool {
	for _, arm := range manifest.Arms {
		if arm.Arm == "control" && len(arm.PluginNames) != 0 {
			return false
		}
		if arm.Arm == "treatment" && (len(arm.PluginNames) != 1 || arm.PluginNames[0] != "megapowers") {
			return false
		}
	}
	return len(manifest.Arms) > 0
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

func publishContainsNo(root string, banned []string) bool {
	for _, out := range []string{filepath.Join(root, "success", "publish"), filepath.Join(root, "failure", "publish")} {
		err := filepath.WalkDir(out, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			lower := strings.ToLower(string(content))
			for _, value := range banned {
				if strings.Contains(lower, strings.ToLower(value)) {
					return fmt.Errorf("banned publish content %q", value)
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
