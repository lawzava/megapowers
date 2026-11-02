package maintain

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const fixtureSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128"><rect width="128" height="128" fill="#1F6FEB"/></svg>`

func fixtureManifest(version string, keywords string) string {
	return `{"name":"megapowers","version":"` + version + `","description":"d","author":{"name":"lawzava"},` +
		`"homepage":"https://github.com/lawzava/megapowers","repository":"https://github.com/lawzava/megapowers","license":"MIT",` +
		`"keywords":[` + keywords + `]}`
}

// newDirectorySource builds a minimal valid source repository for the
// directory edition, including content the build must drop.
func newDirectorySource(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	write := func(rel, content string, mode os.FileMode) {
		t.Helper()
		path := filepath.Join(repo, "plugins", "megapowers", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	write(".claude-plugin/plugin.json", fixtureManifest("1.2.3", `"skills","claude-code"`), 0o644)
	write(".claude-plugin/icon.svg", fixtureSVG, 0o644)
	write(".codex-plugin/plugin.json", fixtureManifest("1.2.3", `"skills","codex","tdd"`), 0o644)
	write("README.md", "# readme\n", 0o644)
	write("hooks/run-hook.cmd", "#!/bin/sh\n", 0o755)
	write("hooks/hooks.json", "{}", 0o644)
	write("output-styles/megapowers.md", "style\n", 0o644)
	write("skills/catalog.json", "{}", 0o644)
	write("skills/alpha/SKILL.md", "---\nname: alpha\ndescription: Use when testing alpha.\nmetadata:\n  short-description: Alpha short\n---\n\n# Alpha\n\nRead [the ref](references/ref.md) and run `scripts/tool.go`.\n", 0o644)
	write("skills/alpha/references/ref.md", "# Ref\n\nSee `scripts/tool.go` and [site](https://example.com/x).\n", 0o644)
	write("skills/alpha/scripts/tool.go", "package main\n\nfunc main() {}\n", 0o755)
	write("skills/alpha/scripts/tool_test.go", "package main\n", 0o644)
	write("skills/beta/SKILL.md", "---\nname: \"beta\"\ndescription: \"Use when beta: colon inside.\"\nmetadata:\n  short-description: Beta short\n---\n\nBody.\n", 0o644)
	write("skills/beta/agents/openai.yaml", "policy:\n  allow_implicit_invocation: false\n", 0o644)
	return repo
}

func buildFixture(t *testing.T) []dirFile {
	t.Helper()
	files, err := buildDirectoryFiles(newDirectorySource(t))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func fixturePackage(t *testing.T) dirPackage {
	t.Helper()
	return newDirPackage(buildFixture(t))
}

func (r dirReport) codes() []string {
	var out []string
	for _, e := range r.Errors {
		out = append(out, e.Code)
	}
	return out
}

func TestDirectoryBuildSelectsSkillsOnly(t *testing.T) {
	files := buildFixture(t)
	var got []string
	for _, f := range files {
		got = append(got, f.Path)
	}
	want := []string{
		".codex-plugin/plugin.json",
		"assets/icon.svg",
		"skills/alpha/SKILL.md",
		"skills/alpha/references/ref.md",
		"skills/alpha/scripts/tool.go",
		"skills/beta/SKILL.md",
		"skills/beta/agents/openai.yaml",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("built files:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	modes := map[string]fs.FileMode{}
	for _, f := range files {
		modes[f.Path] = f.Mode
	}
	if modes["skills/alpha/scripts/tool.go"] != 0o755 || modes["skills/alpha/SKILL.md"] != 0o644 {
		t.Fatalf("modes not normalized: %v", modes)
	}
}

func TestDirectoryBuildManifest(t *testing.T) {
	pkg := fixturePackage(t)
	var manifest map[string]any
	if err := json.Unmarshal(pkg[".codex-plugin/plugin.json"].Data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["name"] != "megapowers" || manifest["version"] != "1.2.3" || manifest["skills"] != "./skills/" {
		t.Fatalf("manifest identity wrong: %v", manifest)
	}
	for _, key := range []string{"hooks", "apps", "mcpServers", "outputStyles"} {
		if _, ok := manifest[key]; ok {
			t.Errorf("manifest has forbidden key %s", key)
		}
	}
	keywords := strings.Join(toStrings(manifest["keywords"]), ",")
	if keywords != "skills,tdd" {
		t.Errorf("keywords = %q, want provider names dropped", keywords)
	}
	ui := manifest["interface"].(map[string]any)
	if _, ok := ui["screenshots"]; ok {
		t.Error("interface has screenshots")
	}
	if ui["composerIcon"] != "./assets/icon.svg" || ui["logo"] != "./assets/icon.svg" {
		t.Errorf("icons wrong: %v %v", ui["composerIcon"], ui["logo"])
	}
	if got := ui["developerName"]; got != "lawzava" {
		t.Errorf("developerName = %v", got)
	}
}

func toStrings(v any) []string {
	var out []string
	for _, item := range v.([]any) {
		out = append(out, item.(string))
	}
	return out
}

func TestDirectoryBuildAddsAgentInterfaceToPackagedCopyOnly(t *testing.T) {
	repo := newDirectorySource(t)
	files, err := buildDirectoryFiles(repo)
	if err != nil {
		t.Fatal(err)
	}
	pkg := newDirPackage(files)
	yaml := string(pkg["skills/beta/agents/openai.yaml"].Data)
	for _, want := range []string{"interface:", `display_name: "Beta"`, `short_description: "Beta short"`, "allow_implicit_invocation: false"} {
		if !strings.Contains(yaml, want) {
			t.Errorf("packaged openai.yaml lacks %q:\n%s", want, yaml)
		}
	}
	source, err := os.ReadFile(filepath.Join(repo, "plugins/megapowers/skills/beta/agents/openai.yaml"))
	if err != nil || strings.Contains(string(source), "interface") {
		t.Fatalf("source file changed: %q %v", source, err)
	}
	if report := validateDirectoryPackage(pkg); len(report.Errors) != 0 {
		t.Fatalf("fixture package has errors: %v", report.Errors)
	}
}

func TestDirectoryBuildRejectsManifestVersionMismatch(t *testing.T) {
	repo := newDirectorySource(t)
	path := filepath.Join(repo, "plugins/megapowers/.claude-plugin/plugin.json")
	if err := os.WriteFile(path, []byte(fixtureManifest("9.9.9", `"x"`)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := buildDirectoryFiles(repo); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("err = %v, want version mismatch", err)
	}
}

func TestDirectoryZipIsDeterministic(t *testing.T) {
	repo := newDirectorySource(t)
	var sums []string
	var blobs [][]byte
	for i := 0; i < 2; i++ {
		files, err := buildDirectoryFiles(repo)
		if err != nil {
			t.Fatal(err)
		}
		blob, err := zipDirectoryFiles(files)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(blob)
		sums = append(sums, hex.EncodeToString(sum[:]))
		blobs = append(blobs, blob)
	}
	if sums[0] != sums[1] {
		t.Fatalf("zip bytes differ: %s vs %s", sums[0], sums[1])
	}
	reader, err := zip.NewReader(bytes.NewReader(blobs[0]), int64(len(blobs[0])))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range reader.File {
		names = append(names, f.Name)
		if strings.HasSuffix(f.Name, "/") {
			t.Errorf("directory entry %s", f.Name)
		}
		if f.Modified.Year() != 1980 || len(f.Extra) != 0 {
			t.Errorf("%s: modified=%v extra=%d", f.Name, f.Modified, len(f.Extra))
		}
		if f.Method != zip.Deflate {
			t.Errorf("%s: method %d", f.Name, f.Method)
		}
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("entries not sorted: %v", names)
	}
}

func TestDirectoryEditionCommandWritesTreeAndZip(t *testing.T) {
	repo := newDirectorySource(t)
	out := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer
	if code := runDirectoryEdition(context.Background(), repo, []string{"--out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	zipPath := filepath.Join(out, "megapowers-1.2.3.zip")
	blob, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(blob)
	for _, want := range []string{zipPath, hex.EncodeToString(sum[:])} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
		}
	}
	if _, err := os.Stat(filepath.Join(out, "megapowers", ".codex-plugin", "plugin.json")); err != nil {
		t.Fatalf("tree not written: %v", err)
	}
	// A second run into the same directory replaces the tree and keeps the bytes.
	stdout.Reset()
	if code := runDirectoryEdition(context.Background(), repo, []string{"--out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("rerun exit %d: %s", code, stderr.String())
	}
	again, _ := os.ReadFile(zipPath)
	if !bytes.Equal(again, blob) {
		t.Fatal("rerun changed zip bytes")
	}
	custom := filepath.Join(t.TempDir(), "custom.zip")
	if code := runDirectoryEdition(context.Background(), repo, []string{"--out", out, "--zip", custom}, &stdout, &stderr); code != 0 {
		t.Fatalf("custom zip exit %d: %s", code, stderr.String())
	}
	if _, err := os.Stat(custom); err != nil {
		t.Fatalf("custom zip missing: %v", err)
	}
}

func TestDirectoryEditionCheckModeLeavesNothingBehind(t *testing.T) {
	repo := newDirectorySource(t)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	var stdout, stderr bytes.Buffer
	if code := runDirectoryEdition(context.Background(), repo, []string{"--check"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	entries, _ := os.ReadDir(tmp)
	if len(entries) != 0 {
		t.Fatalf("check mode left %d entries in TMPDIR", len(entries))
	}
}

func TestDirectoryEditionCheckModeFailsOnInvalidPackage(t *testing.T) {
	repo := newDirectorySource(t)
	// A skill without front matter must fail the check with a named code.
	path := filepath.Join(repo, "plugins/megapowers/skills/beta/SKILL.md")
	if err := os.WriteFile(path, []byte("no front matter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runDirectoryEdition(context.Background(), repo, []string{"--check"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "skill_frontmatter_missing") {
		t.Fatalf("stderr lacks code:\n%s", stderr.String())
	}
}

func TestDirectoryEditionUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runDirectoryEdition(context.Background(), t.TempDir(), []string{"--bogus"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

type packageMutation struct {
	name   string
	code   string
	mutate func(p dirPackage)
}

func setManifest(p dirPackage, edit func(m map[string]any, ui map[string]any)) {
	var m map[string]any
	_ = json.Unmarshal(p[".codex-plugin/plugin.json"].Data, &m)
	ui := m["interface"].(map[string]any)
	edit(m, ui)
	data, _ := json.Marshal(m)
	p[".codex-plugin/plugin.json"] = dirFile{Path: ".codex-plugin/plugin.json", Data: data, Mode: 0o644}
}

func setFile(p dirPackage, path, content string) {
	p[path] = dirFile{Path: path, Data: []byte(content), Mode: 0o644}
}

func TestDirectoryValidatePackageRules(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	cases := []packageMutation{
		{"name too long", "plugin_name_too_long", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["name"] = long(65) })
		}},
		{"name format", "plugin_name_format", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["name"] = "-bad name" })
		}},
		{"version not semver", "plugin_version_not_semver", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["version"] = "1.2" })
		}},
		{"version missing", "plugin_version_missing", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { delete(m, "version") })
		}},
		{"description empty", "plugin_description_empty", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["description"] = "  " })
		}},
		{"description too long", "plugin_description_too_long", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["description"] = long(1025) })
		}},
		{"description control char", "plugin_description_character_unsupported", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["description"] = "a\u200bb" })
		}},
		{"author empty", "plugin_author_name_empty", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["author"] = map[string]any{"name": ""} })
		}},
		{"author too long", "plugin_author_name_too_long", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["author"] = map[string]any{"name": long(121)} })
		}},
		{"author missing", "plugin_developer_missing", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { delete(m, "author") })
		}},
		{"skills path", "plugin_skills_path_unsupported", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["skills"] = "./other/" })
		}},
		{"hooks key", "manifest_forbidden_field", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["hooks"] = "./hooks/hooks.json" })
		}},
		{"apps key", "app_configuration_excluded", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["apps"] = "./.app.json" })
		}},
		{"mcpServers key", "mcp_configuration_excluded", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["mcpServers"] = "./.mcp.json" })
		}},
		{"outputStyles key", "manifest_forbidden_field", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { m["outputStyles"] = "./output-styles/" })
		}},
		{"screenshots", "screenshot_configuration_excluded", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["screenshots"] = []string{"./a.png"} })
		}},
		{"display name long", "plugin_display_name_too_long", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["displayName"] = long(31) })
		}},
		{"display name multiline", "plugin_display_name_character_unsupported", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["displayName"] = "a\nb" })
		}},
		{"short description long", "plugin_short_description_too_long", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["shortDescription"] = long(31) })
		}},
		{"short description missing", "plugin_short_description_missing", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { delete(ui, "shortDescription") })
		}},
		{"long description empty", "plugin_long_description_empty", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["longDescription"] = "" })
		}},
		{"long description long", "plugin_long_description_too_long", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["longDescription"] = long(4001) })
		}},
		{"developer name long", "plugin_developer_name_too_long", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["developerName"] = long(81) })
		}},
		{"category unknown", "plugin_category_unknown", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["category"] = "Stuff" })
		}},
		{"too many capabilities", "plugin_capabilities_too_many", func(p dirPackage) {
			caps := make([]string, 21)
			for i := range caps {
				caps[i] = "c"
			}
			setManifest(p, func(m, ui map[string]any) { ui["capabilities"] = caps })
		}},
		{"capability empty", "plugin_capability_empty", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["capabilities"] = []string{"ok", " "} })
		}},
		{"capability long", "plugin_capability_too_long", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["capabilities"] = []string{long(121)} })
		}},
		{"capability multiline", "plugin_capability_character_unsupported", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["capabilities"] = []string{"a\nb"} })
		}},
		{"too many prompts", "plugin_default_prompt_too_many", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["defaultPrompt"] = []string{"a", "b", "c", "d"} })
		}},
		{"prompt long", "plugin_default_prompt_too_long", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["defaultPrompt"] = []string{long(129)} })
		}},
		{"prompt duplicate", "plugin_default_prompt_duplicate", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["defaultPrompt"] = []string{"Do  it", "do it"} })
		}},
		{"prompt multiline", "plugin_default_prompt_character_unsupported", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["defaultPrompt"] = []string{"a\nb"} })
		}},
		{"brand color format", "plugin_brand_color_format", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["brandColor"] = "#12" })
		}},
		{"brand color contrast", "plugin_brand_color_contrast", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["brandColor"] = "#FFFF00" })
		}},
		{"logo prefix", "branding_asset_path_missing_root_prefix", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["logo"] = "assets/icon.svg" })
		}},
		{"logo traversal", "declared_asset_path_unsafe", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["logo"] = "./../icon.svg" })
		}},
		{"logo missing file", "declared_asset_file_missing", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { ui["composerIcon"] = "./assets/none.svg" })
		}},
		{"logo missing field", "plugin_logo_path_missing", func(p dirPackage) {
			setManifest(p, func(m, ui map[string]any) { delete(ui, "logo") })
		}},
		{"logo format", "image_file_format_unsupported", func(p dirPackage) {
			setFile(p, "assets/icon.gif", "GIF89a")
			setManifest(p, func(m, ui map[string]any) { ui["logo"] = "./assets/icon.gif" })
		}},
		{"svg not square", "svg_dimensions_not_square", func(p dirPackage) {
			setFile(p, "assets/icon.svg", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 64"></svg>`)
		}},
		{"svg too small", "svg_dimensions_too_small", func(p dirPackage) {
			setFile(p, "assets/icon.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"></svg>`)
		}},
		{"svg units", "svg_dimensions_not_numeric", func(p dirPackage) {
			setFile(p, "assets/icon.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="128px" height="128px"></svg>`)
		}},
		{"svg missing dims", "svg_dimensions_missing", func(p dirPackage) {
			setFile(p, "assets/icon.svg", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
		}},
		{"svg root", "svg_root_element_invalid", func(p dirPackage) {
			setFile(p, "assets/icon.svg", `<html width="128" height="128"></html>`)
		}},
		{"svg malformed", "svg_xml_malformed", func(p dirPackage) {
			setFile(p, "assets/icon.svg", `<svg width="128"`)
		}},
		{"svg not positive", "svg_dimensions_not_positive", func(p dirPackage) {
			setFile(p, "assets/icon.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="-5" height="-5"></svg>`)
		}},
		{"hooks dir", "forbidden_path", func(p dirPackage) { setFile(p, "hooks/hooks.json", "{}") }},
		{"output styles dir", "forbidden_path", func(p dirPackage) { setFile(p, "output-styles/x.md", "x") }},
		{"claude plugin dir", "forbidden_path", func(p dirPackage) { setFile(p, ".claude-plugin/plugin.json", "{}") }},
		{"mcp json", "forbidden_path", func(p dirPackage) { setFile(p, ".mcp.json", "{}") }},
		{"app json", "forbidden_path", func(p dirPackage) { setFile(p, ".app.json", "{}") }},
		{"root sibling", "plugin_root_has_siblings", func(p dirPackage) { setFile(p, "README.md", "x") }},
		{"test file", "forbidden_test_file", func(p dirPackage) { setFile(p, "skills/alpha/scripts/x_test.go", "package main") }},
		{"testdata", "forbidden_test_file", func(p dirPackage) { setFile(p, "skills/alpha/testdata/x.txt", "x") }},
		{"file under skills", "skill_file_ignored", func(p dirPackage) { setFile(p, "skills/catalog.json", "{}") }},
		{"skill without SKILL.md", "skill_manifest_missing", func(p dirPackage) { setFile(p, "skills/gamma/notes.md", "x") }},
		{"nested SKILL.md", "skill_manifest_nested", func(p dirPackage) {
			setFile(p, "skills/alpha/inner/SKILL.md", "---\nname: inner\ndescription: d\n---\nbody\n")
		}},
		{"hidden skill dir", "skill_directory_hidden", func(p dirPackage) {
			setFile(p, "skills/.hidden/SKILL.md", "---\nname: hidden\ndescription: d\n---\nbody\n")
		}},
		{"skill symlink", "archive_member_type_unsupported", func(p dirPackage) {
			p["skills/alpha/link"] = dirFile{Path: "skills/alpha/link", Data: []byte("SKILL.md"), Mode: fs.ModeSymlink | 0o777}
		}},
		{"invalid utf8", "skill_manifest_invalid_utf8", func(p dirPackage) {
			p["skills/beta/SKILL.md"] = dirFile{Path: "skills/beta/SKILL.md", Data: []byte("---\nname: beta\ndescription: d\n---\n\xff\xfe\n"), Mode: 0o644}
		}},
		{"front matter missing", "skill_frontmatter_missing", func(p dirPackage) { setFile(p, "skills/beta/SKILL.md", "# no front matter\n") }},
		{"front matter unclosed", "skill_frontmatter_unclosed", func(p dirPackage) { setFile(p, "skills/beta/SKILL.md", "---\nname: beta\ndescription: d\n") }},
		{"front matter malformed", "skill_frontmatter_yaml_malformed", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: beta\nthis is not a mapping\n---\nbody\n")
		}},
		{"name missing", "skill_name_missing", func(p dirPackage) { setFile(p, "skills/beta/SKILL.md", "---\ndescription: d\n---\nbody\n") }},
		{"name empty", "skill_name_empty", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: \"\"\ndescription: d\n---\nbody\n")
		}},
		{"description missing", "skill_description_missing", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: beta\n---\nbody\n")
		}},
		{"description too long", "skill_description_too_long", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: beta\ndescription: "+long(1025)+"\n---\nbody\n")
		}},
		{"body empty", "skill_body_empty", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: beta\ndescription: d\n---\n\n  \n")
		}},
		{"duplicate name", "skill_identity_duplicate", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: alpha\ndescription: d\n---\nbody\n")
		}},
		{"identity too long", "skill_identity_too_long", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: "+long(60)+"\ndescription: d\n---\nbody\n")
		}},
		{"agent yaml no interface", "skill_agent_interface_missing", func(p dirPackage) {
			setFile(p, "skills/beta/agents/openai.yaml", "policy:\n  allow_implicit_invocation: false\n")
		}},
		{"agent yaml no display name", "skill_agent_display_name_missing", func(p dirPackage) {
			setFile(p, "skills/beta/agents/openai.yaml", "interface:\n  short_description: \"x\"\n")
		}},
		{"agent yaml empty short", "skill_agent_short_description_empty", func(p dirPackage) {
			setFile(p, "skills/beta/agents/openai.yaml", "interface:\n  display_name: \"x\"\n  short_description: \"\"\n")
		}},
		{"agent yaml bad policy", "skill_agent_allow_implicit_invocation_wrong_type", func(p dirPackage) {
			setFile(p, "skills/beta/agents/openai.yaml", "interface:\n  display_name: x\n  short_description: y\npolicy:\n  allow_implicit_invocation: maybe\n")
		}},
		{"agent yaml unknown policy", "skill_agent_allow_implicit_invocation_wrong_type", func(p dirPackage) {
			setFile(p, "skills/beta/agents/openai.yaml", "interface:\n  display_name: x\n  short_description: y\npolicy:\n  other: true\n")
		}},
		{"unresolved link", "reference_unresolved", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: beta\ndescription: d\n---\nSee [x](references/missing.md).\n")
		}},
		{"unresolved backtick", "reference_unresolved", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: beta\ndescription: d\n---\nRun `scripts/missing.go`.\n")
		}},
		{"unresolved in reference file", "reference_unresolved", func(p dirPackage) {
			setFile(p, "skills/alpha/references/ref.md", "See `assets/none.md`.\n")
		}},
		{"link to removed output styles", "reference_removed_directory", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: beta\ndescription: d\n---\nSee [style](../../output-styles/megapowers.md).\n")
		}},
		{"backtick to removed hooks", "reference_removed_directory", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: beta\ndescription: d\n---\nSee `hooks/hooks.json`.\n")
		}},
		{"link escapes package", "reference_unresolved", func(p dirPackage) {
			setFile(p, "skills/beta/SKILL.md", "---\nname: beta\ndescription: d\n---\nSee [x](../../../etc/passwd).\n")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pkg := fixturePackage(t)
			tc.mutate(pkg)
			report := validateDirectoryPackage(pkg)
			for _, code := range report.codes() {
				if code == tc.code {
					return
				}
			}
			t.Fatalf("want code %s, got %v", tc.code, report.Errors)
		})
	}
}

func TestDirectoryValidateAcceptsReferenceShapes(t *testing.T) {
	pkg := fixturePackage(t)
	setFile(pkg, "skills/alpha/SKILL.md", "---\nname: alpha\ndescription: d\n---\n"+
		"[a](references/ref.md#anchor) [b](./references/ref.md) [url](https://example.com/a) [mail](mailto:a@b.c) [frag](#top)\n"+
		"`references/ref.md` `scripts/tool.go` `./scripts/tool.go` `scripts/<name>.go` `~/x/y` `/abs/path` `$ROOT/scripts/x.go` `assets/` `agents/openai.yaml`\n"+
		"```sh\ngo run scripts/tool.go --help\n```\n")
	setFile(pkg, "skills/alpha/references/ref.md", "Back to [skill](../SKILL.md) and `scripts/tool.go`.\n")
	if report := validateDirectoryPackage(pkg); len(report.Errors) != 0 {
		t.Fatalf("valid references flagged: %v", report.Errors)
	}
}

func TestDirectoryValidateNotesRunHookMentions(t *testing.T) {
	pkg := fixturePackage(t)
	setFile(pkg, "skills/beta/SKILL.md", "---\nname: beta\ndescription: d\n---\n```sh\ntest -f \"<plugin root>/hooks/run-hook.cmd\"\n```\n")
	report := validateDirectoryPackage(pkg)
	if len(report.Errors) != 0 {
		t.Fatalf("run-hook placeholder must not be an error: %v", report.Errors)
	}
	var hook []dirIssue
	for _, n := range report.Notes {
		if n.Code == "reference_run_hook" {
			hook = append(hook, n)
		}
	}
	if len(hook) != 1 || hook[0].Path != "skills/beta/SKILL.md" || !strings.Contains(hook[0].Message, "hooks/run-hook.cmd") {
		t.Fatalf("notes = %v", report.Notes)
	}
}

func TestDirectoryValidateNotesMetadataIgnored(t *testing.T) {
	report := validateDirectoryPackage(fixturePackage(t))
	var found bool
	for _, n := range report.Notes {
		if n.Code == "skill_metadata_ignored" && strings.Contains(n.Message, "alpha") {
			found = true
		}
	}
	if !found {
		t.Fatalf("metadata note missing: %v", report.Notes)
	}
}

func TestDirectoryValidateBundleSize(t *testing.T) {
	pkg := fixturePackage(t)
	// Incompressible data: a deterministic pseudo-random stream.
	blob := make([]byte, 11<<20)
	state := uint32(1)
	for i := range blob {
		state = state*1664525 + 1013904223
		blob[i] = byte(state >> 24)
	}
	pkg["skills/alpha/assets/big.bin"] = dirFile{Path: "skills/alpha/assets/big.bin", Data: blob, Mode: 0o644}
	report := validateDirectoryPackage(pkg)
	for _, code := range report.codes() {
		if code == "skill_bundle_too_large" {
			return
		}
	}
	t.Fatalf("want skill_bundle_too_large, got %v", report.Errors)
}

func TestDirectoryValidateArchiveEntries(t *testing.T) {
	ok := func(name string) archiveEntry { return archiveEntry{Name: name, Size: 1, Mode: 0o644} }
	deep := strings.Repeat("a/", 20) + "f"
	cases := []struct {
		name    string
		code    string
		entries []archiveEntry
	}{
		{"empty", "archive_empty", nil},
		{"empty path", "archive_member_path_empty", []archiveEntry{ok("")}},
		{"outer whitespace", "archive_member_path_has_outer_whitespace", []archiveEntry{ok(" a/b")}},
		{"backslash", "archive_member_path_has_backslash", []archiveEntry{ok(`a\b`)}},
		{"absolute", "archive_member_path_absolute", []archiveEntry{ok("/a/b")}},
		{"empty segment", "archive_member_path_has_empty_segment", []archiveEntry{ok("a//b")}},
		{"parent segment", "archive_member_path_has_parent_segment", []archiveEntry{ok("a/../b")}},
		{"too deep", "archive_member_path_too_deep", []archiveEntry{ok(deep)}},
		{"too long", "archive_member_path_too_long", []archiveEntry{ok(strings.Repeat("a", 300))}},
		{"case collision", "archive_member_path_normalization_collision", []archiveEntry{ok("A.md"), ok("a.md")}},
		{"duplicate", "archive_member_path_duplicate", []archiveEntry{ok("a.md"), ok("a.md")}},
		{"symlink", "archive_member_type_unsupported", []archiveEntry{{Name: "l", Mode: fs.ModeSymlink | 0o777}}},
		{"member too large", "archive_member_too_large", []archiveEntry{{Name: "a", Size: 101 << 20, Mode: 0o644}}},
		{"type conflict", "archive_member_path_type_conflict", []archiveEntry{ok("a"), ok("a/b")}},
		{"uncompressed too large", "archive_uncompressed_too_large", []archiveEntry{
			{Name: "a", Size: 99 << 20, Mode: 0o644}, {Name: "b", Size: 99 << 20, Mode: 0o644}, {Name: "c", Size: 99 << 20, Mode: 0o644},
			{Name: "d", Size: 99 << 20, Mode: 0o644}, {Name: "e", Size: 99 << 20, Mode: 0o644}, {Name: "f", Size: 99 << 20, Mode: 0o644},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var codes []string
			for _, issue := range validateArchiveEntries(tc.entries, 1<<20) {
				codes = append(codes, issue.Code)
				if issue.Code == tc.code {
					return
				}
			}
			t.Fatalf("want %s, got %v", tc.code, codes)
		})
	}
	t.Run("too many entries", func(t *testing.T) {
		var entries []archiveEntry
		for i := 0; i < 5001; i++ {
			entries = append(entries, ok("f"+strconv.Itoa(i)))
		}
		for _, issue := range validateArchiveEntries(entries, 1<<20) {
			if issue.Code == "archive_too_many_entries" {
				return
			}
		}
		t.Fatal("want archive_too_many_entries")
	})
	t.Run("compressed too large", func(t *testing.T) {
		for _, issue := range validateArchiveEntries([]archiveEntry{ok("a")}, 101<<20) {
			if issue.Code == "archive_too_large" {
				return
			}
		}
		t.Fatal("want archive_too_large")
	})
	t.Run("clean", func(t *testing.T) {
		if issues := validateArchiveEntries([]archiveEntry{ok(".codex-plugin/plugin.json"), ok("skills/a/SKILL.md")}, 1<<20); len(issues) != 0 {
			t.Fatalf("clean archive flagged: %v", issues)
		}
	})
}

func TestDirectoryZipRoundTripValidates(t *testing.T) {
	files := buildFixture(t)
	blob, err := zipDirectoryFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "p.zip")
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := validateDirectoryZip(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("round trip errors: %v", report.Errors)
	}
}

func TestDirectoryZipWithWrapperDirectoryFails(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("megapowers/.codex-plugin/plugin.json")
	_, _ = f.Write([]byte("{}"))
	_ = w.Close()
	path := filepath.Join(t.TempDir(), "p.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := validateDirectoryZip(path)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(report.codes(), "plugin_manifest_missing") {
		t.Fatalf("errors = %v", report.Errors)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestContrastAgainstWhite(t *testing.T) {
	if r, ok := contrastRatio("#1F6FEB", "#FFFFFF"); !ok || r < 2 {
		t.Fatalf("brand blue ratio = %v", r)
	}
	if r, _ := contrastRatio("#FFFFFF", "#FFFFFF"); r != 1 {
		t.Fatalf("white on white = %v", r)
	}
	if r, _ := contrastRatio("#000000", "#FFFFFF"); r < 20.9 || r > 21.1 {
		t.Fatalf("black on white = %v", r)
	}
	if r, _ := contrastRatio("#FFFF00", "#FFFFFF"); r >= 2 {
		t.Fatalf("yellow ratio = %v", r)
	}
}

func TestParseFrontMatterScalars(t *testing.T) {
	fm, err := parseSkillFrontMatter("---\nname: a\ndescription: \"x: \\\"y\\\"\"\nwhen_to_use: 'it''s'\nmetadata:\n  short-description: s\nflag: true\nfolded: >\n  one\n  two\n---\nbody")
	if err != nil {
		t.Fatal(err)
	}
	if fm.Scalars["name"] != "a" || fm.Scalars["description"] != `x: "y"` || fm.Scalars["when_to_use"] != "it's" {
		t.Fatalf("scalars = %v", fm.Scalars)
	}
	if fm.Scalars["folded"] != "one two" || fm.Body != "body" || fm.Nested["metadata"]["short-description"] != "s" {
		t.Fatalf("parsed = %+v", fm)
	}
}

func realRepoRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("MEGAPOWERS_ROOT")
	if root == "" {
		root = filepath.Join("..", "..", "..")
	}
	if _, err := os.Stat(filepath.Join(root, "plugins", "megapowers", ".codex-plugin", "plugin.json")); err != nil {
		t.Skipf("no repository root at %s: %v", root, err)
	}
	return root
}

// The real plugin must build and validate. Failures here name the skill text
// or asset the directory edition still needs fixed.
func TestRealPluginBuildsAndValidatesAsDirectoryEdition(t *testing.T) {
	root := realRepoRoot(t)
	files, err := buildDirectoryFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := zipDirectoryFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "real.zip")
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := validateDirectoryZip(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, note := range report.Notes {
		t.Logf("note: %s", note)
	}
	for _, issue := range report.Errors {
		t.Errorf("error: %s", issue)
	}
}

// OpenAI's instruction-following review asks that skills state the priority
// of explicit user instructions. The packaged copy carries one line; the
// source skills stay unchanged.
func TestDirectoryBuildAppendsUserPrecedenceLineToPackagedSkills(t *testing.T) {
	repo := newDirectorySource(t)
	files, err := buildDirectoryFiles(repo)
	if err != nil {
		t.Fatal(err)
	}
	const line = "Explicit user instructions take precedence over this skill's guidance."
	count := 0
	for _, file := range files {
		if !strings.HasPrefix(file.Path, "skills/") || !strings.HasSuffix(file.Path, "/SKILL.md") {
			continue
		}
		count++
		if got := strings.Count(string(file.Data), line); got != 1 {
			t.Errorf("%s has %d precedence lines, want 1", file.Path, got)
		}
		source, err := os.ReadFile(filepath.Join(repo, "plugins/megapowers", filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), line) {
			t.Errorf("source %s was changed", file.Path)
		}
	}
	if count == 0 {
		t.Fatal("no SKILL.md in package")
	}
}
