package contracts

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This invented CLI only accepts read-only commands. Responses are sequences
// so an empty first page can model location warm-up without wall-clock waits.
func fakeSessionMineOpenCode(t *testing.T, responses map[string][]string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	source := `package main
import ("encoding/json"; "os"; "os/exec"; "path/filepath"; "fmt"; "time")
func main() {
 if len(os.Args)==2 && os.Args[1]=="linger" { time.Sleep(3*time.Second); return }
 exe, _ := os.Executable(); dir := filepath.Dir(exe)
 dataFlags, _ := os.ReadFile(filepath.Join(dir,"flags.json")); var flags map[string]string; json.Unmarshal(dataFlags,&flags)
 envLog, _ := os.OpenFile(filepath.Join(dir,"environment"), os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600); json.NewEncoder(envLog).Encode(os.Environ()); envLog.Close()
 log, _ := os.OpenFile(filepath.Join(dir,"calls"), os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600)
 defer log.Close()
 if len(os.Args)==3 && os.Args[1]=="service" && os.Args[2]=="status" {
  fmt.Fprintln(log,"status")
  if flags["MINE_STATUS_DELAY"]=="1" { time.Sleep(30*time.Second) }
  if flags["MINE_REQUIRE_DEFAULT_STATE"]=="1" && os.Getenv("XDG_STATE_HOME")!="" { fmt.Println("stopped"); return }
  if flags["MINE_LINGER_STATUS"]=="1" { child:=exec.Command(os.Args[0],"linger"); child.Stdout=os.Stdout; child.Start() }
  fmt.Println("http://localhost:1234"); return
 }
 if len(os.Args)!=4 || os.Args[1]!="api" || os.Args[2]!="get" { os.Exit(3) }
 key := os.Args[3]; fmt.Fprintln(log,key)
 if flags["MINE_REQUIRE_DEFAULT_STATE"]=="1" && os.Getenv("XDG_STATE_HOME")!="" { os.Exit(5) }
 data,_ := os.ReadFile(filepath.Join(dir,"responses.json")); var responses map[string][]string; json.Unmarshal(data,&responses)
 seq,ok := responses[key]; if !ok || len(seq)==0 { os.Exit(4) }
 if seq[0]=="!fail" { os.Exit(4) }; if seq[0]=="!wait" { time.Sleep(30*time.Second) }
 output:=seq[0]; stat,_:=os.Stdout.Stat(); if !stat.Mode().IsRegular() && len(output)>40960 { output=output[:40960] }
 fmt.Print(output); if len(seq)>1 { responses[key]=seq[1:]; data,_=json.Marshal(responses); os.WriteFile(filepath.Join(dir,"responses.json"),data,0600) }
}`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(responses)
	if err := os.WriteFile(filepath.Join(dir, "responses.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(dir, "opencode-fixture")
	if output, code := run(t, dir, nil, "go", "build", "-o", command, filepath.Join(dir, "main.go")); code != 0 {
		t.Fatalf("build fake OpenCode (%d): %s", code, output)
	}
	return command, dir
}

func openCodeInfo(id, directory, parent string) map[string]any {
	return map[string]any{"id": id, "parentID": parent, "location": map[string]any{"directory": directory}, "time": map[string]any{"updated": 1793577600000}}
}

func openCodeMessage(kind, text string, content ...map[string]any) map[string]any {
	return map[string]any{"type": kind, "text": text, "time": map[string]any{"created": 1793577600000}, "content": content}
}

func openCodeJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func openCodeExport(info map[string]any, messages ...map[string]any) string {
	return openCodeJSON(map[string]any{"data": map[string]any{"info": info, "messages": messages}})
}

func runSessionMineStreams(t *testing.T, root string, env []string, tool string, args ...string) (string, string) {
	t.Helper()
	// Fixture controls live beside the fake CLI, outside its environment.
	fixture := ""
	for i, arg := range args {
		if arg == "--opencode-cmd" && i+1 < len(args) {
			fixture = filepath.Dir(args[i+1])
		}
	}
	if fixture != "" {
		flags := map[string]string{}
		for _, item := range env {
			key, value, _ := strings.Cut(item, "=")
			if strings.HasPrefix(key, "MINE_") {
				flags[key] = value
			}
		}
		data, _ := json.Marshal(flags)
		if err := os.WriteFile(filepath.Join(fixture, "flags.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(tool, args...)
	cmd.Dir, cmd.Env = root, append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("session-mine: %v\n%s\n%s", err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

func TestSessionMineOpenCode(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	parent := openCodeInfo("ses_parent", "/work/demo", "")
	child := openCodeInfo("ses_child", "/work/demo", "ses_parent")
	other := openCodeInfo("ses_other", "/work/demo-other", "")
	assistant := openCodeMessage("assistant", "", textBlock("VERIFIED: retry flag ready for dev.person@example.com"))
	assistant["model"] = map[string]any{"providerID": "claude-agent"}
	assistant["content"].([]map[string]any)[0]["native"] = map[string]any{"sessionID": "linked-claude"}
	unlinked := openCodeMessage("assistant", "", textBlock("Another provider's native ID is not a Claude transcript link."))
	unlinked["model"] = map[string]any{"providerID": "openai"}
	unlinked["content"].([]map[string]any)[0]["native"] = map[string]any{"sessionID": "unrelated-sdk"}
	toolCall := func(name string, input map[string]any) map[string]any {
		return map[string]any{"type": "tool", "name": name, "state": map[string]any{"status": "completed", "input": input}}
	}
	responses := map[string][]string{
		"/api/session?limit=100&order=desc": {
			`{"data":[],"cursor":{}}`,
			openCodeJSON(map[string]any{"data": []any{parent, other}, "cursor": map[string]any{"next": "page two"}}),
		},
		"/api/session?cursor=page+two&limit=100&order=desc": {openCodeJSON(map[string]any{"data": []any{child, parent}, "cursor": map[string]any{}})},
		"/api/experimental/session/ses_parent/export": {openCodeExport(parent,
			openCodeMessage("user", "Add the retry flag."),
			openCodeMessage("system", "too long system text"),
			openCodeMessage("assistant", "", toolCall("skill", map[string]any{"id": "megapowers:test-first-implementation"})),
			openCodeMessage("assistant", "", toolCall("shell", map[string]any{"command": "sleep 30"})),
			assistant,
			unlinked,
			openCodeMessage("synthetic", "Continue the retry flag job."),
			openCodeMessage("user", "Daily check.\nUnattended run: keep going."),
			openCodeMessage("user", "Too long, shorter please at dev.person@example.com /home/alice/project."),
			openCodeMessage("user", "Still too long. <context>don't ask private</context>"),
		)},
		"/api/experimental/session/ses_child/export": {openCodeExport(child, openCodeMessage("user", "Keep going child retry flag"), openCodeMessage("assistant", "", textBlock("Child done.")))},
		"/api/experimental/session/ses_other/export": {openCodeExport(other, openCodeMessage("user", "Did you test otherworkspace?"))},
	}
	command, fixtures := fakeSessionMineOpenCode(t, responses)
	env := []string{"MINE_FIXTURE=" + fixtures, "SESSION_MINE_OPENCODE_CMD=" + command}
	claudeRoot, codexRoot := filepath.Join(t.TempDir(), "claude"), t.TempDir()
	linked := claudeUserIn("/work/demo", "2026-11-02T00:00:00Z", "Too long duplicated SDK turn")
	var record map[string]any
	json.Unmarshal([]byte(linked), &record)
	record["sessionId"], record["entrypoint"] = "linked-claude", "sdk-ts"
	writeLines(t, filepath.Join(claudeRoot, slugFor("/work/demo"), "renamed.jsonl"), openCodeJSON(record))
	record["sessionId"] = "unrelated-sdk"
	writeLines(t, filepath.Join(claudeRoot, slugFor("/work/demo"), "unrelated.jsonl"), openCodeJSON(record))
	base := []string{"--workspace", "/work/demo", "--since", "2026-11-01", "--claude-root", claudeRoot, "--codex-root", codexRoot, "--format", "json"}
	output, stderr := runSessionMineStreams(t, root, env, tool, append([]string{"corrections"}, base...)...)
	var report struct {
		sessionMineCorrections
		MachineTurns int `json:"machine_turns"`
		Deduplicated int `json:"deduplicated_sessions"`
		BySource     map[string]struct {
			Sessions int `json:"sessions_scanned"`
			Prompts  int `json:"prompt_turns"`
		} `json:"by_source"`
	}
	if json.Unmarshal([]byte(output), &report) != nil || report.SessionsScanned != 3 || report.HumanTurns != 4 || report.MachineTurns != 2 || report.Deduplicated != 1 || report.BySource["opencode"].Sessions != 2 || report.BySource["opencode"].Prompts != 1 {
		t.Fatalf("wrong source/role/dedupe counts: %s\nstderr: %s", output, stderr)
	}
	if strings.Contains(output, "dev.person") || strings.Contains(output, "alice") || strings.Contains(output, "private") || strings.Contains(output, "otherworkspace") || !strings.Contains(output, "<email>") {
		t.Errorf("redaction/scoping: %s", output)
	}
	if verbosity := findClass(report.Classes, "verbosity"); verbosity == nil || verbosity.Count != 3 {
		t.Errorf("verbosity count (including unrelated SDK) wrong: %s", output)
	}
	output, _ = runSessionMineStreams(t, root, env, tool, append([]string{"scorecard", "--harness", "opencode"}, base...)...)
	var card scorecard
	if json.Unmarshal([]byte(output), &card) != nil || card.HumanTurns != 3 || card.SleepCalls != 1 || card.Reports.Verified != 1 || len(card.Skills) != 1 || card.Skills[0].FollowedByCorrection != 1 {
		t.Errorf("scorecard wrong: %s", output)
	}
	output, _ = runSessionMineStreams(t, root, env, tool, append([]string{"recall", "--topic", "child retry flag", "--harness", "opencode"}, base...)...)
	var recalled sessionMineRecall
	if json.Unmarshal([]byte(output), &recalled) != nil || len(recalled.Sessions) != 1 || recalled.Sessions[0].FirstPrompt != "Keep going child retry flag" {
		t.Errorf("child recall wrong: %s", output)
	}
	output, _ = runSessionMineStreams(t, root, env, tool, "corrections", "--harness", "opencode", "--all-workspaces", "--opencode-cmd", command, "--format", "json")
	if json.Unmarshal([]byte(output), &report) != nil || report.BySource["opencode"].Sessions != 3 {
		t.Errorf("all-workspaces: %s", output)
	}
	beforeBoth, _ := os.ReadFile(filepath.Join(fixtures, "calls"))
	output, _ = runSessionMineStreams(t, root, env, tool, append([]string{"corrections", "--harness", "both"}, base...)...)
	if json.Unmarshal([]byte(output), &report) != nil || report.SessionsScanned != 2 || report.Deduplicated != 0 {
		t.Errorf("both compatibility: %s", output)
	}
	afterBoth, _ := os.ReadFile(filepath.Join(fixtures, "calls"))
	if !bytes.Equal(beforeBoth, afterBoth) {
		t.Error("--harness both must not call OpenCode")
	}
}

func TestSessionMineOpenCodeUnavailable(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	claudeRoot := t.TempDir()
	writeLines(t, filepath.Join(claudeRoot, slugFor("/work/demo"), "human.jsonl"), claudeUser("2026-11-02T00:00:00Z", "Did you test it?"))
	output, stderr := runSessionMineStreams(t, root, nil, tool, "--workspace", "/work/demo", "--opencode-cmd", filepath.Join(t.TempDir(), "missing"), "--claude-root", claudeRoot, "--codex-root", t.TempDir(), "--format", "json")
	var report struct {
		Sessions int `json:"sessions_scanned"`
		Skipped  int `json:"skipped_sources"`
	}
	if json.Unmarshal([]byte(output), &report) != nil || report.Sessions != 1 || report.Skipped != 1 || !strings.Contains(stderr, "opencode") {
		t.Fatalf("unavailable source: %s\n%s", output, stderr)
	}
}

func TestSessionMineOpenCodeFailuresAndEvents(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	info := openCodeInfo("ses_events", "/work/demo", "")
	gate := "Load and follow the megapowers verify-and-finish skill before finishing"
	errorTool := map[string]any{"type": "tool", "name": "shell", "state": map[string]any{
		"status": "error", "input": map[string]any{"command": "sleep 30", "background": true},
		"error": map[string]any{"message": "Command failed"}, "content": []any{textBlock(gate)},
	}}
	old := openCodeMessage("user", "Too long old correction")
	old["time"] = map[string]any{"created": 1767225600000}
	exported := openCodeExport(info,
		old, openCodeMessage("user", "Run the retry job."),
		openCodeMessage("assistant", "", errorTool),
		openCodeMessage("assistant", "", map[string]any{"type": "reasoning", "text": "too long hidden"}, textBlock("Ready.")),
		openCodeMessage("assistant", "", textBlock(strings.Repeat("extra ", 10000))),
	)
	for _, scenario := range []string{"events and retry", "truncated list", "unreachable API", "failed export", "repeated cursor"} {
		t.Run(scenario, func(t *testing.T) {
			page := openCodeJSON(map[string]any{"data": []any{info}, "cursor": map[string]any{}})
			responses := map[string][]string{
				"/api/session?limit=100&order=desc":           {page},
				"/api/experimental/session/ses_events/export": {`{"data":null}`, exported},
			}
			switch scenario {
			case "unreachable API":
				responses["/api/session?limit=100&order=desc"] = nil
			case "truncated list":
				responses["/api/session?limit=100&order=desc"] = []string{`{"data":[`}
			case "failed export":
				responses["/api/experimental/session/ses_events/export"] = []string{`{"data":{}}`}
			case "repeated cursor":
				page = openCodeJSON(map[string]any{"data": []any{info}, "cursor": map[string]any{"next": "repeat"}})
				responses["/api/session?limit=100&order=desc"] = []string{page}
				responses["/api/session?cursor=repeat&limit=100&order=desc"] = []string{page}
			}
			command, fixture := fakeSessionMineOpenCode(t, responses)
			output, stderr := runSessionMineStreams(t, root, []string{"MINE_FIXTURE=" + fixture}, tool, "scorecard", "--harness", "opencode", "--opencode-cmd", command, "--workspace", "/work/demo", "--since", "2026-11-01", "--format", "json")
			var card struct {
				scorecard
				SkippedSources  int `json:"skipped_sources"`
				SkippedSessions int `json:"skipped_sessions"`
				PartialSources  int `json:"partial_sources"`
			}
			if err := json.Unmarshal([]byte(output), &card); err != nil {
				t.Fatalf("JSON: %v\n%s", err, output)
			}
			switch scenario {
			case "events and retry":
				if card.HumanTurns != 1 || card.SleepCalls != 0 || card.Gates["verify-and-finish"] != 1 || card.SkippedSources != 0 || card.SkippedSessions != 0 || stderr != "" {
					t.Errorf("events/since/retry: %s\n%s", output, stderr)
				}
			case "truncated list":
				if card.SkippedSources != 1 || card.SessionsScanned != 0 || !strings.Contains(stderr, "truncated") {
					t.Errorf("truncated: %s\n%s", output, stderr)
				}
			case "unreachable API":
				if card.SkippedSources != 1 || card.SessionsScanned != 0 || !strings.Contains(stderr, "API GET failed") {
					t.Errorf("unreachable API: %s\n%s", output, stderr)
				}
			case "failed export":
				if card.SkippedSessions != 1 || card.PartialSources != 1 || card.SessionsScanned != 0 || !strings.Contains(stderr, "partial") {
					t.Errorf("export failure: %s\n%s", output, stderr)
				}
			case "repeated cursor":
				if card.SkippedSources != 0 || card.PartialSources != 1 || card.SessionsScanned != 1 || !strings.Contains(stderr, "pagination") {
					t.Errorf("cursor: %s\n%s", output, stderr)
				}
			}
			calls, _ := os.ReadFile(filepath.Join(fixture, "calls"))
			if !strings.HasPrefix(string(calls), "status\n") || strings.Contains(string(calls), "start") {
				t.Errorf("unexpected CLI operations: %s", calls)
			}
		})
	}
}

func TestSessionMineOpenCodeStateFallbackAndBudgets(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	info := openCodeInfo("ses_budget", "/work/demo", "")
	listPath := "/api/session?limit=100&order=desc"
	nextPath := "/api/session?cursor=next&limit=100&order=desc"
	exportPath := "/api/experimental/session/ses_budget/export"
	for _, scenario := range []string{"state fallback", "inherited state", "empty retry fails", "empty retry timeout", "later page fails", "later export fails", "first request timeout", "later request timeout", "status timeout", "status pipe wait"} {
		t.Run(scenario, func(t *testing.T) {
			page := openCodeJSON(map[string]any{"data": []any{info}, "cursor": map[string]any{}})
			responses := map[string][]string{listPath: {page}, exportPath: {openCodeExport(info, openCodeMessage("user", "Did you test it?"))}}
			env := []string{"XDG_STATE_HOME=/invented/state"}
			wantSessions, wantPartial, wantSkipped, wantCalls := 1, 0, 0, 1
			budget := "5m"
			switch scenario {
			case "state fallback":
				env = append(env, "MINE_REQUIRE_DEFAULT_STATE=1")
				wantCalls = 2
			case "empty retry fails":
				responses[listPath] = []string{`{"data":[],"cursor":{}}`, "!fail"}
				wantSessions = 0
			case "empty retry timeout":
				responses[listPath] = []string{`{"data":[],"cursor":{}}`, "!wait"}
				budget, wantSessions, wantPartial = "150ms", 0, 1
			case "later page fails", "later request timeout":
				responses[listPath] = []string{openCodeJSON(map[string]any{"data": []any{info}, "cursor": map[string]any{"next": "next"}})}
				responses[nextPath] = []string{"!fail"}
				wantPartial = 1
				if scenario == "later request timeout" {
					responses[nextPath] = []string{"!wait"}
					budget = "150ms"
				}
			case "later export fails":
				other := openCodeInfo("ses_failed", "/work/demo", "")
				responses[listPath] = []string{openCodeJSON(map[string]any{"data": []any{info, other}, "cursor": map[string]any{}})}
				responses["/api/experimental/session/ses_failed/export"] = []string{"!fail"}
				wantPartial = 1
			case "first request timeout":
				responses[listPath] = []string{"!wait"}
				budget = "150ms"
				wantPartial = 1
				wantSessions = 0
			case "status timeout":
				env = append(env, "MINE_STATUS_DELAY=1")
				budget = "150ms"
				wantSessions = 0
				wantPartial = 1
			case "status pipe wait":
				env = append(env, "MINE_LINGER_STATUS=1")
				wantSessions = 0
				wantSkipped = 1
			}
			command, fixture := fakeSessionMineOpenCode(t, responses)
			env = append(env, "MINE_FIXTURE="+fixture, "SESSION_MINE_OPENCODE_TIMEOUT="+budget)
			start := time.Now()
			output, stderr := runSessionMineStreams(t, root, env, tool, "--harness", "opencode", "--all-workspaces", "--opencode-cmd", command, "--format", "json")
			elapsed := time.Since(start)
			var report struct {
				Sessions int `json:"sessions_scanned"`
				Partial  int `json:"partial_sources"`
				Skipped  int `json:"skipped_sources"`
				BySource map[string]struct {
					Status      string `json:"status"`
					Environment string `json:"environment"`
				} `json:"by_source"`
			}
			if json.Unmarshal([]byte(output), &report) != nil || report.Sessions != wantSessions || report.Partial != wantPartial || report.Skipped != wantSkipped {
				t.Fatalf("wrong result: %s\n%s", output, stderr)
			}
			if scenario == "state fallback" && (report.BySource["opencode"].Environment != "without_xdg_state_home" || !strings.Contains(stderr, "XDG_STATE_HOME")) {
				t.Errorf("fallback not reported: %s\n%s", output, stderr)
			}
			if scenario == "inherited state" && report.BySource["opencode"].Environment != "inherited" {
				t.Errorf("inherited mode missing: %s", output)
			}
			if wantPartial == 1 && (report.BySource["opencode"].Status != "partial" || !strings.Contains(stderr, "partial")) {
				t.Errorf("partial not reported: %s\n%s", output, stderr)
			}
			calls, _ := os.ReadFile(filepath.Join(fixture, "calls"))
			if strings.Count(string(calls), "status\n") != wantCalls {
				t.Errorf("status attempts: %s", calls)
			}
			if strings.Contains(scenario, "timeout") {
				if elapsed > 2*time.Second {
					t.Errorf("source deadline exceeded: %s", elapsed)
				}
				path := listPath
				if scenario == "later request timeout" {
					path = nextPath
				}
				if scenario == "status timeout" {
					path = "status"
				}
				attempts := 1
				if scenario == "empty retry timeout" {
					attempts = 2
				}
				if strings.Count(string(calls), path+"\n") != attempts {
					t.Errorf("timed-out attempt retried: %s", calls)
				}
			}
			if scenario == "status pipe wait" && elapsed > 2500*time.Millisecond {
				t.Errorf("leftover child's pipe blocked Wait: %s", elapsed)
			}
		})
	}
}

func TestSessionMineOpenCodeDedupeScopeAndRange(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	for _, scenario := range []string{"slug collision", "no in-range OpenCode messages"} {
		t.Run(scenario, func(t *testing.T) {
			workspace := "/work/a-b"
			info := openCodeInfo("ses_linked", workspace, "")
			assistant := openCodeMessage("assistant", "", textBlock("Old reply"))
			assistant["model"] = map[string]any{"providerID": "claude-agent"}
			assistant["content"].([]map[string]any)[0]["native"] = map[string]any{"sessionID": "claude-linked"}
			cwd := workspace
			want := 1
			if scenario == "slug collision" {
				cwd = "/work/a/b"
			} else {
				assistant["time"] = map[string]any{"created": 1767225600000}
			}
			command, fixture := fakeSessionMineOpenCode(t, map[string][]string{
				"/api/session?limit=100&order=desc":           {openCodeJSON(map[string]any{"data": []any{info}, "cursor": map[string]any{}})},
				"/api/experimental/session/ses_linked/export": {openCodeExport(info, assistant)},
			})
			claudeRoot := t.TempDir()
			var record map[string]any
			json.Unmarshal([]byte(claudeUserIn(cwd, "2026-11-02T00:00:00Z", "Did you test this?")), &record)
			record["sessionId"] = "claude-linked"
			writeLines(t, filepath.Join(claudeRoot, slugFor(workspace), "linked.jsonl"), `{"type":"system","sessionId":"claude-linked"}`, openCodeJSON(record))
			output, stderr := runSessionMineStreams(t, root, []string{"MINE_FIXTURE=" + fixture}, tool, "--workspace", workspace, "--since", "2026-11-01", "--opencode-cmd", command, "--claude-root", claudeRoot, "--codex-root", t.TempDir(), "--format", "json")
			var report struct {
				Sessions int `json:"sessions_scanned"`
				Dedupe   int `json:"deduplicated_sessions"`
				BySource map[string]struct {
					Sessions int `json:"sessions_scanned"`
				} `json:"by_source"`
			}
			if json.Unmarshal([]byte(output), &report) != nil || report.Sessions != want || report.Dedupe != 0 {
				t.Fatalf("scope/range dedupe: %s\n%s", output, stderr)
			}
			if scenario == "no in-range OpenCode messages" && (report.BySource["claude"].Sessions != 1 || report.BySource["opencode"].Sessions != 0) {
				t.Errorf("linked Claude transcript lost: %s", output)
			}
		})
	}
}

func TestSessionMineOpenCodeTimeoutOptions(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	output, code := run(t, root, []string{"SESSION_MINE_OPENCODE_TIMEOUT=invalid"}, tool, "--all-workspaces", "--harness", "opencode", "--opencode-timeout", "2s", "--format", "json")
	if code != 0 {
		t.Fatalf("timeout flag must override env (%d): %s", code, output)
	}
	for _, value := range []string{"invalid", "0s", "-1s"} {
		output, code = run(t, root, []string{"SESSION_MINE_OPENCODE_TIMEOUT=" + value}, tool, "--all-workspaces", "--harness", "opencode")
		if code != 2 || !strings.Contains(output, "positive duration") {
			t.Errorf("invalid timeout %q (%d): %s", value, code, output)
		}
	}
}

func TestSessionMineOpenCodeDeadlineRetainsOtherSources(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	command, fixture := fakeSessionMineOpenCode(t, map[string][]string{"/api/session?limit=100&order=desc": {"!wait"}})
	claudeRoot := t.TempDir()
	writeLines(t, filepath.Join(claudeRoot, slugFor("/work/demo"), "human.jsonl"), claudeUser("2026-11-02T00:00:00Z", "Did you test it?"))
	output, stderr := runSessionMineStreams(t, root, []string{"MINE_FIXTURE=" + fixture}, tool, "--workspace", "/work/demo", "--opencode-cmd", command, "--opencode-timeout", "150ms", "--claude-root", claudeRoot, "--codex-root", t.TempDir(), "--format", "json")
	var report struct {
		Sessions int `json:"sessions_scanned"`
		Partial  int `json:"partial_sources"`
		Human    int `json:"human_turns"`
	}
	if json.Unmarshal([]byte(output), &report) != nil || report.Sessions != 1 || report.Partial != 1 || report.Human != 1 || !strings.Contains(stderr, "partial") {
		t.Errorf("deadline lost other sources: %s\n%s", output, stderr)
	}
}

func TestSessionMineInstalledSingleFile(t *testing.T) {
	root := repoRoot(t)
	_ = buildSessionMine(t, root) // supplies an isolated, empty OpenCode CLI
	dir := t.TempDir()
	source, err := os.ReadFile(filepath.Join(root, sessionMineTool))
	if err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(dir, "session-mine.go")
	if err := os.WriteFile(installed, source, 0o600); err != nil {
		t.Fatal(err)
	}
	output, code := run(t, dir, nil, "go", "run", installed, "corrections", "--harness", "opencode", "--all-workspaces", "--format", "json")
	var report sessionMineCorrections
	if code != 0 || json.Unmarshal([]byte(output), &report) != nil || report.SessionsScanned != 0 {
		t.Fatalf("standalone installed copy (%d): %s", code, output)
	}
}

func TestSessionMineOpenCodeEnvironment(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	command, fixture := fakeSessionMineOpenCode(t, map[string][]string{"/api/session?limit=100&order=desc": {`{"data":[],"cursor":{}}`}})
	names := []string{"PATH", "HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR", "TMPDIR"}
	expected := map[string]string{}
	env := []string{"DEMO_PASSWORD=sentinel", "DEMO_TOKEN=sentinel", "DEMO_KEY=sentinel", "DEMO_SECRET=sentinel", "DEMO_AUTH=sentinel", "UNRELATED=sentinel"}
	for _, name := range names {
		value := filepath.Join(t.TempDir(), name)
		if name == "PATH" || name == "HOME" || name == "TMPDIR" {
			value = os.Getenv(name)
		}
		expected[name] = value
		env = append(env, name+"="+value)
	}
	output, _ := runSessionMineStreams(t, root, env, tool, "--harness", "opencode", "--all-workspaces", "--opencode-cmd", command, "--format", "json")
	if !strings.Contains(output, `"status": "complete"`) {
		t.Fatalf("source incomplete: %s", output)
	}
	data, err := os.ReadFile(filepath.Join(fixture, "environment"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("want status and two API calls, got %d", len(lines))
	}
	for _, line := range lines {
		var items []string
		if err := json.Unmarshal([]byte(line), &items); err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, item := range items {
			key, value, _ := strings.Cut(item, "=")
			got[key] = value
			if want, ok := expected[key]; !ok || value != want {
				t.Errorf("unexpected child variable %s", key)
			}
		}
		for key, want := range expected {
			if got[key] != want {
				t.Errorf("missing child variable %s", key)
			}
		}
	}
}
