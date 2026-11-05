package main

import (
	"bytes"
	"strings"
	"testing"
)

func runMCPGate(t *testing.T, env map[string]string, session, transcript, tool, toolInput string) (string, string, int) {
	t.Helper()
	input := `{"session_id":"` + session + `","transcript_path":` + strconvQuote(transcript) + `,"cwd":"/work","hook_event_name":"PreToolUse","tool_name":` + strconvQuote(tool) + `,"tool_input":` + toolInput + `}`
	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"deny-destructive"}, func(key string) string { return env[key] }, strings.NewReader(input), &stdout, &stderr)
	return stdout.String(), stderr.String(), rc
}

func TestGateStopsFirstMCPWriteOncePerSession(t *testing.T) {
	t.Parallel()

	for _, tool := range []string{
		"mcp__claude_ai_Linear__save_comment",
		"mcp__claude_ai_Linear__save_issue",
		"mcp__linear__save_project",
		"mcp__plugin_linear_linear__create_comment",
		"mcp__claude_ai_Slack__slack_send_message",
		"mcp__slack__update_canvas",
		"mcp__railway__redeploy",
		"mcp__railway__service_restart",
		"mcp__github__createPullRequest",
		"mcp__github__add_issue_comment",
		"mcp__github__merge_pull_request",
		"mcp__notion__notion-create-pages",
		"mcp__gmail__send_email",
		"mcp__stripe__cancel_subscription",
		"mcp__drive__upload_file",
		"mcp__linear__delete_comment",
		"mcp__supabase__apply_migration",
		"mcp__supabase__execute_sql",
		"mcp__postgres__run_sql",
		"mcp__db__drop_table",
	} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			first, stderr, rc := runMCPGate(t, env, "session-mcp", "", tool, `{"issueId":"ABC-1","body":"hi"}`)
			parsed := decodeGate(t, first)
			if rc != 0 || stderr != "" || parsed.HookSpecificOutput.PermissionDecision != "deny" || !strings.Contains(parsed.HookSpecificOutput.PermissionDecisionReason, "megapowers safe-effects skill") {
				t.Fatalf("first MCP write must be denied naming safe-effects: rc=%d stdout=%q stderr=%q", rc, first, stderr)
			}
			retry, stderr, rc := runMCPGate(t, env, "session-mcp", "", tool, `{"issueId":"ABC-1","body":"hi"}`)
			if rc != 0 || retry != "" || stderr != "" {
				t.Fatalf("retry must run silently: rc=%d stdout=%q stderr=%q", rc, retry, stderr)
			}
		})
	}
}

func TestGateLetsMCPReadsPassSilently(t *testing.T) {
	t.Parallel()

	for _, tool := range []string{
		"mcp__claude_ai_Linear__list_issues",
		"mcp__claude_ai_Linear__get_issue",
		"mcp__linear__get_comment",
		"mcp__gitlab__list_merge_requests",
		"mcp__slack__slack_search_public",
		"mcp__slack__slack_read_channel",
		"mcp__axiom__queryApl",
		"mcp__context7__resolve-library-id",
		"mcp__context7__query-docs",
		"mcp__railway__fetch_logs",
		"mcp__aws__describe_instances",
		"mcp__docs__find_page",
		"mcp__dns__lookup_record",
		"mcp__drive__view_file",
		"mcp__playwright__browser_snapshot",
		"mcp__playwright__browser_file_upload",
		"mcp__plugin_playwright_playwright__browser_file_upload",
		"mcp__chrome-devtools__upload_file",
		"mcp__puppeteer__set_viewport",
		"mcp__linear",
		"mcp__linear__",
		"mcp____save_issue_without_server",
	} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runMCPGate(t, env, "session-read", "", tool, `{"query":"x","command":["not","a","string"]}`)
			if tool == "mcp____save_issue_without_server" {
				// A malformed name still has an action part; it must not error.
				if rc != 0 || stderr != "" {
					t.Fatalf("rc=%d stderr=%q, want no error", rc, stderr)
				}
				return
			}
			if rc != 0 || stdout != "" || stderr != "" {
				t.Fatalf("rc=%d stdout=%q stderr=%q, want silence", rc, stdout, stderr)
			}
		})
	}
}

// MCP and shell effects share one safe-effects marker: a load or denial on
// either path covers the other.
func TestGateMCPAndShellShareTheEffectMarker(t *testing.T) {
	t.Parallel()

	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	runGate(t, env, "session-share", "gh pr comment 1 --body a")
	if stdout, _, _ := runMCPGate(t, env, "session-share", "", "mcp__linear__save_comment", `{}`); stdout != "" {
		t.Fatalf("MCP write after the shell effect denial must pass: %q", stdout)
	}
	transcript := writeTranscript(t, skillReadTrace(t, "Skill", `{"skill":"megapowers:safe-effects"}`, "Launching skill: megapowers:safe-effects", false)...)
	fresh := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	if stdout, _, _ := runMCPGate(t, fresh, "session-loaded", transcript, "mcp__linear__save_comment", `{}`); stdout != "" {
		t.Fatalf("MCP write after the skill loaded must pass: %q", stdout)
	}
}

func TestGateMCPParallelSiblingIsDenied(t *testing.T) {
	t.Parallel()

	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	transcript := writeTranscript(t, `{"type":"user","message":{"content":"update the tickets"}}`)
	runMCPGate(t, env, "session-sib", transcript, "mcp__linear__save_issue", `{}`)
	sibling, _, _ := runMCPGate(t, env, "session-sib", transcript, "mcp__linear__save_comment", `{}`)
	if decodeGate(t, sibling).HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("parallel MCP sibling must be denied: %q", sibling)
	}
}

// A tool named for execution (execute_sql, run_sql, exec) runs whatever its
// input says, so the input decides: a read-only query passes, anything else
// keeps the safe-effects gate.
func TestGateMCPExecutionToolsReadTheirInput(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, tool, input string
		deny              bool
	}{
		{"stats select", "mcp__db__execute_sql", `{"sql":"SELECT relname, n_live_tup FROM pg_stat_user_tables WHERE schemaname='media'; SELECT count(*) FROM characters.character"}`, false},
		{"lowercase with cte", "mcp__db-brain__execute_sql", `{"sql":"with t as (select 1) select * from t"}`, false},
		{"explain", "mcp__postgres__run_sql", `{"project_id":"p","sql":"EXPLAIN SELECT 1"}`, false},
		{"literal holding a write word", "mcp__db__execute_sql", `{"sql":"SELECT * FROM audit WHERE action = 'delete'"}`, false},
		{"posthog read call", "mcp__beef_posthog__exec", `{"command":"call execute-sql {\"query\": \"WITH k AS (SELECT user_id FROM t WHERE deleted_at IS NULL) SELECT count(*) FROM k\"}"}`, false},
		{"posthog info", "mcp__foxy-posthog__exec", `{"command":"info execute-sql"}`, false},
		{"posthog learn", "mcp__foxy-posthog__exec", `{"command":"learn -s querying"}`, false},
		{"posthog read tool", "mcp__foxy-posthog__exec", `{"command":"call read-data-schema {}"}`, false},
		{"posthog learn phrase", "mcp__beef_posthog__exec", `{"command":"learn -s \"web vitals per page\""}`, false},
		{"posthog call with json flag", "mcp__beef_posthog__exec", `{"command":"call --json view-list {\"limit\": 200}"}`, false},
		{"posthog retrieve", "mcp__beef_posthog__exec", `{"command":"call external-data-schemas-retrieve {\"id\":\"x\"}"}`, false},
		{"statement timeout then select", "mcp__db__execute_sql", `{"sql":"SET statement_timeout = '5s'; SELECT 1"}`, false},
		{"search with pipe", "mcp__beef_posthog__exec", `{"command":"search a|rm -rf x"}`, true},
		{"set role", "mcp__db__execute_sql", `{"sql":"SET ROLE admin; SELECT 1"}`, true},
		{"comment hides write behind timeout", "mcp__db__execute_sql", `{"sql":"/* SET statement_timeout */ DELETE FROM accounts; SELECT 1"}`, true},
		{"line comment hides write", "mcp__db__execute_sql", `{"sql":"-- SET statement_timeout\nDELETE FROM accounts; SELECT 1"}`, true},
		{"comment before select", "mcp__db__execute_sql", `{"sql":"-- table stats\nSELECT 1"}`, false},
		{"shell syntax in flag", "mcp__beef_posthog__exec", `{"command":"call --flag;>gate-write;# retrieve {}"}`, true},
		{"insert", "mcp__db__execute_sql", `{"sql":"INSERT INTO t VALUES (1)"}`, true},
		{"select then delete", "mcp__db__execute_sql", `{"sql":"SELECT 1; DELETE FROM t"}`, true},
		{"cte write", "mcp__db__execute_sql", `{"sql":"WITH d AS (DELETE FROM t RETURNING *) SELECT * FROM d"}`, true},
		{"select into", "mcp__db__execute_sql", `{"sql":"SELECT * INTO backup FROM t"}`, true},
		{"side-effect function", "mcp__db__execute_sql", `{"sql":"SELECT pg_terminate_backend(42)"}`, true},
		{"row lock", "mcp__db__execute_sql", `{"sql":"SELECT * FROM t FOR UPDATE"}`, true},
		{"temp view", "mcp__db__execute_sql", `{"sql":"create temp view v as select 1"}`, true},
		{"posthog write call", "mcp__beef_posthog__exec", `{"command":"call insight-create {\"name\": \"x\"}"}`, true},
		{"posthog write sql", "mcp__beef_posthog__exec", `{"command":"call execute-sql {\"query\": \"DROP TABLE t\"}"}`, true},
		{"unknown exec command", "mcp__beef_posthog__exec", `{"command":"call dashboard-insights-run {}"}`, true},
		{"no recognizable input", "mcp__supabase__execute_sql", `{"issueId":"ABC-1","body":"hi"}`, true},
		{"non-execution write tool ignores input", "mcp__linear__save_issue", `{"sql":"SELECT 1"}`, true},
		{"large object function", "mcp__db__execute_sql", `{"sql":"SELECT lo_create(0)"}`, true},
		{"unicode escaped identifier", "mcp__db__execute_sql", `{"sql":"SELECT U&\"next\\0076al\"('s')"}`, true},
		{"dblink", "mcp__db__execute_sql", `{"sql":"SELECT dblink_connect('x')"}`, true},
		{"decoy command beside sql", "mcp__db__execute_sql", `{"command":"help","sql":"DELETE FROM accounts"}`, true},
		{"decoy sql beside query", "mcp__db__execute_sql", `{"sql":"SELECT 1","query":"DELETE FROM accounts"}`, true},
		{"shell chain after meta verb", "mcp__shell__exec", `{"command":"help ; touch /var/tmp/x"}`, true},
		{"substitution after meta verb", "mcp__shell__exec", `{"command":"info $(touch x)"}`, true},
		{"call with non-json args", "mcp__beef_posthog__exec", `{"command":"call read-data-schema ; rm -rf x"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runMCPGate(t, env, "session-exec", "", tc.tool, tc.input)
			if rc != 0 || stderr != "" {
				t.Fatalf("rc=%d stderr=%q", rc, stderr)
			}
			if denied := stdout != "" && decodeGate(t, stdout).HookSpecificOutput.PermissionDecision == "deny"; denied != tc.deny {
				t.Fatalf("deny=%v, want %v: %q", denied, tc.deny, stdout)
			}
		})
	}
}
