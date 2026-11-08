package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Redacted V2 shapes: run emits state.metadata.metadata; session export emits
// messages[].content with tool names and direct state.metadata/content.
const openCodeChildExport = `{"info":{"id":"ses_child1","parentID":"ses_parent1","outcome":"succeeded","time":{"busy":10,"idle":30}},"messages":[
{"id":"msg_user","type":"user","time":{"created":10},"text":"Check the fix"},
{"id":"msg_tools","type":"assistant","time":{"created":11,"completed":20},"content":[
{"type":"tool","id":"skill1","name":"skill","state":{"status":"completed","input":{"id":"safe-effects"},"content":[{"type":"text","text":"loaded"}]}},
{"type":"tool","id":"edit1","name":"patch","state":{"status":"completed","input":{},"metadata":{"files":[{"file":"store.go"}]}}},
{"type":"tool","id":"shell1","name":"shell","state":{"status":"completed","input":{"command":"go test ./...; echo rc=$?"},"content":[{"type":"text","text":"ok example/hard 0.003s\nrc=0\n"}],"metadata":{"exit":0}}},
{"type":"tool","id":"shell2","name":"shell","state":{"status":"completed","input":{"command":"./deploy.sh"},"content":[{"type":"text","text":"denied"}],"metadata":{"exit":1}}}
],"finish":"tool-calls"},
{"id":"msg_final","type":"assistant","time":{"created":21,"completed":29},"content":[{"type":"text","text":"Child done"}],"finish":"stop"},
{"id":"msg_idle","type":"idle","time":{"created":30},"outcome":"succeeded"}]}`

func openCodeSpawnTrace(tool, id string) []byte {
	return []byte(fmt.Sprintf(`{"type":"tool_use","sessionID":"ses_parent1","part":{"id":"spawn1","tool":%q,"state":{"status":"completed","input":{"agent":"general"},"output":"child done","metadata":{"metadata":{"sessionID":%q,"status":"completed"}}}}}
{"type":"tool_use","part":{"id":"parent-edit","tool":"write","state":{"status":"completed","input":{"path":"parent.go"}}}}
{"type":"text","part":{"text":"Parent done"}}
{"type":"step_finish","part":{"reason":"stop"}}
`, tool, id))
}

func TestOpenCodeChildAttribution(t *testing.T) {
	for _, tool := range []string{"subagent", "task"} {
		t.Run(tool, func(t *testing.T) {
			got, _, err := parseOpenCodeTraceWithChildren(openCodeSpawnTrace(tool, "ses_child1"), func(id string) ([]byte, error) {
				if id != "ses_child1" {
					t.Fatalf("exported %q", id)
				}
				return []byte(openCodeChildExport), nil
			})
			if err != nil {
				t.Fatal(err)
			}
			var kinds []string
			for i, event := range got.Events {
				kinds = append(kinds, event.Kind)
				if event.Step != i+1 {
					t.Fatalf("nonsequential event: %+v", event)
				}
			}
			want := []string{"agent_spawn", "skill_selected", "write", "command", "test", "command", "external_write", "agent_complete", "write", "trace_complete"}
			if !reflect.DeepEqual(kinds, want) || got.Response != "Parent done" || string(got.ChildTraces["ses_child1"]) != openCodeChildExport {
				t.Fatalf("attribution: %+v; kinds=%v", got, kinds)
			}
			if got.Events[0].Path != "ses_child1" || got.Events[7].Path != "ses_child1" || got.Events[4].RC != 0 || got.Events[6].RC != 1 || got.Events[2].SessionID != "ses_child1" {
				t.Fatalf("identity/status lost: %+v", got.Events)
			}
			metrics, _, err := evaluateCaseEvidence(context.Background(), studyCase{Kind: "workflow", ForbiddenEventKinds: []string{"external_write"}}, gatesFile{}, "control", "", 0, got, false, nil)
			if err != nil || metrics["agent_spawn_count"] != 1 || metrics["forbidden_event_attempts"] != 1 || metrics["write_attempts"] != 2 || metrics["test_attempts"] != 1 {
				t.Fatalf("child metrics: %v %v", metrics, err)
			}
			spawns, attempts, _, batch, matching := orchestrationEvidence(got.Events, 1)
			if spawns != 1 || attempts != 1 || !batch || !matching {
				t.Fatalf("dispatch evidence: %+v", got.Events)
			}
		})
	}
}

func TestOpenCodeChildFailures(t *testing.T) {
	for _, tc := range []struct {
		name, id, export, want string
		loadErr                error
	}{
		{"missing ID", "", openCodeChildExport, "session ID", nil},
		{"unsafe ID", "../child", openCodeChildExport, "session ID", nil},
		{"export failure", "ses_child1", "", "export child", errors.New("not found")},
		{"malformed", "ses_child1", "{", "child ses_child1", nil},
		{"wrong ID", "ses_child1", strings.Replace(openCodeChildExport, `"id":"ses_child1"`, `"id":"ses_wrong"`, 1), "identity", nil},
		{"running", "ses_child1", strings.Replace(openCodeChildExport, `"idle":30`, `"idle":0`, 1), "incomplete", nil},
		{"failed", "ses_child1", strings.ReplaceAll(openCodeChildExport, `"outcome":"succeeded"`, `"outcome":"failed"`), "incomplete", nil},
		{"no idle message", "ses_child1", strings.Replace(openCodeChildExport, `,"outcome":"succeeded"}]`, `,"outcome":"succeeded"},{"type":"user","text":"more"}]`, 1), "incomplete", nil},
		{"partial assistant", "ses_child1", strings.Replace(openCodeChildExport, `"completed":20`, `"completed":0`, 1), "incomplete", nil},
		{"partial tool", "ses_child1", strings.Replace(openCodeChildExport, `"status":"completed"`, `"status":"running"`, 1), "incomplete", nil},
		{"no stop", "ses_child1", strings.Replace(openCodeChildExport, `"finish":"stop"`, `"finish":"tool-calls"`, 1), "stop marker", nil},
		{"unknown tool", "ses_child1", strings.Replace(openCodeChildExport, `"name":"patch"`, `"name":"unknown"`, 1), "unrecognized", nil},
		{"unknown content", "ses_child1", strings.Replace(openCodeChildExport, `"type":"tool"`, `"type":"unknown"`, 1), "content", nil},
		{"unknown message", "ses_child1", strings.Replace(openCodeChildExport, `"type":"user"`, `"type":"unknown"`, 1), "message", nil},
		{"null state", "ses_child1", strings.Replace(openCodeChildExport, `"state":{"status":"completed","input":{"id":"safe-effects"},"content":[{"type":"text","text":"loaded"}]}`, `"state":null`, 1), "state", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := parseOpenCodeTraceWithChildren(openCodeSpawnTrace("subagent", tc.id), func(string) ([]byte, error) { return []byte(tc.export), tc.loadErr })
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
			for _, event := range got.Events {
				if event.Kind == "trace_complete" {
					t.Fatal("incomplete child completed parent")
				}
			}
		})
	}
}

func TestOpenCodeChildParentIdentity(t *testing.T) {
	for _, tc := range []struct{ name, trace, child, want string }{
		{"wrong parent", string(openCodeSpawnTrace("subagent", "ses_child1")), strings.Replace(openCodeChildExport, `"parentID":"ses_parent1"`, `"parentID":"ses_other1"`, 1), "parent identity"},
		{"missing spawning ID", strings.Replace(string(openCodeSpawnTrace("subagent", "ses_child1")), `,"sessionID":"ses_parent1"`, "", 1), openCodeChildExport, "spawning session ID"},
		{"root cycle", string(openCodeSpawnTrace("subagent", "ses_parent1")), strings.ReplaceAll(openCodeChildExport, "ses_child1", "ses_parent1"), "cyclic"},
		{"conflicting session IDs", strings.Replace(string(openCodeSpawnTrace("subagent", "ses_child1")), `"id":"spawn1"`, `"id":"spawn1","sessionID":"ses_wrong1"`, 1), openCodeChildExport, "session identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseOpenCodeTraceWithChildren([]byte(tc.trace), func(string) ([]byte, error) { return []byte(tc.child), nil })
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestOpenCodeFailedSpawnCannotClaimSuccessfulCompletion(t *testing.T) {
	trace := strings.Replace(string(openCodeSpawnTrace("subagent", "ses_child1")), `"status":"completed"`, `"status":"error"`, 1)
	got, _, err := parseOpenCodeTraceWithChildren([]byte(trace), func(string) ([]byte, error) { return []byte(openCodeChildExport), nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range got.Events {
		if event.Kind == "agent_complete" && event.RC == 0 {
			t.Fatal("failed spawn claimed a successful completion")
		}
	}
}

func TestOpenCodeNestedChildrenAndBounds(t *testing.T) {
	nested := strings.Replace(openCodeChildExport, `{"type":"tool","id":"skill1","name":"skill","state":{"status":"completed","input":{"id":"safe-effects"},"content":[{"type":"text","text":"loaded"}]}}`, `{"type":"tool","id":"nested","name":"subagent","state":{"status":"completed","input":{},"metadata":{"sessionID":"ses_child2","status":"completed"}}}`, 1)
	load := func(id string) ([]byte, error) {
		if id == "ses_child1" {
			return []byte(nested), nil
		}
		value := strings.ReplaceAll(openCodeChildExport, "ses_child1", "ses_child2")
		return []byte(strings.Replace(value, `"parentID":"ses_parent1"`, `"parentID":"ses_child1"`, 1)), nil
	}
	got, _, err := parseOpenCodeTraceWithChildren(openCodeSpawnTrace("subagent", "ses_child1"), load)
	if err != nil || len(got.ChildTraces) != 2 {
		t.Fatalf("nested children: %+v %v", got, err)
	}
	spawns, _, _, batch, matching := orchestrationEvidence(got.Events, 1)
	if spawns != 1 || !batch || !matching {
		t.Fatalf("nested work changed parent dispatch: %+v", got.Events)
	}
	metrics, _, err := evaluateCaseEvidence(context.Background(), studyCase{Kind: "workflow"}, gatesFile{}, "control", "", 0, got, false, nil)
	if err != nil || metrics["agent_spawn_count"] != 2 || metrics["test_attempts"] != 2 {
		t.Fatalf("nested actions not counted: %v %v", metrics, err)
	}
	for _, mode := range []string{"cycle", "depth", "count"} {
		t.Run(mode, func(t *testing.T) {
			_, _, err := parseOpenCodeTraceWithChildren(openCodeSpawnTrace("subagent", "ses_child1"), func(id string) ([]byte, error) {
				n := 1
				fmt.Sscanf(id, "ses_child%d", &n)
				value := strings.ReplaceAll(nested, "ses_child1", id)
				if n > 1 {
					value = strings.Replace(value, `"parentID":"ses_parent1"`, fmt.Sprintf(`"parentID":"ses_child%d"`, n-1), 1)
				}
				next := fmt.Sprintf("ses_child%d", n+1)
				if mode == "cycle" {
					next = id
				}
				value = strings.Replace(value, `"sessionID":"ses_child2"`, `"sessionID":"`+next+`"`, 1)
				if mode == "count" {
					// Siblings exceed the total session ceiling without exceeding depth.
					spawn := `{"type":"tool","id":"nested%d","name":"subagent","state":{"status":"completed","input":{},"metadata":{"sessionID":"ses_child%d"}}}`
					if n != 1 {
						value := strings.ReplaceAll(openCodeChildExport, "ses_child1", id)
						return []byte(strings.Replace(value, `"parentID":"ses_parent1"`, `"parentID":"ses_child1"`, 1)), nil
					}
					var siblings []string
					for i := 2; i <= 66; i++ {
						siblings = append(siblings, fmt.Sprintf(spawn, i, i))
					}
					value = strings.Replace(openCodeChildExport, `"content":[`, `"content":[`+strings.Join(siblings, ",")+`,`, 1)
				}
				return []byte(value), nil
			})
			want := "bound exceeded"
			if mode == "cycle" {
				want = "repeated or cyclic"
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("want %q, got %v", want, err)
			}
		})
	}
}

func TestOpenCodeChildActionsDoNotBreakRootDispatchBatch(t *testing.T) {
	first := strings.SplitN(string(openCodeSpawnTrace("subagent", "ses_child1")), "\n", 2)[0]
	trace := first + "\n" + strings.ReplaceAll(first, "ses_child1", "ses_child2") + "\n" + `{"type":"text","part":{"text":"done"}}` + "\n" + `{"type":"step_finish","part":{"reason":"stop"}}`
	trace = strings.Replace(trace, `"id":"spawn1"`, `"id":"spawn2"`, 1)
	got, _, err := parseOpenCodeTraceWithChildren([]byte(trace), func(id string) ([]byte, error) {
		return []byte(strings.ReplaceAll(openCodeChildExport, "ses_child1", id)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	spawns, attempts, beforeWait, batch, matching := orchestrationEvidence(got.Events, 2)
	if spawns != 2 || attempts != 2 || !beforeWait || !batch || !matching {
		t.Fatalf("child work interrupted root dispatch: %+v", got.Events)
	}
	if _, _, err := parseOpenCodeTraceWithChildren([]byte(trace), nil); err == nil || !strings.Contains(err.Error(), "requires a session export") {
		t.Fatalf("silently dropped unavailable child export: %v", err)
	}
}

func TestOpenCodeChildExportCommand(t *testing.T) {
	r := actorRequest{Project: t.TempDir()}
	env := []string{"HOME=/isolated/home", "XDG_DATA_HOME=/isolated/data", "XDG_CONFIG_HOME=/arm/config", "XDG_CACHE_HOME=/arm/cache", "XDG_STATE_HOME=/arm/state"}
	command := openCodeExportCommand(context.Background(), "/real/opencode", r, env, "ses_child1")
	if !reflect.DeepEqual(command.Args, []string{"/real/opencode", "session", "export", "--standalone", "ses_child1"}) || !reflect.DeepEqual(command.Env, env) || command.Dir != r.Project {
		t.Fatalf("export escaped actor environment: %+v", command)
	}
}

func TestOpenCodeChildTracePersistence(t *testing.T) {
	out := t.TempDir()
	row := resultRow{CaseID: "case", BlockID: "block", Arm: "control", RunID: "run", Artifacts: map[string]string{}}
	children := map[string][]byte{"ses_child1": []byte(openCodeChildExport)}
	if err := writePrivateChildTraces(out, &row, children); err != nil {
		t.Fatal(err)
	}
	got, err := savedArtifact(out, "traces", ".child-ses_child1.json", row, "child_trace_ses_child1")
	if err != nil || string(got) != openCodeChildExport {
		t.Fatalf("child artifact: %s %v", got, err)
	}
	paths, _ := filepath.Glob(filepath.Join(out, "private", "traces", "*"))
	if len(paths) != 1 {
		t.Fatalf("traces: %v", paths)
	}
	info, err := os.Stat(paths[0])
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private mode: %v %v", info, err)
	}
	if err := os.WriteFile(paths[0], []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := savedArtifact(out, "traces", ".child-ses_child1.json", row, "child_trace_ses_child1"); err == nil {
		t.Fatal("unbound child actions accepted")
	}
}
