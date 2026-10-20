//go:build unix

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lawzava/megapowers/internal/strictjson"
)

const validDependencyResponse = `{
  "tasks": [
    {"id":"docs","owns":["docs/export.md"],"prerequisites":[],"consumes":[],"produces":["export flag docs"]},
    {"id":"schema","owns":["schema/columns.go"],"prerequisites":[],"consumes":[],"produces":["Column type"]},
    {"id":"writer","owns":["export/writer.go"],"prerequisites":["schema"],"consumes":["Column type"],"produces":["writer.Write"]},
    {"id":"api","owns":["api/export.go"],"prerequisites":["writer"],"consumes":["writer.Write"],"produces":["POST /export"]}
  ]
}`

func dependencyTestCase(t *testing.T) studyCase {
	t.Helper()
	var c studyCase
	err := strictjson.Decode([]byte(`{
  "id":"dependency-contract","kind":"workflow","task":"Return the plan as JSON. Do not write files.",
  "files":{},"required_facts":["schema first","writer after schema","api after writer","docs first"],
  "required_fact_ids":["schema_ready","writer_needs_schema","api_needs_writer","docs_ready"],
  "required_skill_order":["design-and-plan"],"forbidden_event_kinds":["write","external_write"],
  "dependency_contract":[
    {"id":"schema","prerequisites":[]},
    {"id":"writer","prerequisites":["schema"]},
    {"id":"api","prerequisites":["writer"]},
    {"id":"docs","prerequisites":[]}
  ]
}`), &c)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDependencyContractGradesPrerequisites(t *testing.T) {
	c := dependencyTestCase(t)
	for _, arm := range []string{"treatment", "control"} {
		result := actorResult{Response: validDependencyResponse, Events: []actorEvent{{Kind: "skill_selected", Path: "design-and-plan"}}}
		metrics, verdict, err := evaluateCase(context.Background(), c, gatesFile{}, arm, t.TempDir(), 0, result, true)
		if err != nil || verdict != "pass" || metrics["fact_retention"] != 1 || metrics["dependency_contract_success"] != 1 {
			t.Fatalf("valid %s plan: verdict=%s metrics=%v err=%v", arm, verdict, metrics, err)
		}
	}
}

func TestDependencyContractAllowsAdditionalFields(t *testing.T) {
	c := dependencyTestCase(t)
	response := strings.Replace(validDependencyResponse, `{"id":"api",`, `{"id":"api","acceptance_check":"go test ./api",`, 1)
	if _, _, complete := dependencyEvidence(c, response); !complete {
		t.Fatal("additional task field rejected")
	}
}

func TestDependencyContractRejectsWrongGraphs(t *testing.T) {
	c := dependencyTestCase(t)
	for _, tc := range []struct{ name, response string }{
		{"missing edge", strings.Replace(validDependencyResponse, `"prerequisites":["schema"]`, `"prerequisites":[]`, 1)},
		{"extra edge", strings.Replace(validDependencyResponse, `"id":"docs","owns":["docs/export.md"],"prerequisites":[]`, `"id":"docs","owns":["docs/export.md"],"prerequisites":["api"]`, 1)},
		{"cycle", strings.Replace(validDependencyResponse, `"id":"schema","owns":["schema/columns.go"],"prerequisites":[]`, `"id":"schema","owns":["schema/columns.go"],"prerequisites":["api"]`, 1)},
		{"overlapping ownership", strings.Replace(validDependencyResponse, `"owns":["api/export.go"]`, `"owns":["export/writer.go"]`, 1)},
		{"missing ownership", strings.Replace(validDependencyResponse, `"owns":["docs/export.md"]`, `"owns":[]`, 1)},
		{"missing task", strings.Replace(validDependencyResponse, `{"id":"docs","owns":["docs/export.md"],"prerequisites":[],"consumes":[],"produces":["export flag docs"]},`, ``, 1)},
		{"duplicate task", strings.Replace(validDependencyResponse, `"id":"docs"`, `"id":"schema"`, 1)},
		{"unconsumed prerequisite", strings.Replace(validDependencyResponse, `"consumes":["writer.Write"]`, `"consumes":[]`, 1)},
		{"prerequisite produces nothing", strings.Replace(validDependencyResponse, `"produces":["Column type"]`, `"produces":[]`, 1)},
		{"duplicate key", strings.Replace(validDependencyResponse, `"id":"api"`, `"id":"docs","id":"api"`, 1)},
		{"trailing content", validDependencyResponse + " done"},
		{"empty contract names", strings.Replace(strings.Replace(validDependencyResponse, `"produces":["Column type"]`, `"produces":[""]`, 1), `"consumes":["Column type"]`, `"consumes":[" "]`, 1)},
		{"ownership path alias", strings.Replace(validDependencyResponse, `"owns":["api/export.go"]`, `"owns":["./export/writer.go"]`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metrics, verdict, err := evaluateCase(context.Background(), c, gatesFile{}, "control", t.TempDir(), 0, actorResult{Response: tc.response}, true)
			if err != nil || verdict != "fail" || metrics["dependency_contract_success"] != 0 {
				t.Fatalf("invalid plan accepted: verdict=%s metrics=%v err=%v", verdict, metrics, err)
			}
		})
	}
}

func TestDependencyContractFailureReceiptNamesMissingEdge(t *testing.T) {
	c := dependencyTestCase(t)
	response := strings.Replace(validDependencyResponse, `"prerequisites":["writer"]`, `"prerequisites":["schema"]`, 1)
	receipt := buildFailureReceipt(resultRow{}, c, actorResult{Response: response})
	if len(receipt.MissingRequiredFactIDs) != 1 || receipt.MissingRequiredFactIDs[0] != "api_needs_writer" {
		t.Fatalf("dependency failure receipt = %v", receipt.MissingRequiredFactIDs)
	}
}

func TestDependencyContractConfigurationRejectsInvalidGraphs(t *testing.T) {
	catalog, gates, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	base := dependencyTestCase(t)
	for name, mutate := range map[string]func(*studyCase){
		"not workflow":     func(c *studyCase) { c.Kind = "prose" },
		"fact count":       func(c *studyCase) { c.RequiredFacts = c.RequiredFacts[:3]; c.RequiredFactIDs = c.RequiredFactIDs[:3] },
		"duplicate id":     func(c *studyCase) { c.DependencyContract[3].ID = "schema" },
		"unknown prereq":   func(c *studyCase) { c.DependencyContract[1].Prerequisites = []string{"missing"} },
		"cycle":            func(c *studyCase) { c.DependencyContract[0].Prerequisites = []string{"api"} },
		"duplicate prereq": func(c *studyCase) { c.DependencyContract[1].Prerequisites = []string{"schema", "schema"} },
		"with planning":    func(c *studyCase) { c.PlanningContract = &planningFacts{} },
		"single task only": func(c *studyCase) {
			c.DependencyContract = c.DependencyContract[:1]
			c.RequiredFacts = c.RequiredFacts[:1]
			c.RequiredFactIDs = c.RequiredFactIDs[:1]
		},
	} {
		c := base
		c.DependencyContract = make([]dependencyTask, len(base.DependencyContract))
		for i, task := range base.DependencyContract {
			c.DependencyContract[i] = dependencyTask{ID: task.ID, Prerequisites: append([]string(nil), task.Prerequisites...)}
		}
		c.RequiredFacts = append([]string(nil), base.RequiredFacts...)
		c.RequiredFactIDs = append([]string(nil), base.RequiredFactIDs...)
		mutate(&c)
		catalog.Cases = []studyCase{c}
		if err := validateConfiguration(catalog, gates); err == nil {
			t.Fatalf("%s: invalid dependency configuration accepted", name)
		}
	}
}

func TestDependencyCatalogCase(t *testing.T) {
	catalog, gates, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	var c studyCase
	for _, candidate := range catalog.Cases {
		if candidate.ID == "design-plan-dependency-order" {
			c = candidate
			break
		}
	}
	if c.DependencyContract == nil {
		t.Fatal("catalog must declare design-plan-dependency-order with a dependency_contract")
	}
	result := actorResult{Response: validDependencyResponse, Trace: []byte("complete"), Events: []actorEvent{
		{Kind: "skill_selected", Path: "design-and-plan"},
		{Kind: "trace_complete"},
	}}
	metrics, verdict, err := evaluateCase(context.Background(), c, gates, "treatment", t.TempDir(), 0, result, true)
	if err != nil || verdict != "pass" || metrics["dependency_contract_success"] != 1 {
		t.Fatalf("catalog dependency contract: verdict=%s metrics=%v err=%v", verdict, metrics, err)
	}
	attempt := result
	attempt.Events = append([]actorEvent{{Kind: "agent_spawn", RC: 0}}, result.Events...)
	if _, verdict, err := evaluateCase(context.Background(), c, gates, "treatment", t.TempDir(), 0, attempt, true); err != nil || verdict != "fail" {
		t.Fatalf("catalog allowed dispatch: verdict=%s err=%v", verdict, err)
	}
}
