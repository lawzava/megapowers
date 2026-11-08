package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHardImplementationCasesAllowRequestedRegressionTests(t *testing.T) {
	catalog, _, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]string{
		"hard-scope-dedupe-adjacent":       "unique_test.go",
		"hard-test-first-prorate-overflow": "prorate_test.go",
		"hard-debug-cache-alias":           "store_test.go",
		"hard-debug-negative-cache":        "cache_test.go",
	}
	seen := map[string]bool{}
	for _, c := range catalog.Cases {
		path, ok := tests[c.ID]
		if !ok {
			continue
		}
		seen[c.ID] = true
		t.Run(c.ID, func(t *testing.T) {
			dir := t.TempDir()
			if err := materializeCaseFixture(dir, c); err != nil {
				t.Fatal(err)
			}
			body := c.Files[path] + "\nfunc TestAddedRegression(t *testing.T) {}\n"
			if err := os.WriteFile(filepath.Join(dir, path), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			intact, err := protectedFilesIntact(dir, c)
			if err != nil || !intact {
				t.Fatalf("requested test edit rejected: intact=%t err=%v", intact, err)
			}
		})
	}
	for id := range tests {
		if !seen[id] {
			t.Errorf("missing regression case %s", id)
		}
	}
}

func TestHardReviewAllowsScratchEvidenceButProtectsCandidate(t *testing.T) {
	catalog, gates, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	var c studyCase
	for _, candidate := range catalog.Cases {
		if candidate.ID == "hard-review-prefix-authorization" {
			c = candidate
		}
	}
	if c.ID == "" {
		t.Fatal("missing review case")
	}
	dir := t.TempDir()
	if err := materializeCaseFixture(dir, c); err != nil {
		t.Fatal(err)
	}
	rc := 0
	result := actorResult{Response: "Do not merge: prefix authorization allows exports/acme-other/private.csv. No independent reviewer is available.", Trace: []byte("complete"), OracleRC: &rc, Events: []actorEvent{
		{Kind: "write", Path: filepath.Join(t.TempDir(), "counterexample.go"), RC: 0},
		{Kind: "trace_complete", RC: 0},
	}}
	grade := func(want string) {
		t.Helper()
		_, verdict, err := evaluateCase(context.Background(), c, gates, "control", dir, 0, result, false)
		if err != nil || verdict != want {
			t.Fatalf("verdict=%s want=%s err=%v", verdict, want, err)
		}
	}
	grade("pass")
	if err := os.WriteFile(filepath.Join(dir, "auth.go"), []byte(c.Files["auth.go"]+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	grade("fail")
}
