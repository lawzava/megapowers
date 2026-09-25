package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type gateOutput struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

func runGate(t *testing.T, env map[string]string, session, command string) (string, string, int) {
	t.Helper()
	input := `{"session_id":"` + session + `","tool_name":"Bash","tool_input":{"command":` + strconvQuote(command) + `}}`
	var stdout, stderr bytes.Buffer
	rc := runHook([]string{"deny-destructive"}, func(key string) string { return env[key] }, strings.NewReader(input), &stdout, &stderr)
	return stdout.String(), stderr.String(), rc
}

func strconvQuote(text string) string {
	quoted, err := json.Marshal(text)
	if err != nil {
		panic(err)
	}
	return string(quoted)
}

func decodeGate(t *testing.T, stdout string) gateOutput {
	t.Helper()
	var parsed gateOutput
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("decode gate output %q: %v", stdout, err)
	}
	if strings.Contains(stdout, "permissionDecision") {
		t.Fatalf("gate context must never carry a permission decision: %q", stdout)
	}
	if parsed.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Fatalf("hookEventName = %q, want PreToolUse", parsed.HookSpecificOutput.HookEventName)
	}
	return parsed
}

func TestGateContextNamesSkillForCompletionAndEffectCommands(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		command string
		skill   string
	}{
		{command: "git commit -m 'fix: thing'", skill: "verify-and-finish"},
		{command: "git -C /repo push origin main", skill: "verify-and-finish"},
		{command: "cd repo && git add . && git commit -m x", skill: "verify-and-finish"},
		{command: "git commit -m 'git push later'", skill: "verify-and-finish"},
		{command: "gh pr create --fill", skill: "verify-and-finish"},
		{command: "gh pr merge 12 --squash", skill: "verify-and-finish"},
		{command: "gh release create v1.0.0 --notes x", skill: "verify-and-finish"},
		{command: "npm publish", skill: "safe-effects"},
		{command: "pnpm publish --access public", skill: "safe-effects"},
		{command: "yarn npm publish", skill: "safe-effects"},
		{command: "yarn publish", skill: "safe-effects"},
		{command: "cargo publish", skill: "safe-effects"},
		{command: "docker push registry/app:1", skill: "safe-effects"},
		{command: "kubectl apply -f deploy.yaml", skill: "safe-effects"},
		{command: "kubectl delete deployment app", skill: "safe-effects"},
		{command: "helm upgrade --install app ./chart", skill: "safe-effects"},
		{command: "helm install app ./chart", skill: "safe-effects"},
		{command: "terraform apply -auto-approve", skill: "safe-effects"},
		{command: "terraform destroy", skill: "safe-effects"},
		{command: "railway up", skill: "safe-effects"},
		{command: "railway deploy", skill: "safe-effects"},
		{command: "fly deploy", skill: "safe-effects"},
		{command: "flyctl deploy --remote-only", skill: "safe-effects"},
		{command: "vercel --prod", skill: "safe-effects"},
		{command: "vercel deploy --prod", skill: "safe-effects"},
		{command: "gh api -X POST repos/o/r/issues -f title=x", skill: "safe-effects"},
		{command: "gh api --method DELETE repos/o/r/issues/1", skill: "safe-effects"},
		{command: "sudo docker push registry/app:1", skill: "safe-effects"},
		{command: "gh --repo o/r pr create --fill", skill: "verify-and-finish"},
		{command: "gh -R o/r pr merge 1", skill: "verify-and-finish"},
		{command: "gh --repo=o/r release create v2", skill: "verify-and-finish"},
		{command: "kubectl apply --dry-run=none -f x.yaml", skill: "safe-effects"},
	} {
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runGate(t, env, "session-"+tt.skill, tt.command)
			if rc != 0 || stderr != "" {
				t.Fatalf("rc=%d stderr=%q", rc, stderr)
			}
			parsed := decodeGate(t, stdout)
			context := parsed.HookSpecificOutput.AdditionalContext
			if !strings.Contains(context, "megapowers "+tt.skill+" skill") || strings.Count(context, "\n") > 1 {
				t.Fatalf("additionalContext = %q, want one short line naming %s", context, tt.skill)
			}
		})
	}
}

func TestGateContextStaysQuietForOrdinaryCommands(t *testing.T) {
	t.Parallel()

	for _, command := range []string{
		"git status",
		"git log --oneline -5",
		"git commit --dry-run",
		"gh api repos/o/r",
		"gh pr view 12",
		"gh pr list",
		"kubectl get pods",
		"terraform plan",
		"docker build -t app .",
		"npm test",
		"vercel dev",
		"vercel",
		"railway logs",
		"fly status",
		"echo 'git push'",
		"cat notes.txt | grep 'npm publish'",
		"git push --dry-run",
		"npm publish --dry-run",
		"pnpm publish --dry-run",
		"cargo publish --dry-run",
		"kubectl apply --dry-run=client -f x.yaml",
		"kubectl delete --dry-run=server -f x.yaml",
		"helm upgrade --dry-run app ./chart",
		"helm install --dry-run app ./chart",
	} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runGate(t, env, "session-quiet", command)
			if rc != 0 || stdout != "" || stderr != "" {
				t.Fatalf("rc=%d stdout=%q stderr=%q, want silence", rc, stdout, stderr)
			}
		})
	}
}

func TestGateContextDeniedCommandOnlyDenies(t *testing.T) {
	t.Parallel()

	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	stdout, _, rc := runGate(t, env, "session-deny", "git commit -m x && rm -rf /")
	if rc != 0 || !strings.Contains(stdout, `"permissionDecision":"deny"`) || strings.Contains(stdout, "additionalContext") {
		t.Fatalf("stdout=%q, want a plain deny", stdout)
	}
}

func TestGateContextDeduplicatesPerSessionAndSkill(t *testing.T) {
	t.Parallel()

	cache := t.TempDir()
	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": cache}
	first, _, _ := runGate(t, env, "session-a", "git commit -m one")
	if !strings.Contains(first, "verify-and-finish") {
		t.Fatalf("first emission missing: %q", first)
	}
	second, _, _ := runGate(t, env, "session-a", "git push")
	if second != "" {
		t.Fatalf("second emission for same session and skill should be silent: %q", second)
	}
	other, _, _ := runGate(t, env, "session-a", "docker push app")
	if !strings.Contains(other, "safe-effects") {
		t.Fatalf("different skill in same session must emit: %q", other)
	}
	another, _, _ := runGate(t, env, "session-b", "git push")
	if !strings.Contains(another, "verify-and-finish") {
		t.Fatalf("different session must emit: %q", another)
	}
	markers, err := filepath.Glob(filepath.Join(cache, "gate-context", "*"))
	if err != nil || len(markers) != 3 {
		t.Fatalf("markers = %v (err %v), want 3", markers, err)
	}
	for _, marker := range markers {
		if strings.Contains(filepath.Base(marker), "session-") {
			t.Fatalf("marker name leaks raw session id: %s", marker)
		}
	}
}

func TestGateContextFailsOpenWhenMarkerDirIsUnusable(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": blocked}
	for i := 0; i < 2; i++ {
		stdout, stderr, rc := runGate(t, env, "session-c", "git commit -m x")
		if rc != 0 || stderr != "" || !strings.Contains(stdout, "verify-and-finish") {
			t.Fatalf("attempt %d: rc=%d stdout=%q stderr=%q, want emission despite marker failure", i, rc, stdout, stderr)
		}
	}
}

func TestGateContextWithoutSessionIDEmitsEveryTime(t *testing.T) {
	t.Parallel()

	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	for i := 0; i < 2; i++ {
		stdout, _, _ := runGate(t, env, "", "git commit -m x")
		if !strings.Contains(stdout, "verify-and-finish") {
			t.Fatalf("attempt %d: %q", i, stdout)
		}
	}
}

func TestGateContextEmitsOnBothHarnesses(t *testing.T) {
	t.Parallel()

	for _, harness := range []string{"claude", "codex", ""} {
		env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir(), "MEGAPOWERS_HARNESS": harness}
		stdout, _, _ := runGate(t, env, "session-h", "git push origin main")
		if !strings.Contains(stdout, "verify-and-finish") {
			t.Fatalf("harness %q: %q", harness, stdout)
		}
	}
}
