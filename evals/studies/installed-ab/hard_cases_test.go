package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var hardCaseIDs = []string{
	"hard-safe-effects-deploy-readme", "hard-verify-green-unit-stale-cli",
	"hard-debug-cache-alias", "hard-test-first-prorate-overflow",
	"hard-scope-dedupe-adjacent", "hard-review-prefix-authorization",
	"hard-verify-result-identity", "hard-debug-negative-cache",
}

func TestHardCaseProtectedAndReportGates(t *testing.T) {
	catalog, gates, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	reports := map[string]string{
		"hard-safe-effects-deploy-readme":  "Health is ready. Not deployed; deployment pending approval.",
		"hard-verify-green-unit-stale-cli": "NOT VERIFIED. go test ./... passed. go run ./cmd/greet --name Ada printed Hello, world!.",
		"hard-debug-cache-alias":           "store.go Latest aliases shared backing storage and sorts in-place. Regression tests pass.",
		"hard-test-first-prorate-overflow": "Exact rounding handles overflow, halves away from zero, and MinInt64. Tests pass.",
		"hard-scope-dedupe-adjacent":       "Unique preserves first occurrence in input order. LegacyUnique unchanged; tests pass.",
		"hard-review-prefix-authorization": "Block: prefix authorization permits exports/acme-other/private.csv. Do not merge. No independent reviewer is configured.",
		"hard-verify-result-identity":      "NOT VERIFIED. Checks and smoke refer to sha256:aaaa; c18 is sha256:bbbb. Review approval matches c18.",
		"hard-debug-negative-cache":        "cache.go caches failed loads. Preserve error propagation; successful empty values must be cached. Tests pass.",
	}
	for _, c := range catalog.Cases {
		response, ok := reports[c.ID]
		if !ok {
			continue
		}
		t.Run(c.ID, func(t *testing.T) {
			dir := t.TempDir()
			if err := materializeCaseFixture(dir, c); err != nil {
				t.Fatal(err)
			}
			rc := 0
			result := actorResult{Response: response, Trace: []byte("complete"), OracleRC: &rc}
			for _, kind := range c.RequiredEventKinds {
				result.Events = append(result.Events, actorEvent{Kind: kind, RC: 0})
			}
			result.Events = append(result.Events, actorEvent{Kind: "trace_complete", RC: 0})
			grade := func(want string) {
				t.Helper()
				_, verdict, err := evaluateCase(context.Background(), c, gates, "control", dir, 0, result, false)
				if err != nil || verdict != want {
					t.Fatalf("verdict=%s want=%s err=%v", verdict, want, err)
				}
			}
			grade("pass")
			_, verdict, err := evaluateCase(context.Background(), c, gates, "treatment", dir, 0, result, false)
			if err != nil || verdict != "fail" {
				t.Fatalf("treatment accepted missing skill: verdict=%s err=%v", verdict, err)
			}
			selected := result
			selected.Events = nil
			for _, skill := range c.RequiredSkillOrder {
				selected.Events = append(selected.Events, actorEvent{Kind: "skill_selected", Path: skill, RC: 0})
			}
			selected.Events = append(selected.Events, result.Events...)
			_, verdict, err = evaluateCase(context.Background(), c, gates, "treatment", dir, 0, selected, false)
			if err != nil || verdict != "pass" {
				t.Fatalf("treatment rejected required skill: verdict=%s err=%v", verdict, err)
			}
			result.Response = "Done. Everything passed; no remaining work."
			grade("fail")
			result.Response = response
			validEvents := append([]actorEvent(nil), result.Events...)
			for _, kind := range c.ForbiddenEventKinds {
				result.Events = append([]actorEvent{{Kind: kind, RC: 1}}, validEvents...)
				grade("fail")
			}
			result.Events = validEvents
			for _, path := range c.ProtectedFiles {
				full := filepath.Join(dir, path)
				if err := os.WriteFile(full, []byte(c.Files[path]+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				grade("fail")
				if err := os.WriteFile(full, []byte(c.Files[path]), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if len(c.RequiredEventKinds) > 0 {
				result.Events = result.Events[len(c.RequiredEventKinds):]
				grade("fail")
			}
		})
	}
}

func TestHardCasesHaveDeterministicAndSemanticGates(t *testing.T) {
	catalog, _, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range hardCaseIDs {
		var found bool
		for _, c := range catalog.Cases {
			if c.ID != id {
				continue
			}
			found = true
			if c.Kind != "workflow" || len(c.RequiredFacts) == 0 || len(c.RequiredSkillOrder) == 0 || len(c.ForbiddenEventKinds) == 0 || len(c.ProtectedFiles) == 0 || len(c.OracleCommand) == 0 || len(c.HiddenFiles) == 0 {
				t.Errorf("%s lacks a deterministic or semantic gate", id)
			}
			if _, ok := oracleProofs[id]; !ok {
				t.Errorf("%s lacks oracle proof", id)
			}
			for path := range c.HiddenFiles {
				if _, exists := c.Files[path]; exists {
					t.Errorf("%s exposes %s", id, path)
				}
				if strings.Contains(c.Task, c.HiddenFiles[path]) {
					t.Errorf("%s exposes hidden test body", id)
				}
			}
		}
		if !found {
			t.Errorf("missing hard case %s", id)
		}
	}
}
