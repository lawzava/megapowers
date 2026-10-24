package contracts

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

type scorecardSkill struct {
	Name                 string `json:"name"`
	Loads                int    `json:"loads"`
	Sessions             int    `json:"sessions"`
	FollowedByCorrection int    `json:"followed_by_correction"`
}

type scorecardRun struct {
	ID                   string `json:"id"`
	Records              int    `json:"records"`
	Oracle               int    `json:"oracle"`
	Continuations        int    `json:"continuations"`
	Blocked              int    `json:"blocked"`
	HoldoutRegistrations int    `json:"holdout_registrations"`
	Rebaselines          int    `json:"rebaselines"`
	HighRiskDecisions    int    `json:"high_risk_decisions"`
}

type scorecardWorkspace struct {
	Workspace   string `json:"workspace"`
	Sessions    int    `json:"sessions"`
	HumanTurns  int    `json:"human_turns"`
	Corrections int    `json:"corrections"`
	SkillLoads  int    `json:"skill_loads"`
	GateBlocks  int    `json:"gate_blocks"`
}

type scorecard struct {
	Scope           string         `json:"scope"`
	SessionsScanned int            `json:"sessions_scanned"`
	HumanTurns      int            `json:"human_turns"`
	Corrections     map[string]int `json:"corrections"`
	Skills          []scorecardSkill
	Gates           map[string]int `json:"gates"`
	Reports         struct {
		Verified    int `json:"verified"`
		NotVerified int `json:"not_verified"`
	} `json:"reports"`
	SleepCalls  int                  `json:"sleep_calls"`
	Runs        []scorecardRun       `json:"runs"`
	ByWorkspace []scorecardWorkspace `json:"by_workspace"`
}

func slugFor(path string) string {
	var b strings.Builder
	for _, r := range path {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// withCwd stamps a Claude record with its working directory, as real
// transcripts do.
func withCwd(cwd string, lines ...string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		var record map[string]any
		_ = json.Unmarshal([]byte(line), &record)
		record["cwd"] = cwd
		body, _ := json.Marshal(record)
		out[i] = string(body)
	}
	return out
}

func toolUse(id, name string, input map[string]any) map[string]any {
	return map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}
}

func codexCall(ts, kind, field, text string) string {
	body, _ := json.Marshal(map[string]any{"timestamp": ts, "type": "response_item", "payload": map[string]any{"type": kind, "name": "exec", field: text}})
	return string(body)
}

// The scorecard turns transcripts and run journals into per-project outcome
// signals: which skills loaded, which corrections followed them, which gates
// fired, and how many reports claimed verification.
func TestSessionMineScorecard(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "ws")
	claudeRoot := filepath.Join(tmp, "claude-projects")
	codexRoot := filepath.Join(tmp, "codex-sessions")
	gate := "PreToolUse:Bash hook error: Load and follow the megapowers verify-and-finish skill before this completion step, then run the command again."
	writeLines(t, filepath.Join(claudeRoot, slugFor(workspace), "sc1.jsonl"), withCwd(workspace,
		claudeUser("2026-10-10T09:00:00Z", "Ship the parser."),
		claudeAssistant("2026-10-10T09:01:00Z", toolUse("t1", "Skill", map[string]any{"skill": "megapowers:test-first-implementation"})),
		claudeAssistant("2026-10-10T09:02:00Z", toolUse("t2", "Bash", map[string]any{"command": "sleep 30 && go test ./..."})),
		claudeUserBlocks("2026-10-10T09:03:00Z", nil, map[string]any{"type": "tool_result", "tool_use_id": "t2", "is_error": true, "content": gate}),
		// Reading the gate's source is not a gate block.
		claudeAssistant("2026-10-10T09:03:10Z", toolUse("t2b", "Read", map[string]any{"file_path": "/src/hooks/gate_context.go"})),
		claudeUserBlocks("2026-10-10T09:03:20Z", nil, map[string]any{"type": "tool_result", "tool_use_id": "t2b", "content": "const completionDeny = \"" + gate + "\""}),
		claudeUserBlocks("2026-10-10T09:03:30Z", nil, map[string]any{"type": "tool_result", "tool_use_id": "t2c", "is_error": true, "content": "grep: " + gate}),
		claudeAssistant("2026-10-10T09:04:00Z", toolUse("t3", "Skill", map[string]any{"skill": "megapowers:verify-and-finish"})),
		claudeAssistant("2026-10-10T09:05:00Z", textBlock("VERIFIED: the parser passes go test ./...")),
		claudeUser("2026-10-10T09:06:00Z", "You didn't verify the empty input."),
		claudeAssistant("2026-10-10T09:07:00Z", toolUse("t4", "Bash", map[string]any{"command": "sleep 5"})),
		claudeAssistant("2026-10-10T09:08:00Z", textBlock("NOT VERIFIED. Remaining: the empty input case.")),
		claudeUser("2026-10-10T09:09:00Z", "Thanks."),
	)...)
	other := filepath.Join(tmp, "private-client")
	writeLines(t, filepath.Join(claudeRoot, slugFor(other), "o1.jsonl"), withCwd(other,
		claudeUser("2026-10-10T10:00:00Z", "Run the private job."),
		claudeAssistant("2026-10-10T10:01:00Z", toolUse("p1", "Skill", map[string]any{"skill": "private-plugin:secret-skill"})),
		// A malformed request that starts with megapowers: must not carry its
		// argument into a sanitized card.
		claudeAssistant("2026-10-10T10:02:00Z", toolUse("p2", "Skill", map[string]any{"skill": "megapowers:writing-agent-instructions /srv/private-client"})),
	)...)
	// A file without any cwd is keyed by its project directory, so it never
	// merges with another project.
	writeLines(t, filepath.Join(claudeRoot, "-legacy-project", "l1.jsonl"),
		claudeUser("2026-10-10T11:00:00Z", "Old session without cwd."),
	)
	writeLines(t, filepath.Join(claudeRoot, "-legacy-two", "l2.jsonl"),
		claudeUser("2026-10-10T12:00:00Z", "Another old session without cwd."),
	)
	writeLines(t, filepath.Join(codexRoot, "2026", "10", "11", "rollout-2026-10-11T10-00-00-sc.jsonl"),
		codexMeta("2026-10-11T10:00:00Z", workspace, "cli"),
		codexMessage("2026-10-11T10:00:01Z", "user", "Plan the cache."),
		codexCall("2026-10-11T10:00:02Z", "custom_tool_call", "input", `tools.exec_command({cmd:"cat /opt/agent/.codex/plugins/cache/megapowers/megapowers/0.41.0/skills/design-and-plan/SKILL.md"})`),
		codexCall("2026-10-11T10:00:03Z", "custom_tool_call_output", "output", "PreToolUse:Bash hook error: Load and follow the megapowers safe-effects skill before this outward effect"),
		codexCall("2026-10-11T10:00:03Z", "custom_tool_call_output", "output", "Script completed\nOutput:\neffectDeny = \"Load and follow the megapowers safe-effects skill before this outward effect\""),
		codexMessage("2026-10-11T10:00:04Z", "assistant", "VERIFIED: the plan covers C1."),
		codexMessage("2026-10-11T10:00:05Z", "user", "Too long, make it shorter."),
	)
	writeLines(t, filepath.Join(workspace, ".megapowers", "run", "r1", "journal.jsonl"),
		`{"event":"oracle","evidence":"go test ok"}`,
		`{"event":"continuation","item":"x"}`,
		`{"event":"blocked","item":"x"}`,
		`{"event":"holdout","holdout_sha256":"sha256:a"}`,
		`{"event":"holdout","holdout_sha256":"sha256:b","decision":"rebaseline holdout scenarios","risk":"high"}`,
		`{"event":"decision","decision":"use v2 API","risk":"high"}`,
	)
	base := []string{"scorecard", "--claude-root", claudeRoot, "--codex-root", codexRoot}

	output, code := run(t, root, nil, tool, append(base, "--workspace", workspace, "--format", "json")...)
	if code != 0 {
		t.Fatalf("scorecard failed (%d): %s", code, output)
	}
	var card scorecard
	if err := json.Unmarshal([]byte(output), &card); err != nil {
		t.Fatalf("scorecard json does not parse: %v\n%s", err, output)
	}
	if card.SessionsScanned != 2 || card.HumanTurns != 5 {
		t.Errorf("sessions %d, human turns %d; want 2 and 5", card.SessionsScanned, card.HumanTurns)
	}
	if card.Corrections["verification"] != 1 || card.Corrections["verbosity"] != 1 || len(card.Corrections) != 2 {
		t.Errorf("corrections = %v, want verification 1 and verbosity 1", card.Corrections)
	}
	want := map[string]scorecardSkill{
		"megapowers:test-first-implementation": {Loads: 1, Sessions: 1, FollowedByCorrection: 1},
		"megapowers:verify-and-finish":         {Loads: 1, Sessions: 1, FollowedByCorrection: 1},
		"megapowers:design-and-plan":           {Loads: 1, Sessions: 1, FollowedByCorrection: 1},
	}
	if len(card.Skills) != len(want) {
		t.Errorf("skills = %+v, want %d entries", card.Skills, len(want))
	}
	for _, skill := range card.Skills {
		w, ok := want[skill.Name]
		if !ok || skill.Loads != w.Loads || skill.Sessions != w.Sessions || skill.FollowedByCorrection != w.FollowedByCorrection {
			t.Errorf("skill %+v, want %+v", skill, w)
		}
	}
	if card.Gates["verify-and-finish"] != 1 || card.Gates["safe-effects"] != 1 {
		t.Errorf("gates = %v, want one completion and one effects block", card.Gates)
	}
	if card.Reports.Verified != 2 || card.Reports.NotVerified != 1 {
		t.Errorf("reports = %+v, want 2 verified and 1 not verified", card.Reports)
	}
	if card.SleepCalls != 2 {
		t.Errorf("sleep calls = %d, want 2", card.SleepCalls)
	}
	if len(card.Runs) != 1 {
		t.Fatalf("runs = %+v, want one", card.Runs)
	}
	if r := card.Runs[0]; r.ID != "r1" || r.Records != 6 || r.Oracle != 1 || r.Continuations != 1 || r.Blocked != 1 || r.HoldoutRegistrations != 2 || r.Rebaselines != 1 || r.HighRiskDecisions != 2 {
		t.Errorf("run = %+v", r)
	}

	// --since drops earlier events as well as messages.
	output, code = run(t, root, nil, tool, append(base, "--workspace", workspace, "--since", "2026-10-11", "--format", "json")...)
	var later scorecard
	if code != 0 || json.Unmarshal([]byte(output), &later) != nil {
		t.Fatalf("scorecard --since failed (%d): %s", code, output)
	}
	if later.SessionsScanned != 1 || later.SleepCalls != 0 || later.Reports.Verified != 1 || len(later.Skills) != 1 {
		t.Errorf("--since scorecard = %+v", later)
	}

	output, code = run(t, root, nil, tool, append(base, "--workspace", workspace)...)
	for _, wantText := range []string{"megapowers:verify-and-finish", "gates:", "reports: 2 verified, 1 not verified", "run r1"} {
		if code != 0 || !strings.Contains(output, wantText) {
			t.Errorf("text scorecard missing %q (%d): %s", wantText, code, output)
		}
	}

	// A sanitized all-workspace card is what may leave a user's machine for the
	// upstream loop: counts only, no paths, run IDs, or non-megapowers names.
	output, code = run(t, root, nil, tool, append(base, "--all-workspaces", "--sanitize", "--format", "json")...)
	if code != 0 {
		t.Fatalf("sanitized scorecard failed (%d): %s", code, output)
	}
	for _, leak := range []string{tmp, "private-client", "secret-skill", "private-plugin", `"r1"`, "/opt/agent", "legacy-project", "/srv/private-client"} {
		if strings.Contains(output, leak) {
			t.Errorf("sanitized scorecard leaks %q: %s", leak, output)
		}
	}
	var all scorecard
	if err := json.Unmarshal([]byte(output), &all); err != nil {
		t.Fatal(err)
	}
	if len(all.ByWorkspace) != 4 || all.SessionsScanned != 5 {
		t.Errorf("sanitized by_workspace = %+v, sessions %d; want 4 workspaces and 5 sessions", all.ByWorkspace, all.SessionsScanned)
	}
	for _, ws := range all.ByWorkspace {
		if ws.Sessions == 2 && ws.HumanTurns != 5 {
			t.Errorf("Claude and Codex sessions of one workspace must merge: %+v", ws)
		}
	}
	for _, ws := range all.ByWorkspace {
		if !strings.HasPrefix(ws.Workspace, "ws-") {
			t.Errorf("workspace key %q is not hashed", ws.Workspace)
		}
	}
	foundOther := false
	for _, skill := range all.Skills {
		foundOther = foundOther || skill.Name == "other"
	}
	if !foundOther {
		t.Errorf("non-megapowers skills must fold into other: %+v", all.Skills)
	}
	if output, code := run(t, root, nil, tool, append(base, "--workspace", workspace, "--sanitize", "--topic", "x")...); code != 2 {
		t.Errorf("--topic with scorecard: want 2, got %d: %s", code, output)
	}
	if output, code := run(t, root, nil, tool, "corrections", "--workspace", workspace, "--sanitize", "--claude-root", claudeRoot); code != 2 {
		t.Errorf("--sanitize outside scorecard: want 2, got %d: %s", code, output)
	}
}
