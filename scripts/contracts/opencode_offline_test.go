package contracts

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// These files are handwritten API-shaped inputs; no exporter or report rewrite.
func TestOpenCodeMinerFilesAndManifest(t *testing.T) {
	root := repoRoot(t)
	tool := buildOpenCodeMine(t, root)
	const valid = `{"data":{"info":{"id":"one","location":{"directory":"/work/demo"}},"messages":[{"type":"user","text":"Did you test it?"}]}}`
	for _, tc := range []struct {
		name, manifest, status, environment        string
		invalid, duplicate, missing                bool
		sessions, skipped, partial, skippedSources int
	}{
		{name: "plain files", status: "complete", environment: "offline", sessions: 1},
		{name: "invalid beside valid", status: "partial", environment: "offline", invalid: true, sessions: 1, skipped: 1, partial: 1},
		{name: "duplicate IDs", status: "complete", environment: "offline", duplicate: true, sessions: 1},
		{name: "missing path", status: "skipped", environment: "offline", missing: true, skippedSources: 1},
		{name: "complete manifest", manifest: `{"status":"complete","skipped_sessions":0,"environment":"inherited"}`, status: "complete", environment: "inherited", sessions: 1},
		{name: "partial manifest", manifest: `{"status":"partial","skipped_sessions":2,"environment":"without_xdg_state_home"}`, status: "partial", environment: "without_xdg_state_home", sessions: 1, skipped: 2, partial: 1},
		{name: "skipped manifest", manifest: `{"status":"skipped","skipped_sessions":0,"environment":"inherited"}`, status: "skipped", environment: "inherited", skippedSources: 1},
		{name: "malformed manifest", manifest: `{`, status: "partial", environment: "offline", sessions: 1, partial: 1},
		{name: "invalid manifest status", manifest: `{"status":"wrong","skipped_sessions":0,"environment":"inherited"}`, status: "partial", environment: "offline", sessions: 1, partial: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, data string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.sessions > 0 {
				write("one.json", valid)
			}
			write("notes.txt", "not JSON; ignored")
			if tc.invalid {
				write("bad.json", "{")
			}
			if tc.duplicate {
				write("two.json", valid)
			}
			if tc.manifest != "" {
				write("export-status", tc.manifest)
			}
			input := dir
			if tc.missing {
				input = filepath.Join(dir, "missing")
			}
			cmd := exec.Command(tool, "--all-workspaces", "--harness", "opencode", "--opencode-export", input, "--format", "json")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			data, err := cmd.Output()
			if err != nil {
				t.Fatal(err, stderr.String())
			}
			var report struct {
				Sessions       int `json:"sessions_scanned"`
				Skipped        int `json:"skipped_sessions"`
				Partial        int `json:"partial_sources"`
				SkippedSources int `json:"skipped_sources"`
				BySource       map[string]struct {
					Status      string `json:"status"`
					Environment string `json:"environment"`
				} `json:"by_source"`
			}
			if err := json.Unmarshal(data, &report); err != nil {
				t.Fatal(err)
			}
			source := report.BySource["opencode"]
			if report.Sessions != tc.sessions || report.Skipped != tc.skipped || report.Partial != tc.partial || report.SkippedSources != tc.skippedSources || source.Status != tc.status || source.Environment != tc.environment {
				t.Fatalf("unexpected miner report: %s\nstderr: %s", data, stderr.String())
			}
		})
	}
}

func TestOpenCodeExporterFiltersAndOutput(t *testing.T) {
	root := repoRoot(t)
	tool := filepath.Join(t.TempDir(), "exporter")
	if out, code := run(t, root, nil, "go", "build", "-o", tool, "./scripts/cmd/opencode-export"); code != 0 {
		t.Fatal(out)
	}
	for _, output := range []string{"relative-export", filepath.Join(root, "exports"), root} {
		out, code := run(t, root, nil, tool, "--output", output, "--all-workspaces")
		if code != 2 {
			t.Fatalf("unsafe output accepted (%d): %s", code, out)
		}
	}
	info := openCodeInfo("keep", "/work/demo", "")
	other := openCodeInfo("other", "/work/other", "")
	old := openCodeInfo("old", "/work/demo", "")
	old["time"] = map[string]any{"updated": 1767225600000}
	command, fixture := fakeSessionMineOpenCode(t, map[string][]string{
		"/api/session?limit=100&order=desc":      {openCodeJSON(map[string]any{"data": []any{info, other, old, openCodeInfo("moved", "/work/demo", "")}})},
		"/api/experimental/session/keep/export":  {openCodeExport(info, openCodeMessage("user", "Ready"))},
		"/api/experimental/session/moved/export": {openCodeExport(openCodeInfo("moved", "/work/other", ""), openCodeMessage("user", "Moved"))},
	})
	outDir := filepath.Join(t.TempDir(), "exports")
	out, code := run(t, root, nil, tool, "--output", outDir, "--workspace", "/work/demo", "--since", "2026-11-01", "--opencode-cmd", command)
	if code != 0 {
		t.Fatalf("filtered export (%d): %s", code, out)
	}
	var summary struct {
		Written int `json:"exports_written"`
	}
	if json.Unmarshal([]byte(out), &summary) != nil || summary.Written != 1 {
		t.Fatalf("moved session was exported: %s", out)
	}
	calls, err := os.ReadFile(filepath.Join(fixture, "calls"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(calls, []byte("/other/export")) || bytes.Contains(calls, []byte("/old/export")) || !bytes.Contains(calls, []byte("/keep/export")) {
		t.Fatalf("wrong exports: %s", calls)
	}
	status, err := os.ReadFile(filepath.Join(outDir, "export-status"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(status, &manifest) != nil || manifest.Status != "complete" {
		t.Fatalf("manifest: %s", status)
	}
}
