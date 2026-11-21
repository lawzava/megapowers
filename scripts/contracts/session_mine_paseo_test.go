package contracts

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

type sessionMineProvenance struct {
	Records    int    `json:"records"`
	Linked     int    `json:"linked"`
	Unreadable int    `json:"unreadable"`
	Note       string `json:"note"`
}

type sessionMinePromptReport struct {
	HumanTurns         int                    `json:"human_turns"`
	MachineTurns       int                    `json:"machine_turns"`
	MachinePromptTurns *int                   `json:"machine_prompt_turns"`
	HumanInMachine     *int                   `json:"human_turns_in_machine_created_sessions"`
	Paseo              *sessionMineProvenance `json:"paseo_provenance"`
	ClassifiedTurns    int                    `json:"classified_turns"`
	Corrections        map[string]int         `json:"corrections"`
	BySource           map[string]struct {
		HumanTurns     int `json:"human_turns"`
		PromptTurns    int `json:"prompt_turns"`
		HumanInMachine int `json:"human_turns_in_machine_created_sessions"`
	} `json:"by_source"`
	ByWorkspace []struct {
		HumanTurns         int `json:"human_turns"`
		MachinePromptTurns int `json:"machine_prompt_turns"`
	} `json:"by_workspace"`
	Sessions []struct {
		FirstPrompt string `json:"first_prompt"`
	} `json:"sessions"`
}

// All identifiers, transcript lines, and metadata below are synthetic.
func TestSessionMinePaseoPrompts(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	for _, harness := range []string{"claude", "codex"} {
		for _, tc := range []struct {
			name, provider, target, injection  string
			labels                             map[string]string
			noRecord, missingHome, malformed   bool
			badSchedule, beforeSince, anchored bool
			wantLinked, wantUnreadable         int
		}{
			{name: "schedule label", labels: map[string]string{"paseo.schedule-id": "schedule-test"}, wantLinked: 1},
			{name: "parent label", labels: map[string]string{"paseo.parent-agent-id": "parent-test"}, wantLinked: 1},
			{name: "run link only", target: "new-agent", wantLinked: 1},
			{name: "self target", target: "self"},
			{name: "no Paseo record", noRecord: true},
			{name: "missing Paseo home", noRecord: true, missingHome: true},
			{name: "malformed agent", malformed: true, wantUnreadable: 1},
			{name: "malformed schedule", badSchedule: true, wantUnreadable: 1},
			{name: "wrong provider", provider: "opencode", labels: map[string]string{"paseo.schedule-id": "schedule-test"}},
			{name: "injected meta", injection: "meta", labels: map[string]string{"paseo.schedule-id": "schedule-test"}, wantLinked: 1},
			{name: "injected structural meta", injection: "structural", labels: map[string]string{"paseo.schedule-id": "schedule-test"}, wantLinked: 1},
			{name: "injected tool result", injection: "tool", labels: map[string]string{"paseo.schedule-id": "schedule-test"}, wantLinked: 1},
			{name: "injected instructions", injection: "agents", labels: map[string]string{"paseo.schedule-id": "schedule-test"}, wantLinked: 1},
			{name: "injected environment", injection: "environment", labels: map[string]string{"paseo.schedule-id": "schedule-test"}, wantLinked: 1},
			{name: "first prompt before since", beforeSince: true, labels: map[string]string{"paseo.schedule-id": "schedule-test"}, wantLinked: 1},
			{name: "already machine prompt", anchored: true, labels: map[string]string{"paseo.schedule-id": "schedule-test"}, wantLinked: 1},
		} {
			t.Run(harness+"/"+tc.name, func(t *testing.T) {
				home := t.TempDir()
				paseo := filepath.Join(home, ".paseo")
				store := filepath.Join(home, "transcripts")
				writeJSON := func(name string, value any) {
					t.Helper()
					data, err := json.Marshal(value)
					if err != nil {
						t.Fatal(err)
					}
					writeLines(t, name, string(data))
				}
				if !tc.missingHome {
					// An empty agents directory is distinct from an absent home.
					writeLines(t, filepath.Join(paseo, "agents", "synthetic", "ignored.txt"), "")
				}
				if !tc.noRecord {
					provider := tc.provider
					if provider == "" {
						provider = harness
					}
					name := filepath.Join(paseo, "agents", "synthetic", "agent-test.json")
					writeJSON(name, map[string]any{
						"id": "agent-test", "provider": provider, "labels": tc.labels,
						"persistence": map[string]string{"sessionId": "session-test"},
					})
					if tc.malformed {
						writeLines(t, name, "{")
					}
				}
				if tc.target != "" || tc.badSchedule {
					name := filepath.Join(paseo, "schedules", "schedule-test.json")
					writeJSON(name, map[string]any{
						"target": map[string]string{"type": tc.target},
						"runs":   []map[string]string{{"agentId": "agent-test"}},
					})
					if tc.badSchedule {
						writeLines(t, name, "{")
					}
				}
				firstTime := "2026-10-02T09:00:00Z"
				if tc.beforeSince {
					firstTime = "2026-09-30T09:00:00Z"
				}
				firstText := "Continue the synthetic check."
				if tc.anchored {
					firstText = "Unattended run: continue the synthetic check."
				}
				// Claude session IDs live on records, not necessarily filenames.
				claudeLine := func(line string) string {
					var record map[string]any
					if err := json.Unmarshal([]byte(line), &record); err != nil {
						t.Fatal(err)
					}
					record["sessionId"] = "session-test"
					data, _ := json.Marshal(record)
					return string(data)
				}
				var lines []string
				name := filepath.Join(store, "project", "different-name.jsonl")
				if harness == "claude" {
					switch tc.injection {
					case "meta":
						lines = append(lines, claudeLine(claudeUserBlocks(firstTime, map[string]any{"isMeta": true}, textBlock("Synthetic setup."))))
					case "structural":
						// Kept as a machine turn, and still not the initial task.
						lines = append(lines, claudeLine(claudeUserBlocks(firstTime, map[string]any{"isMeta": true, "origin": map[string]string{"kind": "task"}}, textBlock("Synthetic setup."))))
					case "tool":
						lines = append(lines, claudeLine(claudeUserBlocks(firstTime, nil, map[string]any{"type": "tool_result", "content": "Synthetic output."})))
					case "agents":
						lines = append(lines, claudeLine(claudeUser(firstTime, "# AGENTS.md instructions\nSynthetic setup.")))
					case "environment":
						lines = append(lines, claudeLine(claudeUser(firstTime, "<environment_context>Synthetic setup.</environment_context>")))
					}
					lines = append(lines,
						claudeLine(claudeUser(firstTime, firstText)),
						claudeAssistant("2026-10-02T09:01:00Z", textBlock("Synthetic reply.")),
						claudeLine(claudeUser("2026-10-02T09:02:00Z", "Did you test the synthetic check?")),
					)
				} else {
					name = filepath.Join(store, "rollout-test.jsonl")
					lines = append(lines, `{"type":"session_meta","payload":{"id":"session-test","cwd":"/synthetic","source":"cli"}}`)
					switch tc.injection {
					case "meta":
						lines = append(lines, codexMeta(firstTime, "/synthetic", "cli"))
					case "tool":
						lines = append(lines, `{"type":"response_item","payload":{"type":"function_call_output","output":"Synthetic output."}}`)
					case "agents":
						lines = append(lines, codexMessage(firstTime, "user", "# AGENTS.md instructions\nSynthetic setup."))
					case "environment":
						lines = append(lines, codexMessage(firstTime, "user", "<environment_context>Synthetic setup.</environment_context>"))
					}
					lines = append(lines,
						codexMessage(firstTime, "user", firstText),
						codexMessage("2026-10-02T09:01:00Z", "assistant", "Synthetic reply."),
						codexMessage("2026-10-02T09:02:00Z", "user", "Did you test the synthetic check?"),
					)
				}
				writeLines(t, name, lines...)
				env := []string{"HOME=" + home}
				for _, mode := range []string{"corrections", "scorecard", "recall"} {
					args := []string{mode, "--all-workspaces", "--harness", harness, "--" + harness + "-root", store, "--since", "2026-10-01"}
					if mode == "recall" {
						args = append(args, "--topic", "synthetic")
					}
					out, code := run(t, root, env, tool, append(args, "--format", "json")...)
					var report sessionMinePromptReport
					if code != 0 || json.Unmarshal([]byte(out), &report) != nil {
						t.Fatalf("%s JSON (%d): %s", mode, code, out)
					}
					wantRecords := 1
					if tc.noRecord {
						wantRecords = 0
					}
					if p := report.Paseo; p == nil || p.Records != wantRecords || p.Linked != tc.wantLinked || p.Unreadable != tc.wantUnreadable {
						t.Errorf("%s provenance=%+v, want records=%d linked=%d unreadable=%d", mode, p, wantRecords, tc.wantLinked, tc.wantUnreadable)
					}
					wantHuman, wantPrompt, wantMachineHuman := 2, 0, 0
					if tc.wantLinked > 0 {
						wantHuman, wantPrompt, wantMachineHuman = 1, 1, 1
					}
					if tc.beforeSince {
						wantPrompt = 0
					}
					if report.MachinePromptTurns == nil || *report.MachinePromptTurns != wantPrompt || report.HumanInMachine == nil || *report.HumanInMachine != wantMachineHuman {
						t.Errorf("%s missing or wrong prompt/human-in-machine counters: %s", mode, out)
					}
					src := report.BySource[harness]
					if src.HumanTurns != wantHuman || src.PromptTurns != wantPrompt || src.HumanInMachine != wantMachineHuman {
						t.Errorf("%s source=%+v, want human=%d prompt=%d human-in-machine=%d", mode, src, wantHuman, wantPrompt, wantMachineHuman)
					}
					wantMachine := 0
					if harness == "claude" && tc.injection == "structural" {
						wantMachine = 1
					}
					if mode != "recall" && (report.HumanTurns != wantHuman || report.MachineTurns != wantMachine) {
						t.Errorf("%s human/machine counts wrong: %s", mode, out)
					}
					if mode == "recall" {
						// The initial task stays the session's first prompt after a human reply.
						wantFirst := "ontinue the synthetic check"
						if tc.beforeSince {
							wantFirst = "Did you test"
						}
						if len(report.Sessions) != 1 || !strings.Contains(report.Sessions[0].FirstPrompt, wantFirst) {
							t.Errorf("recall first prompt wrong, want %q: %s", wantFirst, out)
						}
					}
					if mode == "corrections" && report.ClassifiedTurns != wantHuman {
						t.Errorf("corrections include machine prompt: %s", out)
					}
					if mode == "scorecard" {
						total := 0
						for _, n := range report.Corrections {
							total += n
						}
						if total != wantHuman || len(report.ByWorkspace) != 1 || report.ByWorkspace[0].HumanTurns != wantHuman || report.ByWorkspace[0].MachinePromptTurns != wantPrompt {
							t.Errorf("scorecard correction/workspace counts wrong: %s", out)
						}
					}
					out, code = run(t, root, env, tool, args...)
					if code != 0 {
						t.Fatalf("%s text (%d): %s", mode, code, out)
					}
					sourcePos := strings.Index(out, "sources: skipped")
					provenancePos := strings.Index(out, "Paseo prompt provenance:")
					if sourcePos < 0 || provenancePos < sourcePos || !strings.Contains(out, "machine prompt turns:") || !strings.Contains(out, "human turns in machine-created sessions:") {
						t.Errorf("%s new diagnostics must follow existing output: %s", mode, out)
					}
					if wantRecords == 0 && !strings.Contains(out, "no agent records found; initial prompts keep transcript classification") {
						t.Errorf("%s missing stdout fallback note: %s", mode, out)
					}
					if mode == "scorecard" {
						lines := strings.Split(out, "\n")
						if len(lines) < 4 || !strings.HasPrefix(lines[2], "sessions: ") || !strings.HasPrefix(lines[3], "corrections: ") {
							t.Errorf("existing scalar positions changed: %s", out)
						}
					}
				}
			})
		}
	}
}
