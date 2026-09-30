//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeCodexInvocationPolicy(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plugins", "megapowers", "skills", "manual", "agents")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "..", "SKILL.md"), []byte("---\nname: manual\n---\nBody\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openai.yaml"), []byte("policy:\n  allow_implicit_invocation: false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := loadUnselectableSkills(root, map[string]bool{"manual": true}, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if !got["manual"] {
		t.Fatal("Codex native explicit-only policy was ignored")
	}
	got, err = loadUnselectableSkills(root, map[string]bool{"manual": true}, "claude")
	if err != nil || got["manual"] {
		t.Fatalf("Claude must not use Codex policy: %v, %v", got, err)
	}
	if err := os.Remove(filepath.Join(dir, "openai.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "..", "SKILL.md"), []byte("---\nname: manual\ndisable-model-invocation: true\n---\nBody\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err = loadUnselectableSkills(root, map[string]bool{"manual": true}, "codex")
	if err != nil || got["manual"] {
		t.Fatalf("Codex must not use Claude policy: %v, %v", got, err)
	}
	got, err = loadUnselectableSkills(root, map[string]bool{"manual": true}, "claude")
	if err != nil || !got["manual"] {
		t.Fatalf("Claude policy missing: %v, %v", got, err)
	}
}

func TestCodexPolicyMapping(t *testing.T) {
	for _, tc := range []struct {
		text              string
		disabled, invalid bool
	}{
		{"interface:\n  display_name: Test\npolicy:\n  allow_implicit_invocation: false # explicit\n", true, false},
		{"policy:\n  allow_implicit_invocation: true\n", false, false},
		{"interface:\n  allow_implicit_invocation: false\n", false, false},
		{"policy: {allow_implicit_invocation: false}\n", false, true},
		{"policy:\n  allow_implicit_invocation: false\n  allow_implicit_invocation: true\n", false, true},
		{"policy:\n  allow_implicit_invocation: \"false\"\n", false, true},
	} {
		got, err := codexExplicitOnly(tc.text)
		if (err != nil) != tc.invalid || (err == nil && got != tc.disabled) {
			t.Errorf("%q: %v, %v", tc.text, got, err)
		}
	}
}

func TestAmbiguousJSONConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cases.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"1","schema_version":"2","cases":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	var cases casesFile
	if err := decodeStrict(path, &cases); err == nil {
		t.Fatal("duplicate schema_version was accepted")
	}
}

func TestExplicitInvocationIsNotImplicitRecall(t *testing.T) {
	metrics, verdict := evaluateProbe(probeCase{Kind: "explicit", Expected: "manual"}, []actorEvent{{Kind: "skill_selected", Path: "manual", RC: 0}}, map[string]bool{"manual": true})
	if verdict != "pass" || metrics["explicit_invocation_probe"] != 1 || metrics["implicit_recall_probe"] != 0 {
		t.Fatalf("explicit selection mislabeled: %s %v", verdict, metrics)
	}
}

func TestStagePluginTreePreservesExecutableBits(t *testing.T) {
	source, destination := t.TempDir(), filepath.Join(t.TempDir(), "staged")
	if err := os.MkdirAll(filepath.Join(source, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "hooks", "run-hook.cmd"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := stagePluginTree(source, destination)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(destination, "hooks", "run-hook.cmd"))
	if err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("staged launcher lost its executable bit: %v %v", info.Mode(), err)
	}
	if plain, _ := os.Stat(filepath.Join(destination, "README.md")); plain.Mode().Perm()&0o111 != 0 {
		t.Fatalf("staged plain file became executable: %v", plain.Mode())
	}
	if err := os.Chmod(filepath.Join(source, "hooks", "run-hook.cmd"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := stagePluginTree(source, filepath.Join(t.TempDir(), "staged"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("the plugin hash must cover executable bits")
	}
}

func TestManifestRecordsTheVerifiedBrokerHash(t *testing.T) {
	if got := manifestBrokerHash(runOptions{BrokerHash: "sha256:" + strings.Repeat("a", 64)}); got != "sha256:"+strings.Repeat("a", 64) {
		t.Fatalf("credentialed manifest broker hash = %q", got)
	}
	if got := manifestBrokerHash(runOptions{Selftest: true}); got != hashBytes([]byte("in-process-selftest-fake")) {
		t.Fatalf("selftest manifest broker hash = %q", got)
	}
}

func TestBrokerOutputRequiresEveryField(t *testing.T) {
	complete := `{"schema_version":"1","cli_version":"x","response":"","trace":"","events":[],"plugin_inventory":[],"rc":0,"duration_ms":1,"isolation":{}}`
	if _, err := decodeBrokerOutput([]byte(complete)); err != nil {
		t.Fatalf("complete response rejected: %v", err)
	}
	for _, field := range []string{"rc", "events", "trace"} {
		broken := strings.Replace(complete, `"`+field+`":`, `"omitted_`+field+`":`, 1)
		if _, err := decodeBrokerOutput([]byte(broken)); err == nil {
			t.Errorf("response without %q was accepted", field)
		}
	}
	if _, err := decodeBrokerOutput([]byte(strings.Replace(complete, `"events":[]`, `"events":null`, 1))); err == nil {
		t.Error("response with null events was accepted")
	}
}
