package contracts

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

type machineReport struct {
	HumanTurns      int                `json:"human_turns"`
	MachineTurns    int                `json:"machine_turns"`
	ClassifiedTurns int                `json:"classified_turns"`
	Classes         []sessionMineClass `json:"classes"`
	OneOffs         []sessionMineClass `json:"one_offs"`
}

// Machine-written user turns are not the human's corrections. Every
// transcript line below is invented text.
func TestSessionMineMachineTurns(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	tmp := t.TempDir()
	claudeRoot := filepath.Join(tmp, "claude-projects")
	codexRoot := filepath.Join(tmp, "codex-sessions")
	workspace := "/work/machine"
	slug := filepath.Join(claudeRoot, slugFor(workspace))
	writeLines(t, filepath.Join(slug, "m1.jsonl"),
		claudeUser("2026-10-20T09:00:00Z", "Why did you stop? Keep going."),
		// Compaction summary, marked structurally.
		claudeUserBlocks("2026-10-20T09:01:00Z", map[string]any{"isCompactSummary": true, "isVisibleInTranscriptOnly": true},
			textBlock("Recap of the earlier work. Next step: keep going with the parser. flagcompact")),
		// Compaction summary without the flag: anchored text fallback.
		claudeUser("2026-10-20T09:02:00Z", "This session is being continued from a previous conversation that ran out of context. keep going textcompact"),
		// Task notification, marked by origin and by the system prompt source.
		claudeUserBlocks("2026-10-20T09:03:00Z", map[string]any{"promptSource": "system", "origin": map[string]any{"kind": "task-notification"}},
			textBlock("<task-notification>job done, keep going flagnotice</task-notification>")),
		// Task notification with no flags: the tag still marks it.
		claudeUser("2026-10-20T09:04:00Z", "<task-notification>job done, continue tagnotice</task-notification>"),
		// Autonomous-run prompts.
		claudeUserBlocks("2026-10-20T09:05:00Z", map[string]any{"promptSource": "sdk"},
			textBlock("This is an approved autonomous run, id `demo`. Objective: x. Complete C1 only, then continue later. autorun")),
		claudeUserBlocks("2026-10-20T09:06:00Z", map[string]any{"promptSource": "sdk"}, textBlock("Continue the approved run and finish it.")),
		// A schedule prompt carries an unattended-run paragraph.
		claudeUserBlocks("2026-10-20T09:07:00Z", map[string]any{"promptSource": "sdk"},
			textBlock("Daily check for the demo project.\n\nUnattended run: the first end of turn is the result. Do not ask; continue until done. schedule")),
		// Human turns that only resemble the anchors.
		claudeUser("2026-10-20T09:08:00Z", "Please continue with the docs."),
		claudeUser("2026-10-20T09:09:00Z", "I want an unattended run: continue tomorrow."),
		claudeUser("2026-10-20T09:10:00Z", "Can you explain how this session is being continued from the earlier one? Go ahead."),
	)
	writeLines(t, filepath.Join(codexRoot, "2026", "10", "20", "rollout-2026-10-20T10-00-00-exec.jsonl"),
		codexMeta("2026-10-20T10:00:00Z", workspace, "exec"),
		codexMessage("2026-10-20T10:00:01Z", "user", "keep going with the scripted job execmark"),
	)
	writeLines(t, filepath.Join(codexRoot, "2026", "10", "20", "rollout-2026-10-20T11-00-00-cli.jsonl"),
		codexMeta("2026-10-20T11:00:00Z", workspace, "cli"),
		codexMessage("2026-10-20T11:00:01Z", "user", "This is an approved autonomous run, id `demo`. Continue C2. codexauto"),
		codexMessage("2026-10-20T11:00:02Z", "user", "go ahead and run the tests"),
	)
	base := []string{"--workspace", workspace, "--claude-root", claudeRoot, "--codex-root", codexRoot, "--format", "json"}

	output, code := run(t, root, nil, tool, append([]string{"corrections"}, base...)...)
	var report machineReport
	if code != 0 || json.Unmarshal([]byte(output), &report) != nil {
		t.Fatalf("corrections failed (%d): %s", code, output)
	}
	// Human: stop complaint, "Please continue", the mid-line unattended turn,
	// "Can you explain... Go ahead", and the Codex "go ahead" turn.
	if report.HumanTurns != 5 || report.ClassifiedTurns != 5 {
		t.Errorf("human turns %d, classified %d; want 5 and 5", report.HumanTurns, report.ClassifiedTurns)
	}
	// Machine: two compaction summaries, two task notifications, two run
	// prompts, one schedule prompt, one Codex exec turn, one Codex run prompt.
	if report.MachineTurns != 9 {
		t.Errorf("machine turns = %d, want 9", report.MachineTurns)
	}
	stopped := findClass(report.Classes, "stopped-unnecessarily")
	if stopped == nil || stopped.Count != 5 {
		t.Errorf("stopped-unnecessarily = %+v, want 5 human turns", report.Classes)
	}
	for _, leaked := range []string{"flagcompact", "textcompact", "flagnotice", "tagnotice", "autorun", "schedule", "execmark", "codexauto", "approved run"} {
		if strings.Contains(output, leaked) {
			t.Errorf("machine text %q reached corrections output", leaked)
		}
	}

	output, code = run(t, root, nil, tool, append([]string{"scorecard"}, base...)...)
	var card struct {
		HumanTurns   int            `json:"human_turns"`
		MachineTurns int            `json:"machine_turns"`
		Corrections  map[string]int `json:"corrections"`
	}
	if code != 0 || json.Unmarshal([]byte(output), &card) != nil {
		t.Fatalf("scorecard failed (%d): %s", code, output)
	}
	if card.HumanTurns != 5 || card.MachineTurns != 9 || card.Corrections["stopped-unnecessarily"] != 5 {
		t.Errorf("scorecard = %+v, want human 5, machine 9, stopped 5", card)
	}

	// A session that only a script drove still shows its first prompt in recall.
	output, code = run(t, root, nil, tool, "recall", "--topic", "scripted job", "--workspace", workspace, "--claude-root", claudeRoot, "--codex-root", codexRoot, "--format", "json")
	var recalled sessionMineRecall
	if code != 0 || json.Unmarshal([]byte(output), &recalled) != nil || len(recalled.Sessions) != 1 || !strings.Contains(recalled.Sessions[0].FirstPrompt, "scripted job") {
		t.Errorf("recall of a scripted session lost its first prompt (%d): %s", code, output)
	}

	output, code = run(t, root, nil, tool, "corrections", "--workspace", workspace, "--claude-root", claudeRoot, "--codex-root", codexRoot)
	if code != 0 || !strings.Contains(output, "machine turns 9") {
		t.Errorf("text summary lacks machine turns (%d): %s", code, output)
	}
}

// A sleep counts only when it runs in the foreground, outside any loop, and
// lasts more than 5 seconds: the rule of the deny-fg-sleep hook.
func TestSessionMineForegroundSleeps(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	bash := func(command string, background bool) []string {
		input := map[string]any{"command": command}
		if background {
			input["run_in_background"] = true
		}
		return []string{claudeAssistant("2026-10-12T09:00:00Z", toolUse("t1", "Bash", input))}
	}
	codex := func(kind, field, text string) []string {
		return []string{codexCall("2026-10-12T09:00:00Z", kind, field, text)}
	}
	cases := []struct {
		name  string
		lines []string
		codex bool
		want  int
	}{
		{"plain sleep 30", bash("sleep 30", false), false, 1},
		{"sleep 30 then check", bash("sleep 30 && go test ./...", false), false, 1},
		{"sleep 20 between commands", bash("make build; sleep 20; make test", false), false, 1},
		{"sleep 6", bash("sleep 6", false), false, 1},
		{"sleep 2m", bash("sleep 2m", false), false, 1},
		{"sleep 4 between commands", bash("make build; sleep 4; make test", false), false, 0},
		{"sleep 5", bash("sleep 5", false), false, 0},
		{"background call", bash("sleep 30", true), false, 0},
		{"background operator", bash("sleep 30 & echo started", false), false, 0},
		{"until loop under timeout", bash("timeout 590 bash -c 'until grep -q PASS run.log; do sleep 30; done'", false), false, 0},
		{"until loop in the shell", bash("until test -f ready; do sleep 10; done", false), false, 0},
		{"while loop", bash("while true; do sleep 20; done", false), false, 0},
		{"for loop", bash("for i in 1 2 3; do sleep 10; done", false), false, 0},
		{"grep for the word", bash("grep sleep notes.txt", false), false, 0},
		{"grep for the command text", bash("grep -rn 'sleep 10' .", false), false, 0},
		{"echo of the command text", bash(`echo "sleep 30"`, false), false, 0},
		{"two foreground sleeps count once per call", bash("sleep 10; echo a; sleep 20", false), false, 1},
		{"codex json cmd", codex("function_call", "arguments", `{"cmd":"make build && sleep 30"}`), true, 1},
		{"codex json cmd in a loop", codex("function_call", "arguments", `{"cmd":"until test -f ready; do sleep 10; done"}`), true, 0},
		{"codex json command array", codex("function_call", "arguments", `{"command":["bash","-lc","sleep 30"]}`), true, 1},
		{"codex script cmd", codex("custom_tool_call", "input", `text(await tools.exec_command({cmd:"sleep 30 && ls", yield_time_ms:1000}))`), true, 1},
		{"codex script cmd in a loop", codex("custom_tool_call", "input", `text(await tools.exec_command({cmd:"for i in 1 2; do sleep 30; done"}))`), true, 0},
		{"codex message mentioning sleep", codex("function_call", "arguments", `{"message":"Do not run sleep 30 then check."}`), true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			workspace := filepath.Join(tmp, "ws")
			claudeRoot := filepath.Join(tmp, "claude-projects")
			codexRoot := filepath.Join(tmp, "codex-sessions")
			if tc.codex {
				lines := append([]string{
					codexMeta("2026-10-12T08:59:00Z", workspace, "cli"),
					codexMessage("2026-10-12T08:59:30Z", "user", "Run the job."),
				}, tc.lines...)
				writeLines(t, filepath.Join(codexRoot, "2026", "10", "12", "rollout-sleep.jsonl"), lines...)
			} else {
				lines := append([]string{claudeUser("2026-10-12T08:59:00Z", "Run the job.")}, tc.lines...)
				writeLines(t, filepath.Join(claudeRoot, slugFor(workspace), "s.jsonl"), withCwd(workspace, lines...)...)
			}
			output, code := run(t, root, nil, tool, "scorecard", "--workspace", workspace, "--claude-root", claudeRoot, "--codex-root", codexRoot, "--format", "json")
			var card scorecard
			if code != 0 || json.Unmarshal([]byte(output), &card) != nil {
				t.Fatalf("scorecard failed (%d): %s", code, output)
			}
			if card.SleepCalls != tc.want {
				t.Errorf("sleep calls = %d, want %d", card.SleepCalls, tc.want)
			}
		})
	}
}
