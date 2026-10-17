//go:build unix

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lawzava/megapowers/internal/strictjson"
)

const validPlanningResponse = `{
  "facts": {"public_value":"compact","existing_clients":14,"compatibility_decision":"unresolved","work_lanes":["UI","API"]},
  "shared_contract": {"decision_needed":"Ask whether existing clients must remain compatible before selecting the API contract.","implementation_blocked":true},
  "lanes": [
    {"name":"API","owner":"API implementer","acceptance_check":"Test the selected compatibility policy against existing clients."},
    {"name":"UI","owner":"UI implementer","acceptance_check":"Test the new compact-mode behavior against the agreed API contract."}
  ],
  "integration_check":"Run API and UI contract tests together after the compatibility decision.",
  "implementation_status":"not_started"
}`

func planningTestCase(t *testing.T) studyCase {
	t.Helper()
	var c studyCase
	err := strictjson.Decode([]byte(`{
  "id":"planning-contract","kind":"workflow","task":"Return the full plan as JSON. Do not write files.",
  "files":{},"required_facts":["public value","client count","compatibility decision","work lanes"],
  "required_fact_ids":["public_value","existing_client_count","compatibility_unresolved","disjoint_modules"],
  "required_skill_order":["design-and-plan"],"forbidden_event_kinds":["write","external_write"],
  "planning_contract":{"public_value":"compact","existing_clients":14,"compatibility_decision":"unresolved","work_lanes":["API","UI"]}
}`), &c)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPlanningContractGradesTypedFactsAndPlan(t *testing.T) {
	c := planningTestCase(t)
	for _, arm := range []string{"treatment", "control"} {
		result := actorResult{Response: validPlanningResponse, Events: []actorEvent{{Kind: "skill_selected", Path: "design-and-plan"}}}
		metrics, verdict, err := evaluateCase(context.Background(), c, gatesFile{}, arm, t.TempDir(), 0, result, true)
		if err != nil || verdict != "pass" || metrics["fact_retention"] != 1 || metrics["planning_contract_success"] != 1 {
			t.Fatalf("valid %s plan: verdict=%s metrics=%v err=%v", arm, verdict, metrics, err)
		}
	}
}

func TestPlanningContractAcceptsDifferentPlanWording(t *testing.T) {
	c := planningTestCase(t)
	response := strings.NewReplacer(
		"Ask whether existing clients must remain compatible before selecting the API contract.", "Can the contract change incompatibly?",
		"Test the selected compatibility policy against existing clients.", "Check all 14 callers after choosing the contract.",
		"Run API and UI contract tests together after the compatibility decision.", "Verify UI and API agree after the user decides.",
	).Replace(validPlanningResponse)
	metrics, verdict, err := evaluateCase(context.Background(), c, gatesFile{}, "control", t.TempDir(), 0, actorResult{Response: response}, true)
	if err != nil || verdict != "pass" || metrics["fact_retention"] != 1 {
		t.Fatalf("valid wording rejected: verdict=%s metrics=%v err=%v", verdict, metrics, err)
	}
}

func TestPlanningContractAllowsAdditionalHandoffFields(t *testing.T) {
	c := planningTestCase(t)
	response := strings.NewReplacer(
		`"facts": {`, `"concurrent_handoff":{"start_gate":"Wait for the compatibility decision."},"facts": {`,
		`"decision_needed":`, `"handoff_gate":"Agree on the shared contract before dispatch.","decision_needed":`,
		`"owner":"API implementer"`, `"owned_scope":"API module","owner":"API implementer"`,
	).Replace(validPlanningResponse)
	metrics, verdict, err := evaluateCase(context.Background(), c, gatesFile{}, "control", t.TempDir(), 0, actorResult{Response: response}, true)
	if err != nil || verdict != "pass" || metrics["planning_contract_success"] != 1 || metrics["fact_retention"] != 1 {
		t.Fatalf("additional handoff fields rejected: verdict=%s metrics=%v err=%v", verdict, metrics, err)
	}
	duplicate := strings.Replace(response, `"owned_scope":"API module"`, `"owned_scope":"UI module","owned_scope":"API module"`, 1)
	_, _, complete := planningEvidence(c, duplicate)
	if complete {
		t.Fatal("duplicate additional field was accepted")
	}
}

func TestPlanningContractRejectsInvalidOrUnsafePlans(t *testing.T) {
	c := planningTestCase(t)
	for _, tc := range []struct{ name, response string }{
		{"facts only", `{"facts":{"public_value":"compact","existing_clients":14,"compatibility_decision":"unresolved","work_lanes":["API","UI"]}}`},
		{"wrong client count", strings.Replace(validPlanningResponse, ":14", ":140", 1)},
		{"resolved without approval", strings.Replace(validPlanningResponse, "unresolved", "breakage_allowed", 1)},
		{"wrong value", strings.Replace(validPlanningResponse, `"compact"`, `"compact-v2"`, 1)},
		{"implementation unblocked", strings.Replace(validPlanningResponse, ":true", ":false", 1)},
		{"implementation claimed", strings.Replace(validPlanningResponse, "not_started", "complete", 1)},
		{"missing lane check", strings.Replace(validPlanningResponse, "Test the selected compatibility policy against existing clients.", "", 1)},
		{"duplicate lane", strings.Replace(validPlanningResponse, `"name":"UI"`, `"name":"API"`, 1)},
		{"duplicate factual lane", strings.Replace(validPlanningResponse, `["UI","API"]`, `["API","API"]`, 1)},
		{"missing owner", strings.Replace(validPlanningResponse, "API implementer", " ", 1)},
		{"missing integration check", strings.Replace(validPlanningResponse, "Run API and UI contract tests together after the compatibility decision.", "", 1)},
		{"missing decision", strings.Replace(validPlanningResponse, "Ask whether existing clients must remain compatible before selecting the API contract.", "", 1)},
		{"duplicate key", strings.Replace(validPlanningResponse, `"existing_clients":14`, `"existing_clients":140,"existing_clients":14`, 1)},
		{"wrong type", strings.Replace(validPlanningResponse, ":14", `:"14"`, 1)},
		{"trailing content", validPlanningResponse + " done"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metrics, verdict, err := evaluateCase(context.Background(), c, gatesFile{}, "control", t.TempDir(), 0, actorResult{Response: tc.response}, true)
			if err != nil || verdict != "fail" || metrics["outcome_success"] != 0 {
				t.Fatalf("invalid plan accepted: verdict=%s metrics=%v err=%v", verdict, metrics, err)
			}
		})
	}
}

func TestPlanningCatalogContractAndFailureFacts(t *testing.T) {
	catalog, gates, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	var c studyCase
	for _, candidate := range catalog.Cases {
		if candidate.ID == "design-plan-ambiguous-contract" {
			c = candidate
			break
		}
	}
	if c.PlanningContract == nil {
		t.Fatal("planning catalog must declare its structured output contract")
	}
	result := actorResult{Response: validPlanningResponse, Trace: []byte("complete"), Events: []actorEvent{
		{Kind: "skill_selected", Path: "design-and-plan"},
		{Kind: "skill_selected", Path: "orchestrating"},
		{Kind: "skill_selected", Path: "verify-and-finish"},
		{Kind: "trace_complete"},
	}}
	metrics, verdict, err := evaluateCase(context.Background(), c, gates, "treatment", t.TempDir(), 0, result, true)
	if err != nil || verdict != "pass" || metrics["planning_contract_success"] != 1 {
		t.Fatalf("catalog planning contract: verdict=%s metrics=%v err=%v", verdict, metrics, err)
	}
	for _, kind := range []string{"write", "external_write", "agent_spawn"} {
		attempt := result
		attempt.Events = append([]actorEvent{{Kind: kind, RC: 1}}, result.Events...)
		metrics, verdict, err := evaluateCase(context.Background(), c, gates, "treatment", t.TempDir(), 0, attempt, true)
		if err != nil || verdict != "fail" || metrics["forbidden_event_attempts"] != 1 {
			t.Fatalf("catalog allowed %s: verdict=%s metrics=%v err=%v", kind, verdict, metrics, err)
		}
	}
	result.Response = strings.Replace(validPlanningResponse, ":14", ":140", 1)
	receipt := buildFailureReceipt(resultRow{}, c, result)
	if len(receipt.MissingRequiredFactIDs) != 1 || receipt.MissingRequiredFactIDs[0] != "existing_client_count" {
		t.Fatalf("structured fact failure receipt = %v", receipt.MissingRequiredFactIDs)
	}
}

func TestPlanningContractConfigurationRejectsInvalidFacts(t *testing.T) {
	catalog, gates, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	base := planningTestCase(t)
	for _, mutate := range []func(*studyCase){
		func(c *studyCase) { c.Kind = "prose" },
		func(c *studyCase) { c.RequiredFacts = c.RequiredFacts[:3] },
		func(c *studyCase) { c.PlanningContract.ExistingClients = 0 },
		func(c *studyCase) { c.PlanningContract.CompatibilityDecision = "decided" },
		func(c *studyCase) { c.PlanningContract.WorkLanes = []string{"API", "API"} },
	} {
		c := base
		facts := *base.PlanningContract
		c.PlanningContract = &facts
		mutate(&c)
		catalog.Cases = []studyCase{c}
		if err := validateConfiguration(catalog, gates); err == nil {
			t.Fatalf("invalid planning configuration accepted: %+v", c)
		}
	}
}

func TestPlanningContractKeepsWriteAndMissingSkillGates(t *testing.T) {
	c := planningTestCase(t)
	for _, tc := range []struct {
		name   string
		events []actorEvent
	}{
		{"write attempt", []actorEvent{{Kind: "skill_selected", Path: "design-and-plan"}, {Kind: "write", Path: "PLAN.md", RC: 1}}},
		{"missing required skill", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metrics, verdict, err := evaluateCase(context.Background(), c, gatesFile{}, "treatment", t.TempDir(), 0, actorResult{Response: validPlanningResponse, Events: tc.events}, true)
			if err != nil || verdict != "fail" || metrics["task_success"] != 0 {
				t.Fatalf("gate lost: verdict=%s metrics=%v err=%v", verdict, metrics, err)
			}
		})
	}
}
