package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate contract package")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func requireContains(t *testing.T, body, needle, label string) {
	t.Helper()
	if !strings.Contains(body, needle) {
		t.Errorf("%s: missing %q", label, needle)
	}
}

func requireAbsent(t *testing.T, body, needle, label string) {
	t.Helper()
	if strings.Contains(strings.ToLower(body), strings.ToLower(needle)) {
		t.Errorf("%s: retained %q", label, needle)
	}
}

func run(t *testing.T, root string, env []string, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if err == nil {
		return output.String(), 0
	}
	var exit *exec.ExitError
	if !errorsAs(err, &exit) {
		t.Fatalf("run %s: %v", name, err)
	}
	return output.String(), exit.ExitCode()
}

func errorsAs(err error, target any) bool {
	switch typed := target.(type) {
	case **exec.ExitError:
		value, ok := err.(*exec.ExitError)
		if ok {
			*typed = value
		}
		return ok
	default:
		return false
	}
}

func TestCIContract(t *testing.T) {
	root := repoRoot(t)
	ci := read(t, root, ".github/workflows/ci.yml")
	freshness := read(t, root, ".github/workflows/freshness.yml")
	for _, pin := range []string{
		"actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
		"actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16",
		"actions/setup-node@249970729cb0ef3589644e2896645e5dc5ba9c38",
		"actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f",
	} {
		requireContains(t, ci, pin, "CI action pin")
	}
	requireContains(t, freshness, "persist-credentials: false", "freshness checkout")
	mutable := regexp.MustCompile(`uses:\s+[^#\s]+@v[0-9]+`)
	if mutable.MatchString(ci) || mutable.MatchString(freshness) {
		t.Error("workflow retains a mutable major action reference")
	}
	if strings.Count(ci, "uses: actions/checkout@") != strings.Count(ci, "persist-credentials: false") {
		t.Error("every CI checkout must disable persisted credentials")
	}
	for _, marker := range []string{"@anthropic-ai/claude-code@2.1.257", "go run ./scripts/cmd/maintainer validate", "go run ./evals/cmd/evaltool run-all --json results.jsonl", "go run evals/score.go --strict results.jsonl", "if: always()", "path: results.jsonl"} {
		requireContains(t, ci, marker, "CI contract")
	}
}

func TestDocsContract(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"README.md", "docs/install.md", "docs/orchestration.md", "docs/harness-support.md",
		"docs/advanced/independent-review.md", "docs/advanced/evals.md", "docs/advanced/verification-maps.md",
		"plugins/megapowers/README.md", "evals/README.md", "evals/RESULTS.md",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("missing %s", rel)
		}
	}
	for _, rel := range []string{"docs/setup.md", "docs/agent-install.md", "docs/session-metrics.md", "templates"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			t.Errorf("obsolete surface remains: %s", rel)
		}
	}
	active := []string{"README.md", "SECURITY.md", "CONTRIBUTING.md", "docs/install.md", "docs/orchestration.md", "docs/harness-support.md", "docs/advanced/independent-review.md", "docs/advanced/evals.md", "docs/advanced/verification-maps.md", "plugins/megapowers/README.md", "evals/README.md"}
	for _, rel := range active {
		body := read(t, root, rel)
		for _, removed := range []string{"OpenCode", "models.toml", "delegates.toml", "model catalog", "mega-orchestration", "mega-guardrails"} {
			requireAbsent(t, body, removed, rel)
		}
	}
	readme := read(t, root, "README.md")
	for _, marker := range []string{"exactly one plugin", "Claude Code and Codex", "installed-plugin A/B", "skills/catalog.json", "trusted Codex startup hook"} {
		requireContains(t, readme, marker, "README")
	}
	evalsReadme := read(t, root, "evals/README.md")
	for _, marker := range []string{"four different questions", "report-only"} {
		requireContains(t, evalsReadme, marker, "eval README")
	}
	if !regexp.MustCompile(`(?i)Claude.*enforce|enforce.*Claude`).MatchString(evalsReadme) {
		t.Error("eval README omits Claude enforcement policy")
	}
	if !regexp.MustCompile(`(?i)Codex.*report-only|report-only.*Codex`).MatchString(evalsReadme) {
		t.Error("eval README omits Codex report-only policy")
	}
	assertMarkdownLinks(t, root)
}

func assertMarkdownLinks(t *testing.T, root string) {
	t.Helper()
	tracked, code := run(t, root, nil, "git", "ls-files", "*.md")
	if code != 0 {
		t.Fatal("cannot list tracked Markdown")
	}
	links := regexp.MustCompile(`\]\(([^ )]+[.]md)(?:#[^)]*)?\)`)
	for _, rel := range strings.Fields(tracked) {
		body := read(t, root, rel)
		for _, match := range links.FindAllStringSubmatch(body, -1) {
			target := match[1]
			if strings.Contains(target, "://") {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, filepath.Dir(rel), filepath.FromSlash(target))); err != nil {
				t.Errorf("%s links to missing %s", rel, target)
			}
		}
	}
}

func TestFreshnessContract(t *testing.T) {
	root := repoRoot(t)
	output, code := run(t, root, nil, "go", "test", "./scripts/cmd/check-freshness")
	if code != 0 {
		t.Fatalf("freshness contracts failed:\n%s", output)
	}
}

func TestNativeFirstContract(t *testing.T) {
	root := repoRoot(t)
	names := catalogNames(t, root)
	for _, marketplace := range []struct {
		path, source string
	}{
		{".claude-plugin/marketplace.json", "./plugins/megapowers"},
		{".agents/plugins/marketplace.json", "./plugins/megapowers"},
	} {
		var document map[string]any
		if err := json.Unmarshal([]byte(read(t, root, marketplace.path)), &document); err != nil {
			t.Fatal(err)
		}
		plugins, _ := document["plugins"].([]any)
		if len(plugins) != 1 || plugins[0].(map[string]any)["name"] != "megapowers" {
			t.Errorf("%s must expose only megapowers", marketplace.path)
		}
	}
	actualSkills, err := skillDirectories(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(actualSkills, "\n") != strings.Join(names, "\n") {
		t.Errorf("skill inventory differs from catalog\ncatalog=%v\ndirs=%v", names, actualSkills)
	}
	for _, name := range names {
		link := filepath.Join(root, ".agents/skills", name)
		target, err := os.Readlink(link)
		if err != nil || target != "../../plugins/megapowers/skills/"+name {
			t.Errorf("unexpected skill link %s -> %s (%v)", name, target, err)
		}
	}
	hooks := read(t, root, "plugins/megapowers/hooks/hooks.json")
	for _, marker := range []string{"run-hook.cmd session-start", "run-hook.cmd deny-destructive", `"Bash|PowerShell"`} {
		requireContains(t, hooks, marker, "hook manifest")
	}
	entries, _ := os.ReadDir(filepath.Join(root, "plugins/megapowers/hooks"))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".go") || name == "hooks.json" || name == "run-hook.cmd" {
			continue
		}
		t.Errorf("unsupported hook artifact: %s", name)
	}
	for _, rel := range []string{"plugins/mega-orchestration", "plugins/mega-guardrails", "plugins/megapowers/opencode", "plugins/megapowers/agents", "plugins/megapowers/models.toml", "templates", "scripts/check-enforcement.go", "scripts/lib/validate-helpers.sh"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			t.Errorf("removed surface remains: %s", rel)
		}
	}
}

func TestOutputStyleContract(t *testing.T) {
	root := repoRoot(t)
	style := read(t, root, "plugins/megapowers/output-styles/megapowers.md")
	for _, marker := range []string{
		"name: Megapowers", "description: Direct, concise technical communication for the operator",
		"keep-coding-instructions: true", "force-for-plugin: false", "ASD-STE100-inspired principles",
		"Do not claim formal ASD-STE100 compliance.", "Default to 100 prose words or fewer.",
		"Do not exceed 250 prose words", "Do not use em dashes.", "`humanizing-prose`",
		"named source, direct observation, or explicit uncertainty", "actor, mechanism, scope, condition, or measurement",
	} {
		requireContains(t, style, marker, "output style")
	}
	hooks := read(t, root, "plugins/megapowers/hooks/hooks.json")
	requireContains(t, hooks, "run-hook.cmd session-start", "session-start hook")
	requireContains(t, read(t, root, "docs/harness-support.md"), "MEGAPOWERS_OUTPUT_STYLE=off", "Codex output style opt-out")
}

func TestSkillContracts(t *testing.T) {
	root := repoRoot(t)
	names := catalogNames(t, root)
	if len(names) == 0 {
		t.Fatal("catalog is empty")
	}
	totalWords := 0
	for _, name := range names {
		rel := "plugins/megapowers/skills/" + name + "/SKILL.md"
		body := read(t, root, rel)
		frontmatter, skillBody, err := splitFrontmatter(body)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		fields := parseFrontmatter(frontmatter)
		if fields["name"] != name {
			t.Errorf("%s name = %q", rel, fields["name"])
		}
		for _, field := range []string{"description", "when_to_use", "metadata.short-description"} {
			if strings.TrimSpace(fields[field]) == "" {
				t.Errorf("%s missing %s", rel, field)
			}
		}
		if len(fields["description"]) > 1536 {
			t.Errorf("%s description exceeds 1536 characters", rel)
		}
		words := len(strings.Fields(skillBody))
		totalWords += words
		if words > 400 {
			t.Errorf("%s has %d body words, limit 400", rel, words)
		}
		for _, link := range regexp.MustCompile(`\[[^]]+\]\(([^)]+)\)`).FindAllStringSubmatch(body, -1) {
			target := strings.Split(link[1], "#")[0]
			if strings.Contains(target, "://") || target == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, filepath.Dir(rel), filepath.FromSlash(target))); err != nil {
				t.Errorf("%s links to missing %s", rel, target)
			}
		}
	}
	if limit := len(names) * 330; totalWords > limit {
		t.Errorf("primary skill guidance has %d words, derived limit %d", totalWords, limit)
	}
	loaded := read(t, root, "AGENTS.md")
	for _, name := range names {
		loaded += read(t, root, "plugins/megapowers/skills/"+name+"/SKILL.md")
	}
	for _, removed := range []string{"using-megapowers", "verification-before-completion", "writing-plans", "superpowers", "obra/superpowers", "derived from"} {
		requireAbsent(t, loaded, removed, "agent-loaded guidance")
	}
}

func catalogNames(t *testing.T, root string) []string {
	t.Helper()
	var catalog struct {
		SchemaVersion string `json:"schema_version"`
		Skills        []struct {
			Name, Status string
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(read(t, root, "plugins/megapowers/skills/catalog.json")), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.SchemaVersion != "1" {
		t.Errorf("unexpected catalog schema %q", catalog.SchemaVersion)
	}
	seen := map[string]bool{}
	var names []string
	for _, skill := range catalog.Skills {
		if skill.Name == "" || seen[skill.Name] {
			t.Errorf("invalid or duplicate catalog skill %q", skill.Name)
		}
		if skill.Status != "stable" && skill.Status != "experimental" {
			t.Errorf("invalid status for %s: %s", skill.Name, skill.Status)
		}
		seen[skill.Name] = true
		names = append(names, skill.Name)
	}
	sort.Strings(names)
	return names
}

func skillDirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "plugins/megapowers/skills"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := os.Stat(filepath.Join(root, "plugins/megapowers/skills", entry.Name(), "SKILL.md")); err == nil {
				names = append(names, entry.Name())
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

func splitFrontmatter(document string) (string, string, error) {
	if !strings.HasPrefix(document, "---\n") {
		return "", "", fmt.Errorf("frontmatter does not start on line 1")
	}
	end := strings.Index(document[4:], "\n---\n")
	if end < 0 {
		return "", "", fmt.Errorf("frontmatter is not closed")
	}
	end += 4
	return document[4:end], document[end+5:], nil
}

func parseFrontmatter(frontmatter string) map[string]string {
	out := map[string]string{}
	prefix := ""
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], strings.TrimSpace(parts[1])
		if indent == 0 {
			prefix = key
		} else if prefix != "" {
			key = prefix + "." + key
		}
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		out[key] = value
	}
	return out
}

func TestTestInventory(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range []string{"scripts/tests", "evals/tests", "evals/studies/tests"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".test.sh") {
				continue
			}
			rel := filepath.ToSlash(filepath.Join(dir, entry.Name()))
			body := read(t, root, rel)
			if len(strings.Split(strings.TrimSpace(body), "\n")) > 10 || !strings.Contains(body, "go test") {
				t.Errorf("%s is not a thin Go test launcher", rel)
			}
		}
	}
}

func TestValidationContract(t *testing.T) {
	root := repoRoot(t)
	shim := read(t, root, "scripts/validate.sh")
	if len(strings.Split(strings.TrimSpace(shim), "\n")) > 14 || !strings.Contains(shim, "scripts/cmd/maintainer") {
		t.Error("validate.sh is not a thin Go launcher")
	}
	implementation := read(t, root, "scripts/internal/maintain/validate.go")
	for _, marker := range []string{"deadline(ctx", "plugins/megapowers", ".claude-plugin/marketplace.json", ".agents/plugins/marketplace.json", "go\", \"test\", \"./..."} {
		requireContains(t, implementation, marker, "validation implementation")
	}
	for _, removed := range []string{"OpenCode", "models.toml", "delegates.toml", "skill-router", "copied-agent"} {
		requireAbsent(t, implementation, removed, "validation implementation")
	}
}

func TestVerificationMapContract(t *testing.T) {
	root := repoRoot(t)
	var document struct {
		SchemaVersion string `json:"schema_version"`
		Application   string `json:"application"`
		Status        string `json:"status"`
		Journeys      []struct {
			ID            string   `json:"id"`
			Surface       string   `json:"surface"`
			Harness       string   `json:"harness"`
			IsolatedState string   `json:"isolated_state"`
			Cleanup       string   `json:"cleanup"`
			Doctor        []string `json:"doctor"`
			Runner        []string `json:"runner"`
			Evidence      []string `json:"evidence"`
		} `json:"journeys"`
	}
	body := read(t, root, "verification/megapowers.json")
	if err := json.Unmarshal([]byte(body), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != "1" || document.Application != "megapowers" || document.Status != "pilot" || len(document.Journeys) < 3 {
		t.Error("verification map header or journey inventory is invalid")
	}
	seen := map[string]bool{}
	for _, journey := range document.Journeys {
		if journey.ID == "" || seen[journey.ID] || journey.Surface == "" || journey.IsolatedState == "" || journey.Cleanup == "" || len(journey.Doctor) == 0 || len(journey.Runner) == 0 || len(journey.Evidence) == 0 {
			t.Errorf("invalid journey %+v", journey)
		}
		seen[journey.ID] = true
		if strings.Contains(journey.ID, "exact-tag-install") {
			joined := strings.Join(journey.Runner, " ")
			for _, marker := range []string{"evals/studies/install-smoke/run-smoke.sh", "--out", "--harnesses", "--source", "--ref", "--version"} {
				requireContains(t, joined, marker, journey.ID)
			}
		}
	}
	if regexp.MustCompile(`(?i)(/home/|credentials|auth[.]json|secret|customer)`).MatchString(body) {
		t.Error("verification map contains private or credential state")
	}
}

func TestMemoryHygieneContract(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	tool := "./plugins/megapowers/skills/memory-hygiene/scripts/memory-audit.go"
	write := func(name, records string) string {
		t.Helper()
		path := filepath.Join(tmp, name)
		body := `{"schema_version":"1","records":` + records + "}\n"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	valid := write("valid.json", `[{"id":"direct-rule","claim":"The user directly restricted release authority.","origin":"memory/user-rules.md#release-authority","evidence":"direct-statement","decision":"retain","source":"session:abc:turn:7","observed_at":"2026-08-20","scope":"repository writes"}]`)
	before := readBytes(t, valid)
	output, code := run(t, root, nil, "go", "run", tool, "--input", valid, "--as-of", "2026-08-26")
	if code != 0 || !strings.Contains(output, "1 records: 1 retain") {
		t.Errorf("valid audit failed (%d): %s", code, output)
	}
	if !bytes.Equal(before, readBytes(t, valid)) {
		t.Error("audit mutated its input")
	}
	inferred := write("inferred.json", `[{"id":"soft","claim":"Probably terse.","origin":"memory/profile.md#style","evidence":"inferred","decision":"retain","source":"session:abc","observed_at":"2026-08-20","scope":"global"}]`)
	output, code = run(t, root, nil, "go", "run", tool, "--input", inferred, "--as-of", "2026-08-26")
	if code == 0 || !strings.Contains(strings.ToLower(output), "cannot retain inferred evidence") {
		t.Errorf("inferred retained evidence did not fail closed: %s", output)
	}
	unknown := filepath.Join(tmp, "unknown.json")
	os.WriteFile(unknown, []byte(`{"schema_version":"1","records":[],"write_provider_memory":true}`), 0o600)
	output, code = run(t, root, nil, "go", "run", tool, "--input", unknown, "--as-of", "2026-08-26")
	if code == 0 || !strings.Contains(strings.ToLower(output), "unknown field") {
		t.Errorf("unknown field did not fail closed: %s", output)
	}
	tests := []struct {
		name, records, want string
		succeed             bool
	}{
		{"expired retained fact", `[{"id":"old-limit","claim":"Old limit.","origin":"memory/facts.md#old","evidence":"source-backed","decision":"retain","source":"https://example.invalid/official","observed_at":"2026-07-01","verified_at":"2026-07-01","scope":"service plan","volatile":true,"max_age_days":7}]`, "expired 49 days", false},
		{"expired fact marked revalidate", `[{"id":"old-limit","claim":"Recheck limit.","origin":"memory/facts.md#old","evidence":"source-backed","decision":"revalidate","source":"https://example.invalid/official","observed_at":"2026-07-01","verified_at":"2026-07-01","scope":"service plan","volatile":true,"max_age_days":7}]`, "1 revalidate", true},
		{"retained fact without source", `[{"id":"missing","claim":"Missing source.","origin":"memory/facts.md#missing","evidence":"direct-observation","decision":"retain","observed_at":"2026-08-20","scope":"environment"}]`, "requires source", false},
		{"future evidence", `[{"id":"future","claim":"Future.","origin":"memory/facts.md#future","evidence":"direct-observation","decision":"retain","source":"command:status","observed_at":"2026-08-27","scope":"checkout"}]`, "after as-of", false},
		{"duplicate IDs", `[{"id":"same","claim":"One.","origin":"a","evidence":"unknown","decision":"remove"},{"id":"same","claim":"Two.","origin":"b","evidence":"unknown","decision":"remove"}]`, "duplicate id", false},
		{"unbounded refresh", `[{"id":"unbounded","claim":"Window.","origin":"memory/facts.md#window","evidence":"source-backed","decision":"revalidate","source":"https://example.invalid/official","observed_at":"2026-08-20","scope":"service plan","volatile":true,"max_age_days":36501}]`, "cannot exceed 36500", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := write(strings.ReplaceAll(test.name, " ", "-")+".json", test.records)
			output, code := run(t, root, nil, "go", "run", tool, "--input", path, "--as-of", "2026-08-19")
			if test.succeed && code != 0 {
				t.Errorf("wanted success: %s", output)
			}
			if !test.succeed && code == 0 {
				t.Errorf("wanted failure: %s", output)
			}
			if !strings.Contains(strings.ToLower(output), strings.ToLower(test.want)) {
				t.Errorf("missing %q: %s", test.want, output)
			}
		})
	}
	missingRecords := filepath.Join(tmp, "missing-records.json")
	os.WriteFile(missingRecords, []byte(`{"schema_version":"1"}`), 0o600)
	if output, code := run(t, root, nil, "go", "run", tool, "--input", missingRecords, "--as-of", "2026-08-26"); code == 0 || !strings.Contains(output, "records array is required") {
		t.Errorf("missing records accepted: %s", output)
	}
	secretValue := "abcdefghijklmnop"
	secret := write("secret.json", `[{"id":"secret","claim":"api_key=`+secretValue+`","origin":"memory/facts.md#secret","evidence":"unknown","decision":"remove"}]`)
	if output, code := run(t, root, nil, "go", "run", tool, "--input", secret, "--as-of", "2026-08-26"); code == 0 || !strings.Contains(output, "secret-like content") || strings.Contains(output, secretValue) {
		t.Errorf("secret rejection leaked or failed: %s", output)
	}
	link := filepath.Join(tmp, "link.json")
	if err := os.Symlink(valid, link); err != nil {
		t.Fatal(err)
	}
	if output, code := run(t, root, nil, "go", "run", tool, "--input", link, "--as-of", "2026-08-26"); code == 0 || !strings.Contains(strings.ToLower(output), "symlink") {
		t.Errorf("symlink input accepted: %s", output)
	}
	if output, code := run(t, root, nil, "go", "run", tool, "--input", valid); code == 0 || !strings.Contains(output, "as-of is required") {
		t.Errorf("missing as-of accepted: %s", output)
	}
	if output, code := run(t, root, nil, "go", "run", tool, "--as-of", "2026-08-26"); code == 0 || !strings.Contains(output, "input is required") {
		t.Errorf("missing input accepted: %s", output)
	}
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
