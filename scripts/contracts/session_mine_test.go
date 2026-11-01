package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sessionMineTool = "./plugins/megapowers/skills/writing-agent-instructions/scripts/session-mine.go"

type sessionMineExample struct {
	Harness   string `json:"harness"`
	Session   string `json:"session"`
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	Agent     string `json:"agent"`
}

type sessionMineClass struct {
	Name     string               `json:"name"`
	Count    int                  `json:"count"`
	Examples []sessionMineExample `json:"examples"`
}

type sessionMineCorrections struct {
	SessionsScanned int                `json:"sessions_scanned"`
	HumanTurns      int                `json:"human_turns"`
	ClassifiedTurns int                `json:"classified_turns"`
	SkippedLines    int                `json:"skipped_lines"`
	SkippedFiles    int                `json:"skipped_files"`
	Classes         []sessionMineClass `json:"classes"`
	OneOffs         []sessionMineClass `json:"one_offs"`
}

type sessionMineRecallItem struct {
	Harness        string `json:"harness"`
	Session        string `json:"session"`
	FirstTimestamp string `json:"first_timestamp"`
	LastTimestamp  string `json:"last_timestamp"`
	FirstPrompt    string `json:"first_prompt"`
	LastReply      string `json:"last_reply"`
}

type sessionMineRecall struct {
	SessionsScanned int                     `json:"sessions_scanned"`
	SkippedLines    int                     `json:"skipped_lines"`
	Sessions        []sessionMineRecallItem `json:"sessions"`
}

// Fixture builders. Every transcript line below is invented text.
func claudeUser(ts, content string) string {
	body, _ := json.Marshal(map[string]any{"type": "user", "timestamp": ts, "message": map[string]any{"role": "user", "content": content}})
	return string(body)
}

func claudeUserBlocks(ts string, extra map[string]any, blocks ...map[string]any) string {
	record := map[string]any{"type": "user", "timestamp": ts, "message": map[string]any{"role": "user", "content": blocks}}
	for key, value := range extra {
		record[key] = value
	}
	body, _ := json.Marshal(record)
	return string(body)
}

func claudeAssistant(ts string, blocks ...map[string]any) string {
	body, _ := json.Marshal(map[string]any{"type": "assistant", "timestamp": ts, "message": map[string]any{"role": "assistant", "content": blocks}})
	return string(body)
}

func textBlock(text string) map[string]any { return map[string]any{"type": "text", "text": text} }

func codexMeta(ts, cwd string, source any) string {
	body, _ := json.Marshal(map[string]any{"timestamp": ts, "type": "session_meta", "payload": map[string]any{"cwd": cwd, "source": source}})
	return string(body)
}

func codexMessage(ts, role string, texts ...string) string {
	kind := "input_text"
	if role == "assistant" {
		kind = "output_text"
	}
	var content []map[string]any
	for _, text := range texts {
		content = append(content, map[string]any{"type": kind, "text": text})
	}
	body, _ := json.Marshal(map[string]any{"timestamp": ts, "type": "response_item", "payload": map[string]any{"type": "message", "role": role, "content": content}})
	return string(body)
}

func writeLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func sessionMineFixture(t *testing.T) (claudeRoot, codexRoot string) {
	t.Helper()
	tmp := t.TempDir()
	claudeRoot = filepath.Join(tmp, "claude-projects")
	codexRoot = filepath.Join(tmp, "codex-sessions")
	// Slug: every "/" and "." of /work/demo.v2 becomes "-".
	slug := filepath.Join(claudeRoot, "-work-demo-v2")
	writeLines(t, filepath.Join(slug, "s1.jsonl"),
		claudeUser("2026-09-15T08:00:00Z", "You didn't verify the parser change."),
		claudeUser("2026-10-02T09:00:00Z", "Please add a retry flag to the sync command."),
		claudeAssistant("2026-10-02T09:01:00Z", map[string]any{"type": "thinking", "thinking": "hidden"}, textBlock("Should I also update the docs before I proceed?")),
		claudeUserBlocks("2026-10-02T09:02:00Z", nil, textBlock("Just do it, no need to ask.")),
		claudeAssistant("2026-10-02T09:03:00Z", map[string]any{"type": "tool_use", "id": "t1", "name": "Bash", "input": map[string]any{"command": "ls"}}),
		claudeUserBlocks("2026-10-02T09:04:00Z", nil, map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": "why did you stop toolresult"}),
		claudeUserBlocks("2026-10-02T09:05:00Z", map[string]any{"isMeta": true}, textBlock("why did you stop metarecord")),
		claudeUser("2026-10-02T09:06:00Z", "<command-name>/keep going</command-name>"),
		claudeUserBlocks("2026-10-02T09:07:00Z", nil, textBlock("Base directory for this skill: /skills/x\n\nkeep going skillbody")),
		claudeUser("2026-10-02T09:08:00Z", "<system-reminder>go ahead reminder</system-reminder>"),
		claudeUser("2026-10-02T09:08:30Z", "<local-command-stdout>keep going stdout</local-command-stdout>"),
		claudeAssistant("2026-10-02T09:09:00Z", textBlock("I paused to confirm the plan with you.")),
		claudeUser("2026-10-02T09:10:00Z", "Why did you stop? Keep going."),
		`{"type":"user","message":{not json`,
		claudeAssistant("2026-10-02T09:11:00Z", textBlock("Finished the sync work; questions to dev.person@example.com please.")),
		claudeUser("2026-10-02T09:12:00Z", "Did you test it on the empty input case?"),
	)
	writeLines(t, filepath.Join(slug, "s1", "subagents", "agent-one.jsonl"),
		claudeUserBlocks("2026-10-04T10:00:00Z", map[string]any{"isSidechain": true}, textBlock("keep going with the retry work")),
		claudeAssistant("2026-10-04T10:05:00Z", textBlock("The retry flag now defaults to three attempts.")),
	)
	writeLines(t, filepath.Join(claudeRoot, "-work-other", "o1.jsonl"),
		claudeUser("2026-10-02T09:00:00Z", "why did you stop otherworkspace"),
		claudeUser("2026-10-02T09:01:00Z", "why did you stop otherworkspace again"),
		claudeAssistant("2026-10-02T09:02:00Z", textBlock("retry flag otherworkspace")),
	)
	day := filepath.Join(codexRoot, "2026", "10", "03")
	writeLines(t, filepath.Join(day, "rollout-2026-10-03T10-00-00-aaa.jsonl"),
		codexMeta("2026-10-03T10:00:00Z", "/work/demo.v2", "cli"),
		codexMessage("2026-10-03T10:00:01Z", "user", "<environment_context>keep going envblock</environment_context>"),
		codexMessage("2026-10-03T10:00:02Z", "user", "# AGENTS.md instructions for /work/demo.v2\n\ngo ahead agentsblock"),
		codexMessage("2026-10-03T10:00:03Z", "user", "<user_instructions>continue userinstr</user_instructions>"),
		codexMessage("2026-10-03T10:00:04Z", "user", "<skill>\ngo ahead skillblock</skill>", "Summarize the cache layer."),
		codexMessage("2026-10-03T10:01:00Z", "assistant", "Here is a long summary of the cache layer."),
		codexMessage("2026-10-03T10:02:00Z", "user", "Too long, make it shorter."),
		codexMessage("2026-10-03T10:03:00Z", "assistant", "Trimmed."),
		codexMessage("2026-10-05T11:00:00Z", "user", "Still too long, shorter please."),
		codexMessage("2026-10-05T11:01:00Z", "assistant", "The retry flag is documented in one line."),
	)
	writeLines(t, filepath.Join(day, "rollout-2026-10-03T11-00-00-bbb.jsonl"),
		codexMeta("2026-10-03T11:00:00Z", "/work/other", "cli"),
		codexMessage("2026-10-03T11:00:01Z", "user", "too long otherworkspace"),
		codexMessage("2026-10-03T11:00:02Z", "user", "too long otherworkspace twice"),
		codexMessage("2026-10-06T11:00:03Z", "assistant", "retry flag otherworkspace"),
	)
	writeLines(t, filepath.Join(day, "rollout-2026-10-03T12-00-00-ccc.jsonl"),
		codexMeta("2026-10-03T12:00:00Z", "/work/demo.v2", map[string]any{"subagent": map[string]any{"depth": 1}}),
		codexMessage("2026-10-03T12:00:01Z", "user", "continue with the retry later"),
		codexMessage("2026-10-03T12:00:02Z", "assistant", "Will retry later."),
	)
	return claudeRoot, codexRoot
}

func findClass(classes []sessionMineClass, name string) *sessionMineClass {
	for i := range classes {
		if classes[i].Name == name {
			return &classes[i]
		}
	}
	return nil
}

func TestSessionMineCodexArchiveRoots(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	for _, name := range []string{"default home", "CODEX_HOME", "archive only"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			codexHome := filepath.Join(home, ".codex")
			env := []string{"HOME=" + home, "CODEX_HOME="}
			if name == "CODEX_HOME" {
				codexHome = filepath.Join(home, "custom-codex")
				env[1] = "CODEX_HOME=" + codexHome
			}
			workspace := filepath.Join(home, "workspace")
			active := filepath.Join(codexHome, "sessions")
			archive := filepath.Join(codexHome, "archived_sessions")
			shared := []string{
				codexMeta("2026-10-03T10:00:00Z", workspace, "cli"),
				codexMessage("2026-10-03T10:01:00Z", "user", "Did you test the parser?"),
			}
			if name != "archive only" {
				writeLines(t, filepath.Join(active, "2026", "10", "03", "rollout-shared.jsonl"), shared...)
			}
			writeLines(t, filepath.Join(archive, "rollout-shared.jsonl"), shared...)
			writeLines(t, filepath.Join(archive, "rollout-archived.jsonl"),
				codexMeta("2026-10-03T11:00:00Z", workspace, "cli"),
				codexMessage("2026-10-03T11:01:00Z", "user", "Did you test the archive?"),
			)
			for _, scan := range []struct {
				name, explicitRoot string
				want               int
			}{
				{name: "default roots", want: 2},
				{name: "explicit archive", explicitRoot: archive, want: 2},
				{name: "explicit sessions", explicitRoot: active, want: 1},
			} {
				t.Run(scan.name, func(t *testing.T) {
					want := scan.want
					if name == "archive only" && scan.explicitRoot == active {
						want = 0
					}
					args := []string{"corrections", "--workspace", workspace, "--harness", "codex", "--format", "json"}
					if scan.explicitRoot != "" {
						args = append(args, "--codex-root", scan.explicitRoot)
					}
					output, code := run(t, root, env, tool, args...)
					var report sessionMineCorrections
					if code != 0 || json.Unmarshal([]byte(output), &report) != nil {
						t.Fatalf("corrections failed (%d): %s", code, output)
					}
					if report.SessionsScanned != want || report.HumanTurns != want || report.ClassifiedTurns != want {
						t.Errorf("sessions=%d human=%d classified=%d, want %d each", report.SessionsScanned, report.HumanTurns, report.ClassifiedTurns, want)
					}
				})
			}
		})
	}
}

func TestSessionMineCorrections(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	claudeRoot, codexRoot := sessionMineFixture(t)
	base := []string{"corrections", "--workspace", "/work/demo.v2", "--claude-root", claudeRoot, "--codex-root", codexRoot}

	output, code := run(t, root, nil, tool, append(base, "--since", "2026-10-01", "--format", "json")...)
	if code != 0 {
		t.Fatalf("corrections json failed (%d): %s", code, output)
	}
	var report sessionMineCorrections
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("corrections json does not parse: %v\n%s", err, output)
	}
	if report.SessionsScanned != 4 || report.HumanTurns != 7 || report.ClassifiedTurns != 5 || report.SkippedLines != 1 {
		t.Errorf("summary = %+v, want sessions 4, human 7, classified 5, skipped 1", report)
	}
	stopped := findClass(report.Classes, "stopped-unnecessarily")
	if stopped == nil || stopped.Count != 2 || len(stopped.Examples) != 2 {
		t.Fatalf("stopped-unnecessarily class wrong: %+v", report.Classes)
	}
	if stopped.Examples[0].Harness != "claude" || stopped.Examples[0].Session != "s1.jsonl" || !strings.Contains(stopped.Examples[0].Agent, "update the docs") {
		t.Errorf("stopped example lacks harness, session, or preceding agent text: %+v", stopped.Examples[0])
	}
	if stopped.Examples[1].Agent != "I paused to confirm the plan with you." {
		t.Errorf("preceding agent excerpt wrong: %+v", stopped.Examples[1])
	}
	verbosity := findClass(report.Classes, "verbosity")
	if verbosity == nil || verbosity.Count != 2 || verbosity.Examples[0].Harness != "codex" || verbosity.Examples[0].Session != "rollout-2026-10-03T10-00-00-aaa.jsonl" {
		t.Errorf("codex verbosity class wrong: %+v", verbosity)
	}
	if findClass(report.Classes, "verification") != nil {
		t.Errorf("single verification turn listed as a class: %+v", report.Classes)
	}
	if one := findClass(report.OneOffs, "verification"); one == nil || one.Count != 1 {
		t.Errorf("verification one-off missing: %+v", report.OneOffs)
	}
	for _, leaked := range []string{"toolresult", "metarecord", "skillbody", "reminder", "stdout", "otherworkspace", "envblock", "agentsblock", "userinstr", "skillblock", "retry later", "retry work"} {
		if strings.Contains(output, leaked) {
			t.Errorf("excluded text %q reached corrections output", leaked)
		}
	}

	// Without --since the September verification turn makes the class repeat.
	output, code = run(t, root, nil, tool, append(base, "--format", "json")...)
	var all sessionMineCorrections
	if code != 0 || json.Unmarshal([]byte(output), &all) != nil {
		t.Fatalf("unfiltered run failed (%d): %s", code, output)
	}
	if verification := findClass(all.Classes, "verification"); verification == nil || verification.Count != 2 {
		t.Errorf("since filter not applied: %+v", all.Classes)
	}

	// Limit caps examples, not counts.
	output, _ = run(t, root, nil, tool, append(base, "--since", "2026-10-01", "--format", "json", "--limit", "1")...)
	var limited sessionMineCorrections
	json.Unmarshal([]byte(output), &limited)
	if stopped := findClass(limited.Classes, "stopped-unnecessarily"); stopped == nil || stopped.Count != 2 || len(stopped.Examples) != 1 {
		t.Errorf("limit 1 wrong: %+v", limited.Classes)
	}

	// Harness filter.
	output, _ = run(t, root, nil, tool, append(base, "--since", "2026-10-01", "--format", "json", "--harness", "codex")...)
	var codexOnly sessionMineCorrections
	json.Unmarshal([]byte(output), &codexOnly)
	if codexOnly.SessionsScanned != 2 || findClass(codexOnly.Classes, "stopped-unnecessarily") != nil {
		t.Errorf("harness codex did not exclude claude: %+v", codexOnly)
	}

	// Text output names classes, one-offs, and the summary line.
	output, code = run(t, root, nil, tool, append(base, "--since", "2026-10-01")...)
	if code != 0 {
		t.Fatalf("text output failed (%d): %s", code, output)
	}
	for _, want := range []string{"stopped-unnecessarily (2)", "verbosity (2)", "one-offs", "verification (1)", "sessions scanned 4, human turns 7, classified turns 5", "skipped lines 1"} {
		if !strings.Contains(output, want) {
			t.Errorf("text output missing %q:\n%s", want, output)
		}
	}
}

func TestSessionMineRedaction(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	claudeRoot := filepath.Join(t.TempDir(), "projects")
	// Token shapes are assembled at runtime so no secret-shaped literal ships.
	values := map[string]string{
		"openai":  "sk-" + strings.Repeat("Ab3", 8),
		"github":  "gh" + "p_" + strings.Repeat("Zx9", 12),
		"pat":     "github" + "_pat_" + strings.Repeat("Q7r", 9),
		"slack":   "xox" + "b-" + strings.Repeat("42", 6) + "-abcdefabcd",
		"aws":     "AK" + "IA" + strings.Repeat("Q", 16),
		"bearer":  "tok." + strings.Repeat("Bq", 6),
		"hex":     strings.Repeat("0a1b2c3d", 5),
		"b64":     strings.Repeat("Qm9vYmFy", 5),
		"email":   "pat.doe@example.com",
		"userinf": "hunter" + "secretpw",
		"letters": strings.Repeat("QwErTy", 6),
		"slashy":  strings.Repeat("A", 10) + "/" + strings.Repeat("B", 10) + "/" + strings.Repeat("C", 10),
	}
	prompts := []string{
		"Did you test " + values["openai"] + " here",
		"Did you test " + values["github"] + " here",
		"Did you test " + values["pat"] + " here",
		"Did you test " + values["slack"] + " here",
		"Did you test " + values["aws"] + " here",
		"Did you test Authorization: Bearer " + values["bearer"] + " here",
		"Did you test commit " + values["hex"] + " here",
		"Did you test blob " + values["b64"] + " here",
		"Did you test mail to " + values["email"] + " here",
		"Did you test https://deploy:" + values["userinf"] + "@git.example.com/repo here",
		"Did you test /home/alice/project and /Users/bob/src here",
		"Did you test the long one " + strings.Repeat("word ", 80),
		"Did you test token " + values["letters"] + " here",
		"Did you test plugins/megapowers/skills/writing-agent-instructions/scripts here",
		"Did you test /root/.aws/credentials here",
		`Did you test C:\Users\carol\Documents\notes here`,
		"Did you test /srv/people/zed/notes and /srv/people/zedd/x here",
		"Did you test /srv/profiles/quinn/x here",
		"Did you test key " + values["slashy"] + " here",
		`Did you test C:\Users\Jane Doe\app here`,
		"Did you test this <ctx><ctx>x</ctx>private client Acme</ctx> please",
	}
	var lines []string
	for i, prompt := range prompts {
		lines = append(lines, claudeAssistant("2026-10-02T09:00:00Z", textBlock("Agent reply "+string(rune('a'+i)))), claudeUser("2026-10-02T09:01:00Z", prompt))
	}
	writeLines(t, filepath.Join(claudeRoot, "-work-redact", "r1.jsonl"), lines...)
	homes := []string{"HOME=/srv/people/zed", "USERPROFILE=/srv/profiles/quinn"}
	output, code := run(t, root, homes, tool, "--workspace", "/work/redact", "--harness", "claude", "--claude-root", claudeRoot, "--limit", "50", "--format", "json")
	if code != 0 {
		t.Fatalf("redaction run failed (%d): %s", code, output)
	}
	for name, value := range values {
		if strings.Contains(output, value) {
			t.Errorf("%s value leaked: %s", name, value)
		}
	}
	var report sessionMineCorrections
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("json: %v", err)
	}
	verification := findClass(report.Classes, "verification")
	if verification == nil || verification.Count != len(prompts) {
		t.Fatalf("verification examples missing: %s", output)
	}
	joined := ""
	for _, example := range verification.Examples {
		if n := len([]rune(example.User)); n > 160 || strings.Contains(example.User, "\n") {
			t.Errorf("user excerpt not single line within 160 chars (%d): %q", n, example.User)
		}
		joined += example.User + "\n"
	}
	for _, want := range []string{"<redacted>", "<email>", "Bearer <redacted>", "https://git.example.com/repo", "~/project", "~/src",
		"plugins/megapowers/skills/writing-agent-instructions/scripts", "~/.aws/credentials", `~\Documents\notes`, "~/notes", "/srv/people/zedd/x", "~/x"} {
		if !strings.Contains(joined, want) {
			t.Errorf("redacted output missing %q:\n%s", want, joined)
		}
	}
	for _, leaked := range []string{"alice", "bob", "deploy:", "/root", "carol", "zed/", "quinn", "~d", "Jane", "Doe", "Acme", "</ctx>"} {
		if strings.Contains(joined, leaked) {
			t.Errorf("redaction kept %q", leaked)
		}
	}
}

func TestSessionMineRecall(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	claudeRoot, codexRoot := sessionMineFixture(t)
	base := []string{"recall", "--topic", "Retry FLAG", "--workspace", "/work/demo.v2", "--claude-root", claudeRoot, "--codex-root", codexRoot}
	output, code := run(t, root, nil, tool, append(base, "--format", "json")...)
	if code != 0 {
		t.Fatalf("recall failed (%d): %s", code, output)
	}
	var report sessionMineRecall
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("recall json does not parse: %v\n%s", err, output)
	}
	var order []string
	for _, item := range report.Sessions {
		order = append(order, item.Session)
	}
	want := []string{"rollout-2026-10-03T10-00-00-aaa.jsonl", "agent-one.jsonl", "s1.jsonl"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("recall order = %v, want %v", order, want)
	}
	s1 := report.Sessions[2]
	for _, format := range []string{"text", "json"} {
		output, code := run(t, root, nil, tool, "recall", "--topic", "Dev.Person@example.com", "--workspace", "/work/demo.v2", "--claude-root", claudeRoot, "--codex-root", codexRoot, "--format", format)
		if code != 0 || strings.Contains(strings.ToLower(output), "dev.person") || !strings.Contains(output, "<email>") {
			t.Errorf("%s recall topic not redacted (%d): %s", format, code, output)
		}
	}
	if s1.FirstPrompt != "You didn't verify the parser change." || !strings.Contains(s1.LastReply, "<email>") || strings.Contains(s1.LastReply, "dev.person") {
		t.Errorf("s1 recall excerpts wrong: %+v", s1)
	}
	if !strings.HasPrefix(s1.FirstTimestamp, "2026-09-15") || !strings.HasPrefix(s1.LastTimestamp, "2026-10-02") {
		t.Errorf("s1 timestamps wrong: %+v", s1)
	}
	if report.Sessions[0].FirstPrompt != "Summarize the cache layer." {
		t.Errorf("codex first prompt kept injected block: %+v", report.Sessions[0])
	}
	output, _ = run(t, root, nil, tool, append(base, "--limit", "1")...)
	if !strings.Contains(output, "rollout-2026-10-03T10-00-00-aaa.jsonl") || strings.Contains(output, "s1.jsonl") {
		t.Errorf("recall limit 1 wrong:\n%s", output)
	}
	output, _ = run(t, root, nil, tool, "recall", "--topic", "retry nonexistentword", "--workspace", "/work/demo.v2", "--claude-root", claudeRoot, "--codex-root", codexRoot, "--format", "json")
	var none sessionMineRecall
	if json.Unmarshal([]byte(output), &none) != nil || len(none.Sessions) != 0 {
		t.Errorf("all topic words not required: %s", output)
	}
	output, _ = run(t, root, nil, tool, "recall", "--topic", "retry flag", "--all-workspaces", "--claude-root", claudeRoot, "--codex-root", codexRoot, "--format", "json")
	var everywhere sessionMineRecall
	json.Unmarshal([]byte(output), &everywhere)
	if len(everywhere.Sessions) != 5 {
		t.Errorf("all-workspaces recall found %d sessions, want 5: %s", len(everywhere.Sessions), output)
	}
}

func claudeUserIn(cwd, ts, content string) string {
	body, _ := json.Marshal(map[string]any{"type": "user", "cwd": cwd, "timestamp": ts, "message": map[string]any{"role": "user", "content": content}})
	return string(body)
}

func TestSessionMineScoping(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	tmp := t.TempDir()
	mine := func(claudeRoot, workspace string) (sessionMineCorrections, string) {
		t.Helper()
		output, code := run(t, root, nil, tool, "--workspace", workspace, "--harness", "claude", "--claude-root", claudeRoot, "--format", "json")
		var report sessionMineCorrections
		if code != 0 || json.Unmarshal([]byte(output), &report) != nil {
			t.Fatalf("run failed (%d): %s", code, output)
		}
		return report, output
	}

	t.Run("workspaces sharing a slug", func(t *testing.T) {
		claudeRoot := filepath.Join(tmp, "shared")
		// /work/a-b and /work/a/b both map to the project directory -work-a-b.
		dir := filepath.Join(claudeRoot, "-work-a-b")
		writeLines(t, filepath.Join(dir, "c1.jsonl"),
			claudeUserIn("/work/a-b/sub", "2026-10-02T09:00:00Z", "Status please\n<system-reminder>don't ask</system-reminder>"),
			claudeUserIn("/work/a-b", "2026-10-02T09:01:00Z", "Why did you stop here?"),
			claudeUserIn("/work/a-b", "2026-10-02T09:02:00Z", "<system-reminder>keep going</system-reminder>\n<command-name>/x</command-name>"),
		)
		writeLines(t, filepath.Join(dir, "c2.jsonl"),
			claudeUserIn("/work/a/b", "2026-10-02T09:00:00Z", "too long collision"),
			claudeUserIn("/work/a/b", "2026-10-02T09:01:00Z", "too long collision twice"),
			claudeUser("2026-10-02T09:02:00Z", "too long nocwd"),
		)
		report, output := mine(claudeRoot, "/work/a-b")
		if report.HumanTurns != 2 || report.ClassifiedTurns != 1 || findClass(report.OneOffs, "stopped-unnecessarily") == nil {
			t.Errorf("a-b report wrong: %s", output)
		}
		for _, leaked := range []string{"collision", "nocwd", "don't ask", "keep going"} {
			if strings.Contains(output, leaked) {
				t.Errorf("a-b output kept %q: %s", leaked, output)
			}
		}
		// The reminder-carrying turn is human but classifies as other; recall
		// shows its excerpt as the first prompt.
		output, _ = run(t, root, nil, tool, "recall", "--topic", "status", "--workspace", "/work/a-b", "--harness", "claude", "--claude-root", claudeRoot, "--format", "json")
		var recalled sessionMineRecall
		if json.Unmarshal([]byte(output), &recalled) != nil || len(recalled.Sessions) != 1 || recalled.Sessions[0].FirstPrompt != "Status please" {
			t.Errorf("embedded reminder not stripped from excerpt: %s", output)
		}
		report, output = mine(claudeRoot, "/work/a/b")
		if verbosity := findClass(report.Classes, "verbosity"); verbosity == nil || verbosity.Count != 2 || strings.Contains(output, "nocwd") || strings.Contains(output, "Why did you stop") {
			t.Errorf("a/b report wrong: %s", output)
		}
	})

	t.Run("symlinked store", func(t *testing.T) {
		outside := filepath.Join(tmp, "outside")
		writeLines(t, filepath.Join(outside, "x.jsonl"), claudeUser("2026-10-02T09:00:00Z", "why did you stop outsidestore"))
		claudeRoot := filepath.Join(tmp, "escape")
		if err := os.MkdirAll(claudeRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(claudeRoot, "-work-esc")); err != nil {
			t.Fatal(err)
		}
		report, output := mine(claudeRoot, "/work/esc")
		if report.HumanTurns != 0 || report.SkippedFiles != 1 || strings.Contains(output, "outsidestore") {
			t.Errorf("symlink escape read or not counted: %s", output)
		}
		real := filepath.Join(tmp, "real-store")
		writeLines(t, filepath.Join(real, "-work-ok", "ok.jsonl"), claudeUser("2026-10-02T09:00:00Z", "keep going inside"))
		link := filepath.Join(tmp, "store-link")
		if err := os.Symlink(real, link); err != nil {
			t.Fatal(err)
		}
		if report, output := mine(link, "/work/ok"); report.HumanTurns != 1 || report.SkippedFiles != 0 {
			t.Errorf("symlinked root not resolved: %s", output)
		}
	})
}

func TestSessionMineUsage(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	tmp := t.TempDir()
	for _, args := range [][]string{
		{"corrections"},
		{"recall", "--topic", "x"},
		{"recall", "--workspace", "/work/demo.v2"},
		{"--workspace", "/work/demo.v2", "--harness", "other"},
		{"--workspace", "/work/demo.v2", "--since", "October"},
		{"--workspace", "/work/demo.v2", "--limit", "0"},
		{"summarize", "--workspace", "/work/demo.v2"},
	} {
		full := append([]string{}, args...)
		full = append(full, "--claude-root", tmp, "--codex-root", tmp)
		output, code := run(t, root, nil, tool, full...)
		if code != 2 || !strings.Contains(output, "usage:") {
			t.Errorf("%v: want exit 2 with usage, got %d: %s", args, code, output)
		}
	}
}

// buildSessionMine compiles the tool so tests observe its own exit codes;
// `go run` reports every non-zero exit as 1.
func buildSessionMine(t *testing.T, root string) string {
	t.Helper()
	tool := filepath.Join(t.TempDir(), "session-mine")
	if output, code := run(t, root, nil, "go", "build", "-o", tool, sessionMineTool); code != 0 {
		t.Fatalf("build session-mine (%d): %s", code, output)
	}
	return tool
}
