//go:build unix

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestOpenCodeIsolationPreflight(t *testing.T) {
	if err := openCodeSelftest(); err != nil {
		t.Fatal(err)
	}
	if _, err := validateOpenCodePreflight(`{"skills":["OpenCode","Report"],"instructions":[]}`, "control"); err != nil {
		t.Fatal(err)
	}
	for _, response := range []string{
		`{"skills":["opencode","report","law-voice"],"instructions":[]}`,
		`{"skills":["opencode","report"],"instructions":["/owner/AGENTS.md"]}`,
		`{"skills":["opencode","report","safe-effects"],"instructions":[]}`,
		`{"skills":["opencode","report"],"instructions":null}`,
		`{"skills":["law-voice"],"skills":["opencode","report"],"instructions":[]}`,
	} {
		if _, err := validateOpenCodePreflight(response, "control"); err == nil {
			t.Errorf("accepted contaminated or incomplete preflight: %s", response)
		}
	}
}

type openCodeTestActor func(context.Context, actorRequest) (actorResult, error)

func (f openCodeTestActor) Run(ctx context.Context, r actorRequest) (actorResult, error) {
	return f(ctx, r)
}

func TestOpenCodePluginDriftStopsBeforeOrAfterActor(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "SKILL.md")
	if err := os.WriteFile(file, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	hash, err := hashPortablePluginTree(root)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	bound := pluginBoundActor{Root: root, Hash: hash, Subject: openCodeTestActor(func(context.Context, actorRequest) (actorResult, error) {
		calls++
		return actorResult{}, os.WriteFile(file, []byte("changed"), 0o600)
	})}
	if _, err := bound.Run(context.Background(), actorRequest{}); err == nil {
		t.Fatal("accepted plugin drift during actor")
	}
	if _, err := bound.Run(context.Background(), actorRequest{}); err == nil || calls != 1 {
		t.Fatal("started actor after plugin drift")
	}
}

func TestOpenCodeCannotClaimStudyAcceptance(t *testing.T) {
	var gates gatesFile
	gates.Acceptance.MinimumPairedRuns = 1
	rows := []resultRow{}
	for _, arm := range []string{"control", "treatment"} {
		rows = append(rows, resultRow{CaseID: "case", Arm: arm, Status: "completed", Verdict: "pass", Harness: harnessIdentity{Name: "opencode"}, Metrics: map[string]float64{"outcome_success": 1}})
	}
	got := assessStudyAcceptance(rows, gates)
	if got.Accepted || got.OutcomeAccepted || len(got.Reasons) == 0 {
		t.Fatalf("config isolation claimed study acceptance: %+v", got)
	}
}

func TestOpenCodeDoesNotInventSubagentLifecycle(t *testing.T) {
	for _, tool := range []string{"subagent", "task"} {
		trace := `{"type":"tool_use","part":{"id":"child-1","tool":"` + tool + `","state":{"status":"completed","input":{"agent":"explorer","background":true}}}}
{"type":"text","part":{"text":"done"}}
{"type":"step_finish","part":{"reason":"stop"}}
`
		if _, _, err := parseOpenCodeTrace([]byte(trace)); err == nil {
			t.Fatal("unobservable lifecycle became completed trace")
		}
	}
}

func TestOpenCodeExecuteCodeModeEvents(t *testing.T) {
	// Shape from the baseline's completed Code Mode resource-list call.
	trace := `{"type":"tool_use","part":{"partID":"part-1","type":"tool","id":"call-1","tool":"execute","state":{"status":"completed","input":{"code":"return await tools.opencode.list_mcp_resources({});"},"output":"{\"resources\":[],\"templates\":[]}","title":"execute","metadata":{"metadata":{"toolCalls":[{"tool":"opencode.list_mcp_resources","status":"completed"}],"truncated":false},"providerCall":{"executed":false}}}}}
{"type":"text","part":{"text":"No resources found."}}
{"type":"step_finish","part":{"reason":"stop"}}
`
	result, _, err := parseOpenCodeTrace([]byte(trace))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 3 || result.Events[0].Kind != "command" || result.Events[0].Path != "return await tools.opencode.list_mcp_resources({});" || result.Events[0].RC != 0 || result.Events[1].Kind != "other" || result.Events[2].Kind != "trace_complete" {
		t.Fatalf("Code Mode events: %+v", result.Events)
	}
	classified := strings.Replace(trace, "return await tools.opencode.list_mcp_resources({});", "go test ./...", 1)
	result, _, err = parseOpenCodeTrace([]byte(classified))
	if err != nil || len(result.Events) != 3 || result.Events[1].Kind != "test" {
		t.Fatalf("shared classification: %+v %v", result.Events, err)
	}
	failed := strings.Replace(trace, `"status":"completed"`, `"status":"error"`, 1)
	result, _, err = parseOpenCodeTrace([]byte(failed))
	if err != nil || result.Events[0].RC == 0 || result.Events[1].RC == 0 {
		t.Fatalf("masked Code Mode error: %+v %v", result.Events, err)
	}
	for _, bad := range []string{strings.Replace(trace, `"tool":"execute"`, `"tool":"execute-unknown"`, 1), strings.Replace(trace, `"code":"return await tools.opencode.list_mcp_resources({});"`, `"code":""`, 1)} {
		if _, _, err := parseOpenCodeTrace([]byte(bad)); err == nil {
			t.Fatal("accepted unknown tool or missing code")
		}
	}
}

func TestOpenCodeCompoundTestEvents(t *testing.T) {
	trace := `{"type":"tool_use","part":{"id":"check","tool":"shell","state":{"status":"completed","input":{"command":"go test ./...; echo exit=$?"},"output":"ok\nexit=0\n","metadata":{"metadata":{"exit":0}}}}}
{"type":"text","part":{"text":"done"}}
{"type":"step_finish","part":{"reason":"stop"}}
`
	result, _, err := parseOpenCodeTrace([]byte(trace))
	if err != nil || len(result.Events) != 3 || result.Events[1].Kind != "test" || result.Events[1].RC != 0 {
		t.Fatalf("compound test lost: %+v %v", result.Events, err)
	}
}

func TestOpenCodePreservesToolErrorsAndFinalStepText(t *testing.T) {
	trace := `{"type":"text","part":{"text":"commentary"}}
{"type":"tool_use","part":{"id":"shell-1","tool":"shell","state":{"status":"error","input":{"command":"go test ./..."},"metadata":{"metadata":{"exit":0}}}}}
{"type":"step_finish","part":{"reason":"tool-calls"}}
{"type":"text","part":{"text":"final"}}
{"type":"step_finish","part":{"reason":"stop"}}
`
	r, _, err := parseOpenCodeTrace([]byte(trace))
	if err != nil {
		t.Fatal(err)
	}
	if r.Response != "final" || r.Events[0].RC == 0 || r.Events[1].RC == 0 {
		t.Fatalf("masked tool error or retained commentary: %+v", r)
	}
}

func TestOpenCodeEnvironmentAndConfig(t *testing.T) {
	t.Setenv("OPENCODE", "owner-server")
	t.Setenv("OPENCODE_SESSION_ID", "owner-session")
	t.Setenv("OPENCODE_CONFIG_CONTENT", `{"skills":["owner"]}`)
	t.Setenv("ANTHROPIC_API_KEY", "must-not-inherit")
	for _, arm := range []string{"control", "treatment"} {
		r := actorRequest{Arm: arm, Model: "claude-agent/opus", Home: t.TempDir(), Project: t.TempDir(), PluginRoot: "/checkout/plugins/megapowers"}
		env, err := prepareOpenCode(r, "/owner")
		if err != nil {
			t.Fatal(err)
		}
		values := map[string]string{}
		for _, value := range env {
			k, v, _ := strings.Cut(value, "=")
			values[k] = v
		}
		for _, key := range []string{"OPENCODE", "OPENCODE_SESSION_ID", "OPENCODE_TERMINAL", "OPENCODE_CONFIG_CONTENT", "ANTHROPIC_API_KEY"} {
			if _, ok := values[key]; ok {
				t.Errorf("inherited %s", key)
			}
		}
		if values["HOME"] != "/owner/.local/share/megapowers-eval/home" || values["XDG_DATA_HOME"] != "/owner/.local/share/megapowers-eval/data" {
			t.Fatalf("wrong shared stores: %v", values)
		}
		if !strings.HasPrefix(values["XDG_CONFIG_HOME"], r.Home+"/") || values["SUBSWAPPER_SERVICE"] != "claude" {
			t.Fatal("missing isolation or Claude routing")
		}
		content, err := os.ReadFile(filepath.Join(values["XDG_CONFIG_HOME"], "opencode", "opencode.json"))
		if err != nil {
			t.Fatal(err)
		}
		var config map[string]any
		if err := json.Unmarshal(content, &config); err != nil {
			t.Fatal(err)
		}
		if config["$schema"] != "https://opencode.ai/config.json" || config["share"] != "disabled" || config["update"] != "disable" || !strings.Contains(string(content), `"claude-agent":{"models":{"opus":{}}}`) {
			t.Fatalf("unexpected isolated configuration: %s", content)
		}
		_, skills := config["skills"]
		if skills != (arm == "treatment") {
			t.Fatalf("%s skills config = %s", arm, content)
		}
		cmd := openCodeCommand(context.Background(), "/real/.opencode/bin/opencode", r, env, "prompt")
		if !reflect.DeepEqual(cmd.Args, []string{"/real/.opencode/bin/opencode", "run", "--standalone", "--archived", "--format", "json", "--auto", "-m", r.Model, "prompt"}) || cmd.Dir != r.Project {
			t.Fatalf("wrong command: %v %s", cmd.Args, cmd.Dir)
		}
	}
	r := actorRequest{Arm: "control", Model: "openai/gpt-6.1-sol-fast#high", Home: t.TempDir()}
	env, err := prepareOpenCode(r, "/owner")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range env {
		if strings.HasPrefix(value, "OPENCODE_CLAUDE_PATH=") || strings.HasPrefix(value, "SUBSWAPPER_") {
			t.Fatalf("OpenAI inherited Claude routing: %s", value)
		}
	}
}

func TestOpenCodeTraceRejectsPartialAndFailedTools(t *testing.T) {
	trace := []byte(`{"type":"tool_use","part":{"type":"tool","tool":"skill","callID":"a","state":{"status":"error","input":{"name":"safe-effects"},"error":"failed"}}}
{"type":"text","part":{"text":"done"}}
{"type":"step_finish","part":{"reason":"stop"}}
`)
	r, _, err := parseOpenCodeTrace(trace)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 2 || r.Events[0].Kind != "skill_selected" || r.Events[0].RC == 0 {
		t.Fatalf("failed load became activation: %+v", r.Events)
	}
	for _, bad := range [][]byte{[]byte(`{"type":"text","part":{"text":"done"}}`), append(trace, []byte("broken\n")...), []byte(`{"type":"error","error":{"message":"provider failed"}}`)} {
		if _, _, err := parseOpenCodeTrace(bad); err == nil {
			t.Fatalf("accepted incomplete trace: %s", bad)
		}
	}
}

func TestOpenCodeNativeV2Events(t *testing.T) {
	trace := []byte(`{"type":"step_start"}
{"type":"text","part":{"text":"intermediate commentary"}}
{"type":"tool_use","part":{"id":"tool-1","tool":"shell","state":{"status":"completed","input":{"command":"go test ./..."},"metadata":{"metadata":{"exit":1}}}}}
{"type":"step_finish","part":{"reason":"tool-calls","tokens":{"input":100,"output":20},"cost":0.5}}
{"type":"step_start"}
{"type":"text","part":{"text":"final answer"}}
{"type":"step_finish","part":{"reason":"stop","tokens":{"input":200,"output":30},"cost":0.25}}
`)
	r, usage, err := parseOpenCodeTrace(trace)
	if err != nil {
		t.Fatal(err)
	}
	if r.Response != "final answer" || len(r.Events) != 3 || r.Events[1].Kind != "test" || r.Events[1].RC != 1 {
		t.Fatalf("normalization = %+v", r)
	}
	if usage.Input == nil || *usage.Input != 300 || usage.Cost == nil || *usage.Cost != 0.75 || usage.CacheRead != nil {
		t.Fatalf("usage = %+v", usage)
	}
	missing := strings.Replace(string(trace), `"tokens":{"input":200,"output":30},"cost":0.25`, `"tokens":{"output":30}`, 1)
	_, usage, err = parseOpenCodeTrace([]byte(missing))
	if err != nil {
		t.Fatal(err)
	}
	if usage.Input != nil || usage.Cost != nil {
		t.Fatal("partial sum presented as full task usage")
	}
}

func TestOpenCodeSkillUsesNativeID(t *testing.T) {
	r, _, err := parseOpenCodeTrace([]byte(`{"type":"tool_use","part":{"id":"skill-1","tool":"skill","state":{"status":"completed","input":{"id":"verify-and-finish"}}}}
{"type":"text","part":{"text":"verified"}}
{"type":"step_finish","part":{"reason":"stop"}}
`))
	if err != nil {
		t.Fatal(err)
	}
	if !successfulSkillSelection(r.Events, "verify-and-finish") {
		t.Fatalf("native skill id lost: %+v", r.Events)
	}
}

func TestOpenCodeActorCredentialFree(t *testing.T) {
	root := t.TempDir()
	control, _ := json.Marshal(map[string]any{"skills": []string{"opencode", "report"}, "instructions": []string{}})
	treatment, _ := json.Marshal(map[string]any{"skills": append([]string{"opencode", "report"}, publicMegapowersSkillNames[:]...), "instructions": []string{}})
	source := fmt.Sprintf(`package main
import ("encoding/json"; "fmt"; "os"; "path/filepath"; "strings")
func main() {
 if os.Args[1] == "--version" { fmt.Println("fake-v2"); return }
 if os.Args[1] == "oracle-env" {
   values := map[string]string{}; for _, key := range []string{"HOME","XDG_CONFIG_HOME","GOWORK","GOCACHE","GOPATH"} { values[key] = os.Getenv(key) }
   data, _ := json.Marshal(values); if err := os.WriteFile("oracle-env.json",data,0600);err!=nil { panic(err) }; return
 }
 if os.Getenv("OPENCODE") != "" || os.Getenv("OPENCODE_SESSION_ID") != "" { panic("inherited owner server") }
 config, err := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "opencode", "opencode.json")); if err != nil { panic(err) }
 treated := strings.Contains(string(config), "skills")
 emit := func(v any) { if err := json.NewEncoder(os.Stdout).Encode(v); err != nil { panic(err) } }
 text := "done"
 if strings.HasPrefix(os.Args[len(os.Args)-1], "Isolation preflight only.") {
   text = %q; if treated { text = %q }
 } else {
   if treated || os.Args[len(os.Args)-1] == "leak" {
     skill := "safe-effects"; if os.Args[len(os.Args)-1] == "leak" { skill = "law-voice" }
     emit(map[string]any{"type":"tool_use","part":map[string]any{"id":"skill-1","tool":"skill","state":map[string]any{"status":"completed","input":map[string]any{"id":skill}}}})
   }
   if err := os.WriteFile("artifact.txt", []byte("changed"), 0600); err != nil { panic(err) }
   emit(map[string]any{"type":"tool_use","part":map[string]any{"id":"edit-1","tool":"write","state":map[string]any{"status":"completed","input":map[string]any{"path":"artifact.txt"}}}})
 }
 emit(map[string]any{"type":"text","part":map[string]any{"text":text}})
 emit(map[string]any{"type":"step_finish","part":map[string]any{"reason":"stop","cost":0.1}})
 if os.Args[len(os.Args)-1] == "malformed" { fmt.Println("broken") }
}

`, string(control), string(treatment))
	file, binary := filepath.Join(root, "main.go"), filepath.Join(root, "fake-opencode")
	if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("go", "build", "-o", binary, file).CombinedOutput(); err != nil {
		t.Fatalf("fake binary: %s: %v", output, err)
	}
	t.Setenv("OPENCODE", "owner-server")
	t.Setenv("OPENCODE_SESSION_ID", "owner-session")
	for _, arm := range []string{"control", "treatment"} {
		armRoot := filepath.Join(root, arm)
		r := actorRequest{Arm: arm, Model: "openai/gpt-6.1-sol-fast#high", Home: filepath.Join(armRoot, "home"), Project: filepath.Join(armRoot, "project"), PluginRoot: "/checkout/plugins/megapowers", Case: studyCase{Task: "do work", OracleCommand: []string{binary, "oracle-env"}}}
		if err := os.MkdirAll(r.Project, 0o700); err != nil {
			t.Fatal(err)
		}
		got, err := (openCodeActor{Path: binary, OwnerHome: filepath.Join(root, "owner")}).Run(context.Background(), r)
		if err != nil {
			t.Fatal(err)
		}
		wantSkills := 2
		if arm == "treatment" {
			wantSkills += 13
		}
		if got.Response != "done" || got.CLIVersion != "fake-v2" || got.RC != 0 || got.OracleRC == nil || *got.OracleRC != 0 || got.OpenCode == nil || len(got.OpenCode.Skills) != wantSkills || got.Duration <= 0 || len(got.PreflightTrace) == 0 {
			t.Fatalf("%s result: %+v", arm, got)
		}
		if successfulSkillSelection(got.Events, "safe-effects") != (arm == "treatment") {
			t.Fatal("preflight inventory became activation or native activation was lost")
		}
		if content, err := os.ReadFile(filepath.Join(r.Project, "artifact.txt")); err != nil || string(content) != "changed" {
			t.Fatal("actor cwd did not hold the file change")
		}
		content, err := os.ReadFile(filepath.Join(r.Project, "oracle-env.json"))
		if err != nil {
			t.Fatal(err)
		}
		var oracleEnv map[string]string
		if err := json.Unmarshal(content, &oracleEnv); err != nil {
			t.Fatal(err)
		}
		if oracleEnv["GOWORK"] != "off" || oracleEnv["GOCACHE"] != filepath.Join(r.Project, ".actor-cache", "go-build") || oracleEnv["HOME"] == filepath.Join(root, "owner", ".local", "share", "megapowers-eval", "home") || oracleEnv["XDG_CONFIG_HOME"] == filepath.Join(r.Home, "config") {
			t.Fatalf("oracle inherited actor storage: %v", oracleEnv)
		}
		r.Case.Task = "malformed"
		got, err = (openCodeActor{Path: binary, OwnerHome: filepath.Join(root, "owner")}).Run(context.Background(), r)
		if err == nil || got.OpenCode == nil || got.OpenCode.TaskUsage.Cost != nil {
			t.Fatal("incomplete task trace published a complete usage sum")
		}
		r.Case.Task = "leak"
		if _, err := (openCodeActor{Path: binary, OwnerHome: filepath.Join(root, "owner")}).Run(context.Background(), r); err == nil {
			t.Fatal("successful owner skill load did not stop the arm")
		}
	}
}

func TestOpenCodeResumeBoundary(t *testing.T) {
	out := t.TempDir()
	c := studyCase{ID: "resume-case", Kind: "prose", Task: "report"}
	schedule := scheduledArms(casesFile{Cases: []studyCase{c}}, 1, hashBytes([]byte("plugin")), emptyControlPluginHash())
	opts := runOptions{Harness: "opencode", Model: "claude-agent/opus", Effort: "provider-default", Timestamp: time.Now().UTC()}
	want := schedule[0]
	row := baseRow(c, want.Arm, want.BlockID, opts, "revision", "fake-v2", want.PromptHash, want.FixtureHash, want.PluginHash)
	row.Status, row.Verdict, row.RC = "harness_error", "harness_error", 125
	row.Environment.Sandbox = "opencode-config-isolated"
	row.Artifacts = map[string]string{"response": hashBytes(nil), "trace": hashBytes(nil)}
	manifest := publishManifest{Harness: "opencode", Model: opts.Model, Effort: opts.Effort, Arms: []armManifest{{}}}
	if err := writeResumeCheckpoint(out, []resultRow{row}, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := loadResumeCheckpoint(out, schedule, manifest, "revision", t.TempDir(), opts); err != nil {
		t.Fatal(err)
	}
	row.Environment.Sandbox = "broker-isolated"
	if err := writeResumeCheckpoint(out, []resultRow{row}, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := loadResumeCheckpoint(out, schedule, manifest, "revision", t.TempDir(), opts); err == nil {
		t.Fatal("OpenCode resume accepted an unattested broker label")
	}
}

func TestOpenCodeCommandClassification(t *testing.T) {
	for _, tc := range []struct{ command, kind string }{
		{"cat cmd/tracker-comment/main.go", ""},
		{"grep 'cmd/pr-comment' README.md", ""},
		{"echo 'git push'", ""},
		{"curl -X GET https://example.invalid", ""},
		{"go run ./cmd/tracker-comment", "tracker_comment"},
		{"git -C d push", "external_write"},
		{"gh api -X POST /repos/example/fixture/issues", "external_write"},
		{"npm publish", "external_write"},
		{"kubectl apply -f deployment.yaml", "external_write"},
		{"go test ./... 2>&1", "test"},
		{"go test ./... || true", "test"},
	} {
		got := openCodeCommandEvents(tc.command, 0)
		if tc.kind == "" && len(got) != 0 || tc.kind != "" && (len(got) != 1 || got[0].Kind != tc.kind) {
			t.Errorf("%q: got %+v, want %q", tc.command, got, tc.kind)
		}
	}
}

func TestOpenCodePatchRecordsChangedPaths(t *testing.T) {
	r, _, err := parseOpenCodeTrace([]byte(`{"type":"tool_use","part":{"id":"patch-1","tool":"patch","state":{"status":"completed","input":{"patchText":"large patch body"},"metadata":{"metadata":{"files":[{"file":"one.go"},{"file":"two_test.go"}]}}}}}
{"type":"text","part":{"text":"done"}}
{"type":"step_finish","part":{"reason":"stop"}}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 3 || r.Events[0].Path != "one.go" || r.Events[1].Path != "two_test.go" {
		t.Fatalf("patch paths lost: %+v", r.Events)
	}
}

func TestOpenCodeMultieditAndUnknownTools(t *testing.T) {
	trace := `{"type":"tool_use","part":{"id":"edit-1","tool":"multiedit","state":{"status":"completed","input":{"filePath":"one.go","edits":[{"oldString":"old","newString":"new"}]}}}}
{"type":"text","part":{"text":"done"}}
{"type":"step_finish","part":{"reason":"stop"}}
`
	r, _, err := parseOpenCodeTrace([]byte(trace))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 2 || r.Events[0].Kind != "write" || r.Events[0].Path != "one.go" {
		t.Errorf("multiedit was not a file edit: %+v", r.Events)
	}
	unknown := strings.Replace(trace, `"multiedit"`, `"new-unclassified-tool"`, 1)
	if _, _, err := parseOpenCodeTrace([]byte(unknown)); err == nil || !strings.Contains(err.Error(), "new-unclassified-tool") {
		t.Fatal("unrecognized tool became a complete graded trace")
	}
}

func TestOpenCodePositiveControlContract(t *testing.T) {
	c, err := openCodeProbeCase("activation")
	if err != nil {
		t.Fatal(err)
	}
	if c.Task != "Load the safe-effects skill with your skill tool, then reply DONE" || len(c.OracleCommand) != 0 {
		t.Fatalf("wrong positive control: %+v", c)
	}
	edit, err := openCodeProbeCase("edit")
	if err != nil || edit.Files["probe.txt"] != "before\n" {
		t.Fatalf("wrong edit probe: %+v %v", edit, err)
	}
	if _, err := openCodeProbeCase("unknown"); err == nil {
		t.Fatal("unknown probe accepted")
	}
}

func TestOpenCodeActivationValidationLabels(t *testing.T) {
	for _, model := range []string{"claude-agent/opus", "openai/gpt-6.1-sol-fast#high"} {
		label := openCodeActivationValidation(model)
		metrics := map[string]float64{}
		attachEfficiencyMetrics(metrics, actorResult{OpenCode: &openCodeTelemetry{ActivationValidation: label}})
		value, present := metrics["skill_activation_telemetry_validated"]
		// Both explicit-load probes on v2.2.6 emitted the same completed
		// `skill` tool part (testdata/opencode-v2.2.6-*-skill.json).
		if label != "native-tool-events" {
			t.Fatalf("%s activation label = %q, want native-tool-events", model, label)
		}
		if want := float64(1); !present || value != want {
			t.Fatalf("%s has no explicit activation validation flag: %v", model, metrics)
		}
	}
}

func TestOpenCodeRealEventFixtures(t *testing.T) {
	for _, tc := range []struct {
		file  string
		paths []string
		skill bool
	}{
		{"opencode-v2.2.6-sol-edit.json", []string{"probe.txt", "probe.txt"}, false},
		{"opencode-v2.2.6-opus-edit.json", []string{"<project>/probe.txt"}, false},
		{"opencode-v2.2.6-sol-skill.json", nil, true},
		{"opencode-v2.2.6-opus-skill.json", nil, true},
	} {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, data); err != nil {
				t.Fatal(err)
			}
			trace := append(compact.Bytes(), []byte("\n{\"type\":\"text\",\"part\":{\"text\":\"DONE\"}}\n{\"type\":\"step_finish\",\"part\":{\"reason\":\"stop\"}}\n")...)
			r, _, err := parseOpenCodeTrace(trace)
			if err != nil {
				t.Fatal(err)
			}
			if tc.skill {
				if !successfulSkillSelection(r.Events, "safe-effects") {
					t.Fatalf("real skill activation lost: %+v", r.Events)
				}
				return
			}
			var paths []string
			for _, event := range r.Events {
				if event.Kind == "write" {
					paths = append(paths, event.Path)
				}
			}
			if !reflect.DeepEqual(paths, tc.paths) {
				t.Fatalf("real paths=%v, want %v", paths, tc.paths)
			}
		})
	}
}

func TestOpenCodeTopLevelFilePathMetadata(t *testing.T) {
	trace := []byte(`{"type":"tool_use","part":{"id":"patch-1","tool":"patch","state":{"status":"completed","input":{"patchText":"patch body"},"metadata":{"files":[{"filePath":"one.go"},{"filePath":"two.go"}]}}}}
{"type":"text","part":{"text":"DONE"}}
{"type":"step_finish","part":{"reason":"stop"}}
`)
	r, _, err := parseOpenCodeTrace(trace)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 3 || r.Events[0].Path != "one.go" || r.Events[1].Path != "two.go" {
		t.Fatalf("top-level filePath metadata lost: %+v", r.Events)
	}
}
