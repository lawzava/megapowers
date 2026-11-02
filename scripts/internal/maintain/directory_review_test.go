package maintain

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Cross-vendor review findings against the directory edition.

type rawZipEntry struct {
	name       string
	mode       fs.FileMode
	declared   uint64
	compressed []byte
}

func deflateBytes(t *testing.T, chunk []byte, repeat int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < repeat; i++ {
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// writeFixtureZip writes the valid fixture package plus extra raw entries
// whose headers may disagree with their data.
func writeFixtureZip(t *testing.T, extra []rawZipEntry, skip func(string) bool) string {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range buildFixture(t) {
		if skip != nil && skip(f.Path) {
			continue
		}
		h := &zip.FileHeader{Name: f.Path, Method: zip.Deflate}
		h.SetMode(f.Mode)
		out, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = out.Write(f.Data)
	}
	for _, e := range extra {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate, UncompressedSize64: e.declared, CompressedSize64: uint64(len(e.compressed))}
		if e.mode == 0 {
			e.mode = 0o644
		}
		h.SetMode(e.mode)
		if e.mode&fs.ModeSymlink != 0 {
			// SetMode lets the directory bit win; write the symlink type directly.
			h.CreatorVersion = 3 << 8
			h.ExternalAttrs = (0o120000 | 0o777) << 16
		}
		if strings.HasSuffix(e.name, "/") {
			h.Method = zip.Store
		}
		out, err := w.CreateRaw(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = out.Write(e.compressed)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "p.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func requireCode(t *testing.T, report dirReport, code string) {
	t.Helper()
	if !contains(report.codes(), code) {
		t.Fatalf("want %s, got %v", code, report.Errors)
	}
}

// requireSizeRejection accepts the explicit mismatch code or the standard
// library's own read error: Go 1.25's zip reader already refuses a stream
// that disagrees with its header, and the explicit check makes that
// independent of the library.
func requireSizeRejection(t *testing.T, report dirReport) {
	t.Helper()
	if !contains(report.codes(), "archive_member_size_mismatch") && !contains(report.codes(), "archive_member_unreadable") {
		t.Fatalf("want a size rejection, got %v", report.Errors)
	}
}

// Finding 1: central-directory sizes are claims. Real bytes decide.
func TestDirectoryZipRejectsMemberThatExpandsPastItsHeader(t *testing.T) {
	bomb := deflateBytes(t, make([]byte, 1<<20), 101) // 101 MiB of zeros
	path := writeFixtureZip(t, []rawZipEntry{{name: "skills/alpha/references/bomb.md", declared: 10, compressed: bomb}}, nil)
	report, err := validateDirectoryZip(path)
	if err != nil {
		t.Fatal(err)
	}
	requireSizeRejection(t, report)
}

func TestDirectoryZipRejectsMemberShorterThanItsHeader(t *testing.T) {
	short := deflateBytes(t, []byte("hello"), 1)
	path := writeFixtureZip(t, []rawZipEntry{{name: "skills/alpha/references/short.md", declared: 1000, compressed: short}}, nil)
	report, err := validateDirectoryZip(path)
	if err != nil {
		t.Fatal(err)
	}
	requireSizeRejection(t, report)
}

// Finding 2: directory entries get the same layout and mode checks as files.
func TestDirectoryZipChecksDirectoryEntries(t *testing.T) {
	cases := []struct {
		name string
		dir  string
		mode fs.FileMode
		code string
	}{
		{"hooks", "hooks/", fs.ModeDir | 0o755, "forbidden_path"},
		{"output styles", "output-styles/", fs.ModeDir | 0o755, "forbidden_path"},
		{"claude plugin", ".claude-plugin/", fs.ModeDir | 0o755, "forbidden_path"},
		{"testdata", "skills/alpha/testdata/", fs.ModeDir | 0o755, "forbidden_test_file"},
		{"unknown root", "extra/", fs.ModeDir | 0o755, "unexpected_root_entry"},
		{"symlink dir", "skills/alpha/links/", fs.ModeDir | fs.ModeSymlink | 0o777, "archive_member_type_unsupported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFixtureZip(t, []rawZipEntry{{name: tc.dir, mode: tc.mode}}, nil)
			report, err := validateDirectoryZip(path)
			if err != nil {
				t.Fatal(err)
			}
			requireCode(t, report, tc.code)
		})
	}
	t.Run("plain directory entries under skills are fine", func(t *testing.T) {
		path := writeFixtureZip(t, []rawZipEntry{{name: "skills/alpha/references/", mode: fs.ModeDir | 0o755}}, nil)
		report, err := validateDirectoryZip(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Errors) != 0 {
			t.Fatalf("errors: %v", report.Errors)
		}
	})
}

// Finding 3: dot segments and canonical-path collisions.
func TestDirectoryArchiveCanonicalPaths(t *testing.T) {
	ok := func(name string) archiveEntry { return archiveEntry{Name: name, Size: 1, Mode: 0o644} }
	cases := []struct {
		name    string
		code    string
		entries []archiveEntry
	}{
		{"dot segment", "archive_member_path_has_dot_segment", []archiveEntry{ok("a/./b")}},
		{"leading dot segment", "archive_member_path_has_dot_segment", []archiveEntry{ok("./a")}},
		{"dot collides with clean", "archive_member_path_normalization_collision", []archiveEntry{ok("a/./b"), ok("a/b")}},
		{"file and dir same path", "archive_member_path_type_conflict", []archiveEntry{ok("a"), {Name: "a/", Mode: fs.ModeDir | 0o755}}},
		{"dir mode on file name", "archive_member_type_unsupported", []archiveEntry{{Name: "a/", Mode: fs.ModeSymlink | fs.ModeDir}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var codes []string
			for _, issue := range validateArchiveEntries(tc.entries, 1<<20) {
				codes = append(codes, issue.Code)
			}
			if !contains(codes, tc.code) {
				t.Fatalf("want %s, got %v", tc.code, codes)
			}
		})
	}
}

// Finding 4: skill icon paths follow the same asset rules as manifest icons.
func TestDirectoryAgentIconPaths(t *testing.T) {
	agent := func(field, value string) string {
		return "interface:\n  display_name: \"x\"\n  short_description: \"y\"\n  " + field + ": \"" + value + "\"\n"
	}
	cases := []struct {
		name  string
		setup func(p dirPackage)
		yaml  string
		code  string // empty means no error
	}{
		{"traversal", nil, agent("icon_small", "../../evil.svg"), "declared_asset_path_unsafe"},
		{"absolute", nil, agent("icon_large", "/etc/x.svg"), "declared_asset_path_unsafe"},
		{"outer whitespace", nil, agent("icon_small", " assets/icon.svg"), "declared_asset_path_has_outer_whitespace"},
		{"missing file", nil, agent("icon_small", "assets/none.svg"), "declared_asset_file_missing"},
		{"bad format", func(p dirPackage) { setFile(p, "skills/beta/assets/x.gif", "GIF89a") }, agent("icon_small", "assets/x.gif"), "image_file_format_unsupported"},
		{"not square", func(p dirPackage) {
			setFile(p, "skills/beta/assets/x.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="128" height="64"></svg>`)
		}, agent("icon_large", "./assets/x.svg"), "svg_dimensions_not_square"},
		{"directory", func(p dirPackage) { setFile(p, "skills/beta/assets/d/x.svg", fixtureSVG) }, agent("icon_small", "assets/d"), "declared_asset_not_regular_file"},
		{"valid", func(p dirPackage) { setFile(p, "skills/beta/assets/ok.svg", fixtureSVG) }, agent("icon_small", "./assets/ok.svg"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pkg := fixturePackage(t)
			if tc.setup != nil {
				tc.setup(pkg)
			}
			setFile(pkg, "skills/beta/agents/openai.yaml", tc.yaml)
			report := validateDirectoryPackage(pkg)
			if tc.code == "" {
				if len(report.Errors) != 0 {
					t.Fatalf("errors: %v", report.Errors)
				}
				return
			}
			requireCode(t, report, tc.code)
		})
	}
}

// Finding 5: outputs are written only after the package validates.
func TestDirectoryEditionWritesNothingWhenInvalid(t *testing.T) {
	repo := newDirectorySource(t)
	skill := filepath.Join(repo, "plugins/megapowers/skills/beta/SKILL.md")
	if err := os.WriteFile(skill, []byte("no front matter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	custom := filepath.Join(t.TempDir(), "custom.zip")
	var stdout, stderr bytes.Buffer
	if code := runDirectoryEdition(context.Background(), repo, []string{"--out", out, "--zip", custom}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("output directory exists after a failed validation: %v", err)
	}
	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Errorf("zip exists after a failed validation: %v", err)
	}
}

func TestDirectoryEditionZipOnlyLeavesDefaultTreeAlone(t *testing.T) {
	repo := newDirectorySource(t)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	custom := filepath.Join(t.TempDir(), "custom.zip")
	var stdout, stderr bytes.Buffer
	if code := runDirectoryEdition(context.Background(), repo, []string{"--zip", custom}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d\n%s", code, stderr.String())
	}
	if _, err := os.Stat(custom); err != nil {
		t.Fatalf("zip missing: %v", err)
	}
	entries, _ := os.ReadDir(tmp)
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("--zip alone touched TMPDIR: %v", names)
	}
	if strings.Contains(stdout.String(), "tree:") {
		t.Errorf("stdout reports a tree that was not written:\n%s", stdout.String())
	}
}

// Round 2.

func TestDirectoryZipDirectoryEntriesMustBeEmpty(t *testing.T) {
	t.Run("declared size", func(t *testing.T) {
		path := writeFixtureZip(t, []rawZipEntry{{name: "skills/alpha/references/", mode: fs.ModeDir | 0o755, declared: 5, compressed: []byte("hello")}}, nil)
		report, err := validateDirectoryZip(path)
		if err != nil {
			t.Fatal(err)
		}
		requireCode(t, report, "archive_directory_entry_not_empty")
	})
	t.Run("body without declared size", func(t *testing.T) {
		path := writeFixtureZip(t, []rawZipEntry{{name: "skills/alpha/references/", mode: fs.ModeDir | 0o755, compressed: []byte("hello world payload")}}, nil)
		report, err := validateDirectoryZip(path)
		if err != nil {
			t.Fatal(err)
		}
		requireCode(t, report, "archive_directory_entry_not_empty")
	})
	t.Run("entry list", func(t *testing.T) {
		issues := validateArchiveEntries([]archiveEntry{{Name: "a/", Size: 3, Mode: fs.ModeDir | 0o755}}, 10)
		if len(issues) == 0 || issues[0].Code != "archive_directory_entry_not_empty" {
			t.Fatalf("issues = %v", issues)
		}
	})
}

func TestDirectoryArchiveFoldsParentSegments(t *testing.T) {
	f := func(name string) archiveEntry { return archiveEntry{Name: name, Size: 1, Mode: 0o644} }
	cases := []struct {
		name    string
		code    string
		entries []archiveEntry
	}{
		{"file and folded directory", "archive_member_path_type_conflict", []archiveEntry{f("skills/alpha/item"), f("skills/alpha/ITEM/sub.md")}},
		{"folded skill directories", "archive_member_path_normalization_collision", []archiveEntry{f("skills/alpha/SKILL.md"), f("skills/Alpha/references/doc.md")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var codes []string
			for _, issue := range validateArchiveEntries(tc.entries, 1<<20) {
				codes = append(codes, issue.Code)
			}
			if !contains(codes, tc.code) {
				t.Fatalf("want %s, got %v", tc.code, codes)
			}
		})
	}
	t.Run("package level skill directories", func(t *testing.T) {
		pkg := fixturePackage(t)
		setFile(pkg, "skills/Alpha/SKILL.md", "---\nname: other\ndescription: d\n---\nbody\n")
		requireCode(t, validateDirectoryPackage(pkg), "archive_member_path_normalization_collision")
	})
}

func TestDirectoryAssetPathsRejectDotAndEmptySegments(t *testing.T) {
	agent := func(value string) string {
		return "interface:\n  display_name: \"x\"\n  short_description: \"y\"\n  icon_small: \"" + value + "\"\n"
	}
	for _, value := range []string{"assets/./ok.svg", "././assets/ok.svg", "assets//ok.svg", "./.", "./assets/"} {
		t.Run("agent "+value, func(t *testing.T) {
			pkg := fixturePackage(t)
			setFile(pkg, "skills/beta/assets/ok.svg", fixtureSVG)
			setFile(pkg, "skills/beta/agents/openai.yaml", agent(value))
			requireCode(t, validateDirectoryPackage(pkg), "declared_asset_path_unsafe")
		})
	}
	for _, value := range []string{"./assets/./icon.svg", "././assets/icon.svg", "./assets//icon.svg", "./."} {
		t.Run("manifest "+value, func(t *testing.T) {
			pkg := fixturePackage(t)
			setManifest(pkg, func(m, ui map[string]any) { ui["logo"] = value })
			requireCode(t, validateDirectoryPackage(pkg), "declared_asset_path_unsafe")
		})
	}
}

// Round 3.

func TestDirectoryZipDirectoryEntryTinyBodiesAreRejected(t *testing.T) {
	for _, body := range []string{"x", "xy"} {
		t.Run(body, func(t *testing.T) {
			path := writeFixtureZip(t, []rawZipEntry{{name: "skills/alpha/references/", mode: fs.ModeDir | 0o755, compressed: []byte(body)}}, nil)
			report, err := validateDirectoryZip(path)
			if err != nil {
				t.Fatal(err)
			}
			requireCode(t, report, "archive_directory_entry_not_empty")
		})
	}
}

func TestDirectoryArchiveFoldedDirectoryShapesAreOrderIndependent(t *testing.T) {
	file := func(name string) archiveEntry { return archiveEntry{Name: name, Size: 1, Mode: 0o644} }
	dir := func(name string) archiveEntry { return archiveEntry{Name: name, Mode: fs.ModeDir | 0o755} }
	cases := []struct {
		name string
		want []string
		a, b archiveEntry
	}{
		{"directory spellings", []string{"archive_member_path_normalization_collision"}, file("skills/alpha/references/doc.md"), dir("skills/alpha/References/")},
		{"file versus directory", []string{"archive_member_path_type_conflict"}, file("skills/alpha/item"), dir("skills/alpha/ITEM/")},
		{"file versus nested directory", []string{"archive_member_path_type_conflict"}, file("skills/alpha/item"), file("skills/alpha/ITEM/sub.md")},
	}
	for _, tc := range cases {
		for _, order := range [][]archiveEntry{{tc.a, tc.b}, {tc.b, tc.a}} {
			var codes []string
			seen := map[string]bool{}
			for _, issue := range validateArchiveEntries(order, 1<<20) {
				if !seen[issue.Code] {
					seen[issue.Code] = true
					codes = append(codes, issue.Code)
				}
			}
			sort.Strings(codes)
			if strings.Join(codes, ",") != strings.Join(tc.want, ",") {
				t.Errorf("%s, first=%s: codes %v, want %v", tc.name, order[0].Name, codes, tc.want)
			}
		}
	}
}
