package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runSubagentStartHook(t *testing.T, env map[string]string, input string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"subagent-start"}, func(key string) string { return env[key] }, strings.NewReader(input), &stdout, &stderr)
	return stdout.String(), stderr.String(), rc
}

func decodeSubagentContext(t *testing.T, stdout string) string {
	t.Helper()
	var parsed struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("decode subagent-start output %q: %v", stdout, err)
	}
	if parsed.HookSpecificOutput.HookEventName != "SubagentStart" {
		t.Fatalf("hookEventName = %q, want SubagentStart", parsed.HookSpecificOutput.HookEventName)
	}
	return parsed.HookSpecificOutput.AdditionalContext
}

func TestSubagentStartInjectsReminderAndReportContract(t *testing.T) {
	t.Parallel()

	input := `{"session_id":"s","hook_event_name":"SubagentStart","agent_id":"agent-1","agent_type":"Explore"}`
	for _, harness := range []string{"claude", "codex"} {
		stdout, stderr, rc := runSubagentStartHook(t, map[string]string{"MEGAPOWERS_HARNESS": harness}, input)
		if rc != 0 || stderr != "" {
			t.Fatalf("%s: rc=%d stderr=%q", harness, rc, stderr)
		}
		context := decodeSubagentContext(t, stdout)
		for _, want := range []string{
			"Do not claim a skill without loading it.",
			"Lead with the result",
			"file:line",
			"em dashes",
		} {
			if !strings.Contains(context, want) {
				t.Errorf("%s: subagent context lacks %q:\n%s", harness, want, context)
			}
		}
		if strings.Contains(context, "helper code in Go") || strings.Contains(context, "\u2014") {
			t.Errorf("%s: subagent context carries forbidden content:\n%s", harness, context)
		}
		if len(context) > 1200 {
			t.Errorf("%s: subagent context is %d bytes, want compact", harness, len(context))
		}
	}
}

func TestSubagentStartHonorsStyleOptOut(t *testing.T) {
	t.Parallel()

	stdout, stderr, rc := runSubagentStartHook(t, map[string]string{"MEGAPOWERS_OUTPUT_STYLE": "off"}, `{"hook_event_name":"SubagentStart"}`)
	if rc != 0 || stderr != "" {
		t.Fatalf("rc=%d stderr=%q", rc, stderr)
	}
	context := decodeSubagentContext(t, stdout)
	if !strings.Contains(context, "Do not claim a skill without loading it.") || strings.Contains(context, "Lead with the result") {
		t.Fatalf("style opt-out must keep the skill reminder and drop the report contract:\n%s", context)
	}
}

func TestSubagentStartRejectsOversizedInput(t *testing.T) {
	t.Parallel()

	stdout, stderr, rc := runSubagentStartHook(t, map[string]string{}, strings.Repeat("x", maxHookInputBytes+1))
	if rc == 0 || stdout != "" || !strings.Contains(stderr, "cannot read hook input") {
		t.Fatalf("rc=%d stdout=%q stderr=%q", rc, stdout, stderr)
	}
}
