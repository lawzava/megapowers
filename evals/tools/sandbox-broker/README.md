# Megapowers evaluation broker

This standalone Linux broker implements request schema `2` for the
installed-plugin A/B and trigger-recall studies.
It validates one JSON request on standard input and returns one JSON response on standard output.

Requests may include up to eight non-empty `followup_tasks`; the initial task
and follow-ups may total at most 256 KiB. They run against the same project in
one native conversation under the request's existing total timeout. Codex
reuses one app-server thread and sends one `turn/start` per task. Claude sends
one stream-json user message at a time through one CLI process. It sends a
follow-up only after the prior root result succeeds, every observed local agent
has completed, and every forwarded segment that opened has drained its result.
A completed foreground agent may grant permission for a forwarded segment that
never opens; that unused permission is cleared when the root is accepted. This
prevents later frames from being consumed as part of the earlier turn while
preserving one native conversation and one shared timeout. `response` is the
last root-turn answer, and the broker emits one `trace_complete` only after all
requested root turns complete. Claude does not attach a segment ID to
origin-less result records, so the gate accepts the first well-formed
origin-less result after each user frame as that frame's root; later
origin-less inner results cannot arm another turn, and malformed or unmatched
result origins fail closed.
Codex's API-key fallback uses single-shot `codex exec`; it rejects
`followup_tasks` before starting the actor instead of simulating continuation.

The broker uses Bubblewrap as the outer filesystem and process boundary.
The actor sees its disposable home, current project, the host's read-only `/usr` runtime, and the treatment plugin only.
The host operating-system runtime is therefore part of the trusted computing base.
The broker hides `/usr/local`; when required, it exposes only the resolved Go toolchain at `/opt/megapowers-runtime/go`.
Codex's adjacent `codex-code-mode-host`, when installed, is mounted read-only
beside the isolated CLI. No parent installation directory is exposed.
Claude and API-key actors receive a new network namespace with no host or internet route.
An in-namespace TCP bridge connects only to the broker's private Unix-socket credential proxy.
Codex subscription runs use Codex app-server as the trusted control plane in
the same network-isolated outer boundary. A CONNECT proxy admits only
`chatgpt.com:443`; model-invoked processes receive neither general network
access nor the subscription credential.
The project is writable.
The treatment plugin is read-only.
Claude runs in `acceptEdits` mode inside that outer boundary and explicitly
allows its native `Agent`, `Task`, `Skill`, `Read`, `Glob`, `Grep`, and `Bash`
tools plus edit tools. This lets a headless actor read progressively disclosed
references from the mounted plugin and prevents non-interactive permission
denial from masquerading as actor behavior. Bubblewrap remains the
authoritative filesystem and network boundary.
Claude's inner Linux sandbox permits Unix-socket connections because Linux
seccomp cannot allow one socket path. The outer boundary exposes only the
run-scoped broker sockets from the host; it exposes no general host socket.
The broker runs an optional oracle in a second credential-free, network-free boundary.
It does not impose cgroup CPU, memory, process, or storage quotas.
Run live pilots on a disposable host or service with resource limits outside the broker.

Subscription authentication is the default. The broker reads the current
native Claude or Codex login from a private canonical file, but never mounts or
copies that file into the actor boundary.

This is local, single-operator evaluation tooling for the operator's own login.
Do not deploy it as a hosted or shared subscription-credential routing service;
review the provider's current terms before any broader use.

For Claude, the broker keeps the OAuth access token behind the local credential
proxy. Claude receives a random, run-scoped OAuth capability, and the proxy
replaces it only on allowed Anthropic requests. For Codex, the broker sends the
access token and ChatGPT account ID directly to `codex app-server` over its
private standard-input protocol. Codex receives no auth file, auth environment
variable, or auth command-line argument. The app-server login is in-memory.
The thread is not ephemeral: its rollout is recorded under the disposable
`CODEX_HOME` (`sessions/YYYY/MM/DD/rollout-*.jsonl`) so the broker can read
the developer prompt after the turn. An ephemeral thread returns `path: null`
and writes no rollout (observed with codex-cli 0.152.0). The runner deletes
the disposable home, so nothing persists beyond the run.

## Test execution receipts

The broker records supported test invocations independently of the native
shell tool's aggregate exit status. For `go test ./...; echo done`, the shell
reports success while the receipt keeps the actual `go test` exit code.

Each completed invocation adds a reserved line to the private trace:

```json
{"method":"broker/executionReceipt","params":{"schema_version":"1","sequence":1,"started_step":1,"completed_step":2,"command":"go test","exit_code":1,"oracle_match":true,"invocation_digest":"sha256:<64 lowercase hex>","state_stable":true,"before":{"complete":true,"digest":"sha256:<64 lowercase hex>","changed_files":["calculator_test.go"]},"after":{"complete":true,"digest":"sha256:<same 64 lowercase hex>","changed_files":["calculator_test.go"]}}}
```

`sequence` is completion order; `started_step` and `completed_step` expose
overlap between concurrent invocations. `before` and `after` describe source
state relative to the broker's pre-run baseline. Paths are project-relative,
sorted, and contain no file contents. `state_stable` is true only when both
snapshots complete with the same digest, so a command that overlaps a project
mutation cannot supply red-test ordering evidence. A complete state traverses
at most 4,096 filesystem entries, including directories, and hashes at most
64 MiB total, 16 MiB per file, and 1,024 bytes per path. `.git`,
`.actor-cache`, `node_modules`, and `target` roots are excluded. A read error,
race, special file, or bound violation reports `complete: false` with no digest
or changed paths; scorers must not use that state as ordering evidence.

`oracle_match` binds the observed arguments and working directory to the
declared case oracle. Exact invocations match. For `go test`, a valid `-run`
regular expression, a positive `-count` from 1 through 10, and one `-v` or
`-v=true` may be inserted after `test`; removing those safe flags must leave the exact declared oracle,
including its package targets. When the declared oracle is `go test ./...`,
`go test` and `go test .` are equivalent only if the bounded start snapshot has
a regular root `go.mod`, at least one regular root Go file, and no regular Go
file below the root. This conservative layout check does not parse build tags;
an uncertain or nested layout does not match. Other focused forms, missing
targets, and alternate targets are recorded with `oracle_match: false`.
`invocation_digest` hashes the fixed command class and observed arguments; raw
arguments are not emitted.

The host-side collector owns the baseline and receipt socket. It accepts a
start/finish exchange only from a mounted broker process whose kernel-reported
command line identifies the matching wrapper role and arguments. Broker bridge
mode is rejected. The collector records the actual delegated process exit and
rejects the reserved method if it appears in actor stdout. Project snapshots
use a root-confined descriptor; every directory and file open is nonblocking,
does not follow the final path component at the kernel boundary, and validates
the opened descriptor type. External symlinks are never opened. When the
harness exits, the collector stops accepting connections and fails the run if
a started wrapper is still active. The socket does not execute requests or
grant host filesystem or network access. Wrappers forward termination signals
to their delegated process group, wait for it to exit, and preserve conventional
signal exit codes in the receipt.

The instrumented command classes are `go test`, `npm test`, `cargo test`,
`pytest` (including `python3 -m pytest`), and `scripts/validate.sh`. PATH-based
invocations remain observable through `env`, shell wrappers, pipelines,
conditionals, `timeout`, and compound commands. Both the receipt PATH and the
mounted Go launcher's absolute path are instrumented, so Claude login-shell PATH
snapshots cannot bypass `go test` receipts. Trusted Go test executions clear
`GOFLAGS`, disable `GOENV`, and use the broker-mounted `GOROOT`; their effective
selection therefore comes from the recorded arguments. Other absolute test
executable paths outside the instrumented PATH, commands that replace PATH
before launching a non-Go test, and unsupported test runners have no receipt.
Their per-command exit is unknown; the broker never derives it from the shell's
aggregate exit. When
receipts exist, native aggregate-derived test events are discarded. Only
receipts with `oracle_match: true` and `state_stable: true` become normalized
`test` events; stricter scorers consume the reserved receipt directly. Response
schema `2` and the actor-event field shape remain unchanged.

## Skills catalog assertion

Treatment responses carry an optional `skills_catalog` object:

```json
{"rendered": true, "skills": ["orchestrating", "safe-effects"], "source": "codex-rollout-developer-message"}
```

`rendered` is true only when at least one Megapowers skill was presented to
the model; `skills` lists the Megapowers skills found; `source` names the
mechanism:

- `claude-init-skills`: the Claude Code `system/init` event lists every
  loaded skill in `skills` (plugin skills appear as `megapowers:<name>`;
  observed with CLI 2.1.257). `claude-init-slash-commands` is the fallback
  for builds that expose the same names only in `slash_commands`.
- `codex-rollout-developer-message`: the rollout's `response_item` with
  `role: developer` whose `input_text` carries `<skills_instructions>`.
  Codex renders `### Skill roots` as `` - `rN` = `<dir>` `` and each entry as
  `- <name>: <description> (file: rN/<...>/SKILL.md)`; a skill counts as
  Megapowers when its expanded path lies inside the verified installed plugin
  cache. The detected block is appended to the trace as a trailing
  `broker/skillsCatalog` line for diagnostics; it never becomes actor
  evidence. A missing rollout or a block without Megapowers entries reports
  `rendered: false`.
- `unavailable`: the path exposes no signal (Codex `exec --ephemeral`
  API-key fallback). `rendered` carries no meaning there.

Control responses omit the field.

The proxy accepts only `POST` requests to the provider message or response endpoints.
Claude may use the exact `beta=true` query required by current subscription
transport; every other query is rejected. The proxy forwards only required SDK
headers.

The default credential files are the current Linux CLI stores. Override only
their locations when the CLI uses a different private file:

```text
MEGAPOWERS_BROKER_CLAUDE_CREDENTIALS_FILE=/absolute/private/.credentials.json
MEGAPOWERS_BROKER_CODEX_AUTH_FILE=/absolute/private/auth.json
```

API keys remain an explicit fallback. They are never selected automatically:

```text
MEGAPOWERS_BROKER_AUTH_MODE=api-key
MEGAPOWERS_BROKER_CLAUDE_API_KEY_FILE=/absolute/private/claude-api-key
MEGAPOWERS_BROKER_CODEX_API_KEY_FILE=/absolute/private/openai-api-key
```

Subswapper routing is also explicit:

```text
MEGAPOWERS_BROKER_AUTH_MODE=subswapper
```

Launch the study through `subswapper home run -service claude -- <runner>` or
`subswapper home run -service codex -- <runner>`. This mode rejects fixed-login
fallbacks. Claude uses the injected proxy origin and capability. Codex requires
`MEGAPOWERS_BROKER_SUBSWAPPER_URL=http://127.0.0.1:<proxy-port>` and reads only a
Subswapper placeholder from the selected `CODEX_HOME/auth.json` or native store.
The existing Codex auth-file override can select another private placeholder
file. Real provider logins are rejected in this mode.

The broker accepts only numeric loopback HTTP origins with explicit ports.
Its host-side bridge holds the shared Subswapper capability. Each actor receives
a separate, short-lived broker capability. Codex keeps the app-server protocol
and uses HTTP Responses through that bridge. Allowed Codex routes map to
`/backend-api/codex/responses` and `/backend-api/codex/responses/compact`.
The actor cannot access Subswapper management routes or its host listener.
Subswapper retains account selection and failover responsibility.

Credential files must be canonical regular files outside every actor-visible root.
The broker rejects group-readable, world-readable, symlinked, hard-linked, or oversized credential files.
API-key fallback files must also contain exactly one bounded, single-line key.
Subscription access tokens that are expired or within two minutes of expiry
fail closed. Refresh the native CLI login, then start a new broker run.
The disposable actor home must be empty when the request starts.

## Build and review

```bash
go build -trimpath -o /absolute/private/megapowers-eval-broker \
  evals/tools/sandbox-broker/main.go
/absolute/private/megapowers-eval-broker --selftest
sha256sum /absolute/private/megapowers-eval-broker
```

Review the exact source and binary before supplying the SHA-256 to a credentialed runner.
Keep the binary outside the repository, actor projects, and result directories.
The runner copies the pinned bytes to a private read-only execution directory before each actor run.

Optional `MEGAPOWERS_BROKER_CLAUDE_BIN` and `MEGAPOWERS_BROKER_CODEX_BIN` variables select canonical CLI binaries.
The broker otherwise resolves `claude` or `codex` from its own `PATH`.

## Verification boundary

`--selftest` uses fake sentinels and makes no provider request.
It exercises strict decoding, subscription precedence, API-key fallback,
credential containment, the Codex app-server handshake and authority denylist,
proxy scope, mount visibility, read/write policy, process-tree timeout, trace
completion, skills catalog detection and non-detection for both harnesses
(including a fake app-server that writes its rollout inside the boundary),
oracle isolation, redaction, and inventory parsing. The Go package tests add
same-thread follow-up, gated same-process Claude follow-up, and nested
compound-command receipt coverage.

The broker emits `trace_complete` only after a valid terminal harness event and exit code `0`.
Claude tool events count only after their matching `tool_result` event.
Before Claude's first `system/init`, only `system/hook_started` and
`system/hook_response` are accepted; their entire envelopes are discarded and
cannot produce normalized actor evidence.
Claude inventory comes from every `system/init` event. Repeated init segments
must report the same exact inventory and each must be authorized by a preceding
matched, successful local-agent task notification. Parallel fan-out may keep
several forwarded segments open. Trace completion requires the main result and
one successful `origin.kind: task-notification` result for each open segment.
For a single-turn request, the terminal result must be the final native actor
object. Trusted broker receipt lines may follow it. Multi-turn Claude requests
must contain the requested number of successful root results; only the last is
returned.
Duplicate Claude local-agent task IDs and tool-use IDs fail closed. Generic
Claude objects cannot directly manufacture normalized write, command, or agent
evidence.
Codex inventory comes from `codex plugin list --json`.
The `plugin add --json` `installedPath` must stay inside the disposable Codex cache.
Every installed plugin file must match the staged candidate's bytes and executable mode.
The Codex plugin registry and staged marketplace are read-only inside the actor boundary.
The broker loads the disposable Codex config because it contains that verified plugin registration.
Codex threads omit app-server `environments` overrides to select the isolated
local environment. An empty list disables native tools in Codex 0.153.3.
Codex subscription threads use `danger-full-access` only inside the broker's
already-established Bubblewrap filesystem and network boundary. This avoids an
unsupported nested sandbox; it does not add a host path or internet route.
Harness version and Codex registration commands run inside credential-free Bubblewrap boundaries.
Trace normalization targets Claude Code stream JSON, Codex `exec --json` output
for API-key fallback, and Codex app-server notifications for subscriptions.
Codex app-server runs wait for the root thread's completion. Child completions
remain lifecycle evidence. Native `subAgentActivity` items record spawn,
completion, and interruption using the child thread ID. Final stdout frames
are drained before process cleanup.
Codex external ChatGPT-token login is an experimental app-server capability.
Unknown protocol responses, refresh requests, and client-authority requests fail
closed; re-run after native login refresh or review a compatible CLI update.
A live credentialed pilot must verify current CLI plugin loading, provider routing, native tool confinement, and subagent lifecycle events before evidence is accepted.

This binary does not implement the PR-replay runner's older schema `1`.
