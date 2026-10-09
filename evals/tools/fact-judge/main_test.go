package main

import (
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
		if got := judgedOutcome(row{Metrics: tc.metrics}, factsOK); got != tc.want {
			t.Errorf("%s: judged = %v, want %v", tc.name, got, tc.want)
		}
	}
	if judgedOutcome(row{Metrics: map[string]float64{"outcome_success": 1, "workflow_success": 1}}, verdict{Required: []bool{false}}) {
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
