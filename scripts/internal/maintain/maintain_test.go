package maintain

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseVersionValidation(t *testing.T) {
	for _, version := range []string{"0.1.0", "12.34.56"} {
		if !validReleaseVersion(version) {
			t.Fatalf("validReleaseVersion(%q) = false", version)
		}
	}
	for _, version := range []string{"v1.2.3", "1.2", "1.2.3-rc1", "01.2.3", ""} {
		if validReleaseVersion(version) {
			t.Fatalf("validReleaseVersion(%q) = true", version)
		}
	}
}

func TestVersionAtLeastUsesNumericComponents(t *testing.T) {
	tests := []struct {
		got, minimum string
		want         bool
	}{
		{"0.149.0", "0.149.0", true},
		{"0.150.0", "0.149.0", true},
		{"0.149.10", "0.149.2", true},
		{"0.148.99", "0.149.0", false},
		{"not-a-version", "0.149.0", false},
	}
	for _, tt := range tests {
		if got := versionAtLeast(tt.got, tt.minimum); got != tt.want {
			t.Errorf("versionAtLeast(%q, %q) = %v, want %v", tt.got, tt.minimum, got, tt.want)
		}
	}
}

func TestInstalledTreeRequiresMatchingBytesAndModes(t *testing.T) {
	source := t.TempDir()
	installed := t.TempDir()
	write := func(root, name, body string, mode os.FileMode) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(source, "plugin.txt", "same\n", 0o644)
	write(installed, "plugin.txt", "same\n", 0o644)
	if err := compareTrees(source, installed); err != nil {
		t.Fatalf("identical trees: %v", err)
	}
	write(installed, "plugin.txt", "different\n", 0o644)
	if err := compareTrees(source, installed); err == nil {
		t.Fatal("different bytes accepted")
	}
	write(installed, "plugin.txt", "same\n", 0o600)
	if err := os.Chmod(filepath.Join(installed, "plugin.txt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := compareTrees(source, installed); err == nil {
		t.Fatal("different modes accepted")
	}
}

func TestResultsRequirePassAndRejectFailureOrStrictSkip(t *testing.T) {
	tests := []struct {
		body   string
		strict bool
		want   bool
	}{
		{"claude\tSKIP\tunavailable\n", false, false},
		{"claude\tPASS\tinstalled\ncodex\tSKIP\tunavailable\n", false, true},
		{"claude\tPASS\tinstalled\ncodex\tSKIP\tunavailable\n", true, false},
		{"claude\tPASS\tinstalled\ncodex\tFAIL\tbroken\n", false, false},
	}
	for _, tt := range tests {
		if got := resultsOK([]byte(tt.body), tt.strict); got != tt.want {
			t.Errorf("resultsOK(%q, %v) = %v, want %v", tt.body, tt.strict, got, tt.want)
		}
	}
}

func TestInstalledHookSmokeRunsColdWarmAndDenies(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(t.TempDir(), "plugin")
	hooks := filepath.Join(installed, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"run-hook.cmd", "hook_runner.go", "deny_destructive.go", "output_style.go"} {
		data, err := os.ReadFile(filepath.Join(repo, "plugins", "megapowers", "hooks", name))
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if name == "run-hook.cmd" {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(hooks, name), data, mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := verifyInstalledHookRuntime(context.Background(), t.TempDir(), installed); err == nil || !strings.Contains(err.Error(), "installed Codex session-start") {
		t.Fatalf("missing installed style must fail the runtime probe: %v", err)
	}
	style, err := os.ReadFile(filepath.Join(repo, "plugins", "megapowers", "output-styles", "megapowers.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(installed, "output-styles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "output-styles", "megapowers.md"), style, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyInstalledHookRuntime(context.Background(), t.TempDir(), installed); err != nil {
		t.Fatalf("installed hook runtime: %v", err)
	}
}

func TestMemorySkillPolicySelectsInstalledExplicitOnlyEntry(t *testing.T) {
	installed := filepath.Join(t.TempDir(), "plugin")
	inside := filepath.Join(installed, "skills", "memory-hygiene", "SKILL.md")
	outside := filepath.Join(t.TempDir(), "skills", "memory-hygiene", "SKILL.md")
	data := []any{map[string]any{"skills": []any{
		map[string]any{"name": "memory-hygiene", "path": outside, "enabled": true, "policy": map[string]any{"allowImplicitInvocation": true}},
		map[string]any{"name": "megapowers:memory-hygiene", "path": inside, "enabled": true, "policy": map[string]any{"allowImplicitInvocation": false}, "explicitlyAvailable": true},
		map[string]any{"name": "megapowers:writing-agent-instructions", "path": filepath.Join(installed, "skills", "writing-agent-instructions", "SKILL.md"), "enabled": true},
	}}}
	policyExposed, err := verifyMemorySkillEntries(data, installed)
	if err != nil || !policyExposed {
		t.Fatalf("installed explicit-only skill rejected: %v", err)
	}

	badFields := []map[string]any{
		{"name": "memory-hygiene", "path": inside, "enabled": true, "policy": nil},
		{"name": "memory-hygiene", "path": inside, "enabled": true, "policy": map[string]any{"allowImplicitInvocation": true}},
		{"name": "memory-hygiene", "path": inside, "enabled": false, "policy": map[string]any{"allowImplicitInvocation": false}},
		{"name": "memory-hygiene", "path": inside, "enabled": true, "policy": map[string]any{"allowImplicitInvocation": false}, "availableForExplicitInvocation": false},
	}
	for _, skill := range badFields {
		body, _ := json.Marshal(skill)
		t.Run(strings.ReplaceAll(string(body), "/", "_"), func(t *testing.T) {
			writing := map[string]any{"name": "writing-agent-instructions", "path": filepath.Join(installed, "skills", "writing-agent-instructions", "SKILL.md"), "enabled": true}
			if _, err := verifyMemorySkillEntries([]any{map[string]any{"skills": []any{skill, writing}}}, installed); err == nil {
				t.Fatalf("invalid skill accepted: %s", body)
			}
		})
	}
	policyExposed, err = verifyMemorySkillEntries([]any{map[string]any{"skills": []any{
		map[string]any{"name": "memory-hygiene", "path": inside, "enabled": true},
		map[string]any{"name": "writing-agent-instructions", "path": filepath.Join(installed, "skills", "writing-agent-instructions", "SKILL.md"), "enabled": true},
	}}}, installed)
	if err != nil || policyExposed {
		t.Fatalf("API without policy should be accepted as declaration-only evidence: exposed=%v err=%v", policyExposed, err)
	}
}

func TestCodexSkillEntriesRequireInstalledWritingSkill(t *testing.T) {
	installed := filepath.Join(t.TempDir(), "plugin")
	data := []any{map[string]any{"skills": []any{
		map[string]any{"name": "memory-hygiene", "path": filepath.Join(installed, "skills", "memory-hygiene", "SKILL.md"), "enabled": true},
	}}}
	if _, err := verifyMemorySkillEntries(data, installed); err == nil {
		t.Fatal("missing writing-agent-instructions skill was accepted")
	}
}

func TestSmokeOptionsRejectReleaseFlagsWithoutSource(t *testing.T) {
	for _, args := range [][]string{
		{"--out", t.TempDir(), "--ref", "v1.2.3"},
		{"--out", t.TempDir(), "--version", "1.2.3"},
		{"--out", t.TempDir(), "--repo", t.TempDir(), "--version", "1.2.3"},
	} {
		if _, err := parseSmokeOptions("/repo", args); err == nil {
			t.Fatalf("release-only flags accepted without --source: %v", args)
		}
	}
}

func TestShellSyntaxChecksEveryDiscoveredFile(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.sh")
	second := filepath.Join(root, "second.sh")
	os.WriteFile(first, []byte("#!/bin/sh\ntrue\n"), 0o755)
	os.WriteFile(second, []byte("#!/bin/sh\nif then\n"), 0o755)
	if err := checkShellSyntax(context.Background(), root, []string{first, second}); err == nil {
		t.Fatal("syntax error in non-first shell file was accepted")
	}
}

func TestExecutableCheckRejectsNonExecutableNonFirstFile(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.sh")
	second := filepath.Join(root, "second.sh")
	os.WriteFile(first, []byte("#!/bin/sh\n"), 0o755)
	os.WriteFile(second, []byte("#!/bin/sh\n"), 0o644)
	if err := checkExecutableFiles(root, []string{first, second}); err == nil {
		t.Fatal("non-executable non-first entrypoint was accepted")
	}
}

func TestManifestVersionRequiresMegapowersName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugin.json")
	os.WriteFile(path, []byte(`{"name":"other","version":"1.2.3"}`), 0o644)
	if _, err := manifestVersion(path); err == nil {
		t.Fatal("manifest with wrong plugin name was accepted")
	}
}

func TestDeclaredExplicitOnlyPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openai.yaml")
	for _, tt := range []struct {
		body string
		ok   bool
	}{
		{"policy:\n  allow_implicit_invocation: false\n", true},
		{"interface:\n  display_name: Example\npolicy:\n  allow_implicit_invocation: false\n", false},
		{"policy:\n  allow_implicit_invocation: true\n", false},
		{"# policy:\n#   allow_implicit_invocation: false\n", false},
		{"policy:\n  allow_implicit_invocation: false\n  allow_implicit_invocation: false\n", false},
		{"policy:\n  allow_implicit_invocation: false\n  allow_implicit_invocation: true\n", false},
		{"policy:\n  child:\n    allow_implicit_invocation: false\n", false},
	} {
		os.WriteFile(path, []byte(tt.body), 0o644)
		if err := verifyDeclaredExplicitOnly(path); (err == nil) != tt.ok {
			t.Errorf("verifyDeclaredExplicitOnly(%q) error=%v, want ok=%v", tt.body, err, tt.ok)
		}
	}
}
