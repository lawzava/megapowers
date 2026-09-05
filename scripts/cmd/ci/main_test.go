package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testSHA = "0123456789abcdef0123456789abcdef01234567"

type scriptedExecutor struct {
	results []commandResult
	calls   []commandSpec
}

func (s *scriptedExecutor) Run(_ context.Context, spec commandSpec) (commandResult, error) {
	s.calls = append(s.calls, spec)
	if len(s.results) == 0 {
		return commandResult{}, errors.New("unexpected command")
	}
	result := s.results[0]
	s.results = s.results[1:]
	return result, nil
}

func TestVerifyReleaseRequiresPinnedTagAndCommitSignatures(t *testing.T) {
	runner := &scriptedExecutor{results: []commandResult{
		{Stdout: testSHA + "\n"},
		{},
		{Stderr: "[GNUPG:] VALIDSIG " + releaseKeyFingerprint + " 2026-09-05\n"},
		{Stderr: "[GNUPG:] VALIDSIG 0000000000000000000000000000000000000000 2026-09-05\n"},
	}}

	_, err := verifyRelease(context.Background(), "v1.2.3", testSHA, runner)
	if err == nil || !strings.Contains(err.Error(), "commit") || !strings.Contains(err.Error(), "not signed") {
		t.Fatalf("expected unpinned commit signature rejection, got %v", err)
	}
	if len(runner.calls) != 4 || runner.calls[1].Name != "gpg" || !strings.Contains(runner.calls[1].Stdin, "BEGIN PGP PUBLIC KEY BLOCK") {
		t.Fatalf("release verification did not import the embedded key: %#v", runner.calls)
	}
}

func TestEmbeddedReleaseKeyMatchesPublishedKey(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "release-signing-key.asc"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != releasePublicKey {
		t.Fatal("embedded release key differs from .github/release-signing-key.asc")
	}
}

func TestVerifyReleaseRejectsInvalidInputsBeforeCommands(t *testing.T) {
	for _, tc := range []struct {
		name     string
		tag      string
		expected string
	}{
		{name: "tag", tag: "release-1", expected: ""},
		{name: "sha", tag: "v1", expected: "abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &scriptedExecutor{}
			if _, err := verifyRelease(context.Background(), tc.tag, tc.expected, runner); err == nil {
				t.Fatal("expected validation failure")
			}
			if len(runner.calls) != 0 {
				t.Fatalf("ran commands for invalid input: %#v", runner.calls)
			}
		})
	}
}

func TestManifestVersionsMustMatch(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, "claude.json")
	codex := filepath.Join(dir, "codex.json")
	writeFile(t, claude, `{"version":"1.2.3"}`)
	writeFile(t, codex, `{"version":"1.2.4"}`)

	if _, _, err := readManifestVersions(claude, codex); err == nil || !strings.Contains(err.Error(), "differ") {
		t.Fatalf("expected version mismatch, got %v", err)
	}
	writeFile(t, codex, `{"version":"1.2.3"}`)
	gotClaude, gotCodex, err := readManifestVersions(claude, codex)
	if err != nil || gotClaude != "1.2.3" || gotCodex != "1.2.3" {
		t.Fatalf("unexpected versions %q %q, %v", gotClaude, gotCodex, err)
	}
}

func TestHashPluginTreeIsDeterministicAndRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "z.txt"), "z\n")
	writeFile(t, filepath.Join(root, "a.txt"), "a\n")
	if err := os.Chmod(filepath.Join(root, "z.txt"), 0o755); err != nil {
		t.Fatal(err)
	}

	first, err := hashPluginTree(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := hashPluginTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("tree hash changed without input change:\n%#v\n%#v", first, second)
	}
	if strings.Index(first.HashManifest, "a.txt") > strings.Index(first.HashManifest, "z.txt") {
		t.Fatalf("hash manifest is not sorted: %q", first.HashManifest)
	}
	if !strings.Contains(first.ModeManifest, "644 a.txt\n") || !strings.Contains(first.ModeManifest, "755 z.txt\n") {
		t.Fatalf("mode manifest omitted permissions: %q", first.ModeManifest)
	}
	if err := os.Symlink("a.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := hashPluginTree(root); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestEmitAttestationUsesTypedRunID(t *testing.T) {
	var out strings.Builder
	err := emitAttestation(&out, validAttestationInput())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatal(err)
	}
	if got["run_id"] != float64(42) || got["tag_signature"] != "verified" {
		t.Fatalf("unexpected attestation: %s", out.String())
	}
}

func TestEmitAttestationRejectsFalseGreenInputsBeforeWriting(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*attestationInput)
	}{
		{name: "missing version", mutate: func(in *attestationInput) { in.Version = "" }},
		{name: "tag mismatch", mutate: func(in *attestationInput) { in.Tag = "v9.9.9" }},
		{name: "invalid sha", mutate: func(in *attestationInput) { in.SHA = "abc" }},
		{name: "nonpositive run ID", mutate: func(in *attestationInput) { in.RunID = "0" }},
		{name: "nonnumeric run ID", mutate: func(in *attestationInput) { in.RunID = "nope" }},
		{name: "manifest mismatch", mutate: func(in *attestationInput) { in.CodexVersion = "1.2.4" }},
		{name: "invalid tree hash", mutate: func(in *attestationInput) { in.PluginTreeSHA256 = "abc" }},
		{name: "invalid mode hash", mutate: func(in *attestationInput) { in.ModesManifestSHA256 = "ABCDEF" + strings.Repeat("b", 58) }},
		{name: "claude smoke failed", mutate: func(in *attestationInput) { in.SmokeClaude = "fail" }},
		{name: "codex smoke missing", mutate: func(in *attestationInput) { in.SmokeCodex = "" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := validAttestationInput()
			tc.mutate(&input)
			var out strings.Builder
			if err := emitAttestation(&out, input); err == nil {
				t.Fatal("expected attestation validation failure")
			}
			if out.Len() != 0 {
				t.Fatalf("wrote a false-green attestation: %q", out.String())
			}
		})
	}
}

func TestFastForwardReleaseRejectsDivergedBranch(t *testing.T) {
	runner := &scriptedExecutor{results: []commandResult{
		{Stdout: testSHA + "\n"},
		{},
		{},
		{ExitCode: 1},
	}}
	err := fastForwardRelease(context.Background(), "v1.2.3", runner)
	if err == nil || !strings.Contains(err.Error(), "refusing non-fast-forward") {
		t.Fatalf("expected non-fast-forward rejection, got %v", err)
	}
	for _, call := range runner.calls {
		if call.Name == "git" && len(call.Args) > 0 && call.Args[0] == "push" {
			t.Fatalf("pushed after ancestry failure: %#v", runner.calls)
		}
	}
}

func TestFastForwardReleaseRejectsInvalidResolvedSHA(t *testing.T) {
	runner := &scriptedExecutor{results: []commandResult{{Stdout: "not-a-sha\n"}}}
	if err := fastForwardRelease(context.Background(), "v1.2.3", runner); err == nil || !strings.Contains(err.Error(), "invalid commit SHA") {
		t.Fatalf("expected invalid SHA rejection, got %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("contacted the remote after invalid tag resolution: %#v", runner.calls)
	}
}

func TestFastForwardReleaseCreatesMissingBranchAtExactTagCommit(t *testing.T) {
	runner := &scriptedExecutor{results: []commandResult{
		{Stdout: testSHA + "\n"},
		{ExitCode: 2},
		{},
	}}
	if err := fastForwardRelease(context.Background(), "v1.2.3", runner); err != nil {
		t.Fatal(err)
	}
	wantPush := commandSpec{Name: "git", Args: []string{"push", "origin", testSHA + ":refs/heads/release"}}
	if !reflect.DeepEqual(runner.calls[len(runner.calls)-1], wantPush) {
		t.Fatalf("unexpected push command: %#v", runner.calls)
	}
}

func TestFastForwardReleaseFailsClosedOnRemoteError(t *testing.T) {
	runner := &scriptedExecutor{results: []commandResult{
		{Stdout: testSHA + "\n"},
		{ExitCode: 1, Stderr: "authentication failed"},
	}}
	if err := fastForwardRelease(context.Background(), "v1.2.3", runner); err == nil || !strings.Contains(err.Error(), "inspect remote") {
		t.Fatalf("expected remote inspection failure, got %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("continued after remote error: %#v", runner.calls)
	}
}

func TestStaticcheckChecksWholeModuleAsPackages(t *testing.T) {
	runner := &scriptedExecutor{results: []commandResult{{}}}

	if err := runStaticcheck(context.Background(), "v0.6.1", runner); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("got %d staticcheck calls", len(runner.calls))
	}
	want := commandSpec{Name: "go", Args: []string{"run", "honnef.co/go/tools/cmd/staticcheck@v0.6.1", "./..."}, ShowOutput: true}
	if !reflect.DeepEqual(runner.calls[0], want) {
		t.Fatalf("staticcheck did not analyze the whole module as packages: %#v", runner.calls[0])
	}
}

func TestSmokeGateUsesLastHarnessResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.tsv")
	writeFile(t, path, "claude\tFAIL\told\nclaude\tPASS\tnew\ncodex\tSKIP\tmissing\n")
	if err := requireSmokePass(path, "claude"); err != nil {
		t.Fatal(err)
	}
	if err := requireSmokePass(path, "codex"); err == nil || !strings.Contains(err.Error(), "skip") {
		t.Fatalf("expected codex smoke rejection, got %v", err)
	}
}

func TestReleaseSmokeStreamsMaintainerOutput(t *testing.T) {
	githubEnv := filepath.Join(t.TempDir(), "github-env")
	writeFile(t, githubEnv, "")
	env := map[string]string{"HARNESS": "codex", "GITHUB_ENV": githubEnv}
	runner := &scriptedExecutor{results: []commandResult{{}}}
	if err := runReleaseSmoke(context.Background(), func(key string) string { return env[key] }, runner); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || !runner.calls[0].ShowOutput {
		t.Fatalf("release smoke hid maintainer output: %#v", runner.calls)
	}
}

func TestWindowsHookChecksRequirePropagatedFailureAndDenyJSON(t *testing.T) {
	runner := &scriptedExecutor{results: []commandResult{
		{},
		{Stdout: "Do not claim a skill without loading it."},
		{Stdout: "ASD-STE100-inspired. Do not claim a skill without loading it."},
		{},
		{Stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"blocked"}}`},
		{ExitCode: 1, Stderr: "cannot evaluate command input"},
		{ExitCode: 1},
		{ExitCode: 1},
		{ExitCode: 1},
	}}
	if err := checkWindowsHooks(context.Background(), `plugins\megapowers\hooks\run-hook.cmd`, runner); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 9 {
		t.Fatalf("expected nine direct cmd.exe checks, got %d", len(runner.calls))
	}
	for _, call := range runner.calls {
		if call.Name != "cmd.exe" || len(call.Args) == 0 || call.Args[0] != "/c" {
			t.Fatalf("check did not exercise cmd.exe directly: %#v", call)
		}
	}

	broken := &scriptedExecutor{results: []commandResult{
		{},
		{Stdout: "Do not claim a skill without loading it."},
		{Stdout: "ASD-STE100-inspired. Do not claim a skill without loading it."},
		{},
		{Stdout: `{"hookSpecificOutput":{"permissionDecision":"deny"}}`},
		{},
	}}
	if err := checkWindowsHooks(context.Background(), "hook.cmd", broken); err == nil || !strings.Contains(err.Error(), "propagate") {
		t.Fatalf("expected propagation failure, got %v", err)
	}
}

func TestEvalGateRequiresBothSteps(t *testing.T) {
	if err := requireEvalSuccess("success", "success"); err != nil {
		t.Fatal(err)
	}
	if err := requireEvalSuccess("success", "failure"); err == nil || !strings.Contains(err.Error(), "score") {
		t.Fatalf("expected score failure, got %v", err)
	}
}

func TestAppendWorkflowValuesRejectsLineInjectionBeforeWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github-output")
	writeFile(t, path, "")
	if err := appendWorkflowValues(path, "tag=v1.2.3", "sha=abc\nforged=value"); err == nil {
		t.Fatal("expected workflow output line rejection")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) != 0 {
		t.Fatalf("partially wrote workflow outputs: %q", content)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func validAttestationInput() attestationInput {
	return attestationInput{
		Version: "1.2.3", Tag: "v1.2.3", SHA: testSHA, RunID: "42",
		ClaudeVersion: "1.2.3", CodexVersion: "1.2.3",
		PluginTreeSHA256: strings.Repeat("a", 64), ModesManifestSHA256: strings.Repeat("b", 64),
		SmokeClaude: "pass", SmokeCodex: "pass",
	}
}
