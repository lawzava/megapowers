package maintain

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookFreePlugin(t *testing.T) {
	root := filepath.Join("..", "..", "..", "plugins", "megapowers")
	if _, err := os.Stat(filepath.Join(root, "hooks")); !os.IsNotExist(err) {
		t.Fatal("plugin still ships hooks")
	}
}

func TestStandaloneDoctor(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", "..", "..", "plugins", "megapowers"))
	cmd := exec.Command("go", "run", filepath.Join(root, "skills", "megapowers-doctor", "scripts", "doctor.go"), "--root", root)
	cmd.Env = append(os.Environ(), "MEGAPOWERS_HARNESS=codex")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "plugin version:") || !strings.Contains(string(out), "agent registry:") {
		t.Fatalf("missing diagnostics: %s", out)
	}
	if strings.Contains(string(out), "hooks.json") || strings.Contains(string(out), "style injection enabled") {
		t.Fatalf("obsolete diagnostics: %s", out)
	}
}

func TestDoctorDirectoryManifest(t *testing.T) {
	script, _ := filepath.Abs(filepath.Join("..", "..", "..", "plugins", "megapowers", "skills", "megapowers-doctor", "scripts", "doctor.go"))
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".codex-plugin"), 0755)
	os.WriteFile(filepath.Join(root, ".codex-plugin", "plugin.json"), []byte(`{"version":"1.2.3"}`), 0644)
	cmd := exec.Command("go", "run", script, "--root", root)
	cmd.Env = append(os.Environ(), "MEGAPOWERS_HARNESS=codex", "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "plugin version: 1.2.3") || strings.Contains(string(out), "WARN") {
		t.Fatalf("directory doctor: %v\n%s", err, out)
	}
}

func TestDoctorInstallationDiagnostics(t *testing.T) {
	script, _ := filepath.Abs(filepath.Join("..", "..", "..", "plugins", "megapowers", "skills", "megapowers-doctor", "scripts", "doctor.go"))
	root, home, workspace := t.TempDir(), t.TempDir(), t.TempDir()
	write := func(base, rel, body string) {
		t.Helper()
		path := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(root, ".claude-plugin/plugin.json", `{"version":"1.2.3"}`)
	write(root, ".codex-plugin/plugin.json", `{"version":"1.2.4"}`)
	write(root, "skills/catalog.json", `{"skills":[{"name":"verify-and-finish"}]}`)
	write(home, ".claude/settings.json", `{"outputStyle":"Other"}`)
	write(workspace, ".claude/settings.json", `{"outputStyle":"megapowers:Megapowers"}`)
	write(home, ".config/megapowers/agent-capabilities.md", "expires_at: 2000-01-01\n")
	write(home, ".claude/plugins/marketplaces/megapowers/plugins/megapowers/.claude-plugin/plugin.json", `{"version":"2.0.0"}`)
	cmd := exec.Command("go", "run", script, "--root", root)
	cmd.Env = append(os.Environ(), "HOME="+home, "CLAUDE_CONFIG_DIR="+filepath.Join(home, ".claude"), "MEGAPOWERS_HARNESS=claude", "CLAUDE_PROJECT_DIR="+workspace, "PWD="+workspace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	for _, want := range []string{"version 1.2.4 differs from 1.2.3", "OK, " + filepath.Join(workspace, ".claude", "settings.json"), "marketplace copy is 2.0.0", "expired 2000-01-01", "skill loads, last 7 days"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
}
