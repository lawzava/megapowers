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
	transcript := writeTranscript(t, `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"megapowers:safe-effects"}}]}}`)
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
