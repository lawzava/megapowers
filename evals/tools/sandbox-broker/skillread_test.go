//go:build linux

package main

import "testing"

func TestSkillReadEventsRequireABodyRead(t *testing.T) {
	for command, want := range map[string]int{
		"sed -n 1,200p /p/skills/humanizing-prose/SKILL.md":        1,
		"cat /p/skills/humanizing-prose/SKILL.md | head -40":       1,
		"bash -lc 'head -80 /p/skills/verify-and-finish/SKILL.md'": 1,
		"printf skills/humanizing-prose/SKILL.md":                  0,
		"echo /p/skills/humanizing-prose/SKILL.md":                 0,
		"test -e /p/skills/humanizing-prose/SKILL.md":              0,
		"ls -l /p/skills/humanizing-prose/SKILL.md":                0,
		"stat /p/skills/humanizing-prose/SKILL.md":                 0,
	} {
		if got := len(skillReadEvents(command, 0)); got != want {
			t.Errorf("skillReadEvents(%q) = %d events, want %d", command, got, want)
		}
	}
}
