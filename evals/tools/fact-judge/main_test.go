package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptHidesArmsAndLabelsResponses(t *testing.T) {
	c := studyCase{ID: "case", Task: "Rewrite the note.", RequiredFacts: []string{"blocked", "credentials are unavailable||no credentials"}, ForbiddenFacts: []string{"unblocked"}}
	items := []item{{Label: "R1", Response: "It is blocked.", Row: row{Arm: "treatment"}}, {Label: "R2", Response: "Still blocked.", Row: row{Arm: "control"}}}
	prompt := buildPrompt(c, items)
	for _, want := range []string{"Rewrite the note.", "F1: blocked", "F2: credentials are unavailable / no credentials", "X1: unblocked", "<response id=\"R1\">", "<response id=\"R2\">"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	for _, leak := range []string{"treatment", "control", "case"} {
		if strings.Contains(strings.ToLower(prompt), leak) {
			t.Errorf("prompt leaks %q", leak)
		}
	}
}

func TestParseVerdictsRequiresEveryLabelAndFact(t *testing.T) {
	out := []byte("Here you go:\n```json\n{\"R1\":{\"required\":{\"F1\":true,\"F2\":false},\"forbidden\":{\"X1\":false}},\"R2\":{\"required\":{\"F1\":true,\"F2\":true},\"forbidden\":{\"X1\":true}}}\n```\n")
	verdicts, err := parseVerdicts(out, []string{"R1", "R2"}, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if verdicts["R1"].pass() || !verdicts["R2"].Required[1] || verdicts["R2"].pass() {
		t.Fatalf("verdicts = %+v", verdicts)
	}
	if _, err := parseVerdicts([]byte(`{"R1":{"required":{"F1":true},"forbidden":{"X1":false}}}`), []string{"R1", "R2"}, 2, 1); err == nil {
		t.Fatal("missing label and fact must fail closed")
	}
}

func TestJudgedOutcomeKeepsNonFactFailures(t *testing.T) {
	factsOK := verdict{Required: []bool{true}, Forbidden: []bool{false}}
	for _, tc := range []struct {
		name    string
		metrics map[string]float64
		want    bool
	}{
		{"facts were the only failure", map[string]float64{"outcome_success": 0, "artifact_success": 0, "workflow_success": 1, "oracle_pass": 1}, true},
		{"oracle failed", map[string]float64{"outcome_success": 0, "oracle_pass": 0, "workflow_success": 1}, false},
		{"forbidden event", map[string]float64{"outcome_success": 0, "workflow_success": 1, "forbidden_event_attempts": 1}, false},
		{"too long", map[string]float64{"outcome_success": 0, "workflow_success": 1, "within_max_words": 0}, false},
	} {
		if got := judgedOutcome(row{Metrics: tc.metrics}, studyCase{Kind: "workflow"}, factsOK); got != tc.want {
			t.Errorf("%s: judged = %v, want %v", tc.name, got, tc.want)
		}
	}
	if judgedOutcome(row{Metrics: map[string]float64{"outcome_success": 1, "workflow_success": 1}}, studyCase{Kind: "workflow"}, verdict{Required: []bool{false}}) {
		t.Error("a literal pass the judge finds missing a fact must fail")
	}
}

func TestShuffleIsSeededAndBlind(t *testing.T) {
	rows := []row{{RunID: "a", Arm: "treatment"}, {RunID: "b", Arm: "control"}, {RunID: "c", Arm: "treatment"}, {RunID: "d", Arm: "control"}}
	first := labelItems(rows, map[string]string{"a": "1", "b": "2", "c": "3", "d": "4"}, 7)
	second := labelItems(rows, map[string]string{"a": "1", "b": "2", "c": "3", "d": "4"}, 7)
	for i := range first {
		if first[i].Row.RunID != second[i].Row.RunID || first[i].Label != "R"+string(rune('1'+i)) {
			t.Fatalf("labels not deterministic: %+v vs %+v", first, second)
		}
	}
}

// The runner's outcome omits workflow_success, and bounded_return and the
// dispatch contract count only for output-only orchestration.
func TestJudgedOutcomeFollowsTheRunnerOutcomePerKind(t *testing.T) {
	factsOK := verdict{Required: []bool{true}}
	fanout := row{Metrics: map[string]float64{"outcome_success": 0, "workflow_success": 0, "bounded_return": 0, "dispatch_contract_success": 0, "complete_trace": 1}}
	if !judgedOutcome(fanout, studyCase{Kind: "orchestration", OrchestrationMode: "fanout"}, factsOK) {
		t.Error("fanout must not require bounded_return, dispatch, or workflow_success")
	}
	outputOnly := row{Metrics: map[string]float64{"outcome_success": 0, "bounded_return": 0, "complete_trace": 1, "dispatch_contract_success": 1}}
	if judgedOutcome(outputOnly, studyCase{Kind: "orchestration", OrchestrationMode: "output_only"}, factsOK) {
		t.Error("output_only must require bounded_return")
	}
	if judgedOutcome(row{Metrics: map[string]float64{"complete_trace": 0}}, studyCase{Kind: "workflow"}, factsOK) {
		t.Error("an incomplete trace must fail")
	}
}

// A saved verdict is reused only when the saved prompt is byte-identical, so
// labels and fact numbering are guaranteed to match.
func TestJudgeCaseReusesVerdictForIdenticalPrompt(t *testing.T) {
	out := t.TempDir()
	c := studyCase{ID: "c1", Task: "t", RequiredFacts: []string{"a"}}
	items := []item{{Label: "R1", Response: "a"}}
	if err := os.WriteFile(filepath.Join(out, "prompt-c1.txt"), []byte(buildPrompt(c, items)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "verdict-c1.txt"), []byte(`{"R1":{"required":{"F1":true},"forbidden":{}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	verdicts, err := judgeCase("false {prompt_file}", out, c, items, true)
	if err != nil || !verdicts["R1"].pass() {
		t.Fatalf("reuse failed: %v %+v", err, verdicts)
	}
	if _, err := judgeCase("false {prompt_file}", out, c, []item{{Label: "R1", Response: "changed"}}, true); err == nil {
		t.Fatal("a changed prompt must call the judge again")
	}
}

// Markup facts such as "**" or an em dash are not claims; they stay literal
// even when the judge calls them unasserted.
func TestMarkupForbiddenFactsStayLiteral(t *testing.T) {
	c := studyCase{ForbiddenFacts: []string{"**", "excited", "—"}}
	v := verdict{Forbidden: []bool{false, false, false}}
	applyLiteralMarkup(c, "**Heads up** — freeze starts", &v)
	if !v.Forbidden[0] || v.Forbidden[1] || !v.Forbidden[2] {
		t.Fatalf("forbidden = %v, want markup literal and claims judged", v.Forbidden)
	}
}
