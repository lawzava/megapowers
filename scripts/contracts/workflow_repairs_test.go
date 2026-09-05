package contracts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

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

func TestWritingInstructionHelperPolicyPrecedesBothRoutes(t *testing.T) {
	root := repoRoot(t)
	document := read(t, root, "plugins/megapowers/skills/writing-agent-instructions/SKILL.md")
	helper := regexp.MustCompile(`(?i)every new deterministic (helper|tool)[^.]*\bGo\b`).FindStringIndex(document)
	if helper == nil {
		t.Fatal("shared entrypoint does not require new deterministic helpers to be Go")
	}
	branch := strings.Index(document, "For a skill,")
	if branch < 0 || helper[0] > branch {
		t.Fatal("deterministic helper policy is hidden in a format-specific branch")
	}
	for _, target := range []string{"references/skills.md", "references/repository-instructions.md"} {
		if !strings.Contains(document, "]("+target+")") {
			t.Errorf("entrypoint does not route to %s", target)
		}
		if _, err := os.Stat(filepath.Join(root, "plugins/megapowers/skills/writing-agent-instructions", target)); err != nil {
			t.Errorf("routed reference %s is unreachable: %v", target, err)
		}
	}
}

func TestPlanningContractHandlesExistingAndAbsentSpecificationSystems(t *testing.T) {
	root := repoRoot(t)
	guidance := strings.ToLower(read(t, root, "plugins/megapowers/skills/design-and-plan/SKILL.md"))

	t.Run("existing baseline and active change", func(t *testing.T) {
		fixture := t.TempDir()
		writeFixtureFile(t, fixture, "openspec/specs/retries.md", "# RETRY-1\n")
		writeFixtureFile(t, fixture, "openspec/changes/retry-jitter/tasks.md", "- [ ] RETRY-1\n")
		baseline, active := fixtureSpecState(t, fixture)
		if !baseline || !active {
			t.Fatal("fixture does not exercise both existing specification surfaces")
		}
		for _, concept := range []*regexp.Regexp{
			regexp.MustCompile(`(?i)(detect|inspect|search).*(repository|files|instructions).*(baseline|active change)|(baseline|active change).*(detect|inspect|search)`),
			regexp.MustCompile(`(?i)preserv[^.]*requirement id`),
			regexp.MustCompile(`(?i)map[^.]*scenario[^.]*task[^.]*evidence`),
		} {
			if !concept.MatchString(guidance) {
				t.Errorf("existing-system branch misses %s", concept)
			}
		}
	})

	t.Run("no specification convention", func(t *testing.T) {
		fixture := t.TempDir()
		writeFixtureFile(t, fixture, "go.mod", "module example.test/no-spec\n\ngo 1.25\n")
		baseline, active := fixtureSpecState(t, fixture)
		if baseline || active {
			t.Fatal("fixture unexpectedly contains a specification system")
		}
		for _, concept := range []*regexp.Regexp{
			regexp.MustCompile(`(?i)(without|no) (an |a )?(existing )?(specification|spec) (system|convention)`),
			regexp.MustCompile(`(?i)inline[^.]*requirement`),
			regexp.MustCompile(`(?i)(do not|no)[^.]*new director`),
			regexp.MustCompile(`(?i)(do not|no)[^.]*\b(cli|node)\b`),
		} {
			if !concept.MatchString(guidance) {
				t.Errorf("absent-system branch misses %s", concept)
			}
		}
	})
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

	lower := strings.ToLower(strings.Join(strings.Fields(document), " "))
	for _, concept := range []*regexp.Regexp{
		regexp.MustCompile(`(no.?op|no work).*(inline|do not delegate)|(inline|do not delegate).*(no.?op|no work)`),
		regexp.MustCompile(`sequential.*inline|inline.*sequential`),
		regexp.MustCompile(`explicit[^.]*authoriz[^.]*native agent`),
		regexp.MustCompile(`operator.selected[^.]*access[^.]*before[^.]*(native|rank)`),
		regexp.MustCompile(`one to three direct (children|agents)|1.?3 direct (children|agents)`),
		regexp.MustCompile(`fresh[^.]*bounded[^.]*context`),
		regexp.MustCompile(`fail[^.]*report|report[^.]*fail`),
		regexp.MustCompile(`cancel[^.]*confirm|confirm[^.]*cancel`),
		regexp.MustCompile(`ownership[^.]*disjoint|disjoint[^.]*ownership`),
	} {
		if !concept.MatchString(lower) {
			t.Errorf("orchestration lifecycle misses %s", concept)
		}
	}
	if strings.Contains(lower, "without dispatch is a contract violation") {
		t.Error("skill still forces delegation whenever a lane looks eligible")
	}
}

func TestFinishContractSupportsNoopWithoutWeakeningOpenWork(t *testing.T) {
	root := repoRoot(t)
	document := strings.ToLower(strings.Join(strings.Fields(read(t, root, "plugins/megapowers/skills/verify-and-finish/SKILL.md")), " "))
	for _, concept := range []*regexp.Regexp{
		regexp.MustCompile(`(start|first)[^.]*fresh[^.]*(state|evidence|oracle)`),
		regexp.MustCompile(`verified no.?op[^.]*(stop|return)|stop[^.]*verified no.?op`),
		regexp.MustCompile(`pending review[^.]*(open|unfinished)|open[^.]*pending review`),
		regexp.MustCompile(`local[^.]*(cannot|does not)[^.]*external`),
	} {
		if !concept.MatchString(document) {
			t.Errorf("finish contract misses %s", concept)
		}
	}
}

func writeFixtureFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureSpecState(t *testing.T, root string) (baseline, active bool) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		baseline = baseline || strings.Contains(rel, "/specs/")
		active = active || strings.Contains(rel, "/changes/")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return baseline, active
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
