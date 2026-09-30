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
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
		AdditionalContext        string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

func runGate(t *testing.T, env map[string]string, session, command string) (string, string, int) {
	t.Helper()
	return runGateWithTranscript(t, env, session, "", command)
}

func runGateWithTranscript(t *testing.T, env map[string]string, session, transcript, command string) (string, string, int) {
	t.Helper()
	input := `{"session_id":"` + session + `","transcript_path":` + strconvQuote(transcript) + `,"tool_name":"Bash","tool_input":{"command":` + strconvQuote(command) + `}}`
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
	if parsed.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Fatalf("hookEventName = %q, want PreToolUse", parsed.HookSpecificOutput.HookEventName)
	}
	return parsed
}

// gateText returns whichever gate message the hook emitted: the one-time
// deny reason or the non-blocking fallback context.
func gateText(parsed gateOutput) string {
	return parsed.HookSpecificOutput.PermissionDecisionReason + parsed.HookSpecificOutput.AdditionalContext
}

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
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
		{command: "gh api graphql -f query='mutation { addStar(input: {starrableId: \"x\"}) { clientMutationId } }'", skill: "safe-effects"},
		{command: "gh api graphql --raw-field query='mutation{deleteIssue(input:{issueId:\"x\"}){clientMutationId}}'", skill: "safe-effects"},
		{command: "gh api repos/o/r/pulls/1/reviews --input review.json", skill: "safe-effects"},
		{command: "gh pr comment 12 --body hi", skill: "safe-effects"},
		{command: "gh pr review 12 --approve", skill: "safe-effects"},
		{command: "gh pr edit 12 --add-label bug", skill: "safe-effects"},
		{command: "gh pr close 12", skill: "safe-effects"},
		{command: "gh issue create --title x --body y", skill: "safe-effects"},
		{command: "gh issue comment 3 --body y", skill: "safe-effects"},
		{command: "gh issue close 3", skill: "safe-effects"},
		{command: "gh -R o/r issue edit 3 --title z", skill: "safe-effects"},
		{command: "kubectl --context production apply -f deployment.yaml", skill: "safe-effects"},
		{command: "kubectl -n prod delete pod web-1", skill: "safe-effects"},
		{command: "helm --kube-context production upgrade app ./chart", skill: "safe-effects"},
		{command: "helm -n prod install app ./chart", skill: "safe-effects"},
		{command: "docker --context remote push registry/app:1", skill: "safe-effects"},
		{command: "docker -H ssh://host push registry/app:1", skill: "safe-effects"},
		{command: "npm --prefix packages/app publish", skill: "safe-effects"},
		{command: "pnpm -C packages/app publish", skill: "safe-effects"},
	} {
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			stdout, stderr, rc := runGate(t, env, "session-"+tt.skill, tt.command)
			if rc != 0 || stderr != "" {
				t.Fatalf("rc=%d stderr=%q", rc, stderr)
			}
			text := gateText(decodeGate(t, stdout))
			if !strings.Contains(text, "megapowers "+tt.skill+" skill") || strings.Count(text, "\n") > 1 {
				t.Fatalf("gate message = %q, want one short line naming %s", text, tt.skill)
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
		"gh api graphql -f query='{ viewer { login } }'",
		"gh api graphql -f query='query($o:String!){ repository(owner:$o, name:\"r\"){ id } }' -f o=x",
		"gh api repos/o/r/pulls -X GET -f state=open",
		"gh api --method=GET search/issues -f q=bug",
		"gh pr diff 12",
		"gh pr checks 12",
		"gh issue view 3",
		"gh issue list --state open",
		"kubectl --context production get pods",
		"helm --kube-context production status app",
		"docker --context remote ps",
		"npm --prefix packages/app test",
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
	if rc != 0 || !strings.Contains(stdout, `"permissionDecision":"deny"`) || strings.Contains(stdout, "additionalContext") || strings.Contains(stdout, "verify-and-finish") {
		t.Fatalf("stdout=%q, want the destructive deny alone", stdout)
	}
}

func TestGateDeniesOncePerSessionAndSkill(t *testing.T) {
	t.Parallel()

	cache := t.TempDir()
	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": cache}
	first, _, _ := runGate(t, env, "session-a", "git commit -m one")
	parsed := decodeGate(t, first)
	if parsed.HookSpecificOutput.PermissionDecision != "deny" || !strings.Contains(parsed.HookSpecificOutput.PermissionDecisionReason, "verify-and-finish") {
		t.Fatalf("first completion command must be denied once naming the skill: %q", first)
	}
	second, _, _ := runGate(t, env, "session-a", "git commit -m one")
	if second != "" {
		t.Fatalf("retry in the same session must run unblocked and silent: %q", second)
	}
	other, _, _ := runGate(t, env, "session-a", "docker push app")
	if !strings.Contains(other, `"permissionDecision":"deny"`) || !strings.Contains(other, "safe-effects") {
		t.Fatalf("different skill in same session must deny once: %q", other)
	}
	another, _, _ := runGate(t, env, "session-b", "git push")
	if !strings.Contains(another, `"permissionDecision":"deny"`) {
		t.Fatalf("different session must deny once: %q", another)
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

func TestGateDeniesBothSkillsInOneReason(t *testing.T) {
	t.Parallel()

	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	stdout, _, _ := runGate(t, env, "session-both", "git push && docker push app")
	reason := decodeGate(t, stdout).HookSpecificOutput.PermissionDecisionReason
	if !strings.Contains(reason, "verify-and-finish") || !strings.Contains(reason, "safe-effects") {
		t.Fatalf("reason = %q, want both skills", reason)
	}
}

func TestGateStaysQuietWhenSkillAlreadyLoaded(t *testing.T) {
	t.Parallel()

	for name, line := range map[string]string{
		"claude skill call": `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"megapowers:verify-and-finish"}}]}}`,
		"claude slash body": `{"type":"user","message":{"content":"Base directory for this skill: /cache/megapowers/0.31.1/skills/verify-and-finish\n\n# Verify"}}`,
		"codex body read":   `{"type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"sed -n 1,200p /p/skills/verify-and-finish/SKILL.md\"}"}}`,
		"codex custom tool": `{"type":"response_item","payload":{"type":"custom_tool_call","input":"cat /p/skills/verify-and-finish/SKILL.md"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
			transcript := writeTranscript(t, `{"type":"user","message":{"content":"commit it"}}`, line)
			stdout, stderr, rc := runGateWithTranscript(t, env, "session-loaded", transcript, "git commit -m x")
			if rc != 0 || stdout != "" || stderr != "" {
				t.Fatalf("rc=%d stdout=%q stderr=%q, want silence after the skill loaded", rc, stdout, stderr)
			}
		})
	}
}

func TestGateIgnoresCatalogMentionsOfTheSkill(t *testing.T) {
	t.Parallel()

	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	transcript := writeTranscript(t,
		`{"type":"session_meta","payload":{"instructions":"- verify-and-finish: Use to verify (file: /p/skills/verify-and-finish/SKILL.md)"}}`,
		`{"type":"user","message":{"content":"megapowers:verify-and-finish is listed in the catalog"}}`,
	)
	stdout, _, _ := runGateWithTranscript(t, env, "session-catalog", transcript, "git commit -m x")
	if !strings.Contains(stdout, `"permissionDecision":"deny"`) {
		t.Fatalf("a catalog or prose mention is not a load: %q", stdout)
	}
}

func TestGateFallsBackToContextWhenMarkerDirIsUnusable(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": blocked}
	for i := 0; i < 2; i++ {
		stdout, stderr, rc := runGate(t, env, "session-c", "git commit -m x")
		parsed := decodeGate(t, stdout)
		if rc != 0 || stderr != "" || parsed.HookSpecificOutput.PermissionDecision != "" || !strings.Contains(parsed.HookSpecificOutput.AdditionalContext, "verify-and-finish") {
			t.Fatalf("attempt %d: rc=%d stdout=%q stderr=%q, want non-blocking context when the once-marker cannot be recorded", i, rc, stdout, stderr)
		}
	}
}

func TestGateWithoutSessionIDNeverBlocks(t *testing.T) {
	t.Parallel()

	env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir()}
	for i := 0; i < 2; i++ {
		stdout, _, _ := runGate(t, env, "", "git commit -m x")
		parsed := decodeGate(t, stdout)
		if parsed.HookSpecificOutput.PermissionDecision != "" || !strings.Contains(parsed.HookSpecificOutput.AdditionalContext, "verify-and-finish") {
			t.Fatalf("attempt %d: %q, want non-blocking context", i, stdout)
		}
	}
}

func TestGateDeniesOnBothHarnesses(t *testing.T) {
	t.Parallel()

	for _, harness := range []string{"claude", "codex", ""} {
		env := map[string]string{"MEGAPOWERS_HOOK_CACHE_DIR": t.TempDir(), "MEGAPOWERS_HARNESS": harness}
		stdout, _, _ := runGate(t, env, "session-h", "git push origin main")
		if !strings.Contains(stdout, `"permissionDecision":"deny"`) || !strings.Contains(stdout, "verify-and-finish") {
			t.Fatalf("harness %q: %q", harness, stdout)
		}
	}
}
