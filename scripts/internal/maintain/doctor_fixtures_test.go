package maintain

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Ported from the pre-v0.46 doctor and gate regressions.
// These fixtures exercise the shipped go-run entry point using isolated runtime
// homes. The Go tool itself keeps its normal environment and default caches.
func runDoctorFixtureChild() int {
	var values map[string]string
	if err := json.Unmarshal([]byte(os.Getenv("MEGAPOWERS_DOCTOR_FIXTURE_JSON")), &values); err != nil {
		return 2
	}
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Dir = values["TEST_DOCTOR_CWD"]
	for _, key := range []string{"PATH", "TMPDIR", "SYSTEMROOT"} {
		if value := os.Getenv(key); value != "" {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	for key, value := range values {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return e.ExitCode()
		}
		return 2
	}
	return 0
}
func doctorPluginRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../plugins/megapowers")
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func doctorOutput(t *testing.T, values map[string]string) string { return doctorArgs(t, values) }
func doctorArgs(t *testing.T, values map[string]string, args ...string) string {
	t.Helper()
	script := filepath.Join(doctorPluginRoot(t), "skills/megapowers-doctor/scripts/doctor.go")
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", append([]string{"run", "-exec", launcher, script}, args...)...)
	cmd.Env = append(os.Environ(), "MEGAPOWERS_DOCTOR_FIXTURE_JSON="+string(data))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	return string(out)
}
func doctorWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func strconvQuote(value string) string { b, _ := json.Marshal(value); return string(b) }

var claudeProjectSlug = regexp.MustCompile("[^A-Za-z0-9]")

func skillReadTrace(t *testing.T, name, input, output string, failed bool) []string {
	t.Helper()
	call := map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_use", "id": "read-skill", "name": name, "input": json.RawMessage(input)},
	}}}
	result := map[string]any{"type": "user", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_result", "tool_use_id": "read-skill", "is_error": failed, "content": output},
	}}}
	var lines []string
	for _, record := range []any{call, result} {
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(data))
	}
	return lines
}

func TestDoctorWithoutRootSkipsRelativeFiles(t *testing.T) {
	workspace := t.TempDir()
	doctorWrite(t, filepath.Join(workspace, ".claude-plugin/plugin.json"), `{"version":"99.99.99"}`)
	doctorWrite(t, filepath.Join(workspace, "skills/catalog.json"), `{"skills":[{"name":"safe-effects"}]}`)
	report := doctorOutput(t, map[string]string{"HOME": t.TempDir(), "PWD": workspace, "TEST_DOCTOR_CWD": workspace, "MEGAPOWERS_HARNESS": "codex"})
	if !strings.Contains(report, "plugin root unknown") {
		t.Fatalf("missing root diagnostic: %s", report)
	}
	for _, unexpected := range []string{"plugin version:", "skill loads,", "marketplace copy"} {
		if strings.Contains(report, unexpected) {
			t.Errorf("relative check ran without root: %s", report)
		}
	}
}

func TestDoctorIgnoresExternalDecoys(t *testing.T) {
	root := doctorPluginRoot(t)
	baseline := doctorArgs(t, nil, "--root", root)
	home := t.TempDir()
	workspace := filepath.Join(home, "workspace")
	claude := filepath.Join(home, ".claude")
	codex := filepath.Join(home, ".codex")
	doctorWrite(t, filepath.Join(claude, "settings.json"), `{"outputStyle":"DECOY"}`)
	doctorWrite(t, filepath.Join(workspace, ".claude/settings.local.json"), `{"outputStyle":"DECOY"}`)
	doctorWrite(t, filepath.Join(workspace, ".claude/settings.json"), `{"outputStyle":"DECOY"}`)
	doctorWrite(t, filepath.Join(home, ".config/megapowers/agent-capabilities.md"), "expires_at: 2000-01-01\n")
	for _, p := range []string{filepath.Join(claude, "plugins/marketplaces/megapowers/plugins/megapowers/.claude-plugin/plugin.json"), filepath.Join(codex, ".tmp/marketplaces/megapowers/plugins/megapowers/.codex-plugin/plugin.json")} {
		doctorWrite(t, p, `{"version":"99.99.99"}`)
	}
	project := claudeProjectSlug.ReplaceAllString(workspace, "-")
	doctorWrite(t, filepath.Join(claude, "projects", project, "s.jsonl"), strings.Join(skillReadTrace(t, "Skill", `{"skill":"megapowers:safe-effects"}`, "Launching skill", false), "\n")+"\n")
	doctorWrite(t, filepath.Join(codex, "sessions", time.Now().Format("2006/01/02"), "rollout-decoy.jsonl"), `{"type":"session_meta","payload":{"cwd":`+strconvQuote(workspace)+`}}`+"\n")
	env := map[string]string{"HOME": home, "USERPROFILE": home, "CLAUDE_CONFIG_DIR": claude, "CODEX_HOME": codex, "CLAUDE_PROJECT_DIR": workspace, "PWD": workspace, "TEST_DOCTOR_CWD": workspace}
	if got := doctorArgs(t, env, "--root", root); got != baseline {
		t.Fatalf("external decoys changed report\nbaseline:\n%s\ndecoys:\n%s", baseline, got)
	}
}

func TestDoctorPluginTree(t *testing.T) {
	for _, tc := range []struct{ name, catalog, want string }{
		{"catalog", `{"skills":[{"name":"example"}]}`, "skill catalog: 1 skills"},
		{"malformed catalog", `{`, "WARN: skill catalog is not valid JSON"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			doctorWrite(t, filepath.Join(root, ".claude-plugin/plugin.json"), `{"version":"1.2.3"}`)
			doctorWrite(t, filepath.Join(root, ".codex-plugin/plugin.json"), `{"version":"1.2.4"}`)
			doctorWrite(t, filepath.Join(root, "skills/catalog.json"), tc.catalog)
			doctorWrite(t, filepath.Join(root, "skills/example/SKILL.md"), "# Example")
			doctorWrite(t, filepath.Join(root, "skills/orchestrating/assets/agent-capabilities.md"), "# Template")
			got := doctorArgs(t, nil, "--root", root, "--harness", "claude")
			for _, want := range []string{tc.want, "version 1.2.4 differs from 1.2.3", "agent registry template:", "harness: claude"} {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q: %s", want, got)
				}
			}
		})
	}
}

func TestDoctorRejectsEscapingSymlinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	doctorWrite(t, filepath.Join(outside, "plugin.json"), `{"version":"99.99.99"}`)
	if err := os.Symlink(outside, filepath.Join(root, ".claude-plugin")); err != nil {
		t.Fatal(err)
	}
	got := doctorArgs(t, nil, "--root", root)
	if strings.Contains(got, "99.99.99") || !strings.Contains(got, "WARN: no readable plugin manifest") {
		t.Fatalf("outside manifest read: %s", got)
	}
}

func TestDoctorRootEnvironment(t *testing.T) {
	for _, key := range []string{"MEGAPOWERS_PLUGIN_ROOT", "PLUGIN_ROOT", "CLAUDE_PLUGIN_ROOT"} {
		t.Run(key, func(t *testing.T) {
			got := doctorOutput(t, map[string]string{key: doctorPluginRoot(t)})
			if !strings.Contains(got, "plugin version:") {
				t.Fatalf("root variable ignored: %s", got)
			}
		})
	}
}
