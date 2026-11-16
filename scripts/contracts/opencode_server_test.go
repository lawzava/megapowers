package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --opencode-server targets a server that is not the CLI-managed background
// service. The exporter must skip service-status discovery, pass the URL on
// every CLI call, never start a service, and accept only loopback hosts.
func TestOpenCodeExporterServerOption(t *testing.T) {
	root := repoRoot(t)
	tool := buildOpenCodeExporter(t, root)
	info := openCodeInfo("ses_server", "/work/demo", "")
	responses := map[string][]string{
		"/api/session?limit=100&order=desc":           {openCodeJSON(map[string]any{"data": []any{info}, "cursor": map[string]any{}})},
		"/api/experimental/session/ses_server/export": {openCodeExport(info, openCodeMessage("user", "Ready"))},
	}
	for _, server := range []string{"http://127.0.0.1:4096", "https://localhost:4096/", "http://[::1]:4096", "http://LOCALHOST:4096"} {
		t.Run("accepts "+server, func(t *testing.T) {
			command, fixture := fakeSessionMineOpenCode(t, responses)
			// The fake reports a running service only when asked; a status call
			// would show that discovery was not skipped.
			outDir := filepath.Join(t.TempDir(), "exports")
			out, code := run(t, root, nil, tool, "--output", outDir, "--workspace", "/work/demo", "--opencode-cmd", command, "--opencode-server", server)
			var summary struct {
				Written     int    `json:"exports_written"`
				Status      string `json:"status"`
				Environment string `json:"environment"`
			}
			if code != 0 || json.Unmarshal([]byte(out), &summary) != nil || summary.Written != 1 || summary.Status != "complete" {
				t.Fatalf("server export (%d): %s", code, out)
			}
			calls, _ := os.ReadFile(filepath.Join(fixture, "calls"))
			if strings.Contains(string(calls), "status") {
				t.Errorf("service status called with --opencode-server: %s", calls)
			}
			data, err := os.ReadFile(filepath.Join(fixture, "argv"))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != 2 {
				t.Fatalf("want one list and one export call, got %d: %s", len(lines), data)
			}
			for _, line := range lines {
				var argv []string
				if err := json.Unmarshal([]byte(line), &argv); err != nil {
					t.Fatal(err)
				}
				if len(argv) != 5 || argv[0] != "--server" || argv[1] != server || argv[2] != "api" || argv[3] != "get" {
					t.Errorf("CLI call without --server %s: %q", server, argv)
				}
				for _, arg := range argv {
					if arg == "serve" || arg == "start" || arg == "service" {
						t.Errorf("CLI call may start a service: %q", argv)
					}
				}
			}
		})
	}
	t.Run("password", func(t *testing.T) { testOpenCodeServerPassword(t, root, tool, responses) })
	for _, server := range []string{
		"", "127.0.0.1:4096", "localhost", "ftp://127.0.0.1:4096", "http://", "http://example.com:4096",
		"https://10.0.0.1:4096", "http://127.0.0.2:4096", "http://127.0.0.1.example.com:4096",
		"http://localhost.example.com", "http://user:secret@127.0.0.1:4096", "http://0.0.0.0:4096", "http://::1:4096", "http://[localhost]:4096", "http://[127.0.0.1]:4096",
	} {
		t.Run("rejects "+server, func(t *testing.T) {
			command, fixture := fakeSessionMineOpenCode(t, responses)
			outDir := filepath.Join(t.TempDir(), "exports")
			out, code := run(t, root, nil, tool, "--output", outDir, "--workspace", "/work/demo", "--opencode-cmd", command, "--opencode-server="+server)
			if code != 2 || !strings.Contains(out, "--opencode-server") {
				t.Fatalf("unsafe server accepted (%d): %s", code, out)
			}
			if _, err := os.Stat(filepath.Join(fixture, "argv")); !os.IsNotExist(err) {
				t.Errorf("CLI called for rejected server %q", server)
			}
			if _, err := os.Stat(outDir); !os.IsNotExist(err) {
				t.Errorf("export directory created for rejected server %q", server)
			}
		})
	}
}

// OPENCODE_PASSWORD reaches the child CLI only with --opencode-server, only
// through the environment, and never appears in any exporter output.
func testOpenCodeServerPassword(t *testing.T, root, tool string, responses map[string][]string) {
	const secret = "invented-sentinel-password-7f3a"
	const server = "http://127.0.0.1:4096"
	fixture := func(require bool) (string, string) {
		command, dir := fakeSessionMineOpenCode(t, responses)
		flags := map[string]string{}
		if require {
			flags["MINE_REQUIRE_PASSWORD"] = secret
		}
		data, _ := json.Marshal(flags)
		if err := os.WriteFile(filepath.Join(dir, "flags.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		return command, dir
	}
	childSaw := func(dir string) bool {
		data, _ := os.ReadFile(filepath.Join(dir, "environment"))
		return strings.Contains(string(data), "OPENCODE_PASSWORD=")
	}
	noLeak := func(label, output, outDir, fakeDir string) {
		t.Helper()
		if strings.Contains(output, secret) {
			t.Errorf("%s: password in exporter output: %s", label, output)
		}
		entries, _ := os.ReadDir(outDir)
		for _, entry := range entries {
			data, _ := os.ReadFile(filepath.Join(outDir, entry.Name()))
			if strings.Contains(string(data), secret) {
				t.Errorf("%s: password written to %s", label, entry.Name())
			}
		}
		argv, _ := os.ReadFile(filepath.Join(fakeDir, "argv"))
		if strings.Contains(string(argv), secret) {
			t.Errorf("%s: password in CLI argv: %s", label, argv)
		}
	}

	// With --opencode-server, the password is forwarded and the export works.
	command, dir := fixture(true)
	outDir := filepath.Join(t.TempDir(), "exports")
	out, code := run(t, root, []string{"OPENCODE_PASSWORD=" + secret}, tool, "--output", outDir, "--workspace", "/work/demo", "--opencode-cmd", command, "--opencode-server", server)
	if code != 0 || !strings.Contains(out, `"exports_written":1`) {
		t.Fatalf("password not forwarded (%d): %s", code, out)
	}
	noLeak("forwarded", out, outDir, dir)

	// Without the variable, the skip reason names the cause in fixed words
	// and does not echo CLI stderr.
	command, dir = fixture(true)
	outDir = filepath.Join(t.TempDir(), "exports")
	out, code = run(t, root, []string{"OPENCODE_PASSWORD="}, tool, "--output", outDir, "--workspace", "/work/demo", "--opencode-cmd", command, "--opencode-server", server)
	if code != 1 || !strings.Contains(out, "server requires a password; set OPENCODE_PASSWORD") || strings.Contains(out, "FAKE-CLI-STDERR") {
		t.Fatalf("missing-password reason (%d): %s", code, out)
	}
	if childSaw(dir) {
		t.Error("empty OPENCODE_PASSWORD forwarded")
	}

	// A wrong password that the CLI echoes on stderr reaches no output.
	const wrong = "wrong-pw-sentinel-5d1c"
	command, dir = fixture(true)
	outDir = filepath.Join(t.TempDir(), "exports")
	out, code = run(t, root, []string{"OPENCODE_PASSWORD=" + wrong}, tool, "--output", outDir, "--workspace", "/work/demo", "--opencode-cmd", command, "--opencode-server", server)
	if code != 1 || !strings.Contains(out, "server requires a password; set OPENCODE_PASSWORD") {
		t.Fatalf("wrong-password reason (%d): %s", code, out)
	}
	if strings.Contains(out, wrong) || strings.Contains(out, "FAKE-CLI-STDERR") {
		t.Errorf("wrong password or CLI stderr echoed: %s", out)
	}
	entries, _ := os.ReadDir(outDir)
	for _, entry := range entries {
		data, _ := os.ReadFile(filepath.Join(outDir, entry.Name()))
		if strings.Contains(string(data), wrong) {
			t.Errorf("wrong password written to %s", entry.Name())
		}
	}

	// Without --opencode-server, the child environment stays allowlisted.
	command, dir = fixture(false)
	outDir = filepath.Join(t.TempDir(), "exports")
	out, code = run(t, root, []string{"OPENCODE_PASSWORD=" + secret}, tool, "--output", outDir, "--workspace", "/work/demo", "--opencode-cmd", command)
	if code != 0 {
		t.Fatalf("service export (%d): %s", code, out)
	}
	if childSaw(dir) {
		t.Error("OPENCODE_PASSWORD forwarded without --opencode-server")
	}
	noLeak("service", out, outDir, dir)
}
