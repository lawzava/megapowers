// score.go validates and summarizes megapowers evaluation result rows.
//
// Usage:
//
//	go run evals/score.go --strict results.jsonl
//	cat results.jsonl | go run evals/score.go --strict
//
// Strict mode fails closed. It rejects unknown or malformed data, duplicate runs,
// incomplete treatment/control blocks, incomparable provenance, and every
// infrastructure failure. Deterministic regressions are validated and reported,
// but never included in behavioral effect estimates.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const schemaVersion = "1"

const (
	installedABStudy = "installed-plugin-ab"
	emptyPluginHash  = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

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

type environmentEnvelope struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Sandbox string `json:"sandbox"`
	Locale  string `json:"locale"`
}

type resultRow struct {
	SchemaVersion string              `json:"schema_version"`
	Study         string              `json:"study"`
	EvidenceClass string              `json:"evidence_class"`
	CaseID        string              `json:"case_id"`
	RunID         string              `json:"run_id"`
	BlockID       string              `json:"block_id"`
	Arm           string              `json:"arm"`
	Harness       harnessIdentity     `json:"harness"`
	Source        sourceIdentity      `json:"source"`
	PromptHash    string              `json:"prompt_hash"`
	FixtureHash   string              `json:"fixture_hash"`
	PluginHash    string              `json:"plugin_hash"`
	Status        string              `json:"status"`
	RC            int                 `json:"rc"`
	DurationMS    int64               `json:"duration_ms"`
	Verdict       string              `json:"verdict"`
	Metrics       map[string]float64  `json:"metrics"`
	Artifacts     map[string]string   `json:"artifacts"`
	Environment   environmentEnvelope `json:"environment"`
	Timestamp     string              `json:"timestamp"`
	Phase         string              `json:"phase,omitempty"`
}

type tally struct {
	pass int
	fail int
}

type metricAggregate struct {
	sum   float64
	count int
}

type scoreOptions struct {
	path                   string
	acceptanceCriteriaPath string
}

type acceptanceFile struct {
	SchemaVersion string `json:"schema_version"`
	Acceptance    struct {
		MinimumPairedRuns         int  `json:"minimum_paired_runs"`
		RequireAllTreatmentPasses bool `json:"require_all_treatment_passes"`
	} `json:"acceptance"`
}

func (aggregate *metricAggregate) add(value float64) {
	aggregate.sum += value
	aggregate.count++
}

func (aggregate metricAggregate) mean() float64 {
	return aggregate.sum / float64(aggregate.count)
}

func (t *tally) add(verdict string) {
	if verdict == "pass" {
		t.pass++
	} else {
		t.fail++
	}
}

func (t tally) rate() float64 {
	n := t.pass + t.fail
	if n == 0 {
		return math.NaN()
	}
	return float64(t.pass) / float64(n)
}

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$`)
	hashPattern       = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	metricPattern     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,127}$`)
	brokerBoundaries  = map[string]bool{"bwrap": true, "container": true, "seatbelt": true, "sandbox-exec": true, "appcontainer": true}
)

func requiredIdentifier(name, value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("%s must be a non-empty portable identifier", name)
	}
	return nil
}

func requiredHash(name, value string) error {
	if !hashPattern.MatchString(value) {
		return fmt.Errorf("%s must be sha256:<64 lowercase hex characters>", name)
	}
	return nil
}

func validateRow(row resultRow) error {
	if row.SchemaVersion != schemaVersion {
		return fmt.Errorf("schema_version %q is unsupported", row.SchemaVersion)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"study", row.Study},
		{"case_id", row.CaseID},
		{"run_id", row.RunID},
		{"block_id", row.BlockID},
		{"harness.name", row.Harness.Name},
		{"harness.cli_version", row.Harness.CLIVersion},
		{"harness.model", row.Harness.Model},
		{"harness.effort", row.Harness.Effort},
		{"source.repository", row.Source.Repository},
		{"source.revision", row.Source.Revision},
		{"environment.os", row.Environment.OS},
		{"environment.arch", row.Environment.Arch},
		{"environment.sandbox", row.Environment.Sandbox},
		{"environment.locale", row.Environment.Locale},
	} {
		if err := requiredIdentifier(field.name, field.value); err != nil {
			return err
		}
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"prompt_hash", row.PromptHash},
		{"fixture_hash", row.FixtureHash},
		{"plugin_hash", row.PluginHash},
	} {
		if err := requiredHash(field.name, field.value); err != nil {
			return err
		}
	}
	if row.EvidenceClass != "behavioral" && row.EvidenceClass != "regression" {
		return fmt.Errorf("evidence_class %q is unknown", row.EvidenceClass)
	}
	if row.EvidenceClass == "behavioral" && row.Arm != "treatment" && row.Arm != "control" {
		return fmt.Errorf("behavioral arm %q is unknown", row.Arm)
	}
	if row.EvidenceClass == "regression" && row.Arm != "regression" {
		return fmt.Errorf("regression row must use arm %q", "regression")
	}
	if row.Status == "timeout" {
		return errors.New("timed-out run is an infrastructure failure")
	}
	if row.Status == "harness_error" {
		return errors.New("harness error is an infrastructure failure")
	}
	if row.Status != "completed" {
		return fmt.Errorf("status %q is unknown", row.Status)
	}
	if row.Verdict == "indeterminate" {
		return errors.New("indeterminate verdict cannot be scored")
	}
	if row.Verdict == "harness_error" {
		return errors.New("harness error verdict cannot be scored")
	}
	if row.Verdict != "pass" && row.Verdict != "fail" {
		return fmt.Errorf("verdict %q is unknown", row.Verdict)
	}
	if row.Verdict == "pass" && row.RC != 0 {
		return errors.New("passing row must have rc 0")
	}
	if row.Study == installedABStudy && row.EvidenceClass == "behavioral" && row.RC != 0 {
		return errors.New("installed-plugin completed rows must have actor rc 0")
	}
	if row.DurationMS < 0 {
		return errors.New("duration_ms must not be negative")
	}
	if len(row.Metrics) == 0 {
		return errors.New("metrics must contain at least one named measurement")
	}
	for name, value := range row.Metrics {
		if !metricPattern.MatchString(name) {
			return fmt.Errorf("metric name %q is invalid", name)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("metric %q is not finite", name)
		}
	}
	if row.Study == installedABStudy && row.EvidenceClass == "behavioral" {
		outcome, hasOutcome := row.Metrics["outcome_success"]
		if !hasOutcome || (outcome != 0 && outcome != 1) {
			return errors.New("installed-plugin rows require binary outcome_success")
		}
		taskSuccess, hasTaskSuccess := row.Metrics["task_success"]
		if !hasTaskSuccess || taskSuccess != boolFloat(row.Verdict == "pass") {
			return errors.New("installed-plugin task_success must match the verdict")
		}
	}
	if row.Artifacts == nil {
		return errors.New("artifacts must be an object")
	}
	for name, value := range row.Artifacts {
		if !metricPattern.MatchString(name) {
			return fmt.Errorf("artifact name %q is invalid", name)
		}
		if err := requiredHash("artifact "+name, value); err != nil {
			return err
		}
	}
	if _, err := time.Parse(time.RFC3339, row.Timestamp); err != nil {
		return fmt.Errorf("timestamp must be RFC3339: %w", err)
	}
	return nil
}

func decodeRow(line []byte) (resultRow, error) {
	var row resultRow
	var object map[string]json.RawMessage
	if err := json.Unmarshal(line, &object); err != nil {
		return row, err
	}
	if err := requireFields(object, []string{
		"schema_version", "study", "evidence_class", "case_id", "run_id", "block_id", "arm",
		"harness", "source", "prompt_hash", "fixture_hash", "plugin_hash", "status", "rc",
		"duration_ms", "verdict", "metrics", "artifacts", "environment", "timestamp",
	}); err != nil {
		return row, err
	}
	for name, fields := range map[string][]string{
		"harness":     {"name", "cli_version", "model", "effort"},
		"source":      {"repository", "revision"},
		"environment": {"os", "arch", "sandbox", "locale"},
	} {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(object[name], &nested); err != nil {
			return row, fmt.Errorf("field %q must be an object: %w", name, err)
		}
		if err := requireFields(nested, fields); err != nil {
			return row, fmt.Errorf("field %q: %w", name, err)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&row); err != nil {
		return row, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return row, errors.New("multiple JSON values in one row")
		}
		return row, err
	}
	return row, validateRow(row)
}

func requireFields(object map[string]json.RawMessage, names []string) error {
	for _, name := range names {
		if _, exists := object[name]; !exists {
			return fmt.Errorf("required field %q is missing", name)
		}
	}
	return nil
}

func loadRows(reader io.Reader) ([]resultRow, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	rows := make([]resultRow, 0)
	seenRuns := make(map[string]int)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			return nil, fmt.Errorf("line %d: blank rows are not allowed", lineNumber)
		}
		row, err := decodeRow(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if firstLine, exists := seenRuns[row.RunID]; exists {
			return nil, fmt.Errorf("line %d: duplicate run_id %q first seen on line %d", lineNumber, row.RunID, firstLine)
		}
		seenRuns[row.RunID] = lineNumber
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("no result rows")
	}
	return rows, nil
}

func comparisonIdentity(row resultRow) string {
	return strings.Join([]string{
		row.Study,
		row.CaseID,
		row.Harness.Name,
		row.Harness.CLIVersion,
		row.Harness.Model,
		row.Harness.Effort,
		row.Source.Repository,
		row.Source.Revision,
		row.PromptHash,
		row.FixtureHash,
		row.Environment.OS,
		row.Environment.Arch,
		row.Environment.Sandbox,
		row.Environment.Locale,
	}, "\x00")
}

func validateComparisons(rows []resultRow) error {
	blocks := make(map[string][]resultRow)
	type cell struct {
		arms         map[string]*tally
		pluginHashes map[string]map[string]struct{}
		metricSets   map[string]map[string]struct{}
	}
	cells := make(map[string]*cell)

	for _, row := range rows {
		if row.EvidenceClass != "behavioral" {
			continue
		}
		blockKey := strings.Join([]string{row.Study, row.CaseID, row.BlockID}, "\x00")
		blocks[blockKey] = append(blocks[blockKey], row)
		cellKey := comparisonIdentity(row)
		if cells[cellKey] == nil {
			cells[cellKey] = &cell{
				arms: map[string]*tally{"treatment": {}, "control": {}},
				pluginHashes: map[string]map[string]struct{}{
					"treatment": {},
					"control":   {},
				},
				metricSets: map[string]map[string]struct{}{
					"treatment": {},
					"control":   {},
				},
			}
		}
		cells[cellKey].arms[row.Arm].add(row.Verdict)
		cells[cellKey].pluginHashes[row.Arm][row.PluginHash] = struct{}{}
		cells[cellKey].metricSets[row.Arm][metricSignature(row.Metrics)] = struct{}{}
	}

	for key, block := range blocks {
		if len(block) != 2 {
			return fmt.Errorf("behavioral block %q has %d rows; require one treatment and one control", printableKey(key), len(block))
		}
		byArm := map[string]resultRow{}
		for _, row := range block {
			if _, exists := byArm[row.Arm]; exists {
				return fmt.Errorf("behavioral block %q repeats arm %q", printableKey(key), row.Arm)
			}
			byArm[row.Arm] = row
		}
		treatment, treatmentOK := byArm["treatment"]
		control, controlOK := byArm["control"]
		if !treatmentOK || !controlOK {
			return fmt.Errorf("behavioral block %q is not a treatment/control pair", printableKey(key))
		}
		if comparisonIdentity(treatment) != comparisonIdentity(control) {
			return fmt.Errorf("behavioral block %q mixes provenance", printableKey(key))
		}
		if treatment.Study == installedABStudy {
			if treatment.PluginHash == emptyPluginHash {
				return fmt.Errorf("behavioral block %q treatment plugin hash must not identify the empty plugin set", printableKey(key))
			}
			if treatment.PluginHash == control.PluginHash {
				return fmt.Errorf("behavioral block %q treatment and control plugin hashes must differ", printableKey(key))
			}
			if control.PluginHash != emptyPluginHash {
				return fmt.Errorf("behavioral block %q control plugin hash must identify the empty plugin set", printableKey(key))
			}
		}
	}

	for key, cell := range cells {
		treatment := cell.arms["treatment"]
		control := cell.arms["control"]
		if treatment.pass+treatment.fail != control.pass+control.fail {
			return fmt.Errorf("comparison cell %q has unbalanced treatment/control arms", printableKey(key))
		}
		for _, arm := range []string{"treatment", "control"} {
			if len(cell.pluginHashes[arm]) != 1 {
				return fmt.Errorf("comparison cell %q mixes %s plugin hashes", printableKey(key), arm)
			}
			if len(cell.metricSets[arm]) != 1 {
				return fmt.Errorf("comparison cell %q mixes %s metric sets", printableKey(key), arm)
			}
		}
		var treatmentMetrics, controlMetrics string
		for signature := range cell.metricSets["treatment"] {
			treatmentMetrics = signature
		}
		for signature := range cell.metricSets["control"] {
			controlMetrics = signature
		}
		if treatmentMetrics != controlMetrics {
			return fmt.Errorf("comparison cell %q has incomparable treatment/control metrics", printableKey(key))
		}
	}
	return nil
}

func loadAcceptanceCriteria(path string) (acceptanceFile, error) {
	var criteria acceptanceFile
	content, err := os.ReadFile(path)
	if err != nil {
		return criteria, fmt.Errorf("read acceptance criteria: %w", err)
	}
	if err := json.Unmarshal(content, &criteria); err != nil {
		return criteria, fmt.Errorf("decode acceptance criteria: %w", err)
	}
	if criteria.SchemaVersion != schemaVersion {
		return criteria, fmt.Errorf("acceptance criteria schema_version %q is unsupported", criteria.SchemaVersion)
	}
	if criteria.Acceptance.MinimumPairedRuns <= 0 {
		return criteria, errors.New("acceptance minimum_paired_runs must be positive")
	}
	if !criteria.Acceptance.RequireAllTreatmentPasses {
		return criteria, errors.New("acceptance criteria must require all treatment runs to pass")
	}
	return criteria, nil
}

func validateStudyAcceptance(rows []resultRow, criteria acceptanceFile) error {
	type counts struct {
		treatmentPass  int
		treatmentTotal int
		controlPass    int
		controlTotal   int
	}
	byCase := make(map[string]*counts)
	for _, row := range rows {
		if row.EvidenceClass != "behavioral" || row.Study != installedABStudy {
			return errors.New("study acceptance accepts installed-plugin-ab behavioral rows only")
		}
		if !brokerBoundaries[row.Environment.Sandbox] {
			return fmt.Errorf("case %q does not record a broker-attested OS boundary", row.CaseID)
		}
		if row.Metrics["report_only"] == 1 {
			continue
		}
		cell := byCase[row.CaseID]
		if cell == nil {
			cell = &counts{}
			byCase[row.CaseID] = cell
		}
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
	if len(byCase) == 0 {
		return errors.New("study acceptance found no non-report-only cases")
	}
	caseIDs := make([]string, 0, len(byCase))
	for caseID := range byCase {
		caseIDs = append(caseIDs, caseID)
	}
	sort.Strings(caseIDs)
	for _, caseID := range caseIDs {
		cell := byCase[caseID]
		if cell.treatmentTotal != cell.controlTotal || cell.treatmentTotal < criteria.Acceptance.MinimumPairedRuns {
			return fmt.Errorf("case %q has %d balanced pairs; require %d", caseID, minInt(cell.treatmentTotal, cell.controlTotal), criteria.Acceptance.MinimumPairedRuns)
		}
		if cell.treatmentPass != cell.treatmentTotal {
			return fmt.Errorf("case %q treatment passed %d/%d; require every treatment run to pass", caseID, cell.treatmentPass, cell.treatmentTotal)
		}
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func metricSignature(metrics map[string]float64) string {
	names := make([]string, 0, len(metrics))
	for name := range metrics {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, "\x00")
}

func printableKey(key string) string {
	return strings.ReplaceAll(key, "\x00", "/")
}

// logFactorial returns log(n!) via lgamma(n+1); it never overflows.
func logFactorial(n int) float64 {
	value, _ := math.Lgamma(float64(n + 1))
	return value
}

func logChoose(n, k int) float64 {
	return logFactorial(n) - logFactorial(k) - logFactorial(n-k)
}

func mcnemarExactTwoSided(treatmentOnly, controlOnly int) float64 {
	discordant := treatmentOnly + controlOnly
	if discordant == 0 {
		return 1
	}
	tail := minInt(treatmentOnly, controlOnly)
	sum := 0.0
	for successes := 0; successes <= tail; successes++ {
		sum += math.Exp(logChoose(discordant, successes) - float64(discordant)*math.Log(2))
	}
	return math.Min(2*sum, 1)
}

func discordantOutcomes(pairs map[string]map[string]string) (int, int) {
	treatmentOnly := 0
	controlOnly := 0
	for _, pair := range pairs {
		if pair["treatment"] == "pass" && pair["control"] == "fail" {
			treatmentOnly++
		}
		if pair["treatment"] == "fail" && pair["control"] == "pass" {
			controlOnly++
		}
	}
	return treatmentOnly, controlOnly
}

func comparisonVerdict(row resultRow) string {
	if row.Study == installedABStudy && row.EvidenceClass == "behavioral" {
		if row.Metrics["outcome_success"] == 1 {
			return "pass"
		}
		return "fail"
	}
	return row.Verdict
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func passHatK(pass, n, k int) float64 {
	if k <= 0 || n < k || pass < 0 || pass > n {
		return math.NaN()
	}
	if pass < k {
		return 0
	}
	result := 1.0
	for index := 0; index < k; index++ {
		result *= float64(pass-index) / float64(n-index)
	}
	return result
}

func selftest() int {
	failures := 0
	check := func(name string, got, want, tolerance float64) {
		if math.Abs(got-want) > tolerance {
			fmt.Printf("FAIL %s: got %.12g want %.12g (tol %g)\n", name, got, want, tolerance)
			failures++
		} else {
			fmt.Printf("ok   %s: %.12g (want %.12g)\n", name, got, want)
		}
	}
	check("12 treatment-only discordances mcnemar_p", mcnemarExactTwoSided(12, 0), 0.00048828125, 1e-12)
	check("3 vs 2 discordances mcnemar_p", mcnemarExactTwoSided(3, 2), 1.0, 1e-12)
	check("pass^3 of 10/10", passHatK(10, 10, 3), 1.0, 1e-12)
	check("pass^3 of 9/10", passHatK(9, 10, 3), 0.7, 1e-12)
	check("pass^3 of 2/10", passHatK(2, 10, 3), 0.0, 1e-12)
	check("pass^1 equals the pass rate", passHatK(7, 10, 1), 0.7, 1e-12)
	if !math.IsNaN(passHatK(2, 2, 3)) {
		fmt.Println("FAIL pass^3 with n<k must be NaN")
		failures++
	}
	if failures == 0 {
		fmt.Println("score.go selftest: PASS")
	} else {
		fmt.Printf("score.go selftest: FAIL (%d assertion(s))\n", failures)
	}
	return failures
}

func printScorecard(rows []resultRow) {
	fmt.Println("# megapowers evaluation scorecard")
	fmt.Println()

	regressions := make([]resultRow, 0)
	type behavioralCell struct {
		study     string
		caseID    string
		harness   harnessIdentity
		arms      map[string]*tally
		metrics   map[string]map[string]*metricAggregate
		durations map[string]*metricAggregate
		pairs     map[string]map[string]string
	}
	behavioral := make(map[string]*behavioralCell)
	for _, row := range rows {
		if row.EvidenceClass == "regression" {
			regressions = append(regressions, row)
			continue
		}
		key := comparisonIdentity(row)
		if behavioral[key] == nil {
			behavioral[key] = &behavioralCell{
				study:     row.Study,
				caseID:    row.CaseID,
				harness:   row.Harness,
				arms:      map[string]*tally{"treatment": {}, "control": {}},
				metrics:   map[string]map[string]*metricAggregate{"treatment": {}, "control": {}},
				durations: map[string]*metricAggregate{"treatment": {}, "control": {}},
				pairs:     map[string]map[string]string{},
			}
		}
		cell := behavioral[key]
		verdict := comparisonVerdict(row)
		cell.arms[row.Arm].add(verdict)
		if cell.pairs[row.BlockID] == nil {
			cell.pairs[row.BlockID] = map[string]string{}
		}
		cell.pairs[row.BlockID][row.Arm] = verdict
		for name, value := range row.Metrics {
			if cell.metrics[row.Arm][name] == nil {
				cell.metrics[row.Arm][name] = &metricAggregate{}
			}
			cell.metrics[row.Arm][name].add(value)
		}
		cell.durations[row.Arm].add(float64(row.DurationMS))
	}

	if len(regressions) > 0 {
		sort.Slice(regressions, func(i, j int) bool {
			if regressions[i].Study == regressions[j].Study {
				return regressions[i].CaseID < regressions[j].CaseID
			}
			return regressions[i].Study < regressions[j].Study
		})
		fmt.Println("## Deterministic regressions")
		fmt.Println()
		fmt.Println("These rows validate executable contracts. They are not behavioral skill evidence.")
		fmt.Println()
		fmt.Println("| study | case | verdict | duration_ms |")
		fmt.Println("|---|---|---|---:|")
		for _, row := range regressions {
			fmt.Printf("| %s | %s | %s | %d |\n", row.Study, row.CaseID, row.Verdict, row.DurationMS)
		}
		fmt.Println()
	}

	if len(behavioral) > 0 {
		keys := make([]string, 0, len(behavioral))
		for key := range behavioral {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fmt.Println("## Behavioral treatment/control evidence")
		fmt.Println()
		fmt.Println("Installed-plugin comparisons use outcome_success; treatment activation remains visible in task_success and the row verdict.")
		fmt.Println()
		fmt.Println("| study | case | harness | treatment | control | delta | mcnemar_p | treatment pass^3 | control pass^3 |")
		fmt.Println("|---|---|---|---:|---:|---:|---:|---:|---:|")
		for _, key := range keys {
			cell := behavioral[key]
			treatment := cell.arms["treatment"]
			control := cell.arms["control"]
			treatmentN := treatment.pass + treatment.fail
			controlN := control.pass + control.fail
			treatmentRate := treatment.rate()
			controlRate := control.rate()
			treatmentOnly, controlOnly := discordantOutcomes(cell.pairs)
			fmt.Printf("| %s | %s | %s/%s | %d/%d | %d/%d | %+.0f%% | %.3g | %s | %s |\n",
				cell.study,
				cell.caseID,
				cell.harness.Name,
				cell.harness.Model,
				treatment.pass,
				treatmentN,
				control.pass,
				controlN,
				(treatmentRate-controlRate)*100,
				mcnemarExactTwoSided(treatmentOnly, controlOnly),
				formatRate(passHatK(treatment.pass, treatmentN, 3)),
				formatRate(passHatK(control.pass, controlN, 3)),
			)
		}
		fmt.Println()
		fmt.Println("### Named metric means")
		fmt.Println()
		fmt.Println("| study | case | harness | metric | treatment | control | delta |")
		fmt.Println("|---|---|---|---|---:|---:|---:|")
		for _, key := range keys {
			cell := behavioral[key]
			names := make([]string, 0, len(cell.metrics["treatment"]))
			for name := range cell.metrics["treatment"] {
				names = append(names, name)
			}
			sort.Strings(names)
			names = append(names, "duration_ms")
			for _, name := range names {
				var treatmentMean, controlMean float64
				if name == "duration_ms" {
					treatmentMean = cell.durations["treatment"].mean()
					controlMean = cell.durations["control"].mean()
				} else {
					treatmentMean = cell.metrics["treatment"][name].mean()
					controlMean = cell.metrics["control"][name].mean()
				}
				fmt.Printf("| %s | %s | %s/%s | %s | %.4g | %.4g | %+.4g |\n",
					cell.study, cell.caseID, cell.harness.Name, cell.harness.Model, name,
					treatmentMean, controlMean, treatmentMean-controlMean)
			}
		}
	}
}

func formatRate(value float64) string {
	if math.IsNaN(value) {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%%", value*100)
}

func parseArgs(arguments []string) (scoreOptions, error) {
	var options scoreOptions
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		switch argument {
		case "--strict":
			// Strict is also the default. The explicit flag documents fail-closed intent.
		case "--acceptance-criteria":
			index++
			if index >= len(arguments) || arguments[index] == "" {
				return scoreOptions{}, errors.New("--acceptance-criteria requires a path")
			}
			options.acceptanceCriteriaPath = arguments[index]
		case "-":
			if options.path != "" {
				return scoreOptions{}, errors.New("only one input is allowed")
			}
			options.path = "-"
		default:
			if strings.HasPrefix(argument, "-") {
				return scoreOptions{}, fmt.Errorf("unknown flag %q", argument)
			}
			if options.path != "" {
				return scoreOptions{}, errors.New("only one input is allowed")
			}
			options.path = argument
		}
	}
	return options, nil
}

func run(arguments []string, stdin io.Reader) error {
	options, err := parseArgs(arguments)
	if err != nil {
		return err
	}
	reader := stdin
	if options.path != "" && options.path != "-" {
		file, openErr := os.Open(options.path)
		if openErr != nil {
			return openErr
		}
		defer file.Close()
		reader = file
	}
	rows, err := loadRows(reader)
	if err != nil {
		return err
	}
	if err := validateComparisons(rows); err != nil {
		return err
	}
	if options.acceptanceCriteriaPath != "" {
		criteria, err := loadAcceptanceCriteria(options.acceptanceCriteriaPath)
		if err != nil {
			return err
		}
		if err := validateStudyAcceptance(rows, criteria); err != nil {
			return fmt.Errorf("study acceptance failed: %w", err)
		}
	}
	printScorecard(rows)
	return nil
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--selftest" {
		if selftest() != 0 {
			os.Exit(1)
		}
		return
	}
	if err := run(os.Args[1:], os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, "score:", err)
		os.Exit(2)
	}
}
