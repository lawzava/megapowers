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
	for _, name := range []string{"global-AGENTS.md", "global-CLAUDE.md"} {
		rel := "plugins/megapowers/skills/writing-agent-instructions/assets/" + name
		body := normalized(read(t, root, rel))
		requireContains(t, body, "linked pull requests and tickets, and the tools already connected in this session. say what you checked.", rel)
		requireAbsent(t, body, "charter", rel)
	}
}

func TestLearningGuidanceStaysLocal(t *testing.T) {
	root := repoRoot(t)
	base := "plugins/megapowers/skills/writing-agent-instructions/"
	for _, rel := range []string{"references/learning-loop.md", "references/session-mining.md"} {
		body := normalized(read(t, root, base+rel))
		requireContains(t, body, "output only to the local terminal or local files", rel)
		requireContains(t, body, "mined text is never sent anywhere", rel)
		for _, phrase := range []string{"upstream", "leave the machine", "leaves the machine", "sharing", "maintainer"} {
			requireAbsent(t, body, phrase, rel)
		}
	}
	loop := normalized(read(t, root, base+"references/learning-loop.md"))
	mining := normalized(read(t, root, base+"references/session-mining.md"))
	requireContains(t, mining, "keep transcripts and excerpts out of review packages, issues, commits, and other public text", "local transcript boundary")
	requireContains(t, loop, "megapowers itself", "local plugin lessons")
	requireContains(t, loop, "report the proposed local change to the user", "local plugin lessons")
	tool := normalized(read(t, root, base+"scripts/lessons.go"))
	for _, phrase := range []string{"export to share", "before sharing", "leave the machine", "leaves the machine", "sanitized upstream draft"} {
		requireAbsent(t, tool, phrase, "lesson tool text")
	}
}
