package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func skillReadTrace(t *testing.T, name, input, output string, failed bool) []string {
	t.Helper()
	call := map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_use", "id": "read-skill", "name": name, "input": json.RawMessage(input)},
	}}}
	result := map[string]any{"type": "user", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_result", "tool_use_id": "read-skill", "is_error": failed, "content": output},
	}}}
	var lines []string
	for _, record := range []any{call, result} {
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(data))
	}
	return lines
}

func codexSkillReadTrace(t *testing.T, kind, input, output string) []string {
	t.Helper()
	call := map[string]any{"type": kind, "call_id": "read-skill", "name": "exec_command"}
	if kind == "function_call" {
		call["arguments"] = input
	} else {
		call["input"] = input
	}
	result := map[string]any{"type": kind + "_output", "call_id": "read-skill", "output": output}
	var lines []string
	for _, record := range []any{call, result} {
		data, err := json.Marshal(map[string]any{"type": "response_item", "payload": record})
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(data))
	}
	return lines
}

func TestGateMatchesCodexReadResults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, kind, input, output string
		loaded                    bool
	}{
		{"exec", "function_call", `{"cmd":"cat /p/skills/safe-effects/SKILL.md"}`, `{"output":"---\nname: safe-effects\n---\n# Safe Effects\nCheck the target.","exit_code":0}`, true},
		{"failed exec", "function_call", `{"cmd":"cat /p/skills/safe-effects/SKILL.md"}`, `{"output":"---\nname: safe-effects\n---\n# Safe Effects\nCheck the target.","exit_code":1}`, false},
		{"custom", "custom_tool_call", "cat /p/skills/safe-effects/SKILL.md", "---\nname: safe-effects\n---\n# Safe Effects\nCheck the target.", true},
		{"wrapped code result", "custom_tool_call", "cat /p/skills/safe-effects/SKILL.md", `{"status":"fulfilled","value":{"output":"---\nname: safe-effects\n---\n# Safe Effects\nCheck the target.","exit_code":0}}`, true},
		{"search", "function_call", `{"cmd":"rg safe-effects /p/skills/safe-effects/SKILL.md"}`, `{"output":"name: safe-effects","exit_code":0}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := codexSkillReadTrace(t, tc.kind, tc.input, tc.output)
			loaded, readable := skillLoaded(writeTranscript(t, lines...), effectSkill)
			if loaded != tc.loaded || !readable {
				t.Fatalf("loaded=%v readable=%v, want loaded=%v", loaded, readable, tc.loaded)
			}
		})
	}
}

func TestGateRequiresSuccessfulSkillRead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, tool, input, output string
		failed, loaded            bool
	}{
		{"search", "Bash", `{"command":"rg safe-effects /p/skills/safe-effects/SKILL.md"}`, "name: safe-effects", false, false},
		{"failed read", "Read", `{"file_path":"/p/skills/safe-effects/SKILL.md"}`, "File not found", true, false},
		{"pending read", "Read", `{"file_path":"/p/skills/safe-effects/SKILL.md"}`, "", false, false},
		{"write", "Write", `{"file_path":"/p/skills/safe-effects/SKILL.md"}`, "---\nname: safe-effects\n---\n# Safe Effects\nCheck the target.", false, false},
		{"read", "Read", `{"file_path":"/p/skills/safe-effects/SKILL.md"}`, "---\nname: safe-effects\n---\n# Safe Effects\nCheck the target.", false, true},
		{"windows read", "Read", `{"file_path":"/p/skills/safe-effects/SKILL.md"}`, "---\r\nname: safe-effects\r\n---\r\n# Safe Effects\r\nCheck the target.", false, true},
		{"shell read", "Bash", `{"command":"cat /p/skills/safe-effects/SKILL.md"}`, "---\nname: safe-effects\n---\n# Safe Effects\nCheck the target.", false, true},
		{"wrong body", "Bash", `{"command":"cat /p/skills/safe-effects/SKILL.md"}`, "---\nname: unrelated\n---\n# Unrelated", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := skillReadTrace(t, tc.tool, tc.input, tc.output, tc.failed)
			if tc.name == "pending read" {
				lines = lines[:1]
			}
			path := writeTranscript(t, lines...)
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runGateWithTranscript(t, env, "read-proof", path, "gh pr comment 1 --body ready")
			if rc != 0 || stderr != "" {
				t.Fatalf("rc=%d stderr=%q", rc, stderr)
			}
			if tc.loaded {
				if stdout != "" {
					t.Fatalf("successful skill load was not recognized: %s", stdout)
				}
			} else if !strings.Contains(stdout, `"permissionDecision":"deny"`) {
				t.Fatalf("a search or unsuccessful read must not skip the gate: %q", stdout)
			}
		})
	}
}

func TestGateClassifiesNestedShellEffects(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		"bash -c 'git push'",
		"env sh -lc 'curl -X POST https://example.com'",
		`bash -c "sh -c 'git push'"`,
		"eval 'git push'",
		"printf '%s' \"$(git push)\"",
		"x=$((1<<20))\ngit push",
		"ssh host <<'EOF'\ngit push\nEOF",
		"ssh db1<<'EOF'\ngit push\nEOF",
		"out=$(sh <<'EOF'\ngit push\nEOF\n)",
		"cat <<'EOF' |& sh\ngit push\nEOF",
	} {
		t.Run(command, func(t *testing.T) {
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, _, _ := runGate(t, env, "nested-effect", command)
			if !strings.Contains(stdout, `"permissionDecision":"deny"`) {
				t.Fatalf("nested effect skipped its gate: %q", stdout)
			}
		})
	}
	for _, command := range []string{"bash -c 'git status'", `echo "bash -c 'git push'"`, "echo '$(git push)'"} {
		if got := gateSkills(command); len(got) != 0 {
			t.Errorf("data or read-only command received gates %v: %s", got, command)
		}
	}
}
