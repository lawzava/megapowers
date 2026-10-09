//go:build linux

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func eventKinds(events []actorEvent) string {
	kinds := make([]string, 0, len(events))
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	return strings.Join(kinds, ",")
}

func countKind(events []actorEvent, kind string) int {
	count := 0
	for _, event := range events {
		if event.Kind == kind {
			count++
		}
	}
	return count
}

func TestClassifyCommandOutwardWrites(t *testing.T) {
	for _, command := range []string{
		"git push origin main",
		"git -C repo push",
		"/usr/bin/git push --force-with-lease",
		"env GH_TOKEN=x gh pr create --fill",
		"gh pr merge 12 --squash",
		"gh pr comment 12 --body hi",
		"gh pr review 12 --approve",
		"gh pr edit 12 --title x",
		"gh pr close 12",
		"gh -R owner/repo issue create --title x",
		"gh issue comment 3 -b hi",
		"gh issue edit 3 --add-label bug",
		"gh issue close 3",
		"gh release create v1.0.0",
		"gh api repos/o/r/issues -f title=x",
		"gh api -X POST repos/o/r/labels",
		"gh api --method=DELETE repos/o/r/labels/x",
		"gh api repos/o/r/contents/f --input body.json",
		"curl -X POST https://api.example.com/v1/items",
		"curl -XPUT https://api.example.com/v1/items/1",
		`curl -d '{"a":1}' https://api.example.com/v1`,
		"curl --data-binary @payload.json https://hooks.example.com/x",
		"curl --json '{}' https://api.example.com",
		"curl -F file=@a.txt https://upload.example.com",
		"curl -T a.txt https://upload.example.com/a.txt",
		"curl -sS -H 'Authorization: Bearer x' --request PATCH https://api.example.com/x",
		"curl -X POST $WEBHOOK_URL",
		"wget --post-data a=1 https://example.com/form",
		"wget --method=DELETE https://example.com/x",
		"http POST api.example.com/items name=x",
		"https api.example.com/items name=x",
		"xh PUT api.example.com/items/1 name=x",
		"npm publish",
		"pnpm publish --access public",
		"yarn publish",
		"cargo publish",
		"docker push registry.example.com/app:1",
		"kubectl apply -f deploy.yaml",
		"kubectl -n prod delete pod x",
		"helm upgrade --install app ./chart",
		"helm install app ./chart",
		"terraform apply -auto-approve",
		"tofu destroy",
		"terraform -chdir=infra apply",
		"wrangler deploy",
		"npx wrangler deploy",
		"npx -y wrangler@3 pages deploy dist",
		"pnpm exec wrangler versions deploy",
		"pnpm wrangler publish",
		"yarn wrangler deploy",
		"bunx wrangler deploy",
		"railway up",
		"railway deploy",
		"fly deploy",
		"flyctl deploy --remote-only",
		"vercel --prod",
		"npx vercel deploy --prod",
		"/bin/bash -lc 'go build ./... && git push origin HEAD'",
		"cd app && npm publish",
		"sudo docker push app:1",
		"cat > notes.md <<'EOF'\nnpm publish\nEOF\ngit push",
	} {
		events := classifyCommand(command, 7)
		if countKind(events, "external_write") != 1 {
			t.Errorf("classifyCommand(%q) = [%s], want one external_write", command, eventKinds(events))
			continue
		}
		for _, event := range events {
			if event.Kind == "external_write" && event.RC != 7 {
				t.Errorf("classifyCommand(%q) external_write rc = %d, want the tool rc 7", command, event.RC)
			}
		}
	}
}

func TestClassifyCommandOutwardWriteNearMisses(t *testing.T) {
	for _, command := range []string{
		"curl https://api.example.com/v1/items",
		"curl -X GET https://api.example.com",
		"curl -G -d q=x https://api.example.com/search",
		"curl -I https://example.com",
		"curl -X POST http://localhost:8080/x",
		"curl -d a=1 http://127.0.0.1:3000/x",
		"curl -d a=1 http://127.8.9.10/x",
		"curl -d a=1 http://[::1]:3000/x",
		"curl -X POST http://app.localhost/x",
		"wget https://example.com/file.tar.gz",
		"wget --post-data a=1 http://localhost/form",
		"http api.example.com/items",
		"http GET api.example.com/items q==x",
		"http POST :3000/items name=x",
		"echo git push",
		`grep "gh pr create" notes.md`,
		"printf 'curl -X POST https://example.com'",
		"git push --dry-run",
		"git push -n origin main",
		"git status",
		"git log --grep push",
		"gh pr view 12",
		"gh pr list",
		"gh issue view 3",
		"gh api repos/o/r/pulls",
		"gh api -X GET repos/o/r/issues -f state=open",
		"gh release view v1",
		"npm publish --dry-run",
		"cargo publish --dry-run",
		"npm test",
		"docker pull alpine",
		"docker build -t x .",
		"kubectl get pods",
		"kubectl apply --dry-run=client -f deploy.yaml",
		"helm install --dry-run app ./chart",
		"helm template app ./chart",
		"terraform plan",
		"wrangler dev",
		"npx wrangler deploy --dry-run",
		"railway status",
		"fly status",
		"vercel",
		"vercel dev",
		"go test ./...",
		"cat > notes.md <<'EOF'\ngit push origin main\nnpm publish\nEOF",
		"curl -H 'Content-Type: application/json' -d '{}' http://localhost:3000/x",
		"curl -fsSL https://example.com/install.sh",
		"git commit -m 'then git push'",
	} {
		if events := classifyCommand(command, 0); countKind(events, "external_write") != 0 {
			t.Errorf("classifyCommand(%q) = [%s], want no external_write", command, eventKinds(events))
		}
	}
}

func TestClassifyCommandGitCommit(t *testing.T) {
	for _, command := range []string{
		"git commit -m 'add multiply'",
		"git -C repo commit -am wip",
		"git -c user.name=x commit -m y",
		"git commit --amend --no-edit",
		"/bin/bash -lc 'git add -A && git commit -m x'",
		"cd repo && git commit -m x",
	} {
		events := classifyCommand(command, 3)
		if countKind(events, "git_commit") != 1 || events[len(events)-1].RC != 3 {
			t.Errorf("classifyCommand(%q) = %+v, want one git_commit with rc 3", command, events)
		}
	}
	for _, command := range []string{
		"git commit --dry-run",
		"echo git commit",
		"git log --oneline",
		"grep 'git commit' notes.md",
		"git commit-tree abc",
		"git show HEAD",
	} {
		if events := classifyCommand(command, 0); countKind(events, "git_commit") != 0 {
			t.Errorf("classifyCommand(%q) = [%s], want no git_commit", command, eventKinds(events))
		}
	}
	if got := eventKinds(classifyCommand("git commit -m x && git push origin main", 0)); got != "git_commit,external_write" {
		t.Errorf("segment order lost: [%s]", got)
	}
}

func TestClaudeTraceRecordsCommitsAndOutwardWrites(t *testing.T) {
	bash := func(id, command string, isError bool) string {
		use, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "id": id, "name": "Bash", "input": map[string]any{"command": command}}}}})
		result, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": isError}}}})
		return string(use) + "\n" + string(result) + "\n"
	}
	trace := []byte("{\"type\":\"system\",\"subtype\":\"init\"}\n" +
		bash("t1", "go test ./...", false) +
		bash("t2", `git commit -m "add multiply"`, false) +
		bash("t3", "git push origin main", true) +
		bash("t4", "gh pr create --fill", true) +
		"{\"type\":\"result\",\"is_error\":false,\"result\":\"done\"}\n")
	_, events, complete := normalizeTrace("claude", trace, 0)
	if !complete || eventKinds(events) != "test,git_commit,external_write,external_write,trace_complete" {
		t.Fatalf("complete=%v events=%+v", complete, events)
	}
	if events[1].RC != 0 || events[2].RC != 1 || events[3].RC != 1 || events[1].Step >= events[2].Step {
		t.Fatalf("rc or order lost: %+v", events)
	}
}

func TestCodexTraceRecordsCommitsAndOutwardWrites(t *testing.T) {
	trace := []byte(`{"method":"item/completed","params":{"item":{"id":"c1","type":"commandExecution","status":"completed","command":"/bin/bash -lc 'go test ./...'","exitCode":0}}}
{"method":"item/completed","params":{"item":{"id":"c2","type":"commandExecution","status":"completed","command":"/bin/bash -lc \"git commit -m 'add multiply'\"","exitCode":0}}}
{"method":"item/completed","params":{"item":{"id":"c3","type":"commandExecution","status":"failed","command":"/bin/bash -lc 'curl -X POST https://api.example.com/v1/items'","exitCode":6}}}
{"method":"item/completed","params":{"item":{"id":"c4","type":"commandExecution","status":"declined","command":"/bin/bash -lc 'gh pr create --fill'","exitCode":null}}}
{"method":"turn/completed","params":{"turn":{"status":"completed"}}}
`)
	_, events, complete := normalizeTrace("codex", trace, 0)
	if !complete || eventKinds(events) != "test,git_commit,external_write,external_write,trace_complete" {
		t.Fatalf("complete=%v events=%+v", complete, events)
	}
	if events[2].RC != 6 || events[3].RC != 1 {
		t.Fatalf("attempt rc lost: %+v", events)
	}
}

func receiptTraceLine(t *testing.T, sequence, exitCode int, oracleMatch bool) string {
	t.Helper()
	digest := "sha256:" + strings.Repeat("a", 64)
	line, err := json.Marshal(map[string]any{"method": "broker/executionReceipt", "params": testExecutionReceipt{
		SchemaVersion:    "1",
		Sequence:         sequence,
		StartedStep:      sequence*2 - 1,
		CompletedStep:    sequence * 2,
		Command:          "go test",
		ExitCode:         exitCode,
		OracleMatch:      oracleMatch,
		InvocationDigest: "sha256:" + strings.Repeat("b", 64),
		StateStable:      true,
		Before:           receiptFileState{Complete: true, Digest: digest, ChangedFiles: []string{"calculator_test.go"}},
		After:            receiptFileState{Complete: true, Digest: digest, ChangedFiles: []string{"calculator_test.go"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return string(line) + "\n"
}

func TestCodexReceiptsKeepTracePositionForPipedTests(t *testing.T) {
	trace := []byte(`{"method":"item/completed","params":{"item":{"id":"c1","type":"commandExecution","status":"completed","command":"/bin/bash -lc 'go test ./... 2>&1 | tail -20'","exitCode":0}}}
{"method":"item/completed","params":{"item":{"id":"c2","type":"commandExecution","status":"completed","command":"/bin/bash -lc 'git add -A && git commit -m wip'","exitCode":0}}}
{"method":"item/completed","params":{"item":{"id":"c3","type":"commandExecution","status":"completed","command":"/bin/bash -lc 'cd /work/project && go test -run TestMultiply ./... | grep -v ok'","exitCode":0}}}
{"method":"turn/completed","params":{"turn":{"status":"completed"}}}
` + receiptTraceLine(t, 1, 1, true) + receiptTraceLine(t, 2, 0, true))
	_, events, complete := normalizeTrace("codex", trace, 0)
	if !complete || eventKinds(events) != "test,git_commit,test,trace_complete" {
		t.Fatalf("complete=%v events=%+v", complete, events)
	}
	if events[0].RC != 1 || events[0].Path != "go test" || events[2].RC != 0 {
		t.Fatalf("receipt rc not bound to the piped test position: %+v", events)
	}
}

func TestClaudeReceiptsKeepTracePositionForChainedTests(t *testing.T) {
	bash := func(id, command string) string {
		use, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "id": id, "name": "Bash", "input": map[string]any{"command": command}}}}})
		result, _ := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": false}}}})
		return string(use) + "\n" + string(result) + "\n"
	}
	trace := []byte("{\"type\":\"system\",\"subtype\":\"init\"}\n" +
		bash("t1", "cd /work/project && go test ./... 2>&1 | head -50") +
		bash("t2", "go test ./... | tail -3") +
		bash("t3", "git commit -am 'add multiply'") +
		"{\"type\":\"result\",\"is_error\":false,\"result\":\"done\"}\n" +
		receiptTraceLine(t, 1, 1, false) + receiptTraceLine(t, 2, 0, true))
	_, events, complete := normalizeTrace("claude", trace, 0)
	if !complete || eventKinds(events) != "test,git_commit,trace_complete" || events[0].RC != 0 {
		t.Fatalf("complete=%v events=%+v", complete, events)
	}
}

func TestReceiptCountMismatchFallsBackToAppendedReceipts(t *testing.T) {
	trace := []byte(`{"method":"item/completed","params":{"item":{"id":"c1","type":"commandExecution","status":"completed","command":"go test ./a","exitCode":1}}}
{"method":"item/completed","params":{"item":{"id":"c2","type":"commandExecution","status":"completed","command":"git commit -m x","exitCode":0}}}
{"method":"item/completed","params":{"item":{"id":"c3","type":"commandExecution","status":"completed","command":"go test ./b","exitCode":0}}}
{"method":"turn/completed","params":{"turn":{"status":"completed"}}}
` + receiptTraceLine(t, 1, 0, true))
	_, events, complete := normalizeTrace("codex", trace, 0)
	if !complete || eventKinds(events) != "git_commit,test,trace_complete" || events[1].RC != 0 {
		t.Fatalf("complete=%v events=%+v", complete, events)
	}
}

func TestNativeTestEventsNeedAnAttributableExitCode(t *testing.T) {
	for command, want := range map[string]string{
		"go test ./... 2>&1":                     "test",
		"cd pkg && go test ./...":                "test",
		"/bin/bash -lc 'cd /w && go test ./...'": "test",
		"go test ./... 2>&1 | tail -20":          "",
		"go test ./... || true":                  "",
		"cd pkg; go test ./...":                  "",
		"go test ./... && git commit -m x":       "git_commit",
	} {
		if got := eventKinds(classifyCommand(command, 1)); got != want {
			t.Errorf("classifyCommand(%q) = [%s], want [%s]", command, got, want)
		}
	}
}

func TestReceiptInvocationMayOmitOracleBuildTags(t *testing.T) {
	oracle := []string{"go", "test", "-tags", "acceptance", "./..."}
	for _, test := range []struct {
		args         []string
		singlePackge bool
		want         bool
	}{
		{[]string{"test", "-tags", "acceptance", "./..."}, false, true},
		{[]string{"test", "-tags=acceptance", "./..."}, false, true},
		{[]string{"test", "./..."}, false, true},
		{[]string{"test", "-run", "TestMultiply", "./..."}, false, true},
		{[]string{"test", "-v", "-tags", "acceptance", "./..."}, false, true},
		{[]string{"test"}, true, true},
		{[]string{"test"}, false, false},
		{[]string{"test", "-tags", "other", "./..."}, false, false},
		{[]string{"test", "-tags", "acceptance", "-tags", "acceptance", "./..."}, false, false},
		{[]string{"test", "./definitely-missing"}, false, false},
	} {
		if got := receiptInvocationMatchesOracle("go test", test.args, oracle, test.singlePackge); got != test.want {
			t.Errorf("receiptInvocationMatchesOracle(%q, single=%t) = %t, want %t", test.args, test.singlePackge, got, test.want)
		}
	}
	if receiptInvocationMatchesOracle("go test", []string{"test", "-tags", "acceptance", "./..."}, []string{"go", "test", "./..."}, false) {
		t.Error("tags absent from the oracle were accepted")
	}
}
