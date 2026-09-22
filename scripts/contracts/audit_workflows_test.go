package contracts

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// These source contracts prevent known instruction regressions. They do not
// qualify runtime goal recovery or model adherence.
func TestAuditWorkflowNativeGoalAuthority(t *testing.T) {
	body := read(t, repoRoot(t), "plugins/megapowers/skills/autonomous-run/SKILL.md")
	normalized := strings.ToLower(strings.Join(strings.Fields(body), " "))
	for _, pattern := range []string{
		`compaction[^.]*does not[^.]*revok[^.]*authorit`,
		`native[^.]*current[^.]*tool contract`,
		`checkpoint labels[^.]*not[^.]*native[^.]*status`,
		`experimental[^.]*runtime[^.]*resume[^.]*compaction`,
		`charters and checkpoints[^.]*not authority[^.]*verify`,
	} {
		if !regexp.MustCompile(pattern).MatchString(normalized) {
			t.Errorf("native-goal contract missing %s", pattern)
		}
	}
	for _, obsolete := range []string{
		"compaction, ordinary handoff, or harness switch does not inherit authority",
		"Use `paused` when a cap",
		"Use `blocked` only for a concrete",
	} {
		requireAbsent(t, normalized, strings.ToLower(obsolete), "obsolete native lifecycle policy")
	}
	var catalog struct {
		Skills []struct{ Name, Status string }
	}
	if err := json.Unmarshal([]byte(read(t, repoRoot(t), "plugins/megapowers/skills/catalog.json")), &catalog); err != nil {
		t.Fatal(err)
	}
	for _, skill := range catalog.Skills {
		if skill.Name == "autonomous-run" {
			if skill.Status != "experimental" {
				t.Errorf("autonomous-run status = %q; runtime recovery is not qualified", skill.Status)
			}
			return
		}
	}
	t.Fatal("autonomous-run missing from catalog")
}

func TestAuditWorkflowObservedFailureBoundaries(t *testing.T) {
	for _, tc := range []struct {
		skill    string
		patterns []string
	}{
		{"orchestrating", []string{
			`shared guidance[^.]*one writer|one writer[^.]*shared guidance`,
			`(event|asynchronous)[^.]*wait`,
			`wait[^.]*tool[^.]*deadline`,
			`unchanged[^.]*status|status[^.]*unchanged`,
		}},
		{"safe-effects", []string{
			`before[^.]*paid batch[^.]*resolved[^.]*ids[^.]*count`,
			`bind[^.]*results[^.]*ids`,
		}},
		{"mcp-setup", []string{
			`narrow[^.]*discovery[^.]*does not[^.]*capability`,
			`(launcher|filesystem)[^.]*authentication[^.]*quota`,
		}},
		{"systematic-debugging", []string{
			`(launcher|filesystem)[^.]*authentication[^.]*quota`,
		}},
	} {
		t.Run(tc.skill, func(t *testing.T) {
			body := read(t, repoRoot(t), "plugins/megapowers/skills/"+tc.skill+"/SKILL.md")
			normalized := strings.ToLower(strings.Join(strings.Fields(body), " "))
			for _, pattern := range tc.patterns {
				if !regexp.MustCompile(pattern).MatchString(normalized) {
					t.Errorf("audit workflow contract missing %s", pattern)
				}
			}
		})
	}
}
