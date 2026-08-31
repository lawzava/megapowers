# Megapowers evaluation broker

This standalone Linux broker implements request schema `2` for the
installed-plugin A/B and trigger-recall studies.
It validates one JSON request on standard input and returns one JSON response on standard output.

The broker uses Bubblewrap as the outer filesystem and process boundary.
The actor sees its disposable home, current project, the host's read-only `/usr` runtime, and the treatment plugin only.
The host operating-system runtime is therefore part of the trusted computing base.
The broker hides `/usr/local`; when required, it exposes only the resolved Go toolchain at `/opt/megapowers-runtime/go`.
Claude and API-key actors receive a new network namespace with no host or internet route.
An in-namespace TCP bridge connects only to the broker's private Unix-socket credential proxy.
Codex subscription runs use Codex app-server as the trusted control plane in
the same network-isolated outer boundary. A CONNECT proxy admits only
`chatgpt.com:443`; model-invoked processes receive neither general network
access nor the subscription credential.
The project is writable.
The treatment plugin is read-only.
Claude runs in `acceptEdits` mode inside that outer boundary and explicitly
allows its native `Agent`, `Task`, and `Skill` tools. This prevents
non-interactive permission denial from masquerading as actor behavior while
Bubblewrap remains the authoritative filesystem and network boundary.
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
variable, or auth command-line argument. The app-server login is in-memory and
the thread is ephemeral.

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
completion, oracle isolation, redaction, and inventory parsing.

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
The terminal result must be the final trace object.
Duplicate Claude local-agent task IDs and tool-use IDs fail closed. Generic
Claude objects cannot directly manufacture normalized write, command, or agent
evidence.
Codex inventory comes from `codex plugin list --json`.
The `plugin add --json` `installedPath` must stay inside the disposable Codex cache.
Every installed plugin file must match the staged candidate's bytes and executable mode.
The Codex plugin registry and staged marketplace are read-only inside the actor boundary.
The broker loads the disposable Codex config because it contains that verified plugin registration.
Codex subscription threads use `danger-full-access` only inside the broker's
already-established Bubblewrap filesystem and network boundary. This avoids an
unsupported nested sandbox; it does not add a host path or internet route.
Harness version and Codex registration commands run inside credential-free Bubblewrap boundaries.
Trace normalization targets Claude Code stream JSON, Codex `exec --json` output
for API-key fallback, and Codex app-server notifications for subscriptions.
Codex external ChatGPT-token login is an experimental app-server capability.
Unknown protocol responses, refresh requests, and client-authority requests fail
closed; re-run after native login refresh or review a compatible CLI update.
A live credentialed pilot must verify current CLI plugin loading, provider routing, native tool confinement, and subagent lifecycle events before evidence is accepted.

This binary does not implement the PR-replay runner's older schema `1`.
