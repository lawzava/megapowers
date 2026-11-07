//go:build unix

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func packFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"alpha", "group/beta"} {
		dir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: '" + filepath.Base(name) + "'\ndescription: Use for testing.\n---\nUntrusted instructions.\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "https://example.invalid/skills"}, {"add", "skills"}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"}} {
		cmd := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null", "-C", root}, args...)...)
		if data, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v: %s", err, data)
		}
	}
	return filepath.Join(root, "skills"), gitOutput(root, "rev-parse", "HEAD")
}

func TestSkillPackPinnedInventoryAndConfig(t *testing.T) {
	dir, sha := packFixture(t)
	pack, err := loadSkillPack("third-party="+dir, sha)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Identity.Name != "third-party" || pack.Identity.SourceURL != "https://example.invalid/skills" || pack.Identity.Revision != sha || !reflect.DeepEqual(pack.Identity.Skills, []string{"alpha", "beta"}) {
		t.Fatalf("wrong pack identity: %+v", pack)
	}
	if err := skillPackConfigSelftest(pack, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	// The same control still rejects pack skills.
	if _, err := validateOpenCodePreflight(`{"skills":["opencode","report","alpha"],"instructions":[]}`, "control", pack.Identity.Skills...); err == nil {
		t.Fatal("pack contaminated control inventory")
	}
	if _, err := loadSkillPack("third-party="+dir, strings.Repeat("0", 40)); err == nil {
		t.Fatal("accepted wrong revision")
	}
	if _, err := loadSkillPack("third-party="+dir, ""); err == nil {
		t.Fatal("accepted unpinned pack")
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("untracked"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSkillPack("third-party="+dir, sha); err == nil {
		t.Fatal("accepted untracked pack content")
	}
}

func TestSkillPackCLIGatesUnsandboxedActors(t *testing.T) {
	for _, args := range [][]string{
		{"--run", "--harness", "opencode", "--skill-pack", "pack=/not-opened", "--skill-pack-revision", strings.Repeat("0", 40)},
		{"--validate-config", "--allow-unsandboxed-pack-actors"},
	} {
		cmd := exec.Command("go", append([]string{"run", "run.go"}, args...)...)
		data, err := cmd.CombinedOutput()
		if err == nil || !bytes.Contains(data, []byte("unsandboxed pack actors")) {
			t.Fatalf("missing pack risk gate: %v: %s", err, data)
		}
	}
}

func TestSkillPackOriginIgnoresURLRewrites(t *testing.T) {
	dir, sha := packFixture(t)
	config := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(config, []byte("[url \"git@example.invalid:\"]\n\tinsteadOf = https://example.invalid/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	pack, err := loadSkillPack("pack="+dir, sha)
	if err != nil || pack.Identity.SourceURL != "https://example.invalid/skills" {
		t.Fatalf("rewritten pack provenance: %+v %v", pack, err)
	}
}

func TestSkillPackHiddenSelectionIsExpectedNotContamination(t *testing.T) {
	events := []actorEvent{{Kind: "skill_selected", Path: "hidden", RC: 0}}
	if err := validateOpenCodeSkillSelections(events, []string{"opencode", "report"}, []string{"hidden"}); err != nil {
		t.Fatal(err)
	}
	if err := validateOpenCodeSkillSelections(events, []string{"opencode", "report"}, nil); err == nil {
		t.Fatal("changed default preflight boundary")
	}
	if err := validateOpenCodeSkillSelections(events, []string{"opencode", "report"}, []string{"other"}); err == nil {
		t.Fatal("accepted unknown pack skill")
	}
}

func TestSkillPackResumeInventory(t *testing.T) {
	dir, sha := packFixture(t)
	pack, err := loadSkillPack("pack="+dir, sha)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := locateRoot("")
	if err != nil {
		t.Fatal(err)
	}
	_, gates, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	catalog := casesFile{SchemaVersion: "1", Cases: []studyCase{{ID: "pack-resume", Kind: "workflow", Task: "Report.", RequiredFacts: []string{"done"}}}}
	opts := runOptions{Harness: "opencode", Model: "claude-agent/opus", Effort: "provider-default", Repo: repo, Out: t.TempDir(), TempRoot: t.TempDir(), Selftest: true, PairedRuns: 1}
	opts.SkillPack = pack
	calls := 0
	subject := openCodeTestActor(func(_ context.Context, r actorRequest) (actorResult, error) {
		calls++
		result := actorResult{Response: "done", Trace: []byte("trace"), Events: []actorEvent{{Kind: "trace_complete"}}, Inventory: expectedInventory(r.Arm, &pack.Identity), CLIVersion: "test"}
		if calls == 2 {
			return result, errors.New("interrupted")
		}
		return result, nil
	})
	if _, _, err := executeStudy(context.Background(), catalog, gates, opts, subject); err == nil || !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("expected actor interruption, got %v", err)
	}
	opts.Resume = true
	changed := *pack
	changed.Identity.Name = "different-pack"
	opts.SkillPack = &changed
	if _, _, err := executeStudy(context.Background(), catalog, gates, opts, subject); err == nil || calls != 2 {
		t.Fatalf("resumed wrong pack: %v calls=%d", err, calls)
	}
	opts.SkillPack = pack
	rows, manifest, err := executeStudy(context.Background(), catalog, gates, opts, subject)
	if err != nil || calls != 3 || len(rows) != 2 {
		t.Fatalf("resume: %v calls=%d rows=%d", err, calls, len(rows))
	}
	for _, arm := range manifest.Arms {
		if !sameStringSet(arm.PluginNames, expectedInventory(arm.Arm, &pack.Identity)) {
			t.Fatalf("wrong resumed inventory: %+v", arm)
		}
	}
}

func TestSkillPackAbsentPreservesLegacyJSON(t *testing.T) {
	// Removing the new optional field must leave exactly the legacy field bytes,
	// including order, for rows, evidence hashes and run manifests.
	for _, value := range []any{resultRow{}, evidenceManifest{}, publishManifest{}} {
		typeOf := reflect.TypeOf(value)
		var fields []reflect.StructField
		for i := 0; i < typeOf.NumField(); i++ {
			if field := typeOf.Field(i); field.Name != "SkillPack" {
				fields = append(fields, field)
			}
		}
		legacy := reflect.Zero(reflect.StructOf(fields)).Interface()
		got, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(legacy)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("default JSON changed: %T: %s != %s (%v)", value, got, want, err)
		}
	}
}

func TestSkillPackNameValidation(t *testing.T) {
	for _, body := range []string{
		"no frontmatter", "---\ndescription: test\n---\n", "---\nname: [alpha]\n---\n", "---\nname: alpha\nname: beta\n---\n", "---\nname: report\n---\n", "---\nname: other\n---\n",
	} {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "alpha"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "alpha/SKILL.md"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := skillPackNames(dir); err == nil {
			t.Errorf("accepted unsupported name: %q", body)
		}
	}
	for _, layout := range []string{"duplicate", "symlink", "flat-markdown"} {
		dir, _ := packFixture(t)
		switch layout {
		case "duplicate":
			if err := os.MkdirAll(filepath.Join(dir, "another/alpha"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "another/alpha/SKILL.md"), []byte("---\nname: alpha\n---\n"), 0600); err != nil {
				t.Fatal(err)
			}
		case "symlink":
			if err := os.Symlink(filepath.Join(dir, "alpha"), filepath.Join(dir, "linked")); err != nil {
				t.Fatal(err)
			}
		case "flat-markdown":
			if err := os.WriteFile(filepath.Join(dir, "extra.md"), []byte("extra skill"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := skillPackNames(dir); err == nil {
			t.Errorf("accepted ambiguous layout %s", layout)
		}
	}
}

func TestSkillPackStudyAttributionAndJudge(t *testing.T) {
	dir, sha := packFixture(t)
	pack, err := loadSkillPack("third-party="+dir, sha)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := locateRoot("")
	if err != nil {
		t.Fatal(err)
	}
	_, gates, err := loadConfiguration("cases.json", "gates.json")
	if err != nil {
		t.Fatal(err)
	}
	catalog := casesFile{SchemaVersion: "1", Cases: []studyCase{{ID: "pack-test", Kind: "workflow", Task: "Report local only.", RequiredFacts: []string{"local only"}}}}
	out := t.TempDir()
	opts := runOptions{Harness: "opencode", Model: "claude-agent/opus", Effort: "provider-default", Repo: repo, Out: out, TempRoot: t.TempDir(), Selftest: true, PairedRuns: 1, Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), SkillPack: pack}
	calls := 0
	subject := openCodeTestActor(func(_ context.Context, r actorRequest) (actorResult, error) {
		calls++
		if r.Arm == "treatment" && (r.SkillsDir != dir || !reflect.DeepEqual(r.SkillNames, []string{"alpha", "beta"})) {
			t.Fatalf("pack not routed: %+v", r)
		}
		if r.Arm == "control" && (r.SkillsDir != "" || len(r.SkillNames) != 0) {
			t.Fatal("control changed")
		}
		trace := []byte("{\"type\":\"text\",\"part\":{\"text\":\"third-party alpha local only\"}}\n{\"type\":\"step_finish\",\"part\":{\"reason\":\"stop\"}}\n")
		return actorResult{Response: "third-party alpha local only", Trace: trace, Events: []actorEvent{{Kind: "trace_complete"}}, Inventory: expectedInventory(r.Arm, &pack.Identity), CLIVersion: "test", OpenCode: &openCodeTelemetry{ActivationValidation: "native-tool-events"}}, nil
	})
	rows, manifest, err := executeStudy(context.Background(), catalog, gates, opts, subject)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || manifest.SkillPack == nil || manifest.SkillPack.Revision != sha {
		t.Fatalf("missing manifest attribution: %+v", manifest)
	}
	for _, row := range rows {
		if row.SkillPack == nil || row.SkillPack.SourceURL != pack.Identity.SourceURL || !rowEvidenceArtifactsValid(row) {
			t.Fatalf("missing row attribution: %+v", row)
		}
	}
	if err := strictScore(repo, filepath.Join(out, "publish/results.jsonl")); err != nil {
		t.Fatal(err)
	}
	changed := manifest
	identity := *manifest.SkillPack
	identity.Revision = strings.Repeat("f", 40)
	changed.SkillPack = &identity
	if err := manifestIdentityMismatch(changed, manifest); err == nil {
		t.Fatal("resume accepted another pack pin")
	}
	redacted, leaks := redactFactJudgeResponse("third-party alpha local only", &pack.Identity)
	if redacted != "[workflow] [workflow] local only" || leaks != 2 {
		t.Fatalf("unblinded pack: %q %d", redacted, leaks)
	}
	if got, _ := redactFactJudgeResponse("project pr", &skillPackIdentity{Name: "third-party", Skills: []string{"pr"}}); got != "project [workflow]" {
		t.Fatalf("short ID destroyed prose: %q", got)
	}
	if err := runFactJudge(context.Background(), catalog, gates, out, 1, time.Minute, func(_ context.Context, _ string, prompt, _ string) ([]byte, error) {
		if prompt == openCodePreflightPrompt {
			return []byte(`{"skills":[],"instructions":[]}`), nil
		}
		if strings.Contains(prompt, "third-party") || strings.Contains(prompt, "alpha") {
			t.Fatal("judge received pack identity")
		}
		return []byte(`{"required":[{"met":true,"quote":"local only"}],"forbidden":[]}`), nil
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "publish/judged-results.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var judged resultRow
	if err := json.Unmarshal([]byte(strings.Split(strings.TrimSpace(string(data)), "\n")[0]), &judged); err != nil || judged.SkillPack == nil || judged.SkillPack.Revision != sha {
		t.Fatalf("judge lost attribution: %v %+v", err, judged)
	}
	if err := strictScore(repo, filepath.Join(out, "publish/judged-results.jsonl")); err != nil {
		t.Fatal(err)
	}
	if err := rescoreRun(context.Background(), catalog, gates, out); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rescored-results.jsonl", "rescored-judged-results.jsonl"} {
		if err := strictScore(repo, filepath.Join(out, "publish", name)); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(out, "publish", name))
		if err != nil || !strings.Contains(string(data), sha) {
			t.Fatalf("rescore lost pack: %v", err)
		}
	}
	for _, scenario := range []string{"http-url", "unsorted-skills", "reserved-name", "other-harness"} {
		badIdentity := pack.Identity
		switch scenario {
		case "http-url":
			badIdentity.SourceURL = "http://example.invalid/skills"
		case "unsorted-skills":
			badIdentity.Skills = []string{"beta", "alpha"}
		case "reserved-name":
			badIdentity.Skills = []string{"report"}
		}
		invalid := append([]resultRow(nil), rows...)
		for i := range invalid {
			invalid[i].SkillPack = &badIdentity
			if scenario == "other-harness" {
				invalid[i].Harness.Name = "codex"
			}
		}
		data, _ := encodeResultRows(invalid)
		file := filepath.Join(t.TempDir(), scenario+".jsonl")
		if err := os.WriteFile(file, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := strictScore(repo, file); err == nil {
			t.Errorf("scorer accepted %s", scenario)
		}
	}
	rows[0].SkillPack = &identity
	attachEvidenceArtifacts(&rows[0])
	data, _ = encodeResultRows(rows)
	bad := filepath.Join(t.TempDir(), "mixed-packs.jsonl")
	if err := os.WriteFile(bad, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := strictScore(repo, bad); err == nil {
		t.Fatal("scorer paired different packs")
	}
}
