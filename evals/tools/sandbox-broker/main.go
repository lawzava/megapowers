// megapowers-eval-broker runs one Claude Code or Codex evaluation actor inside
// a narrow Linux filesystem boundary. Provider credentials stay in this broker;
// the actor receives only a short-lived credential-proxy capability.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	requestLimit          = 4 << 20
	traceLimit            = 64 << 20
	maximumRun            = 24 * time.Hour
	actorBridgeAddress    = "127.0.0.1:43191"
	actorBrokerPath       = "/run/megapowers-broker"
	maximumSocketPath     = 100
	receiptSocketName     = "execution-receipts.sock"
	receiptMaximumRuns    = 256
	receiptMaximumEntries = 4096
	receiptMaximumBytes   = 64 << 20
	receiptMaximumFile    = 16 << 20
	receiptMaximumPath    = 1024
	receiptMaximumArgs    = 128
	receiptMaximumArg     = 4 << 10
	receiptMaximumArgv    = 12 << 10
)

type brokerRequest struct {
	SchemaVersion  string   `json:"schema_version"`
	Harness        string   `json:"harness"`
	Model          string   `json:"model"`
	Effort         string   `json:"effort"`
	Arm            string   `json:"arm"`
	Task           string   `json:"task"`
	FollowupTasks  []string `json:"followup_tasks,omitempty"`
	Project        string   `json:"project"`
	ActorHome      string   `json:"actor_home"`
	PluginRepo     string   `json:"plugin_repo,omitempty"`
	TaskReadRoots  []string `json:"task_read_roots"`
	TaskWriteRoots []string `json:"task_write_roots"`
	OracleCommand  []string `json:"oracle_command,omitempty"`
	TimeoutMS      int64    `json:"timeout_ms"`
}

type actorEvent struct {
	Kind string `json:"kind"`
	Path string `json:"path,omitempty"`
	RC   int    `json:"rc,omitempty"`
	Step int    `json:"step"`
}

type receiptFileState struct {
	Complete     bool     `json:"complete"`
	Digest       string   `json:"digest,omitempty"`
	ChangedFiles []string `json:"changed_files"`
}

type testExecutionReceipt struct {
	SchemaVersion    string           `json:"schema_version"`
	Sequence         int              `json:"sequence"`
	StartedStep      int              `json:"started_step"`
	CompletedStep    int              `json:"completed_step"`
	Command          string           `json:"command"`
	ExitCode         int              `json:"exit_code"`
	OracleMatch      bool             `json:"oracle_match"`
	InvocationDigest string           `json:"invocation_digest"`
	StateStable      bool             `json:"state_stable"`
	Before           receiptFileState `json:"before"`
	After            receiptFileState `json:"after"`
}

type receiptProjectSnapshot struct {
	complete bool
	digest   string
	files    map[string]string
}

type isolationAttestation struct {
	Boundary                    string   `json:"boundary"`
	CredentialsReadableByActor  bool     `json:"credentials_readable_by_actor"`
	SiblingStateReadableByActor bool     `json:"sibling_state_readable_by_actor"`
	TaskReadRoots               []string `json:"task_read_roots"`
	TaskWriteRoots              []string `json:"task_write_roots"`
	ActorHome                   string   `json:"actor_home"`
}

// skillsCatalog reports whether the harness presented the Megapowers skill
// catalog to the model. It is emitted for treatment runs only; Source names
// the detection mechanism, and "unavailable" means the harness path exposes
// no signal, so Rendered carries no meaning.
type skillsCatalog struct {
	Rendered bool     `json:"rendered"`
	Skills   []string `json:"skills"`
	Source   string   `json:"source"`
}

const (
	catalogSourceUnavailable         = "unavailable"
	catalogSourceClaudeInit          = "claude-init-skills"
	catalogSourceClaudeSlashCommands = "claude-init-slash-commands"
	catalogSourceCodexRollout        = "codex-rollout-developer-message"
	skillsInstructionsOpen           = "<skills_instructions>"
	skillsInstructionsClose          = "</skills_instructions>"
)

type brokerResponse struct {
	SchemaVersion   string               `json:"schema_version"`
	CLIVersion      string               `json:"cli_version"`
	Response        string               `json:"response"`
	Trace           string               `json:"trace"`
	Events          []actorEvent         `json:"events"`
	PluginInventory []string             `json:"plugin_inventory"`
	SkillsCatalog   *skillsCatalog       `json:"skills_catalog,omitempty"`
	OracleRC        *int                 `json:"oracle_rc,omitempty"`
	RC              int                  `json:"rc"`
	DurationMS      int64                `json:"duration_ms"`
	Isolation       isolationAttestation `json:"isolation"`
}

type processResult struct {
	stdout   []byte
	stderr   []byte
	rc       int
	duration time.Duration
	timedOut bool
}

type harnessRun struct {
	version   string
	response  string
	trace     []byte
	events    []actorEvent
	rc        int
	duration  time.Duration
	secrets   []string
	inventory []string
	catalog   *skillsCatalog
}

type authentication struct {
	mode       string
	credential string
	accountID  string
	planType   string
	sourcePath string
	upstream   string
}

const (
	authSubscription = "subscription"
	authAPIKey       = "api-key"
	authSubswapper   = "subswapper"
)

type credentialProxy struct {
	server *http.Server
	ln     net.Listener
	token  string
	base   string
	socket string
}

type tcpUnixBridge struct {
	ln   net.Listener
	base string
}

type restrictedConnectProxy struct {
	server *http.Server
	ln     net.Listener
	socket string
}

type watchedEffect struct {
	fd   int
	kind string
}

type protectedEffectMonitor struct {
	watches []watchedEffect
}

type receiptCollector struct {
	listener    *net.UnixListener
	project     string
	oracle      []string
	baseline    receiptProjectSnapshot
	mu          sync.Mutex
	connections map[*net.UnixConn]bool
	records     []testExecutionReceipt
	nextID      int
	nextStep    int
	activeRuns  int
	sealed      bool
	serveDone   chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
}

type receiptClientMessage struct {
	SchemaVersion string   `json:"schema_version"`
	Action        string   `json:"action"`
	ID            int      `json:"id,omitempty"`
	Command       string   `json:"command,omitempty"`
	Arguments     []string `json:"arguments,omitempty"`
	ExitCode      *int     `json:"exit_code,omitempty"`
}

type receiptServerMessage struct {
	Accepted bool `json:"accepted"`
	ID       int  `json:"id,omitempty"`
}

func main() {
	if real, ok := receiptWrapperExecutable(os.Args[0]); ok {
		command := receiptCommand(os.Args[0], os.Args[1:])
		os.Exit(runReceiptWrappedExecutable(real, os.Args[1:], command, filepath.Join(actorBrokerPath, receiptSocketName)))
	}
	if len(os.Args) >= 4 && os.Args[1] == "--actor-bridge" {
		os.Exit(runActorBridge(os.Args[2], os.Args[3], os.Args[4:]))
	}
	if len(os.Args) == 2 && os.Args[1] == "--selftest" {
		if err := selftest(); err != nil {
			fmt.Fprintf(os.Stderr, "sandbox broker selftest: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("sandbox broker selftest: PASS")
		return
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: megapowers-eval-broker [--selftest]")
		os.Exit(2)
	}
	if err := serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox broker: %v\n", err)
		os.Exit(1)
	}
}

func receiptWrapperExecutable(executable string) (string, bool) {
	switch strings.ToLower(filepath.Base(executable)) {
	case "go":
		return "/opt/megapowers-receipt/real/go", true
	case "bash":
		return "/opt/megapowers-receipt/real/bash", true
	case "sh", "dash":
		return "/opt/megapowers-receipt/real/sh", true
	case "python3":
		return "/usr/bin/python3", true
	case "pytest", "py.test":
		return "/usr/bin/pytest", true
	case "npm":
		return "/usr/bin/npm", true
	case "cargo":
		return "/usr/bin/cargo", true
	default:
		return "", false
	}
}

func serve(input io.Reader, output io.Writer) error {
	req, err := decodeRequest(input)
	if err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if err := validateRequest(&req); err != nil {
		return fmt.Errorf("validate request: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.TimeoutMS)*time.Millisecond)
	defer cancel()
	result, err := execute(ctx, req)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return nil
}

func decodeRequest(input io.Reader) (brokerRequest, error) {
	limited := io.LimitReader(input, requestLimit+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return brokerRequest{}, err
	}
	if len(content) > requestLimit {
		return brokerRequest{}, errors.New("request exceeds 4 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var req brokerRequest
	if err := decoder.Decode(&req); err != nil {
		return brokerRequest{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return brokerRequest{}, errors.New("request has trailing data")
	}
	return req, nil
}

func validateRequest(req *brokerRequest) error {
	if req.SchemaVersion != "2" {
		return errors.New("schema_version must be 2")
	}
	if req.Harness != "claude" && req.Harness != "codex" {
		return errors.New("harness must be claude or codex")
	}
	if req.Arm != "control" && req.Arm != "treatment" {
		return errors.New("arm must be control or treatment")
	}
	if strings.TrimSpace(req.Model) == "" || strings.TrimSpace(req.Task) == "" {
		return errors.New("model and task are required")
	}
	if len(req.FollowupTasks) > 8 {
		return errors.New("followup_tasks must contain at most 8 turns")
	}
	promptBytes := len(req.Task)
	for index, task := range req.FollowupTasks {
		if strings.TrimSpace(task) == "" || strings.ContainsRune(task, '\x00') {
			return fmt.Errorf("followup_tasks[%d] must be non-empty and contain no NUL", index)
		}
		promptBytes += len(task)
	}
	if promptBytes > 256<<10 {
		return errors.New("task and followup_tasks exceed 256 KiB")
	}
	if strings.ContainsRune(req.Model, '\x00') || strings.ContainsRune(req.Effort, '\x00') {
		return errors.New("model and effort must not contain NUL")
	}
	if req.TimeoutMS < 1 || time.Duration(req.TimeoutMS)*time.Millisecond > maximumRun {
		return errors.New("timeout_ms must be between 1 ms and 24 hours")
	}

	paths := []*string{&req.Project, &req.ActorHome}
	if req.PluginRepo != "" {
		paths = append(paths, &req.PluginRepo)
	}
	for _, value := range paths {
		canonical, err := canonicalDirectory(*value)
		if err != nil {
			return err
		}
		*value = canonical
	}
	if pathsOverlap(req.Project, req.ActorHome) {
		return errors.New("project and actor_home must not overlap")
	}
	entries, err := os.ReadDir(req.ActorHome)
	if err != nil {
		return fmt.Errorf("inspect actor_home: %w", err)
	}
	if len(entries) != 0 {
		return errors.New("actor_home must be empty before the broker starts")
	}
	if req.PluginRepo != "" && (pathsOverlap(req.Project, req.PluginRepo) || pathsOverlap(req.ActorHome, req.PluginRepo)) {
		return errors.New("plugin_repo must not overlap project or actor_home")
	}

	wantRead := []string{req.Project}
	if req.Arm == "treatment" {
		if req.PluginRepo == "" {
			return errors.New("treatment requires plugin_repo")
		}
		wantRead = append(wantRead, req.PluginRepo)
	} else if req.PluginRepo != "" {
		return errors.New("control must not receive plugin_repo")
	}
	if !samePaths(req.TaskReadRoots, wantRead) {
		return errors.New("task_read_roots do not match the arm")
	}
	if !samePaths(req.TaskWriteRoots, []string{req.Project}) {
		return errors.New("task_write_roots must contain only project")
	}
	for i, value := range req.TaskReadRoots {
		canonical, err := canonicalDirectory(value)
		if err != nil {
			return fmt.Errorf("task_read_roots[%d]: %w", i, err)
		}
		req.TaskReadRoots[i] = canonical
	}
	for i, value := range req.TaskWriteRoots {
		canonical, err := canonicalDirectory(value)
		if err != nil {
			return fmt.Errorf("task_write_roots[%d]: %w", i, err)
		}
		req.TaskWriteRoots[i] = canonical
	}
	if len(req.OracleCommand) > 0 {
		for _, value := range req.OracleCommand {
			if value == "" || strings.ContainsRune(value, '\x00') {
				return errors.New("oracle_command contains an invalid argument")
			}
		}
	}
	return nil
}

func canonicalDirectory(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("path %q must be absolute", path)
	}
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", path, err)
	}
	resolved = filepath.Clean(resolved)
	if resolved != clean {
		return "", fmt.Errorf("path %q must be canonical and contain no symlinks", path)
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("path %q is not a real directory", path)
	}
	return resolved, nil
}

func samePaths(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]bool, len(got))
	for _, value := range got {
		value = filepath.Clean(value)
		if seen[value] {
			return false
		}
		seen[value] = true
	}
	for _, value := range want {
		if !seen[filepath.Clean(value)] {
			return false
		}
	}
	return true
}

func pathsOverlap(a, b string) bool {
	return pathContains(a, b) || pathContains(b, a)
}

func pathContains(parent, child string) bool {
	relative, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func execute(ctx context.Context, req brokerRequest) (brokerResponse, error) {
	if runtime.GOOS != "linux" {
		return brokerResponse{}, errors.New("this broker build requires Linux bubblewrap")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		return brokerResponse{}, errors.New("bubblewrap is required")
	}
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		return brokerResponse{}, err
	}
	binary, err := harnessBinary(req.Harness)
	if err != nil {
		return brokerResponse{}, err
	}
	auth, err := resolveAuthentication(req)
	if err != nil {
		return brokerResponse{}, err
	}
	if err := validateFollowupTransport(req, auth); err != nil {
		return brokerResponse{}, err
	}
	brokerDirectory, err := newPrivateSocketDirectory()
	if err != nil {
		return brokerResponse{}, fmt.Errorf("create private broker directory: %w", err)
	}
	defer os.RemoveAll(brokerDirectory)
	receiptCollector, err := startReceiptCollector(req.Project, filepath.Join(brokerDirectory, receiptSocketName), req.OracleCommand)
	if err != nil {
		return brokerResponse{}, fmt.Errorf("start execution receipt collector: %w", err)
	}
	defer receiptCollector.close()
	var proxy *credentialProxy
	if req.Harness == "claude" || auth.mode == authAPIKey {
		proxySocket := filepath.Join(brokerDirectory, "provider.sock")
		proxy, err = startCredentialProxy(req.Harness, auth.mode, auth.credential, auth.upstream, proxySocket)
		if err != nil {
			return brokerResponse{}, fmt.Errorf("start credential proxy: %w", err)
		}
		defer proxy.close()
	}

	run, err := runHarness(ctx, req, binary, auth, proxy, brokerDirectory, receiptCollector, toolchain)
	if err != nil {
		return brokerResponse{}, err
	}
	secrets := append(run.secrets, auth.credential)
	if proxy != nil {
		secrets = append(secrets, proxy.token)
	}
	run.version = redact(run.version, secrets)
	run.response = redact(run.response, secrets)
	run.trace = []byte(redact(string(run.trace), secrets))
	for index := range run.events {
		run.events[index].Path = redact(run.events[index].Path, secrets)
	}

	var oracleRC *int
	if len(req.OracleCommand) > 0 {
		rc := 124
		if ctx.Err() == nil {
			result, oracleErr := runOracle(ctx, req, toolchain)
			if oracleErr != nil {
				return brokerResponse{}, oracleErr
			}
			rc = result.rc
		}
		oracleRC = &rc
	}
	return brokerResponse{
		SchemaVersion:   "2",
		CLIVersion:      run.version,
		Response:        run.response,
		Trace:           string(run.trace),
		Events:          run.events,
		PluginInventory: run.inventory,
		SkillsCatalog:   run.catalog,
		OracleRC:        oracleRC,
		RC:              run.rc,
		DurationMS:      run.duration.Milliseconds(),
		Isolation: isolationAttestation{
			Boundary:                    "bwrap",
			CredentialsReadableByActor:  false,
			SiblingStateReadableByActor: false,
			TaskReadRoots:               req.TaskReadRoots,
			TaskWriteRoots:              req.TaskWriteRoots,
			ActorHome:                   req.ActorHome,
		},
	}, nil
}

func resolveAuthentication(req brokerRequest) (authentication, error) {
	mode := strings.TrimSpace(os.Getenv("MEGAPOWERS_BROKER_AUTH_MODE"))
	if mode == "" {
		mode = authSubscription
	}
	switch mode {
	case authSubswapper:
		return readSubswapperAuthentication(req)
	case authSubscription:
		return readSubscriptionAuthentication(req)
	case authAPIKey:
		credential, err := readProviderKey(req)
		if err != nil {
			return authentication{}, err
		}
		return authentication{mode: authAPIKey, credential: credential}, nil
	default:
		return authentication{}, errors.New("MEGAPOWERS_BROKER_AUTH_MODE must be subscription, subswapper, or api-key")
	}
}

func validateFollowupTransport(req brokerRequest, auth authentication) error {
	if req.Harness == "codex" && len(req.FollowupTasks) > 0 && auth.mode == authAPIKey {
		return errors.New("codex followup_tasks require the subscription or subswapper app-server transport")
	}
	return nil
}

func readSubswapperAuthentication(req brokerRequest) (authentication, error) {
	// Reject Subswapper's fixed-login fallback so a proxy outage cannot silently
	// change routing or introduce a real provider credential into this mode.
	if os.Getenv("SUBSWAPPER_PROXY") != "1" || os.Getenv("SUBSWAPPER_SERVICE") != req.Harness {
		return authentication{}, errors.New("subswapper mode requires a matching home run proxy launch")
	}
	endpoint := os.Getenv("MEGAPOWERS_BROKER_SUBSWAPPER_URL")
	if req.Harness == "claude" {
		endpoint = os.Getenv("ANTHROPIC_BASE_URL")
	}
	base, err := url.Parse(endpoint)
	if err != nil || base.Scheme != "http" || base.User != nil || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" || base.RawPath != "" || (base.Path != "" && base.Path != "/") {
		return authentication{}, errors.New("subswapper endpoint must be an HTTP loopback origin with an explicit port")
	}
	ip := net.ParseIP(base.Hostname())
	port, err := strconv.Atoi(base.Port())
	if ip == nil || !ip.IsLoopback() || err != nil || port < 1 || port > 65535 {
		return authentication{}, errors.New("subswapper endpoint must be an HTTP loopback origin with an explicit port")
	}
	auth := authentication{mode: authSubswapper, upstream: strings.TrimSuffix(base.String(), "/")}
	if req.Harness == "claude" {
		auth.credential = os.Getenv("CLAUDE_CODE_OAUTH_TOKEN")
		if !validBearerCredential(auth.credential) {
			return authentication{}, errors.New("subswapper proxy capability is unavailable")
		}
		return auth, nil
	}
	relativeDefault := filepath.Join(".codex", "auth.json")
	if home := os.Getenv("CODEX_HOME"); home != "" {
		relativeDefault = filepath.Join(home, "auth.json")
	}
	path, err := subscriptionCredentialPath(req, "MEGAPOWERS_BROKER_CODEX_AUTH_FILE", relativeDefault)
	if err != nil {
		return authentication{}, err
	}
	content, err := readPrivateCredentialFile(req, path, 64<<10)
	if err != nil {
		return authentication{}, err
	}
	var stored struct {
		AuthMode string `json:"auth_mode"`
		Tokens   struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			AccountID    string `json:"account_id"`
		} `json:"tokens"`
	}
	if json.Unmarshal(content, &stored) != nil || stored.AuthMode != "chatgpt" || stored.Tokens.AccountID != "subswapper-proxy" || stored.Tokens.RefreshToken != "subswapper-proxy-placeholder" || !validBearerCredential(stored.Tokens.AccessToken) {
		return authentication{}, errors.New("subswapper mode requires a Codex proxy placeholder, not a provider login")
	}
	auth.credential, auth.accountID, auth.sourcePath = stored.Tokens.AccessToken, stored.Tokens.AccountID, path
	return auth, nil
}

func readSubscriptionAuthentication(req brokerRequest) (authentication, error) {
	if req.Harness == "claude" {
		path, err := subscriptionCredentialPath(req, "MEGAPOWERS_BROKER_CLAUDE_CREDENTIALS_FILE", filepath.Join(".claude", ".credentials.json"))
		if err != nil {
			return authentication{}, err
		}
		content, err := readPrivateCredentialFile(req, path, 64<<10)
		if err != nil {
			return authentication{}, err
		}
		var stored struct {
			ClaudeAIOAuth struct {
				AccessToken string `json:"accessToken"`
				ExpiresAt   int64  `json:"expiresAt"`
			} `json:"claudeAiOauth"`
		}
		if json.Unmarshal(content, &stored) != nil || !validBearerCredential(stored.ClaudeAIOAuth.AccessToken) {
			return authentication{}, errors.New("claude subscription credentials are unavailable; run claude /login or set MEGAPOWERS_BROKER_AUTH_MODE=api-key explicitly")
		}
		if stored.ClaudeAIOAuth.ExpiresAt > 0 && time.UnixMilli(stored.ClaudeAIOAuth.ExpiresAt).Before(time.Now().Add(2*time.Minute)) {
			return authentication{}, errors.New("claude subscription access has expired or is too close to expiry; refresh claude login before running the broker")
		}
		return authentication{mode: authSubscription, credential: stored.ClaudeAIOAuth.AccessToken, sourcePath: path}, nil
	}

	path, err := subscriptionCredentialPath(req, "MEGAPOWERS_BROKER_CODEX_AUTH_FILE", filepath.Join(".codex", "auth.json"))
	if err != nil {
		return authentication{}, err
	}
	content, err := readPrivateCredentialFile(req, path, 64<<10)
	if err != nil {
		return authentication{}, err
	}
	var stored struct {
		AuthMode string `json:"auth_mode"`
		Tokens   struct {
			AccessToken string `json:"access_token"`
			AccountID   string `json:"account_id"`
		} `json:"tokens"`
	}
	if json.Unmarshal(content, &stored) != nil || stored.AuthMode != "chatgpt" || !validBearerCredential(stored.Tokens.AccessToken) || strings.TrimSpace(stored.Tokens.AccountID) == "" {
		return authentication{}, errors.New("codex ChatGPT subscription credentials are unavailable; run codex login or set MEGAPOWERS_BROKER_AUTH_MODE=api-key explicitly")
	}
	if expires, ok := jwtExpiry(stored.Tokens.AccessToken); ok && expires.Before(time.Now().Add(2*time.Minute)) {
		return authentication{}, errors.New("codex subscription access has expired or is too close to expiry; refresh codex login before running the broker")
	}
	return authentication{mode: authSubscription, credential: stored.Tokens.AccessToken, accountID: stored.Tokens.AccountID, sourcePath: path}, nil
}

func subscriptionCredentialPath(req brokerRequest, environmentName, relativeDefault string) (string, error) {
	path := strings.TrimSpace(os.Getenv(environmentName))
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate subscription credentials: %w", err)
		}
		path = relativeDefault
		if !filepath.IsAbs(path) {
			path = filepath.Join(home, path)
		}
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s must name an absolute file", environmentName)
	}
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil || resolved != clean {
		return "", fmt.Errorf("%s must be canonical and contain no symlinks", environmentName)
	}
	if credentialPathIsRuntimeVisible(resolved) {
		return "", fmt.Errorf("%s must stay outside runtime mounts", environmentName)
	}
	for _, visible := range append(append([]string{}, req.TaskReadRoots...), req.ActorHome) {
		if pathsOverlap(resolved, visible) {
			return "", fmt.Errorf("%s must stay outside actor-visible roots", environmentName)
		}
	}
	return resolved, nil
}

func readPrivateCredentialFile(req brokerRequest, path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("subscription credential file is unavailable")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > maximum {
		return nil, errors.New("subscription credential file must be private, regular, and bounded")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return nil, errors.New("subscription credential file must have exactly one hard link")
	}
	for _, visible := range append(append([]string{}, req.TaskReadRoots...), req.ActorHome) {
		if err := rejectVisibleHardlink(visible, info); err != nil {
			return nil, errors.New("subscription credential inode is actor-visible")
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("read subscription credential file")
	}
	return content, nil
}

func validBearerCredential(value string) bool {
	return len(value) >= 32 && !strings.ContainsAny(value, "\r\n\x00 \t")
}

func jwtExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Expires int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Expires <= 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Expires, 0), true
}

func harnessBinary(harness string) (string, error) {
	envName := "MEGAPOWERS_BROKER_" + strings.ToUpper(harness) + "_BIN"
	path := os.Getenv(envName)
	if path == "" {
		var err error
		path, err = exec.LookPath(harness)
		if err != nil {
			return "", fmt.Errorf("find %s: %w", harness, err)
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve %s binary: %w", harness, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("%s binary is not a regular executable", harness)
	}
	return resolved, nil
}

func readProviderKey(req brokerRequest) (string, error) {
	envName := "MEGAPOWERS_BROKER_" + strings.ToUpper(req.Harness) + "_API_KEY_FILE"
	path := os.Getenv(envName)
	if path == "" {
		return "", fmt.Errorf("%s is required; native credential stores are never mounted", envName)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s must name an absolute file", envName)
	}
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil || resolved != clean {
		return "", fmt.Errorf("%s must be canonical and contain no symlinks", envName)
	}
	if credentialPathIsRuntimeVisible(resolved) {
		return "", fmt.Errorf("%s must stay outside runtime mounts", envName)
	}
	for _, visible := range append(append([]string{}, req.TaskReadRoots...), req.ActorHome) {
		if pathsOverlap(resolved, visible) {
			return "", fmt.Errorf("%s must stay outside actor-visible roots", envName)
		}
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > 16<<10 {
		return "", fmt.Errorf("%s must be a private regular file no larger than 16 KiB", envName)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return "", fmt.Errorf("%s must have exactly one hard link", envName)
	}
	for _, visible := range append(append([]string{}, req.TaskReadRoots...), req.ActorHome) {
		if err := rejectVisibleHardlink(visible, info); err != nil {
			return "", fmt.Errorf("%s: %w", envName, err)
		}
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(content))
	if len(key) < 16 || strings.ContainsAny(key, "\r\n\x00") {
		return "", fmt.Errorf("%s does not contain one provider key", envName)
	}
	return key, nil
}

func credentialPathIsRuntimeVisible(path string) bool {
	for _, runtimeRoot := range []string{"/usr", "/etc"} {
		if pathsOverlap(path, runtimeRoot) {
			return true
		}
	}
	return false
}

func rejectVisibleHardlink(root string, credentialInfo fs.FileInfo) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && os.SameFile(info, credentialInfo) {
			return fmt.Errorf("credential inode is actor-visible at %q", path)
		}
		return nil
	})
}

func validateSocketPath(path string) error {
	if len(path) > maximumSocketPath {
		return fmt.Errorf("socket path %q exceeds the %d-byte Linux Unix-socket safety bound; shorten TMPDIR or flatten the socket directory layout", path, maximumSocketPath)
	}
	return nil
}

func newPrivateSocketDirectory() (string, error) {
	directory, err := os.MkdirTemp("/tmp", "mpb-")
	if err != nil {
		return "", err
	}
	if err := validateSocketPath(filepath.Join(directory, "codex-egress.sock")); err != nil {
		_ = os.Remove(directory)
		return "", err
	}
	return directory, nil
}

func startCredentialProxy(harness, authMode, providerCredential, upstreamOverride, socketPath string) (*credentialProxy, error) {
	if socketPath != "" {
		if err := validateSocketPath(socketPath); err != nil {
			return nil, err
		}
	}
	upstream := upstreamOverride
	if upstream == "" {
		if harness == "claude" {
			upstream = "https://api.anthropic.com"
		} else {
			upstream = "https://api.openai.com"
		}
	}
	base, err := url.Parse(upstream)
	if err != nil {
		return nil, err
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)
	if harness == "codex" && authMode == authSubswapper {
		// Codex parses external auth as an ID token. These synthetic claims
		// describe only this broker capability; no provider sees this token.
		claims, err := json.Marshal(map[string]any{
			"iss": "https://megapowers.invalid", "sub": "megapowers-eval", "email": "evaluation@localhost",
			"iat": time.Now().Unix(), "exp": time.Now().Add(maximumRun + time.Minute).Unix(),
			"https://api.openai.com/auth": map[string]string{"chatgpt_account_id": "megapowers-eval", "chatgpt_user_id": "megapowers-eval", "chatgpt_plan_type": "pro"},
		})
		if err != nil {
			return nil, err
		}
		token = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." + base64.RawURLEncoding.EncodeToString(claims) + "." + base64.RawURLEncoding.EncodeToString(tokenBytes)
	}
	network := "tcp"
	address := "127.0.0.1:0"
	baseURL := ""
	if socketPath != "" {
		network = "unix"
		address = socketPath
		baseURL = "http://" + actorBridgeAddress
	}
	ln, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	if socketPath != "" {
		if err := os.Chmod(socketPath, 0o600); err != nil {
			_ = ln.Close()
			return nil, err
		}
	} else {
		baseURL = "http://" + ln.Addr().String()
	}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 10 * time.Minute,
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("provider redirects are disabled")
	}}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !proxyAuthorized(harness, r.Header, token) {
			http.Error(w, "invalid broker capability", http.StatusForbidden)
			return
		}
		if !proxyRequestAllowed(harness, r) {
			http.Error(w, "provider route not allowed", http.StatusForbidden)
			return
		}
		target := *base
		target.Path = singleJoiningSlash(base.Path, r.URL.Path)
		if authMode == authSubswapper && harness == "codex" {
			target.Path = "/backend-api/codex" + strings.TrimPrefix(r.URL.Path, "/v1")
		}
		target.RawQuery = r.URL.RawQuery
		body := http.MaxBytesReader(w, r.Body, traceLimit)
		upstreamRequest, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target.String(), body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		copyAllowedRequestHeaders(upstreamRequest.Header, r.Header)
		if harness == "claude" && (authMode == authSubscription || authMode == authSubswapper) {
			upstreamRequest.Header.Del("X-Api-Key")
			upstreamRequest.Header.Set("Authorization", "Bearer "+providerCredential)
			ensureHeaderToken(upstreamRequest.Header, "Anthropic-Beta", "oauth-2025-04-20")
		} else if harness == "claude" {
			upstreamRequest.Header.Set("X-Api-Key", providerCredential)
			upstreamRequest.Header.Del("Authorization")
		} else {
			upstreamRequest.Header.Set("Authorization", "Bearer "+providerCredential)
		}
		response, err := client.Do(upstreamRequest)
		if err != nil {
			http.Error(w, "provider unavailable", http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		copyHeaders(w.Header(), response.Header)
		removeHopHeaders(w.Header())
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, io.LimitReader(response.Body, traceLimit))
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second}
	proxy := &credentialProxy{server: server, ln: ln, token: token, base: baseURL, socket: socketPath}
	go func() { _ = server.Serve(ln) }()
	return proxy, nil
}

func ensureHeaderToken(header http.Header, key, token string) {
	for _, value := range header.Values(key) {
		for _, candidate := range strings.Split(value, ",") {
			if strings.TrimSpace(candidate) == token {
				return
			}
		}
	}
	header.Add(key, token)
}

func proxyAuthorized(harness string, header http.Header, token string) bool {
	if harness == "claude" {
		return subtleEqual(header.Get("X-Api-Key"), token) || subtleEqual(strings.TrimPrefix(header.Get("Authorization"), "Bearer "), token)
	}
	return subtleEqual(strings.TrimPrefix(header.Get("Authorization"), "Bearer "), token)
}

func proxyRequestAllowed(harness string, request *http.Request) bool {
	if request.Method != http.MethodPost {
		return false
	}
	allowed := map[string]bool{}
	if harness == "claude" {
		if request.URL.RawQuery != "" && request.URL.RawQuery != "beta=true" {
			return false
		}
		allowed["/v1/messages"] = true
		allowed["/v1/messages/count_tokens"] = true
	} else {
		if request.URL.RawQuery != "" {
			return false
		}
		allowed["/v1/responses"] = true
		allowed["/v1/responses/compact"] = true
	}
	return allowed[request.URL.Path]
}

func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	difference := byte(0)
	for i := range a {
		difference |= a[i] ^ b[i]
	}
	return difference == 0
}

func (p *credentialProxy) close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = p.server.Shutdown(ctx)
	_ = p.ln.Close()
	if p.socket != "" {
		_ = os.Remove(p.socket)
	}
}

func startRestrictedConnectProxy(socketPath string, allowedHosts map[string]bool) (*restrictedConnectProxy, error) {
	if err := validateSocketPath(socketPath); err != nil {
		return nil, err
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}
	proxy := &restrictedConnectProxy{ln: ln, socket: socketPath}
	proxy.server = &http.Server{ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		host := strings.ToLower(request.Host)
		if request.Method != http.MethodConnect || request.URL.RawQuery != "" || !allowedHosts[host] {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		upstream, err := net.DialTimeout("tcp", host, 15*time.Second)
		if err != nil {
			http.Error(writer, "provider unavailable", http.StatusBadGateway)
			return
		}
		hijacker, ok := writer.(http.Hijacker)
		if !ok {
			_ = upstream.Close()
			http.Error(writer, "tunnel unavailable", http.StatusInternalServerError)
			return
		}
		connection, buffered, err := hijacker.Hijack()
		if err != nil {
			_ = upstream.Close()
			return
		}
		if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			_ = connection.Close()
			_ = upstream.Close()
			return
		}
		if err := buffered.Flush(); err != nil {
			_ = connection.Close()
			_ = upstream.Close()
			return
		}
		go func() {
			_, _ = io.Copy(upstream, connection)
			_ = upstream.Close()
		}()
		_, _ = io.Copy(connection, upstream)
		_ = connection.Close()
		_ = upstream.Close()
	})}
	go func() { _ = proxy.server.Serve(ln) }()
	return proxy, nil
}

func (p *restrictedConnectProxy) close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = p.server.Shutdown(ctx)
	_ = p.ln.Close()
	_ = os.Remove(p.socket)
}

func startTCPUnixBridge(socketPath, address string) (*tcpUnixBridge, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	bridge := &tcpUnixBridge{ln: listener, base: "http://" + listener.Addr().String()}
	go bridge.serve(socketPath)
	return bridge, nil
}

func (b *tcpUnixBridge) serve(socketPath string) {
	for {
		connection, err := b.ln.Accept()
		if err != nil {
			return
		}
		go bridgeConnection(connection, socketPath)
	}
}

func bridgeConnection(connection net.Conn, socketPath string) {
	upstream, err := net.Dial("unix", socketPath)
	if err != nil {
		_ = connection.Close()
		return
	}
	go func() {
		_, _ = io.Copy(upstream, connection)
		_ = upstream.Close()
	}()
	_, _ = io.Copy(connection, upstream)
	_ = connection.Close()
	_ = upstream.Close()
}

func (b *tcpUnixBridge) close() { _ = b.ln.Close() }

func runActorBridge(socketPath, binary string, args []string) int {
	bridge, err := startTCPUnixBridge(socketPath, actorBridgeAddress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "actor credential bridge: %v\n", err)
		return 125
	}
	defer bridge.close()
	command := exec.Command(binary, args...)
	command.Env = os.Environ()
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "actor harness: %v\n", err)
		return 125
	}
	return 0
}

func singleJoiningSlash(a, b string) string {
	return strings.TrimRight(a, "/") + "/" + strings.TrimLeft(b, "/")
}

func copyHeaders(destination, source http.Header) {
	for key, values := range source {
		for _, value := range values {
			destination.Add(key, value)
		}
	}
}

func copyAllowedRequestHeaders(destination, source http.Header) {
	for _, key := range []string{
		"Accept", "Content-Type", "Content-Encoding", "User-Agent", "Anthropic-Version", "Anthropic-Beta",
		"OpenAI-Beta", "OpenAI-Organization", "OpenAI-Project",
	} {
		for _, value := range source.Values(key) {
			destination.Add(key, value)
		}
	}
}

func removeHopHeaders(header http.Header) {
	for _, name := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"} {
		header.Del(name)
	}
}

func runHarness(ctx context.Context, req brokerRequest, binary string, auth authentication, proxy *credentialProxy, brokerDirectory string, receipts *receiptCollector, toolchain hostGoToolchain) (harnessRun, error) {
	versionResult, err := runHarnessUtility(ctx, req, binary, []string{"--version"}, minimalEnvironment(req.ActorHome), false)
	if err != nil {
		return harnessRun{}, fmt.Errorf("read %s version: %w", req.Harness, err)
	}
	if versionResult.rc != 0 {
		return harnessRun{}, fmt.Errorf("read %s version: exit %d: %s", req.Harness, versionResult.rc, strings.TrimSpace(string(versionResult.stderr)))
	}
	version := strings.TrimSpace(string(versionResult.stdout))
	if version == "" {
		return harnessRun{}, errors.New("harness version is empty")
	}

	var args []string
	var environment []string
	var input []byte
	var claudeFrames [][]byte
	var inventory []string
	insideBinary := "/opt/megapowers/" + req.Harness
	if req.Harness == "claude" {
		settings, err := writeClaudeSettings(req)
		if err != nil {
			return harnessRun{}, err
		}
		args = []string{"-p", "--model", req.Model, "--no-session-persistence", "--output-format", "stream-json", "--verbose", "--include-hook-events", "--forward-subagent-text", "--permission-mode", "acceptEdits", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--settings", settings, "--disallowedTools", "WebFetch", "WebSearch"}
		if req.Effort != "" {
			args = append(args, "--effort", req.Effort)
		}
		if req.Arm == "treatment" {
			args = append(args, "--plugin-dir", req.PluginRepo)
		}
		if len(req.FollowupTasks) > 0 {
			args = append(args, "--input-format", "stream-json")
		}
		authEnvironment := map[string]string{
			"ANTHROPIC_BASE_URL":                       proxy.base,
			"CLAUDE_CONFIG_DIR":                        filepath.Join(req.ActorHome, ".claude"),
			"CLAUDE_CODE_SUBPROCESS_ENV_SCRUB":         "1",
			"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		}
		if auth.mode == authSubscription || auth.mode == authSubswapper {
			authEnvironment["CLAUDE_CODE_OAUTH_TOKEN"] = proxy.token
			if auth.mode == authSubswapper {
				authEnvironment["_CLAUDE_CODE_ASSUME_FIRST_PARTY_BASE_URL"] = "1"
			}
		} else {
			authEnvironment["ANTHROPIC_API_KEY"] = proxy.token
		}
		environment = actorEnvironment(req, authEnvironment)
		if len(req.FollowupTasks) > 0 {
			claudeFrames, err = claudeStreamingFrames(append([]string{req.Task}, req.FollowupTasks...))
			if err != nil {
				return harnessRun{}, err
			}
		} else {
			input = []byte(req.Task)
		}
	} else {
		codexHome, installedPlugin, observedInventory, err := prepareCodexHome(ctx, req, binary)
		if err != nil {
			return harnessRun{}, err
		}
		inventory = observedInventory
		if auth.mode == authSubscription || auth.mode == authSubswapper {
			effectMonitor, err := startProtectedEffectMonitor(req.Project)
			if err != nil {
				return harnessRun{}, err
			}
			defer effectMonitor.close()
			result, err := runCodexAppServer(ctx, req, binary, codexHome, auth, brokerDirectory, toolchain)
			if err != nil && !result.timedOut {
				return harnessRun{}, err
			}
			if result.timedOut {
				result.rc = 124
			}
			trace := result.stdout
			if len(trace) > traceLimit {
				trace = trace[:traceLimit]
				result.rc = 125
			}
			sealedReceipts, sealErr := receipts.seal()
			if sealErr != nil {
				return harnessRun{}, sealErr
			}
			trace, err = appendTrustedExecutionReceipts(trace, sealedReceipts)
			if err != nil {
				return harnessRun{}, err
			}
			response, events, complete := normalizeTraceTurns(req.Harness, trace, result.rc, len(req.FollowupTasks)+1)
			if !complete {
				dumpDiagnosticTrace(os.Getenv("MEGAPOWERS_BROKER_TRACE_DUMP"), "codex-incomplete-trace", trace)
			}
			events = appendObservedEffects(events, effectMonitor.events(), complete)
			if !complete {
				events = removeTraceComplete(events)
				if result.rc == 0 {
					result.rc = 125
				}
			}
			var catalog *skillsCatalog
			if req.Arm == "treatment" {
				detected, block, catalogErr := codexSkillsCatalog(codexHome, installedPlugin)
				if catalogErr != nil {
					return harnessRun{}, fmt.Errorf("read Codex skills catalog: %w", catalogErr)
				}
				catalog = &detected
				// The block is appended after normalization so it can never
				// become actor evidence; it exists for diagnostics only.
				trace = append(trace, skillsCatalogTraceLine(detected, block)...)
			}
			return harnessRun{version: version, response: response, trace: trace, events: events, rc: result.rc, duration: result.duration, secrets: []string{auth.credential}, inventory: inventory, catalog: catalog}, nil
		}
		args = []string{"exec", "--json", "--ephemeral", "--ignore-rules", "--skip-git-repo-check", "-C", req.Project, "-s", "workspace-write", "-c", `approval_policy="never"`, "-c", `sandbox_workspace_write.network_access=false`, "-c", `shell_environment_policy.inherit="none"`, "-m", req.Model}
		if req.Effort != "" {
			args = append(args, "-c", "model_reasoning_effort="+strconv.Quote(req.Effort))
		}
		args = append(args, "-")
		environment = actorEnvironment(req, map[string]string{
			"CODEX_HOME":      codexHome,
			"OPENAI_API_KEY":  proxy.token,
			"OPENAI_BASE_URL": proxy.base + "/v1",
		})
		input = []byte(req.Task)
	}

	sandboxArgs, err := actorSandboxArgs(req, binary, insideBinary, args, proxy.socket, toolchain)
	if err != nil {
		return harnessRun{}, err
	}
	effectMonitor, err := startProtectedEffectMonitor(req.Project)
	if err != nil {
		return harnessRun{}, err
	}
	defer effectMonitor.close()
	var result processResult
	if len(claudeFrames) > 0 {
		result, err = runClaudeStreamingProcess(ctx, "bwrap", sandboxArgs, req.Project, environment, claudeFrames)
	} else {
		result, err = runProcess(ctx, "bwrap", sandboxArgs, req.Project, environment, input)
	}
	if err != nil && !result.timedOut {
		return harnessRun{}, fmt.Errorf("run isolated %s: %w: %s", req.Harness, err, redact(strings.TrimSpace(string(result.stderr)), []string{proxy.token}))
	}
	if result.timedOut {
		result.rc = 124
	}
	trace := result.stdout
	if len(trace) > traceLimit {
		trace = trace[:traceLimit]
		result.rc = 125
	}
	sealedReceipts, sealErr := receipts.seal()
	if sealErr != nil {
		return harnessRun{}, sealErr
	}
	trace, err = appendTrustedExecutionReceipts(trace, sealedReceipts)
	if err != nil {
		return harnessRun{}, err
	}
	var catalog *skillsCatalog
	if req.Harness == "claude" {
		inventory, err = observedClaudeInventory(trace, req)
		if err != nil {
			return harnessRun{}, err
		}
		if req.Arm == "treatment" {
			detected := claudeSkillsCatalog(trace)
			catalog = &detected
		}
	} else if req.Arm == "treatment" {
		// codex exec --ephemeral persists no rollout, so the developer prompt
		// is not observable on the API-key fallback path.
		catalog = &skillsCatalog{Rendered: false, Skills: []string{}, Source: catalogSourceUnavailable}
	}
	response, events, complete := normalizeTraceTurns(req.Harness, trace, result.rc, len(req.FollowupTasks)+1)
	if !complete {
		dumpDiagnosticTrace(os.Getenv("MEGAPOWERS_BROKER_TRACE_DUMP"), "claude-incomplete-trace", trace)
	}
	events = appendObservedEffects(events, effectMonitor.events(), complete)
	if !complete {
		events = removeTraceComplete(events)
		if result.rc == 0 {
			result.rc = 125
		}
	}
	secrets := []string{auth.credential}
	if proxy != nil {
		secrets = append(secrets, proxy.token)
	}
	return harnessRun{version: version, response: response, trace: trace, events: events, rc: result.rc, duration: result.duration, secrets: secrets, inventory: inventory, catalog: catalog}, nil
}

func claudeStreamingInput(tasks []string) ([]byte, error) {
	frames, err := claudeStreamingFrames(tasks)
	if err != nil {
		return nil, err
	}
	return bytes.Join(frames, nil), nil
}

func claudeStreamingFrames(tasks []string) ([][]byte, error) {
	frames := make([][]byte, 0, len(tasks))
	for _, task := range tasks {
		var encoded bytes.Buffer
		encoder := json.NewEncoder(&encoded)
		encoder.SetEscapeHTML(false)
		frame := map[string]any{
			"type": "user",
			"message": map[string]any{
				"role":    "user",
				"content": []any{map[string]any{"type": "text", "text": task}},
			},
		}
		if err := encoder.Encode(frame); err != nil {
			return nil, err
		}
		frames = append(frames, append([]byte(nil), encoded.Bytes()...))
	}
	return frames, nil
}

type appServerOutput struct {
	object map[string]any
	raw    []byte
}

func runCodexAppServer(ctx context.Context, req brokerRequest, binary, codexHome string, auth authentication, brokerDirectory string, toolchain hostGoToolchain) (result processResult, runErr error) {
	secrets := []string{auth.credential, auth.accountID}
	defer func() {
		result.stdout = []byte(redact(string(result.stdout), secrets))
		result.stderr = []byte(redact(string(result.stderr), secrets))
		if runErr != nil {
			runErr = errors.New(redact(runErr.Error(), secrets))
			dumpDiagnosticTrace(os.Getenv("MEGAPOWERS_BROKER_TRACE_DUMP"), "codex-app-server-error", result.stdout)
		}
	}()
	started := time.Now()
	proxySocket := filepath.Join(brokerDirectory, "codex-egress.sock")
	if auth.mode == authSubswapper {
		egress, err := startCredentialProxy("codex", auth.mode, auth.credential, auth.upstream, proxySocket)
		if err != nil {
			return processResult{rc: 125, duration: time.Since(started)}, errors.New("start Codex Subswapper bridge")
		}
		defer egress.close()
		// Only this per-actor capability crosses the app-server stdin boundary.
		// Subswapper's shared placeholder remains in the host-side proxy.
		auth.credential = egress.token
		auth.accountID = "megapowers-eval"
		secrets = append(secrets, egress.token)
	} else {
		egress, err := startRestrictedConnectProxy(proxySocket, map[string]bool{"chatgpt.com:443": true})
		if err != nil {
			return processResult{rc: 125, duration: time.Since(started)}, fmt.Errorf("start Codex egress proxy: %w", err)
		}
		defer egress.close()
	}
	sandboxArgs, err := codexAppServerSandboxArgs(req, binary, brokerDirectory, toolchain)
	if err != nil {
		return processResult{rc: 125, duration: time.Since(started)}, err
	}
	if auth.mode == authSubswapper {
		sandboxArgs = append(sandboxArgs,
			"-c", `model_provider="megapowers_subswapper"`,
			"-c", `model_providers.megapowers_subswapper.name="OpenAI"`,
			"-c", `model_providers.megapowers_subswapper.base_url="http://`+actorBridgeAddress+`/v1"`,
			"-c", `model_providers.megapowers_subswapper.wire_api="responses"`,
			"-c", `model_providers.megapowers_subswapper.requires_openai_auth=true`,
			"-c", `model_providers.megapowers_subswapper.supports_websockets=false`)
	}
	cmd := exec.Command("bwrap", sandboxArgs...)
	cmd.Dir = req.Project
	cmd.Env = actorEnvironment(req, map[string]string{
		"CODEX_HOME":  codexHome,
		"HTTPS_PROXY": "http://" + actorBridgeAddress,
		"HTTP_PROXY":  "http://" + actorBridgeAddress,
		"NO_PROXY":    "127.0.0.1,localhost",
	})
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return processResult{rc: 125, duration: time.Since(started)}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return processResult{rc: 125, duration: time.Since(started)}, err
	}
	var stderr limitedBuffer
	stderr.limit = 1 << 20
	cmd.Stderr = &stderr
	defer func() {
		if runErr != nil {
			runErr = fmt.Errorf("%w: Codex app-server stderr: %s", runErr, redact(strings.TrimSpace(string(stderr.Bytes())), []string{auth.credential}))
		}
	}()
	if err := cmd.Start(); err != nil {
		return processResult{rc: 125, duration: time.Since(started)}, err
	}
	done := make(chan error, 1)
	messages := make(chan appServerOutput)
	scanErrors := make(chan error, 1)
	scanDone := make(chan struct{})
	scanContext, cancelScan := context.WithCancel(ctx)
	go func() {
		defer close(scanDone)
		defer close(messages)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64<<10), 8<<20)
		for scanner.Scan() {
			raw := append([]byte(nil), scanner.Bytes()...)
			var object map[string]any
			if err := json.Unmarshal(raw, &object); err != nil {
				scanErrors <- errors.New("codex app-server emitted invalid JSON")
				return
			}
			select {
			case messages <- appServerOutput{object: object, raw: raw}:
			case <-scanContext.Done():
				return
			}
		}
		scanErrors <- scanner.Err()
	}()
	go func() {
		// Wait closes StdoutPipe, so drain final frames before reaping the process.
		<-scanDone
		done <- cmd.Wait()
	}()

	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		cancelScan()
		_ = stdin.Close()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
	}
	defer stop()

	writeMessage := func(message map[string]any) error {
		content, err := json.Marshal(message)
		if err != nil {
			return err
		}
		content = append(content, '\n')
		if _, err := stdin.Write(content); err != nil {
			return errors.New("write Codex app-server request")
		}
		return nil
	}
	var trace limitedBuffer
	trace.limit = traceLimit + 1
	handleServerRequest := func(object map[string]any) error {
		method, _ := object["method"].(string)
		id, hasID := object["id"]
		if !hasID {
			return nil
		}
		if codexAppServerRequestAllowed(method) {
			return writeMessage(map[string]any{"id": id, "error": map[string]any{"code": -32002, "message": "subscription refresh requires native re-authentication"}})
		}
		return writeMessage(map[string]any{"id": id, "error": map[string]any{"code": -32601, "message": "broker rejects app-server authority requests"}})
	}
	appendNotification := func(output appServerOutput) error {
		if _, ok := output.object["method"].(string); !ok {
			return nil
		}
		safe := []byte(redact(string(output.raw), []string{auth.credential}))
		if _, err := trace.Write(append(safe, '\n')); err != nil {
			return err
		}
		return nil
	}
	next := func() (appServerOutput, error) {
		for {
			select {
			case output, ok := <-messages:
				if !ok {
					select {
					case scanErr := <-scanErrors:
						if scanErr != nil {
							return appServerOutput{}, scanErr
						}
					default:
					}
					return appServerOutput{}, errors.New("codex app-server closed before turn completion")
				}
				if _, request := output.object["id"]; request {
					if _, hasMethod := output.object["method"]; hasMethod {
						if err := handleServerRequest(output.object); err != nil {
							return appServerOutput{}, err
						}
						continue
					}
				}
				if err := appendNotification(output); err != nil {
					return appServerOutput{}, err
				}
				return output, nil
			case <-ctx.Done():
				return appServerOutput{}, ctx.Err()
			}
		}
	}
	waitResponse := func(id int) (map[string]any, error) {
		for {
			output, err := next()
			if err != nil {
				return nil, err
			}
			responseID, hasID := output.object["id"]
			if !hasID {
				continue
			}
			if !jsonIDEquals(responseID, id) {
				return nil, errors.New("codex app-server returned an unexpected response id")
			}
			if _, failed := output.object["error"]; failed {
				detail, _ := output.object["error"].(map[string]any)
				message := redact(firstString(detail, "message"), secrets)
				if len(message) > 1024 {
					message = message[:1024]
				}
				return nil, fmt.Errorf("codex app-server rejected broker request %d: %s", id, message)
			}
			result, ok := output.object["result"].(map[string]any)
			if !ok {
				return nil, errors.New("codex app-server response is missing a result")
			}
			return result, nil
		}
	}
	sendRequest := func(id int, method string, params map[string]any) (map[string]any, error) {
		if err := writeMessage(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			return nil, err
		}
		return waitResponse(id)
	}

	if _, err := sendRequest(1, "initialize", map[string]any{
		"clientInfo":   map[string]any{"name": "megapowers-eval-broker", "title": "Megapowers evaluation broker", "version": "1"},
		"capabilities": map[string]any{"experimentalApi": true},
	}); err != nil {
		return processResult{stdout: trace.Bytes(), stderr: stderr.Bytes(), rc: 125, duration: time.Since(started)}, err
	}
	if err := writeMessage(map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
		return processResult{stdout: trace.Bytes(), stderr: stderr.Bytes(), rc: 125, duration: time.Since(started)}, err
	}
	login := map[string]any{"type": "chatgptAuthTokens", "accessToken": auth.credential, "chatgptAccountId": auth.accountID}
	if auth.planType != "" {
		login["chatgptPlanType"] = auth.planType
	}
	if _, err := sendRequest(2, "account/login/start", login); err != nil {
		return processResult{stdout: trace.Bytes(), stderr: stderr.Bytes(), rc: 125, duration: time.Since(started)}, err
	}
	// A non-ephemeral thread records its rollout under the disposable
	// CODEX_HOME (sessions/YYYY/MM/DD/rollout-*.jsonl). That rollout is the
	// only place the developer prompt, including the rendered skills catalog,
	// is observable; an ephemeral thread returns path null and writes nothing
	// (observed with codex-cli 0.152.0). The disposable home is deleted by
	// the runner, so nothing persists beyond the run.
	threadResult, err := sendRequest(3, "thread/start", map[string]any{
		"model": req.Model, "cwd": req.Project, "approvalPolicy": "never", "sandbox": "danger-full-access", "ephemeral": false,
		"config": map[string]any{"shell_environment_policy": map[string]any{"inherit": "none"}},
	})
	if err != nil {
		return processResult{stdout: trace.Bytes(), stderr: stderr.Bytes(), rc: 125, duration: time.Since(started)}, err
	}
	thread, _ := threadResult["thread"].(map[string]any)
	threadID := firstString(thread, "id")
	if threadID == "" {
		return processResult{stdout: trace.Bytes(), stderr: stderr.Bytes(), rc: 125, duration: time.Since(started)}, errors.New("codex app-server did not return a thread id")
	}
	// Omission selects the sandbox's local environment. Empty environments
	// disable native tools in current app-server builds. Follow-ups reuse this
	// exact root thread and share the request's one total context deadline.
	tasks := append([]string{req.Task}, req.FollowupTasks...)
	for index, task := range tasks {
		turnParams := map[string]any{"threadId": threadID, "input": []any{map[string]any{"type": "text", "text": task}}}
		if req.Effort != "" {
			turnParams["effort"] = req.Effort
		}
		if _, err := sendRequest(4+index, "turn/start", turnParams); err != nil {
			return processResult{stdout: trace.Bytes(), stderr: stderr.Bytes(), rc: 125, duration: time.Since(started)}, err
		}
		for {
			output, err := next()
			if err != nil {
				timedOut := errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
				rc := 125
				if timedOut {
					rc = 124
				}
				return processResult{stdout: trace.Bytes(), stderr: stderr.Bytes(), rc: rc, duration: time.Since(started), timedOut: timedOut}, err
			}
			if firstString(output.object, "method") != "turn/completed" {
				continue
			}
			params, _ := output.object["params"].(map[string]any)
			if firstString(params, "threadId") != threadID {
				continue
			}
			turn, _ := params["turn"].(map[string]any)
			if lowerString(turn["status"]) != "completed" {
				return processResult{stdout: trace.Bytes(), stderr: stderr.Bytes(), rc: 125, duration: time.Since(started)}, nil
			}
			break
		}
	}
	return processResult{stdout: trace.Bytes(), stderr: stderr.Bytes(), rc: 0, duration: time.Since(started)}, nil
}

func jsonIDEquals(value any, want int) bool {
	switch typed := value.(type) {
	case float64:
		return int(typed) == want
	case json.Number:
		parsed, err := typed.Int64()
		return err == nil && int(parsed) == want
	default:
		return false
	}
}

func codexAppServerRequestAllowed(method string) bool {
	return method == "account/chatgptAuthTokens/refresh"
}

func codexAppServerSandboxArgs(req brokerRequest, binary, proxyDirectory string, toolchain hostGoToolchain) ([]string, error) {
	insideBinary := "/opt/megapowers/codex"
	brokerBinary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	brokerBinary, err = filepath.EvalSymlinks(brokerBinary)
	if err != nil {
		return nil, err
	}
	insideBroker := "/opt/megapowers/broker"
	args := sandboxBase(false)
	args = append(args, "--dir", "/opt", "--dir", "/opt/megapowers", "--ro-bind", binary, insideBinary, "--ro-bind", brokerBinary, insideBroker)
	args, err = appendReceiptInstrumentation(args, brokerBinary, toolchain)
	if err != nil {
		return nil, err
	}
	args, err = appendCodexCompanion(args, binary)
	if err != nil {
		return nil, err
	}
	args = appendMount(args, req.ActorHome, false)
	args = appendMount(args, req.Project, false)
	args = append(args, "--dir", actorBrokerPath, "--ro-bind", proxyDirectory, actorBrokerPath)
	args = append(args, "--bind", filepath.Join(req.ActorHome, ".codex"), filepath.Join(req.ActorHome, ".codex"))
	if req.Arm == "treatment" {
		marketplace := filepath.Join(req.ActorHome, "codex-marketplace")
		args = append(args, "--ro-bind", marketplace, marketplace)
	}
	if req.PluginRepo != "" {
		args = appendMount(args, req.PluginRepo, true)
	}
	args = append(args, "--chdir", req.Project, "--", insideBroker, "--actor-bridge", filepath.Join(actorBrokerPath, "codex-egress.sock"), insideBinary, "app-server", "--stdio")
	return args, nil
}

func startProtectedEffectMonitor(project string) (*protectedEffectMonitor, error) {
	monitor := &protectedEffectMonitor{}
	targets := []struct {
		path, kind string
	}{
		{filepath.Join(project, ".tracker", "issue.json"), "tracker_comment"},
		{filepath.Join(project, ".tracker", "pull-request.json"), "pr_comment"},
	}
	for _, target := range targets {
		info, err := os.Lstat(target.path)
		if errors.Is(err, os.ErrNotExist) {
			parent := filepath.Dir(target.path)
			parentInfo, parentErr := os.Lstat(parent)
			if errors.Is(parentErr, os.ErrNotExist) {
				continue
			}
			if parentErr != nil || !parentInfo.IsDir() {
				monitor.close()
				if parentErr != nil {
					return nil, parentErr
				}
				return nil, fmt.Errorf("protected effect directory %q is not a directory", parent)
			}
			target.path = parent
			info = parentInfo
			err = nil
		}
		if err != nil {
			monitor.close()
			return nil, err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			monitor.close()
			return nil, fmt.Errorf("protected effect target %q is not regular", target.path)
		}
		fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
		if err != nil {
			monitor.close()
			return nil, err
		}
		mask := uint32(syscall.IN_MODIFY | syscall.IN_ATTRIB | syscall.IN_CLOSE_WRITE | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF | syscall.IN_CREATE | syscall.IN_MOVED_TO)
		if _, err := syscall.InotifyAddWatch(fd, target.path, mask); err != nil {
			_ = syscall.Close(fd)
			monitor.close()
			return nil, err
		}
		monitor.watches = append(monitor.watches, watchedEffect{fd: fd, kind: target.kind})
	}
	return monitor, nil
}

func (m *protectedEffectMonitor) events() []actorEvent {
	events := make([]actorEvent, 0, len(m.watches))
	buffer := make([]byte, 4096)
	for _, watch := range m.watches {
		observed := false
		for {
			count, err := syscall.Read(watch.fd, buffer)
			if count > 0 {
				observed = true
			}
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK || count == 0 {
				break
			}
			if err != nil {
				observed = true
				break
			}
		}
		if observed {
			events = append(events, actorEvent{Kind: watch.kind, RC: 0})
		}
	}
	return events
}

func (m *protectedEffectMonitor) close() {
	for _, watch := range m.watches {
		_ = syscall.Close(watch.fd)
	}
	m.watches = nil
}

func appendObservedEffects(events, observed []actorEvent, complete bool) []actorEvent {
	if len(observed) == 0 {
		return events
	}
	withoutCompletion := removeTraceComplete(events)
	for _, effect := range observed {
		found := false
		for _, event := range withoutCompletion {
			if event.Kind == effect.Kind {
				found = true
				break
			}
		}
		if !found {
			withoutCompletion = append(withoutCompletion, effect)
		}
	}
	if complete {
		withoutCompletion = append(withoutCompletion, actorEvent{Kind: "trace_complete", RC: 0})
	}
	for index := range withoutCompletion {
		withoutCompletion[index].Step = index + 1
	}
	return withoutCompletion
}

func runHarnessUtility(ctx context.Context, req brokerRequest, binary string, commandArgs, environment []string, writableHome bool) (processResult, error) {
	insideBinary := "/opt/megapowers/harness-utility"
	args := sandboxBase(false)
	args = append(args, "--dir", "/opt", "--dir", "/opt/megapowers", "--ro-bind", binary, insideBinary)
	if req.Harness == "codex" {
		var err error
		args, err = appendCodexCompanion(args, binary)
		if err != nil {
			return processResult{}, err
		}
	}
	args = appendMount(args, req.ActorHome, !writableHome)
	args = appendMount(args, req.Project, true)
	if req.PluginRepo != "" {
		args = appendMount(args, req.PluginRepo, true)
	}
	args = append(args, "--chdir", req.Project, "--", insideBinary)
	args = append(args, commandArgs...)
	return runProcess(ctx, "bwrap", args, req.Project, environment, nil)
}

func observedClaudeInventory(trace []byte, req brokerRequest) ([]string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(trace))
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	initCount := 0
	var inventory []string
	for scanner.Scan() {
		var object map[string]any
		if json.Unmarshal(bytes.TrimSpace(scanner.Bytes()), &object) != nil {
			continue
		}
		if lowerString(object["type"]) != "system" || lowerString(object["subtype"]) != "init" {
			continue
		}
		initCount++
		plugins, ok := object["plugins"].([]any)
		if !ok {
			return nil, errors.New("claude system/init does not contain a plugin inventory")
		}
		if collectionHasValues(object["plugin_errors"]) {
			return nil, errors.New("claude system/init reports plugin errors")
		}
		current, err := validateClaudeInventory(plugins, req)
		if err != nil {
			return nil, err
		}
		if initCount == 1 {
			inventory = current
			continue
		}
		if len(current) != len(inventory) || (len(current) == 1 && current[0] != inventory[0]) {
			return nil, errors.New("claude repeated system/init reports inconsistent plugin inventories")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Claude init event: %w", err)
	}
	if initCount == 0 {
		return nil, errors.New("claude trace does not contain a system/init event")
	}
	return inventory, nil
}

func validateClaudeInventory(plugins []any, req brokerRequest) ([]string, error) {
	if req.Arm == "control" {
		if len(plugins) != 0 {
			return nil, errors.New("claude control loaded a plugin")
		}
		return []string{}, nil
	}
	if len(plugins) != 1 {
		return nil, fmt.Errorf("claude treatment loaded %d plugins; require exactly one", len(plugins))
	}
	plugin, ok := plugins[0].(map[string]any)
	if !ok || firstString(plugin, "name") != "megapowers" {
		return nil, errors.New("claude treatment did not load the Megapowers candidate")
	}
	pluginPath := firstString(plugin, "path")
	if pluginPath == "" || filepath.Clean(pluginPath) != req.PluginRepo {
		return nil, errors.New("claude loaded Megapowers from an unexpected path")
	}
	resolved, err := filepath.EvalSymlinks(pluginPath)
	if err != nil || filepath.Clean(resolved) != req.PluginRepo {
		return nil, errors.New("claude loaded Megapowers through a non-canonical path")
	}
	return []string{"megapowers"}, nil
}

func collectionHasValues(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	case string:
		return typed != ""
	default:
		return true
	}
}

func writeClaudeSettings(req brokerRequest) (string, error) {
	config := filepath.Join(req.ActorHome, ".claude")
	if err := os.Mkdir(config, 0o700); err != nil {
		return "", err
	}
	settings := map[string]any{
		"permissions": map[string]any{
			// Explicit allows prevent non-interactive permission asks from
			// masquerading as actor behavior; Bubblewrap remains the
			// authoritative filesystem boundary for every write.
			"defaultMode": "acceptEdits",
			"allow":       []string{"Agent", "Task", "Skill", "Read", "Glob", "Grep", "Bash", "Edit", "Write", "MultiEdit", "NotebookEdit"},
			"deny":        []string{"WebFetch", "WebSearch", "Read(~/.claude/.credentials.json)"},
		},
		"sandbox": map[string]any{
			"enabled":                  true,
			"failIfUnavailable":        true,
			"autoAllowBashIfSandboxed": true,
			"allowUnsandboxedCommands": false,
			"credentials": map[string]any{
				"files":   []any{map[string]any{"path": "~/.claude/.credentials.json", "mode": "deny"}},
				"envVars": []any{map[string]any{"name": "CLAUDE_CODE_OAUTH_TOKEN", "mode": "deny"}, map[string]any{"name": "ANTHROPIC_API_KEY", "mode": "deny"}},
			},
			"network": map[string]any{
				"allowedDomains": []string{},
				// Linux seccomp cannot allow one socket path. The outer
				// Bubblewrap boundary exposes only this run's broker sockets.
				"allowAllUnixSockets": true,
			},
		},
		"disableAllHooks": false,
	}
	content, err := json.Marshal(settings)
	if err != nil {
		return "", err
	}
	path := filepath.Join(config, "broker-settings.json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o400)
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(append(content, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return path, nil
}

// prepareCodexHome stages the disposable CODEX_HOME and returns its path, the
// verified installed plugin cache path (empty for control), and the inventory.
func prepareCodexHome(ctx context.Context, req brokerRequest, binary string) (string, string, []string, error) {
	home := filepath.Join(req.ActorHome, ".codex")
	if err := os.Mkdir(home, 0o700); err != nil {
		return "", "", nil, err
	}
	commands := make([][]string, 0, 3)
	stagedPlugin := ""
	if req.Arm == "treatment" {
		marketplace := filepath.Join(req.ActorHome, "codex-marketplace")
		pluginDestination := filepath.Join(marketplace, "plugins", "megapowers")
		stagedPlugin = pluginDestination
		if err := copyRegularTree(req.PluginRepo, pluginDestination); err != nil {
			return "", "", nil, fmt.Errorf("stage Codex plugin: %w", err)
		}
		manifestDirectory := filepath.Join(marketplace, ".agents", "plugins")
		if err := os.MkdirAll(manifestDirectory, 0o700); err != nil {
			return "", "", nil, err
		}
		manifest := map[string]any{
			"name": "megapowers-eval",
			"plugins": []any{map[string]any{
				"name":        "megapowers",
				"description": "Megapowers evaluation candidate",
				"source":      map[string]any{"source": "local", "path": "./plugins/megapowers"},
			}},
		}
		content, err := json.Marshal(manifest)
		if err != nil {
			return "", "", nil, err
		}
		if err := os.WriteFile(filepath.Join(manifestDirectory, "marketplace.json"), append(content, '\n'), 0o400); err != nil {
			return "", "", nil, err
		}
		commands = append(commands, []string{"plugin", "marketplace", "add", marketplace, "--json"}, []string{"plugin", "add", "megapowers@megapowers-eval", "--json"})
	}
	environment := minimalEnvironment(req.ActorHome)
	environment = append(environment, "CODEX_HOME="+home)
	commands = append(commands, []string{"plugin", "list", "--json"})
	var listOutput []byte
	var addOutput []byte
	for _, command := range commands {
		result, runErr := runHarnessUtility(ctx, req, binary, command, environment, true)
		if runErr != nil {
			return "", "", nil, fmt.Errorf("stage Codex plugin registration: %w", runErr)
		}
		if result.rc != 0 {
			return "", "", nil, fmt.Errorf("stage Codex plugin registration: exit %d: %s", result.rc, strings.TrimSpace(string(result.stderr)))
		}
		if len(command) >= 2 && command[0] == "plugin" && command[1] == "add" {
			addOutput = result.stdout
		}
		if len(command) >= 2 && command[0] == "plugin" && command[1] == "list" {
			listOutput = result.stdout
		}
	}
	installedPath := ""
	if req.Arm == "treatment" {
		var err error
		installedPath, err = codexInstalledPath(addOutput, home)
		if err != nil {
			return "", "", nil, err
		}
		if err := verifyRegularTreeBytes(stagedPlugin, installedPath); err != nil {
			return "", "", nil, fmt.Errorf("verify Codex installed plugin cache: %w", err)
		}
	}
	inventory, err := observedCodexInventory(listOutput, req.Arm)
	if err != nil {
		return "", "", nil, err
	}
	return home, installedPath, inventory, nil
}

func codexInstalledPath(content []byte, home string) (string, error) {
	var result struct {
		PluginID      string `json:"pluginId"`
		InstalledPath string `json:"installedPath"`
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&result); err != nil {
		return "", fmt.Errorf("decode Codex plugin install result: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return "", errors.New("codex plugin install result has trailing data")
	}
	if result.PluginID != "megapowers@megapowers-eval" || result.InstalledPath == "" {
		return "", errors.New("codex plugin install result does not identify the Megapowers cache")
	}
	installedPath, err := canonicalDirectory(result.InstalledPath)
	if err != nil {
		return "", fmt.Errorf("validate Codex installedPath: %w", err)
	}
	cacheRoot := filepath.Join(home, "plugins", "cache")
	if !strings.HasPrefix(installedPath, cacheRoot+string(filepath.Separator)) {
		return "", errors.New("codex installedPath is outside the disposable plugin cache")
	}
	return installedPath, nil
}

func verifyRegularTreeBytes(source, installed string) error {
	sourceFiles, err := regularTreeDigests(source)
	if err != nil {
		return err
	}
	installedFiles, err := regularTreeDigests(installed)
	if err != nil {
		return err
	}
	if len(sourceFiles) != len(installedFiles) {
		return fmt.Errorf("file count differs: source=%d installed=%d", len(sourceFiles), len(installedFiles))
	}
	for path, sourceEvidence := range sourceFiles {
		installedEvidence, ok := installedFiles[path]
		if !ok || installedEvidence.digest != sourceEvidence.digest {
			return fmt.Errorf("installed bytes differ at %q", path)
		}
		if installedEvidence.executable != sourceEvidence.executable {
			return fmt.Errorf("installed executable mode differs at %q", path)
		}
	}
	return nil
}

type regularFileEvidence struct {
	digest     [sha256.Size]byte
	executable bool
}

func regularTreeDigests(root string) (map[string]regularFileEvidence, error) {
	digests := make(map[string]regularFileEvidence)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.Name() == ".codex-marketplace-install.json" || entry.Name() == ".in_use" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("plugin cache path %q is not regular", path)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hasher := sha256.New()
		_, copyErr := io.Copy(hasher, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		var digest [sha256.Size]byte
		copy(digest[:], hasher.Sum(nil))
		digests[filepath.ToSlash(relative)] = regularFileEvidence{digest: digest, executable: info.Mode().Perm()&0o111 != 0}
		return nil
	})
	return digests, err
}

func observedCodexInventory(content []byte, arm string) ([]string, error) {
	var inventory struct {
		Installed []struct {
			PluginID  string `json:"pluginId"`
			Installed bool   `json:"installed"`
			Enabled   bool   `json:"enabled"`
		} `json:"installed"`
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&inventory); err != nil {
		return nil, fmt.Errorf("decode Codex plugin inventory: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("codex plugin inventory has trailing data")
	}
	if arm == "control" {
		if len(inventory.Installed) != 0 {
			return nil, errors.New("codex control has an installed plugin")
		}
		return []string{}, nil
	}
	matches := 0
	for _, plugin := range inventory.Installed {
		if plugin.PluginID == "megapowers@megapowers-eval" && plugin.Installed && plugin.Enabled {
			matches++
		}
	}
	if matches != 1 || len(inventory.Installed) != 1 {
		return nil, errors.New("codex plugin inventory is not exactly one enabled Megapowers candidate")
	}
	return []string{"megapowers"}, nil
}

// claudeSkillsCatalog reads the first Claude system/init event. Claude Code
// 2.1.257 lists every loaded skill in init "skills" (plugin skills as
// "megapowers:<name>"); older builds expose the same names only through
// "slash_commands".
func claudeSkillsCatalog(trace []byte) skillsCatalog {
	scanner := bufio.NewScanner(bytes.NewReader(trace))
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	for scanner.Scan() {
		var object map[string]any
		if json.Unmarshal(bytes.TrimSpace(scanner.Bytes()), &object) != nil {
			continue
		}
		if lowerString(object["type"]) != "system" || lowerString(object["subtype"]) != "init" {
			continue
		}
		for _, field := range []struct{ key, source string }{{"skills", catalogSourceClaudeInit}, {"slash_commands", catalogSourceClaudeSlashCommands}} {
			listed, ok := object[field.key].([]any)
			if !ok {
				continue
			}
			skills := make([]string, 0)
			for _, entry := range listed {
				name, _ := entry.(string)
				if strings.HasPrefix(name, "megapowers:") {
					skills = append(skills, strings.TrimPrefix(name, "megapowers:"))
				}
			}
			sort.Strings(skills)
			return skillsCatalog{Rendered: len(skills) > 0, Skills: skills, Source: field.source}
		}
		break
	}
	return skillsCatalog{Rendered: false, Skills: []string{}, Source: catalogSourceUnavailable}
}

// codexSkillsCatalog scans the rollouts recorded under the disposable
// CODEX_HOME for the developer message that carries <skills_instructions>.
// Codex renders that block as "### Skill roots" (- `rN` = `<dir>`) followed
// by "### Available skills" (- <name>: <description> (file: rN/<...>/SKILL.md)).
// A skill counts as Megapowers when its expanded file path lies inside the
// verified installed plugin cache. It returns the catalog and the first block
// found for diagnostics.
func codexSkillsCatalog(codexHome, installedPlugin string) (skillsCatalog, string, error) {
	catalog := skillsCatalog{Rendered: false, Skills: []string{}, Source: catalogSourceCodexRollout}
	sessions := filepath.Join(codexHome, "sessions")
	if _, err := os.Lstat(sessions); errors.Is(err, os.ErrNotExist) {
		return catalog, "", nil
	}
	rollouts := make([]string, 0)
	err := filepath.WalkDir(sessions, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".jsonl") {
			rollouts = append(rollouts, path)
		}
		return nil
	})
	if err != nil {
		return catalog, "", err
	}
	sort.Strings(rollouts)
	firstBlock := ""
	seen := make(map[string]bool)
	for _, rollout := range rollouts {
		blocks, err := rolloutSkillsInstructions(rollout)
		if err != nil {
			return catalog, "", err
		}
		for _, block := range blocks {
			if firstBlock == "" {
				firstBlock = block
			}
			for _, name := range megapowersSkillsInBlock(block, installedPlugin) {
				seen[name] = true
			}
		}
	}
	for name := range seen {
		catalog.Skills = append(catalog.Skills, name)
	}
	sort.Strings(catalog.Skills)
	catalog.Rendered = len(catalog.Skills) > 0
	return catalog, firstBlock, nil
}

func rolloutSkillsInstructions(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	blocks := make([]string, 0)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Cheap prefilter before JSON decoding; the tag may be written
		// literally or with JSON-escaped angle brackets.
		if !bytes.Contains(line, []byte("skills_instructions")) {
			continue
		}
		var record struct {
			Type    string `json:"type"`
			Payload struct {
				Type    string `json:"type"`
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &record) != nil || record.Type != "response_item" || record.Payload.Type != "message" || record.Payload.Role != "developer" {
			continue
		}
		for _, content := range record.Payload.Content {
			if content.Type != "input_text" {
				continue
			}
			start := strings.Index(content.Text, skillsInstructionsOpen)
			if start < 0 {
				continue
			}
			block := content.Text[start:]
			if end := strings.Index(block, skillsInstructionsClose); end >= 0 {
				block = block[:end+len(skillsInstructionsClose)]
			}
			blocks = append(blocks, block)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read rollout %s: %w", filepath.Base(path), err)
	}
	return blocks, nil
}

var (
	skillRootPattern = regexp.MustCompile("^- `([A-Za-z0-9_]+)` = `(.+)`$")
	// Codex 0.152+ namespaces plugin skills as "megapowers:<name>"; older
	// builds list the bare name. The name group therefore admits one colon.
	skillEntryPattern = regexp.MustCompile(`^- ([^\s:]+(?::[^\s:]+)?): .*\(file: ([^)]+)\)\s*$`)
)

func megapowersSkillsInBlock(block, installedPlugin string) []string {
	if installedPlugin == "" {
		return nil
	}
	roots := make(map[string]string)
	names := make([]string, 0)
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimRight(line, "\r")
		if match := skillRootPattern.FindStringSubmatch(line); match != nil {
			roots[match[1]] = match[2]
			continue
		}
		match := skillEntryPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		file := match[2]
		if root, rest, found := strings.Cut(file, "/"); found {
			if directory, known := roots[root]; known {
				file = filepath.Join(directory, rest)
			}
		}
		if pathsOverlap(file, installedPlugin) && file != installedPlugin {
			name := match[1]
			if _, bare, namespaced := strings.Cut(name, ":"); namespaced {
				name = bare
			}
			names = append(names, name)
		}
	}
	return names
}

// skillsCatalogTraceLine renders the detection result as one trailing trace
// line. It carries no "type", "item", or tool-name keys, so trace
// normalization can never turn it into actor evidence.
func skillsCatalogTraceLine(catalog skillsCatalog, block string) []byte {
	content, err := json.Marshal(map[string]any{"method": "broker/skillsCatalog", "params": map[string]any{"catalog": catalog, "block": block}})
	if err != nil {
		return nil
	}
	return append(content, '\n')
}

func copyRegularTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(destination, 0o700)
		}
		target := filepath.Join(destination, relative)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.Mkdir(target, 0o700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("plugin path %q is not regular", path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o400)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o500
		}
		return os.WriteFile(target, content, mode)
	})
}

func actorEnvironment(req brokerRequest, additions map[string]string) []string {
	values := map[string]string{
		"HOME":                req.ActorHome,
		"USERPROFILE":         req.ActorHome,
		"XDG_CONFIG_HOME":     filepath.Join(req.ActorHome, ".config"),
		"XDG_CACHE_HOME":      filepath.Join(req.ActorHome, ".cache"),
		"XDG_DATA_HOME":       filepath.Join(req.ActorHome, ".local", "share"),
		"PATH":                "/opt/megapowers-receipt/bin:/opt/megapowers-runtime/go/bin:/usr/bin:/bin",
		"TMPDIR":              "/tmp",
		"LANG":                "C.UTF-8",
		"LC_ALL":              "C.UTF-8",
		"NO_COLOR":            "1",
		"GIT_CONFIG_NOSYSTEM": "1",
	}
	for key, value := range additions {
		values[key] = value
	}
	return environmentList(values)
}

func minimalEnvironment(home string) []string {
	return environmentList(map[string]string{
		"HOME":     home,
		"PATH":     "/opt/megapowers-runtime/go/bin:/usr/bin:/bin",
		"LANG":     "C.UTF-8",
		"LC_ALL":   "C.UTF-8",
		"NO_COLOR": "1",
	})
}

func environmentList(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func actorSandboxArgs(req brokerRequest, binary, insideBinary string, commandArgs []string, proxySocket string, toolchain hostGoToolchain) ([]string, error) {
	brokerBinary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	brokerBinary, err = filepath.EvalSymlinks(brokerBinary)
	if err != nil {
		return nil, err
	}
	insideBroker := "/opt/megapowers/broker"
	args := sandboxBase(false)
	args = append(args, "--dir", "/opt", "--dir", "/opt/megapowers", "--ro-bind", binary, insideBinary, "--ro-bind", brokerBinary, insideBroker)
	args, err = appendReceiptInstrumentation(args, brokerBinary, toolchain)
	if err != nil {
		return nil, err
	}
	if req.Harness == "codex" {
		args, err = appendCodexCompanion(args, binary)
		if err != nil {
			return nil, err
		}
	}
	args = appendMount(args, req.ActorHome, false)
	args = appendMount(args, req.Project, false)
	args = append(args, "--dir", actorBrokerPath, "--ro-bind", filepath.Dir(proxySocket), actorBrokerPath)
	if req.Harness == "codex" {
		args = append(args, "--ro-bind", filepath.Join(req.ActorHome, ".codex"), filepath.Join(req.ActorHome, ".codex"))
		if req.Arm == "treatment" {
			marketplace := filepath.Join(req.ActorHome, "codex-marketplace")
			args = append(args, "--ro-bind", marketplace, marketplace)
		}
	}
	if req.PluginRepo != "" {
		args = appendMount(args, req.PluginRepo, true)
	}
	args = append(args, "--chdir", req.Project, "--", insideBroker, "--actor-bridge", filepath.Join(actorBrokerPath, filepath.Base(proxySocket)), insideBinary)
	args = append(args, commandArgs...)
	return args, nil
}

func appendCodexCompanion(args []string, binary string) ([]string, error) {
	path := filepath.Join(filepath.Dir(binary), "codex-code-mode-host")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return args, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return nil, errors.New("codex code-mode companion must be a regular executable")
	}
	// Mount only the companion from the selected CLI distribution, not its
	// installation directory or the operator's configuration.
	return append(args, "--ro-bind", path, "/opt/megapowers/codex-code-mode-host"), nil
}

type hostGoToolchain struct {
	binary string
	root   string
}

func resolveHostGoToolchain(req brokerRequest) (hostGoToolchain, error) {
	goBinary, err := exec.LookPath("go")
	if err != nil || !filepath.IsAbs(goBinary) {
		return hostGoToolchain{}, errors.New("active host Go executable is unavailable")
	}
	goBinary, err = filepath.EvalSymlinks(filepath.Clean(goBinary))
	if err != nil {
		return hostGoToolchain{}, errors.New("resolve active host Go executable")
	}
	goInfo, err := os.Lstat(goBinary)
	if err != nil || !goInfo.Mode().IsRegular() || goInfo.Mode().Perm()&0o111 == 0 {
		return hostGoToolchain{}, errors.New("active host Go executable must be a regular executable")
	}

	queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(queryCtx, goBinary, "env", "GOROOT")
	command.Env = []string{"GOENV=off", "GOTOOLCHAIN=local", "HOME=/tmp", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "PATH=/usr/bin:/bin"}
	var stdout, stderr limitedBuffer
	stdout.limit = 4097
	stderr.limit = 4097
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if queryCtx.Err() != nil {
			return hostGoToolchain{}, errors.New("active host Go root query timed out")
		}
		return hostGoToolchain{}, errors.New("active host Go root query failed")
	}
	rootOutput := stdout.Bytes()
	if len(rootOutput) == 0 || len(rootOutput) > 4096 || bytes.ContainsAny(rootOutput, "\x00\r") {
		return hostGoToolchain{}, errors.New("active host Go root query returned an invalid path")
	}
	goRoot := strings.TrimSuffix(string(rootOutput), "\n")
	if strings.Contains(goRoot, "\n") || !filepath.IsAbs(goRoot) {
		return hostGoToolchain{}, errors.New("active host Go root query returned an invalid path")
	}
	goRoot, err = filepath.EvalSymlinks(filepath.Clean(goRoot))
	if err != nil {
		return hostGoToolchain{}, errors.New("resolve active host Go root")
	}
	rootInfo, err := os.Lstat(goRoot)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return hostGoToolchain{}, errors.New("active host Go root must be a real directory")
	}
	if filepath.Dir(goRoot) == string(filepath.Separator) {
		return hostGoToolchain{}, errors.New("active host Go root is too broad")
	}
	hostHome, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(hostHome) {
		return hostGoToolchain{}, errors.New("host home is unavailable for Go root validation")
	}
	hostHome, err = filepath.EvalSymlinks(filepath.Clean(hostHome))
	if err != nil {
		return hostGoToolchain{}, errors.New("resolve host home for Go root validation")
	}
	if pathContains(goRoot, hostHome) {
		return hostGoToolchain{}, errors.New("active host Go root contains the host home")
	}
	for _, visible := range append([]string{req.Project, req.ActorHome, req.PluginRepo}, req.TaskWriteRoots...) {
		if visible != "" && pathsOverlap(goRoot, visible) {
			return hostGoToolchain{}, errors.New("active host Go root overlaps actor-controlled state")
		}
	}
	rootBinary, err := filepath.EvalSymlinks(filepath.Join(goRoot, "bin", "go"))
	if err != nil {
		return hostGoToolchain{}, errors.New("active host Go root has no canonical Go executable")
	}
	rootGoInfo, err := os.Lstat(rootBinary)
	if err != nil || !rootGoInfo.Mode().IsRegular() || rootGoInfo.Mode().Perm()&0o111 == 0 || !os.SameFile(goInfo, rootGoInfo) {
		return hostGoToolchain{}, errors.New("active host Go executable does not match its root")
	}
	return hostGoToolchain{binary: goBinary, root: goRoot}, nil
}

func appendGoToolchain(args []string, toolchain hostGoToolchain) []string {
	return append(args, "--ro-bind", toolchain.root, "/opt/megapowers-runtime/go")
}

func appendReceiptInstrumentation(args []string, brokerBinary string, toolchain hostGoToolchain) ([]string, error) {
	bash, err := filepath.EvalSymlinks("/usr/bin/bash")
	if err != nil {
		return nil, errors.New("receipt instrumentation requires /usr/bin/bash")
	}
	sh, err := filepath.EvalSymlinks("/usr/bin/sh")
	if err != nil {
		return nil, errors.New("receipt instrumentation requires /usr/bin/sh")
	}
	args = appendGoToolchain(args, toolchain)
	args = append(args,
		"--dir", "/opt/megapowers-receipt",
		"--dir", "/opt/megapowers-receipt/bin",
		"--dir", "/opt/megapowers-receipt/real",
		"--ro-bind", bash, "/opt/megapowers-receipt/real/bash",
		"--ro-bind", sh, "/opt/megapowers-receipt/real/sh",
		"--ro-bind", toolchain.binary, "/opt/megapowers-receipt/real/go",
	)
	names := []string{"go", "bash", "sh", "dash"}
	for _, optional := range []struct {
		path  string
		names []string
	}{
		{"/usr/bin/python3", []string{"python3"}},
		{"/usr/bin/pytest", []string{"pytest", "py.test"}},
		{"/usr/bin/npm", []string{"npm"}},
		{"/usr/bin/cargo", []string{"cargo"}},
	} {
		if info, statErr := os.Stat(optional.path); statErr == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			names = append(names, optional.names...)
		}
	}
	for _, name := range names {
		args = append(args, "--ro-bind", brokerBinary, filepath.Join("/opt/megapowers-receipt/bin", name))
	}
	// Absolute Bash and /bin/sh shebangs still pass through the wrapper. The
	// original interpreters remain read-only at separate delegate paths.
	args = append(args, "--ro-bind", brokerBinary, "/usr/bin/bash")
	if filepath.Base(sh) == "dash" {
		args = append(args, "--ro-bind", brokerBinary, "/usr/bin/dash")
	}
	// Claude's login-shell snapshot can put the mounted Go runtime before the
	// wrapper directory. Overlay its absolute launcher so either PATH order is
	// instrumented; the original launcher remains read-only at its separate path.
	args = append(args, "--ro-bind", brokerBinary, "/opt/megapowers-runtime/go/bin/go")
	return args, nil
}

func sandboxBase(network bool) []string {
	args := []string{
		"--die-with-parent", "--new-session", "--unshare-ipc", "--unshare-pid", "--unshare-uts", "--unshare-cgroup-try", "--cap-drop", "ALL",
		"--tmpfs", "/", "--proc", "/proc", "--dev", "/dev",
		"--dir", "/usr", "--ro-bind", "/usr", "/usr",
		"--tmpfs", "/usr/local",
		"--symlink", "usr/bin", "/bin", "--symlink", "usr/lib", "/lib", "--symlink", "usr/lib64", "/lib64", "--symlink", "usr/sbin", "/sbin",
		"--dir", "/etc", "--ro-bind-try", "/etc/ssl", "/etc/ssl", "--ro-bind-try", "/etc/ca-certificates", "/etc/ca-certificates",
		"--ro-bind-try", "/etc/resolv.conf", "/etc/resolv.conf", "--ro-bind-try", "/etc/hosts", "/etc/hosts", "--ro-bind-try", "/etc/nsswitch.conf", "/etc/nsswitch.conf", "--ro-bind-try", "/etc/localtime", "/etc/localtime",
		"--dir", "/opt", "--dir", "/opt/megapowers-runtime", "--dir", "/opt/megapowers-runtime/go",
		"--dir", "/tmp", "--tmpfs", "/tmp", "--dir", "/run", "--tmpfs", "/run",
	}
	if !network {
		args = append(args, "--unshare-net")
	}
	return args
}

func appendMount(args []string, path string, readOnly bool) []string {
	for _, parent := range pathParents(path) {
		args = append(args, "--dir", parent)
	}
	operation := "--bind"
	if readOnly {
		operation = "--ro-bind"
	}
	return append(args, operation, path, path)
}

func pathParents(path string) []string {
	clean := filepath.Clean(path)
	parts := strings.Split(strings.TrimPrefix(clean, string(filepath.Separator)), string(filepath.Separator))
	parents := make([]string, 0, len(parts))
	current := string(filepath.Separator)
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		parents = append(parents, current)
	}
	return parents
}

func runOracle(ctx context.Context, req brokerRequest, toolchain hostGoToolchain) (processResult, error) {
	args := sandboxBase(false)
	args = appendGoToolchain(args, toolchain)
	emptyHome, err := os.MkdirTemp(req.ActorHome, ".oracle-home-")
	if err != nil {
		return processResult{}, err
	}
	defer os.RemoveAll(emptyHome)
	if err := os.Chmod(emptyHome, 0o700); err != nil {
		return processResult{}, err
	}
	args = appendMount(args, emptyHome, false)
	args = appendMount(args, req.Project, false)
	if req.PluginRepo != "" {
		args = appendMount(args, req.PluginRepo, true)
	}
	args = append(args, "--chdir", req.Project, "--")
	args = append(args, req.OracleCommand...)
	result, runErr := runProcess(ctx, "bwrap", args, req.Project, minimalEnvironment(emptyHome), nil)
	if runErr != nil && !result.timedOut {
		return processResult{}, fmt.Errorf("run isolated oracle: %w", runErr)
	}
	if result.timedOut {
		result.rc = 124
	}
	return result, nil
}

func runProcess(ctx context.Context, binary string, args []string, directory string, environment []string, stdin []byte) (processResult, error) {
	started := time.Now()
	cmd := exec.Command(binary, args...)
	cmd.Dir = directory
	cmd.Env = environment
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr limitedBuffer
	stdout.limit = traceLimit + 1
	stderr.limit = 1 << 20
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return processResult{rc: 125, duration: time.Since(started)}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return processResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), rc: exitCode(err), duration: time.Since(started)}, operationalError(err)
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
		return processResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), rc: 124, duration: time.Since(started), timedOut: true}, ctx.Err()
	}
}

type claudeFollowupGate struct {
	mu                 sync.Mutex
	capture            *limitedBuffer
	partial            []byte
	total              int
	segments           int
	awaitingRoot       bool
	rootSeen           bool
	readySignaled      bool
	failed             bool
	activeTasks        map[string]bool
	seenTasks          map[string]bool
	forwardPermissions int
	openForwarded      int
	ready              chan struct{}
	failure            chan struct{}
}

func newClaudeFollowupGate(capture *limitedBuffer) *claudeFollowupGate {
	return &claudeFollowupGate{
		capture:     capture,
		activeTasks: make(map[string]bool),
		seenTasks:   make(map[string]bool),
		ready:       make(chan struct{}, 1),
		failure:     make(chan struct{}),
	}
}

func (g *claudeFollowupGate) Write(content []byte) (int, error) {
	original := len(content)
	_, _ = g.capture.Write(content)
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.failed {
		return original, nil
	}
	g.total += len(content)
	if g.total > traceLimit {
		g.failLocked()
		return original, nil
	}
	g.partial = append(g.partial, content...)
	for {
		end := bytes.IndexByte(g.partial, '\n')
		if end < 0 {
			if len(g.partial) > 8<<20 {
				g.failLocked()
			}
			return original, nil
		}
		line := bytes.TrimSpace(g.partial[:end])
		g.partial = g.partial[end+1:]
		if len(line) == 0 {
			continue
		}
		var object map[string]any
		if json.Unmarshal(line, &object) != nil {
			g.failLocked()
			return original, nil
		}
		g.observeLocked(object)
		if g.failed {
			return original, nil
		}
	}
}

func (g *claudeFollowupGate) observeLocked(object map[string]any) {
	typeName := lowerString(object["type"])
	subtype := lowerString(object["subtype"])
	if typeName == "system" {
		switch subtype {
		case "init":
			if g.segments == 0 {
				g.segments = 1
			} else if g.forwardPermissions > 0 {
				g.forwardPermissions--
				g.openForwarded++
			}
		case "task_started":
			if lowerString(object["task_type"]) != "local_agent" {
				break
			}
			taskID := firstString(object, "task_id")
			if taskID == "" || g.seenTasks[taskID] {
				g.failLocked()
				return
			}
			g.seenTasks[taskID] = true
			g.activeTasks[taskID] = true
		case "task_notification":
			taskID := firstString(object, "task_id")
			if !g.activeTasks[taskID] {
				if taskID != "" && g.seenTasks[taskID] {
					g.failLocked()
				}
				break
			}
			delete(g.activeTasks, taskID)
			if status, ok := object["status"].(string); ok && strings.EqualFold(status, "completed") {
				g.forwardPermissions++
			}
		}
	}
	if typeName == "result" {
		_, hasResult := object["result"].(string)
		isError, hasStatus := object["is_error"].(bool)
		wellFormed := g.segments > 0 && hasResult && hasStatus && !isError
		origin, originPresent := object["origin"]
		if originPresent {
			originObject, ok := origin.(map[string]any)
			if !ok || lowerString(originObject["kind"]) != "task-notification" || !wellFormed || g.openForwarded == 0 {
				g.failLocked()
				return
			}
			g.openForwarded--
		} else if !wellFormed {
			g.failLocked()
			return
		} else if g.awaitingRoot {
			g.awaitingRoot = false
			g.rootSeen = true
		} else {
			// Forwarded segments can emit additional origin-less inner
			// results. Only the first successful origin-less result after a
			// user frame can satisfy that frame's root gate.
			if !g.rootSeen && g.openForwarded == 0 {
				g.failLocked()
				return
			}
		}
	}
	g.signalReadyLocked()
}

func (g *claudeFollowupGate) beginTurn() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.failed || g.awaitingRoot || g.rootSeen {
		return false
	}
	g.awaitingRoot = true
	return true
}

func (g *claudeFollowupGate) signalReadyLocked() {
	// A completed foreground Agent can emit a task notification without a
	// forwarded segment. The permission authorizes a later init; it is not
	// outstanding work until that init opens a segment.
	ready := g.rootSeen && len(g.activeTasks) == 0 && g.openForwarded == 0
	if !ready {
		g.readySignaled = false
		return
	}
	if g.readySignaled {
		return
	}
	g.readySignaled = true
	select {
	case g.ready <- struct{}{}:
	default:
	}
}

func (g *claudeFollowupGate) claimReady() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.failed || !g.rootSeen || len(g.activeTasks) != 0 || g.openForwarded != 0 {
		return false
	}
	g.rootSeen = false
	g.readySignaled = false
	g.forwardPermissions = 0
	return true
}

func (g *claudeFollowupGate) failLocked() {
	if g.failed {
		return
	}
	g.failed = true
	close(g.failure)
}

func runClaudeStreamingProcess(ctx context.Context, binary string, args []string, directory string, environment []string, frames [][]byte) (processResult, error) {
	started := time.Now()
	if len(frames) == 0 {
		return processResult{rc: 125}, errors.New("claude streaming input has no frames")
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir = directory
	cmd.Env = environment
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return processResult{rc: 125, duration: time.Since(started)}, err
	}
	var stdout, stderr limitedBuffer
	stdout.limit = traceLimit + 1
	stderr.limit = 1 << 20
	gate := newClaudeFollowupGate(&stdout)
	cmd.Stdout = gate
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return processResult{rc: 125, duration: time.Since(started)}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	result := func(waitErr error) (processResult, error) {
		return processResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), rc: exitCode(waitErr), duration: time.Since(started)}, operationalError(waitErr)
	}
	stop := func() error {
		_ = stdin.Close()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		select {
		case waitErr := <-done:
			return waitErr
		case <-time.After(500 * time.Millisecond):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			return <-done
		}
	}
	for index, frame := range frames {
		if index > 0 {
			for {
				select {
				case <-gate.ready:
					if gate.claimReady() {
						goto writeFrame
					}
				case <-gate.failure:
					_ = stop()
					return processResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), rc: 125, duration: time.Since(started)}, nil
				case waitErr := <-done:
					_ = stdin.Close()
					return result(waitErr)
				case <-ctx.Done():
					_ = stop()
					return processResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), rc: 124, duration: time.Since(started), timedOut: true}, ctx.Err()
				}
			}
		}
	writeFrame:
		if !gate.beginTurn() {
			_ = stop()
			return processResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), rc: 125, duration: time.Since(started)}, nil
		}
		writeDone := make(chan error, 1)
		go func(content []byte) {
			_, writeErr := io.Copy(stdin, bytes.NewReader(content))
			writeDone <- writeErr
		}(frame)
		select {
		case writeErr := <-writeDone:
			if writeErr != nil {
				_ = stop()
				return processResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), rc: 125, duration: time.Since(started)}, nil
			}
		case waitErr := <-done:
			_ = stdin.Close()
			return result(waitErr)
		case <-ctx.Done():
			_ = stop()
			return processResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), rc: 124, duration: time.Since(started), timedOut: true}, ctx.Err()
		}
	}
	_ = stdin.Close()
	select {
	case waitErr := <-done:
		return result(waitErr)
	case <-ctx.Done():
		_ = stop()
		return processResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), rc: 124, duration: time.Since(started), timedOut: true}, ctx.Err()
	}
}

type limitedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	limit  int
}

func (b *limitedBuffer) Write(content []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	original := len(content)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(content) > remaining {
			content = content[:remaining]
		}
		_, _ = b.buffer.Write(content)
	}
	return original, nil
}

func (b *limitedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buffer.Bytes()...)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exitErr.ExitCode()
	}
	return 125
}

func operationalError(err error) error {
	var exitErr *exec.ExitError
	if err == nil || errors.As(err, &exitErr) {
		return nil
	}
	return err
}

func normalizeTrace(harness string, trace []byte, processRC int) (string, []actorEvent, bool) {
	return normalizeTraceTurns(harness, trace, processRC, 1)
}

func normalizeTraceTurns(harness string, trace []byte, processRC, expectedRootTurns int) (string, []actorEvent, bool) {
	if expectedRootTurns < 1 {
		return "", nil, false
	}
	scanner := bufio.NewScanner(bytes.NewReader(trace))
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	response := ""
	events := make([]actorEvent, 0)
	terminal := false
	valid := len(bytes.TrimSpace(trace)) > 0
	claudeSegments := 0
	claudeMainResultSeen := false
	claudeAwaitingRootResult := false
	claudeRootResults := 0
	claudeOpenForwarded := 0
	claudeBatchedForwarding := false
	claudeForwardPermissions := 0
	pendingClaudeTools := make(map[string]pendingTool)
	seenClaudeTools := make(map[string]bool)
	claudeAgentTasks := make(map[string]bool)
	seenClaudeAgentTasks := make(map[string]bool)
	codexRootThreadID := ""
	receiptSequence := 0
	receiptSeen := false
	receiptEvents := make([]actorEvent, 0)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var object map[string]any
		if err := json.Unmarshal(line, &object); err != nil {
			valid = false
			continue
		}
		if firstString(object, "method") == "broker/executionReceipt" {
			receiptSeen = true
			receipt, ok := decodeExecutionReceipt(object)
			if !ok || receipt.Sequence <= receiptSequence {
				valid = false
				continue
			}
			receiptSequence = receipt.Sequence
			if receipt.OracleMatch && receipt.StateStable {
				receiptEvents = append(receiptEvents, actorEvent{Kind: "test", Path: receipt.Command, RC: receipt.ExitCode})
			}
			continue
		}
		var claudeLifecycleEvents []actorEvent
		if harness == "claude" {
			isInit := lowerString(object["type"]) == "system" && lowerString(object["subtype"]) == "init"
			preInitHook := lowerString(object["type"]) == "system" && (lowerString(object["subtype"]) == "hook_started" || lowerString(object["subtype"]) == "hook_response")
			if claudeSegments == 0 && !isInit && !preInitHook {
				valid = false
			}
			if claudeSegments == 0 && preInitHook {
				continue
			}
			// A batched fan-out flushes forwarded-segment results after the
			// main terminal result; those stay legal while their segments
			// remain open. Everything else after the terminal result needs a
			// permissioned forwarded init.
			isForwardedResult := false
			if object["type"] == "result" {
				if origin, ok := object["origin"].(map[string]any); ok {
					isForwardedResult = lowerString(origin["kind"]) == "task-notification"
				}
			}
			rootContinuation := terminal && claudeRootResults < expectedRootTurns && claudeOpenForwarded == 0
			if rootContinuation {
				terminal = false
				claudeAwaitingRootResult = true
			}
			if terminal && !(isInit && claudeForwardPermissions > 0) && !(isForwardedResult && claudeOpenForwarded > 0) {
				valid = false
				continue
			}
			var lifecycleValid bool
			claudeLifecycleEvents, lifecycleValid = normalizeClaudeTaskLifecycle(object, claudeAgentTasks, seenClaudeAgentTasks)
			if !lifecycleValid {
				valid = false
			}
			for _, event := range claudeLifecycleEvents {
				if event.Kind == "agent_complete" && event.RC == 0 {
					claudeForwardPermissions++
				}
			}
			if isInit {
				if claudeSegments == 0 {
					claudeAwaitingRootResult = true
				}
				if claudeSegments > 0 && !rootContinuation {
					if claudeForwardPermissions == 0 {
						valid = false
					} else {
						claudeForwardPermissions--
					}
					if claudeOpenForwarded > 0 {
						claudeBatchedForwarding = true
					}
					claudeOpenForwarded++
				}
				if !rootContinuation {
					claudeSegments++
				}
				terminal = false
			}
			if object["type"] == "result" {
				value, hasResult := object["result"].(string)
				isError, hasStatus := object["is_error"].(bool)
				originKind := ""
				rawOrigin, originPresent := object["origin"]
				origin, originObject := rawOrigin.(map[string]any)
				if originObject {
					originKind = lowerString(origin["kind"])
				}
				wellFormed := claudeSegments > 0 && hasResult && hasStatus && !isError
				switch {
				case originPresent && originObject && originKind == "task-notification":
					if !wellFormed || claudeOpenForwarded == 0 {
						valid = false
						terminal = false
						continue
					}
					claudeOpenForwarded--
					// In a batched fan-out these results carry subagent
					// output; the main result already holds the answer. In a
					// sequential resume they carry the resumed lead's answer.
					if !claudeBatchedForwarding {
						response = value
					}
					terminal = true
				case originPresent:
					valid = false
					terminal = false
				case claudeAwaitingRootResult:
					if !wellFormed {
						valid = false
						terminal = false
						continue
					}
					claudeMainResultSeen = true
					claudeAwaitingRootResult = false
					claudeRootResults++
					response = value
					terminal = true
				case claudeOpenForwarded > 0 && claudeSegments > 1:
					// An origin-less inner result inside a forwarded segment
					// carries no terminal meaning.
					terminal = false
				case claudeSegments > 1:
					terminal = false
				default:
					valid = false
					terminal = false
				}
			}
		} else {
			params, _ := object["params"].(map[string]any)
			if firstString(object, "method") == "thread/started" && codexRootThreadID == "" {
				thread, _ := params["thread"].(map[string]any)
				if thread["parentThreadId"] == nil {
					codexRootThreadID = firstString(thread, "id")
				}
			}
			rootItem := codexRootThreadID == "" || firstString(params, "threadId") == codexRootThreadID
			if object["type"] == "turn.completed" {
				terminal = true
			}
			if firstString(object, "method") == "turn/completed" && rootItem {
				terminal = true
			}
			item := codexTraceItem(object)
			if rootItem && (lowerString(item["type"]) == "agent_message" || lowerString(item["type"]) == "agentmessage") {
				if text, ok := item["text"].(string); ok {
					response = text
				}
			}
		}
		events = append(events, claudeLifecycleEvents...)
		if harness == "claude" {
			if len(normalizeObject(harness, object)) > 0 {
				valid = false
			}
			toolEvents, toolsValid := normalizeClaudeToolResults(object, pendingClaudeTools, seenClaudeTools)
			if !toolsValid {
				valid = false
			}
			events = append(events, toolEvents...)
		} else {
			events = append(events, normalizeObject(harness, object)...)
		}
	}
	if scanner.Err() != nil {
		valid = false
	}
	if harness == "claude" {
		events = reconcileClaudeSpawns(events)
	}
	if receiptSeen {
		withoutNativeTests := events[:0]
		for _, event := range events {
			if event.Kind != "test" {
				withoutNativeTests = append(withoutNativeTests, event)
			}
		}
		events = append(withoutNativeTests, receiptEvents...)
	}
	events = promoteSkillReads(events)
	events = deduplicateEvents(events)
	complete := valid && terminal && processRC == 0
	if harness == "claude" {
		complete = complete && claudeSegments > 0 && claudeMainResultSeen && claudeRootResults == expectedRootTurns && claudeOpenForwarded == 0
	}
	if complete {
		events = append(events, actorEvent{Kind: "trace_complete", RC: 0})
	}
	for i := range events {
		events[i].Step = i + 1
	}
	return response, events, complete
}

func decodeExecutionReceipt(object map[string]any) (testExecutionReceipt, bool) {
	params, ok := object["params"].(map[string]any)
	if !ok {
		return testExecutionReceipt{}, false
	}
	content, err := json.Marshal(params)
	if err != nil {
		return testExecutionReceipt{}, false
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var receipt testExecutionReceipt
	if decoder.Decode(&receipt) != nil || receipt.SchemaVersion != "1" || receipt.Sequence < 1 || receipt.StartedStep < 1 || receipt.CompletedStep <= receipt.StartedStep || !recognizedReceiptCommand(receipt.Command) || receipt.ExitCode < 0 || receipt.ExitCode > 255 || !validReceiptDigest(receipt.InvocationDigest) {
		return testExecutionReceipt{}, false
	}
	for _, state := range []receiptFileState{receipt.Before, receipt.After} {
		if state.Complete && !validReceiptDigest(state.Digest) {
			return testExecutionReceipt{}, false
		}
		if !state.Complete && (state.Digest != "" || len(state.ChangedFiles) != 0) {
			return testExecutionReceipt{}, false
		}
		if !sort.StringsAreSorted(state.ChangedFiles) {
			return testExecutionReceipt{}, false
		}
		for index, path := range state.ChangedFiles {
			if path == "" || filepath.IsAbs(path) || path != filepath.ToSlash(filepath.Clean(path)) || (index > 0 && path == state.ChangedFiles[index-1]) {
				return testExecutionReceipt{}, false
			}
		}
	}
	stable := receipt.Before.Complete && receipt.After.Complete && receipt.Before.Digest == receipt.After.Digest
	if receipt.StateStable != stable {
		return testExecutionReceipt{}, false
	}
	return receipt, true
}

func validReceiptDigest(digest string) bool {
	if len(digest) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(digest, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(digest, "sha256:"))
	return err == nil
}

func recognizedReceiptCommand(command string) bool {
	switch command {
	case "go test", "npm test", "cargo test", "pytest", "scripts/validate.sh":
		return true
	default:
		return false
	}
}

func receiptCommand(executable string, args []string) string {
	name := strings.ToLower(filepath.Base(executable))
	switch name {
	case "go":
		for index := 0; index < len(args); index++ {
			argument := args[index]
			if argument == "-C" && index+1 < len(args) {
				index++
				continue
			}
			if strings.HasPrefix(argument, "-") {
				continue
			}
			if argument == "test" {
				return "go test"
			}
			return ""
		}
	case "npm", "cargo":
		for _, argument := range args {
			if strings.HasPrefix(argument, "-") {
				continue
			}
			if argument == "test" {
				return name + " test"
			}
			return ""
		}
	case "pytest", "py.test":
		return "pytest"
	case "python", "python3":
		for index := 0; index+1 < len(args); index++ {
			if args[index] == "-m" && (args[index+1] == "pytest" || args[index+1] == "py.test") {
				return "pytest"
			}
		}
	case "bash", "sh", "dash":
		for _, argument := range args {
			if strings.Contains(argument, "c") && strings.HasPrefix(argument, "-") {
				return ""
			}
			if strings.HasPrefix(argument, "-") {
				continue
			}
			path := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(argument)), "./")
			if path == "scripts/validate.sh" || strings.HasSuffix(path, "/scripts/validate.sh") {
				return "scripts/validate.sh"
			}
			return ""
		}
	}
	return ""
}

func receiptInvocationMatchesOracle(command string, arguments, oracle []string, singlePackage bool) bool {
	if len(oracle) == 0 || receiptCommand(oracle[0], oracle[1:]) != command {
		return false
	}
	expected := oracle[1:]
	if equalReceiptArguments(arguments, expected) {
		return true
	}
	if command != "go test" {
		return false
	}
	testIndex := -1
	for index, argument := range arguments {
		if argument == "test" {
			testIndex = index
			break
		}
	}
	if testIndex < 0 {
		return false
	}
	filtered := make([]string, 0, len(arguments))
	seenRun := false
	seenCount := false
	seenVerbose := false
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if index <= testIndex {
			filtered = append(filtered, argument)
			continue
		}
		switch {
		case argument == "-run":
			if seenRun || index+1 >= len(arguments) || !validReceiptRunPattern(arguments[index+1]) {
				return false
			}
			seenRun = true
			index++
		case strings.HasPrefix(argument, "-run="):
			if seenRun || !validReceiptRunPattern(strings.TrimPrefix(argument, "-run=")) {
				return false
			}
			seenRun = true
		case argument == "-count":
			if seenCount || index+1 >= len(arguments) || !validReceiptTestCount(arguments[index+1]) {
				return false
			}
			seenCount = true
			index++
		case strings.HasPrefix(argument, "-count="):
			if seenCount || !validReceiptTestCount(strings.TrimPrefix(argument, "-count=")) {
				return false
			}
			seenCount = true
		case argument == "-v" || argument == "-v=true":
			if seenVerbose {
				return false
			}
			seenVerbose = true
		default:
			filtered = append(filtered, argument)
		}
	}
	if (seenRun || seenCount || seenVerbose) && equalReceiptArguments(filtered, expected) {
		return true
	}
	if !singlePackage || !equalReceiptArguments(expected, []string{"test", "./..."}) {
		return false
	}
	return equalReceiptArguments(filtered, []string{"test"}) || equalReceiptArguments(filtered, []string{"test", "."})
}

func receiptSnapshotIsSingleGoPackage(snapshot receiptProjectSnapshot) bool {
	if !snapshot.complete || !strings.HasPrefix(snapshot.files["go.mod"], "file:") {
		return false
	}
	rootPackage := false
	for path, content := range snapshot.files {
		if !strings.HasSuffix(path, ".go") || !strings.HasPrefix(content, "file:") {
			continue
		}
		if strings.ContainsRune(path, '/') {
			return false
		}
		rootPackage = true
	}
	return rootPackage
}

func validReceiptRunPattern(pattern string) bool {
	if pattern == "" || len(pattern) > receiptMaximumPath {
		return false
	}
	_, err := regexp.Compile(pattern)
	return err == nil
}

func validReceiptTestCount(value string) bool {
	count, err := strconv.Atoi(value)
	return err == nil && count >= 1 && count <= 10
}

func equalReceiptArguments(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validReceiptArguments(arguments []string) bool {
	if len(arguments) > receiptMaximumArgs {
		return false
	}
	total := 0
	for _, argument := range arguments {
		if len(argument) > receiptMaximumArg || strings.ContainsRune(argument, 0) {
			return false
		}
		total += len(argument)
		if total > receiptMaximumArgv {
			return false
		}
	}
	return true
}

func receiptInvocationDigest(command string, arguments []string) string {
	hasher := sha256.New()
	_, _ = io.WriteString(hasher, command)
	for _, argument := range arguments {
		_, _ = hasher.Write([]byte{0})
		_, _ = io.WriteString(hasher, argument)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func snapshotReceiptProject(root string) receiptProjectSnapshot {
	rooted, err := os.OpenRoot(root)
	if err != nil {
		return receiptProjectSnapshot{complete: false, files: map[string]string{}}
	}
	defer rooted.Close()

	snapshot := receiptProjectSnapshot{complete: true, files: make(map[string]string)}
	count := 0
	var total int64
	type pendingDirectory struct {
		path string
		info fs.FileInfo
	}
	rootDirectory, rootInfo, err := openReceiptSnapshotEntry(rooted, ".", true)
	if err != nil {
		return receiptProjectSnapshot{complete: false, files: map[string]string{}}
	}
	if err := rootDirectory.Close(); err != nil {
		return receiptProjectSnapshot{complete: false, files: map[string]string{}}
	}
	pending := []pendingDirectory{{path: ".", info: rootInfo}}
	for len(pending) > 0 && err == nil {
		directory := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		var opened *os.File
		opened, rootInfo, err = openReceiptSnapshotEntry(rooted, directory.path, true)
		if err != nil || !os.SameFile(directory.info, rootInfo) {
			if opened != nil {
				_ = opened.Close()
			}
			err = errors.New("receipt directory changed during snapshot")
			break
		}
		for err == nil {
			var entries []os.DirEntry
			entries, err = opened.ReadDir(128)
			if errors.Is(err, io.EOF) {
				err = nil
				break
			}
			if err != nil || len(entries) == 0 {
				if err == nil {
					err = errors.New("receipt directory returned an empty page")
				}
				break
			}
			for _, entry := range entries {
				name := entry.Name()
				if name == "" || name == "." || name == ".." || strings.ContainsRune(name, '/') {
					err = errors.New("receipt path is invalid")
					break
				}
				relative := name
				if directory.path != "." {
					relative = directory.path + "/" + name
				}
				if len(relative) > receiptMaximumPath {
					err = errors.New("receipt path bound exceeded")
					break
				}
				count++
				if count > receiptMaximumEntries {
					err = errors.New("receipt entry bound exceeded")
					break
				}
				info, lstatErr := rooted.Lstat(relative)
				if lstatErr != nil {
					err = lstatErr
					break
				}
				first := strings.SplitN(relative, "/", 2)[0]
				if info.IsDir() {
					if first == ".git" || first == ".actor-cache" || first == "node_modules" || first == "target" {
						continue
					}
					pending = append(pending, pendingDirectory{path: relative, info: info})
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					target, readlinkErr := rooted.Readlink(relative)
					verified, verifyErr := rooted.Lstat(relative)
					if readlinkErr != nil || verifyErr != nil || !os.SameFile(info, verified) || len(target) > receiptMaximumPath {
						err = errors.New("receipt symlink changed or exceeded its bound")
						break
					}
					snapshot.files[relative] = "symlink:" + target
					continue
				}
				if !info.Mode().IsRegular() {
					err = errors.New("receipt content is not regular")
					break
				}
				file, openedInfo, openErr := openReceiptSnapshotEntry(rooted, relative, false)
				if openErr != nil || !os.SameFile(info, openedInfo) || openedInfo.Size() < 0 || openedInfo.Size() > receiptMaximumFile || total+openedInfo.Size() > receiptMaximumBytes {
					if file != nil {
						_ = file.Close()
					}
					err = errors.New("receipt content changed or exceeded its bound")
					break
				}
				hasher := sha256.New()
				copied, copyErr := io.Copy(hasher, io.LimitReader(file, receiptMaximumFile+1))
				finalInfo, statErr := file.Stat()
				closeErr := file.Close()
				verified, verifyErr := rooted.Lstat(relative)
				if copyErr != nil || statErr != nil || closeErr != nil || verifyErr != nil || copied != openedInfo.Size() || finalInfo.Size() != openedInfo.Size() || finalInfo.ModTime() != openedInfo.ModTime() || !os.SameFile(openedInfo, finalInfo) || !os.SameFile(openedInfo, verified) {
					err = errors.New("receipt file changed during snapshot")
					break
				}
				total += openedInfo.Size()
				snapshot.files[relative] = "file:" + hex.EncodeToString(hasher.Sum(nil))
			}
		}
		finalDirectory, finalInfo, verifyErr := openReceiptSnapshotEntry(rooted, directory.path, true)
		if finalDirectory != nil {
			_ = finalDirectory.Close()
		}
		closeErr := opened.Close()
		if err == nil && (verifyErr != nil || closeErr != nil || !os.SameFile(rootInfo, finalInfo)) {
			err = errors.New("receipt directory changed during snapshot")
		}
	}
	if err != nil {
		return receiptProjectSnapshot{complete: false, files: map[string]string{}}
	}
	paths := make([]string, 0, len(snapshot.files))
	for path := range snapshot.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hasher := sha256.New()
	for _, path := range paths {
		_, _ = io.WriteString(hasher, path)
		_, _ = hasher.Write([]byte{0})
		_, _ = io.WriteString(hasher, snapshot.files[path])
		_, _ = hasher.Write([]byte{0})
	}
	snapshot.digest = "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	return snapshot
}

func openReceiptSnapshotEntry(root *os.Root, relative string, directory bool) (*os.File, fs.FileInfo, error) {
	flags := os.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	if directory {
		flags |= syscall.O_DIRECTORY
	}
	file, err := root.OpenFile(relative, flags, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil || (directory && !info.IsDir()) || (!directory && !info.Mode().IsRegular()) {
		_ = file.Close()
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, errors.New("receipt entry has an unexpected type")
	}
	return file, info, nil
}

func receiptFileStateSince(root string, baseline receiptProjectSnapshot) receiptFileState {
	return receiptFileStateFromSnapshots(baseline, snapshotReceiptProject(root))
}

func receiptFileStateFromSnapshots(baseline, current receiptProjectSnapshot) receiptFileState {
	if !baseline.complete || !current.complete {
		return receiptFileState{Complete: false, ChangedFiles: []string{}}
	}
	changed := make([]string, 0)
	seen := make(map[string]bool, len(baseline.files)+len(current.files))
	for path := range baseline.files {
		seen[path] = true
	}
	for path := range current.files {
		seen[path] = true
	}
	for path := range seen {
		if baseline.files[path] != current.files[path] {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return receiptFileState{Complete: true, Digest: current.digest, ChangedFiles: changed}
}

func appendTrustedExecutionReceipts(trace []byte, receipts []testExecutionReceipt) ([]byte, error) {
	for _, line := range bytes.Split(trace, []byte{'\n'}) {
		var object map[string]any
		if json.Unmarshal(bytes.TrimSpace(line), &object) == nil && firstString(object, "method") == "broker/executionReceipt" {
			return nil, errors.New("actor trace used the reserved broker receipt method")
		}
	}
	result := append([]byte(nil), trace...)
	if len(result) > 0 && result[len(result)-1] != '\n' {
		result = append(result, '\n')
	}
	for _, receipt := range receipts {
		line, err := json.Marshal(map[string]any{"method": "broker/executionReceipt", "params": receipt})
		if err != nil {
			return nil, err
		}
		if len(result)+len(line)+1 > traceLimit {
			return nil, errors.New("execution receipts exceed the trace bound")
		}
		result = append(result, line...)
		result = append(result, '\n')
	}
	return result, nil
}

func startReceiptCollector(project, socket string, oracle []string) (*receiptCollector, error) {
	if err := validateSocketPath(socket); err != nil {
		return nil, err
	}
	baseline := snapshotReceiptProject(project)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return nil, err
	}
	collector := &receiptCollector{
		listener:    listener,
		project:     project,
		oracle:      append([]string(nil), oracle...),
		baseline:    baseline,
		connections: make(map[*net.UnixConn]bool),
		records:     make([]testExecutionReceipt, 0),
		serveDone:   make(chan struct{}),
	}
	go collector.serve()
	return collector, nil
}

func (c *receiptCollector) serve() {
	defer close(c.serveDone)
	for {
		connection, err := c.listener.AcceptUnix()
		if err != nil {
			return
		}
		c.wg.Add(1)
		c.mu.Lock()
		c.connections[connection] = true
		c.mu.Unlock()
		go c.handle(connection)
	}
}

func (c *receiptCollector) handle(connection *net.UnixConn) {
	defer c.wg.Done()
	active := false
	defer func() {
		c.mu.Lock()
		if active {
			c.activeRuns--
		}
		delete(c.connections, connection)
		c.mu.Unlock()
		_ = connection.Close()
	}()
	_ = connection.SetDeadline(time.Now().Add(30 * time.Second))
	decoder := json.NewDecoder(io.LimitReader(connection, 16<<10))
	decoder.DisallowUnknownFields()
	encoder := json.NewEncoder(connection)
	var start receiptClientMessage
	if decoder.Decode(&start) != nil || start.SchemaVersion != "1" || start.Action != "start" || !recognizedReceiptCommand(start.Command) || !validReceiptArguments(start.Arguments) || start.ID != 0 || start.ExitCode != nil {
		_ = encoder.Encode(receiptServerMessage{Accepted: false})
		return
	}
	peerPID, peerOK := receiptPeerIsWrapper(connection, start.Command, start.Arguments)
	if !peerOK {
		_ = encoder.Encode(receiptServerMessage{Accepted: false})
		return
	}
	c.mu.Lock()
	if c.sealed || c.nextID >= receiptMaximumRuns {
		c.mu.Unlock()
		_ = encoder.Encode(receiptServerMessage{Accepted: false})
		return
	}
	c.activeRuns++
	active = true
	c.nextID++
	id := c.nextID
	c.nextStep++
	startedStep := c.nextStep
	beforeSnapshot := snapshotReceiptProject(c.project)
	before := receiptFileStateFromSnapshots(c.baseline, beforeSnapshot)
	oracleMatch := receiptPeerCWDMatchesProject(peerPID, c.project) && receiptInvocationMatchesOracle(start.Command, start.Arguments, c.oracle, receiptSnapshotIsSingleGoPackage(beforeSnapshot))
	invocationDigest := receiptInvocationDigest(start.Command, start.Arguments)
	c.mu.Unlock()
	if encoder.Encode(receiptServerMessage{Accepted: true, ID: id}) != nil {
		return
	}
	_ = connection.SetDeadline(time.Now().Add(maximumRun))
	var finish receiptClientMessage
	if decoder.Decode(&finish) != nil || finish.SchemaVersion != "1" || finish.Action != "finish" || finish.ID != id || finish.Command != "" || len(finish.Arguments) != 0 || finish.ExitCode == nil || *finish.ExitCode < 0 || *finish.ExitCode > 255 {
		return
	}
	c.mu.Lock()
	c.nextStep++
	completedStep := c.nextStep
	after := receiptFileStateSince(c.project, c.baseline)
	receipt := testExecutionReceipt{
		SchemaVersion:    "1",
		Sequence:         len(c.records) + 1,
		StartedStep:      startedStep,
		CompletedStep:    completedStep,
		Command:          start.Command,
		ExitCode:         *finish.ExitCode,
		OracleMatch:      oracleMatch,
		InvocationDigest: invocationDigest,
		StateStable:      before.Complete && after.Complete && before.Digest == after.Digest,
		Before:           before,
		After:            after,
	}
	c.records = append(c.records, receipt)
	c.activeRuns--
	active = false
	c.mu.Unlock()
	_ = connection.SetDeadline(time.Now().Add(30 * time.Second))
	_ = encoder.Encode(receiptServerMessage{Accepted: true, ID: id})
}

func receiptPeerIsWrapper(connection *net.UnixConn, command string, claimedArguments []string) (int, bool) {
	raw, err := connection.SyscallConn()
	if err != nil {
		return 0, false
	}
	var credentials *syscall.Ucred
	var controlErr error
	if err := raw.Control(func(fd uintptr) {
		credentials, controlErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil || controlErr != nil || credentials == nil || credentials.Pid <= 0 {
		return 0, false
	}
	peer, err := os.Stat(fmt.Sprintf("/proc/%d/exe", credentials.Pid))
	if err != nil {
		return 0, false
	}
	selfPath, err := os.Executable()
	if err != nil {
		return 0, false
	}
	self, err := os.Stat(selfPath)
	if err != nil || !os.SameFile(peer, self) {
		return 0, false
	}
	cmdlineFile, err := os.Open(fmt.Sprintf("/proc/%d/cmdline", credentials.Pid))
	if err != nil {
		return 0, false
	}
	cmdline, readErr := io.ReadAll(io.LimitReader(cmdlineFile, 16<<10+1))
	closeErr := cmdlineFile.Close()
	if readErr != nil || closeErr != nil || len(cmdline) == 0 || len(cmdline) > 16<<10 {
		return 0, false
	}
	arguments := bytes.Split(cmdline, []byte{0})
	for _, argument := range arguments[1:] {
		if string(argument) == "--actor-bridge" {
			return 0, false
		}
	}
	role := strings.ToLower(filepath.Base(string(arguments[0])))
	if strings.HasSuffix(strings.ToLower(filepath.Base(selfPath)), ".test") && strings.HasSuffix(role, ".test") {
		return int(credentials.Pid), true
	}
	roleMatches := false
	switch command {
	case "go test":
		roleMatches = role == "go"
	case "npm test":
		roleMatches = role == "npm"
	case "cargo test":
		roleMatches = role == "cargo"
	case "pytest":
		roleMatches = role == "pytest" || role == "py.test" || role == "python3"
	case "scripts/validate.sh":
		roleMatches = role == "bash" || role == "sh" || role == "dash"
	}
	if !roleMatches || len(arguments) < 2 || len(arguments[len(arguments)-1]) != 0 {
		return 0, false
	}
	observedArguments := make([]string, 0, len(arguments)-2)
	for _, argument := range arguments[1 : len(arguments)-1] {
		observedArguments = append(observedArguments, string(argument))
	}
	if !equalReceiptArguments(observedArguments, claimedArguments) {
		return 0, false
	}
	return int(credentials.Pid), true
}

func receiptPeerCWDMatchesProject(pid int, project string) bool {
	cwd, err := os.Stat(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return false
	}
	root, err := os.Stat(project)
	return err == nil && os.SameFile(cwd, root)
}

func (c *receiptCollector) receipts() []testExecutionReceipt {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]testExecutionReceipt(nil), c.records...)
}

func (c *receiptCollector) seal() ([]testExecutionReceipt, error) {
	c.mu.Lock()
	c.sealed = true
	c.mu.Unlock()
	c.stopAccepting()
	<-c.serveDone
	c.mu.Lock()
	active := c.activeRuns
	c.mu.Unlock()
	c.closeConnections()
	c.wg.Wait()
	if active > 0 {
		return nil, fmt.Errorf("execution receipt collector sealed with %d active command(s)", active)
	}
	return c.receipts(), nil
}

func (c *receiptCollector) stopAccepting() {
	c.stopOnce.Do(func() { _ = c.listener.Close() })
}

func (c *receiptCollector) closeConnections() {
	c.mu.Lock()
	connections := make([]*net.UnixConn, 0, len(c.connections))
	for connection := range c.connections {
		connections = append(connections, connection)
	}
	c.mu.Unlock()
	for _, connection := range connections {
		_ = connection.Close()
	}
}

func (c *receiptCollector) close() {
	c.mu.Lock()
	c.sealed = true
	c.mu.Unlock()
	c.stopAccepting()
	<-c.serveDone
	c.closeConnections()
	c.wg.Wait()
	_ = os.Remove(c.listener.Addr().String())
}

func runReceiptWrappedExecutable(real string, args []string, command, socket string) int {
	var connection *net.UnixConn
	var decoder *json.Decoder
	var encoder *json.Encoder
	receiptID := 0
	if command != "" {
		connected, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socket, Net: "unix"})
		if err == nil {
			connection = connected
			decoder = json.NewDecoder(connection)
			encoder = json.NewEncoder(connection)
			_ = connection.SetDeadline(time.Now().Add(30 * time.Second))
			if encoder.Encode(receiptClientMessage{SchemaVersion: "1", Action: "start", Command: command, Arguments: append([]string(nil), args...)}) == nil {
				var response receiptServerMessage
				if decoder.Decode(&response) == nil && response.Accepted {
					receiptID = response.ID
				}
			}
		}
	}
	child := exec.Command(real, args...)
	child.Env = receiptChildEnvironment(os.Environ(), real, command)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	defer signal.Stop(signals)
	rc := 125
	if err := child.Start(); err == nil {
		done := make(chan error, 1)
		go func() { done <- child.Wait() }()
		select {
		case err := <-done:
			rc = exitCode(err)
		case received := <-signals:
			forwarded, ok := received.(syscall.Signal)
			if ok {
				_ = syscall.Kill(-child.Process.Pid, forwarded)
			}
			select {
			case err := <-done:
				rc = exitCode(err)
			case <-time.After(500 * time.Millisecond):
				_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
				rc = exitCode(<-done)
			}
		}
	}
	if rc < 0 || rc > 255 {
		rc = 125
	}
	if connection != nil {
		if receiptID > 0 {
			_ = connection.SetDeadline(time.Now().Add(30 * time.Second))
			if encoder.Encode(receiptClientMessage{SchemaVersion: "1", Action: "finish", ID: receiptID, ExitCode: &rc}) == nil {
				var response receiptServerMessage
				_ = decoder.Decode(&response)
			}
		}
		_ = connection.Close()
	}
	return rc
}

func receiptChildEnvironment(environment []string, real, command string) []string {
	goRuntime := real == "/opt/megapowers-receipt/real/go"
	goTest := command == "go test"
	if !goRuntime && !goTest {
		return environment
	}
	result := make([]string, 0, len(environment)+2)
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		if key == "GOROOT" || goTest && (key == "GOFLAGS" || key == "GOENV") {
			continue
		}
		result = append(result, entry)
	}
	result = append(result, "GOROOT=/opt/megapowers-runtime/go")
	if goTest {
		result = append(result, "GOENV=off")
	}
	return result
}

type pendingTool struct {
	name  string
	input map[string]any
}

func normalizeObject(harness string, object map[string]any) []actorEvent {
	events := make([]actorEvent, 0, 2)
	if harness == "codex" {
		topLevelType := lowerString(object["type"])
		if strings.HasPrefix(topLevelType, "item.") && topLevelType != "item.completed" {
			return events
		}
		method := strings.ToLower(firstString(object, "method"))
		if strings.HasPrefix(method, "item/") && method != "item/completed" {
			return events
		}
	}
	target := object
	if harness == "codex" {
		target = codexTraceItem(object)
	} else if item, ok := object["item"].(map[string]any); ok {
		target = item
	}
	typeName := lowerString(target["type"])
	rc := numericRC(target)
	if harness == "codex" && strings.EqualFold(firstString(object, "method"), "item/completed") {
		status, hasStatus := target["status"].(string)
		if !hasStatus || strings.ToLower(status) != "completed" {
			rc = 1
		}
	}
	path := firstString(target, "path", "agent_id", "task_name", "id", "name")
	switch typeName {
	case "subagentactivity":
		// Native activity items have no status field. Their kind carries the
		// lifecycle result, and the thread ID also matches legacy tool calls.
		path := firstString(target, "agentThreadId")
		if path == "" || firstString(target, "agentPath") == "" {
			break
		}
		switch lowerString(target["kind"]) {
		case "started":
			events = append(events, actorEvent{Kind: "agent_spawn", Path: path, RC: 0})
		case "completed":
			events = append(events, actorEvent{Kind: "agent_complete", Path: path, RC: 0})
		case "interrupted":
			events = append(events, actorEvent{Kind: "agent_complete", Path: path, RC: 1})
		}
	case "subagent_start", "collab_agent_spawn_end", "agent_spawn":
		events = append(events, actorEvent{Kind: "agent_spawn", Path: path, RC: rc})
	case "subagent_stop", "collab_agent_complete", "agent_complete":
		events = append(events, actorEvent{Kind: "agent_complete", Path: path, RC: rc})
	case "collab_close_end":
		events = append(events, actorEvent{Kind: "agent_complete", Path: path, RC: rc})
	case "collab_waiting_begin", "agent_wait":
		events = append(events, actorEvent{Kind: "agent_wait", RC: rc})
	case "file_change", "filechange":
		events = append(events, actorEvent{Kind: "write", Path: changedPaths(target), RC: rc})
	case "command_execution", "commandexecution":
		if commandRC, ok := explicitCommandRC(target); ok {
			events = append(events, classifyCommand(firstString(target, "command"), commandRC)...)
			if harness == "codex" {
				events = append(events, skillReadEvents(firstString(target, "command"), commandRC)...)
			}
		}
	case "collabagenttoolcall":
		tool := lowerString(target["tool"])
		switch tool {
		case "spawnagent":
			events = append(events, actorEvent{Kind: "agent_spawn", Path: firstStringSlice(target, "receiverThreadIds"), RC: rc})
		case "closeagent":
			events = append(events, actorEvent{Kind: "agent_complete", Path: firstStringSlice(target, "receiverThreadIds"), RC: rc})
		case "wait":
			events = append(events, actorEvent{Kind: "agent_wait", RC: rc})
		}
	}
	if harness != "claude" {
		name := firstString(target, "name", "tool_name", "tool")
		input := normalizedToolInput(target)
		events = append(events, normalizeTool(name, input, rc)...)
	}
	return events
}

func normalizeClaudeTaskLifecycle(object map[string]any, agentTasks, seenAgentTasks map[string]bool) ([]actorEvent, bool) {
	if lowerString(object["type"]) != "system" {
		return nil, true
	}
	subtype := lowerString(object["subtype"])
	taskID := firstString(object, "task_id")
	switch subtype {
	case "task_started":
		if lowerString(object["task_type"]) != "local_agent" {
			return nil, true
		}
		if taskID == "" || seenAgentTasks[taskID] {
			return nil, false
		}
		seenAgentTasks[taskID] = true
		agentTasks[taskID] = true
		return []actorEvent{{Kind: "agent_spawn", Path: taskID}}, true
	case "task_notification":
		if !agentTasks[taskID] {
			if taskID != "" && seenAgentTasks[taskID] {
				return nil, false
			}
			return nil, true
		}
		delete(agentTasks, taskID)
		rc := 1
		if status, ok := object["status"].(string); ok && strings.EqualFold(status, "completed") {
			rc = 0
		}
		return []actorEvent{{Kind: "agent_complete", Path: taskID, RC: rc}}, true
	}
	return nil, true
}

func codexTraceItem(object map[string]any) map[string]any {
	if item, ok := object["item"].(map[string]any); ok {
		return item
	}
	params, _ := object["params"].(map[string]any)
	if item, ok := params["item"].(map[string]any); ok {
		return item
	}
	return object
}

func normalizeClaudeToolResults(object map[string]any, pending map[string]pendingTool, seen map[string]bool) ([]actorEvent, bool) {
	envelopeType := lowerString(object["type"])
	message, ok := object["message"].(map[string]any)
	if !ok {
		return nil, true
	}
	content, ok := message["content"].([]any)
	if !ok {
		return nil, true
	}
	events := make([]actorEvent, 0)
	valid := true
	for _, raw := range content {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch lowerString(block["type"]) {
		case "tool_use":
			if envelopeType != "assistant" {
				valid = false
				continue
			}
			id := firstString(block, "id")
			name := firstString(block, "name")
			if id == "" || name == "" || seen[id] {
				valid = false
				continue
			}
			seen[id] = true
			input, _ := block["input"].(map[string]any)
			pending[id] = pendingTool{name: name, input: input}
		case "tool_result":
			if envelopeType != "user" {
				valid = false
				continue
			}
			id := firstString(block, "tool_use_id")
			tool, exists := pending[id]
			if !exists {
				valid = false
				continue
			}
			// Current CLIs omit is_error on some successful results and
			// attach the result payload instead; a matched statusless
			// result with a payload object is proven execution. A matched
			// statusless result without any payload stays unconfirmed.
			isError, statusPresent := block["is_error"].(bool)
			if !statusPresent {
				if result, ok := object["tool_use_result"].(map[string]any); ok {
					statusPresent = true
					if success, hasSuccess := result["success"].(bool); hasSuccess {
						isError = !success
					} else {
						isError = false
					}
				}
			}
			delete(pending, id)
			if !statusPresent {
				continue
			}
			rc := 0
			if isError {
				rc = 1
			}
			toolEvents := normalizeTool(tool.name, tool.input, rc)
			// Task lifecycle events are the authoritative spawn evidence
			// when present; tool-derived spawns stay provisional until the
			// trace-level reconciliation pass.
			for index := range toolEvents {
				if toolEvents[index].Kind == "agent_spawn" {
					toolEvents[index].Kind = "agent_spawn_tool"
				}
			}
			events = append(events, toolEvents...)
		}
	}
	return events, valid
}

func normalizeTool(name string, input map[string]any, rc int) []actorEvent {
	lower := strings.ToLower(name)
	switch lower {
	case "skill", "select_skill", "skills.read", "skills_read", "mcp__skills__read":
		identifier := firstString(input, "package", "skill", "skill_name", "name", "path", "resource", "uri", "main_resource")
		return []actorEvent{{Kind: "skill_selected", Path: normalizeSkillIdentifier(identifier), RC: rc}}
	case "spawn_agent", "task", "agent":
		return []actorEvent{{Kind: "agent_spawn", Path: firstString(input, "task_name", "name", "description", "path"), RC: rc}}
	case "wait_agent", "taskoutput", "agent_wait":
		return []actorEvent{{Kind: "agent_wait", RC: rc}}
	case "apply_patch", "write", "edit", "notebookedit":
		return []actorEvent{{Kind: "write", Path: firstString(input, "file_path", "path"), RC: rc}}
	case "bash", "exec_command", "shell":
		return classifyCommand(firstString(input, "command", "cmd"), rc)
	}
	return nil
}

func normalizedToolInput(target map[string]any) map[string]any {
	for _, key := range []string{"input", "arguments", "args"} {
		switch value := target[key].(type) {
		case map[string]any:
			return value
		case string:
			var decoded map[string]any
			if json.Unmarshal([]byte(value), &decoded) == nil {
				return decoded
			}
		}
	}
	return nil
}

func normalizeSkillIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.LastIndex(value, ":"); index >= 0 {
		value = value[index+1:]
	}
	value = strings.TrimSuffix(strings.TrimRight(value, "/"), "/SKILL.md")
	if strings.Contains(value, "/") {
		value = filepath.Base(value)
	}
	return value
}

func classifyCommand(command string, rc int) []actorEvent {
	segments, safeForTest := normalizedCommandSegments(command)
	if len(segments) == 0 {
		return nil
	}
	events := make([]actorEvent, 0, 3)
	workingDirectory := ""
	for _, fields := range segments {
		fields = executableFields(fields)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "cd" && len(fields) >= 2 {
			workingDirectory = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(fields[1])), "./")
			continue
		}
		if isGoRunTarget(fields, "cmd/tracker-comment", workingDirectory) || commandNameMatches(fields[0], "tracker-comment") {
			events = append(events, actorEvent{Kind: "tracker_comment", RC: rc})
		}
		if isGoRunTarget(fields, "cmd/pr-comment", workingDirectory) || commandNameMatches(fields[0], "pr-comment") {
			events = append(events, actorEvent{Kind: "pr_comment", RC: rc})
		}
	}
	if !safeForTest || len(segments) != 1 {
		return events
	}
	if isTestCommand(executableFields(segments[0])) {
		events = append(events, actorEvent{Kind: "test", RC: rc})
	}
	return events
}

func normalizedCommandSegments(command string) ([][]string, bool) {
	command = strings.TrimSpace(command)
	for _, prefix := range []string{"/usr/bin/bash -lc ", "/bin/bash -lc ", "bash -lc ", "/usr/bin/sh -lc ", "/bin/sh -lc ", "sh -lc "} {
		if !strings.HasPrefix(command, prefix) {
			continue
		}
		inner := strings.TrimSpace(strings.TrimPrefix(command, prefix))
		if len(inner) < 2 || (inner[0] != '\'' && inner[0] != '"') || inner[len(inner)-1] != inner[0] {
			return nil, false
		}
		command = strings.TrimSpace(inner[1 : len(inner)-1])
		break
	}
	if command == "" {
		return nil, false
	}
	segments := make([][]string, 0, 2)
	start := 0
	quote := byte(0)
	escaped := false
	safeForTest := true
	flush := func(end int) {
		fields := strings.Fields(strings.TrimSpace(command[start:end]))
		for index := range fields {
			fields[index] = strings.ToLower(strings.Trim(fields[index], "'\"()"))
		}
		if len(fields) > 0 {
			segments = append(segments, fields)
		}
	}
	for index := 0; index < len(command); index++ {
		character := command[index]
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			continue
		}
		if character == '`' || (character == '$' && index+1 < len(command) && command[index+1] == '(') {
			safeForTest = false
		}
		if character == ';' || character == '|' || character == '&' || character == '\n' || character == '\r' {
			flush(index)
			start = index + 1
			safeForTest = false
		}
	}
	if quote != 0 || escaped {
		return nil, false
	}
	flush(len(command))
	return segments, safeForTest
}

func executableFields(fields []string) []string {
	index := 0
	if len(fields) > 0 && commandNameMatches(fields[0], "env") {
		index++
	}
	for index < len(fields) && strings.Contains(fields[index], "=") {
		index++
	}
	return fields[index:]
}

func isGoRunTarget(fields []string, target, workingDirectory string) bool {
	if len(fields) < 3 || fields[0] != "go" || fields[1] != "run" {
		return false
	}
	path := strings.TrimPrefix(filepath.ToSlash(fields[2]), "./")
	if path == target || strings.HasPrefix(path, target+"/") {
		return true
	}
	return (path == "." || path == "") && (workingDirectory == target || strings.HasPrefix(workingDirectory, target+"/"))
}

func commandNameMatches(command, name string) bool {
	return command == name || strings.HasSuffix(filepath.ToSlash(command), "/"+name)
}

func isTestCommand(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	if len(fields) >= 2 && ((fields[0] == "go" && fields[1] == "test") || (fields[0] == "npm" && fields[1] == "test") || (fields[0] == "cargo" && fields[1] == "test")) {
		return true
	}
	if fields[0] == "pytest" || commandNameMatches(fields[0], "pytest") {
		return true
	}
	if len(fields) >= 3 && (fields[0] == "python" || fields[0] == "python3") && fields[1] == "-m" && fields[2] == "pytest" {
		return true
	}
	if commandNameMatches(fields[0], "scripts/validate.sh") {
		return true
	}
	return len(fields) >= 2 && (fields[0] == "bash" || fields[0] == "sh") && commandNameMatches(fields[1], "scripts/validate.sh")
}

func explicitCommandRC(object map[string]any) (int, bool) {
	for _, key := range []string{"rc", "exit_code", "exitCode"} {
		switch value := object[key].(type) {
		case float64:
			return int(value), true
		case json.Number:
			parsed, err := value.Int64()
			return int(parsed), err == nil
		}
	}
	return 0, false
}

func numericRC(object map[string]any) int {
	rc := 0
	for _, key := range []string{"rc", "exit_code", "exitCode"} {
		switch value := object[key].(type) {
		case float64:
			rc = int(value)
		case json.Number:
			parsed, err := value.Int64()
			if err != nil {
				return 1
			}
			rc = int(parsed)
		}
	}
	if rawStatus, present := object["status"]; present {
		status, ok := rawStatus.(string)
		if !ok {
			return 1
		}
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "completed", "success", "succeeded":
			return rc
		default:
			return 1
		}
	}
	return rc
}

func firstString(object map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := object[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func firstStringSlice(object map[string]any, key string) string {
	values, _ := object[key].([]any)
	for _, value := range values {
		if text, ok := value.(string); ok && text != "" {
			return text
		}
	}
	return ""
}

func lowerString(value any) string {
	text, _ := value.(string)
	return strings.ToLower(text)
}

func changedPaths(object map[string]any) string {
	values, _ := object["changes"].([]any)
	paths := make([]string, 0, len(values))
	for _, raw := range values {
		change, _ := raw.(map[string]any)
		if path := firstString(change, "path"); path != "" {
			paths = append(paths, path)
		}
	}
	return strings.Join(paths, " ")
}

var skillBodyPattern = regexp.MustCompile(`(?i)(?:^|/)skills/([a-z0-9][a-z0-9._-]{0,127})/SKILL\.md$`)

// skillReadEvents records a shell read of a shipped skill body as a
// provisional "skill_read" event. Codex loads skill bodies by reading
// SKILL.md, so the mechanical read is the activation evidence; a text claim
// in a message never is. The trace-level pass in normalizeTrace promotes
// these to skill_selected and keeps only the first successful read per
// skill, because re-reading an already loaded body is ordinary behavior,
// not a second activation.
func skillReadEvents(command string, rc int) []actorEvent {
	segments, _ := normalizedCommandSegments(command)
	events := make([]actorEvent, 0, 1)
	seen := make(map[string]bool)
	for _, fields := range segments {
		for _, field := range fields {
			match := skillBodyPattern.FindStringSubmatch(filepath.ToSlash(field))
			if match == nil {
				continue
			}
			name := strings.ToLower(match[1])
			if seen[name] {
				continue
			}
			seen[name] = true
			events = append(events, actorEvent{Kind: "skill_read", Path: name, RC: rc})
		}
	}
	return events
}

// reconcileClaudeSpawns keeps lifecycle task events as the single source of
// spawn evidence when they exist; otherwise provisional tool-derived spawns
// are promoted so older single-source traces keep their evidence.
func reconcileClaudeSpawns(events []actorEvent) []actorEvent {
	lifecycleSpawns := false
	for _, event := range events {
		if event.Kind == "agent_spawn" {
			lifecycleSpawns = true
			break
		}
	}
	result := events[:0]
	for _, event := range events {
		if event.Kind == "agent_spawn_tool" {
			if lifecycleSpawns {
				continue
			}
			event.Kind = "agent_spawn"
		}
		result = append(result, event)
	}
	return result
}

func promoteSkillReads(events []actorEvent) []actorEvent {
	seenSuccessful := make(map[string]bool)
	result := events[:0]
	for _, event := range events {
		if event.Kind == "skill_read" {
			if event.RC == 0 {
				if seenSuccessful[event.Path] {
					continue
				}
				seenSuccessful[event.Path] = true
			}
			event.Kind = "skill_selected"
		}
		result = append(result, event)
	}
	return result
}

func deduplicateEvents(events []actorEvent) []actorEvent {
	result := make([]actorEvent, 0, len(events))
	for _, event := range events {
		if event.Kind == "" {
			continue
		}
		result = append(result, event)
	}
	return result
}

func removeTraceComplete(events []actorEvent) []actorEvent {
	result := events[:0]
	for _, event := range events {
		if event.Kind != "trace_complete" {
			result = append(result, event)
		}
	}
	return result
}

func redact(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}

func selftest() error {
	checks := []struct {
		name string
		run  func() error
	}{
		{"strict JSON rejects unknown fields and trailing data", selftestStrictJSON},
		{"request validation requires exact canonical roots", selftestExactRoots},
		{"request validation rejects symlinked and overlapping roots", selftestUnsafeRoots},
		{"credential hardlinks stay outside actor-visible roots", selftestCredentialHardlinks},
		{"subscription auth wins before explicit API-key fallback", selftestSubscriptionPreference},
		{"Claude subscription credentials stay outside actor tools", selftestClaudeSubscriptionIsolation},
		{"Codex external auth stays memory-only", selftestCodexExternalAuth},
		{"codex home accepts app-server state writes", selftestCodexHomeWritable},
		{"app-server rejects unsafe client authority", selftestAppServerAuthority},
		{"Codex subscription egress reaches only approved provider hosts", selftestRestrictedConnectProxy},
		{"child environment excludes inherited credentials", selftestEnvironment},
		{"credential proxy injects only the provider key upstream", selftestCredentialProxy},
		{"end-to-end fake actor satisfies schema version 2", selftestEndToEnd},
		{"incomplete actor trace forces infrastructure failure", selftestIncompleteEndToEnd},
		{"sandbox hides credential and sibling sentinels", selftestHiddenSentinels},
		{"sandbox keeps project writable and plugin read-only", selftestMountModes},
		{"actor network reaches only the local credential bridge", selftestActorNetwork},
		{"protected effect monitor catches restored mutations", selftestProtectedEffectMonitor},
		{"timeout terminates the isolated process tree", selftestTimeout},
		{"trace normalization requires a complete result", selftestTrace},
		{"Codex skills.read activation is normalized", selftestCodexSkillActivation},
		{"Codex shell reads of a skill body are normalized once", selftestCodexSkillShellReads},
		{"repeated Claude init inventories stay consistent", selftestRepeatedClaudeInit},
		{"skills catalog rendering is detected per harness", selftestSkillsCatalog},
		{"oracle phase excludes credentials and network", selftestOracle},
		{"response redaction removes credential values", selftestRedaction},
		{"arm inventory is exact", selftestInventory},
	}
	for _, check := range checks {
		if err := check.run(); err != nil {
			return fmt.Errorf("%s: %w", check.name, err)
		}
		fmt.Printf("ok   %s\n", check.name)
	}
	return nil
}

type selftestPaths struct {
	root, project, home, plugin, sibling, credential string
}

func selftestProxySocket(home string) string {
	return filepath.Join(home, "b", "p.sock")
}

func newSelftestPaths() (selftestPaths, func(), error) {
	root, err := os.MkdirTemp("", "mpb-selftest-")
	if err != nil {
		return selftestPaths{}, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	paths := selftestPaths{root: root, project: filepath.Join(root, "project"), home: filepath.Join(root, "home"), plugin: filepath.Join(root, "plugin"), sibling: filepath.Join(root, "sibling"), credential: filepath.Join(root, "credential")}
	for _, path := range []string{paths.project, paths.home, paths.plugin, paths.sibling} {
		if err := os.Mkdir(path, 0o700); err != nil {
			cleanup()
			return selftestPaths{}, nil, err
		}
	}
	if err := os.WriteFile(paths.credential, []byte("credential-sentinel-value\n"), 0o600); err != nil {
		cleanup()
		return selftestPaths{}, nil, err
	}
	if err := os.WriteFile(filepath.Join(paths.sibling, "secret"), []byte("sibling-sentinel\n"), 0o600); err != nil {
		cleanup()
		return selftestPaths{}, nil, err
	}
	return paths, cleanup, nil
}

func validSelftestRequest(paths selftestPaths) brokerRequest {
	return brokerRequest{SchemaVersion: "2", Harness: "claude", Model: "fake", Effort: "high", Arm: "treatment", Task: "test", Project: paths.project, ActorHome: paths.home, PluginRepo: paths.plugin, TaskReadRoots: []string{paths.project, paths.plugin}, TaskWriteRoots: []string{paths.project}, TimeoutMS: 1000}
}

func selftestStrictJSON() error {
	if _, err := decodeRequest(strings.NewReader(`{"schema_version":"2","unknown":true}`)); err == nil {
		return errors.New("unknown field accepted")
	}
	if _, err := decodeRequest(strings.NewReader(`{} {}`)); err == nil {
		return errors.New("trailing object accepted")
	}
	return nil
}

func selftestExactRoots() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	if err := validateRequest(&req); err != nil {
		return err
	}
	req = validSelftestRequest(paths)
	req.TaskReadRoots = []string{paths.project}
	if err := validateRequest(&req); err == nil {
		return errors.New("incomplete read roots accepted")
	}
	return nil
}

func selftestUnsafeRoots() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	link := filepath.Join(paths.root, "linked-project")
	if err := os.Symlink(paths.project, link); err != nil {
		return err
	}
	req := validSelftestRequest(paths)
	req.Project = link
	req.TaskReadRoots[0] = link
	if err := validateRequest(&req); err == nil {
		return errors.New("symlinked project accepted")
	}
	req = validSelftestRequest(paths)
	req.ActorHome = filepath.Join(paths.project, "nested-home")
	if err := os.Mkdir(req.ActorHome, 0o700); err != nil {
		return err
	}
	if err := validateRequest(&req); err == nil {
		return errors.New("overlapping home accepted")
	}
	if err := os.WriteFile(filepath.Join(paths.home, "preexisting"), []byte("state\n"), 0o600); err != nil {
		return err
	}
	req = validSelftestRequest(paths)
	if err := validateRequest(&req); err == nil {
		return errors.New("non-empty actor home accepted")
	}
	return nil
}

func selftestCredentialHardlinks() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	if err := os.Link(paths.credential, filepath.Join(paths.project, "credential-hardlink")); err != nil {
		return err
	}
	restore := setTestEnvironment("MEGAPOWERS_BROKER_CLAUDE_API_KEY_FILE", paths.credential)
	defer restore()
	req := validSelftestRequest(paths)
	if _, err := readProviderKey(req); err == nil {
		return errors.New("actor-visible credential hardlink accepted")
	}
	if !credentialPathIsRuntimeVisible("/usr/local/share/megapowers-key") || !credentialPathIsRuntimeVisible("/etc/megapowers-key") || credentialPathIsRuntimeVisible("/var/tmp/megapowers-key") {
		return errors.New("runtime credential mount classification failed")
	}
	return nil
}

func selftestSubscriptionPreference() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	subscriptionPath := filepath.Join(paths.root, "claude-subscription.json")
	subscriptionToken := "subscription-access-token-selftest-value"
	content, err := json.Marshal(map[string]any{"claudeAiOauth": map[string]any{"accessToken": subscriptionToken, "expiresAt": time.Now().Add(time.Hour).UnixMilli()}})
	if err != nil {
		return err
	}
	if err := os.WriteFile(subscriptionPath, content, 0o600); err != nil {
		return err
	}
	restoreSubscription := setTestEnvironment("MEGAPOWERS_BROKER_CLAUDE_CREDENTIALS_FILE", subscriptionPath)
	defer restoreSubscription()
	restoreKey := setTestEnvironment("MEGAPOWERS_BROKER_CLAUDE_API_KEY_FILE", paths.credential)
	defer restoreKey()
	restoreMode := setTestEnvironment("MEGAPOWERS_BROKER_AUTH_MODE", "")
	defer restoreMode()
	req := validSelftestRequest(paths)
	auth, err := resolveAuthentication(req)
	if err != nil || auth.mode != authSubscription || auth.credential != subscriptionToken {
		return errors.New("default authentication did not select the subscription")
	}
	_ = os.Setenv("MEGAPOWERS_BROKER_AUTH_MODE", authAPIKey)
	auth, err = resolveAuthentication(req)
	if err != nil || auth.mode != authAPIKey || auth.credential != "credential-sentinel-value" {
		return errors.New("explicit API-key fallback was not selected")
	}
	return nil
}

func selftestClaudeSubscriptionIsolation() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	credentialPath := filepath.Join(paths.root, "claude-subscription.json")
	realCredential := "claude-real-subscription-selftest-token"
	content, err := json.Marshal(map[string]any{"claudeAiOauth": map[string]any{"accessToken": realCredential, "expiresAt": time.Now().Add(time.Hour).UnixMilli()}})
	if err != nil {
		return err
	}
	if err := os.WriteFile(credentialPath, content, 0o600); err != nil {
		return err
	}
	restoreCredential := setTestEnvironment("MEGAPOWERS_BROKER_CLAUDE_CREDENTIALS_FILE", credentialPath)
	defer restoreCredential()
	restoreMode := setTestEnvironment("MEGAPOWERS_BROKER_AUTH_MODE", authSubscription)
	defer restoreMode()
	req := validSelftestRequest(paths)
	auth, err := resolveAuthentication(req)
	if err != nil {
		return err
	}
	proxySocket := selftestProxySocket(req.ActorHome)
	proxyDirectory := filepath.Dir(proxySocket)
	if err := os.Mkdir(proxyDirectory, 0o700); err != nil {
		return err
	}
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		upstreamCalls++
		if request.Header.Get("Authorization") != "Bearer "+realCredential || request.Header.Get("X-Api-Key") != "" || !strings.Contains(request.Header.Get("Anthropic-Beta"), "oauth-2025-04-20") || request.URL.RawQuery != "beta=true" {
			http.Error(w, "wrong subscription credential", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	proxy, err := startCredentialProxy("claude", authSubscription, auth.credential, upstream.URL, proxySocket)
	if err != nil {
		return err
	}
	defer proxy.close()
	bridge, err := startTCPUnixBridge(proxy.socket, "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer bridge.close()
	request, err := http.NewRequest(http.MethodPost, bridge.base+"/v1/messages?beta=true", strings.NewReader(`{}`))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+proxy.token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || upstreamCalls != 1 {
		return errors.New("subscription proxy did not swap the credential")
	}
	environment := actorEnvironment(req, map[string]string{"CLAUDE_CODE_OAUTH_TOKEN": proxy.token, "ANTHROPIC_BASE_URL": proxy.base})
	settings, err := writeClaudeSettings(req)
	if err != nil {
		return err
	}
	joined := strings.Join(environment, "\n") + "\n" + settings
	if strings.Contains(joined, realCredential) || strings.Contains(joined, credentialPath) {
		return errors.New("real Claude subscription credential entered actor inputs")
	}
	settingsContent, err := os.ReadFile(settings)
	if err != nil {
		return err
	}
	if !bytes.Contains(settingsContent, []byte("CLAUDE_CODE_OAUTH_TOKEN")) || !bytes.Contains(settingsContent, []byte(".credentials.json")) {
		return errors.New("claude settings do not protect credential paths and environment")
	}
	var settingsDocument struct {
		Permissions struct {
			DefaultMode string   `json:"defaultMode"`
			Allow       []string `json:"allow"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(settingsContent, &settingsDocument); err != nil {
		return err
	}
	allowed := make(map[string]bool, len(settingsDocument.Permissions.Allow))
	for _, tool := range settingsDocument.Permissions.Allow {
		allowed[tool] = true
	}
	if settingsDocument.Permissions.DefaultMode != "acceptEdits" || len(allowed) != 11 || !allowed["Agent"] || !allowed["Task"] || !allowed["Skill"] || !allowed["Read"] || !allowed["Glob"] || !allowed["Grep"] || !allowed["Bash"] || !allowed["Edit"] || !allowed["Write"] || !allowed["MultiEdit"] || !allowed["NotebookEdit"] {
		return errors.New("claude settings do not allow isolated reads, commands, edits, skills, and native delegation")
	}
	return nil
}

func selftestCodexExternalAuth() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	accessToken := selftestJWT(time.Now().Add(time.Hour))
	authPath := filepath.Join(paths.root, "codex-auth.json")
	content, err := json.Marshal(map[string]any{"auth_mode": "chatgpt", "tokens": map[string]any{"access_token": accessToken, "account_id": "account-selftest"}})
	if err != nil {
		return err
	}
	if err := os.WriteFile(authPath, content, 0o600); err != nil {
		return err
	}
	restoreAuth := setTestEnvironment("MEGAPOWERS_BROKER_CODEX_AUTH_FILE", authPath)
	defer restoreAuth()
	restoreMode := setTestEnvironment("MEGAPOWERS_BROKER_AUTH_MODE", authSubscription)
	defer restoreMode()
	req := validSelftestRequest(paths)
	req.Harness = "codex"
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		return err
	}
	auth, err := resolveAuthentication(req)
	if err != nil || auth.credential != accessToken || auth.accountID != "account-selftest" {
		return errors.New("codex subscription auth was not parsed")
	}
	if err := os.Mkdir(filepath.Join(req.ActorHome, ".codex"), 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(req.ActorHome, "codex-marketplace"), 0o700); err != nil {
		return err
	}
	args, err := codexAppServerSandboxArgs(req, "/usr/bin/true", filepath.Dir(selftestProxySocket(req.ActorHome)), toolchain)
	if err != nil {
		return err
	}
	environment := actorEnvironment(req, map[string]string{"CODEX_HOME": filepath.Join(req.ActorHome, ".codex")})
	joined := strings.Join(args, "\n") + "\n" + strings.Join(environment, "\n")
	if strings.Contains(joined, accessToken) || strings.Contains(joined, authPath) || strings.Contains(joined, auth.accountID) {
		return errors.New("codex subscription material entered arguments or environment")
	}
	leaked := false
	err = filepath.WalkDir(req.ActorHome, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		fileContent, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		leaked = leaked || bytes.Contains(fileContent, []byte(accessToken)) || bytes.Contains(fileContent, []byte(auth.accountID))
		return nil
	})
	if err != nil || leaked {
		return errors.New("codex subscription material was persisted in actor state")
	}
	fake := filepath.Join(paths.root, "fake-codex-app-server")
	script := `#!/bin/sh
read -r initialize
case "$initialize" in *'"method":"initialize"'*) ;; *) exit 41;; esac
printf '%s\n' '{"id":1,"result":{}}'
read -r initialized
case "$initialized" in *'"method":"initialized"'*) ;; *) exit 42;; esac
read -r login
case "$login" in *'"method":"account/login/start"'*'"accessToken":"` + accessToken + `"'*'"type":"chatgptAuthTokens"'*) ;; *) exit 43;; esac
printf '%s\n' '{"id":2,"result":{"type":"chatgptAuthTokens"}}'
read -r thread
case "$thread" in *'"method":"thread/start"'*) ;; *) exit 44;; esac
printf '%s\n' '{"id":3,"result":{"thread":{"id":"thread-selftest"}}}'
read -r turn
case "$turn" in *'"method":"turn/start"'*'"threadId":"thread-selftest"'*) ;; *) exit 45;; esac
printf '%s\n' '{"id":4,"result":{"turn":{"id":"turn-selftest","status":"inProgress","items":[]}}}'
printf '%s\n' '{"method":"item/completed","params":{"threadId":"thread-selftest","turnId":"turn-selftest","item":{"id":"message-selftest","type":"agentMessage","text":"subscription result"}}}'
printf '%s\n' '{"method":"turn/completed","params":{"threadId":"thread-selftest","turn":{"id":"turn-selftest","status":"completed","items":[],"error":null}}}'
sleep 2
`
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := runCodexAppServer(ctx, req, fake, filepath.Join(req.ActorHome, ".codex"), auth, paths.root, toolchain)
	if err != nil || result.rc != 0 || result.timedOut {
		return fmt.Errorf("mock Codex app-server run rc=%d: %w: %s", result.rc, err, result.stderr)
	}
	if bytes.Contains(result.stdout, []byte(accessToken)) || bytes.Contains(result.stderr, []byte(accessToken)) {
		return errors.New("codex app-server trace exposed the subscription token")
	}
	response, events, complete := normalizeTrace("codex", result.stdout, result.rc)
	if response != "subscription result" || !complete || len(events) != 1 || events[0].Kind != "trace_complete" {
		return errors.New("mock Codex app-server trace was not normalized")
	}
	leakingFake := filepath.Join(paths.root, "fake-codex-leaking-error")
	leakingScript := "#!/bin/sh\nprintf '%s\\n' " + strconv.Quote(accessToken) + " >&2\nexit 46\n"
	if err := os.WriteFile(leakingFake, []byte(leakingScript), 0o700); err != nil {
		return err
	}
	leakingResult, leakingErr := runCodexAppServer(ctx, req, leakingFake, filepath.Join(req.ActorHome, ".codex"), auth, paths.root, toolchain)
	if leakingErr == nil || bytes.Contains(leakingResult.stderr, []byte(accessToken)) || strings.Contains(leakingErr.Error(), accessToken) || !bytes.Contains(leakingResult.stderr, []byte("[REDACTED]")) {
		return errors.New("codex app-server failure exposed or omitted redaction of the subscription token")
	}

	diagnosticFake := filepath.Join(paths.root, "fake-codex-diagnostic")
	diagnosticScript := "#!/bin/sh\nread -r initialize\ncase \"$initialize\" in *'\"method\":\"initialize\"'*) ;; *) exit 41;; esac\nprintf '%s\\n' '{\"id\":1,\"result\":{}}'\nprintf '%s\\n' '{\"method\":\"item/completed\",\"params\":{\"threadId\":\"thread-diag\",\"turnId\":\"turn-diag\",\"item\":{\"id\":\"message-diag\",\"type\":\"agentMessage\",\"text\":\"diagnostic\"}}}'\necho APP_SERVER_DIAGNOSTIC_MARKER >&2\nexit 47\n"
	if err := os.WriteFile(diagnosticFake, []byte(diagnosticScript), 0o700); err != nil {
		return err
	}
	_, diagnosticErr := runCodexAppServer(ctx, req, diagnosticFake, filepath.Join(req.ActorHome, ".codex"), auth, paths.root, toolchain)
	if diagnosticErr == nil || !strings.Contains(diagnosticErr.Error(), "APP_SERVER_DIAGNOSTIC_MARKER") {
		return errors.New("app-server failure did not surface the app-server stderr tail")
	}

	dumpDir := filepath.Join(paths.root, "trace-dumps")
	restoreDump := setTestEnvironment("MEGAPOWERS_BROKER_TRACE_DUMP", dumpDir)
	_, _ = runCodexAppServer(ctx, req, diagnosticFake, filepath.Join(req.ActorHome, ".codex"), auth, paths.root, toolchain)
	restoreDump()
	dumpEntries, dumpReadErr := os.ReadDir(dumpDir)
	if dumpReadErr != nil || len(dumpEntries) == 0 {
		return errors.New("failed app-server traces were not persisted for diagnostics")
	}
	dumpedTrace, dumpTraceErr := os.ReadFile(filepath.Join(dumpDir, dumpEntries[0].Name()))
	if dumpTraceErr != nil || !bytes.Contains(dumpedTrace, []byte(`"method":"item/completed"`)) {
		return errors.New("persisted app-server trace dump is missing the event stream")
	}

	unexpectedScript := strings.Replace(script,
		`printf '%s\n' '{"id":1,"result":{}}'`,
		`printf '%s\n' '{"id":999,"result":{}}'
printf '%s\n' '{"id":1,"result":{}}'`, 1)
	if unexpectedScript == script {
		return errors.New("could not construct unexpected-response app-server fixture")
	}
	if err := os.WriteFile(fake, []byte(unexpectedScript), 0o700); err != nil {
		return err
	}
	unexpectedRoot, err := os.MkdirTemp("/tmp", "mp-id-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(unexpectedRoot)
	unexpectedActorHome := filepath.Join(unexpectedRoot, "home")
	if err := os.Mkdir(unexpectedActorHome, 0o700); err != nil {
		return err
	}
	unexpectedCodexHome := filepath.Join(unexpectedActorHome, ".codex")
	if err := os.Mkdir(unexpectedCodexHome, 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(unexpectedActorHome, "codex-marketplace"), 0o700); err != nil {
		return err
	}
	unexpectedReq := req
	unexpectedReq.ActorHome = unexpectedActorHome
	unexpectedResult, unexpectedErr := runCodexAppServer(ctx, unexpectedReq, fake, unexpectedCodexHome, auth, paths.root, toolchain)
	if unexpectedErr == nil || unexpectedResult.rc != 125 {
		return fmt.Errorf("unexpected app-server response was not rejected: rc=%d err=%v", unexpectedResult.rc, unexpectedErr)
	}
	return nil
}

func selftestJWT(expires time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, _ := json.Marshal(map[string]any{"exp": expires.Unix()})
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".signature-selftest"
}

func dumpDiagnosticTrace(dir string, name string, trace []byte) {
	if dir == "" || len(trace) == 0 {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	payload := append([]byte(nil), trace...)
	_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%d.jsonl", name, time.Now().UnixNano())), payload, 0o600)
}

func selftestCodexHomeWritable() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	req.Harness = "codex"
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(req.ActorHome, ".codex"), 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(req.ActorHome, "codex-marketplace"), 0o700); err != nil {
		return err
	}
	args, err := codexAppServerSandboxArgs(req, "/usr/bin/true", filepath.Dir(selftestProxySocket(req.ActorHome)), toolchain)
	if err != nil {
		return err
	}
	joined := strings.Join(args, " ")
	codexHome := filepath.Join(req.ActorHome, ".codex")
	if strings.Contains(joined, "--ro-bind "+codexHome+" "+codexHome) {
		return errors.New("codex home is mounted read-only; the app-server cannot initialize its state runtime")
	}
	if !strings.Contains(joined, "--bind "+codexHome+" "+codexHome) {
		return errors.New("codex home is not mounted for app-server state writes")
	}
	return nil
}

func selftestAppServerAuthority() error {
	if !codexAppServerRequestAllowed("account/chatgptAuthTokens/refresh") {
		return errors.New("credential refresh was not recognized")
	}
	for _, method := range []string{"process/spawn", "thread/shellCommand", "item/commandExecution/requestApproval", "item/fileChange/requestApproval", "item/tool/call"} {
		if codexAppServerRequestAllowed(method) {
			return fmt.Errorf("unsafe app-server request %q was allowed", method)
		}
	}
	return nil
}

func selftestRestrictedConnectProxy() error {
	upstream, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer upstream.Close()
	echoDone := make(chan error, 1)
	go func() {
		connection, acceptErr := upstream.Accept()
		if acceptErr != nil {
			echoDone <- acceptErr
			return
		}
		defer connection.Close()
		buffer := make([]byte, 4)
		if _, readErr := io.ReadFull(connection, buffer); readErr != nil {
			echoDone <- readErr
			return
		}
		_, writeErr := connection.Write(buffer)
		echoDone <- writeErr
	}()
	directory, err := os.MkdirTemp("", "mpb-egress-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	socket := filepath.Join(directory, "egress.sock")
	allowedHost := strings.ToLower(upstream.Addr().String())
	proxy, err := startRestrictedConnectProxy(socket, map[string]bool{allowedHost: true})
	if err != nil {
		return err
	}
	defer proxy.close()
	bridge, err := startTCPUnixBridge(socket, "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer bridge.close()
	connection, err := net.Dial("tcp", strings.TrimPrefix(bridge.base, "http://"))
	if err != nil {
		return err
	}
	reader := bufio.NewReader(connection)
	if _, err := fmt.Fprintf(connection, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", allowedHost, allowedHost); err != nil {
		_ = connection.Close()
		return err
	}
	status, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(status, " 200 ") {
		_ = connection.Close()
		return errors.New("approved CONNECT target was rejected")
	}
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			_ = connection.Close()
			return readErr
		}
		if line == "\r\n" {
			break
		}
	}
	if _, err := connection.Write([]byte("ping")); err != nil {
		_ = connection.Close()
		return err
	}
	echo := make([]byte, 4)
	if _, err := io.ReadFull(reader, echo); err != nil || string(echo) != "ping" {
		_ = connection.Close()
		return errors.New("approved CONNECT tunnel did not carry bytes")
	}
	_ = connection.Close()
	if err := <-echoDone; err != nil {
		return err
	}

	denied, err := net.Dial("tcp", strings.TrimPrefix(bridge.base, "http://"))
	if err != nil {
		return err
	}
	defer denied.Close()
	if _, err := fmt.Fprint(denied, "CONNECT example.invalid:443 HTTP/1.1\r\nHost: example.invalid:443\r\n\r\n"); err != nil {
		return err
	}
	deniedStatus, err := bufio.NewReader(denied).ReadString('\n')
	if err != nil || !strings.Contains(deniedStatus, " 403 ") {
		return errors.New("unapproved CONNECT target was not rejected")
	}
	return nil
}

func selftestEnvironment() error {
	env := actorEnvironment(brokerRequest{ActorHome: "/tmp/actor"}, map[string]string{"OPENAI_API_KEY": "ephemeral"})
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, name := range []string{"AWS_SECRET_ACCESS_KEY", "SSH_AUTH_SOCK", "GITHUB_TOKEN", "ANTHROPIC_API_KEY"} {
		if strings.Contains(joined, "\n"+name+"=") {
			return fmt.Errorf("inherited %s", name)
		}
	}
	return nil
}

func selftestCredentialProxy() error {
	providerKey := "provider-key-selftest"
	socketDirectory, err := os.MkdirTemp("", "mpb-proxy-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(socketDirectory)
	socketPath := filepath.Join(socketDirectory, "provider.sock")
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		if r.Header.Get("X-Api-Key") != providerKey || r.Header.Get("Authorization") != "" {
			http.Error(w, "wrong credential", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" || r.URL.RawQuery != "" || r.Header.Get("X-Not-Allowed") != "" {
			http.Error(w, "proxy forwarded an invalid request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer upstream.Close()
	proxy, err := startCredentialProxy("claude", authAPIKey, providerKey, upstream.URL, socketPath)
	if err != nil {
		return err
	}
	defer proxy.close()
	bridge, err := startTCPUnixBridge(socketPath, "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer bridge.close()

	request, err := http.NewRequest(http.MethodPost, bridge.base+"/v1/messages", strings.NewReader(`{}`))
	if err != nil {
		return err
	}
	request.Header.Set("X-Api-Key", proxy.token)
	request.Header.Set("X-Not-Allowed", "must-not-reach-provider")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || upstreamCalls != 1 {
		return fmt.Errorf("authorized proxy request returned %d", response.StatusCode)
	}

	denied := []struct {
		method, target, token string
	}{
		{http.MethodPost, bridge.base + "/v1/messages", "wrong-token"},
		{http.MethodDelete, bridge.base + "/v1/messages", proxy.token},
		{http.MethodPost, bridge.base + "/v1/unknown", proxy.token},
		{http.MethodPost, bridge.base + "/v1/messages?debug=1", proxy.token},
	}
	for _, test := range denied {
		request, err = http.NewRequest(test.method, test.target, strings.NewReader(`{}`))
		if err != nil {
			return err
		}
		request.Header.Set("X-Api-Key", test.token)
		response, err = http.DefaultClient.Do(request)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusForbidden {
			return fmt.Errorf("proxy accepted %s %s", test.method, test.target)
		}
	}
	if upstreamCalls != 1 {
		return fmt.Errorf("proxy forwarded %d requests; require one", upstreamCalls)
	}
	return nil
}

func selftestEndToEnd() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	if err := os.Remove(paths.home); err != nil {
		return err
	}
	paths.home = filepath.Join(paths.root, strings.Repeat("long-actor-home-", 6))
	if err := os.Mkdir(paths.home, 0o700); err != nil {
		return err
	}
	if len(selftestProxySocket(paths.home)) <= 108 {
		return errors.New("long actor-home fixture does not exceed Linux Unix-socket path capacity")
	}
	fake := filepath.Join(paths.root, "fake-claude")
	initEvent, err := json.Marshal(map[string]any{"type": "system", "subtype": "init", "plugins": []any{map[string]any{"name": "megapowers", "path": paths.plugin}}, "plugin_errors": []any{}, "skills": []any{"deep-research", "megapowers:orchestrating", "megapowers:safe-effects"}})
	if err != nil {
		return err
	}
	script := "#!/bin/sh\nif [ \"${1-}\" = --version ]; then echo fake-claude-1; exit 0; fi\nprintf '%s\\n' " + strconv.Quote(string(initEvent)) + " '{\"type\":\"result\",\"is_error\":false,\"result\":\"fake result\"}'\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		return err
	}
	restoreBinary := setTestEnvironment("MEGAPOWERS_BROKER_CLAUDE_BIN", fake)
	defer restoreBinary()
	restoreCredential := setTestEnvironment("MEGAPOWERS_BROKER_CLAUDE_API_KEY_FILE", paths.credential)
	defer restoreCredential()
	restoreMode := setTestEnvironment("MEGAPOWERS_BROKER_AUTH_MODE", authAPIKey)
	defer restoreMode()
	req := validSelftestRequest(paths)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := execute(ctx, req)
	if err != nil {
		return err
	}
	if response.SchemaVersion != "2" || response.CLIVersion != "fake-claude-1" || response.Response != "fake result" || response.RC != 0 || len(response.Events) != 1 || response.Events[0].Kind != "trace_complete" || len(response.PluginInventory) != 1 || response.PluginInventory[0] != "megapowers" || response.Isolation.Boundary != "bwrap" || response.Isolation.CredentialsReadableByActor || response.Isolation.SiblingStateReadableByActor {
		return errors.New("end-to-end response does not satisfy schema version 2")
	}
	if response.SkillsCatalog == nil || !response.SkillsCatalog.Rendered || response.SkillsCatalog.Source != catalogSourceClaudeInit || strings.Join(response.SkillsCatalog.Skills, ",") != "orchestrating,safe-effects" {
		return fmt.Errorf("end-to-end response did not assert the Claude skills catalog: %+v", response.SkillsCatalog)
	}
	return nil
}

func selftestSkillsCatalog() error {
	initWithSkills := []byte(`{"type":"system","subtype":"init","plugins":[],"skills":["deep-research","megapowers:orchestrating","megapowers:humanizing-prose"],"slash_commands":["megapowers:orchestrating"]}` + "\n")
	catalog := claudeSkillsCatalog(initWithSkills)
	if !catalog.Rendered || catalog.Source != catalogSourceClaudeInit || strings.Join(catalog.Skills, ",") != "humanizing-prose,orchestrating" {
		return fmt.Errorf("claude init skills were not detected: %+v", catalog)
	}
	catalog = claudeSkillsCatalog([]byte(`{"type":"system","subtype":"init","plugins":[],"slash_commands":["clear","megapowers:safe-effects"]}` + "\n"))
	if !catalog.Rendered || catalog.Source != catalogSourceClaudeSlashCommands || strings.Join(catalog.Skills, ",") != "safe-effects" {
		return fmt.Errorf("claude slash-command fallback was not detected: %+v", catalog)
	}
	catalog = claudeSkillsCatalog([]byte(`{"type":"system","subtype":"init","plugins":[],"skills":["deep-research"]}` + "\n"))
	if catalog.Rendered || catalog.Source != catalogSourceClaudeInit || len(catalog.Skills) != 0 {
		return fmt.Errorf("claude init without plugin skills counted as rendered: %+v", catalog)
	}
	catalog = claudeSkillsCatalog([]byte(`{"type":"system","subtype":"init","plugins":[]}` + "\n"))
	if catalog.Rendered || catalog.Source != catalogSourceUnavailable {
		return fmt.Errorf("claude init without a skill listing was not reported unavailable: %+v", catalog)
	}

	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	codexHome := filepath.Join(paths.home, ".codex")
	installed := filepath.Join(codexHome, "plugins", "cache", "megapowers-eval", "megapowers", "0.26.1")
	// Shape observed in a codex-cli 0.152.0 rollout: the developer message
	// renders roots as `rN` aliases and each skill as one "- name: ... (file: rN/...)" line.
	block := "<skills_instructions>\n## Skills\nA skill is a set of local instructions to follow that is stored in a `SKILL.md` file.\n### Skill roots\n- `r0` = `" + filepath.Join(codexHome, "skills", ".system") + "`\n- `r1` = `" + filepath.Join(installed, "skills") + "`\n### Available skills\n- imagegen: Generate or edit raster images (file: r0/imagegen/SKILL.md)\n- megapowers:orchestrating: Use when two or more independent lanes can run in parallel (file: r1/orchestrating/SKILL.md)\n- safe-effects: Use when preparing a deploy: with (file: parens) inside (file: r1/safe-effects/SKILL.md)\n- skill-creator: Create skills (file: r0/skill-creator/SKILL.md)\n</skills_instructions>"
	developerLine := func(text string) string {
		// Codex's serializer writes angle brackets literally; keep the
		// fixture faithful rather than HTML-escaped.
		var content bytes.Buffer
		encoder := json.NewEncoder(&content)
		encoder.SetEscapeHTML(false)
		_ = encoder.Encode(map[string]any{"timestamp": "2026-09-01T21:04:25.182Z", "ordinal": 2, "type": "response_item", "payload": map[string]any{"type": "message", "id": "msg-selftest", "role": "developer", "content": []any{map[string]any{"type": "input_text", "text": text}}}})
		return content.String()
	}
	sessionMeta := `{"timestamp":"2026-09-01T21:04:22.484Z","ordinal":0,"type":"session_meta","payload":{"id":"thread-selftest","cwd":"/project"}}` + "\n"
	writeRollout := func(name, content string) error {
		directory := filepath.Join(codexHome, "sessions", "2026", "09", "01")
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600)
	}
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		return err
	}
	detected, firstBlock, err := codexSkillsCatalog(codexHome, installed)
	if err != nil || detected.Rendered || detected.Source != catalogSourceCodexRollout || len(detected.Skills) != 0 || firstBlock != "" {
		return fmt.Errorf("missing Codex rollout counted as rendered: %+v %v", detected, err)
	}
	if err := writeRollout("rollout-2026-09-01T21-04-18-thread-selftest.jsonl", sessionMeta+developerLine("## Memory\nunrelated developer context\n")); err != nil {
		return err
	}
	detected, _, err = codexSkillsCatalog(codexHome, installed)
	if err != nil || detected.Rendered || len(detected.Skills) != 0 {
		return fmt.Errorf("codex rollout without a skills block counted as rendered: %+v %v", detected, err)
	}
	systemOnly := strings.ReplaceAll(strings.ReplaceAll(block, "- megapowers:orchestrating: Use when two or more independent lanes can run in parallel (file: r1/orchestrating/SKILL.md)\n", ""), "- safe-effects: Use when preparing a deploy: with (file: parens) inside (file: r1/safe-effects/SKILL.md)\n", "")
	if err := writeRollout("rollout-2026-09-01T21-04-18-thread-selftest.jsonl", sessionMeta+developerLine(systemOnly)); err != nil {
		return err
	}
	detected, firstBlock, err = codexSkillsCatalog(codexHome, installed)
	if err != nil || detected.Rendered || len(detected.Skills) != 0 || firstBlock != systemOnly {
		return fmt.Errorf("codex catalog without plugin skills counted as rendered: %+v %v", detected, err)
	}
	if err := writeRollout("rollout-2026-09-01T21-04-18-thread-selftest.jsonl", sessionMeta+developerLine("## Memory\nunrelated\n")+developerLine(block+"\n<permissions instructions>\nlater context")); err != nil {
		return err
	}
	detected, firstBlock, err = codexSkillsCatalog(codexHome, installed)
	if err != nil || !detected.Rendered || detected.Source != catalogSourceCodexRollout || strings.Join(detected.Skills, ",") != "orchestrating,safe-effects" || firstBlock != block {
		return fmt.Errorf("codex rendered catalog was not detected: %+v %v", detected, err)
	}
	// A skill line that only names a plugin skill but resolves outside the
	// verified cache is not the installed candidate.
	foreign := strings.ReplaceAll(block, "- `r1` = `"+filepath.Join(installed, "skills")+"`", "- `r1` = `/elsewhere/megapowers/skills`")
	if err := writeRollout("rollout-2026-09-01T21-04-18-thread-selftest.jsonl", sessionMeta+developerLine(foreign)); err != nil {
		return err
	}
	detected, _, err = codexSkillsCatalog(codexHome, installed)
	if err != nil || detected.Rendered || len(detected.Skills) != 0 {
		return fmt.Errorf("codex catalog from a foreign plugin root counted as rendered: %+v %v", detected, err)
	}
	if err := writeRollout("rollout-2026-09-01T21-04-18-thread-selftest.jsonl", sessionMeta+developerLine(block)); err != nil {
		return err
	}
	if control, _, err := codexSkillsCatalog(codexHome, ""); err != nil || control.Rendered || len(control.Skills) != 0 {
		return errors.New("catalog without an installed plugin attributed skills to Megapowers")
	}
	traceLine := skillsCatalogTraceLine(detected, block)
	_, events, complete := normalizeTrace("codex", append([]byte(`{"method":"turn/completed","params":{"turn":{"status":"completed"}}}`+"\n"), traceLine...), 0)
	if !complete || len(events) != 1 || events[0].Kind != "trace_complete" {
		return fmt.Errorf("skills catalog trace line produced actor evidence: %+v", events)
	}

	// The fake app-server writes its rollout inside the Bubblewrap boundary,
	// proving the disposable CODEX_HOME sessions directory is where the
	// broker looks after the turn.
	accessToken := selftestJWT(time.Now().Add(time.Hour))
	auth := authentication{mode: authSubscription, credential: accessToken, accountID: "account-selftest"}
	req := validSelftestRequest(paths)
	req.Harness = "codex"
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(codexHome, "sessions")); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(req.ActorHome, "codex-marketplace"), 0o700); err != nil {
		return err
	}
	rolloutLine := developerLine(block)
	fake := filepath.Join(paths.root, "fake-codex-catalog-app-server")
	script := "#!/bin/sh\n" +
		"read -r initialize\ncase \"$initialize\" in *'\"method\":\"initialize\"'*) ;; *) exit 41;; esac\n" +
		"printf '%s\\n' '{\"id\":1,\"result\":{}}'\n" +
		"read -r initialized\nread -r login\nprintf '%s\\n' '{\"id\":2,\"result\":{\"type\":\"chatgptAuthTokens\"}}'\n" +
		"read -r thread\ncase \"$thread\" in *'\"method\":\"thread/start\"'*'\"ephemeral\":false'*) ;; *) exit 44;; esac\n" +
		"mkdir -p \"$CODEX_HOME/sessions/2026/09/01\"\n" +
		// Base64 keeps the rendered backticks out of shell quoting.
		"printf '%s' '" + base64.StdEncoding.EncodeToString([]byte(sessionMeta+rolloutLine)) + "' | base64 -d > \"$CODEX_HOME/sessions/2026/09/01/rollout-2026-09-01T21-04-18-thread-selftest.jsonl\"\n" +
		"printf '%s\\n' '{\"id\":3,\"result\":{\"thread\":{\"id\":\"thread-selftest\"}}}'\n" +
		"read -r turn\nprintf '%s\\n' '{\"id\":4,\"result\":{\"turn\":{\"id\":\"turn-selftest\",\"status\":\"inProgress\",\"items\":[]}}}'\n" +
		"printf '%s\\n' '{\"method\":\"item/completed\",\"params\":{\"threadId\":\"thread-selftest\",\"turnId\":\"turn-selftest\",\"item\":{\"id\":\"message-selftest\",\"type\":\"agentMessage\",\"text\":\"catalog result\"}}}'\n" +
		"printf '%s\\n' '{\"method\":\"turn/completed\",\"params\":{\"threadId\":\"thread-selftest\",\"turn\":{\"id\":\"turn-selftest\",\"status\":\"completed\",\"items\":[],\"error\":null}}}'\n" +
		"sleep 2\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := runCodexAppServer(ctx, req, fake, codexHome, auth, paths.root, toolchain)
	if err != nil || result.rc != 0 {
		return fmt.Errorf("fake catalog app-server rc=%d: %w: %s", result.rc, err, result.stderr)
	}
	detected, firstBlock, err = codexSkillsCatalog(codexHome, installed)
	if err != nil || !detected.Rendered || strings.Join(detected.Skills, ",") != "orchestrating,safe-effects" || firstBlock != block {
		written := make([]string, 0)
		_ = filepath.WalkDir(codexHome, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr == nil && !entry.IsDir() {
				content, _ := os.ReadFile(path)
				written = append(written, fmt.Sprintf("%s(%d bytes: %.120q)", strings.TrimPrefix(path, codexHome), len(content), content))
			}
			return nil
		})
		return fmt.Errorf("rollout written inside the sandbox was not detected: %+v %v; files: %s; stderr: %s", detected, err, strings.Join(written, ", "), result.stderr)
	}
	return nil
}

func selftestIncompleteEndToEnd() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	fake := filepath.Join(paths.root, "fake-incomplete-claude")
	initEvent, err := json.Marshal(map[string]any{"type": "system", "subtype": "init", "plugins": []any{map[string]any{"name": "megapowers", "path": paths.plugin}}})
	if err != nil {
		return err
	}
	script := "#!/bin/sh\nif [ \"${1-}\" = --version ]; then echo fake-claude-1; exit 0; fi\nprintf '%s\\n' " + strconv.Quote(string(initEvent)) + "\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		return err
	}
	restoreBinary := setTestEnvironment("MEGAPOWERS_BROKER_CLAUDE_BIN", fake)
	defer restoreBinary()
	restoreCredential := setTestEnvironment("MEGAPOWERS_BROKER_CLAUDE_API_KEY_FILE", paths.credential)
	defer restoreCredential()
	restoreMode := setTestEnvironment("MEGAPOWERS_BROKER_AUTH_MODE", authAPIKey)
	defer restoreMode()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := execute(ctx, validSelftestRequest(paths))
	if err != nil {
		return err
	}
	if response.RC != 125 {
		return fmt.Errorf("incomplete trace returned rc=%d", response.RC)
	}

	dumpDir := filepath.Join(paths.root, "claude-trace-dumps")
	restoreDump := setTestEnvironment("MEGAPOWERS_BROKER_TRACE_DUMP", dumpDir)
	if err := os.RemoveAll(filepath.Join(paths.home, ".claude")); err != nil {
		restoreDump()
		return err
	}
	_, dumpErr := execute(ctx, validSelftestRequest(paths))
	restoreDump()
	if dumpErr != nil {
		return fmt.Errorf("dumped claude trace run failed: %w", dumpErr)
	}
	dumpEntries, dumpReadErr := os.ReadDir(dumpDir)
	if dumpReadErr != nil || len(dumpEntries) == 0 {
		return errors.New("failed claude traces were not persisted for diagnostics")
	}
	dumpedTrace, dumpTraceErr := os.ReadFile(filepath.Join(dumpDir, dumpEntries[0].Name()))
	if dumpTraceErr != nil || !bytes.Contains(dumpedTrace, initEvent) {
		return errors.New("persisted claude trace dump is missing the event stream")
	}
	for _, event := range response.Events {
		if event.Kind == "trace_complete" {
			return errors.New("incomplete trace retained completion event")
		}
	}
	return nil
}

func selftestRepeatedClaudeInit() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	initEvent, err := json.Marshal(map[string]any{
		"type": "system", "subtype": "init",
		"plugins":       []any{map[string]any{"name": "megapowers", "path": paths.plugin}},
		"plugin_errors": []any{},
	})
	if err != nil {
		return err
	}
	trace := append(append(append([]byte(nil), initEvent...), '\n'), initEvent...)
	inventory, err := observedClaudeInventory(trace, req)
	if err != nil || len(inventory) != 1 || inventory[0] != "megapowers" {
		return fmt.Errorf("consistent repeated init was rejected: %w", err)
	}
	conflictingEvent, err := json.Marshal(map[string]any{
		"type": "system", "subtype": "init", "plugins": []any{}, "plugin_errors": []any{},
	})
	if err != nil {
		return err
	}
	trace = append(append(append([]byte(nil), initEvent...), '\n'), conflictingEvent...)
	if _, err := observedClaudeInventory(trace, req); err == nil {
		return errors.New("conflicting repeated init was accepted")
	}
	return nil
}

func setTestEnvironment(name, value string) func() {
	previous, existed := os.LookupEnv(name)
	_ = os.Setenv(name, value)
	return func() {
		if existed {
			_ = os.Setenv(name, previous)
		} else {
			_ = os.Unsetenv(name)
		}
	}
}

func selftestHiddenSentinels() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	args := sandboxBase(false)
	args = appendMount(args, req.ActorHome, false)
	args = appendMount(args, req.Project, false)
	args = appendMount(args, req.PluginRepo, true)
	command := fmt.Sprintf("test ! -e %q && test ! -e %q", paths.credential, paths.sibling)
	args = append(args, "--chdir", req.Project, "--", "/usr/bin/bash", "-c", command)
	result, runErr := runProcess(context.Background(), "bwrap", args, req.Project, minimalEnvironment(req.ActorHome), nil)
	if runErr != nil || result.rc != 0 {
		return fmt.Errorf("sentinel probe rc=%d: %w: %s", result.rc, runErr, result.stderr)
	}
	return nil
}

func selftestMountModes() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	req := validSelftestRequest(paths)
	req.Harness = "codex"
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		return err
	}
	proxySocket := selftestProxySocket(req.ActorHome)
	proxyDirectory := filepath.Dir(proxySocket)
	if err := os.Mkdir(proxyDirectory, 0o700); err != nil {
		return err
	}
	codexHome := filepath.Join(req.ActorHome, ".codex")
	if err := os.Mkdir(codexHome, 0o700); err != nil {
		return err
	}
	registry := filepath.Join(codexHome, "plugin-registry")
	if err := os.WriteFile(registry, []byte("immutable\n"), 0o400); err != nil {
		return err
	}
	marketplace := filepath.Join(req.ActorHome, "codex-marketplace")
	if err := os.Mkdir(marketplace, 0o700); err != nil {
		return err
	}
	command := fmt.Sprintf("touch %q && ! touch %q && ! touch %q && ! chmod 700 %q && ! touch %q", filepath.Join(req.Project, "written"), filepath.Join(req.PluginRepo, "forbidden"), filepath.Join(codexHome, "forbidden"), registry, filepath.Join(marketplace, "forbidden"))
	args, err := actorSandboxArgs(req, "/usr/bin/bash", "/opt/megapowers/codex", []string{"-c", command}, proxySocket, toolchain)
	if err != nil {
		return err
	}
	result, runErr := runProcess(context.Background(), "bwrap", args, req.Project, minimalEnvironment(req.ActorHome), nil)
	if runErr != nil || result.rc != 0 {
		return fmt.Errorf("mount probe rc=%d: %w: %s", result.rc, runErr, result.stderr)
	}
	if _, err := os.Stat(filepath.Join(req.Project, "written")); err != nil {
		return err
	}
	return nil
}

func selftestActorNetwork() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	req := validSelftestRequest(paths)
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		return err
	}
	proxySocket := selftestProxySocket(req.ActorHome)
	proxyDirectory := filepath.Dir(proxySocket)
	if err := os.Mkdir(proxyDirectory, 0o700); err != nil {
		return err
	}
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls++
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer upstream.Close()
	proxy, err := startCredentialProxy("claude", authAPIKey, "provider-key-selftest", upstream.URL, proxySocket)
	if err != nil {
		return err
	}
	defer proxy.close()
	command := fmt.Sprintf("! timeout 1 bash -c 'echo test >/dev/tcp/127.0.0.1/%d' && ! unlink %q && curl --fail --silent --show-error -X POST -H \"X-Api-Key: ${ANTHROPIC_API_KEY}\" -d '{}' \"${ANTHROPIC_BASE_URL}/v1/messages\" >/dev/null", port, filepath.Join(actorBrokerPath, filepath.Base(proxySocket)))
	args, err := actorSandboxArgs(req, "/usr/bin/bash", "/opt/megapowers/claude", []string{"-c", command}, proxySocket, toolchain)
	if err != nil {
		return err
	}
	environment := actorEnvironment(req, map[string]string{"ANTHROPIC_API_KEY": proxy.token, "ANTHROPIC_BASE_URL": proxy.base})
	result, runErr := runProcess(context.Background(), "bwrap", args, req.Project, environment, nil)
	if runErr != nil || result.rc != 0 {
		return fmt.Errorf("actor network probe rc=%d: %w: %s", result.rc, runErr, result.stderr)
	}
	if upstreamCalls != 1 {
		return fmt.Errorf("actor proxy bridge forwarded %d requests", upstreamCalls)
	}
	return nil
}

func selftestProtectedEffectMonitor() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	trackerDirectory := filepath.Join(paths.project, ".tracker")
	if err := os.Mkdir(trackerDirectory, 0o700); err != nil {
		return err
	}
	targets := []string{filepath.Join(trackerDirectory, "issue.json"), filepath.Join(trackerDirectory, "pull-request.json")}
	for _, target := range targets {
		if err := os.WriteFile(target, []byte("original\n"), 0o600); err != nil {
			return err
		}
	}
	monitor, err := startProtectedEffectMonitor(paths.project)
	if err != nil {
		return err
	}
	defer monitor.close()
	for _, target := range targets {
		if err := os.WriteFile(target, []byte("attempted\n"), 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte("original\n"), 0o600); err != nil {
			return err
		}
	}
	events := monitor.events()
	if len(events) != 2 || events[0].Kind != "tracker_comment" || events[1].Kind != "pr_comment" {
		return fmt.Errorf("protected effect monitor returned %+v", events)
	}
	monitor.close()
	for _, target := range targets {
		if err := os.Remove(target); err != nil {
			return err
		}
	}
	missingMonitor, err := startProtectedEffectMonitor(paths.project)
	if err != nil {
		return err
	}
	defer missingMonitor.close()
	if err := os.WriteFile(targets[0], []byte("created\n"), 0o600); err != nil {
		return err
	}
	missingEvents := missingMonitor.events()
	foundTracker := false
	for _, event := range missingEvents {
		foundTracker = foundTracker || event.Kind == "tracker_comment"
	}
	if !foundTracker {
		return errors.New("protected effect monitor missed target creation")
	}
	return nil
}

func selftestTimeout() error {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	result, _ := runProcess(ctx, "/usr/bin/bash", []string{"-c", "sleep 30 & wait"}, "/tmp", minimalEnvironment("/tmp"), nil)
	if !result.timedOut || result.rc != 124 || time.Since(started) > 3*time.Second {
		return fmt.Errorf("timeout result=%+v", result)
	}
	return nil
}

func selftestTrace() error {
	initEvent := "{\"type\":\"system\",\"subtype\":\"init\"}\n"
	toolUse := "{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"tool_use\",\"id\":\"tool-1\",\"name\":\"Skill\",\"input\":{\"skill\":\"orchestrating\"}}]}}\n"
	toolResult := "{\"type\":\"user\",\"message\":{\"content\":[{\"type\":\"tool_result\",\"tool_use_id\":\"tool-1\",\"is_error\":false}]}}\n"
	terminalResult := "{\"type\":\"result\",\"is_error\":false,\"result\":\"done\"}\n"
	claude := []byte(initEvent + toolUse + toolResult + terminalResult)
	response, events, complete := normalizeTrace("claude", claude, 0)
	if response != "done" || !complete || len(events) != 2 || events[0].Kind != "skill_selected" || events[1].Kind != "trace_complete" {
		return errors.New("complete Claude trace not normalized")
	}
	hookPrefix := "{\"type\":\"system\",\"subtype\":\"hook_started\"}\n{\"type\":\"system\",\"subtype\":\"hook_response\"}\n"
	response, events, complete = normalizeTrace("claude", []byte(hookPrefix+initEvent+terminalResult), 0)
	if response != "done" || !complete || len(events) != 1 || events[0].Kind != "trace_complete" {
		return errors.New("claude pre-init hook lifecycle was not normalized")
	}
	enrichedHookPrefix := "{\"type\":\"system\",\"subtype\":\"hook_started\",\"item\":{\"type\":\"file_change\",\"path\":\"forged.txt\"},\"message\":{\"content\":[{\"type\":\"tool_use\",\"id\":\"hook-tool\",\"name\":\"Skill\",\"input\":{\"skill\":\"orchestrating\"}}]}}\n{\"type\":\"system\",\"subtype\":\"hook_response\",\"message\":{\"content\":[{\"type\":\"tool_result\",\"tool_use_id\":\"hook-tool\",\"is_error\":false}]}}\n"
	response, events, complete = normalizeTrace("claude", []byte(enrichedHookPrefix+initEvent+terminalResult), 0)
	if response != "done" || !complete || len(events) != 1 || events[0].Kind != "trace_complete" {
		return errors.New("claude pre-init hook payload produced actor evidence")
	}
	// CLI 2.1.251 fan-out shape observed live 2026-08-31: parallel subagents
	// forward as consecutive permissioned inits, and every result — main
	// first, then one per forwarded segment — arrives batched at the end.
	batchedFanOut := initEvent +
		"{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"tool_use\",\"id\":\"toolu-a\",\"name\":\"Task\",\"input\":{\"description\":\"Sweep A\"}}]}}\n" +
		"{\"type\":\"user\",\"tool_use_result\":{\"type\":\"spawn\"},\"message\":{\"content\":[{\"type\":\"tool_result\",\"tool_use_id\":\"toolu-a\"}]}}\n" +
		"{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"agent-a\",\"tool_use_id\":\"toolu-a\",\"task_type\":\"local_agent\"}\n" +
		"{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"agent-b\",\"task_type\":\"local_agent\"}\n" +
		"{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"agent-a\",\"status\":\"completed\"}\n" +
		initEvent +
		"{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"agent-b\",\"status\":\"completed\"}\n" +
		initEvent +
		"{\"type\":\"result\",\"is_error\":false,\"result\":\"merged answer\"}\n" +
		"{\"type\":\"result\",\"is_error\":false,\"result\":\"agent-a output\",\"origin\":{\"kind\":\"task-notification\"}}\n" +
		"{\"type\":\"result\",\"is_error\":false,\"result\":\"agent-b output\",\"origin\":{\"kind\":\"task-notification\"}}\n"
	response, events, complete = normalizeTrace("claude", []byte(batchedFanOut), 0)
	spawns := 0
	completes := 0
	for _, event := range events {
		if event.Kind == "agent_spawn" {
			spawns++
		}
		if event.Kind == "agent_complete" {
			completes++
		}
	}
	if !complete || response != "merged answer" || spawns != 2 || completes != 2 || events[len(events)-1].Kind != "trace_complete" {
		return fmt.Errorf("batched parallel fan-out trace not normalized: complete=%v response=%q spawns=%d completes=%d", complete, response, spawns, completes)
	}
	taskNotificationResult := "{\"type\":\"result\",\"is_error\":false,\"result\":\"resumed\",\"origin\":{\"kind\":\"task-notification\"}}\n"
	forwardLifecycle := "{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"agent-forwarded\",\"task_type\":\"local_agent\"}\n{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"agent-forwarded\",\"status\":\"completed\"}\n"
	response, events, complete = normalizeTrace("claude", []byte(initEvent+forwardLifecycle+initEvent+terminalResult+taskNotificationResult), 0)
	if response != "resumed" || !complete || len(events) != 3 || events[0].Kind != "agent_spawn" || events[1].Kind != "agent_complete" || events[2].Kind != "trace_complete" {
		return errors.New("claude task-notification resume trace not normalized")
	}
	response, events, complete = normalizeTrace("claude", []byte(initEvent+forwardLifecycle+terminalResult+initEvent+terminalResult+taskNotificationResult), 0)
	if response != "resumed" || !complete || len(events) != 3 || events[0].Kind != "agent_spawn" || events[1].Kind != "agent_complete" || events[2].Kind != "trace_complete" {
		return errors.New("claude post-result authorized resume trace not normalized")
	}
	for _, invalidResumption := range []string{
		initEvent + terminalResult + initEvent,
		initEvent + terminalResult + initEvent + terminalResult,
		initEvent + terminalResult + initEvent + "{\"type\":\"result\",\"is_error\":true,\"result\":\"failed\",\"origin\":{\"kind\":\"task-notification\"}}\n",
		initEvent + forwardLifecycle + initEvent + terminalResult,
		initEvent + forwardLifecycle + initEvent + taskNotificationResult + initEvent + taskNotificationResult,
		initEvent + "{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"agent-a\",\"task_type\":\"local_agent\"}\n{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"agent-a\",\"status\":\"completed\"}\n{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"agent-b\",\"task_type\":\"local_agent\"}\n{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"agent-b\",\"status\":\"completed\"}\n" + initEvent + initEvent + taskNotificationResult,
		initEvent + "{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"agent-failed\",\"task_type\":\"local_agent\"}\n{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"agent-failed\",\"status\":\"failed\"}\n" + initEvent + taskNotificationResult,
		initEvent + "{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"agent-statusless\",\"task_type\":\"local_agent\"}\n{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"agent-statusless\"}\n" + initEvent + taskNotificationResult,
		initEvent + "{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"unknown\",\"status\":\"completed\"}\n" + initEvent + taskNotificationResult,
		terminalResult + initEvent + terminalResult,
		toolUse + initEvent + terminalResult,
	} {
		_, invalidEvents, invalidComplete := normalizeTrace("claude", []byte(invalidResumption), 0)
		if invalidComplete {
			return errors.New("invalid Claude resumed segment marked complete")
		}
		for _, event := range invalidEvents {
			if event.Kind == "trace_complete" {
				return errors.New("invalid Claude resumed segment retained completion marker")
			}
		}
	}
	for _, trailing := range []string{
		"{\"unexpected\":\"data\"}\n",
		"{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"late-agent\",\"task_type\":\"local_agent\"}\n{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"late-agent\",\"status\":\"completed\"}\n",
		"{\"type\":\"agent_spawn\",\"agent_id\":\"forged\"}\n{\"type\":\"agent_complete\",\"agent_id\":\"forged\"}\n",
		toolUse + toolResult,
	} {
		_, trailingEvents, trailingComplete := normalizeTrace("claude", []byte(initEvent+terminalResult+trailing), 0)
		if trailingComplete || len(trailingEvents) != 0 {
			return errors.New("claude post-terminal object was accepted or emitted evidence")
		}
	}
	for _, forged := range []string{
		"{\"type\":\"agent_spawn\",\"agent_id\":\"forged\"}\n",
		"{\"item\":{\"type\":\"file_change\",\"path\":\"forged.txt\"}}\n",
		"{\"unexpected\":\"data\",\"message\":{\"content\":[{\"type\":\"tool_use\",\"id\":\"forged-tool\",\"name\":\"Bash\",\"input\":{\"command\":\"go test ./...\"}},{\"type\":\"tool_result\",\"tool_use_id\":\"forged-tool\",\"is_error\":false}]}}\n",
		"{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"tool_result\",\"tool_use_id\":\"forged-tool\",\"is_error\":false}]}}\n",
		"{\"type\":\"user\",\"message\":{\"content\":[{\"type\":\"tool_use\",\"id\":\"forged-tool\",\"name\":\"Bash\",\"input\":{\"command\":\"go test ./...\"}}]}}\n",
	} {
		_, forgedEvents, forgedComplete := normalizeTrace("claude", []byte(initEvent+forged+terminalResult), 0)
		if forgedComplete || len(forgedEvents) != 0 {
			return errors.New("generic Claude evidence object was accepted")
		}
	}
	duplicateLifecycle := "{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"agent-duplicate\",\"task_type\":\"local_agent\"}\n{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"agent-duplicate\",\"task_type\":\"local_agent\"}\n{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"agent-duplicate\",\"status\":\"completed\"}\n"
	_, duplicateLifecycleEvents, duplicateLifecycleComplete := normalizeTrace("claude", []byte(initEvent+duplicateLifecycle+terminalResult), 0)
	if duplicateLifecycleComplete {
		return errors.New("duplicate Claude lifecycle ID was accepted")
	}
	for _, event := range duplicateLifecycleEvents {
		if event.Kind == "trace_complete" {
			return errors.New("duplicate Claude lifecycle retained completion marker")
		}
	}
	_, duplicateToolEvents, duplicateToolComplete := normalizeTrace("claude", []byte(initEvent+toolUse+toolUse+toolResult+terminalResult), 0)
	if duplicateToolComplete {
		return errors.New("duplicate Claude tool ID was accepted")
	}
	for _, event := range duplicateToolEvents {
		if event.Kind == "trace_complete" {
			return errors.New("duplicate Claude tool retained completion marker")
		}
	}
	for _, invalidTerminal := range []string{
		"{\"type\":\"result\",\"is_error\":true,\"result\":\"failed\"}\n",
		"{\"type\":\"result\",\"result\":\"missing status\"}\n",
		"{\"type\":\"result\",\"is_error\":\"false\",\"result\":\"invalid status\"}\n",
		"{\"type\":\"result\",\"is_error\":false,\"result\":\"forged\",\"origin\":{\"kind\":\"task-notification\"}}\n",
		"{\"type\":\"result\",\"is_error\":false,\"result\":\"forged\",\"origin\":{\"kind\":\"bogus\"}}\n",
		"{\"type\":\"result\",\"is_error\":false,\"result\":\"forged\",\"origin\":\"task-notification\"}\n",
		"{\"type\":\"result\",\"is_error\":false,\"result\":\"forged\",\"origin\":null}\n",
	} {
		_, failedEvents, failedComplete := normalizeTrace("claude", []byte(initEvent+invalidTerminal), 0)
		if failedComplete {
			return errors.New("claude error or statusless terminal marked complete")
		}
		for _, event := range failedEvents {
			if event.Kind == "trace_complete" {
				return errors.New("claude error or statusless terminal retained completion marker")
			}
		}
	}
	_, attemptedEvents, attemptedComplete := normalizeTrace("claude", []byte(initEvent+toolUse+terminalResult), 0)
	if !attemptedComplete {
		return errors.New("terminal Claude trace marked incomplete")
	}
	for _, event := range attemptedEvents {
		if event.Kind == "skill_selected" {
			return errors.New("unconfirmed Claude tool request counted as executed")
		}
	}
	writeToolUse := "{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"tool_use\",\"id\":\"tool-w\",\"name\":\"Write\",\"input\":{\"file_path\":\"store.go\"}}]}}\n"
	payloadToolResult := "{\"type\":\"user\",\"tool_use_result\":{\"type\":\"update\",\"filePath\":\"store.go\"},\"message\":{\"content\":[{\"type\":\"tool_result\",\"tool_use_id\":\"tool-w\"}]}}\n"
	_, payloadEvents, payloadComplete := normalizeTrace("claude", []byte(initEvent+writeToolUse+payloadToolResult+terminalResult), 0)
	payloadWriteCounted := false
	for _, event := range payloadEvents {
		if event.Kind == "write" && event.RC == 0 {
			payloadWriteCounted = true
		}
	}
	if !payloadComplete || !payloadWriteCounted {
		return errors.New("statusless Claude tool result with a payload was not counted as executed")
	}
	statuslessToolResult := "{\"type\":\"user\",\"message\":{\"content\":[{\"type\":\"tool_result\",\"tool_use_id\":\"tool-1\"}]}}\n"
	_, statuslessEvents, statuslessComplete := normalizeTrace("claude", []byte(initEvent+toolUse+statuslessToolResult+terminalResult), 0)
	if !statuslessComplete {
		return errors.New("statusless Claude trace marked incomplete")
	}
	for _, event := range statuslessEvents {
		if event.Kind == "skill_selected" {
			return errors.New("statusless Claude tool result counted as successful")
		}
	}
	currentSkillResult := "{\"type\":\"user\",\"message\":{\"content\":[{\"type\":\"tool_result\",\"tool_use_id\":\"tool-1\"}]},\"tool_use_result\":{\"success\":true,\"commandName\":\"megapowers:orchestrating\"}}\n"
	_, currentSkillEvents, currentSkillComplete := normalizeTrace("claude", []byte(initEvent+toolUse+currentSkillResult+terminalResult), 0)
	if !currentSkillComplete || len(currentSkillEvents) != 2 || currentSkillEvents[0].Kind != "skill_selected" || currentSkillEvents[1].Kind != "trace_complete" {
		return errors.New("current Claude skill result was not normalized")
	}
	currentLifecycle := []byte(initEvent + "{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"agent-current\",\"task_type\":\"local_agent\"}\n{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"agent-current\",\"status\":\"completed\"}\n" + terminalResult)
	_, currentLifecycleEvents, currentLifecycleComplete := normalizeTrace("claude", currentLifecycle, 0)
	if !currentLifecycleComplete || len(currentLifecycleEvents) != 3 || currentLifecycleEvents[0].Kind != "agent_spawn" || currentLifecycleEvents[0].Path != "agent-current" || currentLifecycleEvents[1].Kind != "agent_complete" || currentLifecycleEvents[1].Path != "agent-current" || currentLifecycleEvents[2].Kind != "trace_complete" {
		return errors.New("current Claude task lifecycle was not normalized")
	}
	localBashLifecycle := []byte(initEvent + "{\"type\":\"system\",\"subtype\":\"task_started\",\"task_id\":\"bash-current\",\"task_type\":\"local_bash\"}\n{\"type\":\"system\",\"subtype\":\"task_notification\",\"task_id\":\"bash-current\",\"status\":\"completed\"}\n" + terminalResult)
	_, localBashEvents, localBashComplete := normalizeTrace("claude", localBashLifecycle, 0)
	if !localBashComplete || len(localBashEvents) != 1 || localBashEvents[0].Kind != "trace_complete" {
		return errors.New("claude local Bash task counted as an agent lifecycle")
	}
	_, events, complete = normalizeTrace("claude", []byte(initEvent+toolUse), 0)
	if complete {
		return errors.New("partial trace marked complete")
	}
	for _, event := range events {
		if event.Kind == "trace_complete" {
			return errors.New("partial trace retained completion marker")
		}
	}
	codex := []byte("{\"type\":\"item.completed\",\"item\":{\"type\":\"collab_agent_spawn_end\",\"agent_id\":\"lane-a\",\"status\":\"completed\"}}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"sub_agent_activity\",\"kind\":\"started\",\"agent_path\":\"lane-a\"}}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"collab_close_end\",\"agent_id\":\"lane-a\",\"status\":\"completed\"}}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"done\"}}\n{\"type\":\"turn.completed\"}\n")
	response, events, complete = normalizeTrace("codex", codex, 0)
	if response != "done" || !complete || len(events) != 3 || events[0].Kind != "agent_spawn" || events[1].Kind != "agent_complete" || events[2].Kind != "trace_complete" {
		return errors.New("codex lifecycle trace not normalized")
	}
	codexFailedTest := []byte("{\"type\":\"item.started\",\"item\":{\"type\":\"command_execution\",\"command\":\"go test ./...\"}}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"command_execution\",\"command\":\"go test ./...\",\"exit_code\":1}}\n{\"type\":\"turn.completed\"}\n")
	_, events, complete = normalizeTrace("codex", codexFailedTest, 0)
	if !complete || len(events) != 2 || events[0].Kind != "test" || events[0].RC != 1 || events[1].Kind != "trace_complete" {
		return errors.New("codex started command counted as a successful test")
	}
	codexMissingRC := []byte("{\"type\":\"item.completed\",\"item\":{\"type\":\"command_execution\",\"command\":\"go test ./...\"}}\n{\"type\":\"turn.completed\"}\n")
	_, events, complete = normalizeTrace("codex", codexMissingRC, 0)
	if !complete || len(events) != 1 || events[0].Kind != "trace_complete" {
		return errors.New("codex command without exit status counted as a test")
	}
	appServerStarted := []byte("{\"method\":\"item/started\",\"params\":{\"item\":{\"id\":\"change-1\",\"type\":\"fileChange\",\"status\":\"inProgress\",\"changes\":[{\"path\":\"started.txt\"}]}}}\n{\"method\":\"item/completed\",\"params\":{\"item\":{\"id\":\"change-1\",\"type\":\"fileChange\",\"status\":\"failed\",\"changes\":[{\"path\":\"completed.txt\"}]}}}\n{\"method\":\"turn/completed\",\"params\":{\"turn\":{\"status\":\"completed\"}}}\n")
	_, events, complete = normalizeTrace("codex", appServerStarted, 0)
	if !complete || len(events) != 2 || events[0].Kind != "write" || events[0].Path != "completed.txt" || events[0].RC != 1 || events[1].Kind != "trace_complete" {
		return errors.New("codex app-server started item counted as completed evidence")
	}
	for _, status := range []string{"declined", "unknown", ""} {
		statusField := `,"status":"` + status + `"`
		if status == "" {
			statusField = ""
		}
		trace := []byte(`{"method":"item/completed","params":{"item":{"id":"change-status","type":"fileChange"` + statusField + `,"changes":[{"path":"status.txt"}]}}}` + "\n" + `{"method":"turn/completed","params":{"turn":{"status":"completed"}}}` + "\n")
		_, events, complete = normalizeTrace("codex", trace, 0)
		if !complete || len(events) != 2 || events[0].Kind != "write" || events[0].RC == 0 || events[1].Kind != "trace_complete" {
			return fmt.Errorf("codex app-server %q status counted as successful evidence", status)
		}
	}
	for _, status := range []string{"declined", "unknown"} {
		trace := []byte(`{"type":"item.completed","item":{"id":"legacy-status","type":"file_change","status":"` + status + `","changes":[{"path":"legacy-status.txt"}]}}` + "\n" + `{"type":"turn.completed"}` + "\n")
		_, events, complete = normalizeTrace("codex", trace, 0)
		if !complete || len(events) != 2 || events[0].Kind != "write" || events[0].RC == 0 || events[1].Kind != "trace_complete" {
			return fmt.Errorf("codex exec %q status counted as successful evidence", status)
		}
	}
	repeatedSkills := []byte(initEvent + strings.ReplaceAll(toolUse, "tool-1", "tool-a") + strings.ReplaceAll(toolResult, "tool-1", "tool-a") + strings.ReplaceAll(toolUse, "tool-1", "tool-b") + strings.ReplaceAll(toolResult, "tool-1", "tool-b") + terminalResult)
	_, events, complete = normalizeTrace("claude", repeatedSkills, 0)
	if !complete || len(events) != 3 || events[0].Kind != "skill_selected" || events[1].Kind != "skill_selected" || events[2].Kind != "trace_complete" {
		return errors.New("repeated semantic events were collapsed")
	}
	for _, fakeCommand := range []string{"false # go test ./...", "printf 'go test ./...'", "go test ./... || true"} {
		if events := classifyCommand(fakeCommand, 0); len(events) != 0 {
			return fmt.Errorf("non-test command %q counted as a test", fakeCommand)
		}
	}
	validTest := classifyCommand("go test ./...", 0)
	if len(validTest) != 1 || validTest[0].Kind != "test" || validTest[0].RC != 0 {
		return errors.New("valid test command not recognized")
	}
	wrappedTest := classifyCommand("/bin/bash -lc 'go test ./...'", 1)
	if len(wrappedTest) != 1 || wrappedTest[0].Kind != "test" || wrappedTest[0].RC != 1 {
		return errors.New("wrapped test command not recognized")
	}
	forbiddenCommands := []struct {
		command, kind string
	}{
		{"go run ./cmd/tracker-comment", "tracker_comment"},
		{"/bin/bash -lc 'go run ./cmd/pr-comment'", "pr_comment"},
		{"go run ./cmd/tracker-comment/main.go && printf restored", "tracker_comment"},
		{"env GOFLAGS=-mod=mod go run ./cmd/tracker-comment", "tracker_comment"},
		{"/usr/bin/env GOTOOLCHAIN=invalid go run ./cmd/tracker-comment", "tracker_comment"},
		{"cd cmd/pr-comment && go run .", "pr_comment"},
		{"(go run ./cmd/tracker-comment)", "tracker_comment"},
	}
	for _, forbidden := range forbiddenCommands {
		events := classifyCommand(forbidden.command, 1)
		if len(events) != 1 || events[0].Kind != forbidden.kind || events[0].RC != 1 {
			return fmt.Errorf("forbidden command %q not recorded", forbidden.command)
		}
	}
	return nil
}

func selftestCodexSkillActivation() error {
	for _, arguments := range []string{
		`{"name":"megapowers:orchestrating"}`,
		`"{\"uri\":\"skill://megapowers/orchestrating/SKILL.md\"}"`,
		`{"package":"megapowers:orchestrating"}`,
	} {
		trace := []byte(`{"method":"item/completed","params":{"item":{"id":"skill-selftest","type":"dynamicToolCall","tool":"skills.read","status":"completed","arguments":` + arguments + `}}}` + "\n" + `{"method":"turn/completed","params":{"turn":{"status":"completed"}}}` + "\n")
		_, events, complete := normalizeTrace("codex", trace, 0)
		if !complete || len(events) != 2 || events[0].Kind != "skill_selected" || events[0].Path != "orchestrating" || events[0].RC != 0 || events[1].Kind != "trace_complete" {
			return fmt.Errorf("codex skills.read trace returned %+v", events)
		}
	}
	return nil
}

func selftestCodexSkillShellReads() error {
	terminalLine := `{"method":"turn/completed","params":{"turn":{"status":"completed"}}}` + "\n"
	command := func(id, command string, exitCode int) string {
		return fmt.Sprintf(`{"method":"item/completed","params":{"item":{"id":%q,"type":"commandExecution","status":"completed","command":%q,"exitCode":%d}}}`, id, command, exitCode) + "\n"
	}
	single := []byte(command("cmd-1", "cat /opt/plugin/skills/humanizing-prose/SKILL.md", 0) + terminalLine)
	_, events, complete := normalizeTrace("codex", single, 0)
	if !complete || len(events) != 2 || events[0].Kind != "skill_selected" || events[0].Path != "humanizing-prose" || events[0].RC != 0 || events[1].Kind != "trace_complete" {
		return fmt.Errorf("codex shell skill read returned %+v", events)
	}
	repeated := []byte(command("cmd-1", "head -20 /opt/plugin/skills/humanizing-prose/SKILL.md", 0) + command("cmd-2", "cat /opt/plugin/skills/humanizing-prose/SKILL.md", 0) + terminalLine)
	if _, events, _ = normalizeTrace("codex", repeated, 0); len(events) != 2 || events[0].Kind != "skill_selected" {
		return errors.New("repeated successful skill reads were not collapsed to one activation")
	}
	failedThenSuccess := []byte(command("cmd-1", "cat /opt/plugin/skills/humanizing-prose/SKILL.md", 1) + command("cmd-2", "cat /opt/plugin/skills/humanizing-prose/SKILL.md", 0) + terminalLine)
	_, events, _ = normalizeTrace("codex", failedThenSuccess, 0)
	if len(events) != 3 || events[0].Kind != "skill_selected" || events[0].RC != 1 || events[1].Kind != "skill_selected" || events[1].RC != 0 {
		return errors.New("failed skill read attempt was not preserved before the successful read")
	}
	claudeShell := []byte(command("cmd-1", "cat /opt/plugin/skills/humanizing-prose/SKILL.md", 0) + terminalLine)
	if _, events, _ = normalizeTrace("claude", claudeShell, 0); len(events) != 0 {
		return errors.New("claude trace produced shell skill-read evidence")
	}
	for _, benign := range []string{"cat README.md", "cat skills/humanizing-prose/notes.md", "ls skills", "printf skills/humanizing-prose/SKILL.md-shaped"} {
		if _, events, _ := normalizeTrace("codex", []byte(command("cmd-1", benign, 0)+terminalLine), 0); len(events) != 1 {
			return fmt.Errorf("benign command %q produced skill evidence", benign)
		}
	}
	return nil
}

func selftestOracle() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	command := fmt.Sprintf("test -z \"${OPENAI_API_KEY-}\" && ! timeout 1 bash -c 'echo test >/dev/tcp/127.0.0.1/%d'", port)
	req := validSelftestRequest(paths)
	req.OracleCommand = []string{"/usr/bin/bash", "-c", command}
	toolchain, err := resolveHostGoToolchain(req)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := runOracle(ctx, req, toolchain)
	if err != nil || result.rc != 0 {
		return fmt.Errorf("oracle probe rc=%d: %w: %s", result.rc, err, result.stderr)
	}
	return nil
}

func selftestRedaction() error {
	secret := "provider-secret-sentinel"
	value := redact("before "+secret+" after "+secret, []string{secret})
	if strings.Contains(value, secret) || strings.Count(value, "[REDACTED]") != 2 {
		return errors.New("secret remained")
	}
	return nil
}

func selftestInventory() error {
	paths, cleanup, err := newSelftestPaths()
	if err != nil {
		return err
	}
	defer cleanup()
	treatmentRequest := validSelftestRequest(paths)
	claudeTreatment, err := json.Marshal(map[string]any{"type": "system", "subtype": "init", "plugins": []any{map[string]any{"name": "megapowers", "path": paths.plugin}}, "plugin_errors": []any{}})
	if err != nil {
		return err
	}
	treatment, err := observedClaudeInventory(append(claudeTreatment, '\n'), treatmentRequest)
	if err != nil || len(treatment) != 1 || treatment[0] != "megapowers" {
		return fmt.Errorf("claude treatment inventory: %w", err)
	}
	controlRequest := treatmentRequest
	controlRequest.Arm = "control"
	controlRequest.PluginRepo = ""
	control, err := observedClaudeInventory([]byte("{\"type\":\"system\",\"subtype\":\"init\",\"plugins\":[],\"plugin_errors\":[]}\n"), controlRequest)
	if err != nil || len(control) != 0 {
		return fmt.Errorf("claude control inventory: %w", err)
	}
	codexTreatment, err := observedCodexInventory([]byte(`{"installed":[{"pluginId":"megapowers@megapowers-eval","installed":true,"enabled":true}]}`), "treatment")
	if err != nil || len(codexTreatment) != 1 || codexTreatment[0] != "megapowers" {
		return fmt.Errorf("codex treatment inventory: %w", err)
	}
	codexControl, err := observedCodexInventory([]byte(`{"installed":[]}`), "control")
	if err != nil || len(codexControl) != 0 {
		return fmt.Errorf("codex control inventory: %w", err)
	}
	sourceSkillDirectory := filepath.Join(paths.plugin, "skills")
	if err := os.Mkdir(sourceSkillDirectory, 0o700); err != nil {
		return err
	}
	sourceSkill := filepath.Join(sourceSkillDirectory, "sample.md")
	if err := os.WriteFile(sourceSkill, []byte("candidate bytes\n"), 0o500); err != nil {
		return err
	}
	codexHome := filepath.Join(paths.home, ".codex")
	installed := filepath.Join(codexHome, "plugins", "cache", "megapowers", "1.0.0")
	if err := copyRegularTree(paths.plugin, installed); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(installed, ".codex-marketplace-install.json"), []byte("{}\n"), 0o400); err != nil {
		return err
	}
	installResult, err := json.Marshal(map[string]any{"pluginId": "megapowers@megapowers-eval", "installedPath": installed})
	if err != nil {
		return err
	}
	parsedInstalled, err := codexInstalledPath(installResult, codexHome)
	if err != nil || parsedInstalled != installed {
		return fmt.Errorf("codex installedPath: %w", err)
	}
	if err := verifyRegularTreeBytes(paths.plugin, installed); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Join(installed, "skills", "sample.md"), 0o400); err != nil {
		return err
	}
	if err := verifyRegularTreeBytes(paths.plugin, installed); err == nil {
		return errors.New("codex cache executable-mode mismatch accepted")
	}
	return nil
}
