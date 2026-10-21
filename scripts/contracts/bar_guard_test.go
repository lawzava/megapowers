package contracts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

const barGuardTool = "plugins/megapowers/skills/verify-and-finish/scripts/bar-guard.go"

type barGuardFinding struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	Waived bool   `json:"waived"`
	Reason string `json:"reason"`
}

var barGuardEnv = []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}

func barGuardGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	flags := []string{"-c", "user.name=Contract", "-c", "user.email=contract", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}
	if out, code := run(t, repo, barGuardEnv, "git", append(flags, args...)...); code != 0 {
		t.Fatalf("git %v (%d): %s", args, code, out)
	}
}

func barGuardWrite(t *testing.T, dir, path, body string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func barGuardRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	barGuardGit(t, repo, "init", "-q")
	for path, body := range files {
		barGuardWrite(t, repo, path, body)
	}
	barGuardGit(t, repo, "add", ".")
	barGuardGit(t, repo, "commit", "-q", "--allow-empty", "-m", "initial")
	return repo
}

func barGuardJSON(t *testing.T, bin, dir string, wantCode int, args ...string) []barGuardFinding {
	t.Helper()
	out, code := run(t, dir, barGuardEnv, bin, append([]string{"--json"}, args...)...)
	if code != wantCode {
		t.Fatalf("bar-guard %v exit %d, want %d:\n%s", args, code, wantCode, out)
	}
	var findings []barGuardFinding
	if err := json.Unmarshal([]byte(out), &findings); err != nil || findings == nil {
		t.Fatalf("expected a JSON array: %v\n%s", err, out)
	}
	return findings
}

func barGuardPatch(t *testing.T, path, before, after string) string {
	t.Helper()
	lines := func(body string) []string {
		if body == "" {
			return nil
		}
		return strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	}
	old, added := lines(before), lines(after)
	patch := fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ -1,%d +1,%d @@\n", path, path, len(old), len(added))
	for _, line := range old {
		patch += "-" + line + "\n"
	}
	for _, line := range added {
		patch += "+" + line + "\n"
	}
	dir := t.TempDir()
	barGuardWrite(t, dir, "change.diff", patch)
	return filepath.Join(dir, "change.diff")
}

func TestBarGuardContract(t *testing.T) {
	root := repoRoot(t)
	bin := filepath.Join(t.TempDir(), "bar-guard")
	if out, code := run(t, root, nil, "go", "build", "-o", bin, "./"+barGuardTool); code != 0 {
		t.Fatalf("build bar-guard (%d): %s", code, out)
	}

	t.Run("added patterns", func(t *testing.T) {
		cases := []struct{ kind, text string }{
			{"skipped-test", `t.Skip("later")`},
			{"skipped-test", `it.skip("later", check)`},
			{"skipped-test", `xit("later", check)`},
			{"skipped-test", `xdescribe("later", check)`},
			{"skipped-test", `describe.only("focus", check)`},
			{"skipped-test", `@pytest.mark.skip(reason="later")`},
			{"skipped-test", `@unittest.skip("later")`},
			{"skipped-test", `pytest.skip("later")`},
			{"skipped-test", `#[ignore]`},
			{"suppression", `//nolint:errcheck`},
			{"suppression", `/* eslint-disable no-console */`},
			{"suppression", `// @ts-ignore`},
			{"suppression", `// @ts-expect-error`},
			{"suppression", `value = call() # noqa`},
			{"suppression", `value = call() # type: ignore`},
			{"suppression", `value = call() // gitleaks:allow`},
			{"suppression", `#[allow(dead_code)]`},
			{"suppression", `// @ts-nocheck`},
			{"suppression", `/* istanbul ignore next */`},
			{"suppression", `run() # pragma: no cover`},
			{"empty-error-handler", `try { run() } catch {}`},
			{"empty-error-handler", `try { run() } catch (e) { }`},
			{"empty-error-handler", `except ValueError: pass`},
			{"empty-error-handler", `except: pass`},
			{"empty-error-handler", `if err != nil { }`},
		}
		for i, tc := range cases {
			t.Run(fmt.Sprintf("%s-%d", tc.kind, i), func(t *testing.T) {
				repo := barGuardRepo(t, map[string]string{"tests/check.txt": "baseline\n"})
				barGuardWrite(t, repo, "tests/check.txt", "baseline\n  "+tc.text+"  \n")
				got := barGuardJSON(t, bin, repo, 1)
				if len(got) != 1 || got[0].Path != "tests/check.txt" || got[0].Line != 2 || got[0].Kind != tc.kind || got[0].Text != tc.text || got[0].Waived {
					t.Errorf("finding = %+v", got)
				}
			})
		}
	})

	t.Run("deleted test paths", func(t *testing.T) {
		paths := []string{"check_test.go", "check.test.ts", "check.spec.jsx", "test_check.py", "check_test.py", "tests/check.rs", "pkg/__tests__/check.js", "empty test_test.go"}
		files := map[string]string{"source.go": "assert(ok)\n"}
		for _, path := range paths {
			files[path] = "assert(ok)\n"
		}
		files["empty test_test.go"] = ""
		repo := barGuardRepo(t, files)
		for path := range files {
			if err := os.Remove(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
				t.Fatal(err)
			}
		}
		got := barGuardJSON(t, bin, repo, 1)
		if len(got) != len(paths) {
			t.Fatalf("deleted files = %+v, want %d", got, len(paths))
		}
		seen := map[string]bool{}
		for _, finding := range got {
			if finding.Kind != "deleted-test-file" || finding.Line != 1 {
				t.Errorf("deletion = %+v", finding)
			}
			seen[finding.Path] = true
		}
		for _, path := range paths {
			if !seen[path] {
				t.Errorf("missing deletion of %s", path)
			}
		}
	})

	t.Run("assertion lines decrease across the whole file", func(t *testing.T) {
		checks := []string{"assert ok", "assert.Equal(t, got, want)", "expect(got).toBe(want)", "require.Equal(t, got, want)", "t.Error(got)", "t.Fatalf(\"bad\")", "t.Fail()", "got.should.equal(want)"}
		files := map[string]string{}
		for i, check := range checks {
			files[fmt.Sprintf("case%d_test.go", i)] = "before\n" + check + "\nafter\n"
		}
		repo := barGuardRepo(t, files)
		for path := range files {
			barGuardWrite(t, repo, path, "before\nafter\n")
		}
		got := barGuardJSON(t, bin, repo, 1)
		if len(got) != len(checks) {
			t.Fatalf("removed assertions = %+v", got)
		}
		for _, finding := range got {
			if finding.Kind != "assertion-removed" || finding.Line != 2 {
				t.Errorf("assertion = %+v", finding)
			}
		}
		patch := barGuardPatch(t, "check_test.go", "assert(ok)\nexpect(ok)\n", "require.True(t, ok)\n")
		if got := barGuardJSON(t, bin, t.TempDir(), 1, "--diff", patch); len(got) != 1 {
			t.Errorf("net decrease of one = %+v", got)
		}
		patch = barGuardPatch(t, "source.go", "assert(ok)\n", "work()\n")
		if got := barGuardJSON(t, bin, t.TempDir(), 0, "--diff", patch); len(got) != 0 {
			t.Errorf("non-test assertions = %+v", got)
		}
		patch = filepath.Join(t.TempDir(), "move.diff")
		barGuardWrite(t, filepath.Dir(patch), filepath.Base(patch), "--- a/check_test.go\n+++ b/check_test.go\n@@ -1 +1 @@\n-assert(ok)\n+setup()\n@@ -20 +20 @@\n-work()\n+expect(ok)\n")
		if got := barGuardJSON(t, bin, t.TempDir(), 0, "--diff", patch); len(got) != 0 {
			t.Errorf("assertion moved across hunks = %+v", got)
		}
	})

	t.Run("numeric thresholds stay narrow", func(t *testing.T) {
		cases := []struct {
			before, after string
			want          int
		}{
			{"threshold: 80", "threshold: 70", 1},
			{`"coverage": 0.8,`, `"coverage": 0.7,`, 1},
			{"min_coverage = 80", "min_coverage = 79", 1},
			{"maxWarnings: 1", "maxWarnings: 2", 1},
			{"max_issues: 0", "max_issues: 1", 1},
			{"limit: 10", "limit: 11", 1},
			{"timeout: 30", "timeout: 60", 1},
			{"threshold: 80", "threshold: 90", 0},
			{"coverage: 80", "coverage: 90", 0},
			{"minimum: 2", "minimum: 3", 0},
			{"max_issues: 2", "max_issues: 1", 0},
			{"limit: 10", "limit: 9", 0},
			{"timeout: 30", "timeout: 20", 0},
			{"coverage: 80", "coverage: 80", 0},
			{"retries: 2", "retries: 3", 0},
			{"coverage: 80", "threshold: 70", 0},
			{"// coverage: 80", "// coverage: 70", 0},
			{"coverage: compute(80)", "coverage: compute(70)", 0},
			{"coverage: 80 + offset", "coverage: 70 + offset", 0},
			{"coverage: 80%", "coverage: 0.7", 0},
			{"coverage: 80\ncoverage: 90", "coverage: 70\ncoverage: 95", 0},
		}
		for i, tc := range cases {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				patch := barGuardPatch(t, "checks.yml", tc.before+"\n", tc.after+"\n")
				got := barGuardJSON(t, bin, t.TempDir(), tc.want, "--diff", patch)
				if len(got) != tc.want || len(got) > 0 && (got[0].Kind != "threshold-loosened" || got[0].Text != tc.after) {
					t.Errorf("threshold %q -> %q: %+v", tc.before, tc.after, got)
				}
			})
		}
		patch := filepath.Join(t.TempDir(), "separate.diff")
		barGuardWrite(t, filepath.Dir(patch), filepath.Base(patch), "--- a/checks.yml\n+++ b/checks.yml\n@@ -1 +1 @@\n-coverage: 80\n+other: 1\n@@ -20 +20 @@\n-other: 2\n+coverage: 70\n")
		if got := barGuardJSON(t, bin, t.TempDir(), 0, "--diff", patch); len(got) != 0 {
			t.Errorf("thresholds paired across hunks: %+v", got)
		}
	})

	t.Run("tightening and assertion replacement stay clean", func(t *testing.T) {
		before := "t.Skip(\"later\")\n//nolint:errcheck\nassert(ok)\ncoverage: 80\nlimit: 10\n"
		after := "require.True(t, ok)\ncoverage: 90\nlimit: 9\n"
		repo := barGuardRepo(t, map[string]string{"check_test.go": before})
		barGuardWrite(t, repo, "check_test.go", after)
		if got := barGuardJSON(t, bin, repo, 0); len(got) != 0 {
			t.Errorf("tightening = %+v", got)
		}
		out, code := run(t, repo, barGuardEnv, bin)
		if code != 0 || out != "bar-guard: 0 findings (0 waived)\n" {
			t.Errorf("clean output (%d): %s", code, out)
		}
	})

	t.Run("staged unstaged and untracked from a subdirectory", func(t *testing.T) {
		repo := barGuardRepo(t, map[string]string{"staged_test.go": "baseline\n", "unstaged.go": "baseline\n"})
		barGuardWrite(t, repo, "staged_test.go", "baseline\nt.Skip(\"later\")\n")
		barGuardGit(t, repo, "add", "staged_test.go")
		barGuardWrite(t, repo, "unstaged.go", "baseline\n//nolint:errcheck\n")
		barGuardWrite(t, repo, "new test_test.go", "t.Skip(\"new\")\n")
		barGuardWrite(t, repo, "nested/keep.txt", "plain\n")
		got := barGuardJSON(t, bin, filepath.Join(repo, "nested"), 1)
		if len(got) != 3 || got[0].Path != "new test_test.go" || got[1].Path != "staged_test.go" || got[2].Path != "unstaged.go" {
			t.Fatalf("working tree findings = %+v", got)
		}
		out, code := run(t, filepath.Join(repo, "nested"), barGuardEnv, bin)
		want := "new test_test.go:1: skipped-test: t.Skip(\"new\")\n"
		if code != 1 || !strings.HasPrefix(out, want) || !strings.HasSuffix(out, "bar-guard: 3 findings (0 waived)\n") {
			t.Errorf("text output (%d): %s", code, out)
		}
	})

	t.Run("base uses the merge base", func(t *testing.T) {
		repo := barGuardRepo(t, nil)
		barGuardGit(t, repo, "branch", "upstream")
		barGuardWrite(t, repo, "feature_test.go", "t.Skip(\"feature\")\n")
		barGuardGit(t, repo, "add", ".")
		barGuardGit(t, repo, "commit", "-q", "-m", "feature")
		barGuardGit(t, repo, "checkout", "-q", "upstream")
		barGuardWrite(t, repo, "upstream_test.go", "assert(ok)\n")
		barGuardGit(t, repo, "add", ".")
		barGuardGit(t, repo, "commit", "-q", "-m", "upstream")
		barGuardGit(t, repo, "checkout", "-q", "main")
		if got := barGuardJSON(t, bin, repo, 0); len(got) != 0 {
			t.Errorf("default HEAD includes committed changes: %+v", got)
		}
		got := barGuardJSON(t, bin, repo, 1, "--base", "upstream")
		if len(got) != 1 || got[0].Path != "feature_test.go" || got[0].Kind != "skipped-test" {
			t.Errorf("merge-base findings = %+v", got)
		}
	})

	t.Run("waivers need a reason and remain visible", func(t *testing.T) {
		for _, tc := range []struct {
			marker string
			waived bool
		}{
			{"bar-guard:allow upstream service unavailable", true},
			{"bar-guard:allow", false},
		} {
			repo := barGuardRepo(t, nil)
			barGuardWrite(t, repo, "check_test.go", "t.Skip(\"later\") // "+tc.marker+"\n")
			want := 1
			if tc.waived {
				want = 0
			}
			got := barGuardJSON(t, bin, repo, want)
			if len(got) != 1 || got[0].Waived != tc.waived || tc.waived && got[0].Reason != "upstream service unavailable" {
				t.Errorf("waiver = %+v", got)
			}
			out, code := run(t, repo, barGuardEnv, bin)
			if code != want || tc.waived && !strings.Contains(out, "skipped-test (waived):") {
				t.Errorf("waived text (%d): %s", code, out)
			}
		}
		repo := barGuardRepo(t, nil)
		barGuardWrite(t, repo, "check_test.go", "t.Skip(\"one\") // bar-guard:allow known issue\nt.Skip(\"two\")\n")
		if got := barGuardJSON(t, bin, repo, 1); len(got) != 2 {
			t.Errorf("waiver hid another finding: %+v", got)
		}
	})

	t.Run("patch parsing and bounded source text", func(t *testing.T) {
		patch := filepath.Join(t.TempDir(), "change.diff")
		body := "diff --git a/check_test.go b/check_test.go\n--- a/check_test.go\n+++ b/check_test.go\n@@ -10,1 +10,2 @@\n--- old content\n+++new content\n+t.Skip(\"later\")\n\\ No newline at end of file\n"
		barGuardWrite(t, filepath.Dir(patch), filepath.Base(patch), body)
		got := barGuardJSON(t, bin, t.TempDir(), 1, "--diff", patch)
		if len(got) != 1 || got[0].Path != "check_test.go" || got[0].Line != 11 {
			t.Errorf("patch line numbers = %+v", got)
		}
		patch = barGuardPatch(t, "check_test.go", "", "t.Skip(\""+strings.Repeat("界", 150)+"\")\n")
		got = barGuardJSON(t, bin, t.TempDir(), 1, "--diff", patch)
		if len(got) != 1 || utf8.RuneCountInString(got[0].Text) != 120 || !utf8.ValidString(got[0].Text) {
			t.Errorf("source truncation = %+v", got)
		}
		out, code := run(t, root, nil, "go", "run", "./"+barGuardTool, "--diff", patch)
		if code == 0 || !strings.Contains(out, "skipped-test:") {
			t.Errorf("go run (%d): %s", code, out)
		}
		repo := barGuardRepo(t, map[string]string{"quoted\tname_test.go": "baseline\n"})
		barGuardWrite(t, repo, "quoted\tname_test.go", "baseline\nt.Skip(\"later\")\n")
		got = barGuardJSON(t, bin, repo, 1)
		if len(got) != 1 || got[0].Path != "quoted\tname_test.go" || got[0].Line != 2 {
			t.Errorf("quoted git path = %+v", got)
		}
	})

	t.Run("tool errors cannot look clean", func(t *testing.T) {
		outside := t.TempDir()
		env := append(append([]string{}, barGuardEnv...), "GIT_CEILING_DIRECTORIES="+filepath.Dir(outside))
		out, code := run(t, outside, env, bin)
		if code != 2 || !strings.Contains(out, "bar-guard:") || strings.Contains(out, "0 findings") {
			t.Errorf("outside work tree (%d): %s", code, out)
		}
		repo := barGuardRepo(t, nil)
		out, code = run(t, repo, append(append([]string{}, barGuardEnv...), "PATH="+t.TempDir()), bin)
		if code != 2 || strings.Contains(out, "0 findings") {
			t.Errorf("git unavailable (%d): %s", code, out)
		}
		badPatch := filepath.Join(t.TempDir(), "bad.diff")
		barGuardWrite(t, filepath.Dir(badPatch), filepath.Base(badPatch), "--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n")
		garbage := filepath.Join(t.TempDir(), "garbage.diff")
		barGuardWrite(t, filepath.Dir(garbage), filepath.Base(garbage), "this is not a unified diff\n")
		for name, args := range map[string][]string{
			"bad base":      {"--base", "no-such-ref"},
			"bad flag":      {"--unknown"},
			"extra arg":     {"extra"},
			"missing patch": {"--diff", filepath.Join(outside, "missing.diff")},
			"broken patch":  {"--diff", badPatch},
			"garbage patch": {"--diff", garbage},
		} {
			out, code := run(t, repo, barGuardEnv, bin, args...)
			if code != 2 || strings.Contains(out, "0 findings") {
				t.Errorf("%s (%d): %s", name, code, out)
			}
		}
		out, code = run(t, outside, env, bin, "--help")
		if code != 0 || !strings.Contains(out, "origin/main") || !strings.Contains(out, "merge base") {
			t.Errorf("help (%d): %s", code, out)
		}
	})
}

func TestBarGuardCorrections(t *testing.T) {
	root := repoRoot(t)
	bin := filepath.Join(t.TempDir(), "bar-guard")
	if out, code := run(t, root, nil, "go", "build", "-o", bin, "./"+barGuardTool); code != 0 {
		t.Fatalf("build bar-guard (%d): %s", code, out)
	}
	patchFile := func(t *testing.T, body string) string {
		t.Helper()
		patch := filepath.Join(t.TempDir(), "change.diff")
		barGuardWrite(t, filepath.Dir(patch), filepath.Base(patch), body)
		return patch
	}

	t.Run("header without a hunk is malformed", func(t *testing.T) {
		for _, body := range []string{
			"--- a/x_test.go\n+++ b/x_test.go\n",
			"diff --git a/x_test.go b/x_test.go\nindex 1234567..7654321 100644\n",
			"diff --git a/x_test.go b/x_test.go\nold mode 100644\nnew mode 100755\n--- a/x_test.go\n+++ b/x_test.go\n",
			"diff --git a/x.go b/x.go\nnew mode 100755\n",
			"diff --git a/x.go b/y.go\nrename to y.go\n",
			"diff --git a/x.go b/x.go\nnew file mode 100644\nindex 0000000..1234567\n",
			"diff --git a/x.go b/x.go\nnew file mode 100644\n",
			"diff --git a/x.go b/x.go\nold mode 100644\nnew mode 100755\nindex 1234567..7654321\n",
			"diff --git a/x.go b/y.go\nsimilarity index 80%\nrename from x.go\nrename to y.go\n",
			"--- a/x_test.go\n+++ b/x_test.go\n--- a/y.go\n+++ b/y.go\n@@ -1 +1 @@\n-old\n+new\n",
		} {
			out, code := run(t, t.TempDir(), barGuardEnv, bin, "--diff", patchFile(t, body))
			if code != 2 || strings.Contains(out, "0 findings") {
				t.Errorf("truncated diff exit %d, want 2: %s", code, out)
			}
		}
	})

	t.Run("legitimate metadata stays clean", func(t *testing.T) {
		for _, body := range []string{
			"diff --git a/old.go b/new.go\nsimilarity index 100%\nrename from old.go\nrename to new.go\n",
			"diff --git a/run.sh b/run.sh\nold mode 100644\nnew mode 100755\n",
			"diff --git a/data.bin b/data.bin\nindex 1234567..7654321 100644\nBinary files a/data.bin and b/data.bin differ\n",
			"diff --git a/empty.go b/empty.go\nnew file mode 100644\nindex 0000000..e69de29\n",
		} {
			if got := barGuardJSON(t, bin, t.TempDir(), 0, "--diff", patchFile(t, body)); len(got) != 0 {
				t.Errorf("metadata findings = %+v", got)
			}
		}
	})

	t.Run("index weakening survives an unstaged reversal", func(t *testing.T) {
		repo := barGuardRepo(t, map[string]string{"check_test.go": "baseline\n"})
		barGuardWrite(t, repo, "check_test.go", "baseline\nt.Skip(\"later\")\n")
		barGuardGit(t, repo, "add", "check_test.go")
		barGuardWrite(t, repo, "check_test.go", "baseline\n")
		got := barGuardJSON(t, bin, repo, 1)
		if len(got) != 1 || got[0].Kind != "skipped-test" || got[0].Line != 2 {
			t.Errorf("index findings = %+v", got)
		}
		barGuardWrite(t, repo, "check_test.go", "baseline\nt.Skip(\"later\")\n//nolint:errcheck\n")
		got = barGuardJSON(t, bin, repo, 1)
		if len(got) != 2 {
			t.Errorf("duplicate findings across index and work tree = %+v", got)
		}
	})

	t.Run("removing the only error handler statement", func(t *testing.T) {
		for _, tc := range []struct{ path, before, after string }{
			{"source.go", "if err != nil {\n\treturn err\n}\n", "if err != nil {\n}\n"},
			{"source.js", "try { work() } catch (err) {\n  throw err\n}\n", "try { work() } catch (err) {\n}\n"},
			{"source.ts", "catch (err) {\n  throw err\n}\n", "catch (err) {\n}\n"},
			{"source.py", "try:\n    work()\nexcept ValueError:\n    raise\nnext_call()\n", "try:\n    work()\nexcept ValueError:\nnext_call()\n"},
			{"last.py", "except Exception:\n    raise\n", "except Exception:\n"},
			{"blank.go", "if err != nil {\n\n\n\n\n\treturn err\n\n\n\n\n}\n", "if err != nil {\n\n\n\n\n\n\n\n\n}\n"},
			{"multi.go", "if err != nil {\n\treport(err)\n\treturn err\n}\n", "if err != nil {\n}\n"},
		} {
			t.Run(tc.path, func(t *testing.T) {
				repo := barGuardRepo(t, map[string]string{tc.path: tc.before})
				barGuardWrite(t, repo, tc.path, tc.after)
				got := barGuardJSON(t, bin, repo, 1)
				if len(got) != 1 || got[0].Kind != "empty-error-handler" {
					t.Errorf("empty handler = %+v", got)
				}
				patch := barGuardPatch(t, tc.path, tc.before, tc.after)
				if got := barGuardJSON(t, bin, t.TempDir(), 1, "--diff", patch); len(got) != 1 || got[0].Kind != "empty-error-handler" {
					t.Errorf("patch empty handler = %+v", got)
				}
			})
		}
		repo := barGuardRepo(t, map[string]string{"staged.go": "if err != nil {\n\treturn err\n}\n"})
		barGuardWrite(t, repo, "staged.go", "if err != nil {\n}\n")
		barGuardGit(t, repo, "add", "staged.go")
		barGuardWrite(t, repo, "staged.go", "if err != nil {\n\treturn err\n}\n")
		if got := barGuardJSON(t, bin, repo, 1); len(got) != 1 || got[0].Kind != "empty-error-handler" {
			t.Errorf("staged empty handler = %+v", got)
		}
	})

	t.Run("additional skip and suppression APIs", func(t *testing.T) {
		for _, tc := range []struct{ kind, text string }{
			{"skipped-test", `t.Skipf("later: %s", reason)`},
			{"skipped-test", `t.SkipNow()`},
			{"skipped-test", `@pytest.mark.skipif(condition, reason="later")`},
			{"skipped-test", `@pytest.mark.xfail`},
			{"skipped-test", `pytest.mark.xfail(reason="later")`},
			{"skipped-test", `it.todo("later")`},
			{"skipped-test", `test.skip("later", check)`},
			{"skipped-test", `describe.skip("later", check)`},
			{"suppression", `//lint:ignore SA1000 known issue`},
			{"suppression", `// #nosec G104`},
			{"suppression", `work() // nosemgrep`},
			{"suppression", `#[allow(clippy::unwrap_used)]`},
			{"suppression", `// eslint-disable-next-line no-console`},
			{"suppression", `@SuppressWarnings("unchecked")`},
			{"suppression", `# pylint: disable=unused-variable`},
			{"suppression", `# pyright: ignore[reportUnknownVariableType]`},
		} {
			t.Run(tc.text, func(t *testing.T) {
				patch := barGuardPatch(t, "tests/check.txt", "", tc.text+"\n")
				got := barGuardJSON(t, bin, t.TempDir(), 1, "--diff", patch)
				if len(got) != 1 || got[0].Kind != tc.kind {
					t.Errorf("additional API = %+v", got)
				}
			})
		}
	})

	t.Run("waived emptied handler remains visible", func(t *testing.T) {
		patch := barGuardPatch(t, "source.go", "if err != nil {\n\treturn err\n}\n", "if err != nil { // bar-guard:allow documented fallback\n}\n")
		got := barGuardJSON(t, bin, t.TempDir(), 0, "--diff", patch)
		if len(got) != 1 || got[0].Kind != "empty-error-handler" || !got[0].Waived || got[0].Reason != "documented fallback" {
			t.Errorf("waived handler = %+v", got)
		}
	})

	t.Run("individual tests removed", func(t *testing.T) {
		for _, tc := range []struct{ path, declaration string }{
			{"check_test.go", "func TestGone(t *testing.T) {\n}\n"},
			{"test_check.py", "def test_gone():\n    work()\n"},
			{"check.test.js", "it('gone', () => {\n});\n"},
			{"check.spec.ts", "test(\"gone\", () => {\n});\n"},
			{"src/lib.rs", "#[test]\nfn gone() {\n}\n"},
		} {
			t.Run(tc.path, func(t *testing.T) {
				patch := barGuardPatch(t, tc.path, tc.declaration+"keep()\n", "keep()\n")
				got := barGuardJSON(t, bin, t.TempDir(), 1, "--diff", patch)
				if len(got) != 1 || got[0].Kind != "test-removed" {
					t.Errorf("removed test = %+v", got)
				}
				patch = barGuardPatch(t, tc.path, tc.declaration+"keep()\n", "keep()\n"+tc.declaration)
				if got := barGuardJSON(t, bin, t.TempDir(), 0, "--diff", patch); len(got) != 0 {
					t.Errorf("moved test = %+v", got)
				}
			})
		}
		body := "--- a/a_test.go\n+++ b/a_test.go\n@@ -1 +0,0 @@\n-func TestMoved(t *testing.T) {}\n--- a/b_test.go\n+++ b/b_test.go\n@@ -0,0 +1 @@\n+func TestMoved(t *testing.T) {}\n"
		if got := barGuardJSON(t, bin, t.TempDir(), 0, "--diff", patchFile(t, body)); len(got) != 0 {
			t.Errorf("test moved across files = %+v", got)
		}
		body = "--- a/src/lib.rs\n+++ b/src/lib.rs\n@@ -1,3 +1,2 @@\n-#[test]\n fn gone() {\n }\n"
		if got := barGuardJSON(t, bin, t.TempDir(), 1, "--diff", patchFile(t, body)); len(got) != 1 || got[0].Kind != "test-removed" {
			t.Errorf("removed Rust test attribute = %+v", got)
		}
	})

	t.Run("additional test locations", func(t *testing.T) {
		for _, tc := range []struct{ path, marker string }{
			{"src/test/check.go", ""}, {"pkg/test/check.py", ""},
			{"src/WidgetTest.java", ""}, {"WidgetTests.java", ""},
			{"src/lib.rs", "#[cfg(test)]\n"}, {"src/other.rs", "#[test]\n"},
		} {
			t.Run(tc.path, func(t *testing.T) {
				for _, after := range []string{tc.marker + "work()\n", "work()\n"} {
					patch := barGuardPatch(t, tc.path, tc.marker+"assert(ok)\n", after)
					got := barGuardJSON(t, bin, t.TempDir(), 1, "--diff", patch)
					if len(got) != 1 || got[0].Kind != "assertion-removed" {
						t.Errorf("test location = %+v", got)
					}
				}
			})
		}
		patch := barGuardPatch(t, "pkg/test/check.py", "def test_gone():\n    work()\nkeep()\n", "keep()\n")
		if got := barGuardJSON(t, bin, t.TempDir(), 1, "--diff", patch); len(got) != 1 || got[0].Kind != "test-removed" {
			t.Errorf("test declaration in test directory = %+v", got)
		}
		patch = barGuardPatch(t, "src/lib.rs", "#[cfg(test)]\n#[test]\nfn gone() {}\nkeep()\n", "keep()\n")
		if got := barGuardJSON(t, bin, t.TempDir(), 1, "--diff", patch); len(got) != 1 || got[0].Kind != "test-removed" {
			t.Errorf("Rust test declaration = %+v", got)
		}
	})

	t.Run("numeric overflow stays visible", func(t *testing.T) {
		overflow := strings.Repeat("9", 400)
		for _, tc := range []struct{ before, after string }{
			{"coverage: 80", "coverage: " + overflow},
			{"coverage: " + overflow, "coverage: 80"},
			{"", "coverage: " + overflow},
		} {
			patch := barGuardPatch(t, "checks.yml", tc.before+"\n", tc.after+"\n")
			got := barGuardJSON(t, bin, t.TempDir(), 1, "--diff", patch)
			if len(got) != 1 || got[0].Kind != "threshold-changed" || !strings.Contains(got[0].Text, "unverifiable") {
				t.Errorf("unverifiable threshold = %+v", got)
			}
		}
	})

	t.Run("ordinary changes stay clean", func(t *testing.T) {
		for _, tc := range []struct{ path, before, after string }{
			{"source.go", "if err != nil {\n\tlog(err)\n\treturn err\n}\n", "if err != nil {\n\treturn err\n}\n"},
			{"source.go", "if err != nil {\n\treturn err\n}\n", "if err != nil {\n\tpanic(err)\n}\n"},
			{"source.py", "except Exception:\n    log(err)\n    raise\n", "except Exception:\n    raise\n"},
			{"source.go", "if ready {\n\twork()\n}\n", "if ready {\n}\n"},
			{"source.js", "function test(value) {\n  work()\n}\n", "work()\n"},
			{"source.py", "def test_helper():\n    work()\n", "work()\n"},
			{"check_test.go", "func TestKeep(t *testing.T) {\n\ttest(\"value\", check)\n}\n", "func TestKeep(t *testing.T) {\n\twork()\n}\n"},
			{"test_check.py", "def test_keep():\n    test(\"value\", check)\n", "def test_keep():\n    work()\n"},
			{"src/lib.rs", "fn helper() {}\n", "fn helper2() {}\n"},
			{"checks.yml", "retries: 3\n", "retries: " + strings.Repeat("9", 400) + "\n"},
			{"source.go", "", "t.Skipper()\n// lint:ignored\n"},
		} {
			patch := barGuardPatch(t, tc.path, tc.before, tc.after)
			if got := barGuardJSON(t, bin, t.TempDir(), 0, "--diff", patch); len(got) != 0 {
				t.Errorf("ordinary change in %s = %+v", tc.path, got)
			}
		}
	})
}

// Review round 2: an assertion removed inside an existing Rust test, whose
// #[test] attribute is unchanged context, must still count as a test change.
func TestBarGuardRustContextTest(t *testing.T) {
	root := repoRoot(t)
	bin := filepath.Join(t.TempDir(), "bar-guard")
	if out, code := run(t, root, nil, "go", "build", "-o", bin, "./"+barGuardTool); code != 0 {
		t.Fatalf("build bar-guard (%d): %s", code, out)
	}
	before := "pub fn is_valid(n: i32) -> bool { n >= 0 }\n\n#[test]\nfn rejects_invalid() {\n    assert!(!is_valid(-1));\n}\n"
	after := "pub fn is_valid(n: i32) -> bool { n >= 0 }\n\n#[test]\nfn rejects_invalid() {\n}\n"
	repo := barGuardRepo(t, map[string]string{"src/lib.rs": before})
	barGuardWrite(t, repo, "src/lib.rs", after)
	got := barGuardJSON(t, bin, repo, 1)
	if len(got) == 0 || got[0].Kind != "assertion-removed" {
		t.Errorf("rust context test findings = %+v", got)
	}
}
