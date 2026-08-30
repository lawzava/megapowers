// Command run executes the trigger-recall activation study.
//
// It measures, per shipped skill and harness, whether the installed plugin's
// trigger surface causes correct activation (recall) and correct
// non-activation (precision) on the fixed probe corpus in cases.json. Every
// row is single-arm: the plugin is always installed. Task quality is out of
// scope; the verdict comes only from trace-proven skill selection events.
//
// Credential-free mechanics:
//
//	go run evals/studies/trigger-recall/run.go --selftest
//	go run evals/studies/trigger-recall/run.go --validate-config \
//	  --cases evals/studies/trigger-recall/cases.json \
//	  --gates evals/studies/trigger-recall/gates.json
//
// Explicit real run (maintainer-local, never CI):
//
//	go run evals/studies/trigger-recall/run.go --run --credentialed \
//	  --harness claude --model <exact-model> --effort <exact-effort> \
//	  --sandbox-broker /absolute/path/to/reviewed-broker \
//	  --broker-sha256 "$BROKER_SHA256" --reps 3 \
//	  --out results/trigger-recall-claude
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
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	studyName               = "trigger-recall"
	captureLimit            = 10 << 20
	captureTruncationNotice = "\n[megapowers: subprocess output truncated at the 10485760-byte capture limit]\n"
	subprocessWaitDelay     = 5 * time.Second
	defaultActorTimeout     = 5 * time.Minute
)

var validKinds = map[string]bool{"verbatim": true, "paraphrase": true, "buried": true, "near-miss": true, "no-skill": true}

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$`)

type probeCase struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Prompt     string            `json:"prompt"`
	Files      map[string]string `json:"files,omitempty"`
	Expected   string            `json:"expected,omitempty"`
	Allowed    []string          `json:"allowed,omitempty"`
	Provenance string            `json:"provenance"`
}

type casesFile struct {
	SchemaVersion string      `json:"schema_version"`
	Cases         []probeCase `json:"cases"`
}

type gatesAcceptance struct {
	MinimumReps                  int     `json:"minimum_reps"`
	DefaultMinRecall             float64 `json:"default_min_recall"`
	DefaultMaxFalseSelectionRate float64 `json:"default_max_false_selection_rate"`
	PerSkill                     map[string]struct {
		MinRecall float64 `json:"min_recall"`
	} `json:"per_skill"`
}

type gatesFile struct {
	SchemaVersion string          `json:"schema_version"`
	Mode          string          `json:"mode"`
	Acceptance    gatesAcceptance `json:"acceptance"`
}

type skillCatalog struct {
	SchemaVersion string `json:"schema_version"`
	Skills        []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"skills"`
}

type actorEvent struct {
	Kind string `json:"kind"`
	Path string `json:"path,omitempty"`
	RC   int    `json:"rc,omitempty"`
	Step int    `json:"step"`
}

type actorRequest struct {
	Case       probeCase
	Harness    string
	Model      string
	Effort     string
	Home       string
	Project    string
	PluginRoot string
	Timeout    time.Duration
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

type caseTally struct {
	CaseID   string `json:"case_id"`
	Expected string `json:"expected,omitempty"`
	Pass     int    `json:"pass"`
	Total    int    `json:"total"`
}

type gateAssessment struct {
	Mode        string      `json:"mode"`
	MinimumReps int         `json:"minimum_reps"`
	Violations  []string    `json:"violations"`
	Cases       []caseTally `json:"cases"`
}

type runEnvironment struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Locale string `json:"locale"`
}

type publishManifest struct {
	SchemaVersion   string         `json:"schema_version"`
	Study           string         `json:"study"`
	Evidence        string         `json:"evidence"`
	Harness         string         `json:"harness"`
	Model           string         `json:"model"`
	Effort          string         `json:"effort"`
	Reps            int            `json:"reps"`
	Environment     runEnvironment `json:"environment"`
	BrokerHash      string         `json:"broker_hash"`
	PluginHash      string         `json:"plugin_hash"`
	CaseCatalogHash string         `json:"case_catalog_hash"`
	GatesHash       string         `json:"gates_hash"`
	Retries         int            `json:"retries"`
	Assessment      gateAssessment `json:"assessment"`
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
	Reps         int
	ActorTimeout time.Duration
}

func main() {
	flags := flag.NewFlagSet("trigger-recall", flag.ContinueOnError)
	selftest := flags.Bool("selftest", false, "run credential-free mechanics checks")
	validateConfig := flags.Bool("validate-config", false, "validate the corpus and gates, then exit")
	run := flags.Bool("run", false, "execute the study")
	credentialed := flags.Bool("credentialed", false, "explicitly allow a real harness run")
	casesPath := flags.String("cases", "", "probe corpus path")
	gatesPath := flags.String("gates", "", "gates path")
	harness := flags.String("harness", "", "harness name (claude or codex)")
	model := flags.String("model", "", "exact model identifier")
	effort := flags.String("effort", "high", "exact effort value")
	broker := flags.String("sandbox-broker", "", "absolute path to the reviewed isolation broker")
	brokerHash := flags.String("broker-sha256", "", "sha256:<hex> pin of the reviewed broker")
	reps := flags.Int("reps", 1, "repetitions per probe")
	filter := flags.String("filter", "", "run only case ids containing this substring")
	actorTimeout := flags.Duration("actor-timeout", defaultActorTimeout, "per-probe actor deadline")
	out := flags.String("out", "", "output directory for the publish bundle")
	repo := flags.String("repo", "", "repository root (default: ascend from the working directory)")
	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	if *selftest {
		if err := runSelftest(); err != nil {
			fmt.Println("trigger-recall selftest: FAIL")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("trigger-recall selftest: PASS")
		return
	}

	root, err := locateRoot(*repo)
	if err != nil {
		fatal(err)
	}
	catalog, err := loadCatalog(root)
	if err != nil {
		fatal(err)
	}
	cases, gates, err := loadConfiguration(*casesPath, *gatesPath)
	if err != nil {
		fatal(err)
	}
	if err := validateConfiguration(cases, gates, catalog); err != nil {
		fatal(err)
	}
	if *validateConfig {
		fmt.Println("trigger-recall configuration: valid")
		return
	}
	if !*run {
		fatal(errors.New("choose --selftest, --validate-config, or --run"))
	}
	if !*credentialed {
		fatal(errors.New("--run requires explicit --credentialed acknowledgement"))
	}
	if *harness != "claude" && *harness != "codex" {
		fatal(errors.New("--harness must be claude or codex"))
	}
	if *model == "" || *effort == "" {
		fatal(errors.New("--model and --effort must be exact identities"))
	}
	if *out == "" {
		fatal(errors.New("--run requires --out"))
	}
	if *reps < 1 {
		fatal(errors.New("--reps must be at least 1"))
	}
	verifiedBrokerHash, err := validateBroker(*broker, *brokerHash, root, *out)
	if err != nil {
		fatal(err)
	}
	selected := filterCases(cases, *filter)
	if len(selected.Cases) == 0 {
		fatal(fmt.Errorf("--filter %q matches no cases", *filter))
	}
	options := runOptions{Harness: *harness, Model: *model, Effort: *effort, Repo: root, Out: *out, Timestamp: time.Now().UTC(), Reps: *reps, ActorTimeout: *actorTimeout}
	subject := brokerActor{Path: *broker, ExpectedHash: verifiedBrokerHash}
	_, manifest, err := executeStudy(context.Background(), selected, gates, catalog, options, subject)
	if err != nil {
		fatal(err)
	}
	if gates.Mode == "enforce" && len(manifest.Assessment.Violations) > 0 {
		fatal(fmt.Errorf("gate violations: %s", strings.Join(manifest.Assessment.Violations, "; ")))
	}
	for _, violation := range manifest.Assessment.Violations {
		fmt.Fprintf(os.Stderr, "report-only violation: %s\n", violation)
	}
	fmt.Printf("trigger-recall run complete: %d cases x %d reps published to %s\n", len(selected.Cases), *reps, filepath.Join(*out, "publish"))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "trigger-recall:", err)
	os.Exit(1)
}

func locateRoot(explicit string) (string, error) {
	start := explicit
	if start == "" {
		working, err := os.Getwd()
		if err != nil {
			return "", err
		}
		start = working
	}
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(current, "plugins", "megapowers", "skills", "catalog.json")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("repository root with plugins/megapowers not found")
		}
		current = parent
	}
}

func decodeStrict(path string, target any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("%s: trailing data", path)
	}
	return nil
}

func loadCatalog(root string) (map[string]bool, error) {
	var catalog skillCatalog
	if err := decodeStrict(filepath.Join(root, "plugins", "megapowers", "skills", "catalog.json"), &catalog); err != nil {
		return nil, err
	}
	if catalog.SchemaVersion != "1" || len(catalog.Skills) == 0 {
		return nil, errors.New("skill catalog is unsupported or empty")
	}
	names := make(map[string]bool, len(catalog.Skills))
	for _, skill := range catalog.Skills {
		names[skill.Name] = true
	}
	return names, nil
}

func loadConfiguration(casesPath, gatesPath string) (casesFile, gatesFile, error) {
	var cases casesFile
	var gates gatesFile
	if casesPath == "" || gatesPath == "" {
		return cases, gates, errors.New("--cases and --gates are required")
	}
	if err := decodeStrict(casesPath, &cases); err != nil {
		return cases, gates, err
	}
	if err := decodeStrict(gatesPath, &gates); err != nil {
		return cases, gates, err
	}
	return cases, gates, nil
}

func validateCases(cases casesFile, catalog map[string]bool) error {
	if cases.SchemaVersion != "1" {
		return errors.New("cases schema_version is unsupported")
	}
	if len(cases.Cases) == 0 {
		return errors.New("the corpus is empty")
	}
	seen := make(map[string]bool)
	for _, c := range cases.Cases {
		if !identifierPattern.MatchString(c.ID) {
			return fmt.Errorf("case id %q is not portable", c.ID)
		}
		if seen[c.ID] {
			return fmt.Errorf("case id %q is duplicated", c.ID)
		}
		seen[c.ID] = true
		if !validKinds[c.Kind] {
			return fmt.Errorf("case %q kind %q is unknown", c.ID, c.Kind)
		}
		if strings.TrimSpace(c.Prompt) == "" {
			return fmt.Errorf("case %q has an empty prompt", c.ID)
		}
		if strings.TrimSpace(c.Provenance) == "" {
			return fmt.Errorf("case %q has no provenance", c.ID)
		}
		if c.Expected != "" && !catalog[c.Expected] {
			return fmt.Errorf("case %q expects unknown skill %q", c.ID, c.Expected)
		}
		if c.Kind == "no-skill" && c.Expected != "" {
			return fmt.Errorf("no-skill case %q must not expect a selection", c.ID)
		}
		for _, allowed := range c.Allowed {
			if !catalog[allowed] {
				return fmt.Errorf("case %q allows unknown skill %q", c.ID, allowed)
			}
			if allowed == c.Expected {
				return fmt.Errorf("case %q lists its expected skill as allowed", c.ID)
			}
		}
		for name := range c.Files {
			if err := safeRelative(name); err != nil {
				return fmt.Errorf("case %q: %w", c.ID, err)
			}
		}
	}
	return nil
}

func validateConfiguration(cases casesFile, gates gatesFile, catalog map[string]bool) error {
	if err := validateCases(cases, catalog); err != nil {
		return err
	}
	recallProbes := make(map[string]int)
	noSkillProbes := 0
	for _, c := range cases.Cases {
		if c.Expected != "" {
			recallProbes[c.Expected]++
		}
		if c.Kind == "no-skill" {
			noSkillProbes++
		}
	}
	for skill := range catalog {
		if recallProbes[skill] < 3 {
			return fmt.Errorf("skill %q has %d recall probes; require at least 3", skill, recallProbes[skill])
		}
	}
	if noSkillProbes < 10 {
		return fmt.Errorf("the no-skill pool has %d probes; require at least 10", noSkillProbes)
	}
	if gates.SchemaVersion != "1" {
		return errors.New("gates schema_version is unsupported")
	}
	if gates.Mode != "report-only" && gates.Mode != "enforce" {
		return fmt.Errorf("gates mode %q must be report-only or enforce", gates.Mode)
	}
	if gates.Acceptance.MinimumReps < 1 {
		return errors.New("gates minimum_reps must be at least 1")
	}
	for name, rate := range map[string]float64{
		"default_min_recall":               gates.Acceptance.DefaultMinRecall,
		"default_max_false_selection_rate": gates.Acceptance.DefaultMaxFalseSelectionRate,
	} {
		if rate < 0 || rate > 1 {
			return fmt.Errorf("gates %s must be within [0,1]", name)
		}
	}
	for skill, override := range gates.Acceptance.PerSkill {
		if !catalog[skill] {
			return fmt.Errorf("gates override names unknown skill %q", skill)
		}
		if override.MinRecall < 0 || override.MinRecall > 1 {
			return fmt.Errorf("gates override for %q must be within [0,1]", skill)
		}
	}
	return nil
}

func filterCases(cases casesFile, filter string) casesFile {
	if filter == "" {
		return cases
	}
	selected := casesFile{SchemaVersion: cases.SchemaVersion}
	for _, c := range cases.Cases {
		if strings.Contains(c.ID, filter) {
			selected.Cases = append(selected.Cases, c)
		}
	}
	return selected
}

func evaluateProbe(c probeCase, events []actorEvent, catalog map[string]bool) (map[string]float64, string) {
	allowed := map[string]bool{}
	for _, name := range c.Allowed {
		allowed[name] = true
	}
	expectedSelected := false
	unexpected := 0
	attempts := 0
	for _, event := range events {
		if event.Kind != "skill_selected" || event.Path == "" {
			continue
		}
		attempts++
		if event.Path == c.Expected {
			if event.RC == 0 {
				expectedSelected = true
			}
			continue
		}
		// Only shipped skills count against precision; harness-native or
		// third-party skills are outside this study's contract.
		if catalog[event.Path] && !allowed[event.Path] {
			unexpected++
		}
	}
	pass := unexpected == 0 && (c.Expected == "" || expectedSelected)
	verdict := "fail"
	if pass {
		verdict = "pass"
	}
	metrics := map[string]float64{
		"activation_success":    boolMetric(pass),
		"expected_selected":     boolMetric(expectedSelected),
		"unexpected_selections": float64(unexpected),
		"selection_attempts":    float64(attempts),
	}
	return metrics, verdict
}

func assessGates(tallies []caseTally, gates gatesFile, reps int) gateAssessment {
	assessment := gateAssessment{Mode: gates.Mode, MinimumReps: gates.Acceptance.MinimumReps, Violations: []string{}, Cases: tallies}
	if reps < gates.Acceptance.MinimumReps {
		assessment.Violations = append(assessment.Violations, fmt.Sprintf("%d reps below the %d minimum", reps, gates.Acceptance.MinimumReps))
	}
	for _, tally := range tallies {
		if tally.Total == 0 {
			continue
		}
		rate := float64(tally.Pass) / float64(tally.Total)
		if tally.Expected != "" {
			minRecall := gates.Acceptance.DefaultMinRecall
			if override, exists := gates.Acceptance.PerSkill[tally.Expected]; exists {
				minRecall = override.MinRecall
			}
			if rate < minRecall {
				assessment.Violations = append(assessment.Violations, fmt.Sprintf("case %s recall %d/%d below %.2f", tally.CaseID, tally.Pass, tally.Total, minRecall))
			}
			continue
		}
		if falseRate := 1 - rate; falseRate > gates.Acceptance.DefaultMaxFalseSelectionRate {
			assessment.Violations = append(assessment.Violations, fmt.Sprintf("case %s false selections %d/%d above %.2f", tally.CaseID, tally.Total-tally.Pass, tally.Total, gates.Acceptance.DefaultMaxFalseSelectionRate))
		}
	}
	return assessment
}

func executeStudy(ctx context.Context, cases casesFile, gates gatesFile, catalog map[string]bool, opts runOptions, subject actor) ([]resultRow, publishManifest, error) {
	if err := validateCases(cases, catalog); err != nil {
		return nil, publishManifest{}, err
	}
	parent := opts.TempRoot
	if parent == "" {
		parent = os.Getenv("TMPDIR")
	}
	if parent == "" {
		parent = os.TempDir()
	}
	oldUmask := setUmask077()
	root, err := os.MkdirTemp(parent, "megapowers-trigger-recall-*")
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
		actorPluginRoot = filepath.Join(root, "verified-plugin")
		pluginHash, err = stagePluginTree(filepath.Join(opts.Repo, "plugins", "megapowers"), actorPluginRoot)
		if err != nil {
			return nil, publishManifest{}, err
		}
	}
	caseCatalogHash, gatesHash, err := configurationHashes(cases, gates)
	if err != nil {
		return nil, publishManifest{}, err
	}
	cliVersion := "selftest"
	retries := 0
	rows := make([]resultRow, 0, len(cases.Cases)*opts.Reps)
	tallies := make([]caseTally, 0, len(cases.Cases))
	for _, c := range cases.Cases {
		tally := caseTally{CaseID: c.ID, Expected: c.Expected}
		for rep := 1; rep <= opts.Reps; rep++ {
			blockID := fmt.Sprintf("rep-%d", rep)
			var result actorResult
			var actorErr error
			var timedOut bool
			// One automatic retry absorbs a transient actor failure; the
			// broker requires an empty disposable home, so every attempt
			// gets fresh directories. A second failure still fails closed.
			for attempt := 1; attempt <= 2; attempt++ {
				armRoot := filepath.Join(root, c.ID, fmt.Sprintf("%s-attempt-%d", blockID, attempt))
				home := filepath.Join(armRoot, "home")
				project := filepath.Join(armRoot, "project")
				for _, directory := range []string{home, project} {
					if err := os.MkdirAll(directory, 0o700); err != nil {
						return rows, publishManifest{}, err
					}
				}
				if err := materializeFixture(project, c.Files); err != nil {
					return rows, publishManifest{}, err
				}
				request := actorRequest{Case: c, Harness: opts.Harness, Model: opts.Model, Effort: opts.Effort, Home: home, Project: project, PluginRoot: actorPluginRoot, Timeout: opts.ActorTimeout}
				actorCtx, cancelActor := context.WithTimeout(ctx, opts.ActorTimeout)
				result, actorErr = subject.Run(actorCtx, request)
				timedOut = errors.Is(actorCtx.Err(), context.DeadlineExceeded)
				cancelActor()
				if result.CLIVersion != "" {
					cliVersion = result.CLIVersion
				}
				if actorErr == nil && !timedOut && result.RC == 0 {
					break
				}
				if attempt == 1 {
					retries++
				}
			}
			if timedOut || actorErr != nil || result.RC != 0 {
				if err := writeFailureEvidence(opts.Out, c.ID, blockID, result, actorErr); err != nil {
					return rows, publishManifest{}, err
				}
			}
			if timedOut {
				return rows, publishManifest{}, fmt.Errorf("%s/%s actor timed out after %s", c.ID, blockID, opts.ActorTimeout)
			}
			if actorErr != nil {
				return rows, publishManifest{}, fmt.Errorf("%s/%s actor error: %w", c.ID, blockID, actorErr)
			}
			if result.RC != 0 {
				return rows, publishManifest{}, fmt.Errorf("%s/%s actor exited %d", c.ID, blockID, result.RC)
			}
			metrics, verdict := evaluateProbe(c, result.Events, catalog)
			sandbox := "selftest"
			if result.Sandbox != "" {
				sandbox = portableIdentifier(result.Sandbox)
			}
			row := resultRow{
				SchemaVersion: "1",
				Study:         studyName,
				EvidenceClass: "activation",
				CaseID:        c.ID,
				RunID:         fmt.Sprintf("%s-%s", c.ID, blockID),
				BlockID:       blockID,
				Arm:           "treatment",
				Harness:       harnessIdentity{Name: opts.Harness, CLIVersion: portableIdentifier(cliVersion), Model: opts.Model, Effort: opts.Effort},
				Source:        sourceIdentity{Repository: "megapowers", Revision: revision},
				PromptHash:    hashBytes([]byte(c.Prompt)),
				FixtureHash:   hashFixture(c.Files),
				PluginHash:    pluginHash,
				Status:        "completed",
				RC:            result.RC,
				DurationMS:    maxInt64(result.Duration.Milliseconds(), 0),
				Verdict:       verdict,
				Metrics:       metrics,
				Artifacts:     map[string]string{"response": hashBytes([]byte(result.Response)), "trace": hashBytes(result.Trace)},
				Environment:   environment{OS: runtime.GOOS, Arch: runtime.GOARCH, Sandbox: sandbox, Locale: portableIdentifier(locale())},
				Timestamp:     opts.Timestamp.Format(time.RFC3339),
			}
			rows = append(rows, row)
			tally.Total++
			if verdict == "pass" {
				tally.Pass++
			}
		}
		tallies = append(tallies, tally)
	}
	evidence := "credentialed"
	if opts.Selftest {
		evidence = "selftest-mechanics-only"
	}
	manifest := publishManifest{
		SchemaVersion:   "1",
		Study:           studyName,
		Evidence:        evidence,
		Harness:         opts.Harness,
		Model:           opts.Model,
		Effort:          opts.Effort,
		Reps:            opts.Reps,
		Environment:     runEnvironment{OS: runtime.GOOS, Arch: runtime.GOARCH, Locale: portableIdentifier(locale())},
		BrokerHash:      hashBytes([]byte("in-process-selftest-fake")),
		PluginHash:      pluginHash,
		CaseCatalogHash: caseCatalogHash,
		GatesHash:       gatesHash,
		Retries:         retries,
		Assessment:      assessGates(tallies, gates, opts.Reps),
	}
	if err := writePublish(opts.Out, rows, manifest); err != nil {
		return rows, manifest, err
	}
	return rows, manifest, nil
}

// writeFailureEvidence keeps the failed attempt's trace and error privately
// under <out>/failures/, outside publish/. It exists for maintainer
// diagnosis of systematic actor failures; failure evidence is never part of
// the sanitized publish bundle.
func writeFailureEvidence(out, caseID, blockID string, result actorResult, actorErr error) error {
	if out == "" {
		return nil
	}
	failures := filepath.Join(out, "failures")
	if err := os.MkdirAll(failures, 0o700); err != nil {
		return err
	}
	message := ""
	if actorErr != nil {
		message = actorErr.Error()
	}
	evidence, err := json.MarshalIndent(map[string]any{
		"case":     caseID,
		"block":    blockID,
		"rc":       result.RC,
		"error":    message,
		"response": result.Response,
		"trace":    string(result.Trace),
	}, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(failures, fmt.Sprintf("%s-%s.json", caseID, blockID)), append(evidence, '\n'), 0o600)
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

// stagePluginTree copies the shipped plugin into a private read-only location
// and returns a deterministic content hash of the staged bytes.
func stagePluginTree(source, destination string) (string, error) {
	type stagedFile struct {
		name    string
		content []byte
	}
	files := make([]stagedFile, 0)
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("plugin tree contains symlink %q", path)
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, stagedFile{name: filepath.ToSlash(relative), content: content})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	hasher := sha256.New()
	for _, file := range files {
		fmt.Fprintf(hasher, "%d:%s:%d:", len(file.name), file.name, len(file.content))
		hasher.Write(file.content)
		target := filepath.Join(destination, filepath.FromSlash(file.name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return "", err
		}
		if err := os.WriteFile(target, file.content, 0o600); err != nil {
			return "", err
		}
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func safeRelative(name string) error {
	clean := filepath.Clean(name)
	if name == "" || filepath.IsAbs(name) || clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return fmt.Errorf("unsafe fixture path %q", name)
	}
	return nil
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
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
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

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// syscall.Umask is isolated to these tiny helpers so all created private
// state starts with owner-only permissions.
func setUmask077() int       { return syscall.Umask(0o077) }
func restoreUmask(value int) { syscall.Umask(value) }

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
	return append(b.buffer.Bytes(), captureTruncationNotice...)
}

func isolateChild(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	command.WaitDelay = subprocessWaitDelay
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
		return "", errors.New("--broker-sha256 does not match the separately reviewed broker")
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

type brokerActor struct {
	Path         string
	ExpectedHash string
}

type brokerRequest struct {
	SchemaVersion  string   `json:"schema_version"`
	Harness        string   `json:"harness"`
	Model          string   `json:"model"`
	Effort         string   `json:"effort"`
	Arm            string   `json:"arm"`
	Task           string   `json:"task"`
	Project        string   `json:"project"`
	ActorHome      string   `json:"actor_home"`
	PluginRepo     string   `json:"plugin_repo,omitempty"`
	TaskReadRoots  []string `json:"task_read_roots"`
	TaskWriteRoots []string `json:"task_write_roots"`
	OracleCommand  []string `json:"oracle_command,omitempty"`
	TimeoutMS      int64    `json:"timeout_ms"`
}

type isolationAttestation struct {
	Boundary                    string   `json:"boundary"`
	CredentialsReadableByActor  *bool    `json:"credentials_readable_by_actor"`
	SiblingStateReadableByActor *bool    `json:"sibling_state_readable_by_actor"`
	TaskReadRoots               []string `json:"task_read_roots"`
	TaskWriteRoots              []string `json:"task_write_roots"`
	ActorHome                   string   `json:"actor_home"`
}

type brokerResponse struct {
	SchemaVersion   string               `json:"schema_version"`
	CLIVersion      string               `json:"cli_version"`
	Response        string               `json:"response"`
	Trace           string               `json:"trace"`
	Events          []actorEvent         `json:"events"`
	PluginInventory []string             `json:"plugin_inventory"`
	OracleRC        *int                 `json:"oracle_rc,omitempty"`
	RC              int                  `json:"rc"`
	DurationMS      int64                `json:"duration_ms"`
	Isolation       isolationAttestation `json:"isolation"`
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

func makeBrokerRequest(request actorRequest) (brokerRequest, []string) {
	roots := []string{request.Project, request.PluginRoot}
	brokerTimeout := request.Timeout - 5*time.Second
	if brokerTimeout <= 0 {
		brokerTimeout = request.Timeout / 2
	}
	return brokerRequest{SchemaVersion: "2", Harness: request.Harness, Model: request.Model, Effort: request.Effort, Arm: "treatment", Task: request.Case.Prompt, Project: request.Project, ActorHome: request.Home, PluginRepo: request.PluginRoot, TaskReadRoots: roots, TaskWriteRoots: []string{request.Project}, TimeoutMS: brokerTimeout.Milliseconds()}, roots
}

func validateIsolation(response brokerResponse, expectedRoots []string, expectedHome string) error {
	if response.SchemaVersion != "2" || response.CLIVersion == "" {
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
	if expectedHome == "" || filepath.Clean(response.Isolation.ActorHome) != filepath.Clean(expectedHome) {
		return errors.New("sandbox broker did not attest the disposable actor home")
	}
	if len(response.PluginInventory) != 1 || response.PluginInventory[0] != "megapowers" {
		return errors.New("broker inventory is not exactly megapowers")
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
	command := exec.CommandContext(ctx, stagedBroker)
	command.Dir = filepath.Dir(stagedBroker)
	isolateChild(command)
	command.Stdin = bytes.NewReader(input)
	var stdout, stderr boundedOutput
	stdout.limit = captureLimit
	stderr.limit = captureLimit
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return actorResult{RC: 125}, fmt.Errorf("sandbox broker failed: %w: %s", err, strings.TrimSpace(string(stderr.Bytes())))
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
	if err := validateIsolation(response, roots, request.Home); err != nil {
		return actorResult{RC: 125}, err
	}
	if response.OracleRC != nil {
		return actorResult{RC: 125}, errors.New("sandbox broker returned an unexpected oracle result")
	}
	return actorResult{Response: response.Response, Trace: []byte(response.Trace), Events: response.Events, Inventory: response.PluginInventory, CLIVersion: response.CLIVersion, Sandbox: response.Isolation.Boundary, RC: response.RC, Duration: time.Duration(response.DurationMS) * time.Millisecond}, nil
}

// --- selftest ---

type scriptedActor struct {
	events   map[string][]actorEvent
	fail     map[string]error
	failOnce map[string]error
	rc       map[string]int
	calls    map[string]int
}

func (s scriptedActor) Run(_ context.Context, request actorRequest) (actorResult, error) {
	if s.calls != nil {
		s.calls[request.Case.ID]++
	}
	if err := s.failOnce[request.Case.ID]; err != nil {
		delete(s.failOnce, request.Case.ID)
		return actorResult{RC: 125}, err
	}
	if err := s.fail[request.Case.ID]; err != nil {
		return actorResult{RC: 125}, err
	}
	return actorResult{
		Response:   "done",
		Trace:      []byte("trace"),
		Events:     s.events[request.Case.ID],
		CLIVersion: "selftest",
		Sandbox:    "selftest",
		RC:         s.rc[request.Case.ID],
	}, nil
}

type blockingActor struct{}

func (blockingActor) Run(ctx context.Context, _ actorRequest) (actorResult, error) {
	<-ctx.Done()
	return actorResult{RC: 124}, ctx.Err()
}

func runSelftest() error {
	failures := 0
	check := func(description string, ok bool) {
		if ok {
			fmt.Println("ok  ", description)
		} else {
			fmt.Println("FAIL", description)
			failures++
		}
	}
	root, err := locateRoot("")
	if err != nil {
		return err
	}
	catalog, err := loadCatalog(root)
	if err != nil {
		return err
	}
	gates := gatesFile{SchemaVersion: "1", Mode: "report-only", Acceptance: gatesAcceptance{MinimumReps: 1, DefaultMinRecall: 0.67, DefaultMaxFalseSelectionRate: 0.34}}
	sentinel := "SENTINEL-PROMPT-DO-NOT-PUBLISH"
	corpus := casesFile{SchemaVersion: "1", Cases: []probeCase{
		{ID: "recall-hit", Kind: "verbatim", Prompt: sentinel + " hit", Expected: "systematic-debugging", Provenance: "selftest"},
		{ID: "recall-miss", Kind: "paraphrase", Prompt: sentinel + " miss", Expected: "systematic-debugging", Provenance: "selftest"},
		{ID: "recall-failed-attempt", Kind: "buried", Prompt: sentinel + " failed", Expected: "systematic-debugging", Provenance: "selftest"},
		{ID: "precision-unexpected", Kind: "near-miss", Prompt: sentinel + " unexpected", Provenance: "selftest"},
		{ID: "precision-failed-attempt", Kind: "near-miss", Prompt: sentinel + " failed-attempt", Provenance: "selftest"},
		{ID: "allowed-co-select", Kind: "verbatim", Prompt: sentinel + " allowed", Expected: "systematic-debugging", Allowed: []string{"verify-and-finish"}, Provenance: "selftest"},
		{ID: "no-skill-clean", Kind: "no-skill", Prompt: sentinel + " clean", Provenance: "selftest"},
	}}
	scripted := scriptedActor{
		events: map[string][]actorEvent{
			"recall-hit":               {{Kind: "skill_selected", Path: "systematic-debugging", RC: 0, Step: 1}},
			"recall-miss":              {},
			"recall-failed-attempt":    {{Kind: "skill_selected", Path: "systematic-debugging", RC: 1, Step: 1}},
			"precision-unexpected":     {{Kind: "skill_selected", Path: "orchestrating", RC: 0, Step: 1}},
			"precision-failed-attempt": {{Kind: "skill_selected", Path: "orchestrating", RC: 1, Step: 1}},
			"allowed-co-select": {
				{Kind: "skill_selected", Path: "systematic-debugging", RC: 0, Step: 1},
				{Kind: "skill_selected", Path: "verify-and-finish", RC: 0, Step: 2},
			},
			"no-skill-clean": {{Kind: "skill_selected", Path: "random-helper", RC: 0, Step: 1}},
		},
		fail: map[string]error{},
		rc:   map[string]int{},
	}
	tempParent, err := os.MkdirTemp("", "trigger-recall-selftest-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempParent)
	outSuccess := filepath.Join(tempParent, "success")
	options := runOptions{Harness: "claude", Model: "selftest-model", Effort: "high", Repo: root, Out: outSuccess, TempRoot: tempParent, Timestamp: time.Now().UTC(), Selftest: true, Reps: 1, ActorTimeout: time.Minute}
	rows, manifest, err := executeStudy(context.Background(), corpus, gates, catalog, options, scripted)
	if err != nil {
		return err
	}
	verdicts := map[string]string{}
	metricsByCase := map[string]map[string]float64{}
	for _, row := range rows {
		verdicts[row.CaseID] = row.Verdict
		metricsByCase[row.CaseID] = row.Metrics
	}
	check("recall accepts a successful expected selection", verdicts["recall-hit"] == "pass")
	check("recall rejects a missing expected selection", verdicts["recall-miss"] == "fail")
	check("recall rejects a failed expected selection attempt", verdicts["recall-failed-attempt"] == "fail")
	check("precision rejects an unexpected skill selection", verdicts["precision-unexpected"] == "fail")
	check("precision counts failed unexpected attempts", verdicts["precision-failed-attempt"] == "fail" && metricsByCase["precision-failed-attempt"]["unexpected_selections"] == 1)
	check("precision ignores allowed co-selection", verdicts["allowed-co-select"] == "pass")
	check("no-skill probes reject any megapowers selection", verdicts["precision-unexpected"] == "fail" && metricsByCase["precision-unexpected"]["unexpected_selections"] == 1)
	check("non-catalog selections stay out of precision", verdicts["no-skill-clean"] == "pass")
	binaryOK := true
	for _, row := range rows {
		success := row.Metrics["activation_success"]
		if (success != 0 && success != 1) || success != boolMetric(row.Verdict == "pass") {
			binaryOK = false
		}
	}
	check("rows carry binary activation_success", binaryOK)
	check("rows pass the strict scorer", strictScore(root, filepath.Join(outSuccess, "publish", "results.jsonl")) == nil)

	outReps := filepath.Join(tempParent, "reps")
	repOptions := options
	repOptions.Out = outReps
	repOptions.Reps = 2
	repRows, _, err := executeStudy(context.Background(), casesFile{SchemaVersion: "1", Cases: corpus.Cases[:1]}, gates, catalog, repOptions, scripted)
	blocks := map[string]bool{}
	uniqueBlocks := err == nil && len(repRows) == 2
	for _, row := range repRows {
		if blocks[row.BlockID] {
			uniqueBlocks = false
		}
		blocks[row.BlockID] = true
	}
	check("reps emit unique blocks", uniqueBlocks)

	leftover, err := filepath.Glob(filepath.Join(tempParent, "megapowers-trigger-recall-*"))
	check("temporary state removed after success", err == nil && len(leftover) == 0)

	failing := scriptedActor{events: map[string][]actorEvent{}, fail: map[string]error{"recall-hit": errors.New("scripted actor failure")}, rc: map[string]int{}, calls: map[string]int{}}
	failOptions := options
	failOptions.Out = filepath.Join(tempParent, "failure")
	_, _, err = executeStudy(context.Background(), casesFile{SchemaVersion: "1", Cases: corpus.Cases[:1]}, gates, catalog, failOptions, failing)
	check("actor errors fail closed", err != nil && strings.Contains(err.Error(), "actor error"))
	check("repeated actor failures still fail closed", err != nil && failing.calls["recall-hit"] == 2)
	evidence, evidenceErr := os.ReadFile(filepath.Join(failOptions.Out, "failures", "recall-hit-rep-1.json"))
	check("actor failures persist private diagnostics", evidenceErr == nil && strings.Contains(string(evidence), "scripted actor failure"))

	transient := scriptedActor{
		events:   map[string][]actorEvent{"recall-hit": {{Kind: "skill_selected", Path: "systematic-debugging", RC: 0, Step: 1}}},
		fail:     map[string]error{},
		failOnce: map[string]error{"recall-hit": errors.New("transient scripted failure")},
		rc:       map[string]int{},
		calls:    map[string]int{},
	}
	transientOptions := options
	transientOptions.Out = filepath.Join(tempParent, "transient")
	transientRows, transientManifest, err := executeStudy(context.Background(), casesFile{SchemaVersion: "1", Cases: corpus.Cases[:1]}, gates, catalog, transientOptions, transient)
	check("transient actor failures retry once", err == nil && len(transientRows) == 1 && transientRows[0].Verdict == "pass" && transientManifest.Retries == 1 && transient.calls["recall-hit"] == 2)

	deadlineOptions := options
	deadlineOptions.Out = filepath.Join(tempParent, "deadline")
	deadlineOptions.ActorTimeout = 10 * time.Millisecond
	_, _, err = executeStudy(context.Background(), casesFile{SchemaVersion: "1", Cases: corpus.Cases[:1]}, gates, catalog, deadlineOptions, blockingActor{})
	check("actor deadlines fail closed", err != nil && strings.Contains(err.Error(), "timed out"))

	_, err = validateBroker("", "", root, outSuccess)
	check("live runs require isolated broker", err != nil)

	request, roots := makeBrokerRequest(actorRequest{Case: corpus.Cases[0], Harness: "claude", Model: "m", Effort: "high", Home: "/private/home", Project: "/private/project", PluginRoot: "/private/plugin", Timeout: time.Minute})
	rootsExcludeRepo := len(roots) == 2 && request.Project == "/private/project" && request.PluginRepo == "/private/plugin"
	for _, readRoot := range request.TaskReadRoots {
		if pathsOverlap(readRoot, root) {
			rootsExcludeRepo = false
		}
	}
	check("broker request excludes repository root", rootsExcludeRepo && samePaths(request.TaskWriteRoots, []string{"/private/project"}))

	floorTallies := []caseTally{{CaseID: "recall-hit", Expected: "systematic-debugging", Pass: 0, Total: 3}}
	enforceGates := gates
	enforceGates.Mode = "enforce"
	enforceAssessment := assessGates(floorTallies, enforceGates, 3)
	check("enforce gates reject a recall floor", len(enforceAssessment.Violations) == 1 && strings.Contains(enforceAssessment.Violations[0], "recall 0/3"))
	reportAssessment := assessGates(floorTallies, gates, 3)
	check("report-only gates record violations without failing", reportAssessment.Mode == "report-only" && len(reportAssessment.Violations) == 1)

	sanitized := publishFilesOnly(outSuccess) && publishLacks(outSuccess, []string{sentinel, tempParent})
	check("publish bundle contains sanitized files only", sanitized && len(manifest.Assessment.Cases) == len(corpus.Cases))

	if failures > 0 {
		return fmt.Errorf("%d selftest check(s) failed", failures)
	}
	return nil
}

func strictScore(root, rows string) error {
	command := exec.Command("go", "run", filepath.Join(root, "evals", "score.go"), "--strict", rows)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("strict scorer failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func publishFilesOnly(out string) bool {
	publish := filepath.Join(out, "publish")
	files := make([]string, 0)
	err := filepath.WalkDir(publish, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			relative, relErr := filepath.Rel(publish, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	sort.Strings(files)
	return err == nil && len(files) == 2 && files[0] == "manifest.json" && files[1] == "results.jsonl"
}

func publishLacks(out string, banned []string) bool {
	err := filepath.WalkDir(filepath.Join(out, "publish"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, value := range banned {
			if strings.Contains(string(content), value) {
				return fmt.Errorf("banned publish content %q", value)
			}
		}
		return nil
	})
	return err == nil
}
