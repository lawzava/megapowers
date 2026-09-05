package contracts

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGitleaksConfigScopesFixtureExceptions(t *testing.T) {
	binary, err := exec.LookPath("gitleaks")
	if err != nil {
		t.Skip("Gitleaks is checked separately in CI")
	}
	root := repoRoot(t)
	paths := []string{"scripts/cmd/ci/release.go", "scripts/contracts/review_regression_test.go"}
	for _, mutate := range append([]string{""}, paths...) {
		name := mutate
		if name == "" {
			name = "public key and synthetic provider"
		}
		t.Run(name, func(t *testing.T) {
			fixture := t.TempDir()
			for _, path := range append([]string{".gitleaks.toml"}, paths...) {
				content, err := os.ReadFile(filepath.Join(root, path))
				if err != nil {
					t.Fatal(err)
				}
				if path == mutate {
					// Generate a scanner probe with no provider or account behind it.
					var probe [48]byte
					if _, err := rand.Read(probe[:]); err != nil {
						t.Fatal(err)
					}
					content = append(content, []byte("\nvar serviceAPIKey = \""+base64.RawURLEncoding.EncodeToString(probe[:])+"\"\n")...)
				}
				target := filepath.Join(fixture, path)
				if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, content, 0600); err != nil {
					t.Fatal(err)
				}
			}
			report := filepath.Join(t.TempDir(), "findings.json")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "dir", ".", "--no-banner", "--redact", "--report-format", "json", "--report-path", report)
			cmd.Dir = fixture
			err := cmd.Run()
			want := 0
			if mutate != "" {
				want = 1
			}
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal("Gitleaks did not execute")
				}
				code = exit.ExitCode()
			}
			if code != want {
				t.Fatalf("Gitleaks exit = %d, want %d", code, want)
			}
			data, err := os.ReadFile(report)
			if err != nil {
				t.Fatal(err)
			}
			var findings []struct{ File, RuleID string }
			if err := json.Unmarshal(data, &findings); err != nil {
				t.Fatal(err)
			}
			if mutate != "" && (len(findings) != 1 || filepath.ToSlash(findings[0].File) != mutate || findings[0].RuleID != "generic-api-key") {
				t.Fatal("Gitleaks did not isolate the added scanner probe")
			}
		})
	}
}
