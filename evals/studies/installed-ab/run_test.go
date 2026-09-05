package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAssessStudyAcceptanceSeparatesOutcomeAndActivation(t *testing.T) {
	var gates gatesFile
	gates.Acceptance.MinimumPairedRuns = 1
	gates.Acceptance.RequireAllTreatmentPasses = true
	rows := []resultRow{
		{
			CaseID: "activation-miss", BlockID: "activation-miss-001", Arm: "treatment", Status: "completed", Verdict: "fail",
			Metrics: map[string]float64{"outcome_success": 1, "task_success": 0, "skill_contract_required": 1, "skill_contract_success": 0},
		},
		{
			CaseID: "activation-miss", BlockID: "activation-miss-001", Arm: "control", Status: "completed", Verdict: "pass",
			Metrics: map[string]float64{"outcome_success": 1, "task_success": 1, "skill_contract_required": 0, "skill_contract_success": 0},
		},
	}

	got := assessStudyAcceptance(rows, gates)
	if len(got.Cases) != 1 {
		t.Fatalf("case assessments = %d, want 1", len(got.Cases))
	}
	caseGot := got.Cases[0]
	if !caseGot.OutcomeAccepted || !got.OutcomeAccepted {
		t.Fatalf("outcome axis rejected a successful treatment outcome: case=%+v study=%+v", caseGot, got)
	}
	if caseGot.ActivationAccepted || got.ActivationAccepted {
		t.Fatalf("activation axis accepted a required activation miss: case=%+v study=%+v", caseGot, got)
	}
	if caseGot.Accepted || got.Accepted {
		t.Fatalf("combined acceptance ignored the explicit activation gate: case=%+v study=%+v", caseGot, got)
	}
}

func TestOrchestrationDispatchBatchIgnoresChildCompletionTiming(t *testing.T) {
	events := []actorEvent{
		{Kind: "agent_spawn", Path: "api", RC: 0, Step: 1},
		{Kind: "agent_complete", Path: "api", RC: 0, Step: 2},
		{Kind: "agent_spawn", Path: "storage", RC: 0, Step: 3},
		{Kind: "agent_complete", Path: "storage", RC: 0, Step: 4},
		{Kind: "agent_spawn", Path: "ui", RC: 0, Step: 5},
		{Kind: "agent_wait", RC: 0, Step: 6},
		{Kind: "agent_complete", Path: "ui", RC: 0, Step: 7},
	}

	spawns, attempts, beforeWait, dispatchedWithoutInterveningWork, matching := orchestrationEvidence(events, 3)
	if spawns != 3 || attempts != 3 || !beforeWait || !dispatchedWithoutInterveningWork || !matching {
		t.Fatalf("scheduler timing changed model-controlled dispatch grading: spawns=%d attempts=%d before_wait=%t dispatch_batch=%t matching=%t", spawns, attempts, beforeWait, dispatchedWithoutInterveningWork, matching)
	}
}

func TestOrchestrationDispatchBatchRejectsInterveningModelWork(t *testing.T) {
	events := []actorEvent{
		{Kind: "agent_spawn", Path: "api", RC: 0, Step: 1},
		{Kind: "write", Path: "notes.md", RC: 0, Step: 2},
		{Kind: "agent_spawn", Path: "storage", RC: 0, Step: 3},
		{Kind: "agent_spawn", Path: "ui", RC: 0, Step: 4},
		{Kind: "agent_complete", Path: "api", RC: 0, Step: 5},
		{Kind: "agent_complete", Path: "storage", RC: 0, Step: 6},
		{Kind: "agent_complete", Path: "ui", RC: 0, Step: 7},
	}

	_, _, _, dispatchedWithoutInterveningWork, matching := orchestrationEvidence(events, 3)
	if dispatchedWithoutInterveningWork {
		t.Fatal("dispatch batch accepted model-controlled work between spawn calls")
	}
	if !matching {
		t.Fatal("matching successful joins should remain independent from dispatch ordering")
	}
}

func TestPromptHashBindsFollowupTasksAndBrokerRequest(t *testing.T) {
	base := studyCase{ID: "followup", Task: "First turn"}
	withFollowup := base
	withFollowup.FollowupTasks = []string{"Second turn", "Third turn"}
	if promptHash(base) == promptHash(withFollowup) {
		t.Fatal("prompt hash does not bind follow-up turns")
	}
	request, _ := makeBrokerRequest(actorRequest{
		Case: withFollowup, Arm: "control", Harness: "codex", Model: "model", Effort: "high",
		Home: "/tmp/home", Project: "/tmp/project", Timeout: time.Minute,
	})
	if !reflect.DeepEqual(request.FollowupTasks, withFollowup.FollowupTasks) {
		t.Fatalf("broker follow-ups = %#v, want %#v", request.FollowupTasks, withFollowup.FollowupTasks)
	}
}

func TestFilterCasesPreservesCatalogOrderAndRejectsUnknownIDs(t *testing.T) {
	catalog := casesFile{SchemaVersion: "1", Cases: []studyCase{{ID: "first"}, {ID: "second"}, {ID: "third"}}}
	filtered, err := filterCases(catalog, []string{"third", "first"})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{filtered.Cases[0].ID, filtered.Cases[1].ID}
	if !reflect.DeepEqual(got, []string{"first", "third"}) {
		t.Fatalf("filtered order = %v, want catalog order", got)
	}
	if _, err := filterCases(catalog, []string{"missing"}); err == nil {
		t.Fatal("unknown case filter was accepted")
	}
}

func TestEvaluateCaseAppliesDeclaredSafetyToProse(t *testing.T) {
	var gates gatesFile
	gates.Prose.MinimumFactRetention = 1
	gates.Prose.MaximumInventedFacts = 0
	c := studyCase{
		ID: "safe-prose", Kind: "prose", Task: "report", RequiredFacts: []string{"ready"},
		ForbiddenEventKinds: []string{"write"},
	}
	result := actorResult{Response: "ready", Events: []actorEvent{{Kind: "write", Path: "report.md", RC: 0}}}
	metrics, verdict, err := evaluateCase(context.Background(), c, gates, "treatment", t.TempDir(), 0, result, true)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "fail" || metrics["outcome_success"] != 0 || metrics["forbidden_event_attempts"] != 1 {
		t.Fatalf("declared prose safety was not a hard outcome gate: verdict=%s metrics=%v", verdict, metrics)
	}
}

func TestEvaluateCaseDoesNotRequireUndeclaredSkillActivation(t *testing.T) {
	var gates gatesFile
	gates.Prose.MinimumFactRetention = 1
	gates.Prose.MaximumInventedFacts = 0
	c := studyCase{ID: "simple-noop", Kind: "prose", Task: "report", RequiredFacts: []string{"ready"}}
	metrics, verdict, err := evaluateCase(context.Background(), c, gates, "treatment", t.TempDir(), 0, actorResult{Response: "ready"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "pass" || metrics["skill_contract_required"] != 0 {
		t.Fatalf("undeclared skill activation became a treatment gate: verdict=%s metrics=%v", verdict, metrics)
	}
}

func TestOrchestrationFanoutCanBoundExplicitSpawnAttempts(t *testing.T) {
	maximum := 3
	var gates gatesFile
	gates.Orchestration.MinimumSuccessfulSpawns = 3
	minimum, gotMaximum := orchestrationSpawnBounds(studyCase{
		OrchestrationMode: "fanout", RequiredAgentSpawns: 3, MaximumAgentSpawns: &maximum,
	}, gates)
	if minimum != 3 || gotMaximum != 3 {
		t.Fatalf("fanout bounds = (%d,%d), want (3,3)", minimum, gotMaximum)
	}
}

func TestDesignPlanFactsAcceptParaphraseAndDoNotRejectWarning(t *testing.T) {
	var catalog casesFile
	if err := decodeStrict("cases.json", &catalog); err != nil {
		t.Fatal(err)
	}
	var design studyCase
	for _, c := range catalog.Cases {
		if c.ID == "design-plan-ambiguous-contract" {
			design = c
			break
		}
	}
	if design.ID == "" {
		t.Fatal("design-plan-ambiguous-contract is missing")
	}
	response := "Current public value is compact. There are 14 existing clients. Compatibility remains unresolved. API and UI are separate work lanes. Do not conclude that clients may break."
	retained, invented := factCounts(response, design.RequiredFacts, design.ForbiddenFacts)
	if retained != len(design.RequiredFacts) || invented != 0 {
		t.Fatalf("explicit fact alternatives graded a paraphrase or warning incorrectly: retained=%d/%d invented=%d", retained, len(design.RequiredFacts), invented)
	}
	unsafe := "Compatibility decision: breakage allowed."
	_, invented = factCounts(unsafe, design.RequiredFacts, design.ForbiddenFacts)
	if invented != 1 {
		t.Fatalf("case-specific unsafe assertion was not rejected: invented=%d", invented)
	}
}

func TestAuthoringFactsAcceptOneScopedLineLayout(t *testing.T) {
	var catalog casesFile
	if err := decodeStrict("cases.json", &catalog); err != nil {
		t.Fatal(err)
	}
	var authoring studyCase
	for _, c := range catalog.Cases {
		if c.ID == "author-instructions-smallest-scope" {
			authoring = c
			break
		}
	}
	if authoring.ID == "" {
		t.Fatal("author-instructions-smallest-scope is missing")
	}
	response := "Keep root AGENTS.md and root CLAUDE.md with @AGENTS.md. Do not copy the root rules. Add one scoped line under services/payments: amounts use integer cents. If a deterministic helper becomes necessary, write it in Go."
	retained, invented := factCounts(response, authoring.RequiredFacts, authoring.ForbiddenFacts)
	if retained != len(authoring.RequiredFacts) || invented != 0 {
		t.Fatalf("smallest valid authoring layout = retained %d/%d invented %d", retained, len(authoring.RequiredFacts), invented)
	}
}

func TestFailureReceiptNamesBoundedEvidenceWithoutRawContent(t *testing.T) {
	c := studyCase{
		ID: "receipt", RequiredFacts: []string{"compatibility is unresolved", "14 clients"},
		RequiredFactIDs: []string{"compatibility", "client_count"},
		ForbiddenFacts:  []string{"breakage allowed"}, ForbiddenFactIDs: []string{"breakage_allowed"},
		RequiredSkillOrder: []string{"design-and-plan"}, ForbiddenSkills: []string{"test-first-implementation"}, ForbiddenEventKinds: []string{"write"},
	}
	row := resultRow{RunID: "receipt-001-treatment-run", BlockID: "receipt-001", CaseID: c.ID, Arm: "treatment", Verdict: "fail"}
	result := actorResult{
		Response: "14 clients. Breakage allowed. private-response-marker",
		Events: []actorEvent{
			{Kind: "skill_selected", Path: "design-and-plan", RC: 0, Step: 1},
			{Kind: "skill_selected", Path: "test-first-implementation", RC: 0, Step: 2},
			{Kind: "skill_selected", Path: "sk-abcdefghijklmnopqrstuvwxyz0123456789", RC: 0, Step: 3},
			{Kind: "write", Path: "private/path.txt", RC: 0, Step: 4},
			{Kind: "test", Path: "go test ./... --token private-token", RC: 1, Step: 5},
		},
	}
	receipt := buildFailureReceipt(row, c, result)
	if !reflect.DeepEqual(receipt.MissingRequiredFactIDs, []string{"compatibility"}) || !reflect.DeepEqual(receipt.DetectedForbiddenFactIDs, []string{"breakage_allowed"}) {
		t.Fatalf("fact IDs = missing %v forbidden %v", receipt.MissingRequiredFactIDs, receipt.DetectedForbiddenFactIDs)
	}
	if !reflect.DeepEqual(receipt.SkillSelections, []skillSelectionReceipt{{Skill: "design-and-plan", RC: 0}, {Skill: "test-first-implementation", RC: 0}, {Skill: "redacted", RC: 0}}) {
		t.Fatalf("skill selections = %#v", receipt.SkillSelections)
	}
	if !reflect.DeepEqual(receipt.ForbiddenEventKinds, []eventAttemptReceipt{{Kind: "write", Attempts: 1}}) {
		t.Fatalf("forbidden events = %#v", receipt.ForbiddenEventKinds)
	}
	if !reflect.DeepEqual(receipt.TestResults, []testResultReceipt{{Command: "go test", RC: 1, Step: 5}}) {
		t.Fatalf("test results = %#v", receipt.TestResults)
	}
	payload, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range [][]byte{[]byte("private-response-marker"), []byte("private-token"), []byte("private/path.txt"), []byte("sk-abcdefghijklmnopqrstuvwxyz0123456789")} {
		if bytes.Contains(payload, private) {
			t.Fatalf("private failure receipt leaked %q: %s", private, payload)
		}
	}
	out := t.TempDir()
	if err := writeFailureReceipt(out, row, c, result); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(out, "private", "failure-receipts", row.RunID+".json")
	info, err := os.Stat(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private receipt mode = %04o, want 0600", info.Mode().Perm())
	}
	written, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var writtenReceipt failureReceipt
	if err := json.Unmarshal(written, &writtenReceipt); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writtenReceipt, receipt) {
		t.Fatalf("written private receipt differs from bounded receipt: %s", written)
	}
}

func TestFailureReceiptMarksMissingTrustedTDDReceipt(t *testing.T) {
	c := studyCase{ID: "tdd-receipt-gap", Kind: "tdd"}
	row := resultRow{RunID: "tdd-receipt-gap-001", BlockID: "tdd-receipt-gap", CaseID: c.ID, Arm: "treatment", Verdict: "fail"}
	result := actorResult{
		Trace:  []byte(`{"type":"actor"}`),
		Events: []actorEvent{{Kind: "test", Path: "go test", RC: 1, Step: 2}},
	}

	receipt := buildFailureReceipt(row, c, result)
	if !reflect.DeepEqual(receipt.ObservabilityGaps, []string{"trusted_test_execution_receipt_missing"}) {
		t.Fatalf("TDD observability gaps = %v", receipt.ObservabilityGaps)
	}
	if len(receipt.TestResults) != 1 {
		t.Fatalf("native test attempt should remain diagnostic, got %#v", receipt.TestResults)
	}
}

func TestFailureReceiptNamesValidatedPublicCatalogSkillsOnly(t *testing.T) {
	c := studyCase{ID: "catalog-receipt", RequiredSkillOrder: []string{"orchestrating"}}
	row := resultRow{RunID: "catalog-receipt-001", BlockID: "catalog-receipt", CaseID: c.ID, Arm: "treatment", Verdict: "fail"}
	result := actorResult{
		SkillsCatalog: []string{"orchestrating", "sk-abcdefghijklmnopqrstuvwxyz0123456789", "verify-and-finish"},
		Events: []actorEvent{
			{Kind: "skill_selected", Path: "orchestrating", RC: 0},
			{Kind: "skill_selected", Path: "verify-and-finish", RC: 0},
			{Kind: "skill_selected", Path: "sk-abcdefghijklmnopqrstuvwxyz0123456789", RC: 0},
		},
	}

	receipt := buildFailureReceipt(row, c, result)
	want := []skillSelectionReceipt{{Skill: "orchestrating", RC: 0}, {Skill: "verify-and-finish", RC: 0}, {Skill: "redacted", RC: 0}}
	if !reflect.DeepEqual(receipt.SkillSelections, want) {
		t.Fatalf("catalog-backed skill selections = %#v, want %#v", receipt.SkillSelections, want)
	}
}

func TestValidateBrokerSkillsCatalogAllowsOnlyPublicMegapowersNames(t *testing.T) {
	valid := &brokerSkillsCatalog{Rendered: true, Skills: []string{"orchestrating", "verify-and-finish"}, Source: "codex-rollout-developer-message"}
	got, err := validateBrokerSkillsCatalog(valid, "treatment")
	if err != nil || !reflect.DeepEqual(got, valid.Skills) {
		t.Fatalf("valid broker catalog = %v, %v", got, err)
	}
	unknown := &brokerSkillsCatalog{Rendered: true, Skills: []string{"orchestrating", "sk-abcdefghijklmnopqrstuvwxyz0123456789"}, Source: "codex-rollout-developer-message"}
	if _, err := validateBrokerSkillsCatalog(unknown, "treatment"); err == nil {
		t.Fatal("token-shaped unknown skill was accepted into the diagnostic catalog")
	}
	if _, err := validateBrokerSkillsCatalog(valid, "control"); err == nil {
		t.Fatal("control arm accepted a treatment skill catalog")
	}
}

func TestFailureReceiptIncludesSanitizedUnboundExecutionReceipts(t *testing.T) {
	c := studyCase{ID: "unbound-tdd-receipts", Kind: "tdd"}
	row := resultRow{RunID: "unbound-tdd-receipts-001", BlockID: "unbound-tdd-receipts", CaseID: c.ID, Arm: "treatment", Verdict: "fail"}
	state := receiptState("sha256:"+strings.Repeat("a", 64), "private_test.go")
	first := validExecutionReceipt(1, 1, 2, 0, false, true, state, state)
	second := validExecutionReceipt(2, 3, 4, 0, false, true, state, state)

	receipt := buildFailureReceipt(row, c, actorResult{Trace: receiptTrace(t, first, second)})
	want := []testExecutionReceiptDiagnostic{
		{Sequence: 1, Command: "go test", ExitCode: 0, OracleMatch: false, StateStable: true},
		{Sequence: 2, Command: "go test", ExitCode: 0, OracleMatch: false, StateStable: true},
	}
	if !reflect.DeepEqual(receipt.TestExecutionReceipts, want) {
		t.Fatalf("execution receipt diagnostics = %#v, want %#v", receipt.TestExecutionReceipts, want)
	}
	if !reflect.DeepEqual(receipt.ObservabilityGaps, []string{"trusted_test_execution_receipt_unbound"}) {
		t.Fatalf("unbound execution receipt gaps = %v", receipt.ObservabilityGaps)
	}
	payload, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private_test.go", "sha256:" + strings.Repeat("a", 64), "invocation_digest", "changed_files"} {
		if bytes.Contains(payload, []byte(private)) {
			t.Fatalf("private execution receipt leaked %q: %s", private, payload)
		}
	}
}

func TestPublicSkillDiagnosticAllowlistMatchesShippedCatalog(t *testing.T) {
	entries, err := os.ReadDir("../../../plugins/megapowers/skills")
	if err != nil {
		t.Fatal(err)
	}
	shipped := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			shipped = append(shipped, entry.Name())
		}
	}
	if !reflect.DeepEqual(shipped, publicMegapowersSkillNames[:]) {
		t.Fatalf("diagnostic allowlist = %v, shipped catalog = %v", publicMegapowersSkillNames, shipped)
	}
}

func TestImplicitOrchestrationDispatchIsDiagnostic(t *testing.T) {
	var gates gatesFile
	gates.Orchestration.MinimumSuccessfulSpawns = 3
	gates.Orchestration.MinimumFactRetention = 1
	gates.Orchestration.MaximumInventedFacts = 0
	gates.Orchestration.RequireSpawnsBeforeFirstWait = true
	gates.Orchestration.RequireSpawnBatchBeforeCompletion = true
	gates.Orchestration.RequireMatchingCompletions = true
	gates.Orchestration.RequireCompleteTrace = true
	c := studyCase{
		ID: "implicit", Kind: "orchestration", Task: "audit", OrchestrationMode: "fanout",
		RequiredAgentSpawns: 3, RequiredFacts: []string{"supported finding"}, RequiredSkillOrder: []string{"orchestrating"},
	}
	result := actorResult{
		Response: "supported finding", Trace: []byte("complete"),
		Events: []actorEvent{{Kind: "skill_selected", Path: "orchestrating", RC: 0, Step: 1}, {Kind: "trace_complete", RC: 0, Step: 2}},
	}
	metrics, _, err := evaluateCase(context.Background(), c, gates, "treatment", t.TempDir(), 0, result, true)
	if err != nil {
		t.Fatal(err)
	}
	if metrics["outcome_success"] != 1 || metrics["dispatch_contract_success"] != 0 || metrics["delegation_required"] != 0 {
		t.Fatalf("implicit routing was scored as a hidden task requirement: %v", metrics)
	}
}

func TestExplicitOrchestrationDispatchRemainsHardGate(t *testing.T) {
	var gates gatesFile
	gates.Orchestration.MinimumSuccessfulSpawns = 3
	gates.Orchestration.MinimumFactRetention = 1
	gates.Orchestration.MaximumInventedFacts = 0
	gates.Orchestration.RequireSpawnsBeforeFirstWait = true
	gates.Orchestration.RequireSpawnBatchBeforeCompletion = true
	gates.Orchestration.RequireMatchingCompletions = true
	gates.Orchestration.RequireCompleteTrace = true
	c := studyCase{
		ID: "explicit", Kind: "orchestration", Task: "delegate", OrchestrationMode: "fanout", RequireDelegation: true,
		RequiredAgentSpawns: 3, RequiredFacts: []string{"supported finding"}, RequiredSkillOrder: []string{"orchestrating"},
	}
	result := actorResult{
		Response: "supported finding", Trace: []byte("complete"),
		Events: []actorEvent{{Kind: "skill_selected", Path: "orchestrating", RC: 0, Step: 1}, {Kind: "trace_complete", RC: 0, Step: 2}},
	}
	metrics, _, err := evaluateCase(context.Background(), c, gates, "treatment", t.TempDir(), 0, result, true)
	if err != nil {
		t.Fatal(err)
	}
	if metrics["outcome_success"] != 0 || metrics["delegation_required"] != 1 {
		t.Fatalf("explicit dispatch contract was not a hard task gate: %v", metrics)
	}
}

func TestSkillSequenceCollapsesRepeatedAdjacentFollowupActivation(t *testing.T) {
	events := []actorEvent{
		{Kind: "skill_selected", Path: "design-and-plan", RC: 0, Step: 1},
		{Kind: "skill_selected", Path: "design-and-plan", RC: 0, Step: 9},
	}
	ordered, unexpected := skillSelectionEvidence(events, []string{"design-and-plan"}, nil)
	if !ordered || unexpected != 0 {
		t.Fatalf("legitimate follow-up activation failed exact sequence: ordered=%t unexpected=%d", ordered, unexpected)
	}
	reversed, _ := skillSelectionEvidence([]actorEvent{
		{Kind: "skill_selected", Path: "verify-and-finish", RC: 0},
		{Kind: "skill_selected", Path: "design-and-plan", RC: 0},
	}, []string{"design-and-plan", "verify-and-finish"}, nil)
	if reversed {
		t.Fatal("collapsing repeated activation accepted reversed distinct skills")
	}
}

func TestTDDReceiptEvidenceAcceptsStableBoundRedGreenSequence(t *testing.T) {
	c := studyCase{OracleCommand: []string{"go", "test", "./..."}, Files: map[string]string{
		"calculator.go":                 "package calculator\n",
		"calculator_acceptance_test.go": "package calculator\n",
	}}
	red := validExecutionReceipt(1, 1, 2, 1, true, true, receiptState("sha256:"+strings.Repeat("a", 64), "calculator_multiply_test.go"), receiptState("sha256:"+strings.Repeat("a", 64), "calculator_multiply_test.go"))
	green := validExecutionReceipt(2, 3, 4, 0, true, true, receiptState("sha256:"+strings.Repeat("b", 64), "calculator.go", "calculator_multiply_test.go"), receiptState("sha256:"+strings.Repeat("b", 64), "calculator.go", "calculator_multiply_test.go"))

	testFirst, redFirst, err := tddEvidenceForResult(c, actorResult{Trace: receiptTrace(t, red, green)}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !testFirst || !redFirst {
		t.Fatalf("stable broker red-green sequence = test_first %t red_first %t, want true true", testFirst, redFirst)
	}
}

func TestTDDReceiptEvidenceRejectsConcurrentImplementationWrite(t *testing.T) {
	c := studyCase{OracleCommand: []string{"go", "test", "./..."}, Files: map[string]string{"calculator.go": "package calculator\n"}}
	red := validExecutionReceipt(1, 1, 2, 1, true, false,
		receiptState("sha256:"+strings.Repeat("a", 64), "calculator_multiply_test.go"),
		receiptState("sha256:"+strings.Repeat("b", 64), "calculator.go", "calculator_multiply_test.go"),
	)
	green := validExecutionReceipt(2, 3, 4, 0, true, true,
		receiptState("sha256:"+strings.Repeat("b", 64), "calculator.go", "calculator_multiply_test.go"),
		receiptState("sha256:"+strings.Repeat("b", 64), "calculator.go", "calculator_multiply_test.go"),
	)
	legacyPassing := []actorEvent{
		{Kind: "write", Path: "calculator_multiply_test.go", Step: 1},
		{Kind: "test", Path: "go test", RC: 1, Step: 2},
		{Kind: "write", Path: "calculator.go", Step: 3},
	}

	testFirst, redFirst, err := tddEvidenceForResult(c, actorResult{Trace: receiptTrace(t, red, green), Events: legacyPassing}, false)
	if err != nil {
		t.Fatal(err)
	}
	if testFirst || redFirst {
		t.Fatalf("concurrent implementation mutation supplied TDD proof: test_first %t red_first %t", testFirst, redFirst)
	}
}

func TestTDDReceiptEvidenceRejectsMissingTargetWithoutLegacyFallback(t *testing.T) {
	c := studyCase{OracleCommand: []string{"go", "test", "./..."}, Files: map[string]string{"calculator.go": "package calculator\n"}}
	missingTarget := validExecutionReceipt(1, 1, 2, 1, false, true,
		receiptState("sha256:"+strings.Repeat("a", 64), "calculator_multiply_test.go"),
		receiptState("sha256:"+strings.Repeat("a", 64), "calculator_multiply_test.go"),
	)
	green := validExecutionReceipt(2, 3, 4, 0, true, true,
		receiptState("sha256:"+strings.Repeat("b", 64), "calculator.go", "calculator_multiply_test.go"),
		receiptState("sha256:"+strings.Repeat("b", 64), "calculator.go", "calculator_multiply_test.go"),
	)
	legacyPassing := []actorEvent{
		{Kind: "write", Path: "calculator_multiply_test.go", Step: 1},
		{Kind: "test", Path: "go test", RC: 1, Step: 2},
		{Kind: "write", Path: "calculator.go", Step: 3},
	}

	testFirst, redFirst, err := tddEvidenceForResult(c, actorResult{Trace: receiptTrace(t, missingTarget, green), Events: legacyPassing}, false)
	if err != nil {
		t.Fatal(err)
	}
	if testFirst || redFirst {
		t.Fatalf("unbound missing-target command supplied TDD proof: test_first %t red_first %t", testFirst, redFirst)
	}
}

func TestTDDReceiptEvidenceRejectsMalformedReservedReceiptWithoutLegacyFallback(t *testing.T) {
	c := studyCase{OracleCommand: []string{"go", "test", "./..."}, Files: map[string]string{"calculator.go": "package calculator\n"}}
	malformed := validExecutionReceipt(1, 1, 2, 1, true, true,
		receiptState("sha256:"+strings.Repeat("a", 64), "calculator_multiply_test.go"),
		receiptState("sha256:"+strings.Repeat("a", 64), "calculator_multiply_test.go"),
	)
	delete(malformed, "state_stable")
	legacyPassing := []actorEvent{
		{Kind: "write", Path: "calculator_multiply_test.go", Step: 1},
		{Kind: "test", Path: "go test", RC: 1, Step: 2},
		{Kind: "write", Path: "calculator.go", Step: 3},
	}

	if _, _, err := tddEvidenceForResult(c, actorResult{Trace: receiptTrace(t, malformed), Events: legacyPassing}, false); err == nil {
		t.Fatal("malformed reserved receipt silently fell back to legacy actor events")
	}
	truncated := []byte(`{"method":"broker/executionReceipt","params":`)
	if _, _, err := tddEvidenceForResult(c, actorResult{Trace: truncated, Events: legacyPassing}, false); err == nil {
		t.Fatal("truncated reserved receipt silently fell back to legacy actor events")
	}
}

func TestTDDReceiptEvidenceRejectsAmbiguousReceiptSequence(t *testing.T) {
	c := studyCase{OracleCommand: []string{"go", "test", "./..."}, Files: map[string]string{"calculator.go": "package calculator\n"}}
	state := receiptState("sha256:"+strings.Repeat("a", 64), "calculator_multiply_test.go")
	first := validExecutionReceipt(1, 1, 2, 1, true, true, state, state)
	duplicate := validExecutionReceipt(1, 3, 4, 0, true, true, state, state)
	legacyPassing := []actorEvent{
		{Kind: "write", Path: "calculator_multiply_test.go", Step: 1},
		{Kind: "test", Path: "go test", RC: 1, Step: 2},
		{Kind: "write", Path: "calculator.go", Step: 3},
	}

	if _, _, err := tddEvidenceForResult(c, actorResult{Trace: receiptTrace(t, first, duplicate), Events: legacyPassing}, false); err == nil {
		t.Fatal("ambiguous receipt sequence silently fell back to legacy actor events")
	}
}

func TestTDDReceiptEvidenceRetainsLegacySupportWithoutReservedReceipts(t *testing.T) {
	events := []actorEvent{
		{Kind: "write", Path: "calculator_multiply_test.go", Step: 1},
		{Kind: "test", Path: "go test", RC: 1, Step: 2},
		{Kind: "write", Path: "calculator.go", Step: 3},
	}
	testFirst, redFirst, err := tddEvidenceForResult(studyCase{}, actorResult{Trace: []byte(`{"type":"actor"}`), Events: events}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !testFirst || !redFirst {
		t.Fatalf("receipt-free legacy trace lost TDD evidence: test_first %t red_first %t", testFirst, redFirst)
	}
}

func TestReceiptCommandForOracleClassifiesValidateScript(t *testing.T) {
	got, ok := receiptCommandForOracle([]string{"bash", "scripts/validate.sh"})
	if !ok || got != "scripts/validate.sh" {
		t.Fatalf("validate script command = %q, %t", got, ok)
	}
}

func TestEvaluateCaseSeparatesPassingArtifactFromMissingTDDWorkflow(t *testing.T) {
	project := t.TempDir()
	c := studyCase{
		ID: "tdd-artifact", Kind: "tdd",
		Files: map[string]string{
			"calculator.go":                 "package calculator\n",
			"calculator_acceptance_test.go": "package calculator\n",
		},
		ProtectedFiles: []string{"calculator_acceptance_test.go"},
		OracleCommand:  []string{"go", "test", "./..."},
	}
	if err := materializeFixture(project, c.Files); err != nil {
		t.Fatal(err)
	}
	missingTarget := validExecutionReceipt(1, 1, 2, 1, false, true,
		receiptState("sha256:"+strings.Repeat("a", 64), "calculator_multiply_test.go"),
		receiptState("sha256:"+strings.Repeat("a", 64), "calculator_multiply_test.go"),
	)
	zero := 0
	gates := gatesFile{}
	gates.TDD.RequireTestBeforeImplementation = true
	gates.TDD.RequireRedBeforeImplementation = true
	gates.TDD.RequirePassingOracle = true
	metrics, verdict, err := evaluateCase(context.Background(), c, gates, "treatment", project, 0, actorResult{Trace: receiptTrace(t, missingTarget), OracleRC: &zero}, false)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "fail" || metrics["artifact_success"] != 1 || metrics["workflow_success"] != 0 || metrics["outcome_success"] != 0 {
		t.Fatalf("passing artifact/missing TDD workflow axes = verdict %s metrics %v", verdict, metrics)
	}
}

func TestEvaluateCaseLiveTDDRejectsReceiptFreeNativeEvents(t *testing.T) {
	project := t.TempDir()
	c := studyCase{
		ID: "live-tdd", Kind: "tdd",
		Files:          map[string]string{"calculator.go": "package calculator\n", "calculator_acceptance_test.go": "package calculator\n"},
		ProtectedFiles: []string{"calculator_acceptance_test.go"},
		OracleCommand:  []string{"go", "test", "./..."},
	}
	if err := materializeFixture(project, c.Files); err != nil {
		t.Fatal(err)
	}
	zero := 0
	legacyPassing := []actorEvent{
		{Kind: "write", Path: "calculator_multiply_test.go", Step: 1},
		{Kind: "test", Path: "go test", RC: 1, Step: 2},
		{Kind: "write", Path: "calculator.go", Step: 3},
	}
	gates := gatesFile{}
	gates.TDD.RequireTestBeforeImplementation = true
	gates.TDD.RequireRedBeforeImplementation = true
	gates.TDD.RequirePassingOracle = true
	metrics, verdict, err := evaluateCase(context.Background(), c, gates, "treatment", project, 0, actorResult{Trace: []byte(`{"type":"actor"}`), Events: legacyPassing, OracleRC: &zero}, false)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "fail" || metrics["artifact_success"] != 1 || metrics["workflow_success"] != 0 || metrics["outcome_success"] != 0 {
		t.Fatalf("receipt-free live TDD bypassed trusted workflow proof: verdict %s metrics %v", verdict, metrics)
	}
}

func receiptState(digest string, changedFiles ...string) map[string]any {
	return map[string]any{"complete": true, "digest": digest, "changed_files": changedFiles}
}

func validExecutionReceipt(sequence, started, completed, exitCode int, oracleMatch, stateStable bool, before, after map[string]any) map[string]any {
	return map[string]any{
		"schema_version":    "1",
		"sequence":          sequence,
		"started_step":      started,
		"completed_step":    completed,
		"command":           "go test",
		"invocation_digest": "sha256:" + strings.Repeat("c", 64),
		"oracle_match":      oracleMatch,
		"state_stable":      stateStable,
		"exit_code":         exitCode,
		"before":            before,
		"after":             after,
	}
}

func receiptTrace(t *testing.T, receipts ...map[string]any) []byte {
	t.Helper()
	var trace bytes.Buffer
	for _, receipt := range receipts {
		line, err := json.Marshal(map[string]any{"method": "broker/executionReceipt", "params": receipt})
		if err != nil {
			t.Fatal(err)
		}
		trace.Write(line)
		trace.WriteByte('\n')
	}
	return trace.Bytes()
}
