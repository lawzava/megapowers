package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if _, wrapper := receiptWrapperExecutable(os.Args[0]); wrapper {
		main()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--actor-bridge" {
		main()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "app-server" {
		os.Exit(fakeSubswapperAppServer())
	}
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		_, _ = fmt.Fprintln(os.Stdout, "fake-claude 1.0")
		os.Exit(0)
	}
	if slicesContain(os.Args, "claude-followup-gate-fixture") {
		os.Exit(fakeClaudeFollowupGate())
	}
	os.Exit(m.Run())
}

func slicesContain(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func requireBrokerBubblewrap(t *testing.T) {
	t.Helper()
	binary, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skip("bubblewrap executable is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	args := []string{
		"--die-with-parent", "--new-session", "--unshare-ipc", "--unshare-pid", "--unshare-uts", "--unshare-cgroup-try", "--unshare-net", "--cap-drop", "ALL",
		"--ro-bind", "/", "/", "--", "/usr/bin/true",
	}
	result, runErr := runProcess(ctx, binary, args, "/", minimalEnvironment("/tmp"), nil)
	if result.timedOut {
		t.Skip("bubblewrap capability probe timed out while creating the broker's required isolated namespaces, including --unshare-net")
	}
	if runErr == nil && result.rc == 0 {
		return
	}
	detail := strings.TrimSpace(string(result.stderr))
	if newline := strings.IndexByte(detail, '\n'); newline >= 0 {
		detail = detail[:newline]
	}
	if len(detail) > 240 {
		detail = detail[:240]
	}
	if detail == "" {
		detail = "no diagnostic output"
	}
	t.Skipf("bubblewrap cannot create the broker's required isolated namespaces, including --unshare-net (rc=%d): %s", result.rc, detail)
}

func fakeClaudeFollowupGate() int {
	reader := bufio.NewReader(os.Stdin)
	first, err := reader.ReadBytes('\n')
	if err != nil || !claudeFrameHasText(first, "first") {
		return 50
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(map[string]any{"type": "system", "subtype": "init", "plugins": []any{}, "plugin_errors": []any{}}); err != nil {
		return 51
	}
	if claudeFrameReady(reader) {
		return 52
	}
	for _, event := range []map[string]any{
		{"type": "system", "subtype": "task_started", "task_id": "agent-a", "task_type": "local_agent"},
		{"type": "system", "subtype": "task_notification", "task_id": "agent-a", "status": "completed"},
		{"type": "system", "subtype": "init", "plugins": []any{}, "plugin_errors": []any{}},
		{"type": "result", "is_error": false, "result": "first answer"},
	} {
		if err := encoder.Encode(event); err != nil {
			return 53
		}
	}
	if claudeFrameReady(reader) {
		return 54
	}
	if err := encoder.Encode(map[string]any{"type": "result", "is_error": false, "result": "agent output", "origin": map[string]any{"kind": "task-notification"}}); err != nil {
		return 55
	}
	second, err := reader.ReadBytes('\n')
	if err != nil || !claudeFrameHasText(second, "second") {
		return 56
	}
	if err := encoder.Encode(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{}}}); err != nil {
		return 57
	}
	if err := encoder.Encode(map[string]any{"type": "result", "is_error": false, "result": "second answer"}); err != nil {
		return 58
	}
	return 0
}

func claudeFrameHasText(line []byte, want string) bool {
	var frame struct {
		Type    string `json:"type"`
		Message struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	return json.Unmarshal(line, &frame) == nil && frame.Type == "user" && frame.Message.Role == "user" && len(frame.Message.Content) == 1 && frame.Message.Content[0].Type == "text" && frame.Message.Content[0].Text == want
}

func claudeFrameReady(reader *bufio.Reader) bool {
	if reader.Buffered() > 0 {
		return true
	}
	fd := int(os.Stdin.Fd())
	if syscall.SetNonblock(fd, true) != nil {
		return true
	}
	defer func() { _ = syscall.SetNonblock(fd, false) }()
	_, err := reader.Peek(1)
	return err == nil || !errors.Is(err, syscall.EAGAIN)
}

// The fake uses the real sandbox and HTTP bridge, but no provider or login.
func fakeSubswapperAppServer() int {
	if !strings.Contains(strings.Join(os.Args, "\n"), `model_providers.megapowers_subswapper.base_url="http://`+actorBridgeAddress+`/v1"`) {
		return 40
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	token := ""
	exitImmediately := false
	childCompletesFirst := false
	followups := false
	turnCount := 0
	for scanner.Scan() {
		var message map[string]any
		if json.Unmarshal(scanner.Bytes(), &message) != nil {
			return 41
		}
		method, _ := message["method"].(string)
		params, _ := message["params"].(map[string]any)
		if method == "thread/start" || method == "turn/start" {
			if _, disablesDefault := params["environments"]; disablesDefault {
				return 45
			}
		}
		if method == "turn/start" && followups {
			inputs, _ := params["input"].([]any)
			entry := map[string]any{}
			if len(inputs) == 1 {
				entry, _ = inputs[0].(map[string]any)
			}
			expected := []string{"test", "second turn", "third turn"}
			if params["threadId"] != "thread-test" || turnCount >= len(expected) || entry["text"] != expected[turnCount] {
				return 46
			}
		}
		result := map[string]any{}
		switch method {
		case "initialized":
			continue
		case "account/login/start":
			token, _ = params["accessToken"].(string)
		case "thread/start":
			exitImmediately = params["model"] == "exit-immediately"
			childCompletesFirst = params["model"] == "child-completes-first"
			followups = params["model"] == "followups"
			result["thread"] = map[string]any{"id": "thread-test"}
		}
		if err := encoder.Encode(map[string]any{"id": message["id"], "result": result}); err != nil {
			return 42
		}
		if method != "turn/start" {
			continue
		}
		request, _ := http.NewRequest(http.MethodPost, "http://"+actorBridgeAddress+"/v1/responses", strings.NewReader(`{}`))
		request.Header.Set("Authorization", "Bearer "+token)
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Do(request)
		if err != nil {
			return 43
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return 44
		}
		if childCompletesFirst {
			_ = encoder.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "child-thread", "turn": map[string]any{"id": "child-turn", "status": "failed"}}})
		}
		turnCount++
		if followups {
			_ = encoder.Encode(map[string]any{"method": "item/completed", "params": map[string]any{"threadId": "thread-test", "item": map[string]any{"type": "agentMessage", "text": fmt.Sprintf("turn-%d", turnCount)}}})
		}
		if exitImmediately {
			// A burst leaves unread notifications when the process exits.
			for range 1000 {
				_ = encoder.Encode(map[string]any{"method": "item/agentMessage/delta", "params": map[string]any{"delta": "diagnostic"}})
			}
		}
		_ = encoder.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-test", "turn": map[string]any{"id": "turn-test", "status": "completed"}}})
		if exitImmediately {
			return 0
		}
	}
	return 0
}

func TestSubswapperCodexAppServer(t *testing.T) {
	requireBrokerBubblewrap(t)
	testSubswapperCodexAppServer(t, "")
}

func TestCodexExitDrainsStdout(t *testing.T) {
	requireBrokerBubblewrap(t)
	for range 20 {
		testSubswapperCodexAppServer(t, "exit-immediately")
	}
}

func TestCodexWaitsForRootTurn(t *testing.T) {
	requireBrokerBubblewrap(t)
	testSubswapperCodexAppServer(t, "child-completes-first")
}

func TestCodexFollowupsReuseRootThread(t *testing.T) {
	requireBrokerBubblewrap(t)
	result := testSubswapperCodexAppServer(t, "followups")
	response, events, complete := normalizeTrace("codex", result.stdout, result.rc)
	if !complete || response != "turn-3" || len(events) != 1 || events[0].Kind != "trace_complete" {
		t.Fatalf("followups did not complete in the root conversation: complete=%t response=%q events=%+v", complete, response, events)
	}
}

func TestClaudeFollowupsUseOneStreamingConversation(t *testing.T) {
	input, err := claudeStreamingInput([]string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(input)), "\n")
	if len(lines) != 2 {
		t.Fatalf("stream has %d turns, want 2", len(lines))
	}
	for index, line := range lines {
		var frame struct {
			Type    string `json:"type"`
			Message struct {
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &frame) != nil || frame.Type != "user" || frame.Message.Role != "user" || len(frame.Message.Content) != 1 || frame.Message.Content[0].Text != []string{"first", "second"}[index] {
			t.Fatalf("invalid streaming turn %d: %s", index+1, line)
		}
	}
	trace := []byte("{\"type\":\"system\",\"subtype\":\"init\",\"plugins\":[]}\n{\"type\":\"result\",\"is_error\":false,\"result\":\"first answer\"}\n{\"type\":\"assistant\",\"message\":{\"content\":[]}}\n{\"type\":\"result\",\"is_error\":false,\"result\":\"second answer\"}\n")
	response, events, complete := normalizeTraceTurns("claude", trace, 0, 2)
	if !complete || response != "second answer" || len(events) != 1 || events[0].Kind != "trace_complete" {
		t.Fatalf("Claude follow-up trace did not finish on the last turn: complete=%t response=%q events=%+v", complete, response, events)
	}
}

func TestClaudeFollowupInputWaitsForRootAndForwardedWork(t *testing.T) {
	requireBrokerBubblewrap(t)
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	req.Arm = "control"
	req.Model = "claude-followup-gate-fixture"
	req.PluginRepo = ""
	req.TaskReadRoots = []string{req.Project}
	req.Task = "first"
	req.FollowupTasks = []string{"second"}
	brokerDirectory, err := newPrivateSocketDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(brokerDirectory)
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("fake Claude unexpectedly contacted the provider")
	}))
	defer upstream.Close()
	proxy, err := startCredentialProxy("claude", authSubswapper, "host-only-capability", upstream.URL, filepath.Join(brokerDirectory, "proxy.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.close()
	receipts, err := startReceiptCollector(req.Project, filepath.Join(brokerDirectory, receiptSocketName), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer receipts.close()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	run, err := runHarness(ctx, req, binary, authentication{mode: authSubswapper, credential: "host-only-capability", upstream: upstream.URL}, proxy, brokerDirectory, receipts, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	if run.rc != 0 || run.response != "second answer" || len(run.events) != 3 || run.events[0].Kind != "agent_spawn" || run.events[1].Kind != "agent_complete" || run.events[2].Kind != "trace_complete" {
		t.Fatalf("Claude follow-up was delivered before the prior turn drained: rc=%d response=%q events=%+v trace=%s", run.rc, run.response, run.events, run.trace)
	}
}

func TestClaudeFollowupTimeoutKillsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	command := "read -r first; sleep 30 & child=$!; printf '%s' \"$child\" > " + strconv.Quote(pidFile) + "; wait"
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result, err := runClaudeStreamingProcess(ctx, "/usr/bin/bash", []string{"-c", command}, "/tmp", minimalEnvironment("/tmp"), [][]byte{[]byte("first\n"), []byte("second\n")})
	if !result.timedOut || result.rc != 124 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Claude follow-up timeout result = %+v, %v", result, err)
	}
	content, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(content))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Claude follow-up timeout left child %d running: %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestClaudeFollowupGateDoesNotReuseInnerResult(t *testing.T) {
	var capture limitedBuffer
	capture.limit = traceLimit + 1
	gate := newClaudeFollowupGate(&capture)
	if !gate.beginTurn() {
		t.Fatal("initial Claude root turn was not armed")
	}
	_, _ = gate.Write([]byte("{\"type\":\"system\",\"subtype\":\"init\"}\n{\"type\":\"result\",\"is_error\":false,\"result\":\"root\"}\n"))
	select {
	case <-gate.ready:
		if !gate.claimReady() {
			t.Fatal("successful root result did not satisfy the first gate")
		}
	case <-time.After(time.Second):
		t.Fatal("successful root result did not signal the first gate")
	}
	_, _ = gate.Write([]byte("{\"type\":\"result\",\"is_error\":false,\"result\":\"late forwarded inner result\"}\n"))
	select {
	case <-gate.ready:
		t.Fatal("origin-less inner result armed another user turn")
	default:
	}
	select {
	case <-gate.failure:
	default:
		t.Fatal("origin-less result outside an armed root turn did not fail closed")
	}
}

func TestClaudeFollowupGateRejectsUnmatchedResultOrigin(t *testing.T) {
	for name, result := range map[string]string{
		"wrong kind":       `{"type":"result","is_error":false,"result":"child","origin":{"kind":"other"}}`,
		"non-object":       `{"type":"result","is_error":false,"result":"child","origin":"task-notification"}`,
		"failed forwarded": `{"type":"result","is_error":true,"result":"child","origin":{"kind":"task-notification"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var capture limitedBuffer
			capture.limit = traceLimit + 1
			gate := newClaudeFollowupGate(&capture)
			if !gate.beginTurn() {
				t.Fatal("initial Claude root turn was not armed")
			}
			_, _ = gate.Write([]byte("{\"type\":\"system\",\"subtype\":\"init\"}\n" + result + "\n"))
			select {
			case <-gate.failure:
			default:
				t.Fatal("unmatched or malformed result origin did not fail closed")
			}
			select {
			case <-gate.ready:
				t.Fatal("unmatched or malformed result origin opened the follow-up gate")
			default:
			}
		})
	}
}

func TestClaudeFollowupGateAllowsUnusedForwardPermission(t *testing.T) {
	var capture limitedBuffer
	capture.limit = traceLimit + 1
	gate := newClaudeFollowupGate(&capture)
	if !gate.beginTurn() {
		t.Fatal("initial Claude root turn was not armed")
	}
	_, _ = gate.Write([]byte(strings.Join([]string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"system","subtype":"task_started","task_id":"agent-a","task_type":"local_agent"}`,
		`{"type":"system","subtype":"task_notification","task_id":"agent-a","status":"completed"}`,
		`{"type":"result","is_error":false,"result":"root"}`,
	}, "\n") + "\n"))
	select {
	case <-gate.ready:
		if !gate.claimReady() {
			t.Fatal("completed foreground agent did not satisfy the root gate")
		}
		gate.mu.Lock()
		remaining := gate.forwardPermissions
		gate.mu.Unlock()
		if remaining != 0 {
			t.Fatalf("claimed root retained %d unused forwarding permissions", remaining)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("unused forwarding permission blocked the next user turn")
	}
}

func TestFollowupRequestBounds(t *testing.T) {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	req.FollowupTasks = make([]string, 9)
	for index := range req.FollowupTasks {
		req.FollowupTasks[index] = "turn"
	}
	if err := validateRequest(&req); err == nil {
		t.Fatal("nine follow-up turns were accepted")
	}
	req = validSelftestRequest(paths)
	req.FollowupTasks = []string{" "}
	if err := validateRequest(&req); err == nil {
		t.Fatal("empty follow-up turn was accepted")
	}
	req = validSelftestRequest(paths)
	req.FollowupTasks = []string{strings.Repeat("x", 256<<10)}
	if err := validateRequest(&req); err == nil {
		t.Fatal("oversized follow-up payload was accepted")
	}
}

func TestPathsOverlapIncludesFilesystemRoot(t *testing.T) {
	for _, test := range [][2]string{{"/", "/tmp/project"}, {"/tmp/project", "/"}} {
		if !pathsOverlap(test[0], test[1]) {
			t.Errorf("pathsOverlap(%q, %q) = false, want true", test[0], test[1])
		}
	}
}

func TestClaudeSettingsAllowBrokerUnixSocket(t *testing.T) {
	req := brokerRequest{ActorHome: t.TempDir()}
	path, err := writeClaudeSettings(req)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Sandbox struct {
			Network struct {
				AllowAllUnixSockets bool `json:"allowAllUnixSockets"`
			} `json:"network"`
		} `json:"sandbox"`
	}
	if err := json.Unmarshal(content, &settings); err != nil {
		t.Fatal(err)
	}
	if !settings.Sandbox.Network.AllowAllUnixSockets {
		t.Fatal("Claude's Linux sandbox blocks the broker receipt socket")
	}
}

func TestClaudeSettingsAllowPluginReferenceReads(t *testing.T) {
	req := brokerRequest{ActorHome: t.TempDir()}
	path, err := writeClaudeSettings(req)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(content, &settings); err != nil {
		t.Fatal(err)
	}
	allowed := make(map[string]bool, len(settings.Permissions.Allow))
	for _, tool := range settings.Permissions.Allow {
		allowed[tool] = true
	}
	for _, tool := range []string{"Read", "Glob", "Grep"} {
		if !allowed[tool] {
			t.Errorf("Claude settings do not allow %s for plugin references", tool)
		}
	}
}

func TestCodexAPIKeyFollowupsFailClosed(t *testing.T) {
	req := brokerRequest{Harness: "codex", FollowupTasks: []string{"second turn"}}
	if err := validateFollowupTransport(req, authentication{mode: authAPIKey}); err == nil {
		t.Fatal("single-shot Codex API-key transport accepted a follow-up")
	}
	for _, mode := range []string{authSubscription, authSubswapper} {
		if err := validateFollowupTransport(req, authentication{mode: mode}); err != nil {
			t.Fatalf("Codex app-server mode %q rejected follow-ups: %v", mode, err)
		}
	}
	req.Harness = "claude"
	if err := validateFollowupTransport(req, authentication{mode: authAPIKey}); err != nil {
		t.Fatalf("Claude streaming transport rejected follow-ups: %v", err)
	}
}

func TestCodexNativeAgentActivity(t *testing.T) {
	for _, test := range []struct {
		kind, event string
		rc          int
	}{{"started", "agent_spawn", 0}, {"completed", "agent_complete", 0}, {"interrupted", "agent_complete", 1}} {
		t.Run(test.kind, func(t *testing.T) {
			object := map[string]any{"method": "item/completed", "params": map[string]any{"threadId": "root", "item": map[string]any{"type": "subAgentActivity", "id": "call", "kind": test.kind, "agentThreadId": "child", "agentPath": "/root/read"}}}
			events := normalizeObject("codex", object)
			if len(events) != 1 || events[0].Kind != test.event || events[0].Path != "child" || events[0].RC != test.rc {
				t.Fatalf("native lifecycle lost: %+v", events)
			}
		})
	}
}

func TestCodexChildTurnCannotCompleteTrace(t *testing.T) {
	trace := []byte(`{"method":"thread/started","params":{"thread":{"id":"root","parentThreadId":null}}}
{"method":"item/completed","params":{"threadId":"child","item":{"type":"agentMessage","text":"child result"}}}
{"method":"turn/completed","params":{"threadId":"child","turn":{"status":"completed"}}}
`)
	response, _, complete := normalizeTrace("codex", trace, 0)
	if complete || response != "" {
		t.Fatalf("child was treated as root: complete=%v response=%q", complete, response)
	}
}

func testSubswapperCodexAppServer(t *testing.T, model string) processResult {
	t.Helper()
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	req.Harness = "codex"
	if model != "" {
		req.Model = model
	}
	if model == "followups" {
		req.FollowupTasks = []string{"second turn", "third turn"}
	}
	for _, name := range []string{".codex", "codex-marketplace"} {
		if err := os.Mkdir(filepath.Join(req.ActorHome, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	secret := strings.Repeat("host-only-capability", 3)
	seen := make(chan bool, 8)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.URL.Path == "/backend-api/codex/responses" && r.Header.Get("Authorization") == "Bearer "+secret
		_, _ = io.WriteString(w, `{}`)
	}))
	defer upstream.Close()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	brokerDirectory, err := newPrivateSocketDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(brokerDirectory)
	result, err := runCodexAppServer(ctx, req, binary, filepath.Join(req.ActorHome, ".codex"), authentication{mode: "subswapper", credential: secret, accountID: "subswapper-proxy", upstream: upstream.URL}, brokerDirectory, toolchain)
	if err != nil || result.rc != 0 {
		t.Fatalf("isolated app-server bridge failed: %v, rc=%d", err, result.rc)
	}
	select {
	case ok := <-seen:
		if !ok {
			t.Fatal("upstream identity or path incorrect")
		}
	default:
		t.Fatal("upstream did not receive a request")
	}
	if strings.Contains(string(result.stdout)+string(result.stderr), secret) {
		t.Fatal("host capability entered trace")
	}
	return result
}

func TestCodexCompanionMount(t *testing.T) {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	req.Harness = "codex"
	binary := filepath.Join(paths.root, "codex")
	companion := filepath.Join(paths.root, "codex-code-mode-host")
	if err := os.WriteFile(binary, []byte("fake"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(companion, []byte("fake"), 0o700); err != nil {
		t.Fatal(err)
	}
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		t.Fatal(err)
	}
	args, err := codexAppServerSandboxArgs(req, binary, paths.root, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, "\n"), "--ro-bind\n"+companion+"\n/opt/megapowers/codex-code-mode-host\n") {
		t.Fatal("Codex companion is missing from the read-only runtime")
	}
}

func TestCodexSandboxUsesActiveGoToolchain(t *testing.T) {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	req.Harness = "codex"
	binary := filepath.Join(paths.root, "codex")
	if err := os.WriteFile(binary, []byte("fake"), 0o700); err != nil {
		t.Fatal(err)
	}
	goRoot := filepath.Join(t.TempDir(), "alternate-go")
	goBinary := filepath.Join(goRoot, "bin", "go")
	if err := os.MkdirAll(filepath.Dir(goBinary), 0o700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nif [ \"$1\" = env ] && [ \"$2\" = GOROOT ]; then\n  printf '%s\\n' " + strconv.Quote(goRoot) + "\n  exit 0\nfi\nexit 64\n"
	if err := os.WriteFile(goBinary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(goBinary))
	t.Setenv("HOME", filepath.Dir(goRoot))

	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		t.Fatal(err)
	}
	args, err := codexAppServerSandboxArgs(req, binary, paths.root, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, "\n")
	for _, mount := range []string{
		"--ro-bind\n" + goRoot + "\n/opt/megapowers-runtime/go\n",
		"--ro-bind\n" + goBinary + "\n/opt/megapowers-receipt/real/go\n",
	} {
		if !strings.Contains(joined, mount) {
			t.Fatalf("active Go toolchain mount is missing: %q", mount)
		}
	}
	if strings.Contains(joined, "/usr/local/go") {
		t.Fatal("sandbox retained the fixed /usr/local/go toolchain path")
	}
}

func TestCodexSandboxRejectsActorControlledGoToolchain(t *testing.T) {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	req.Harness = "codex"
	binary := filepath.Join(paths.root, "codex")
	if err := os.WriteFile(binary, []byte("fake"), 0o700); err != nil {
		t.Fatal(err)
	}
	goRoot := filepath.Join(req.Project, "actor-go")
	goBinary := filepath.Join(goRoot, "bin", "go")
	if err := os.MkdirAll(filepath.Dir(goBinary), 0o700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nif [ \"$1\" = env ] && [ \"$2\" = GOROOT ]; then\n  printf '%s\\n' " + strconv.Quote(goRoot) + "\n  exit 0\nfi\nexit 64\n"
	if err := os.WriteFile(goBinary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(goBinary))

	if _, err := resolveHostGoToolchain(req); err == nil {
		t.Fatal("actor-controlled Go toolchain was accepted as a host runtime")
	}
}

func TestCodexSandboxRejectsGoRootContainingHostHome(t *testing.T) {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	hostHome := t.TempDir()
	goBinary := filepath.Join(hostHome, "bin", "go")
	if err := os.MkdirAll(filepath.Dir(goBinary), 0o700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nif [ \"$1\" = env ] && [ \"$2\" = GOROOT ]; then\n  printf '%s\\n' " + strconv.Quote(hostHome) + "\n  exit 0\nfi\nexit 64\n"
	if err := os.WriteFile(goBinary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", hostHome)
	t.Setenv("PATH", filepath.Dir(goBinary))

	if _, err := resolveHostGoToolchain(req); err == nil {
		t.Fatal("Go root containing the host home was accepted as a runtime mount")
	}
}

func TestCodexAbsoluteShellTestEvidence(t *testing.T) {
	for _, shell := range []string{"/usr/bin/bash", "/usr/bin/sh", "/bin/bash", "/bin/sh"} {
		events := classifyCommand(shell+" -lc 'go test ./...'", 1)
		if len(events) != 1 || events[0].Kind != "test" || events[0].RC != 1 {
			t.Errorf("failed test execution was not recognized through %s", shell)
		}
	}
}

func TestStructuredTestReceiptOverridesCompoundAggregateExit(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	invocation := "sha256:" + strings.Repeat("b", 64)
	trace := []byte(fmt.Sprintf(`{"method":"item/completed","params":{"item":{"id":"compound","type":"commandExecution","status":"completed","command":"go test ./...; echo done","exitCode":0}}}
{"method":"broker/executionReceipt","params":{"schema_version":"1","sequence":1,"started_step":1,"completed_step":2,"command":"go test","exit_code":1,"oracle_match":true,"invocation_digest":%q,"state_stable":true,"before":{"complete":true,"digest":%q,"changed_files":["calculator_test.go"]},"after":{"complete":true,"digest":%q,"changed_files":["calculator_test.go"]}}}
{"method":"turn/completed","params":{"turn":{"status":"completed"}}}
`, invocation, digest, digest))
	_, events, complete := normalizeTrace("codex", trace, 0)
	if !complete {
		t.Fatal("receipt trace was not complete")
	}
	if len(events) != 2 || events[0].Kind != "test" || events[0].Path != "go test" || events[0].RC != 1 || events[1].Kind != "trace_complete" {
		t.Fatalf("compound aggregate replaced the per-command result: %+v", events)
	}
}

func TestIrrelevantReceiptDoesNotFallBackToNativeAggregate(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	receipt := testExecutionReceipt{
		SchemaVersion:    "1",
		Sequence:         1,
		StartedStep:      1,
		CompletedStep:    2,
		Command:          "go test",
		ExitCode:         1,
		OracleMatch:      false,
		InvocationDigest: "sha256:" + strings.Repeat("b", 64),
		StateStable:      true,
		Before:           receiptFileState{Complete: true, Digest: digest, ChangedFiles: []string{"calculator_test.go"}},
		After:            receiptFileState{Complete: true, Digest: digest, ChangedFiles: []string{"calculator_test.go"}},
	}
	receiptLine, err := json.Marshal(map[string]any{"method": "broker/executionReceipt", "params": receipt})
	if err != nil {
		t.Fatal(err)
	}
	trace := []byte(`{"method":"item/completed","params":{"item":{"id":"irrelevant","type":"commandExecution","status":"failed","command":"go test ./definitely-missing","exitCode":1}}}` + "\n" + string(receiptLine) + "\n" + `{"method":"turn/completed","params":{"turn":{"status":"completed"}}}` + "\n")
	_, events, complete := normalizeTrace("codex", trace, 0)
	if !complete || len(events) != 1 || events[0].Kind != "trace_complete" {
		t.Fatalf("irrelevant receipt fell back to native aggregate evidence: %+v", events)
	}
}

func TestReceiptCommandClassification(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"go", []string{"test", "./..."}, "go test"},
		{"go", []string{"-C", "pkg", "test", "./..."}, "go test"},
		{"bash", []string{"scripts/validate.sh"}, "scripts/validate.sh"},
		{"dash", []string{"./scripts/validate.sh"}, "scripts/validate.sh"},
		{"pytest", []string{"-q"}, "pytest"},
		{"python3", []string{"-m", "pytest", "-q"}, "pytest"},
		{"bash", []string{"-lc", "go test ./...; echo done"}, ""},
		{"go", []string{"run", "./cmd/check"}, ""},
	}
	for _, test := range tests {
		if got := receiptCommand(test.name, test.args); got != test.want {
			t.Errorf("receiptCommand(%q, %q) = %q, want %q", test.name, test.args, got, test.want)
		}
	}
}

func TestReceiptInvocationMatchesDeclaredOracleTargets(t *testing.T) {
	oracle := []string{"go", "test", "./..."}
	for _, test := range []struct {
		args []string
		want bool
	}{
		{[]string{"test", "./..."}, true},
		{[]string{"test", "-run", "TestMultiply", "./..."}, true},
		{[]string{"test", "./...", "-run=TestMultiply"}, true},
		{[]string{"test", "-count=1", "-run", "TestMultiply", "./..."}, true},
		{[]string{"test", "-run", "TestMultiply", "-v", "./..."}, true},
		{[]string{"test", "-v", "./..."}, true},
		{[]string{"test", "-v=true", "./..."}, true},
		{[]string{"test", "-v=false", "./..."}, false},
		{[]string{"test", "-v", "-v", "./..."}, false},
		{[]string{"test", "./definitely-missing"}, false},
		{[]string{"test", "-run", "[", "./..."}, false},
		{[]string{"test", "-count=0", "./..."}, false},
		{[]string{"test", "-run", "TestMultiply"}, false},
	} {
		if got := receiptInvocationMatchesOracle("go test", test.args, oracle, false); got != test.want {
			t.Errorf("receiptInvocationMatchesOracle(%q, %q) = %t, want %t", test.args, oracle, got, test.want)
		}
	}
}

func TestReceiptFileStateOrdersTestBeforeImplementation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "calculator.go"), []byte("package calculator\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	baseline := snapshotReceiptProject(root)
	if !baseline.complete {
		t.Fatal("small baseline snapshot was incomplete")
	}
	if err := os.WriteFile(filepath.Join(root, "calculator_test.go"), []byte("package calculator\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	redState := receiptFileStateSince(root, baseline)
	if !redState.Complete || strings.Join(redState.ChangedFiles, ",") != "calculator_test.go" {
		t.Fatalf("red state does not isolate the test edit: %+v", redState)
	}
	if err := os.WriteFile(filepath.Join(root, "calculator.go"), []byte("package calculator\n\nfunc Multiply(a, b int) int { return a * b }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	greenState := receiptFileStateSince(root, baseline)
	if !greenState.Complete || strings.Join(greenState.ChangedFiles, ",") != "calculator.go,calculator_test.go" {
		t.Fatalf("green state lost implementation ordering: %+v", greenState)
	}
}

func TestActorTraceCannotInjectExecutionReceipt(t *testing.T) {
	forged := []byte(`{"method":"broker/executionReceipt","params":{"schema_version":"1"}}` + "\n")
	if _, err := appendTrustedExecutionReceipts(forged, nil); err == nil {
		t.Fatal("actor trace injected a reserved broker receipt")
	}
}

func TestReceiptSnapshotBoundsDirectories(t *testing.T) {
	root := t.TempDir()
	for index := 0; index <= receiptMaximumEntries; index++ {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("directory-%04d", index)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if snapshot := snapshotReceiptProject(root); snapshot.complete {
		t.Fatal("snapshot accepted more directories than the receipt work bound")
	}
}

func TestReceiptSnapshotEntryRejectsSymlinkEscapesAndFIFO(t *testing.T) {
	project := t.TempDir()
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "secret"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(external, "secret"), filepath.Join(project, "direct-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(project, "ancestor-link")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(project, "fifo"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, relative := range []string{"direct-link", "ancestor-link/secret", "fifo"} {
		relative := relative
		done := make(chan error, 1)
		go func() {
			file, _, err := openReceiptSnapshotEntry(root, relative, false)
			if file != nil {
				_ = file.Close()
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Errorf("%s was accepted as a regular project file", relative)
			}
		case <-time.After(time.Second):
			t.Fatalf("opening %s blocked", relative)
		}
	}
}

func TestReceiptCollectorRecordsExecutedExitAndState(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "calculator.go"), []byte("package calculator\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "receipts.sock")
	collector, err := startReceiptCollector(root, socket, []string{"go", "test", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	defer collector.close()
	if err := os.WriteFile(filepath.Join(root, "calculator_test.go"), []byte("package calculator\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rc := runReceiptWrappedExecutable("/usr/bin/false", nil, "go test", socket)
	if rc != 1 {
		t.Fatalf("wrapped executable returned %d, want 1", rc)
	}
	receipts := collector.receipts()
	if len(receipts) != 1 || receipts[0].ExitCode != 1 || receipts[0].Command != "go test" || strings.Join(receipts[0].Before.ChangedFiles, ",") != "calculator_test.go" {
		t.Fatalf("collector did not observe the executed failure: %+v", receipts)
	}
}

func TestReceiptCollectorBindsInvocationToOracle(t *testing.T) {
	root := t.TempDir()
	socket := filepath.Join(t.TempDir(), "receipts.sock")
	collector, err := startReceiptCollector(root, socket, []string{"go", "test", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	defer collector.close()
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(previousDirectory) }()
	for _, arguments := range [][]string{{"test", "./definitely-missing"}, {"test", "-run", "TestMultiply", "./..."}} {
		if rc := runReceiptWrappedExecutable("/usr/bin/false", arguments, "go test", socket); rc != 1 {
			t.Fatalf("wrapped executable returned %d", rc)
		}
	}
	receipts := collector.receipts()
	if len(receipts) != 2 || receipts[0].OracleMatch || !receipts[1].OracleMatch {
		t.Fatalf("collector did not bind package targets to the oracle: %+v", receipts)
	}
	for _, receipt := range receipts {
		if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(receipt.InvocationDigest) {
			t.Fatalf("receipt omitted its bounded invocation digest: %+v", receipt)
		}
	}
}

func TestReceiptCollectorMatchesSinglePackageGoShorthand(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":        "module example.com/single-package\n\ngo 1.25.0\n",
		"calculator.go": "package calculator\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	socketDirectory, err := newPrivateSocketDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(socketDirectory)
	collector, err := startReceiptCollector(root, filepath.Join(socketDirectory, "receipts.sock"), []string{"go", "test", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	defer collector.close()
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(previousDirectory) }()
	for _, arguments := range [][]string{{"test"}, {"test", "."}, {"test", "./definitely-missing"}} {
		if rc := runReceiptWrappedExecutable("/usr/bin/false", arguments, "go test", collector.listener.Addr().String()); rc != 1 {
			t.Fatalf("wrapped executable returned %d", rc)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "nested.go"), []byte("package nested\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rc := runReceiptWrappedExecutable("/usr/bin/false", []string{"test"}, "go test", collector.listener.Addr().String()); rc != 1 {
		t.Fatalf("wrapped executable returned %d", rc)
	}
	receipts := collector.receipts()
	if len(receipts) != 4 {
		t.Fatalf("collector recorded %d receipts, want 4", len(receipts))
	}
	for index, want := range []bool{true, true, false, false} {
		if receipts[index].OracleMatch != want {
			t.Errorf("receipt %d oracle_match = %t, want %t: %+v", index, receipts[index].OracleMatch, want, receipts[index])
		}
	}
}

func TestReceiptCollectorMarksConcurrentMutationUnstable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "calculator_test.go"), []byte("package calculator\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	socketDirectory, err := newPrivateSocketDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(socketDirectory)
	collector, err := startReceiptCollector(root, filepath.Join(socketDirectory, receiptSocketName), []string{"go", "test", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	defer collector.close()
	connection, err := net.DialUnix("unix", nil, collector.listener.Addr().(*net.UnixAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	encoder := json.NewEncoder(connection)
	decoder := json.NewDecoder(connection)
	if err := encoder.Encode(receiptClientMessage{SchemaVersion: "1", Action: "start", Command: "go test", Arguments: []string{"test", "./..."}}); err != nil {
		t.Fatal(err)
	}
	var response receiptServerMessage
	if err := decoder.Decode(&response); err != nil || !response.Accepted {
		t.Fatalf("start receipt: %+v, %v", response, err)
	}
	if err := os.WriteFile(filepath.Join(root, "calculator.go"), []byte("package calculator\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rc := 1
	if err := encoder.Encode(receiptClientMessage{SchemaVersion: "1", Action: "finish", ID: response.ID, ExitCode: &rc}); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&response); err != nil || !response.Accepted {
		t.Fatalf("finish receipt: %+v, %v", response, err)
	}
	receipts := collector.receipts()
	if len(receipts) != 1 || receipts[0].StateStable {
		t.Fatalf("concurrent mutation was accepted as stable red evidence: %+v", receipts)
	}
}

func TestReceiptCollectorSealRejectsActiveCommand(t *testing.T) {
	root := t.TempDir()
	collector, err := startReceiptCollector(root, filepath.Join(t.TempDir(), "receipts.sock"), []string{"go", "test", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	defer collector.close()
	connection, err := net.DialUnix("unix", nil, collector.listener.Addr().(*net.UnixAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := json.NewEncoder(connection).Encode(receiptClientMessage{SchemaVersion: "1", Action: "start", Command: "go test"}); err != nil {
		t.Fatal(err)
	}
	var response receiptServerMessage
	if err := json.NewDecoder(connection).Decode(&response); err != nil || !response.Accepted {
		t.Fatalf("start receipt: %+v, %v", response, err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := collector.seal()
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("collector sealed successfully with an active command")
		}
	case <-time.After(time.Second):
		t.Fatal("collector seal blocked on an active command")
	}
}

func TestReceiptCollectorCloseSynchronizesAcceptRegistration(t *testing.T) {
	for attempt := 0; attempt < 20; attempt++ {
		socketDirectory, err := newPrivateSocketDirectory()
		if err != nil {
			t.Fatal(err)
		}
		collector, err := startReceiptCollector(t.TempDir(), filepath.Join(socketDirectory, "r.sock"), []string{"go", "test", "./..."})
		if err != nil {
			_ = os.RemoveAll(socketDirectory)
			t.Fatal(err)
		}
		dialsDone := make(chan struct{})
		go func() {
			defer close(dialsDone)
			for range 20 {
				connection, err := net.DialUnix("unix", nil, collector.listener.Addr().(*net.UnixAddr))
				if err == nil {
					_ = connection.Close()
				}
			}
		}()
		closed := make(chan struct{})
		go func() {
			collector.close()
			close(closed)
		}()
		select {
		case <-closed:
		case <-time.After(time.Second):
			t.Fatal("collector close blocked during accept registration")
		}
		<-dialsDone
		if err := os.RemoveAll(socketDirectory); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReceiptCollectorRejectsActorBridgePeer(t *testing.T) {
	root := t.TempDir()
	collector, err := startReceiptCollector(root, filepath.Join(t.TempDir(), "receipts.sock"), []string{"go", "test", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	defer collector.close()
	ctx, cancel := context.WithCancel(context.Background())
	bridge := exec.CommandContext(ctx, os.Args[0], "--actor-bridge", collector.listener.Addr().String(), "/bin/sleep", "5")
	if err := bridge.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		_ = bridge.Wait()
	}()
	var connection net.Conn
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		connection, err = net.DialTimeout("tcp", actorBridgeAddress, 25*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("connect actor bridge: %v", err)
	}
	defer connection.Close()
	if err := json.NewEncoder(connection).Encode(receiptClientMessage{SchemaVersion: "1", Action: "start", Command: "go test"}); err != nil {
		t.Fatal(err)
	}
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	var response receiptServerMessage
	err = json.NewDecoder(connection).Decode(&response)
	if err == nil && response.Accepted {
		t.Fatal("collector accepted a forged receipt through actor bridge mode")
	}
}

func TestReceiptGoWrapperClearsCommandAlteringEnvironment(t *testing.T) {
	t.Setenv("GOFLAGS", "-run [")
	t.Setenv("GOENV", filepath.Join(t.TempDir(), "host-go-env"))
	observed := filepath.Join(t.TempDir(), "observed")
	arguments := []string{"-c", `printf '%s:%s\n' "${GOFLAGS-unset}" "${GOENV-unset}" > "$1"`, "bash", observed}
	if rc := runReceiptWrappedExecutable("/usr/bin/bash", arguments, "go test", filepath.Join(t.TempDir(), "missing.sock")); rc != 0 {
		t.Fatalf("wrapped environment probe returned %d", rc)
	}
	content, err := os.ReadFile(observed)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(content)); got != "unset:off" {
		t.Fatalf("Go command-altering environment reached the test process: %q", got)
	}
}

func TestReceiptWrapperForwardsTermination(t *testing.T) {
	if os.Getenv("MEGAPOWERS_TEST_RECEIPT_SIGNAL_HELPER") == "1" {
		pidFile := os.Getenv("MEGAPOWERS_TEST_RECEIPT_SIGNAL_PID_FILE")
		arguments := []string{"-c", `printf '%d\n' $$ > "$1"; exec /usr/bin/sleep 30`, "bash", pidFile}
		os.Exit(runReceiptWrappedExecutable("/usr/bin/bash", arguments, "", ""))
	}

	pidFile := filepath.Join(t.TempDir(), "child.pid")
	command := exec.Command(os.Args[0], "-test.run=^TestReceiptWrapperForwardsTermination$")
	command.Env = append(os.Environ(), "MEGAPOWERS_TEST_RECEIPT_SIGNAL_HELPER=1", "MEGAPOWERS_TEST_RECEIPT_SIGNAL_PID_FILE="+pidFile)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	wrapperDone := make(chan error, 1)
	go func() { wrapperDone <- command.Wait() }()

	childPID := 0
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(pidFile)
		if err == nil {
			childPID, err = strconv.Atoi(strings.TrimSpace(string(content)))
			if err == nil && childPID > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID == 0 {
		_ = command.Process.Kill()
		<-wrapperDone
		t.Fatal("receipt wrapper child did not start")
	}
	defer func() { _ = syscall.Kill(childPID, syscall.SIGKILL) }()
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-wrapperDone:
		var exitErr *exec.ExitError
		if err == nil || !errors.As(err, &exitErr) || exitErr.ExitCode() != 128+int(syscall.SIGTERM) {
			t.Fatalf("receipt wrapper did not preserve the child signal exit: %v", err)
		}
	case <-time.After(2 * time.Second):
		_ = command.Process.Kill()
		<-wrapperDone
		t.Fatal("receipt wrapper did not terminate after SIGTERM")
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if err := syscall.Kill(childPID, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("receipt wrapper left its child running after SIGTERM")
}

func TestReceiptInstrumentationObservesNestedCompoundFailure(t *testing.T) {
	requireBrokerBubblewrap(t)
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":             "module receipt-selftest\n\ngo 1.25.0\n",
		"calculator.go":      "package calculator\n",
		"calculator_test.go": "package calculator\n\nimport \"testing\"\n\nfunc TestRed(t *testing.T) { t.Fatal(\"red\") }\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	socketDirectory, err := newPrivateSocketDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(socketDirectory)
	collector, err := startReceiptCollector(root, filepath.Join(socketDirectory, receiptSocketName), []string{"go", "test", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	defer collector.close()
	broker, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	req := brokerRequest{Project: root, ActorHome: t.TempDir()}
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		t.Fatal(err)
	}
	args, err := appendReceiptInstrumentation(sandboxBase(false), broker, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	args = appendMount(args, root, false)
	args = append(args, "--dir", actorBrokerPath, "--ro-bind", socketDirectory, actorBrokerPath, "--chdir", root, "--", "/usr/bin/bash", "-lc", "go test ./...; exit 0")
	result, err := runProcess(context.Background(), "bwrap", args, root, actorEnvironment(req, nil), nil)
	if err != nil || result.rc != 0 {
		t.Fatalf("compound wrapper failed: err=%v rc=%d stderr=%s", err, result.rc, result.stderr)
	}
	receipts := collector.receipts()
	if len(receipts) != 1 || receipts[0].Command != "go test" || receipts[0].ExitCode == 0 {
		t.Fatalf("nested failed test was not observed independently of aggregate success: %+v", receipts)
	}
}

func TestReceiptInstrumentationObservesRuntimeGoPath(t *testing.T) {
	requireBrokerBubblewrap(t)
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":             "module receipt-runtime-path\n\ngo 1.25.0\n",
		"calculator.go":      "package calculator\n",
		"calculator_test.go": "package calculator\n\nimport \"testing\"\n\nfunc TestRed(t *testing.T) { t.Fatal(\"red\") }\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	socketDirectory, err := newPrivateSocketDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(socketDirectory)
	collector, err := startReceiptCollector(root, filepath.Join(socketDirectory, receiptSocketName), []string{"go", "test", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	defer collector.close()
	broker, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	req := brokerRequest{Project: root, ActorHome: t.TempDir()}
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		t.Fatal(err)
	}
	args, err := appendReceiptInstrumentation(sandboxBase(false), broker, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	args = appendMount(args, root, false)
	args = append(args, "--dir", actorBrokerPath, "--ro-bind", socketDirectory, actorBrokerPath, "--chdir", root, "--", "/usr/bin/bash", "-lc", "/opt/megapowers-runtime/go/bin/go test ./...; exit 0")
	result, err := runProcess(context.Background(), "bwrap", args, root, actorEnvironment(req, nil), nil)
	if err != nil || result.rc != 0 {
		t.Fatalf("runtime-path command failed: err=%v rc=%d stderr=%s", err, result.rc, result.stderr)
	}
	receipts := collector.receipts()
	if len(receipts) != 1 || receipts[0].Command != "go test" || receipts[0].ExitCode == 0 {
		t.Fatalf("runtime Go path bypassed receipt instrumentation: %+v", receipts)
	}
}

func TestReceiptInstrumentationPreservesNonTestGoBuild(t *testing.T) {
	requireBrokerBubblewrap(t)
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":  "module example.com/receipt-build\n\ngo 1.25.0\n",
		"main.go": "package main\n\nfunc main() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	broker, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	req := brokerRequest{Project: root, ActorHome: t.TempDir()}
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		t.Fatal(err)
	}
	args, err := appendReceiptInstrumentation(sandboxBase(false), broker, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	args = appendMount(args, root, false)
	args = append(args, "--chdir", root, "--", "/opt/megapowers-runtime/go/bin/go", "build", "-o", "session-hook", ".")
	result, err := runProcess(context.Background(), "bwrap", args, root, actorEnvironment(req, nil), nil)
	if err != nil || result.rc != 0 {
		t.Fatalf("instrumented non-test Go command failed: err=%v rc=%d stdout=%s stderr=%s", err, result.rc, result.stdout, result.stderr)
	}
	if info, err := os.Stat(filepath.Join(root, "session-hook")); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("instrumented Go build did not produce an executable: %v", err)
	}
}

func TestSubswapperAuthentication(t *testing.T) {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	t.Setenv("MEGAPOWERS_BROKER_AUTH_MODE", "subswapper")
	t.Setenv("SUBSWAPPER_PROXY", "1")
	t.Setenv("SUBSWAPPER_SERVICE", "claude")
	t.Setenv("ANTHROPIC_BASE_URL", "http://127.0.0.1:32123")
	secret := strings.Repeat("host-only-capability", 3)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", secret)
	auth, err := resolveAuthentication(req)
	if err != nil || auth.credential != secret {
		t.Fatalf("proxy authentication unavailable: %v", err)
	}
	for _, endpoint := range []string{"https://example.com", "http://localhost:1234", "http://127.0.0.1:1234/path", "http://user:secret@127.0.0.1:1234", "http://127.0.0.1:1234?secret", "http://127.0.0.1", "http://0.0.0.0:1234"} {
		t.Setenv("ANTHROPIC_BASE_URL", endpoint)
		if _, err := resolveAuthentication(req); err == nil || strings.Contains(err.Error(), endpoint) {
			t.Fatal("unsafe endpoint accepted or disclosed")
		}
	}
	t.Setenv("ANTHROPIC_BASE_URL", "http://127.0.0.1:32123")
	t.Setenv("SUBSWAPPER_PROXY", "")
	if _, err := resolveAuthentication(req); err == nil {
		t.Fatal("fixed-token fallback accepted")
	}
	t.Setenv("SUBSWAPPER_PROXY", "1")
	t.Setenv("SUBSWAPPER_SERVICE", "codex")
	if _, err := resolveAuthentication(req); err == nil {
		t.Fatal("wrong service accepted")
	}

	req.Harness = "codex"
	t.Setenv("MEGAPOWERS_BROKER_SUBSWAPPER_URL", "http://127.0.0.1:32124")
	authPath := filepath.Join(paths.root, "auth.json")
	t.Setenv("MEGAPOWERS_BROKER_CODEX_AUTH_FILE", authPath)
	writeAuth := func(account, refresh string) {
		content, _ := json.Marshal(map[string]any{"auth_mode": "chatgpt", "tokens": map[string]any{"access_token": secret, "account_id": account, "refresh_token": refresh}})
		if err := os.WriteFile(authPath, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeAuth("subswapper-proxy", "subswapper-proxy-placeholder")
	auth, err = resolveAuthentication(req)
	if err != nil || auth.credential != secret {
		t.Fatalf("placeholder authentication unavailable: %v", err)
	}
	for _, pair := range [][2]string{{"real-account", "subswapper-proxy-placeholder"}, {"subswapper-proxy", "real-refresh"}} {
		writeAuth(pair[0], pair[1])
		if _, err := resolveAuthentication(req); err == nil {
			t.Fatal("real login accepted as proxy placeholder")
		}
	}
}

func TestSubswapperProxyRoutes(t *testing.T) {
	for _, harness := range []string{"claude", "codex"} {
		t.Run(harness, func(t *testing.T) {
			secret := strings.Repeat("host-only-capability", 3)
			type observed struct{ path, authorization, apiKey, encoding string }
			seen := make(chan observed, 3)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen <- observed{r.URL.RequestURI(), r.Header.Get("Authorization"), r.Header.Get("X-Api-Key"), r.Header.Get("Content-Encoding")}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer upstream.Close()
			proxy, err := startCredentialProxy(harness, "subswapper", secret, upstream.URL, "")
			if err != nil {
				t.Fatal(err)
			}
			defer proxy.close()
			if harness == "codex" {
				if expires, ok := jwtExpiry(proxy.token); !ok || expires.Before(time.Now()) {
					t.Fatal("Codex external auth requires a JWT-shaped per-actor capability")
				}
			}
			paths := []string{"/v1/messages?beta=true", "/v1/messages/count_tokens"}
			want := paths
			if harness == "codex" {
				paths = []string{"/v1/responses", "/v1/responses/compact"}
				want = []string{"/backend-api/codex/responses", "/backend-api/codex/responses/compact"}
			}
			for i, path := range paths {
				request, _ := http.NewRequest(http.MethodPost, proxy.base+path, strings.NewReader(`{}`))
				request.Header.Set("Authorization", "Bearer "+proxy.token)
				request.Header.Set("Content-Encoding", "zstd")
				response, err := http.DefaultClient.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				_ = response.Body.Close()
				if response.StatusCode != 200 {
					t.Fatalf("route rejected: %d", response.StatusCode)
				}
				got := <-seen
				if got.path != want[i] || got.authorization != "Bearer "+secret || got.apiKey != "" || got.encoding != "zstd" {
					t.Fatal("upstream route or credential substitution incorrect")
				}
			}
			for _, path := range []string{"/subswapper/health", "/v1/other", "/v1/responses?override=true"} {
				request, _ := http.NewRequest(http.MethodPost, proxy.base+path, nil)
				request.Header.Set("Authorization", "Bearer "+proxy.token)
				response, err := http.DefaultClient.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				_ = response.Body.Close()
				if response.StatusCode != http.StatusForbidden {
					t.Fatal("non-model route accepted")
				}
			}
		})
	}
}
