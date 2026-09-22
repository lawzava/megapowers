package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestActivationRequiresMembershipNotReadOrder(t *testing.T) {
	required := []string{"design-and-plan", "verify-and-finish"}
	read := func(skill string, rc int) actorEvent { return actorEvent{Kind: "skill_selected", Path: skill, RC: rc} }
	for _, tc := range []struct {
		name   string
		events []actorEvent
		want   bool
	}{
		{"declared order", []actorEvent{read(required[0], 0), read(required[1], 0)}, true},
		{"reversed reads", []actorEvent{read(required[1], 0), read(required[0], 0)}, true},
		{"nonadjacent repeated reads", []actorEvent{read(required[0], 0), read(required[1], 0), read(required[0], 0)}, true},
		{"missing", []actorEvent{read(required[0], 0)}, false},
		{"extra", []actorEvent{read(required[0], 0), read(required[1], 0), read("humanizing-prose", 0)}, false},
		{"forbidden", []actorEvent{read(required[0], 0), read(required[1], 0), read("safe-effects", 0)}, false},
		{"failed then successful", []actorEvent{read(required[0], 1), read(required[0], 0), read(required[1], 0)}, false},
		{"empty failed selection", []actorEvent{read(required[0], 0), read(required[1], 0), read("", 1)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			present, unexpected := skillSelectionEvidence(tc.events, required, []string{"safe-effects"})
			if got := present && unexpected == 0; got != tc.want {
				t.Fatalf("activation = %t (present %t, unexpected %d), want %t", got, present, unexpected, tc.want)
			}
		})
	}
}

func TestReversedSkillReadsPreserveOutcomeAndOrderDiagnostic(t *testing.T) {
	c := studyCase{Kind: "prose", RequiredFacts: []string{"kept"}, RequiredSkillOrder: []string{"design-and-plan", "verify-and-finish"}}
	result := actorResult{Response: "kept", Events: []actorEvent{
		{Kind: "skill_selected", Path: "verify-and-finish"},
		{Kind: "skill_selected", Path: "design-and-plan"},
	}}
	metrics, verdict, err := evaluateCase(context.Background(), c, gatesFile{}, "treatment", t.TempDir(), 0, result, true)
	if err != nil || verdict != "pass" || metrics["skill_contract_success"] != 1 || metrics["skill_membership"] != 1 || metrics["skill_order"] != 0 || metrics["outcome_success"] != 1 {
		t.Fatalf("reversed reads: verdict %s, metrics %v, error %v", verdict, metrics, err)
	}
}

func TestEfficiencyReportsKnownAndMissingTraceUsage(t *testing.T) {
	for _, tc := range []struct {
		name, trace string
		want        map[string]float64
	}{
		{"missing", "", map[string]float64{"usage_input_cached_tokens_known": 0, "usage_input_uncached_tokens_known": 0, "usage_output_tokens_known": 0}},
		{"valid trace without usage", `{"type":"turn.completed"}`, map[string]float64{"usage_trace_complete": 1, "usage_output_tokens_known": 0}},
		{"truncated trace", "{\"type\":\"turn.completed\",\"usage\":{\"output_tokens\":12}}\n" + `{"padding":"` + strings.Repeat("x", 8<<20) + `"}`, map[string]float64{"usage_trace_complete": 0, "usage_output_tokens_known": 0}},
		{"malformed trailing trace", "{\"type\":\"turn.completed\",\"usage\":{\"output_tokens\":12}}\n{", map[string]float64{"usage_trace_complete": 0, "usage_output_tokens_known": 0}},
		{"child starts first", "{\"method\":\"thread/started\",\"params\":{\"thread\":{\"id\":\"child\",\"parentThreadId\":\"root\"}}}\n" +
			"{\"method\":\"thread/tokenUsage/updated\",\"params\":{\"threadId\":\"child\",\"tokenUsage\":{\"total\":{\"outputTokens\":900}}}}\n" +
			"{\"method\":\"thread/started\",\"params\":{\"thread\":{\"id\":\"root\"}}}\n" +
			`{"method":"thread/tokenUsage/updated","params":{"threadId":"root","tokenUsage":{"total":{"outputTokens":12}}}}`, map[string]float64{"usage_output_tokens": 12}},
		{"codex exec", `{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":30,"output_tokens":12}}`, map[string]float64{"usage_input_cached_tokens": 30, "usage_input_uncached_tokens": 70, "usage_output_tokens": 12, "usage_input_cached_tokens_known": 1}},
		{"partial", `{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":12}}`, map[string]float64{"usage_input_cached_tokens_known": 0, "usage_input_uncached_tokens_known": 0, "usage_output_tokens": 12}},
		{"reported zero", `{"type":"turn.completed","usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0}}`, map[string]float64{"usage_input_cached_tokens": 0, "usage_input_uncached_tokens": 0, "usage_output_tokens": 0, "usage_output_tokens_known": 1}},
		{"fractional usage", `{"type":"turn.completed","usage":{"input_tokens":1.5,"cached_input_tokens":0,"output_tokens":1.5}}`, map[string]float64{"usage_input_uncached_tokens_known": 0, "usage_output_tokens_known": 0}},
		{"latest missing usage", "{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":100,\"cached_input_tokens\":30,\"output_tokens\":12}}\n" + `{"type":"turn.completed"}`, map[string]float64{"usage_output_tokens_known": 0}},
		{"unbound root", `{"method":"thread/tokenUsage/updated","params":{"threadId":"unknown","tokenUsage":{"total":{"inputTokens":200,"cachedInputTokens":60,"outputTokens":24}}}}`, map[string]float64{"usage_output_tokens_known": 0}},
		{"invalid", `{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":30,"output_tokens":-1}}`, map[string]float64{"usage_input_cached_tokens_known": 0, "usage_input_uncached_tokens_known": 0, "usage_output_tokens_known": 0}},
		{"claude root", `{"type":"result","usage":{"input_tokens":10,"cache_creation_input_tokens":20,"cache_read_input_tokens":30,"output_tokens":12}}`, map[string]float64{"usage_input_cached_tokens": 30, "usage_input_uncached_tokens": 30, "usage_output_tokens": 12}},
		{"claude forwarded excluded", `{"type":"result","origin":{"kind":"task-notification"},"usage":{"input_tokens":10,"cache_creation_input_tokens":20,"cache_read_input_tokens":30,"output_tokens":12}}`, map[string]float64{"usage_output_tokens_known": 0}},
		{"codex root snapshots", "{\"method\":\"thread/started\",\"params\":{\"thread\":{\"id\":\"root\"}}}\n" +
			"{\"method\":\"thread/tokenUsage/updated\",\"params\":{\"threadId\":\"root\",\"tokenUsage\":{\"total\":{\"inputTokens\":100,\"cachedInputTokens\":30,\"outputTokens\":12}}}}\n" +
			"{\"method\":\"thread/tokenUsage/updated\",\"params\":{\"threadId\":\"child\",\"tokenUsage\":{\"total\":{\"inputTokens\":900,\"cachedInputTokens\":300,\"outputTokens\":120}}}}\n" +
			`{"method":"thread/tokenUsage/updated","params":{"threadId":"root","tokenUsage":{"total":{"inputTokens":200,"cachedInputTokens":60,"outputTokens":24}}}}`, map[string]float64{"usage_input_cached_tokens": 60, "usage_input_uncached_tokens": 140, "usage_output_tokens": 24}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := studyCase{Kind: "prose", RequiredFacts: []string{"kept"}}
			metrics, verdict, err := evaluateCase(context.Background(), c, gatesFile{}, "control", t.TempDir(), 0, actorResult{Response: "kept", Trace: []byte(tc.trace)}, true)
			if err != nil || verdict != "pass" {
				t.Fatalf("telemetry changed outcome: %s %v", verdict, err)
			}
			for key, want := range tc.want {
				if got, exists := metrics[key]; !exists || got != want {
					t.Errorf("%s = %v (present %t), want %v", key, got, exists, want)
				}
			}
			for _, name := range []string{"input_cached_tokens", "input_uncached_tokens", "output_tokens"} {
				if metrics["usage_"+name+"_known"] == 0 {
					if _, exists := metrics["usage_"+name]; exists {
						t.Errorf("unknown %s published as a count", name)
					}
				}
			}
		})
	}
}

func TestDurationAvailabilityMatchesPublishedMilliseconds(t *testing.T) {
	metrics := map[string]float64{}
	attachEfficiencyMetrics(metrics, actorResult{Duration: time.Nanosecond})
	if metrics["duration_ms_known"] != 0 {
		t.Fatalf("submillisecond duration publishes zero milliseconds but claims known: %v", metrics)
	}
}
