package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunOutputStyleForCodex(t *testing.T) {
	dir := t.TempDir()
	styleDir := filepath.Join(dir, "output-styles")
	if err := os.Mkdir(styleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	style := "---\nname: Megapowers\n---\nDirect prose.\n"
	if err := os.WriteFile(filepath.Join(styleDir, "megapowers.md"), []byte(style), 0o644); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"MEGAPOWERS_HARNESS":     "codex",
		"MEGAPOWERS_PLUGIN_ROOT": dir,
	}
	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"output-style"}, func(key string) string { return env[key] }, strings.NewReader(`{"hook_event_name":"SessionStart"}`), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runHook returned %d: %s", rc, stderr.String())
	}
	if strings.Contains(stdout.String(), "name: Megapowers") || !strings.Contains(stdout.String(), "Direct prose.") || strings.Contains(stdout.String(), skillLoadingReminder) {
		t.Fatalf("unexpected output style: %q", stdout.String())
	}
}

func TestRunOutputStyleOptOut(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"MEGAPOWERS_HARNESS":      "codex",
		"MEGAPOWERS_OUTPUT_STYLE": "off",
	}
	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"output-style"}, func(key string) string { return env[key] }, strings.NewReader(`{}`), &stdout, &stderr)
	if rc != 0 || stdout.Len() != 0 {
		t.Fatalf("runHook rc=%d stdout=%q stderr=%q, want silent opt-out", rc, stdout.String(), stderr.String())
	}
}

func TestSessionStartSeparatesWorkflowFromStyle(t *testing.T) {
	for _, harness := range []string{"codex", "claude"} {
		for _, styleMode := range []string{"", "off"} {
			for _, source := range []string{"startup", "resume", "compact"} {
				t.Run(harness+"/"+styleMode+"/"+source, func(t *testing.T) {
					root := t.TempDir()
					if err := os.Mkdir(filepath.Join(root, "output-styles"), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(root, "output-styles", "megapowers.md"), []byte("---\nname: Example\n---\nSTYLE_SENTINEL\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					env := map[string]string{"MEGAPOWERS_HARNESS": harness, "MEGAPOWERS_OUTPUT_STYLE": styleMode, "MEGAPOWERS_PLUGIN_ROOT": root}
					var stdout, stderr bytes.Buffer
					rc := runHook([]string{"session-start"}, func(key string) string { return env[key] }, strings.NewReader(`{"hook_event_name":"SessionStart","source":"`+source+`"}`), &stdout, &stderr)
					if rc != 0 || stderr.Len() != 0 {
						t.Fatalf("session-start rc=%d stderr=%q", rc, stderr.String())
					}
					if strings.Count(stdout.String(), skillLoadingReminder) != 1 {
						t.Fatalf("workflow guidance must appear exactly once: %q", stdout.String())
					}
					if want := harness == "codex" && styleMode != "off"; strings.Contains(stdout.String(), "STYLE_SENTINEL") != want {
						t.Fatalf("style visibility differs from preference: %q", stdout.String())
					}
				})
			}
		}
	}
}

func TestSessionStartRejectsOversizedInputEvenWithStyleOff(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"session-start"}, func(string) string { return "off" }, strings.NewReader(strings.Repeat("x", maxHookInputBytes+1)), &stdout, &stderr)
	if rc == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "cannot read hook input") {
		t.Fatalf("rc=%d stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
}

func TestRunOutputStyleHarnessSelection(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		env     map[string]string
		enabled bool
	}{
		{name: "explicit Codex", env: map[string]string{"MEGAPOWERS_HARNESS": "codex"}, enabled: true},
		{name: "explicit Claude", env: map[string]string{"MEGAPOWERS_HARNESS": "claude"}},
		{name: "Codex plugin root", env: map[string]string{"PLUGIN_ROOT": "/plugin"}, enabled: true},
		{name: "Claude default", env: map[string]string{}},
		{name: "opt out wins", env: map[string]string{"MEGAPOWERS_HARNESS": "codex", "MEGAPOWERS_OUTPUT_STYLE": "off"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := outputStyleEnabled(func(key string) string { return tt.env[key] }); got != tt.enabled {
				t.Fatalf("outputStyleEnabled() = %v, want %v", got, tt.enabled)
			}
		})
	}
}

func TestRepositoryOutputStyleRendersWithoutFrontmatter(t *testing.T) {
	t.Parallel()

	path := filepath.Join(packageDir(t), "..", "output-styles", "megapowers.md")
	var output bytes.Buffer
	if err := writeOutputStyle(path, &output); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"ASD-STE100-inspired principles",
		"Default to 100 prose words or fewer.",
		"Do not exceed 250 prose words",
		"Preserve exact identifiers, commands, numbers, caveats, decisions, and material uncertainty.",
	} {
		if !strings.Contains(output.String(), required) {
			t.Errorf("rendered output omits %q", required)
		}
	}
	if strings.Contains(output.String(), "force-for-plugin:") {
		t.Error("rendered output exposes Claude frontmatter")
	}
}
