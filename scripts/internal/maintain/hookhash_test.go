package maintain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHookTree(t *testing.T, key string) string {
	t.Helper()
	root := t.TempDir()
	hooks := filepath.Join(root, "plugins", "megapowers", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"b.go":      "package main\n",
		"a.go":      "package main\n\nfunc main() {}\n",
		"a_test.go": "package main\n",
		"run-hook.cmd": "set \"RUNNER_PREFIX=!CACHE_DIR!\\megapowers-hook-" + key + "-windows-%PROCESSOR_ARCHITECTURE%\"\n" +
			"runner_prefix=\"$cache_dir/megapowers-hook-" + key + "-$platform_os-$platform_arch\"\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(hooks, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func expectedHookKey() string {
	hash := sha256.New()
	fmt.Fprintf(hash, "%s\x00", "a.go")
	io.WriteString(hash, "package main\n\nfunc main() {}\n")
	fmt.Fprintf(hash, "%s\x00", "b.go")
	io.WriteString(hash, "package main\n")
	return fmt.Sprintf("%x", hash.Sum(nil))[:16]
}

func TestHookHashReportsStaleLauncher(t *testing.T) {
	root := writeHookTree(t, "0000000000000000")
	var stdout, stderr bytes.Buffer
	if code := Main(context.Background(), root, []string{"hook-hash"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, want 1\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), expectedHookKey()) || !strings.Contains(stderr.String(), "hook-hash --write") {
		t.Fatalf("stderr lacks the expected key or fix command:\n%s", stderr.String())
	}
}

func TestHookHashWriteUpdatesBothLauncherBranches(t *testing.T) {
	root := writeHookTree(t, "0000000000000000")
	var stdout, stderr bytes.Buffer
	if code := Main(context.Background(), root, []string{"hook-hash", "--write"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d\nstderr: %s", code, stderr.String())
	}
	launcher, err := os.ReadFile(filepath.Join(root, "plugins", "megapowers", "hooks", "run-hook.cmd"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(launcher), "megapowers-hook-"+expectedHookKey()+"-"); got != 2 {
		t.Fatalf("updated key occurrences = %d, want 2:\n%s", got, launcher)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Main(context.Background(), root, []string{"hook-hash"}, &stdout, &stderr); code != 0 {
		t.Fatalf("check after write exit = %d\nstderr: %s", code, stderr.String())
	}
}
