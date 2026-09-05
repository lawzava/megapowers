package main

import (
	"os"
	"path/filepath"
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
