package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// requiredSkillFacts lists short facts each skill body must carry. Facts are
// matched as case-insensitive substrings of whitespace-normalized text so a
// rewrite that keeps the meaning does not break them. They protect behavior
// (a linked reference, a stop rule, a named artifact), not phrasing.
var requiredSkillFacts = map[string][]string{
	"autonomous-run": {
		"compaction does not revoke",
		"charter.md", "checkpoint.md", "journal.jsonl", "handoff.md",
		"native goal",
	},
	"design-and-plan": {
		"acceptance oracle",
		"requirement id",
		"references/openspec.md",
		"failing test",
		"do not create new directories",
	},
	"humanizing-prose": {
		"../../output-styles/megapowers.md",
		"every recommendation",
	},
	"independent-review": {
		"gives only context separation, not independence",
		"--approve-external",
		"author and provider labels must differ",
		"same declaration",
	},
	"mcp-setup": {
		"restart the session",
		"print the keys, never the values",
		"at most once per server",
		"user action",
	},
	"megapowers-doctor": {
		"run-hook.cmd",
		"doctor",
		"WARN",
		"references/fixes.md",
		"skills/<name>",
	},
	"orchestrating": {
		"native agents are the default",
		"one to three direct children",
		"one writer",
		"agent-capabilities.md",
		"cannot authorize",
		"references/native-dispatch.md",
	},
	"safe-effects": {
		"paid batch",
	},
	"systematic-debugging": {
		"quota",
		"unrelated caller cleanup",
		"same cause",
	},
	"test-first-implementation": {
		"production code follows a failing test",
		"references/go.md", "references/python.md", "references/typescript.md",
		"do not add production apis only for tests",
	},
	"upgrading-megapowers": {
		"references/channels.md",
		"verified no-op",
		"never install unreleased branch state",
	},
	"verify-and-finish": {
		"verified no-op",
		"verified: <claim>",
		"not verified. remaining: <gap>",
		"local build cannot prove",
	},
	"writing-agent-instructions": {
		"references/skills.md",
		"references/repository-instructions.md",
	},
}

// forbiddenSkillText lists text that must not return: removed duplicates of
// repository policy, self-authorizing claims, and incident-specific filler.
var forbiddenSkillText = map[string][]string{
	"autonomous-run":             {"Experimental pending"},
	"design-and-plan":            {"OpenSpec CLI"},
	"humanizing-prose":           {"named source, direct observation, or explicit uncertainty"},
	"independent-review":         {"provides context separation"},
	"mcp-setup":                  {"narrow discovery miss"},
	"orchestrating":              {"explicitly authorizes", "without dispatch is a contract violation"},
	"test-first-implementation":  {"skills supply defaults only where the repository is silent"},
	"verify-and-finish":          {"index size cap"},
	"writing-agent-instructions": {"in Go"},
}

// forbiddenEverywhere applies to every shipped skill body and the output style.
var forbiddenEverywhere = []string{
	"Write all new helper code",
	"Python or another scripting language",
	"—",
}

func normalized(body string) string {
	return strings.ToLower(strings.Join(strings.Fields(body), " "))
}

// The bare style name left the Claude Code style silently off for 20 days, so
// the doctor's fix must name the plugin-qualified value.
func TestDoctorFixesNameQualifiedStyle(t *testing.T) {
	root := repoRoot(t)
	fixes := read(t, root, "plugins/megapowers/skills/megapowers-doctor/references/fixes.md")
	requireContains(t, fixes, `"outputStyle": "megapowers:Megapowers"`, "doctor fixes")
}

func TestSkillRequiredFacts(t *testing.T) {
	root := repoRoot(t)
	for _, name := range catalogNames(t, root) {
		t.Run(name, func(t *testing.T) {
			body := read(t, root, "plugins/megapowers/skills/"+name+"/SKILL.md")
			text := normalized(body)
			for _, fact := range requiredSkillFacts[name] {
				if !strings.Contains(text, strings.ToLower(fact)) {
					t.Errorf("missing required fact %q", fact)
				}
			}
			for _, needle := range append(forbiddenSkillText[name], forbiddenEverywhere...) {
				requireAbsent(t, text, needle, "removed guidance")
			}
		})
	}
	style := read(t, root, "plugins/megapowers/output-styles/megapowers.md")
	for _, needle := range forbiddenEverywhere {
		requireAbsent(t, style, needle, "output style")
	}
}

func TestExperimentalStatusPinned(t *testing.T) {
	root := repoRoot(t)
	var catalog struct {
		Skills []struct{ Name, Status string }
	}
	if err := json.Unmarshal([]byte(read(t, root, "plugins/megapowers/skills/catalog.json")), &catalog); err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, skill := range catalog.Skills {
		status[skill.Name] = skill.Status
	}
	// autonomous-run: runtime resume and compaction recovery are unproven.
	// megapowers-doctor: no behavioral validation yet.
	for _, name := range []string{"autonomous-run", "megapowers-doctor"} {
		if status[name] != "experimental" {
			t.Errorf("%s status = %q, want experimental", name, status[name])
		}
	}
}

func TestWorkflowSkillDiscoveryBoundaries(t *testing.T) {
	root := repoRoot(t)
	tests := []struct {
		name     string
		positive []*regexp.Regexp
		negative []*regexp.Regexp
	}{
		{
			name: "design-and-plan",
			positive: []*regexp.Regexp{
				regexp.MustCompile(`(?i)specif|requirement|trade.?off|multi.?step plan`),
			},
			negative: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(mechanical|single obvious|straightforward).*(edit|change)|settled plan`),
			},
		},
		{
			name: "evidence-research",
			positive: []*regexp.Regexp{
				regexp.MustCompile(`(?i)evidence.*(beyond|outside).*(repository|repo)`),
			},
			negative: []*regexp.Regexp{
				regexp.MustCompile(`(?i)repository.only|code trac|provided (material|document)`),
			},
		},
		{
			name: "verify-and-finish",
			positive: []*regexp.Regexp{
				regexp.MustCompile(`(?i)(claim|verify|completion|completed).*(success|outcome)|hand.?off|commit|deploy`),
			},
			negative: []*regexp.Regexp{
				regexp.MustCompile(`(?i)intermediate|isolated test|naming.*(oracle|command)`),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := read(t, root, "plugins/megapowers/skills/"+test.name+"/SKILL.md")
			frontmatter, _, err := splitFrontmatter(document)
			if err != nil {
				t.Fatal(err)
			}
			description := parseFrontmatter(frontmatter)["description"]
			if len(description) > 420 {
				t.Fatalf("description is %d bytes; discovery text should stay discriminating", len(description))
			}
			for _, concept := range append(test.positive, test.negative...) {
				if !concept.MatchString(description) {
					t.Errorf("description misses discovery boundary %s", concept)
				}
			}
		})
	}
}

func TestWritingInstructionRoutesToReferences(t *testing.T) {
	root := repoRoot(t)
	document := read(t, root, "plugins/megapowers/skills/writing-agent-instructions/SKILL.md")
	for _, target := range []string{"references/skills.md", "references/repository-instructions.md"} {
		if !strings.Contains(document, "]("+target+")") {
			t.Errorf("entrypoint does not route to %s", target)
		}
		if _, err := os.Stat(filepath.Join(root, "plugins/megapowers/skills/writing-agent-instructions", target)); err != nil {
			t.Errorf("routed reference %s is unreachable: %v", target, err)
		}
	}
}

func TestNativeFanoutExamplesDispatchThenJoinEveryIdentity(t *testing.T) {
	root := repoRoot(t)
	document := read(t, root, "plugins/megapowers/skills/orchestrating/SKILL.md")
	referenceLink := regexp.MustCompile(`(?i)before dispatch[^.\n]*read\s+\[[^]]+\]\((references/[^)]+)\)`).FindStringSubmatch(document)
	if len(referenceLink) != 2 {
		t.Fatal("orchestrating entrypoint must directly require its native reference before dispatch")
	}
	reference := read(t, root, "plugins/megapowers/skills/orchestrating/"+referenceLink[1])
	for _, harness := range []struct {
		name, call string
	}{
		{"Codex", "spawn_agent"},
		{"Claude Code", "Agent"},
	} {
		t.Run(harness.name, func(t *testing.T) {
			block := harnessExample(t, reference, harness.name)
			lines := nonemptyLines(block)
			if len(lines) < 3 {
				t.Fatalf("example is too short: %q", block)
			}
			assign := regexp.MustCompile(`^([a-z][a-z0-9_]*)\s*=\s*` + regexp.QuoteMeta(harness.call) + `\b`)
			first, second := assign.FindStringSubmatch(lines[0]), assign.FindStringSubmatch(lines[1])
			if len(first) != 2 || len(second) != 2 || first[1] == second[1] {
				t.Fatalf("first two operations must dispatch distinct native identities: %q", lines[:2])
			}
			tail := strings.ToLower(strings.Join(lines[2:], " "))
			for _, identity := range []string{first[1], second[1]} {
				if !regexp.MustCompile(`\b` + regexp.QuoteMeta(identity) + `\b`).MatchString(tail) {
					t.Errorf("example does not join identity %s", identity)
				}
			}
			if !regexp.MustCompile(`\b(wait|join|collect|terminal)\b`).MatchString(tail) {
				t.Error("example does not join dispatched work")
			}
			if !strings.Contains(tail, "lead") {
				t.Error("example omits useful independent lead work after dispatch")
			}
		})
	}
}

func TestOpenSpecReferenceLoadsOnlyWhenDirectoryExists(t *testing.T) {
	root := repoRoot(t)
	reference := normalized(read(t, root, "plugins/megapowers/skills/design-and-plan/references/openspec.md"))
	for _, fact := range []string{"openspec/", "does not establish a repository convention", "do not run or add an openspec cli"} {
		if !strings.Contains(reference, fact) {
			t.Errorf("openspec reference missing %q", fact)
		}
	}
	entry := normalized(read(t, root, "plugins/megapowers/skills/design-and-plan/SKILL.md"))
	if !regexp.MustCompile(`openspec/[^.]*(exists|present)[^.]*references/openspec\.md|references/openspec\.md[^.]*openspec/[^.]*(exists|present)`).MatchString(entry) {
		t.Error("design-and-plan must load the OpenSpec reference only when an openspec/ directory exists")
	}
}

func harnessExample(t *testing.T, document, harness string) string {
	t.Helper()
	pattern := regexp.MustCompile(`(?ms)^` + regexp.QuoteMeta(harness) + `:\s*\n` + "```" + `[^\n]*\n(.*?)\n` + "```" + `$`)
	match := pattern.FindStringSubmatch(document)
	if len(match) != 2 {
		t.Fatalf("missing fenced %s native fan-out example", harness)
	}
	return match[1]
}

func nonemptyLines(body string) []string {
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
