package contracts

import "testing"

func TestShippedGuidanceUsesSessionTools(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"autonomous-run/SKILL.md",
		"writing-agent-instructions/assets/global-AGENTS.md",
		"writing-agent-instructions/assets/global-CLAUDE.md",
		"verify-and-finish/references/run-receipt.md",
		"verify-and-finish/references/verification-driver.md",
	} {
		body := normalized(read(t, root, "plugins/megapowers/skills/"+rel))
		for _, phrase := range []string{"authenticated clis", "environment key names", "environment variables, seed data, and authentication", "gh run watch", "gh pr checks"} {
			requireAbsent(t, body, phrase, rel)
		}
	}
}
