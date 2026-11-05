package maintain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Ported from the pre-v0.46 doctor and gate regressions.
// These fixtures exercise the shipped go-run entry point using isolated runtime
// homes. The Go tool itself keeps its normal environment and default caches.
func runDoctorFixtureChild() int {
	var values map[string]string
	if err := json.Unmarshal([]byte(os.Getenv("MEGAPOWERS_DOCTOR_FIXTURE_JSON")), &values); err != nil {
		return 2
	}
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Dir = values["TEST_DOCTOR_CWD"]
	for _, key := range []string{"PATH", "TMPDIR", "SYSTEMROOT"} {
		if value := os.Getenv(key); value != "" {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	for key, value := range values {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return e.ExitCode()
		}
		return 2
	}
	return 0
}
func doctorPluginRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../plugins/megapowers")
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func doctorOutput(t *testing.T, values map[string]string) string { return doctorArgs(t, values) }
func doctorArgs(t *testing.T, values map[string]string, args ...string) string {
	t.Helper()
	script := filepath.Join(doctorPluginRoot(t), "skills/megapowers-doctor/scripts/doctor.go")
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", append([]string{"run", "-exec", launcher, script}, args...)...)
	cmd.Env = append(os.Environ(), "MEGAPOWERS_DOCTOR_FIXTURE_JSON="+string(data))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	return string(out)
}
func doctorWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func strconvQuote(value string) string { b, _ := json.Marshal(value); return string(b) }

var claudeProjectSlug = regexp.MustCompile("[^A-Za-z0-9]")

// A real count, rather than the shared label on the no-sessions diagnostic.
func doctorLoads(t *testing.T, lines ...string) bool {
	t.Helper()
	home := t.TempDir()
	workspace := t.TempDir()
	project := claudeProjectSlug.ReplaceAllString(workspace, "-")
	doctorWrite(t, filepath.Join(home, ".claude/projects", project, "s.jsonl"), strings.Join(lines, "\n")+"\n")
	report := doctorOutput(t, map[string]string{"HOME": home, "PWD": workspace, "MEGAPOWERS_HARNESS": "codex", "MEGAPOWERS_PLUGIN_ROOT": doctorPluginRoot(t)})
	prefix := "skill loads, last 7 days in " + workspace + ": 1 transcript(s); "
	if !strings.Contains(report, prefix) {
		t.Fatalf("missing transcript count: %s", report)
	}
	return strings.Contains(report, prefix+"safe-effects 1")
}
func writeClaudeSettings(t *testing.T, home, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorCountsSkillLoadsInTheWorkspace(t *testing.T) {
	home := t.TempDir()
	writeClaudeSettings(t, home, `{"outputStyle":"megapowers:Megapowers"}`)
	doctorWrite(t, filepath.Join(home, ".claude", "projects", "-work-app", "s1.jsonl"),
		strings.Join(skillReadTrace(t, "Skill", `{"skill":"megapowers:verify-and-finish"}`, "Launching skill: megapowers:verify-and-finish", false), "\n")+"\n")
	doctorWrite(t, filepath.Join(home, ".claude", "projects", "-work-other", "s2.jsonl"),
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"megapowers:grill-me"}}]}}`+"\n")
	doctorWrite(t, filepath.Join(home, ".codex", "sessions", filepath.FromSlash(time.Now().Format("2006/01/02")), "rollout-a.jsonl"),
		`{"type":"session_meta","payload":{"cwd":"/work/app"}}`+"\n"+
			`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<skill>\n<name>megapowers:safe-effects</name>\n<path>/p/skills/safe-effects/SKILL.md</path>\n# Safe Effects\nCheck the target."}]}}`+"\n")
	doctorWrite(t, filepath.Join(home, ".codex", "sessions", filepath.FromSlash(time.Now().Format("2006/01/02")), "rollout-b.jsonl"),
		`{"type":"session_meta","payload":{"cwd":"/work/other"}}`+"\n"+
			`{"type":"response_item","payload":{"type":"function_call","arguments":"cat /p/skills/grill-me/SKILL.md"}}`+"\n")
	env := map[string]string{
		"HOME":                   home,
		"PWD":                    "/work/app",
		"MEGAPOWERS_PLUGIN_ROOT": doctorPluginRoot(t),
	}
	report := doctorOutput(t, env)
	want := "skill loads, last 7 days in /work/app: 2 transcript(s); safe-effects 1, verify-and-finish 1"
	if !strings.Contains(report, want) {
		t.Errorf("doctor report lacks %q:\n%s", want, report)
	}
	if strings.Contains(report, "grill-me") {
		t.Errorf("loads from another workspace leaked into the count:\n%s", report)
	}
}

func TestDoctorCountsSkillLoadsFromGitRootAndCwd(t *testing.T) {
	for _, name := range []string{"repo root", "repo subdirectory", "outside git"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("GIT_CEILING_DIRECTORIES", home)
			workspace := filepath.Join(home, "workspace")
			cwd := workspace
			if name == "repo subdirectory" {
				cwd = filepath.Join(workspace, "subdir")
			}
			if err := os.MkdirAll(cwd, 0o700); err != nil {
				t.Fatal(err)
			}
			if name != "outside git" {
				if output, err := exec.Command("git", "-C", workspace, "init", "-q").CombinedOutput(); err != nil {
					t.Fatalf("git init: %v\n%s", err, output)
				}
			}
			workspaces := []string{workspace}
			if cwd != workspace {
				workspaces = append(workspaces, cwd)
			}
			claudeLoad := strings.Join(skillReadTrace(t, "Skill", `{"skill":"megapowers:verify-and-finish"}`, "Launching skill: megapowers:verify-and-finish", false), "\n") + "\n"
			codexLoad := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<skill>\n<name>megapowers:safe-effects</name>\n<path>/p/skills/safe-effects/SKILL.md</path>\n# Safe Effects\nCheck the target."}]}}` + "\n"
			for i, dir := range append(workspaces, filepath.Join(home, "other")) {
				project := claudeProjectSlug.ReplaceAllString(dir, "-")
				doctorWrite(t, filepath.Join(home, ".claude", "projects", project, "s1.jsonl"), claudeLoad)
				doctorWrite(t, filepath.Join(home, ".codex", "sessions", filepath.FromSlash(time.Now().Format("2006/01/02")), fmt.Sprintf("rollout-%d.jsonl", i)),
					`{"type":"session_meta","payload":{"cwd":`+strconvQuote(dir)+`}}`+"\n"+codexLoad)
			}
			env := map[string]string{
				"HOME":                   home,
				"PWD":                    cwd,
				"MEGAPOWERS_PLUGIN_ROOT": doctorPluginRoot(t),
			}
			report := doctorOutput(t, env)
			want := fmt.Sprintf("skill loads, last 7 days in %s: %d transcript(s); safe-effects %d, verify-and-finish %d", workspace, 2*len(workspaces), len(workspaces), len(workspaces))
			if !strings.Contains(report, want) {
				t.Errorf("doctor report lacks %q:\n%s", want, report)
			}
		})
	}
}

func TestDoctorMatchesCodexWorkspaceByValue(t *testing.T) {
	home := t.TempDir()
	writeClaudeSettings(t, home, `{"outputStyle":"megapowers:Megapowers"}`)
	day := time.Now().Format("2006/01/02")
	sessions := filepath.Join(home, ".codex", "sessions", filepath.FromSlash(day))
	load := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<skill>\n<name>megapowers:safe-effects</name>\n<path>/p/skills/safe-effects/SKILL.md</path>\n# Safe Effects\nCheck the target."}]}}` + "\n"
	doctorWrite(t, filepath.Join(sessions, "rollout-a.jsonl"), `{"type": "session_meta", "payload": {"cwd": "/work/R\u0026D"}}`+"\n"+load)
	doctorWrite(t, filepath.Join(sessions, "rollout-big.jsonl"), `{"type":"session_meta","payload":{"cwd":"/work/R&D","x":"`+strings.Repeat("a", 9<<20)+`"}}`+"\n"+load)
	env := map[string]string{
		"HOME":                   home,
		"PWD":                    "/work/R&D",
		"MEGAPOWERS_PLUGIN_ROOT": doctorPluginRoot(t),
	}
	report := doctorOutput(t, env)
	want := "skill loads, last 7 days in /work/R&D: 1 transcript(s); safe-effects 1"
	if !strings.Contains(report, want) {
		t.Errorf("doctor report lacks %q:\n%s", want, report)
	}
}

func TestDoctorUsesUserprofileWhenHomeIsUnset(t *testing.T) {
	// Windows sets USERPROFILE and leaves HOME unset.
	profile := t.TempDir()
	writeClaudeSettings(t, profile, `{"outputStyle":"megapowers:Megapowers"}`)
	report := doctorOutput(t, map[string]string{"USERPROFILE": profile, "MEGAPOWERS_HARNESS": "claude", "MEGAPOWERS_PLUGIN_ROOT": doctorPluginRoot(t)})
	if strings.Contains(report, "cannot locate settings.json") || !strings.Contains(report, "outputStyle: megapowers:Megapowers") {
		t.Fatalf("doctor must read settings under USERPROFILE:\n%s", report)
	}
	if !strings.Contains(report, "agent registry: none") {
		t.Fatalf("doctor must look for the agent registry under USERPROFILE:\n%s", report)
	}
}

func TestDoctorDetectsHarnessFromEnvironment(t *testing.T) {
	root := doctorPluginRoot(t)
	home := t.TempDir()
	codex := doctorOutput(t, map[string]string{"HOME": home, "PLUGIN_ROOT": root, "CLAUDE_PLUGIN_ROOT": root, "MEGAPOWERS_PLUGIN_ROOT": root})
	if !strings.Contains(codex, "harness: codex") {
		t.Fatalf("PLUGIN_ROOT should imply codex:\n%s", codex)
	}
	claude := doctorOutput(t, map[string]string{"HOME": home, "CLAUDECODE": "1", "MEGAPOWERS_PLUGIN_ROOT": root})
	if !strings.Contains(claude, "harness: claude") {
		t.Fatalf("CLAUDECODE should imply claude:\n%s", claude)
	}
	unknown := doctorOutput(t, map[string]string{"HOME": home, "MEGAPOWERS_PLUGIN_ROOT": root})
	if !strings.Contains(unknown, "harness: unknown") || !strings.Contains(unknown, "claude outputStyle") {
		t.Fatalf("unknown harness must report Claude style check:\n%s", unknown)
	}
}

func TestDoctorAcceptsTheFilledRegistryTemplate(t *testing.T) {
	template, err := os.ReadFile(filepath.Join(doctorPluginRoot(t), "skills", "orchestrating", "assets", "agent-capabilities.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(template, []byte("expires_at: <YYYY-MM-DD>")) {
		t.Fatalf("template no longer carries the expires_at placeholder this test fills")
	}
	filled := bytes.ReplaceAll(template, []byte("<YYYY-MM-DD>"), []byte("2099-10-25"))
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "megapowers")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent-capabilities.md"), filled, 0o600); err != nil {
		t.Fatal(err)
	}
	report := doctorOutput(t, map[string]string{"HOME": home, "MEGAPOWERS_HARNESS": "codex", "MEGAPOWERS_PLUGIN_ROOT": doctorPluginRoot(t)})
	if !strings.Contains(report, "expires 2099-10-25") || strings.Contains(report, "WARN: agent registry") {
		t.Fatalf("a filled template with its inline comment must date correctly:\n%s", report)
	}
}

func TestDoctorWarnsOnWrongOutputStyleValue(t *testing.T) {
	home := t.TempDir()
	writeClaudeSettings(t, home, `{"outputStyle":"Megapowers"}`)
	env := map[string]string{"HOME": home, "MEGAPOWERS_HARNESS": "claude", "MEGAPOWERS_PLUGIN_ROOT": doctorPluginRoot(t)}
	report := doctorOutput(t, env)
	if !strings.Contains(report, "WARN") || !strings.Contains(report, `outputStyle: Megapowers`) || !strings.Contains(report, "megapowers:Megapowers") {
		t.Fatalf("doctor must report the actual value and the expected one:\n%s", report)
	}
}

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

func TestDoctorMatchesCodexReadResults(t *testing.T) {
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
			loaded, readable := doctorLoads(t, lines...), true
			if loaded != tc.loaded || !readable {
				t.Fatalf("loaded=%v readable=%v, want loaded=%v", loaded, readable, tc.loaded)
			}
		})
	}
}

func TestDoctorMatchesBatchedExecSkillReads(t *testing.T) {
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
				loaded, readable := doctorLoads(t, lines...), true
				if loaded != tc.loaded || !readable {
					t.Fatalf("loaded=%v readable=%v, want loaded=%v", loaded, readable, tc.loaded)
				}
			})
		}
	}
}

func TestDoctorRequiresSuccessfulSkillRead(t *testing.T) {
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
			if got := doctorLoads(t, lines...); got != tc.loaded {
				t.Fatalf("loaded=%v want %v", got, tc.loaded)
			}
		})
	}
}

func TestDoctorWithoutRootSkipsRelativeFiles(t *testing.T) {
	workspace := t.TempDir()
	doctorWrite(t, filepath.Join(workspace, ".claude-plugin/plugin.json"), `{"version":"99.99.99"}`)
	doctorWrite(t, filepath.Join(workspace, "skills/catalog.json"), `{"skills":[{"name":"safe-effects"}]}`)
	report := doctorOutput(t, map[string]string{"HOME": t.TempDir(), "PWD": workspace, "TEST_DOCTOR_CWD": workspace, "MEGAPOWERS_HARNESS": "codex"})
	if !strings.Contains(report, "plugin root unknown") {
		t.Fatalf("missing root diagnostic: %s", report)
	}
	for _, unexpected := range []string{"plugin version:", "skill loads,", "marketplace copy"} {
		if strings.Contains(report, unexpected) {
			t.Errorf("relative check ran without root: %s", report)
		}
	}
}

func TestDoctorExplicitHarness(t *testing.T) {
	for _, harness := range []string{"claude", "codex"} {
		t.Run(harness, func(t *testing.T) {
			home := t.TempDir()
			report := doctorArgs(t, map[string]string{"HOME": home, "MEGAPOWERS_HARNESS": "claude", "CLAUDECODE": "1"}, "--root", doctorPluginRoot(t), "--harness", harness)
			if !strings.Contains(report, "harness: "+harness) {
				t.Errorf("wrong harness: %s", report)
			}
			if got := strings.Contains(report, "claude outputStyle:"); got != (harness == "claude") {
				t.Errorf("style diagnostic for %s: %s", harness, report)
			}
		})
	}
}

func TestDoctorInjectedSkillBodies(t *testing.T) {
	for _, tc := range []struct {
		name, line string
		loaded     bool
	}{
		{"Claude slash body", `{"type":"user","message":{"content":"Base directory for this skill: /p/skills/safe-effects\n\n# Safe Effects"}}`, true},
		{"Codex injected body", `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<skill>\n<name>megapowers:safe-effects</name>\n<path>/p/skills/safe-effects/SKILL.md</path>\n# Safe Effects"}]}}`, true},
		{"assistant mention", `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"<skill>\n<name>megapowers:safe-effects</name>\n<path>/p/skills/safe-effects/SKILL.md</path>\n# Safe Effects"}]}}`, false},
		{"catalog", `{"type":"session_meta","payload":{"instructions":"safe-effects: Use for effects (file: /p/skills/safe-effects/SKILL.md)"}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := doctorLoads(t, tc.line); got != tc.loaded {
				t.Fatalf("loaded=%v want %v", got, tc.loaded)
			}
		})
	}
}
