//go:build unix

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func savedRescoreRun(t *testing.T, withChild ...bool) (casesFile, gatesFile, string) {
	t.Helper()
	_, gates, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	c := studyCase{ID: "saved", Kind: "workflow", Task: "Check locally.", RequiredFacts: []string{"local only"}, RequiredEventKinds: []string{"test"}, OracleCommand: []string{"must-not-execute"}, ProtectedFiles: []string{"fixture"}}
	catalog := casesFile{SchemaVersion: "1", Cases: []studyCase{c}}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "publish"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "private/judge"), 0700); err != nil {
		t.Fatal(err)
	}
	trace := []byte(`{"type":"tool_use","part":{"id":"test","tool":"shell","state":{"status":"completed","input":{"command":"go test ./...; echo exit=$?"},"output":"exit=0\n","metadata":{"exit":0}}}}
{"type":"text","part":{"text":"local"}}
{"type":"step_finish","part":{"reason":"stop"}}
`)
	if len(withChild) > 0 && withChild[0] {
		spawn := bytes.SplitN(openCodeSpawnTrace("subagent", "ses_child1"), []byte("\n"), 2)[0]
		trace = append(append(spawn, '\n'), trace...)
	}
	row := resultRow{CaseID: c.ID, RunID: "saved-run", BlockID: "1", Arm: "control", Status: "completed", Harness: harnessIdentity{Name: "opencode", Model: "claude-agent/opus"}, PromptHash: promptHash(c), FixtureHash: caseFixtureHash(c), Metrics: map[string]float64{"workflow_success": 0, "oracle_pass": 1, "protected_fixture_intact": 1}, Artifacts: map[string]string{"response": hashBytes([]byte("local")), "trace": hashBytes(trace)}}
	attachEvidenceArtifacts(&row)
	// A previous interrupted attempt must not replace the hash-matching evidence.
	if err := writePrivateResponse(dir, row, "old", []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateResponse(dir, row, "local", trace); err != nil {
		t.Fatal(err)
	}
	if len(withChild) > 0 && withChild[0] {
		if err := writePrivateChildTraces(dir, &row, map[string][]byte{"ses_child1": []byte(openCodeChildExport)}); err != nil {
			t.Fatal(err)
		}
	}
	rows, _ := encodeResultRows([]resultRow{row})
	if err := os.WriteFile(filepath.Join(dir, "publish/results.jsonl"), rows, 0600); err != nil {
		t.Fatal(err)
	}
	ch, gh, _ := configurationHashes(catalog, gates)
	manifest, _ := json.Marshal(publishManifest{CaseCatalogHash: ch, GatesHash: gh, Arms: []armManifest{{CaseID: c.ID, BlockID: row.BlockID, Arm: row.Arm, OpenCode: &openCodeTelemetry{ActivationValidation: "native-tool-events"}}}})
	if err := os.WriteFile(filepath.Join(dir, "publish/manifest.json"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
	verdict := `{"run_id":"saved-run","judge_model":"openai/gpt-6.1-sol-fast#high","attempts":1,"status":"judged","verdict":{"required":[{"met":true,"quote":"local"}],"forbidden":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "private/judge/0001-verdict.json"), []byte(verdict), 0600); err != nil {
		t.Fatal(err)
	}
	judgeManifest, _ := json.Marshal(map[string]any{"seed": 1, "input_results_hash": hashBytes(rows), "case_catalog_hash": ch, "row_order": []string{row.RunID}, "judge_errors": 0, "response_leak_counts": map[string]int{row.RunID: 0}})
	if err := os.WriteFile(filepath.Join(dir, "private/judge/manifest.json"), judgeManifest, 0600); err != nil {
		t.Fatal(err)
	}
	return catalog, gates, dir
}

func TestRescoreSavedOpenCodeRun(t *testing.T) {
	catalog, gates, dir := savedRescoreRun(t)
	original, _ := os.ReadFile(filepath.Join(dir, "publish/results.jsonl"))
	if err := rescoreRun(context.Background(), catalog, gates, dir); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"rescored-results.jsonl", "rescored-judged-results.jsonl"} {
		data, err := os.ReadFile(filepath.Join(dir, "publish", file))
		if err != nil {
			t.Fatal(err)
		}
		var row resultRow
		if err := json.Unmarshal(bytes.TrimSpace(data), &row); err != nil {
			t.Fatal(err)
		}
		if row.Metrics["workflow_success"] != 1 || row.Metrics["test_attempts"] != 1 || row.Metrics["oracle_pass"] != 1 || row.Metrics["protected_fixture_intact"] != 1 || row.Metrics["outcome_success"] != 0 {
			t.Fatalf("bad rescore: %+v", row.Metrics)
		}
		if strings.Contains(file, "judged") && (row.Metrics["judge_fact_pass"] != 1 || row.Metrics["judge_outcome_success"] != 1) {
			t.Fatalf("judge not merged: %+v", row.Metrics)
		}
		if !rowEvidenceArtifactsValid(row) {
			t.Fatal("stale evidence hashes")
		}
	}
	after, _ := os.ReadFile(filepath.Join(dir, "publish/results.jsonl"))
	if !bytes.Equal(original, after) {
		t.Fatal("original overwritten")
	}
	if err := rescoreRun(context.Background(), catalog, gates, dir); err == nil {
		t.Fatal("overwrote rescore")
	}
}

func TestRescoreSavedOpenCodeChildren(t *testing.T) {
	for _, mutation := range []string{"none", "missing", "tampered"} {
		t.Run(mutation, func(t *testing.T) {
			catalog, gates, dir := savedRescoreRun(t, true)
			paths, _ := filepath.Glob(filepath.Join(dir, "private/traces/*.child-*.json"))
			if len(paths) != 1 {
				t.Fatalf("child traces: %v", paths)
			}
			switch mutation {
			case "missing":
				if err := os.Remove(paths[0]); err != nil {
					t.Fatal(err)
				}
			case "tampered":
				if err := os.WriteFile(paths[0], []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err := rescoreRun(context.Background(), catalog, gates, dir)
			if mutation != "none" {
				if err == nil {
					t.Fatal("rescore silently dropped unseen child actions")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "publish/rescored-results.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			var row resultRow
			if err := json.Unmarshal(bytes.TrimSpace(data), &row); err != nil {
				t.Fatal(err)
			}
			if row.Metrics["agent_spawn_count"] != 1 || row.Metrics["test_attempts"] != 2 || row.Metrics["write_attempts"] != 1 {
				t.Fatalf("rescore dropped child evidence: %v", row.Metrics)
			}
			data, err = os.ReadFile(filepath.Join(dir, "publish/rescored-judged-results.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			row = resultRow{}
			if err := json.Unmarshal(bytes.TrimSpace(data), &row); err != nil {
				t.Fatal(err)
			}
			if row.Artifacts["child_trace_ses_child1"] != hashBytes([]byte(openCodeChildExport)) {
				t.Fatalf("judged rescore lost child hash: %v", row.Artifacts)
			}
		})
	}
}

func TestRescoreRejectsInvalidSavedEvidence(t *testing.T) {
	for _, mutation := range []string{"trace", "response", "catalog", "judge", "missing-judge", "artifact", "tampered-metric", "judge-manifest", "manifest-arm", "nil-metrics", "duplicate-row", "broker-row", "incomplete-row"} {
		t.Run(mutation, func(t *testing.T) {
			catalog, gates, dir := savedRescoreRun(t)
			switch mutation {
			case "trace", "response":
				kind, ext := "traces", ".jsonl"
				if mutation == "response" {
					kind, ext = "responses", ".txt"
				}
				files, _ := filepath.Glob(filepath.Join(dir, "private", kind, "*"+ext))
				for _, file := range files {
					if err := os.WriteFile(file, []byte("tampered"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "catalog":
				catalog.Cases[0].Task = "changed"
			case "judge":
				if err := os.WriteFile(filepath.Join(dir, "private/judge/0001-verdict.json"), []byte(`{"run_id":"saved-run","status":"judged","verdict":{}}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing-judge":
				if err := os.Remove(filepath.Join(dir, "private/judge/0001-verdict.json")); err != nil {
					t.Fatal(err)
				}
			case "artifact":
				path := filepath.Join(dir, "publish/results.jsonl")
				data, _ := os.ReadFile(path)
				data = bytes.ReplaceAll(data, []byte(`"oracle_pass":1,`), nil)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "tampered-metric":
				path := filepath.Join(dir, "publish/results.jsonl")
				data, _ := os.ReadFile(path)
				data = bytes.ReplaceAll(data, []byte(`"oracle_pass":1`), []byte(`"oracle_pass":0`))
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "judge-manifest":
				path := filepath.Join(dir, "private/judge/manifest.json")
				if err := os.WriteFile(path, []byte(`{"input_results_hash":"wrong"}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "manifest-arm":
				path := filepath.Join(dir, "publish/manifest.json")
				var manifest publishManifest
				if err := decodeStrict(path, &manifest); err != nil {
					t.Fatal(err)
				}
				manifest.Arms = nil
				data, _ := json.Marshal(manifest)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "nil-metrics", "duplicate-row", "broker-row", "incomplete-row":
				path := filepath.Join(dir, "publish/results.jsonl")
				data, _ := os.ReadFile(path)
				var row resultRow
				if err := json.Unmarshal(bytes.TrimSpace(data), &row); err != nil {
					t.Fatal(err)
				}
				switch mutation {
				case "nil-metrics":
					row.Metrics = nil
				case "broker-row":
					row.Harness.Name = "claude"
				case "incomplete-row":
					row.Status = "harness_error"
				}
				attachEvidenceArtifacts(&row)
				rows := []resultRow{row}
				if mutation == "duplicate-row" {
					rows = append(rows, row)
				}
				data, _ = encodeResultRows(rows)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := rescoreRun(context.Background(), catalog, gates, dir); err == nil {
				t.Fatal("accepted invalid saved evidence")
			}
			if _, err := os.Stat(filepath.Join(dir, "publish/rescored-results.jsonl")); !os.IsNotExist(err) {
				t.Fatal("published partial rescore")
			}
		})
	}
}

func TestRescoreKeepsArtifactAndJudgeErrorGates(t *testing.T) {
	for _, scenario := range []string{"failed-artifact", "judge-error"} {
		t.Run(scenario, func(t *testing.T) {
			catalog, gates, dir := savedRescoreRun(t)
			if scenario == "failed-artifact" {
				path := filepath.Join(dir, "publish/results.jsonl")
				data, _ := os.ReadFile(path)
				var row resultRow
				if err := json.Unmarshal(bytes.TrimSpace(data), &row); err != nil {
					t.Fatal(err)
				}
				row.Metrics["oracle_pass"] = 0
				attachEvidenceArtifacts(&row)
				data, _ = encodeResultRows([]resultRow{row})
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(dir, "private/judge/manifest.json")
				manifestData, _ := os.ReadFile(path)
				var manifest map[string]any
				if err := json.Unmarshal(manifestData, &manifest); err != nil {
					t.Fatal(err)
				}
				manifest["input_results_hash"] = hashBytes(data)
				manifestData, _ = json.Marshal(manifest)
				if err := os.WriteFile(path, manifestData, 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				data := `{"run_id":"saved-run","judge_model":"openai/gpt-6.1-sol-fast#high","attempts":2,"status":"judge_error","error":"invalid response","verdict":{"required":null,"forbidden":null}}`
				if err := os.WriteFile(filepath.Join(dir, "private/judge/0001-verdict.json"), []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := rescoreRun(context.Background(), catalog, gates, dir); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(filepath.Join(dir, "publish/rescored-judged-results.jsonl"))
			var row resultRow
			if err := json.Unmarshal(bytes.TrimSpace(data), &row); err != nil {
				t.Fatal(err)
			}
			if row.Metrics["judge_outcome_success"] != 0 {
				t.Fatal("rescore weakened a hard gate")
			}
			if scenario == "judge-error" && (row.Metrics["judge_error"] != 1 || row.Metrics["judge_fact_pass"] != 0) {
				t.Fatal("judge error became a fact pass")
			}
		})
	}
}

func TestRescorePreservesTrustedTDDGate(t *testing.T) {
	_, gates, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	c := studyCase{Kind: "tdd", OracleCommand: []string{"must-not-execute"}}
	result := actorResult{Events: []actorEvent{
		{Kind: "write", Path: "calculator_test.go", Step: 1},
		{Kind: "test", RC: 1, Step: 2},
		{Kind: "write", Path: "calculator.go", Step: 3},
		{Kind: "test", Step: 4},
	}}
	metrics, verdict, err := evaluateCaseEvidence(context.Background(), c, gates, "control", "", 0, result, false, map[string]float64{"oracle_pass": 1, "protected_fixture_intact": 1})
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "fail" || metrics["artifact_success"] != 1 || metrics["workflow_success"] != 0 || metrics["red_before_implementation"] != 0 || metrics["test_before_implementation"] != 0 {
		t.Fatalf("untrusted native events became receipts: %+v", metrics)
	}
}
