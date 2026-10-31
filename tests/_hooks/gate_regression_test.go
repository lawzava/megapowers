package main

import (
	"encoding/json"
	"fmt"
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

// batchedExecTrace mirrors a Codex code-mode `exec` call whose script runs
// several commands through Promise.allSettled and prints one JSON block per
// command. Recorded traces from 2026-10-13 spread each settled result
// ({"i":N,"status":...,"value":...}); scripts may also nest it under "result".
func batchedExecTrace(t *testing.T, script string, blocks ...string) []string {
	t.Helper()
	output := []any{map[string]any{"type": "input_text", "text": "Script completed\nWall time 1.3 seconds\nOutput:\n"}}
	for _, block := range blocks {
		output = append(output, map[string]any{"type": "input_text", "text": block})
	}
	call := map[string]any{"type": "custom_tool_call", "call_id": "batch", "name": "exec", "input": script}
	result := map[string]any{"type": "custom_tool_call_output", "call_id": "batch", "output": output}
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

func settledBlock(t *testing.T, index int, status string, exitCode int, output string) string {
	t.Helper()
	result := map[string]any{"status": status}
	value := map[string]any{"chunk_id": "c", "wall_time_seconds": 0.04, "exit_code": exitCode, "output": output}
	if status == "rejected" {
		result["reason"] = output
	} else {
		result["value"] = value
	}
	data, err := json.Marshal(map[string]any{"i": index, "result": result})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// spreadSettled rewrites a {"i":N,"result":{...}} block into the
// {"i":N,...result} form that text({i,...r}) prints.
func spreadSettled(t *testing.T, block string) string {
	t.Helper()
	var nested struct {
		I      int            `json:"i"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal([]byte(block), &nested); err != nil {
		t.Fatal(err)
	}
	nested.Result["i"] = nested.I
	data, err := json.Marshal(nested.Result)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestGateMatchesBatchedExecSkillReads(t *testing.T) {
	t.Parallel()
	const body = "---\nname: safe-effects\ndescription: Use when preparing a deploy.\n---\n\n# Safe Effects\n\nCheck the target."
	const script = "const results=await Promise.allSettled([\n tools.exec_command({cmd:\"go run run-receipt.go -- pnpm install\"}),\n tools.exec_command({cmd:\"git diff --stat\"}),\n tools.exec_command({cmd:\"cat /p/skills/safe-effects/SKILL.md\"})\n]); results.forEach((r,i)=>text({i,result:r}));"
	collision := "run-receipt: create log: open /var/tmp/megapowers-receipts/20261013T153226Z-47.log: file exists"
	for _, tc := range []struct {
		name   string
		script string
		blocks func(t *testing.T) []string
		loaded bool
	}{
		{"settled read", script, func(t *testing.T) []string {
			return []string{settledBlock(t, 0, "fulfilled", 0, "ok\n"), settledBlock(t, 1, "fulfilled", 0, " 3 files changed\n"), settledBlock(t, 2, "fulfilled", 0, body)}
		}, true},
		{"settled read beside a failed sibling", script, func(t *testing.T) []string {
			return []string{settledBlock(t, 0, "fulfilled", 1, collision), settledBlock(t, 1, "fulfilled", 0, " 3 files changed\n"), settledBlock(t, 2, "fulfilled", 0, body)}
		}, true},
		{"failed read", script, func(t *testing.T) []string {
			return []string{settledBlock(t, 0, "fulfilled", 0, "ok\n"), settledBlock(t, 2, "fulfilled", 1, "cat: /p/skills/safe-effects/SKILL.md: No such file or directory\n")}
		}, false},
		{"rejected read", script, func(t *testing.T) []string {
			return []string{settledBlock(t, 0, "fulfilled", 0, "ok\n"), settledBlock(t, 2, "rejected", 0, body)}
		}, false},
		{"failed read with body text elsewhere in its block", script, func(t *testing.T) []string {
			return []string{settledBlock(t, 2, "fulfilled", 2, body)}
		}, false},
		{"MCP read marked isError", script, func(t *testing.T) []string {
			failed, _ := json.Marshal(map[string]any{"i": 2, "result": map[string]any{"status": "fulfilled", "value": map[string]any{"isError": true, "content": []any{map[string]any{"type": "text", "text": body}}}}})
			return []string{settledBlock(t, 0, "fulfilled", 0, "ok\n"), string(failed)}
		}, false},
		{"catalog listing only", "const r=await Promise.allSettled([tools.exec_command({cmd:\"ls /p/skills/safe-effects/SKILL.md\"}),tools.exec_command({cmd:\"cat catalog.md\"})]); r.forEach((r,i)=>text({i,result:r}));", func(t *testing.T) []string {
			return []string{settledBlock(t, 0, "fulfilled", 0, "/p/skills/safe-effects/SKILL.md\n"), settledBlock(t, 1, "fulfilled", 0, "- megapowers:safe-effects: Use when preparing a deploy.\n")}
		}, false},
	} {
		for _, spread := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/spread=%v", tc.name, spread), func(t *testing.T) {
				blocks := tc.blocks(t)
				if spread {
					for i := range blocks {
						blocks[i] = spreadSettled(t, blocks[i])
					}
				}
				lines := batchedExecTrace(t, tc.script, blocks...)
				loaded, readable := skillLoaded(writeTranscript(t, lines...), effectSkill)
				if loaded != tc.loaded || !readable {
					t.Fatalf("loaded=%v readable=%v, want loaded=%v", loaded, readable, tc.loaded)
				}
			})
		}
	}
}
