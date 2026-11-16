package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

type followUpCounts struct {
	Workspace    string   `json:"workspace"`
	FollowUps    *int     `json:"follow_ups"`
	Nudges       *int     `json:"nudges"`
	CloseOffs    *int     `json:"close_off_requests"`
	NudgeRate    *float64 `json:"nudges_per_100_follow_ups"`
	CloseOffRate *float64 `json:"close_off_requests_per_100_follow_ups"`
}

// followUpExports writes invented OpenCode exports: a root session that opens
// with a scheduled prompt, a root session whose first typed turn precedes
// --since, a child session, and a root session in another workspace.
func followUpExports(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	closing := openCodeMessage("user", "Wrap up now.")
	closing["skills"] = []any{map[string]any{"id": "close-off", "name": "close-off", "text": "INVENTED-BODY"}}
	write("scheduled.json", openCodeExport(openCodeInfo("ses_scheduled", "/work/demo", ""),
		// A marked scheduled prompt is the first prompt, so the reply after
		// it is a follow-up.
		openCodeMessage("user", "Daily invented check.\nUnattended run: report only."),
		openCodeMessage("assistant", "", textBlock("Report ready.")),
		openCodeMessage("user", "Thanks. Now fix the invented flaky test."),
		openCodeMessage("assistant", "", textBlock("Working.")),
		openCodeMessage("user", "still at it?"),
		openCodeMessage("user", `<subagent id="ses_kid">child idle</subagent>`),
		openCodeMessage("synthetic", "Continue."),
		closing,
	))
	// Child turns are subagent prompts; none of them counts.
	write("child.json", openCodeExport(openCodeInfo("ses_kid", "/work/demo", "ses_scheduled"),
		openCodeMessage("user", "Invented child task."),
		openCodeMessage("user", "continue"),
		openCodeMessage("user", "close off"),
	))
	old := openCodeMessage("user", "Invented opening request.")
	old["time"] = map[string]any{"created": 1767225600000}
	write("window.json", openCodeExport(openCodeInfo("ses_window", "/work/demo", ""),
		old,
		openCodeMessage("user", longContinue),
		openCodeMessage("user", shortStuck),
		openCodeMessage("user", "go on"),
		openCodeMessage("user", "Please close-off this session."),
	))
	write("other.json", openCodeExport(openCodeInfo("ses_other", "/work/other", ""),
		// A notification is never a prompt turn, so the request after it is
		// the first prompt, not a follow-up.
		openCodeMessage("user", `<paseo-system kind="notice">invented notice</paseo-system>`),
		openCodeMessage("user", "Invented single request."),
	))
	return dir
}

const (
	longContinue = "continue with the migration to staging and then deploy"
	shortStuck   = "I got stuck on the form, can you fill it in for me please"
)

func TestOpenCodeMinerFollowUps(t *testing.T) {
	root := repoRoot(t)
	tool := buildOpenCodeMine(t, root)
	for _, text := range []string{longContinue, shortStuck} {
		if utf8.RuneCountInString(text) > 60 {
			t.Fatalf("fixture %q must be at most 60 characters", text)
		}
	}
	dir := followUpExports(t)
	base := []string{"scorecard", "--harness", "opencode", "--opencode-export", dir, "--since", "2026-11-01"}
	check := func(label string, got followUpCounts, follow, nudges, closes int, nudgeRate, closeRate float64) {
		t.Helper()
		if got.FollowUps == nil || got.Nudges == nil || got.CloseOffs == nil {
			t.Fatalf("%s: follow-up keys missing: %+v", label, got)
		}
		if *got.FollowUps != follow || *got.Nudges != nudges || *got.CloseOffs != closes {
			t.Errorf("%s: follow-ups/nudges/close-offs = %d/%d/%d, want %d/%d/%d", label, *got.FollowUps, *got.Nudges, *got.CloseOffs, follow, nudges, closes)
		}
		if follow == 0 {
			if got.NudgeRate != nil || got.CloseOffRate != nil {
				t.Errorf("%s: rates must be null without follow-ups: %+v", label, got)
			}
			return
		}
		if got.NudgeRate == nil || got.CloseOffRate == nil || *got.NudgeRate != nudgeRate || *got.CloseOffRate != closeRate {
			t.Errorf("%s: rates wrong: %+v", label, got)
		}
	}

	// One workspace: 3 follow-ups in the scheduled session (the first reply to
	// the scheduled prompt counts), 4 in the window session (its first prompt
	// precedes --since but still counts as first).
	output, code := run(t, root, nil, tool, append(append([]string{}, base...), "--workspace", "/work/demo", "--format", "json")...)
	var card struct {
		followUpCounts
		HumanTurns   int `json:"human_turns"`
		MachineTurns int `json:"machine_turns"`
	}
	if code != 0 || json.Unmarshal([]byte(output), &card) != nil {
		t.Fatalf("scorecard (%d): %s", code, output)
	}
	check("workspace", card.followUpCounts, 7, 3, 2, 42.9, 28.6)
	if card.HumanTurns != 7 || card.MachineTurns != 3 {
		t.Errorf("existing human/machine counts changed: %d/%d", card.HumanTurns, card.MachineTurns)
	}
	if !strings.Contains(output, `"close_off_requests_per_100_follow_ups": 28.6`) {
		t.Errorf("rate must use one decimal: %s", output)
	}

	output, code = run(t, root, nil, tool, append(append([]string{}, base...), "--workspace", "/work/demo")...)
	for _, want := range []string{
		"sessions: 3, human turns: 7, machine turns: 3",
		"opencode follow-ups: 7, nudges: 3 (42.9 per 100 follow-ups), close-off requests: 2 (28.6 per 100 follow-ups)",
	} {
		if code != 0 || !strings.Contains(output, want) {
			t.Errorf("text scorecard lacks %q (%d): %s", want, code, output)
		}
	}

	output, code = run(t, root, nil, tool, append(append([]string{}, base...), "--workspace", "/work/other")...)
	if want := "opencode follow-ups: 0, nudges: 0 (n/a per 100 follow-ups), close-off requests: 0 (n/a per 100 follow-ups)"; code != 0 || !strings.Contains(output, want) {
		t.Errorf("zero follow-ups must print n/a (%d): %s", code, output)
	}

	for _, sanitize := range []bool{false, true} {
		args := append(append([]string{}, base...), "--all-workspaces", "--format", "json")
		if sanitize {
			args = append(args, "--sanitize")
		}
		output, code = run(t, root, nil, tool, args...)
		var all struct {
			followUpCounts
			ByWorkspace []followUpCounts `json:"by_workspace"`
		}
		if code != 0 || json.Unmarshal([]byte(output), &all) != nil || len(all.ByWorkspace) != 2 {
			t.Fatalf("all workspaces (%d): %s", code, output)
		}
		check("all", all.followUpCounts, 7, 3, 2, 42.9, 28.6)
		for _, ws := range all.ByWorkspace {
			if ws.FollowUps != nil && *ws.FollowUps == 0 {
				check("other workspace", ws, 0, 0, 0, 0, 0)
			} else {
				check("demo workspace", ws, 7, 3, 2, 42.9, 28.6)
			}
		}
		if strings.Contains(output, "INVENTED-BODY") || (sanitize && strings.Contains(output, "/work/")) {
			t.Errorf("sanitize/attachment leak: %s", output)
		}
	}

	output, code = run(t, root, nil, tool, append(append([]string{}, base...), "--all-workspaces")...)
	for _, want := range []string{
		"by workspace, opencode follow-ups (follow-ups / nudges / close-off requests / nudges per 100 / close-off requests per 100):",
		"- /work/demo: 7 / 3 / 2 / 42.9 / 28.6",
		"- /work/other: 0 / 0 / 0 / n/a / n/a",
		// Existing lines stay unchanged.
		"by workspace (sessions / human turns / corrections / skill loads / gate blocks / verified / not verified / sleep calls):",
	} {
		if code != 0 || !strings.Contains(output, want) {
			t.Errorf("text scorecard lacks %q (%d): %s", want, code, output)
		}
	}
}
