package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// openCodeCurrentExport builds the unwrapped {info, messages} shape that
// current OpenCode session exports use. All text is invented.
func openCodeCurrentExport(info map[string]any, messages ...map[string]any) string {
	return openCodeJSON(map[string]any{"info": info, "messages": messages})
}

func openCodeCurrentSession() string {
	attached := openCodeMessage("user", "Please close off the session.")
	attached["skills"] = []any{map[string]any{"id": "close-off", "name": "close-off", "text": "INVENTED-ATTACHED-BODY"}}
	skillCall := map[string]any{"type": "tool", "name": "skill", "state": map[string]any{"status": "completed", "input": map[string]any{"id": "verify-and-finish"}, "content": []any{}}}
	return openCodeCurrentExport(openCodeInfo("ses_current", "/work/demo", ""),
		attached,
		openCodeMessage("assistant", "", map[string]any{"type": "reasoning", "text": "hidden"}, skillCall, textBlock("Done.")),
		openCodeMessage("user", `<paseo-system kind="notice">agent finished</paseo-system>`),
		openCodeMessage("user", `<subagent id="ses_child">child idle</subagent>`),
		openCodeMessage("user", `<shell id="1">exit 0</shell>`),
		openCodeMessage("synthetic", "Continue the job."),
		openCodeMessage("idle", ""),
		openCodeMessage("compaction", ""),
		openCodeMessage("agent-switched", ""),
		openCodeMessage("system", "system text"),
		openCodeMessage("user", "Too long, shorter please."),
	)
}

// Current exports carry harness-attached skills on user messages and machine
// notifications as user text; both must stay apart from typed turns and model
// skill loads.
func TestOpenCodeMinerCurrentExportShape(t *testing.T) {
	root := repoRoot(t)
	tool := buildOpenCodeMine(t, root)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ses_current.json"), []byte(openCodeCurrentSession()), 0o600); err != nil {
		t.Fatal(err)
	}
	base := []string{"--all-workspaces", "--harness", "opencode", "--opencode-export", dir}

	output, code := run(t, root, nil, tool, append([]string{"scorecard", "--format", "json"}, base...)...)
	var card struct {
		scorecard
		MachineTurns   int `json:"machine_turns"`
		AttachedSkills []struct {
			Name        string `json:"name"`
			Attachments int    `json:"attachments"`
			Sessions    int    `json:"sessions"`
		} `json:"attached_skills"`
		BySource map[string]struct {
			SkillAttachments int `json:"skill_attachments"`
			SkillLoads       int `json:"skill_loads"`
		} `json:"by_source"`
	}
	if code != 0 || json.Unmarshal([]byte(output), &card) != nil {
		t.Fatalf("scorecard (%d): %s", code, output)
	}
	if card.SessionsScanned != 1 || card.HumanTurns != 2 || card.MachineTurns != 4 {
		t.Errorf("sessions/human/machine = %d/%d/%d, want 1/2/4: %s", card.SessionsScanned, card.HumanTurns, card.MachineTurns, output)
	}
	if len(card.Skills) != 1 || card.Skills[0].Name != "verify-and-finish" || card.Skills[0].Loads != 1 {
		t.Errorf("model skill loads must exclude attachments: %+v", card.Skills)
	}
	if len(card.AttachedSkills) != 1 || card.AttachedSkills[0].Name != "close-off" || card.AttachedSkills[0].Attachments != 1 || card.AttachedSkills[0].Sessions != 1 {
		t.Errorf("attached skills: %+v", card.AttachedSkills)
	}
	if source := card.BySource["opencode"]; source.SkillAttachments != 1 || source.SkillLoads != 1 {
		t.Errorf("by_source attachment/load counts: %+v", source)
	}
	if strings.Contains(output, "INVENTED-ATTACHED-BODY") {
		t.Error("attached skill body leaked into the report")
	}

	output, code = run(t, root, nil, tool, append([]string{"scorecard"}, base...)...)
	for _, want := range []string{"attached skills (attachments / sessions):", "- close-off: 1 / 1", "- verify-and-finish: 1 / 1 / 1", "skill attachments 1"} {
		if code != 0 || !strings.Contains(output, want) {
			t.Errorf("text scorecard lacks %q (%d): %s", want, code, output)
		}
	}

	output, code = run(t, root, nil, tool, append([]string{"corrections", "--format", "json"}, base...)...)
	var corrections struct {
		sessionMineCorrections
		MachineTurns int `json:"machine_turns"`
		BySource     map[string]struct {
			SkillAttachments int `json:"skill_attachments"`
		} `json:"by_source"`
	}
	if code != 0 || json.Unmarshal([]byte(output), &corrections) != nil {
		t.Fatalf("corrections (%d): %s", code, output)
	}
	if corrections.HumanTurns != 2 || corrections.MachineTurns != 4 || corrections.BySource["opencode"].SkillAttachments != 1 {
		t.Errorf("corrections counts: %s", output)
	}
	if verbosity := findClass(corrections.OneOffs, "verbosity"); verbosity == nil || verbosity.Count != 1 {
		t.Errorf("typed correction lost: %s", output)
	}
	for _, leaked := range []string{"agent finished", "child idle", "exit 0", "INVENTED-ATTACHED-BODY"} {
		if strings.Contains(output, leaked) {
			t.Errorf("machine or attachment text counted as a human excerpt: %q", leaked)
		}
	}
}

// The API can return the unwrapped shape too; the exporter must keep it.
func TestOpenCodeExporterCurrentAPIShape(t *testing.T) {
	root := repoRoot(t)
	exporter := buildOpenCodeExporter(t, root)
	tool := buildOpenCodeMine(t, root)
	info := openCodeInfo("ses_current", "/work/demo", "")
	command, _ := fakeSessionMineOpenCode(t, map[string][]string{
		"/api/session?limit=100&order=desc":            {openCodeJSON(map[string]any{"data": []any{info}, "cursor": map[string]any{}})},
		"/api/experimental/session/ses_current/export": {openCodeCurrentSession()},
	})
	outDir := filepath.Join(t.TempDir(), "exports")
	out, code := run(t, root, nil, exporter, "--output", outDir, "--all-workspaces", "--opencode-cmd", command)
	var summary struct {
		Written int    `json:"exports_written"`
		Status  string `json:"status"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &summary) != nil || summary.Written != 1 || summary.Status != "complete" {
		t.Fatalf("unwrapped export (%d): %s", code, out)
	}
	output, code := run(t, root, nil, tool, "scorecard", "--all-workspaces", "--harness", "opencode", "--opencode-export", outDir, "--format", "json")
	var card scorecard
	if code != 0 || json.Unmarshal([]byte(output), &card) != nil || card.SessionsScanned != 1 || card.HumanTurns != 2 {
		t.Fatalf("mined export (%d): %s", code, output)
	}
}
