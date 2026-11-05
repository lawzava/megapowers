package maintain

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The directory edition is a skills-only build of plugins/megapowers for the
// OpenAI Plugins Directory. It omits the Claude output style, the Claude
// manifest, the repository skill inventory, and the
// plugin README, and it rewrites the listing manifest. See
// developers.openai.com/plugins/deploy/submission-errors for the rules the
// validator enforces.

const (
	directoryPluginDir = "plugins/megapowers"
	directoryIconSrc   = ".claude-plugin/icon.svg"
	directoryIconDst   = "assets/icon.svg"
)

const directoryDescription = "Workflow skills for coding agents: planning, test-first implementation, " +
	"systematic debugging, verification before completion, safe handling of real-world side effects, " +
	"independent cross-provider review, parallel orchestration, evidence research, and instruction authoring. " +
	"Skills only: the package ships no hooks, output style, or MCP server."

const directoryLongDescription = `megapowers is a set of task-level workflow skills for coding agents. A skill loads when the task matches its description and gives the agent a short, checkable procedure.

What it covers:
- Specify non-trivial changes and compare design tradeoffs before building.
- Implement behavior changes test-first.
- Diagnose failures of unknown cause before editing.
- Verify an outcome before claiming it is done, committing, or handing off.
- Gate deploys, messages, migrations, and other real-world effects behind explicit authority and read-back.
- Split independent work across parallel agents and coordinate the results.
- Run adversarial review of high-stakes changes with a provider other than the author.
- Research decisions that need evidence beyond the repository.
- Keep long unattended runs moving across context resets.
- Interview you to stress-test a plan (grill-me, started by name only).
- Draft and edit prose, write agent instructions and skills, set up MCP servers, and check or upgrade this plugin.

Who it is for: developers who work with coding agents in a repository and want consistent practice for planning, testing, debugging, review, and release work.

Limits: this edition contains skills only. It has no hooks, no startup output style, and no destructive-command guard. Some skills bundle small Go programs in their scripts folders; running them needs a local shell and the Go toolchain. Review and orchestration skills use whichever agents or providers your environment already provides. Skills are guidance text for the agent; they do not enforce behavior or replace your own review.`

var directoryCapabilities = []string{
	"Specify changes and compare design tradeoffs",
	"Implement behavior changes test-first",
	"Diagnose failures by root cause before editing",
	"Verify outcomes before claiming completion",
	"Gate real-world side effects behind authority checks",
	"Orchestrate parallel agent lanes",
	"Run independent cross-provider review",
	"Research decisions with classified evidence",
	"Keep long unattended runs moving across resets",
	"Stress-test a plan through a structured interview",
	"Draft and edit prose",
	"Write and audit agent instructions and skills",
	"Set up and verify MCP servers",
	"Check and upgrade the plugin install",
}

var directoryDefaultPrompts = []string{
	"Plan this change, then implement it test-first.",
	"Find the root cause of this failing test before fixing it.",
	"Verify this work is done before I merge it.",
}

// Field order below is the output order.
type directoryManifest struct {
	Name        string             `json:"name"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	Author      directoryAuthor    `json:"author"`
	Homepage    string             `json:"homepage,omitempty"`
	Repository  string             `json:"repository,omitempty"`
	License     string             `json:"license,omitempty"`
	Keywords    []string           `json:"keywords,omitempty"`
	Skills      string             `json:"skills"`
	Interface   directoryInterface `json:"interface"`
}

type directoryAuthor struct {
	Name string `json:"name"`
}

type directoryInterface struct {
	DisplayName      string   `json:"displayName"`
	ShortDescription string   `json:"shortDescription"`
	LongDescription  string   `json:"longDescription"`
	DeveloperName    string   `json:"developerName"`
	Category         string   `json:"category"`
	Capabilities     []string `json:"capabilities"`
	DefaultPrompt    []string `json:"defaultPrompt"`
	BrandColor       string   `json:"brandColor"`
	ComposerIcon     string   `json:"composerIcon"`
	Logo             string   `json:"logo"`
	WebsiteURL       string   `json:"websiteURL"`
}

type sourceManifest struct {
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Homepage   string   `json:"homepage"`
	Repository string   `json:"repository"`
	License    string   `json:"license"`
	Keywords   []string `json:"keywords"`
}

func readSourceManifest(file string) (sourceManifest, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return sourceManifest{}, err
	}
	var m sourceManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return sourceManifest{}, fmt.Errorf("%s: %w", file, err)
	}
	return m, nil
}

// buildDirectoryFiles assembles the directory edition from the repository at
// root. It returns files sorted by package path with normalized modes.
func buildDirectoryFiles(root string) ([]dirFile, error) {
	src := filepath.Join(root, filepath.FromSlash(directoryPluginDir))
	codex, err := readSourceManifest(filepath.Join(src, ".codex-plugin", "plugin.json"))
	if err != nil {
		return nil, err
	}
	claude, err := readSourceManifest(filepath.Join(src, ".claude-plugin", "plugin.json"))
	if err != nil {
		return nil, err
	}
	if codex.Version == "" || codex.Version != claude.Version {
		return nil, fmt.Errorf("plugin manifest version mismatch: codex=%q claude=%q", codex.Version, claude.Version)
	}
	if codex.Name == "" || codex.Name != claude.Name {
		return nil, fmt.Errorf("plugin manifest name mismatch: codex=%q claude=%q", codex.Name, claude.Name)
	}
	icon, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(directoryIconSrc)))
	if err != nil {
		return nil, err
	}

	var files []dirFile
	skillsRoot := filepath.Join(src, "skills")
	err = filepath.WalkDir(skillsRoot, func(p string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(skillsRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.Contains(rel, "/") || strings.HasSuffix(rel, "_test.go") {
			return nil // files directly under skills/ (catalog.json) and test files stay out
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("skills/%s is not a regular file", rel)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o755
		}
		files = append(files, dirFile{Path: "skills/" + rel, Data: data, Mode: mode})
		return nil
	})
	if err != nil {
		return nil, err
	}
	pkg := newDirPackage(files)
	for _, name := range pkg.paths() {
		parts := strings.Split(name, "/")
		if len(parts) == 4 && parts[2] == "agents" && parts[3] == "openai.yaml" {
			data, err := withAgentInterface(pkg, parts[1], pkg[name].Data)
			if err != nil {
				return nil, err
			}
			file := pkg[name]
			file.Data = data
			pkg[name] = file
		}
	}
	for _, name := range pkg.paths() {
		if parts := strings.Split(name, "/"); len(parts) == 3 && parts[2] == "SKILL.md" {
			file := pkg[name]
			file.Data = withUserPrecedence(file.Data)
			pkg[name] = file
		}
	}
	files = files[:0]
	for _, name := range pkg.paths() {
		files = append(files, pkg[name])
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no skills found under %s/skills", directoryPluginDir)
	}

	manifest := directoryManifest{
		Name:        codex.Name,
		Version:     codex.Version,
		Description: directoryDescription,
		Author:      directoryAuthor{Name: "lawzava"},
		Homepage:    codex.Homepage,
		Repository:  codex.Repository,
		License:     codex.License,
		Skills:      "./skills/",
		Interface: directoryInterface{
			DisplayName:      codex.Name,
			ShortDescription: "Agent workflow skills",
			LongDescription:  directoryLongDescription,
			DeveloperName:    "lawzava",
			Category:         "Developer Tools",
			Capabilities:     directoryCapabilities,
			DefaultPrompt:    directoryDefaultPrompts,
			BrandColor:       "#1F6FEB",
			ComposerIcon:     "./" + directoryIconDst,
			Logo:             "./" + directoryIconDst,
			WebsiteURL:       "https://github.com/lawzava/megapowers",
		},
	}
	for _, keyword := range codex.Keywords {
		if keyword != "claude-code" && keyword != "codex" {
			manifest.Keywords = append(manifest.Keywords, keyword)
		}
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return nil, err
	}
	files = append(files,
		dirFile{Path: ".codex-plugin/plugin.json", Data: buf.Bytes(), Mode: 0o644},
		dirFile{Path: directoryIconDst, Data: icon, Mode: 0o644},
	)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// withAgentInterface returns data unchanged when it already declares an
// interface. The directory requires one whenever agents/openai.yaml ships
// (skill_agent_interface_missing), so a file with only a policy gets an
// interface block in the packaged copy. The source file is not modified.
func withAgentInterface(pkg dirPackage, skill string, data []byte) ([]byte, error) {
	root, err := parseYAMLSubset(string(data))
	if err != nil || root.kind != yamlMap {
		return data, nil // the validator reports the malformed file
	}
	if _, ok := root.get("interface"); ok {
		return data, nil
	}
	skillFile, ok := pkg["skills/"+skill+"/SKILL.md"]
	if !ok {
		return nil, fmt.Errorf("skills/%s has agents/openai.yaml but no SKILL.md", skill)
	}
	fm, err := parseSkillFrontMatter(string(skillFile.Data))
	if err != nil {
		return data, nil // the validator reports the malformed SKILL.md
	}
	name := collapseSpace(fm.Scalars["name"])
	if name == "" {
		name = skill
	}
	short := collapseSpace(fm.Nested["metadata"]["short-description"])
	if short == "" {
		short = collapseSpace(fm.Scalars["description"])
	}
	if short == "" {
		return nil, fmt.Errorf("skills/%s has no short-description or description for its agent interface", skill)
	}
	quote := func(s string) string {
		out, _ := json.Marshal(s)
		return string(out)
	}
	var b strings.Builder
	b.WriteString("interface:\n")
	fmt.Fprintf(&b, "  display_name: %s\n", quote(displayNameFor(name)))
	fmt.Fprintf(&b, "  short_description: %s\n", quote(short))
	b.Write(data)
	return []byte(b.String()), nil
}

func displayNameFor(skill string) string {
	words := strings.FieldsFunc(skill, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// zipDirectoryFiles writes a byte-for-byte reproducible ZIP: sorted entries,
// a fixed DOS timestamp, no extra fields, no directory entries, normalized
// modes, and a fixed deflate level. Output is stable for one Go release;
// the standard library may change deflate output between releases.
func zipDirectoryFiles(files []dirFile) ([]byte, error) {
	sorted := append([]dirFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	writer.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})
	for _, f := range sorted {
		header := &zip.FileHeader{Name: f.Path, Method: zip.Deflate}
		header.ModifiedDate = 1<<5 | 1 // 1980-01-01
		header.ModifiedTime = 0
		header.SetMode(f.Mode)
		w, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.Data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeDirectoryTree(dir string, files []dirFile) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for _, f := range files {
		target := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, f.Data, f.Mode); err != nil {
			return err
		}
		if err := os.Chmod(target, f.Mode); err != nil {
			return err
		}
	}
	return nil
}

type directoryOptions struct {
	out   string
	zip   string
	check bool
}

func parseDirectoryArgs(args []string) (directoryOptions, error) {
	var opts directoryOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := func(name string) (string, error) {
			if v, ok := strings.CutPrefix(arg, name+"="); ok {
				return v, nil
			}
			if i+1 >= len(args) || args[i+1] == "" {
				return "", fmt.Errorf("%s needs a value", name)
			}
			i++
			return args[i], nil
		}
		var err error
		switch {
		case arg == "--check":
			opts.check = true
		case arg == "--out" || strings.HasPrefix(arg, "--out="):
			opts.out, err = value("--out")
		case arg == "--zip" || strings.HasPrefix(arg, "--zip="):
			opts.zip, err = value("--zip")
		default:
			err = fmt.Errorf("unknown argument %s", arg)
		}
		if err != nil {
			return opts, err
		}
	}
	if opts.check && (opts.out != "" || opts.zip != "") {
		return opts, fmt.Errorf("--check builds in a temporary directory; it takes no --out or --zip")
	}
	return opts, nil
}

// callerPath resolves a relative path against the shell directory that
// scripts/validate.sh recorded, since the maintainer runs from the repo root.
func callerPath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if caller := os.Getenv("MEGAPOWERS_CALLER_CWD"); caller != "" {
		return filepath.Join(caller, p)
	}
	return p
}

func runDirectoryEdition(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	const usage = "usage: maintainer directory-edition [--out DIR] [--zip FILE] [--check]"
	opts, err := parseDirectoryArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "directory-edition: %v\n%s\n", err, usage)
		return 2
	}
	files, err := buildDirectoryFiles(root)
	if err != nil {
		fmt.Fprintf(stderr, "directory-edition: %v\n", err)
		return 2
	}
	blob, err := zipDirectoryFiles(files)
	if err != nil {
		fmt.Fprintf(stderr, "directory-edition: %v\n", err)
		return 2
	}
	manifestVersion := versionFromFiles(files)

	// Validate the exact bytes before anything touches the disk.
	report := validateDirectoryZipBytes(blob)
	for _, note := range report.Notes {
		fmt.Fprintf(stdout, "note: %s\n", note)
	}
	if len(report.Errors) != 0 {
		for _, issue := range report.Errors {
			fmt.Fprintf(stderr, "error: %s\n", issue)
		}
		fmt.Fprintf(stderr, "directory-edition: %d validation error(s)\n", len(report.Errors))
		return 1
	}
	sum := sha256.Sum256(blob)
	if opts.check {
		fmt.Fprintf(stdout, "directory-edition: check ok (%d files, %d bytes, sha256 %x)\n", len(files), len(blob), sum)
		return 0
	}

	// --zip alone writes only that file; the default tree directory is
	// touched only when no --zip is given or --out is explicit.
	var zipPath, treeDir string
	writeTree := opts.zip == "" || opts.out != ""
	if writeTree {
		out := opts.out
		if out == "" {
			base := os.Getenv("TMPDIR")
			if base == "" {
				base = os.TempDir()
			}
			out = filepath.Join(base, "megapowers-directory")
		}
		out = callerPath(out)
		treeDir = filepath.Join(out, "megapowers")
		zipPath = filepath.Join(out, "megapowers-"+manifestVersion+".zip")
	}
	if opts.zip != "" {
		zipPath = callerPath(opts.zip)
	}
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		fmt.Fprintf(stderr, "directory-edition: %v\n", err)
		return 2
	}
	if writeTree {
		if err := writeDirectoryTree(treeDir, files); err != nil {
			fmt.Fprintf(stderr, "directory-edition: %v\n", err)
			return 2
		}
	}
	if err := os.WriteFile(zipPath, blob, 0o644); err != nil {
		fmt.Fprintf(stderr, "directory-edition: %v\n", err)
		return 2
	}
	if writeTree {
		fmt.Fprintf(stdout, "tree: %s\n", treeDir)
	}
	fmt.Fprintf(stdout, "zip: %s\nsize: %d bytes\nsha256: %x\n", zipPath, len(blob), sum)
	return 0
}

func versionFromFiles(files []dirFile) string {
	for _, f := range files {
		if f.Path == ".codex-plugin/plugin.json" {
			var m sourceManifest
			if json.Unmarshal(f.Data, &m) == nil {
				return m.Version
			}
		}
	}
	return "unknown"
}

// checkDirectoryEdition builds the directory edition in memory and validates
// the ZIP bytes without writing anything. validate.go calls it.
func checkDirectoryEdition(root string) (dirReport, error) {
	files, err := buildDirectoryFiles(root)
	if err != nil {
		return dirReport{}, err
	}
	blob, err := zipDirectoryFiles(files)
	if err != nil {
		return dirReport{}, err
	}
	return validateDirectoryZipBytes(blob), nil
}

// userPrecedenceLine makes the priority of explicit user instructions clear,
// as OpenAI's instruction-following review asks. Only the packaged copy gets it.
const userPrecedenceLine = "Explicit user instructions take precedence over this skill's guidance."

func withUserPrecedence(data []byte) []byte {
	if strings.Contains(string(data), userPrecedenceLine) {
		return data
	}
	trimmed := strings.TrimRight(string(data), "\n")
	return []byte(trimmed + "\n\n" + userPrecedenceLine + "\n")
}
