package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func pluginVersion(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(packageDir(t), "..", ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version == "" {
		t.Fatalf("plugin manifest version: %v", err)
	}
	return manifest.Version
}

func writeClaudeSettings(t *testing.T, home, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func doctorOutput(t *testing.T, env map[string]string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	// Doctor must never wait on stdin: a terminal caller would hang.
	rc := runHook([]string{"doctor"}, func(key string) string { return env[key] }, iotest.ErrReader(errors.New("stdin must not be read")), &stdout, &stderr)
	if rc != 0 || stderr.Len() != 0 {
		t.Fatalf("doctor rc=%d stderr=%q stdout=%q", rc, stderr.String(), stdout.String())
	}
	return stdout.String()
}

func TestDoctorReportsHealthyClaudeSetup(t *testing.T) {
	home := t.TempDir()
	writeClaudeSettings(t, home, `{"outputStyle":"megapowers:Megapowers"}`)
	env := map[string]string{
		"HOME":                      home,
		"MEGAPOWERS_HARNESS":        "claude",
		"MEGAPOWERS_PLUGIN_ROOT":    filepath.Dir(packageDir(t)),
		"MEGAPOWERS_HOOK_CACHE_DIR": filepath.Join(home, ".cache", "megapowers-hooks"),
	}
	report := doctorOutput(t, env)
	for _, want := range []string{
		"megapowers doctor",
		"plugin root: " + filepath.Dir(packageDir(t)),
		"plugin version: " + pluginVersion(t),
		"harness: claude",
		"go toolchain: go1.",
		"runner cache: " + filepath.Join(home, ".cache", "megapowers-hooks"),
		"claude outputStyle: megapowers:Megapowers",
		"hooks.json events: PreToolUse (Bash|PowerShell), SessionStart, SubagentStart",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("doctor report lacks %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "WARN") {
		t.Errorf("healthy setup reported WARN:\n%s", report)
	}
}

func TestDoctorWarnsOnWrongOutputStyleValue(t *testing.T) {
	home := t.TempDir()
	writeClaudeSettings(t, home, `{"outputStyle":"Megapowers"}`)
	env := map[string]string{"HOME": home, "MEGAPOWERS_HARNESS": "claude", "MEGAPOWERS_PLUGIN_ROOT": filepath.Dir(packageDir(t))}
	report := doctorOutput(t, env)
	if !strings.Contains(report, "WARN") || !strings.Contains(report, `outputStyle: Megapowers`) || !strings.Contains(report, "megapowers:Megapowers") {
		t.Fatalf("doctor must report the actual value and the expected one:\n%s", report)
	}
}

func TestDoctorToleratesMissingSettingsAndManifest(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{"HOME": home, "MEGAPOWERS_HARNESS": "claude", "MEGAPOWERS_PLUGIN_ROOT": t.TempDir()}
	report := doctorOutput(t, env)
	for _, want := range []string{"WARN", "settings.json", "plugin version:", "hooks.json"} {
		if !strings.Contains(report, want) {
			t.Errorf("doctor report lacks %q:\n%s", want, report)
		}
	}
}

func TestDoctorReportsCodexStyleState(t *testing.T) {
	home := t.TempDir()
	root := filepath.Dir(packageDir(t))
	on := doctorOutput(t, map[string]string{"HOME": home, "MEGAPOWERS_HARNESS": "codex", "MEGAPOWERS_PLUGIN_ROOT": root})
	if !strings.Contains(on, "harness: codex") || !strings.Contains(on, "MEGAPOWERS_OUTPUT_STYLE: unset (style injected)") || strings.Contains(on, "claude outputStyle") {
		t.Fatalf("codex doctor report:\n%s", on)
	}
	off := doctorOutput(t, map[string]string{"HOME": home, "MEGAPOWERS_HARNESS": "codex", "MEGAPOWERS_PLUGIN_ROOT": root, "MEGAPOWERS_OUTPUT_STYLE": "off"})
	if !strings.Contains(off, "MEGAPOWERS_OUTPUT_STYLE: off (style injection disabled)") {
		t.Fatalf("codex doctor report with style off:\n%s", off)
	}
}

func TestDoctorReportsAgentRegistryExpiry(t *testing.T) {
	previous := doctorNow
	doctorNow = func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { doctorNow = previous })
	root := filepath.Dir(packageDir(t))
	for _, test := range []struct {
		name, body string
		want       []string
		warn       bool
	}{
		{"absent", "", []string{"agent registry: none"}, false},
		{"current", "```yaml\nversion: 2\nexpires_at: 2026-10-25\n```\n", []string{"agent registry:", "expires 2026-10-25"}, false},
		{"expires today", "expires_at: 2026-09-28\n", []string{"expires 2026-09-28"}, false},
		{"expired", "expires_at: 2026-09-01\n", []string{"expired 2026-09-01", "orchestrating ignores it"}, true},
		{"no expiry", "version: 2\n", []string{"no readable expires_at", "orchestrating ignores it"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			if test.body != "" {
				dir := filepath.Join(home, ".config", "megapowers")
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "agent-capabilities.md"), []byte(test.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			report := doctorOutput(t, map[string]string{"HOME": home, "MEGAPOWERS_HARNESS": "codex", "MEGAPOWERS_PLUGIN_ROOT": root})
			for _, want := range test.want {
				if !strings.Contains(report, want) {
					t.Errorf("report lacks %q:\n%s", want, report)
				}
			}
			if got := strings.Contains(report, "WARN: agent registry"); got != test.warn {
				t.Errorf("registry WARN = %v, want %v:\n%s", got, test.warn, report)
			}
		})
	}
}

func TestDoctorDetectsHarnessFromEnvironment(t *testing.T) {
	root := filepath.Dir(packageDir(t))
	home := t.TempDir()
	codex := doctorOutput(t, map[string]string{"HOME": home, "PLUGIN_ROOT": root, "CLAUDE_PLUGIN_ROOT": root, "MEGAPOWERS_PLUGIN_ROOT": root})
	if !strings.Contains(codex, "harness: codex") {
		t.Fatalf("PLUGIN_ROOT should imply codex:\n%s", codex)
	}
	claude := doctorOutput(t, map[string]string{"HOME": home, "CLAUDECODE": "1", "MEGAPOWERS_PLUGIN_ROOT": root})
	if !strings.Contains(claude, "harness: claude") {
		t.Fatalf("CLAUDECODE should imply claude:\n%s", claude)
	}
	unknown := doctorOutput(t, map[string]string{"HOME": home, "MEGAPOWERS_PLUGIN_ROOT": root})
	if !strings.Contains(unknown, "harness: unknown") || !strings.Contains(unknown, "claude outputStyle") || !strings.Contains(unknown, "MEGAPOWERS_OUTPUT_STYLE") {
		t.Fatalf("unknown harness must report both style checks:\n%s", unknown)
	}
}
