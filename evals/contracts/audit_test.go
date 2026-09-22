package contracts

import (
	"strings"
	"testing"
)

func TestOptionalUsageDiagnosticsPreserveStrictOutcomeComparison(t *testing.T) {
	for _, tc := range []struct {
		name      string
		change    func([]map[string]any)
		wantError string
	}{
		{"known versus unknown", func(rows []map[string]any) {
			rows[0]["metrics"].(map[string]any)["usage_output_tokens"] = 12
			rows[0]["metrics"].(map[string]any)["usage_output_tokens_known"] = 1
		}, ""},
		{"known input counters", func(rows []map[string]any) {
			for _, key := range []string{"usage_input_cached_tokens", "usage_input_uncached_tokens"} {
				rows[0]["metrics"].(map[string]any)[key] = 12
				rows[0]["metrics"].(map[string]any)[key+"_known"] = 1
			}
		}, ""},
		{"fractional counter", func(rows []map[string]any) {
			rows[0]["metrics"].(map[string]any)["usage_output_tokens"] = 1.5
			rows[0]["metrics"].(map[string]any)["usage_output_tokens_known"] = 1
		}, `diagnostic "usage_output_tokens" must be a nonnegative integer token count`},
		{"nonbinary flag", func(rows []map[string]any) { rows[0]["metrics"].(map[string]any)["usage_output_tokens_known"] = 2 }, `diagnostic "usage_output_tokens" must match its binary known flag`},
		{"nonbinary duration", func(rows []map[string]any) { rows[0]["metrics"].(map[string]any)["duration_ms_known"] = 2 }, `diagnostic "duration_ms_known" must be binary`},
		{"nonbinary source", func(rows []map[string]any) { rows[0]["metrics"].(map[string]any)["usage_source_codex_latest_turn"] = 2 }, `diagnostic "usage_source_codex_latest_turn" must be binary`},
		{"nonbinary trace flag", func(rows []map[string]any) { rows[0]["metrics"].(map[string]any)["usage_trace_complete"] = 2 }, `diagnostic "usage_trace_complete" must be binary`},
		{"conflicting sources", func(rows []map[string]any) { rows[0]["metrics"].(map[string]any)["usage_source_codex_root_thread"] = 1 }, "usage source diagnostics must be mutually exclusive"},
		{"positive duration marked unknown", func(rows []map[string]any) { rows[0]["metrics"].(map[string]any)["duration_ms_known"] = 0 }, "duration_ms_known must agree with positive duration_ms"},
		{"zero duration marked known", func(rows []map[string]any) { rows[0]["duration_ms"] = 0 }, "duration_ms_known must agree with positive duration_ms"},
		{"unknown duration", func(rows []map[string]any) {
			rows[0]["duration_ms"] = 0
			rows[0]["metrics"].(map[string]any)["duration_ms_known"] = 0
		}, ""},
		{"source on incomplete trace without counts", func(rows []map[string]any) { rows[0]["metrics"].(map[string]any)["usage_trace_complete"] = 0 }, "usage source requires a completely parsed trace"},
		{"unsourced count", func(rows []map[string]any) {
			rows[0]["metrics"].(map[string]any)["usage_source_codex_latest_turn"] = 0
			rows[0]["metrics"].(map[string]any)["usage_output_tokens_known"] = 1
			rows[0]["metrics"].(map[string]any)["usage_output_tokens"] = 12
		}, `diagnostic "usage_output_tokens" requires one source and a completely parsed trace`},
		{"incomplete trace cannot carry count", func(rows []map[string]any) {
			rows[0]["metrics"].(map[string]any)["usage_trace_complete"] = 0
			rows[0]["metrics"].(map[string]any)["usage_output_tokens_known"] = 1
			rows[0]["metrics"].(map[string]any)["usage_output_tokens"] = 12
		}, "usage source requires a completely parsed trace"},
		{"unknown usage cannot carry a count", func(rows []map[string]any) { rows[0]["metrics"].(map[string]any)["usage_output_tokens"] = 12 }, `diagnostic "usage_output_tokens" must match its binary known flag`},
		{"known usage requires count", func(rows []map[string]any) { rows[0]["metrics"].(map[string]any)["usage_output_tokens_known"] = 1 }, `diagnostic "usage_output_tokens" must match its binary known flag`},
		{"outcome still required", func(rows []map[string]any) { delete(rows[0]["metrics"].(map[string]any), "fact_retention") }, "incomparable treatment/control metrics"},
		{"unrecognized diagnostic stays strict", func(rows []map[string]any) { rows[0]["metrics"].(map[string]any)["usage_extra_tokens"] = 12 }, "incomparable treatment/control metrics"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []map[string]any{baseBehavioralRow("treatment-1", "block-1", "treatment"), baseBehavioralRow("control-1", "block-1", "control")}
			for _, row := range rows {
				metrics := row["metrics"].(map[string]any)
				metrics["duration_ms_known"] = 1
				metrics["usage_trace_complete"] = 1
				metrics["usage_source_codex_latest_turn"] = 1
				metrics["usage_source_codex_root_thread"] = 0
				metrics["usage_source_claude_latest_root_result"] = 0
				for _, key := range []string{"usage_output_tokens", "usage_input_cached_tokens", "usage_input_uncached_tokens"} {
					row["metrics"].(map[string]any)[key+"_known"] = 0
				}
			}
			tc.change(rows)
			output, code := score(t, writeRows(t, rows...))
			pass := tc.wantError == ""
			if (code == 0) != pass || (!pass && !strings.Contains(output, tc.wantError)) {
				t.Fatalf("score exit %d, want error %q: %s", code, tc.wantError, output)
			}
			if pass && (strings.Contains(output, "| usage_") || strings.Contains(output, "| duration_ms_known |")) {
				t.Fatal("scorecard compares efficiency diagnostics as outcome means")
			}
		})
	}
}
