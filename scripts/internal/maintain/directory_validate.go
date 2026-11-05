package maintain

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"math"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits come from OpenAI's plugin submission error reference
// (developers.openai.com/plugins/deploy/submission-errors, read 2026-11-02).
// Where the reference states no number (path length, skill bundle size) the
// value here is a conservative local bound, marked below.
const (
	dirMaxArchiveCompressed   = 100 << 20
	dirMaxArchiveUncompressed = 512 << 20
	dirMaxArchiveMember       = 100 << 20
	dirMaxArchiveEntries      = 5000
	dirMaxPathSegments        = 20
	dirMaxPathBytes           = 255      // assumption: the reference names no number
	dirMaxSkillBundle         = 10 << 20 // assumption: the reference reports the limit at upload time
	dirMaxImageBytes          = 5 << 20
)

var directoryCategories = map[string]bool{
	"Productivity": true, "Creativity": true, "Developer Tools": true,
	"Business & Operations": true, "Data & Analytics": true, "Communication": true,
	"Education & Research": true, "Security": true, "Finance": true, "Healthcare": true,
	"Travel": true, "Entertainment": true, "Other": true,
}

var (
	pluginNamePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	semverPattern      = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)
	hexColorPattern    = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	numericPattern     = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)
	mentionPattern     = regexp.MustCompile(`(^|\s)@\w`)
	markdownLinkTarget = regexp.MustCompile(`\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	inlineCodePattern  = regexp.MustCompile("`([^`\n]+)`")
	lineSuffixPattern  = regexp.MustCompile(`:\d+(:\d+)?$`)
)

type dirFile struct {
	Path string
	Data []byte
	Mode fs.FileMode
}

// dirPackage maps slash-separated package paths to their content.
type dirPackage map[string]dirFile

func newDirPackage(files []dirFile) dirPackage {
	out := make(dirPackage, len(files))
	for _, f := range files {
		out[f.Path] = f
	}
	return out
}

func (p dirPackage) paths() []string {
	out := make([]string, 0, len(p))
	for name := range p {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// hasDir reports whether any package file lives under dir.
func (p dirPackage) hasDir(dir string) bool {
	prefix := strings.TrimSuffix(dir, "/") + "/"
	for name := range p {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

type dirIssue struct {
	Code    string
	Path    string
	Message string
}

func (i dirIssue) String() string {
	out := i.Code
	if i.Path != "" {
		out += " " + i.Path
	}
	if i.Message != "" {
		out += ": " + i.Message
	}
	return out
}

type dirReport struct {
	Errors []dirIssue
	Notes  []dirIssue
}

type archiveEntry struct {
	Name           string
	Size           uint64
	CompressedSize uint64
	Method         uint16 // ZIP compression method: stored or deflate
	Mode           fs.FileMode
}

type dirChecker struct {
	pkg            dirPackage
	report         dirReport
	metadataSkills []string
}

func (c *dirChecker) fail(code, p, format string, args ...any) {
	c.report.Errors = append(c.report.Errors, dirIssue{code, p, fmt.Sprintf(format, args...)})
}

func (c *dirChecker) note(code, p, format string, args ...any) {
	c.report.Notes = append(c.report.Notes, dirIssue{code, p, fmt.Sprintf(format, args...)})
}

// validateArchiveEntries applies the ZIP structure rules to raw entry names.
func validateArchiveEntries(entries []archiveEntry, compressedTotal int64) []dirIssue {
	var issues []dirIssue
	add := func(code, name, format string, args ...any) {
		issues = append(issues, dirIssue{code, name, fmt.Sprintf(format, args...)})
	}
	if len(entries) == 0 {
		add("archive_empty", "", "archive has no entries")
	}
	if compressedTotal > dirMaxArchiveCompressed {
		add("archive_too_large", "", "compressed ZIP is %d bytes, limit %d", compressedTotal, dirMaxArchiveCompressed)
	}
	if len(entries) > dirMaxArchiveEntries {
		add("archive_too_many_entries", "", "%d entries, limit %d", len(entries), dirMaxArchiveEntries)
	}
	var total uint64
	type seenEntry struct {
		name  string
		canon string
		isDir bool
	}
	seen := map[string]seenEntry{}
	filePaths := map[string]bool{}
	allPaths := map[string]bool{}
	spellings := map[string]string{}
	type spellingClash struct{ folded, name, prefix, other string }
	var clashes []spellingClash
	for _, e := range entries {
		name := e.Name
		isDir := strings.HasSuffix(name, "/")
		clean := strings.TrimSuffix(name, "/")
		switch {
		case name == "":
			add("archive_member_path_empty", name, "empty entry path")
			continue
		case strings.TrimSpace(name) != name:
			add("archive_member_path_has_outer_whitespace", name, "entry path has leading or trailing whitespace")
		}
		if strings.Contains(name, `\`) {
			add("archive_member_path_has_backslash", name, "entry path uses a backslash")
		}
		if strings.HasPrefix(name, "/") || (len(name) >= 2 && name[1] == ':' && unicode.IsLetter(rune(name[0]))) {
			add("archive_member_path_absolute", name, "entry path is not relative")
		}
		segments := strings.Split(strings.TrimPrefix(clean, "/"), "/")
		for _, seg := range segments {
			if seg == "" {
				add("archive_member_path_has_empty_segment", name, "entry path has an empty segment")
				break
			}
		}
		for _, seg := range segments {
			if seg == ".." {
				add("archive_member_path_has_parent_segment", name, "entry path has a .. segment")
				break
			}
		}
		for _, seg := range segments {
			if seg == "." {
				add("archive_member_path_has_dot_segment", name, "entry path has a . segment")
				break
			}
		}
		if len(segments) > dirMaxPathSegments {
			add("archive_member_path_too_deep", name, "%d segments, limit %d", len(segments), dirMaxPathSegments)
		}
		if len(name) > dirMaxPathBytes {
			add("archive_member_path_too_long", name, "%d bytes, limit %d", len(name), dirMaxPathBytes)
		}
		wantType := fs.FileMode(0)
		if isDir {
			wantType = fs.ModeDir
		}
		if e.Mode.Type() != wantType {
			add("archive_member_type_unsupported", name, "entry is not a regular file or directory")
		}
		// A stored directory has no body; an empty deflate stream is two bytes.
		if isDir && (e.Size != 0 || e.CompressedSize > 2 || (e.Method == zip.Store && e.CompressedSize != 0)) {
			add("archive_directory_entry_not_empty", name, "directory entry declares %d bytes (%d compressed)", e.Size, e.CompressedSize)
		}
		if e.Size > dirMaxArchiveMember {
			add("archive_member_too_large", name, "%d bytes, limit %d", e.Size, dirMaxArchiveMember)
		}
		total += e.Size
		// Collisions are judged on the canonical path, so a/./b and a/b clash.
		canon := path.Clean(strings.TrimPrefix(clean, "/"))
		key := strings.ToLower(canon)
		if prev, ok := seen[key]; ok {
			switch {
			case prev.name == name:
				add("archive_member_path_duplicate", name, "duplicate entry path")
			case prev.isDir != isDir:
				add("archive_member_path_type_conflict", name, "%s is both a file and a directory", canon)
			default:
				add("archive_member_path_normalization_collision", name, "collides with %s after normalization", prev.name)
			}
		}
		seen[key] = seenEntry{name: name, canon: canon, isDir: isDir}
		canonSegments := strings.Split(canon, "/")
		if !isDir {
			filePaths[key] = true
		}
		for i := range canonSegments {
			prefix := strings.Join(canonSegments[:i+1], "/")
			folded := strings.ToLower(prefix)
			if i < len(canonSegments)-1 || isDir {
				// Directories that differ only by case clash after extraction.
				if other, ok := spellings[folded]; ok && other != prefix {
					clashes = append(clashes, spellingClash{folded, name, prefix, other})
				}
			}
			if _, ok := spellings[folded]; !ok {
				spellings[folded] = prefix
			}
			allPaths[folded] = true
		}
	}
	for _, clash := range clashes {
		// A path that is also a file is reported once, as a type conflict.
		if !filePaths[clash.folded] {
			add("archive_member_path_normalization_collision", clash.name, "directory %s collides with %s after case folding", clash.prefix, clash.other)
		}
	}
	if total > dirMaxArchiveUncompressed {
		add("archive_uncompressed_too_large", "", "%d bytes extracted, limit %d", total, dirMaxArchiveUncompressed)
	}
	for _, name := range sortedKeys(filePaths) {
		prefix := name + "/"
		for other := range allPaths {
			if strings.HasPrefix(other, prefix) {
				add("archive_member_path_type_conflict", name, "file path also holds %s", other)
				break
			}
		}
	}
	return issues
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// validateDirectoryZip validates the archive structure and, when that is
// clean, the package inside it.
func validateDirectoryZip(zipPath string) (dirReport, error) {
	info, err := os.Stat(zipPath)
	if err != nil {
		return dirReport{}, err
	}
	if info.Size() > dirMaxArchiveCompressed {
		return dirReport{Errors: []dirIssue{{"archive_too_large", zipPath, fmt.Sprintf("compressed ZIP is %d bytes, limit %d", info.Size(), dirMaxArchiveCompressed)}}}, nil
	}
	blob, err := os.ReadFile(zipPath)
	if err != nil {
		return dirReport{}, err
	}
	return validateDirectoryZipBytes(blob), nil
}

// validateDirectoryZipBytes validates an in-memory ZIP. Header sizes are
// claims: each member is read with a bound derived from its header, and any
// disagreement between header and real bytes is an error.
func validateDirectoryZipBytes(blob []byte) dirReport {
	reader, err := zip.NewReader(bytes.NewReader(blob), int64(len(blob)))
	if err != nil {
		return dirReport{Errors: []dirIssue{{"archive_format_not_zip", "", err.Error()}}}
	}
	var entries []archiveEntry
	for _, f := range reader.File {
		entries = append(entries, archiveEntry{Name: f.Name, Size: f.UncompressedSize64, CompressedSize: f.CompressedSize64, Method: f.Method, Mode: f.Mode()})
	}
	var report dirReport
	report.Errors = validateArchiveEntries(entries, int64(len(blob)))
	if len(report.Errors) != 0 {
		return report
	}
	pkg := dirPackage{}
	var total uint64
	for _, f := range reader.File {
		if strings.HasSuffix(f.Name, "/") {
			// Directory entries are checked like files for layout and mode.
			if body, err := f.Open(); err != nil {
				report.Errors = append(report.Errors, dirIssue{"archive_directory_entry_not_empty", f.Name, err.Error()})
				continue
			} else {
				n, readErr := io.CopyN(io.Discard, body, 1)
				body.Close()
				if n > 0 || (readErr != nil && readErr != io.EOF) {
					report.Errors = append(report.Errors, dirIssue{"archive_directory_entry_not_empty", f.Name, "directory entry holds data"})
					continue
				}
			}
			pkg[strings.TrimSuffix(f.Name, "/")] = dirFile{Path: strings.TrimSuffix(f.Name, "/"), Mode: fs.ModeDir | 0o755}
			continue
		}
		body, err := f.Open()
		if err != nil {
			report.Errors = append(report.Errors, dirIssue{"archive_member_unreadable", f.Name, err.Error()})
			continue
		}
		data, err := io.ReadAll(io.LimitReader(body, int64(f.UncompressedSize64)+1))
		body.Close()
		switch {
		case err != nil:
			report.Errors = append(report.Errors, dirIssue{"archive_member_unreadable", f.Name, err.Error()})
			continue
		case uint64(len(data)) != f.UncompressedSize64:
			report.Errors = append(report.Errors, dirIssue{"archive_member_size_mismatch", f.Name, fmt.Sprintf("header says %d bytes, member holds at least %d", f.UncompressedSize64, len(data))})
			continue
		}
		total += uint64(len(data))
		if total > dirMaxArchiveUncompressed {
			report.Errors = append(report.Errors, dirIssue{"archive_uncompressed_too_large", "", fmt.Sprintf("extracted bytes exceed %d", dirMaxArchiveUncompressed)})
			return report
		}
		pkg[f.Name] = dirFile{Path: f.Name, Data: data, Mode: f.Mode()}
	}
	if len(report.Errors) != 0 {
		return report
	}
	inner := validateDirectoryPackage(pkg)
	report.Errors = append(report.Errors, inner.Errors...)
	report.Notes = append(report.Notes, inner.Notes...)
	return report
}

// validateDirectoryPackage checks a built package tree against the OpenAI
// plugin directory rules for a skills-only plugin.
func validateDirectoryPackage(pkg dirPackage) dirReport {
	c := &dirChecker{pkg: pkg}
	c.checkLayout()
	pluginName := c.checkManifest()
	c.checkSkills(pluginName)
	c.checkReferences()
	if len(c.metadataSkills) > 0 {
		c.note("skill_metadata_ignored", "skills", "front matter metadata is ignored by the importer (%d skills: %s); interface settings belong in agents/openai.yaml", len(c.metadataSkills), strings.Join(c.metadataSkills, ", "))
	}
	sort.SliceStable(c.report.Errors, func(i, j int) bool { return c.report.Errors[i].Path < c.report.Errors[j].Path })
	return c.report
}

func (c *dirChecker) checkLayout() {
	allowedRoots := map[string]bool{".codex-plugin": true, "skills": true, "assets": true}
	reported := map[string]bool{}
	for _, name := range c.pkg.paths() {
		file := c.pkg[name]
		isDir := file.Mode.Type() == fs.ModeDir
		if file.Mode.Type() != 0 && !isDir {
			c.fail("archive_member_type_unsupported", name, "entry is not a regular file or directory")
		}
		segments := strings.Split(name, "/")
		root := segments[0]
		base := segments[len(segments)-1]
		parents := segments[:len(segments)-1]
		if isDir {
			parents = segments
		}
		if !allowedRoots[root] && !reported[root] {
			reported[root] = true
			switch {
			case root == "hooks" || root == "output-styles" || root == ".claude-plugin" || root == ".mcp.json" || root == ".app.json":
				c.fail("forbidden_path", root, "the directory edition ships skills only")
			case len(segments) == 1 && !isDir:
				c.fail("plugin_root_has_siblings", root, "only .codex-plugin, skills, and assets may sit at the package root")
			default:
				c.fail("unexpected_root_entry", root, "only .codex-plugin, skills, and assets may sit at the package root")
			}
		}
		if base == "hooks.json" && root != "hooks" {
			c.fail("forbidden_path", name, "hooks.json is not allowed")
		}
		if strings.HasSuffix(base, "_test.go") || containsSegment(parents, "testdata") {
			c.fail("forbidden_test_file", name, "test files and testdata do not ship in the package")
		}
	}
}

func containsSegment(segments []string, want string) bool {
	for _, s := range segments {
		if s == want {
			return true
		}
	}
	return false
}

func supportedText(s string, multiline bool) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		switch {
		case r == '\n' && multiline:
		case unicode.IsControl(r), r == 0x2028, r == 0x2029, unicode.Is(unicode.Cf, r):
			return false
		}
	}
	return true
}

type fieldState int

const (
	fieldMissing fieldState = iota
	fieldOK
	fieldWrongType
)

func jsonString(m map[string]any, key string) (string, fieldState) {
	raw, ok := m[key]
	if !ok {
		return "", fieldMissing
	}
	s, ok := raw.(string)
	if !ok {
		return "", fieldWrongType
	}
	return s, fieldOK
}

// text validates one required text field and returns its value.
func (c *dirChecker) text(file string, m map[string]any, key, prefix, missingSuffix string, max int, multiline bool) (string, bool) {
	value, state := jsonString(m, key)
	switch {
	case state == fieldMissing:
		c.fail(prefix+missingSuffix, file, "%s is required", key)
	case state == fieldWrongType:
		c.fail(prefix+"_wrong_type", file, "%s must be a string", key)
	case strings.TrimSpace(value) == "":
		c.fail(prefix+"_empty", file, "%s must be non-empty", key)
	case utf8.RuneCountInString(value) > max:
		c.fail(prefix+"_too_long", file, "%s is %d characters, limit %d", key, utf8.RuneCountInString(value), max)
	case !supportedText(value, multiline):
		c.fail(prefix+"_character_unsupported", file, "%s holds unsupported or multi-line text", key)
	default:
		return value, true
	}
	return "", false
}

const manifestPath = ".codex-plugin/plugin.json"

func (c *dirChecker) checkManifest() string {
	file, ok := c.pkg[manifestPath]
	if !ok {
		c.fail("plugin_manifest_missing", manifestPath, "package root has no .codex-plugin/plugin.json")
		return ""
	}
	if file.Mode.Type() != 0 {
		c.fail("plugin_manifest_not_file", manifestPath, "manifest is not a regular file")
		return ""
	}
	if !utf8.Valid(file.Data) {
		c.fail("plugin_manifest_unreadable", manifestPath, "manifest is not UTF-8")
		return ""
	}
	var root any
	if err := json.Unmarshal(file.Data, &root); err != nil {
		c.fail("plugin_manifest_json_malformed", manifestPath, "%v", err)
		return ""
	}
	m, ok := root.(map[string]any)
	if !ok {
		c.fail("plugin_manifest_root_not_object", manifestPath, "manifest root is not an object")
		return ""
	}
	allowed := map[string]bool{"name": true, "version": true, "description": true, "author": true, "homepage": true,
		"repository": true, "license": true, "keywords": true, "skills": true, "interface": true}
	for _, key := range sortedMapKeys(m) {
		switch key {
		case "hooks", "outputStyles":
			c.fail("manifest_forbidden_field", manifestPath, "%s is not allowed in the directory edition", key)
		case "apps":
			c.fail("app_configuration_excluded", manifestPath, "apps is not allowed in a ZIP upload")
		case "mcpServers":
			c.fail("mcp_configuration_excluded", manifestPath, "mcpServers is not allowed in the directory edition")
		default:
			if !allowed[key] {
				c.fail("manifest_unknown_field", manifestPath, "unexpected manifest field %s", key)
			}
		}
	}

	name, state := jsonString(m, "name")
	switch {
	case state == fieldMissing:
		c.fail("plugin_name_missing", manifestPath, "name is required")
	case state == fieldWrongType:
		c.fail("plugin_name_wrong_type", manifestPath, "name must be a string")
	case name == "":
		c.fail("plugin_name_empty", manifestPath, "name must be non-empty")
	case utf8.RuneCountInString(name) > 64:
		c.fail("plugin_name_too_long", manifestPath, "name is %d characters, limit 64", utf8.RuneCountInString(name))
	case !pluginNamePattern.MatchString(name):
		c.fail("plugin_name_format", manifestPath, "name must match %s", pluginNamePattern)
	}

	version, state := jsonString(m, "version")
	switch {
	case state == fieldMissing:
		c.fail("plugin_version_missing", manifestPath, "version is required")
	case state == fieldWrongType:
		c.fail("plugin_version_wrong_type", manifestPath, "version must be a string")
	case version == "":
		c.fail("plugin_version_empty", manifestPath, "version must be non-empty")
	case len(version) > 64:
		c.fail("plugin_version_too_long", manifestPath, "version is longer than 64 characters")
	case !semverPattern.MatchString(version):
		c.fail("plugin_version_not_semver", manifestPath, "version %q is not semantic", version)
	}

	c.text(manifestPath, m, "description", "plugin_description", "_missing", 1024, true)

	switch author := m["author"].(type) {
	case nil:
		c.fail("plugin_developer_missing", manifestPath, "author.name is required")
	case map[string]any:
		c.text(manifestPath, author, "name", "plugin_author_name", "_empty", 120, false)
	default:
		c.fail("plugin_author_wrong_type", manifestPath, "author must be an object")
	}

	if value, state := jsonString(m, "homepage"); state == fieldWrongType {
		c.fail("plugin_homepage_wrong_type", manifestPath, "homepage must be a string")
	} else if state == fieldOK {
		c.checkHTTPS(manifestPath, "plugin_homepage", "homepage", value, 2048)
	}

	skills, state := jsonString(m, "skills")
	switch {
	case state == fieldMissing || (state == fieldOK && strings.TrimSpace(skills) == ""):
		c.fail("plugin_skills_path_empty", manifestPath, "skills must point at ./skills/")
	case state == fieldWrongType:
		c.fail("plugin_skills_path_wrong_type", manifestPath, "skills must be a string")
	case skills != "./skills/":
		c.fail("plugin_skills_path_unsupported", manifestPath, "skills is %q, want ./skills/", skills)
	}
	if !c.pkg.hasDir("skills") {
		c.fail("plugin_skills_directory_missing", "skills", "package has no skills/ directory")
	}

	c.checkInterface(m)
	return name
}

func sortedMapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (c *dirChecker) checkHTTPS(file, prefix, field, value string, max int) {
	parsed, err := url.Parse(value)
	switch {
	case strings.TrimSpace(value) == "":
		c.fail(prefix+"_empty", file, "%s must be non-empty", field)
	case err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || !supportedText(value, false):
		c.fail(prefix+"_format", file, "%s must be an HTTPS URL without credentials", field)
	case len(value) > max:
		c.fail(prefix+"_too_long", file, "%s is longer than %d characters", field, max)
	}
}

func (c *dirChecker) checkInterface(m map[string]any) {
	raw, present := m["interface"]
	ui, ok := raw.(map[string]any)
	if !present || !ok {
		c.fail("plugin_interface_wrong_type", manifestPath, "interface must be an object")
		return
	}
	allowed := map[string]bool{"displayName": true, "shortDescription": true, "longDescription": true,
		"developerName": true, "category": true, "capabilities": true, "defaultPrompt": true,
		"brandColor": true, "brandColorDark": true, "composerIcon": true, "logo": true,
		"websiteURL": true, "privacyPolicyURL": true, "termsOfServiceURL": true, "supportURL": true}
	for _, key := range sortedMapKeys(ui) {
		if key == "screenshots" {
			c.fail("screenshot_configuration_excluded", manifestPath, "interface.screenshots is not allowed in a skills-only ZIP")
		} else if !allowed[key] {
			c.fail("manifest_unknown_field", manifestPath, "unexpected interface field %s", key)
		}
	}
	c.text(manifestPath, ui, "displayName", "plugin_display_name", "_empty", 30, false)
	c.text(manifestPath, ui, "shortDescription", "plugin_short_description", "_missing", 30, false)
	c.text(manifestPath, ui, "longDescription", "plugin_long_description", "_empty", 4000, true)
	c.text(manifestPath, ui, "developerName", "plugin_developer_name", "_empty", 80, false)

	category, state := jsonString(ui, "category")
	switch {
	case state == fieldWrongType:
		c.fail("plugin_category_wrong_type", manifestPath, "category must be a string")
	case state == fieldMissing || strings.TrimSpace(category) == "":
		c.fail("plugin_category_empty", manifestPath, "category is required")
	case !directoryCategories[category]:
		c.fail("plugin_category_unknown", manifestPath, "category %q is not a supported category", category)
	}

	c.checkCapabilities(ui)
	c.checkPrompts(ui)
	for _, field := range []struct{ key, prefix, color string }{{"brandColor", "plugin_brand_color", "#FFFFFF"}, {"brandColorDark", "plugin_brand_color_dark", "#212121"}} {
		value, state := jsonString(ui, field.key)
		switch {
		case state == fieldMissing:
		case state == fieldWrongType:
			c.fail(field.prefix+"_wrong_type", manifestPath, "%s must be a string", field.key)
		case value == "":
			c.fail(field.prefix+"_empty", manifestPath, "%s must be non-empty", field.key)
		case !hexColorPattern.MatchString(value):
			c.fail(field.prefix+"_format", manifestPath, "%s must be a six-digit hex color", field.key)
		default:
			if ratio, ok := contrastRatio(value, field.color); ok && ratio < 2 {
				c.fail(field.prefix+"_contrast", manifestPath, "%s %s has contrast %.2f:1 against %s, need 2:1", field.key, value, ratio, field.color)
			}
		}
	}
	for _, field := range []struct{ key, prefix string }{{"websiteURL", "plugin_website_url"}, {"privacyPolicyURL", "plugin_privacy_policy_url"},
		{"termsOfServiceURL", "plugin_terms_of_service_url"}, {"supportURL", "plugin_support_url"}} {
		if value, state := jsonString(ui, field.key); state == fieldWrongType {
			c.fail(field.prefix+"_wrong_type", manifestPath, "%s must be a string", field.key)
		} else if state == fieldOK {
			c.checkHTTPS(manifestPath, field.prefix, field.key, value, 1024)
		}
	}
	c.checkIcon(ui, "composerIcon", "plugin_composer_icon_path_missing")
	c.checkIcon(ui, "logo", "plugin_logo_path_missing")
}

func (c *dirChecker) checkCapabilities(ui map[string]any) {
	raw, present := ui["capabilities"]
	if !present {
		return
	}
	list, ok := raw.([]any)
	if !ok {
		c.fail("plugin_capabilities_wrong_type", manifestPath, "capabilities must be a list of strings")
		return
	}
	if len(list) > 20 {
		c.fail("plugin_capabilities_too_many", manifestPath, "%d capabilities, limit 20", len(list))
	}
	for _, item := range list {
		s, ok := item.(string)
		switch {
		case !ok:
			c.fail("plugin_capability_wrong_type", manifestPath, "capability entries must be strings")
		case strings.TrimSpace(s) == "":
			c.fail("plugin_capability_empty", manifestPath, "capability entries must be non-empty")
		case utf8.RuneCountInString(s) > 120:
			c.fail("plugin_capability_too_long", manifestPath, "capability %q is longer than 120 characters", truncateForError(s))
		case !supportedText(s, false):
			c.fail("plugin_capability_character_unsupported", manifestPath, "capability %q holds unsupported or multi-line text", truncateForError(s))
		}
	}
}

func (c *dirChecker) checkPrompts(ui map[string]any) {
	raw, present := ui["defaultPrompt"]
	if !present {
		return
	}
	var list []any
	switch v := raw.(type) {
	case string:
		list = []any{v}
	case []any:
		list = v
	default:
		c.fail("plugin_default_prompt_wrong_type", manifestPath, "defaultPrompt must be a string or list of strings")
		return
	}
	if len(list) > 3 {
		c.fail("plugin_default_prompt_too_many", manifestPath, "%d prompts, limit 3", len(list))
	}
	seen := map[string]bool{}
	for _, item := range list {
		s, ok := item.(string)
		switch {
		case !ok:
			c.fail("plugin_default_prompt_entry_wrong_type", manifestPath, "prompt entries must be strings")
			continue
		case strings.TrimSpace(s) == "":
			c.fail("plugin_default_prompt_empty", manifestPath, "prompt entries must be non-empty")
			continue
		case utf8.RuneCountInString(s) > 128:
			c.fail("plugin_default_prompt_too_long", manifestPath, "prompt %q is longer than 128 characters", truncateForError(s))
		case !supportedText(s, false):
			c.fail("plugin_default_prompt_character_unsupported", manifestPath, "prompt %q holds unsupported or multi-line text", truncateForError(s))
		}
		if mentionPattern.MatchString(s) {
			c.fail("plugin_default_prompt_mention", manifestPath, "prompt %q holds an @mention", truncateForError(s))
		}
		key := strings.ToLower(strings.Join(strings.Fields(s), " "))
		if seen[key] {
			c.fail("plugin_default_prompt_duplicate", manifestPath, "prompt %q is duplicated", truncateForError(s))
		}
		seen[key] = true
	}
}

// contrastRatio returns the WCAG contrast ratio of two six-digit hex colors.
func contrastRatio(a, b string) (float64, bool) {
	la, ok := relativeLuminance(a)
	lb, ok2 := relativeLuminance(b)
	if !ok || !ok2 {
		return 0, false
	}
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05), true
}

func relativeLuminance(hex string) (float64, bool) {
	if !hexColorPattern.MatchString(hex) {
		return 0, false
	}
	var channels [3]float64
	for i := range channels {
		v, err := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		if err != nil {
			return 0, false
		}
		c := float64(v) / 255
		if c <= 0.03928 {
			channels[i] = c / 12.92
		} else {
			channels[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*channels[0] + 0.7152*channels[1] + 0.0722*channels[2], true
}

func (c *dirChecker) checkIcon(ui map[string]any, field, missingCode string) {
	raw, present := ui[field]
	if !present {
		c.fail(missingCode, manifestPath, "interface.%s is required", field)
		return
	}
	value, ok := raw.(string)
	if !ok {
		c.fail("declared_asset_path_wrong_type", manifestPath, "interface.%s must be a string", field)
		return
	}
	c.declaredAsset(manifestPath, "interface."+field, value, "", true)
}

// declaredAsset checks one declared asset path and, when it resolves to a
// regular file in the package, the image behind it. base is the directory the
// path is relative to ("" for the package root); manifest icons must carry a
// ./ prefix, skill icons may omit it.
func (c *dirChecker) declaredAsset(owner, label, value, base string, requirePrefix bool) {
	switch {
	case value == "":
		c.fail("declared_asset_path_empty", owner, "%s is empty", label)
		return
	case strings.TrimSpace(value) != value:
		c.fail("declared_asset_path_has_outer_whitespace", owner, "%s has outer whitespace", label)
		return
	case strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0:
		c.fail("declared_asset_path_has_control_character", owner, "%s has a control character", label)
		return
	case requirePrefix && !strings.HasPrefix(value, "./"):
		c.fail("branding_asset_path_missing_root_prefix", owner, "%s must start with ./", label)
		return
	}
	rel := strings.TrimPrefix(value, "./")
	segments := strings.Split(rel, "/")
	if strings.Contains(rel, `\`) || strings.HasPrefix(rel, "/") || containsSegment(segments, "..") ||
		containsSegment(segments, ".") || containsSegment(segments, "") ||
		(len(rel) >= 2 && rel[1] == ':') || rel == "" {
		c.fail("declared_asset_path_unsafe", owner, "%s %q is not a safe relative path", label, value)
		return
	}
	clean := path.Clean(rel)
	if base != "" {
		clean = path.Join(base, clean)
	}
	file, exists := c.pkg[clean]
	if !exists {
		if c.pkg.hasDir(clean) {
			c.fail("declared_asset_not_regular_file", owner, "%s %q is a directory", label, value)
		} else {
			c.fail("declared_asset_file_missing", owner, "%s %q does not exist in the package", label, value)
		}
		return
	}
	if file.Mode.Type() != 0 {
		c.fail("declared_asset_not_regular_file", owner, "%s %q is not a regular file", label, value)
		return
	}
	c.checkImage(clean, file.Data)
}

func (c *dirChecker) checkImage(name string, data []byte) {
	if len(data) > dirMaxImageBytes {
		c.fail("image_file_too_large", name, "%d bytes, limit %d", len(data), dirMaxImageBytes)
		return
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".svg":
		c.checkSVG(name, data)
	case ".png", ".jpg", ".jpeg":
		want := "png"
		if ext := strings.ToLower(path.Ext(name)); ext != ".png" {
			want = "jpeg"
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			c.fail("raster_image_decode_failed", name, "%v", err)
			return
		}
		if format != want {
			c.fail("raster_image_extension_content_mismatch", name, "extension says %s, content is %s", want, format)
			return
		}
		c.checkRasterSize(name, cfg.Width, cfg.Height)
	case ".webp":
		w, h, ok := webpDimensions(data)
		if !ok {
			c.fail("raster_image_decode_failed", name, "not a readable WebP file")
			return
		}
		c.checkRasterSize(name, w, h)
	default:
		c.fail("image_file_format_unsupported", name, "image must end in .png, .jpg, .jpeg, .webp, or .svg")
	}
}

func (c *dirChecker) checkRasterSize(name string, w, h int) {
	switch {
	case w != h:
		c.fail("raster_image_not_square", name, "%dx%d is not square", w, h)
	case w < 48:
		c.fail("raster_image_dimensions_too_small", name, "%dx%d is under 48x48", w, h)
	case w > 4096:
		c.fail("raster_image_dimensions_too_large", name, "%dx%d is over 4096x4096", w, h)
	}
}

// webpDimensions reads the canvas size from a RIFF/WEBP header.
func webpDimensions(d []byte) (int, int, bool) {
	if len(d) < 30 || string(d[0:4]) != "RIFF" || string(d[8:12]) != "WEBP" {
		return 0, 0, false
	}
	switch string(d[12:16]) {
	case "VP8X":
		w := 1 + int(d[24]) | int(d[25])<<8 | int(d[26])<<16
		h := 1 + int(d[27]) | int(d[28])<<8 | int(d[29])<<16
		return w, h, true
	case "VP8 ":
		if d[23] != 0x9d || d[24] != 0x01 || d[25] != 0x2a {
			return 0, 0, false
		}
		return int(binary.LittleEndian.Uint16(d[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(d[28:30]) & 0x3fff), true
	case "VP8L":
		if d[20] != 0x2f {
			return 0, 0, false
		}
		bits := binary.LittleEndian.Uint32(d[21:25])
		return int(bits&0x3fff) + 1, int((bits>>14)&0x3fff) + 1, true
	}
	return 0, 0, false
}

func (c *dirChecker) checkSVG(name string, data []byte) {
	if !utf8.Valid(data) {
		c.fail("svg_xml_malformed", name, "SVG is not UTF-8")
		return
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var root *xml.StartElement
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			c.fail("svg_xml_malformed", name, "%v", err)
			return
		}
		if start, ok := token.(xml.StartElement); ok && root == nil {
			copied := start.Copy()
			root = &copied
		}
	}
	if root == nil || root.Name.Local != "svg" {
		c.fail("svg_root_element_invalid", name, "root element is not <svg>")
		return
	}
	attrs := map[string]string{}
	for _, a := range root.Attr {
		if a.Name.Space == "" {
			attrs[a.Name.Local] = a.Value
		}
	}
	number := func(s string) (float64, bool) {
		s = strings.TrimSpace(s)
		if !numericPattern.MatchString(s) {
			return 0, false
		}
		v, err := strconv.ParseFloat(s, 64)
		return v, err == nil
	}
	var sizes [][2]float64
	if vb, ok := attrs["viewBox"]; ok {
		parts := strings.FieldsFunc(vb, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' || r == '\n' })
		if len(parts) != 4 {
			c.fail("svg_dimensions_not_numeric", name, "viewBox %q is not four numbers", vb)
			return
		}
		var nums [4]float64
		for i, part := range parts {
			v, ok := number(part)
			if !ok {
				c.fail("svg_dimensions_not_numeric", name, "viewBox %q is not four numbers", vb)
				return
			}
			nums[i] = v
		}
		sizes = append(sizes, [2]float64{nums[2], nums[3]})
	}
	ws, hasW := attrs["width"]
	hs, hasH := attrs["height"]
	if hasW || hasH {
		w, okW := number(ws)
		h, okH := number(hs)
		if !hasW || !hasH {
			if len(sizes) == 0 {
				c.fail("svg_dimensions_missing", name, "SVG needs both width and height or a viewBox")
				return
			}
		} else if !okW || !okH {
			c.fail("svg_dimensions_not_numeric", name, "width %q and height %q must be plain numbers", ws, hs)
			return
		} else {
			sizes = append(sizes, [2]float64{w, h})
		}
	}
	if len(sizes) == 0 {
		c.fail("svg_dimensions_missing", name, "SVG needs a numeric viewBox or numeric width and height")
		return
	}
	for _, s := range sizes {
		switch {
		case !(s[0] > 0 && s[1] > 0) || math.IsInf(s[0], 0) || math.IsInf(s[1], 0):
			c.fail("svg_dimensions_not_positive", name, "dimensions %vx%v are not positive finite numbers", s[0], s[1])
			return
		case s[0] != s[1]:
			c.fail("svg_dimensions_not_square", name, "dimensions %vx%v are not square", s[0], s[1])
			return
		case s[0] < 48:
			c.fail("svg_dimensions_too_small", name, "dimensions %vx%v are under 48x48", s[0], s[1])
			return
		}
	}
}

func (c *dirChecker) checkSkills(pluginName string) {
	dirs := map[string]bool{}
	for _, name := range c.pkg.paths() {
		parts := strings.Split(name, "/")
		if parts[0] != "skills" {
			continue
		}
		switch {
		case len(parts) == 2 && c.pkg[name].Mode.Type() == fs.ModeDir:
			dirs[parts[1]] = true
		case len(parts) == 2:
			c.fail("skill_file_ignored", name, "files directly under skills/ are not imported as skills")
		case len(parts) >= 3:
			dirs[parts[1]] = true
			if parts[len(parts)-1] == "SKILL.md" && len(parts) > 3 {
				c.fail("skill_manifest_nested", name, "SKILL.md must sit directly in skills/<skill>/")
			}
		}
	}
	if len(dirs) == 0 {
		c.fail("plugin_runtime_surface_missing", "skills", "package has no skills")
	}
	identities := map[string]string{}
	folded := map[string]string{}
	for _, dir := range sortedKeys(dirs) {
		if other, ok := folded[strings.ToLower(dir)]; ok {
			c.fail("archive_member_path_normalization_collision", "skills/"+dir, "skill directory collides with skills/%s after case folding", other)
			continue
		}
		folded[strings.ToLower(dir)] = dir
		if strings.HasPrefix(dir, ".") {
			c.fail("skill_directory_hidden", "skills/"+dir, "skill directory names must not begin with a dot")
			continue
		}
		c.checkSkill(pluginName, dir, identities)
	}
}

func (c *dirChecker) checkSkill(pluginName, dir string, identities map[string]string) {
	manifest := "skills/" + dir + "/SKILL.md"
	file, ok := c.pkg[manifest]
	if !ok {
		c.fail("skill_manifest_missing", "skills/"+dir, "skill directory has no SKILL.md")
		return
	}
	if file.Mode.Type() != 0 {
		c.fail("skill_manifest_not_regular_file", manifest, "SKILL.md is not a regular file")
		return
	}
	if !utf8.Valid(file.Data) {
		c.fail("skill_manifest_invalid_utf8", manifest, "SKILL.md is not valid UTF-8")
		return
	}
	c.checkSkillBundleSize(dir)
	fm, err := parseSkillFrontMatter(string(file.Data))
	if err != nil {
		var coded *dirError
		if errors.As(err, &coded) {
			c.fail(coded.Code, manifest, "%s", coded.Message)
		} else {
			c.fail("skill_frontmatter_yaml_malformed", manifest, "%v", err)
		}
		return
	}
	if fm.Kinds["metadata"] == yamlMap {
		c.metadataSkills = append(c.metadataSkills, dir)
	}
	name, nameOK := c.skillScalar(manifest, fm, "name", 0)
	c.skillScalar(manifest, fm, "description", 1024)
	if strings.TrimSpace(fm.Body) == "" {
		c.fail("skill_body_empty", manifest, "skill instructions are empty")
	}
	if nameOK {
		name = collapseSpace(name)
		if id := pluginName + ":" + name; utf8.RuneCountInString(id) > 64 {
			c.fail("skill_identity_too_long", manifest, "%s is %d characters, limit 64", id, utf8.RuneCountInString(id))
		}
		if other, dup := identities[name]; dup {
			c.fail("skill_identity_duplicate", manifest, "name %q is also used by %s", name, other)
		}
		identities[name] = manifest
		if name != dir {
			c.note("skill_name_directory_mismatch", manifest, "name %q differs from directory %q", name, dir)
		}
	}
	if agent, ok := c.pkg["skills/"+dir+"/agents/openai.yaml"]; ok {
		c.checkAgentYAML("skills/"+dir+"/agents/openai.yaml", agent)
	}
}

func collapseSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

func (c *dirChecker) skillScalar(file string, fm skillFrontMatter, key string, max int) (string, bool) {
	prefix := "skill_" + key
	kind, present := fm.Kinds[key]
	switch {
	case !present:
		c.fail(prefix+"_missing", file, "%s is required", key)
		return "", false
	case kind != yamlScalar && kind != yamlNull:
		c.fail(prefix+"_wrong_type", file, "%s must be a string", key)
		return "", false
	}
	value := collapseSpace(fm.Scalars[key])
	switch {
	case value == "":
		c.fail(prefix+"_empty", file, "%s must be non-empty", key)
	case max > 0 && utf8.RuneCountInString(value) > max:
		c.fail(prefix+"_too_long", file, "%s is %d characters, limit %d", key, utf8.RuneCountInString(value), max)
	case !supportedText(value, false):
		c.fail(prefix+"_character_unsupported", file, "%s holds unsupported text", key)
	default:
		return value, true
	}
	return "", false
}

func (c *dirChecker) checkSkillBundleSize(dir string) {
	prefix := "skills/" + dir + "/"
	counter := &countingWriter{}
	writer, err := flate.NewWriter(counter, 6)
	if err != nil {
		return
	}
	for _, name := range c.pkg.paths() {
		if strings.HasPrefix(name, prefix) {
			_, _ = writer.Write(c.pkg[name].Data)
		}
	}
	_ = writer.Close()
	if counter.n > dirMaxSkillBundle {
		c.fail("skill_bundle_too_large", "skills/"+dir, "compressed bundle is %d bytes, local limit %d", counter.n, dirMaxSkillBundle)
	}
}

type countingWriter struct{ n int }

func (w *countingWriter) Write(p []byte) (int, error) { w.n += len(p); return len(p), nil }

func (c *dirChecker) checkAgentYAML(name string, file dirFile) {
	if file.Mode.Type() != 0 {
		c.fail("skill_agent_not_regular_file", name, "agents/openai.yaml is not a regular file")
		return
	}
	if !utf8.Valid(file.Data) {
		c.fail("skill_agent_invalid_utf8", name, "agents/openai.yaml is not UTF-8")
		return
	}
	root, err := parseYAMLSubset(string(file.Data))
	if err != nil {
		c.fail("skill_agent_yaml_malformed", name, "%v", err)
		return
	}
	if root.kind != yamlMap {
		c.fail("skill_agent_top_level_wrong_type", name, "top level must be a mapping")
		return
	}
	iface, ok := root.get("interface")
	switch {
	case !ok || iface.kind == yamlNull:
		c.fail("skill_agent_interface_missing", name, "interface is required when agents/openai.yaml is included")
	case iface.kind != yamlMap:
		c.fail("skill_agent_interface_wrong_type", name, "interface must be a mapping")
	default:
		for _, field := range []struct{ key, prefix string }{{"display_name", "skill_agent_display_name"}, {"short_description", "skill_agent_short_description"}} {
			v, ok := iface.get(field.key)
			switch {
			case !ok:
				c.fail(field.prefix+"_missing", name, "interface.%s is required", field.key)
			case v.kind != yamlScalar && v.kind != yamlNull:
				c.fail(field.prefix+"_wrong_type", name, "interface.%s must be a string", field.key)
			case strings.TrimSpace(v.text) == "":
				c.fail(field.prefix+"_empty", name, "interface.%s must not be empty", field.key)
			}
		}
		for _, key := range []string{"icon_small", "icon_large"} {
			if v, ok := iface.get(key); ok {
				if v.kind != yamlScalar && v.kind != yamlNull {
					c.fail("skill_agent_"+key+"_wrong_type", name, "interface.%s must be a relative file path", key)
				} else if strings.TrimSpace(v.text) == "" {
					c.fail("skill_agent_"+key+"_empty", name, "interface.%s must not be empty", key)
				} else {
					c.declaredAsset(name, "interface."+key, v.text, path.Dir(path.Dir(name)), false)
				}
			}
		}
		if v, ok := iface.get("brand_color"); ok {
			switch {
			case v.kind != yamlScalar && v.kind != yamlNull:
				c.fail("skill_agent_brand_color_wrong_type", name, "interface.brand_color must be a string")
			case strings.TrimSpace(v.text) == "":
				c.fail("skill_agent_brand_color_empty", name, "interface.brand_color must not be empty")
			case !hexColorPattern.MatchString(v.text):
				c.fail("skill_agent_brand_color_format", name, "interface.brand_color must be a six-digit hex color")
			}
		}
		if v, ok := iface.get("default_prompt"); ok {
			switch {
			case v.kind != yamlScalar && v.kind != yamlNull:
				c.fail("skill_agent_default_prompt_wrong_type", name, "interface.default_prompt must be a string")
			case strings.TrimSpace(v.text) == "":
				c.fail("skill_agent_default_prompt_empty", name, "interface.default_prompt must not be empty")
			}
		}
	}
	if policy, ok := root.get("policy"); ok && policy.kind != yamlNull {
		if policy.kind != yamlMap {
			c.fail("skill_agent_policy_wrong_type", name, "policy must be a mapping")
		} else {
			for _, key := range policy.keys {
				value := policy.m[key]
				switch key {
				case "allow_implicit_invocation":
					if value.kind != yamlScalar || value.quoted || (value.text != "true" && value.text != "false") {
						c.fail("skill_agent_allow_implicit_invocation_wrong_type", name, "policy.allow_implicit_invocation must be true or false")
					}
				case "products":
					valid := value.kind == yamlList && len(value.list) > 0
					for _, item := range value.list {
						if item.kind != yamlScalar || (item.text != "CHAT" && item.text != "CODEX") {
							valid = false
						}
					}
					if !valid {
						c.fail("skill_agent_allow_implicit_invocation_wrong_type", name, "policy.products must list CHAT, CODEX, or both")
					}
				default:
					c.fail("skill_agent_allow_implicit_invocation_wrong_type", name, "policy may contain only products and allow_implicit_invocation, found %s", key)
				}
			}
		}
	}
	if deps, ok := root.get("dependencies"); ok && deps.kind != yamlNull {
		if deps.kind != yamlMap {
			c.fail("skill_agent_dependencies_wrong_type", name, "dependencies must be a mapping")
		} else {
			for _, key := range deps.keys {
				if key != "tools" {
					c.fail("skill_agent_dependency_unsupported", name, "only dependencies.tools is supported, found %s", key)
				}
			}
		}
	}
}

// --- skill front matter ---

type dirError struct{ Code, Message string }

func (e *dirError) Error() string { return e.Code + ": " + e.Message }

type skillFrontMatter struct {
	Scalars map[string]string
	Nested  map[string]map[string]string
	Kinds   map[string]yamlKind
	Body    string
}

func parseSkillFrontMatter(text string) (skillFrontMatter, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t") != "---" {
		return skillFrontMatter{}, &dirError{"skill_frontmatter_missing", "SKILL.md must start with a --- front matter line"}
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return skillFrontMatter{}, &dirError{"skill_frontmatter_unclosed", "front matter has no closing --- line"}
	}
	root, err := parseYAMLSubset(strings.Join(lines[1:end], "\n"))
	if err != nil {
		return skillFrontMatter{}, &dirError{"skill_frontmatter_yaml_malformed", err.Error()}
	}
	if root.kind != yamlMap {
		return skillFrontMatter{}, &dirError{"skill_frontmatter_wrong_type", "front matter must be a YAML mapping"}
	}
	fm := skillFrontMatter{
		Scalars: map[string]string{},
		Nested:  map[string]map[string]string{},
		Kinds:   map[string]yamlKind{},
		Body:    strings.Join(lines[end+1:], "\n"),
	}
	for _, key := range root.keys {
		value := root.m[key]
		fm.Kinds[key] = value.kind
		switch value.kind {
		case yamlScalar:
			fm.Scalars[key] = value.text
		case yamlNull:
			fm.Scalars[key] = ""
		case yamlMap:
			inner := map[string]string{}
			for _, k := range value.keys {
				inner[k] = value.m[k].text
			}
			fm.Nested[key] = inner
		}
	}
	return fm, nil
}

// --- references ---

var removedPackageDirs = []string{"hooks", "output-styles"}

func (c *dirChecker) checkReferences() {
	for _, name := range c.pkg.paths() {
		parts := strings.Split(name, "/")
		if parts[0] != "skills" || len(parts) < 3 || !strings.HasSuffix(name, ".md") || strings.HasPrefix(parts[1], ".") {
			continue
		}
		inScope := name == "skills/"+parts[1]+"/SKILL.md" || (len(parts) > 3 && parts[2] == "references")
		if !inScope || !utf8.Valid(c.pkg[name].Data) {
			continue
		}
		c.checkFileReferences(parts[1], name, string(c.pkg[name].Data))
	}
}

func (c *dirChecker) checkFileReferences(skill, file, text string) {
	skillDir := "skills/" + skill
	fileDir := path.Dir(file)
	reported := map[string]bool{}
	notedHook := false
	inFence := false
	fence := ""
	for n, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.Contains(line, "hooks/run-hook.cmd") && !notedHook {
			notedHook = true
			c.note("reference_run_hook", file, "line %d mentions hooks/run-hook.cmd, which the directory edition does not ship", n+1)
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if !inFence {
				inFence, fence = true, marker
			} else if marker == fence {
				inFence = false
			}
			continue
		}
		var tokens []refToken
		if !inFence {
			for _, m := range markdownLinkTarget.FindAllStringSubmatch(line, -1) {
				tokens = append(tokens, refToken{raw: m[1], link: true})
			}
			for _, m := range inlineCodePattern.FindAllStringSubmatch(line, -1) {
				for _, word := range strings.Fields(m[1]) {
					tokens = append(tokens, refToken{raw: word})
				}
			}
		} else {
			for _, word := range strings.Fields(line) {
				tokens = append(tokens, refToken{raw: word})
			}
		}
		for _, tok := range tokens {
			target, ok := tok.candidate()
			if !ok || reported[target] {
				continue
			}
			bases := []string{skillDir, fileDir}
			if tok.link {
				bases = []string{fileDir}
			}
			stripped := strings.TrimLeft(strings.ReplaceAll(target, "../", ""), "./")
			if strings.HasPrefix(stripped, "hooks/run-hook.cmd") {
				continue
			}
			resolvedAny := false
			var removed, outside bool
			for _, base := range bases {
				resolved := path.Join(base, target)
				if resolved == ".." || strings.HasPrefix(resolved, "../") {
					outside = true
					continue
				}
				first := strings.SplitN(resolved, "/", 2)[0]
				for _, dir := range removedPackageDirs {
					if first == dir {
						removed = true
					}
				}
				if _, isFile := c.pkg[resolved]; isFile || c.pkg.hasDir(resolved) {
					resolvedAny = true
				}
			}
			for _, dir := range removedPackageDirs {
				if strings.HasPrefix(stripped, dir+"/") {
					removed = true
				}
			}
			switch {
			case resolvedAny:
			case removed:
				reported[target] = true
				c.fail("reference_removed_directory", file, "%s points into %s, which the directory edition removes", target, strings.Join(removedPackageDirs, " or "))
			default:
				reported[target] = true
				if outside {
					c.fail("reference_unresolved", file, "%s leaves the package", target)
				} else {
					c.fail("reference_unresolved", file, "%s does not resolve to a file in the package", target)
				}
			}
		}
	}
}

type refToken struct {
	raw  string
	link bool
}

// candidate returns the cleaned relative path a token refers to, or false
// when the token is a URL, a placeholder, or not path-shaped.
func (t refToken) candidate() (string, bool) {
	s := t.raw
	if !t.link {
		s = strings.Trim(s, "\"'()[]<>,;")
		s = strings.TrimRight(s, ".:!?")
		s = strings.TrimPrefix(s, "=")
	}
	if i := strings.IndexByte(s, '#'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '?'); i >= 0 && t.link {
		s = s[:i]
	}
	s = lineSuffixPattern.ReplaceAllString(s, "")
	if decoded, err := url.PathUnescape(s); err == nil {
		s = decoded
	}
	if s == "" || strings.Contains(s, "://") || strings.HasPrefix(s, "mailto:") ||
		strings.ContainsAny(s, "<>{}$*|\\ ") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~") {
		return "", false
	}
	if t.link {
		return s, !strings.Contains(s, ":")
	}
	stripped := strings.TrimLeft(strings.ReplaceAll(s, "../", ""), "./")
	if stripped == "" || strings.Contains(s, ":") {
		return "", false
	}
	switch {
	case strings.HasPrefix(s, "../") || strings.HasPrefix(s, "./"):
		return s, strings.Contains(stripped, "/") || strings.Contains(stripped, ".")
	}
	// Bare directory names ("assets/") and agents/openai.yaml are generic
	// mentions of per-skill conventions, not references into this package.
	switch s {
	case "references/", "scripts/", "assets/", "agents/":
		return "", false
	}
	for _, prefix := range []string{"references/", "scripts/", "assets/", "hooks/", "output-styles/"} {
		if strings.HasPrefix(s, prefix) {
			return s, true
		}
	}
	return "", false
}
