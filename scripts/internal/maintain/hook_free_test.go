package maintain

import (
	"encoding/json"
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
	for _, name := range []string{".claude-plugin/plugin.json", ".codex-plugin/plugin.json"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		var manifest map[string]json.RawMessage
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		if _, ok := manifest["hooks"]; ok {
			t.Errorf("%s declares hooks", name)
		}
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
	if !strings.Contains(string(out), "plugin version:") || !strings.Contains(string(out), "agent registry template:") {
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
	root := t.TempDir()
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
	cmd := exec.Command("go", "run", script, "--root", root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	for _, want := range []string{"version 1.2.4 differs from 1.2.3", "skill catalog: 1 skills", "skill verify-and-finish body unreadable"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
}
