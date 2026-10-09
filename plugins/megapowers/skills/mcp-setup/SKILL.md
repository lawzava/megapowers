---
name: mcp-setup
description: Use when installing, configuring, debugging, or verifying an MCP server for a harness, or when a configured server's tools are missing or failing.
when_to_use: "Trigger phrases: MCP tools missing, server not showing up, /mcp shows disconnected, re-authenticate the MCP, add an MCP server, tools still unavailable after login, mcp authorized but calls fail."
metadata:
  short-description: Install, repair, or verify an MCP server for a harness
---

# MCP Setup

Resolve the exact scope first: which harness, which configuration file, and
whether the server is global, project, or repository-local. Keep one
registration channel per server; duplicate registrations across scopes produce
conflicting tool lists and ambiguous failures.

Servers register at session start. Some harnesses reconnect a server or
refresh its tools live; after a configuration change, reconnect the server from
the harness's server menu when it offers one, otherwise restart the session. A
tool missing from a live session is not evidence of a broken server.

Treat diagnostic output as secret-bearing. A harness's server-list command and
configuration dumps can print arguments, headers, and connection strings
verbatim; filter values out before the output reaches the transcript. Start
with a read-only status check of the configured servers before any login
attempt. Match the authentication flow to the execution mode. Browser OAuth
needs an interactive terminal, not a local browser: on a remote or display-less
host, a login command that prints the authorization URL and accepts the pasted
callback URL completes the grant. A non-interactive session cannot finish it. Provision a token, complete the grant interactively
beforehand, or route through a proxy command that owns its own authentication.
Record where the credential lives, and never write it into configuration
committed to a repository.

Attempt re-authentication at most once per server. If it fails, stop: report
the exact error text and the user action needed (complete the browser grant,
issue a token with the missing scope, or fix the registration). Do not retry,
probe other login commands, or substitute a CLI for the server unless asked.

Verify with a fresh probe, not the current session: start a new
non-interactive session, list the server's tools, and call one read-only tool.
Redact tokens and account identifiers before quoting probe output. A sandboxed
probe can report a healthy server as broken when the sandbox blocks its state
or socket files; re-run the probe outside the sandbox before concluding
failure.

On failure, report the harness, scope, configuration path, server name,
transport, authentication mode, exit status, and raw error. Use
`systematic-debugging` to distinguish launcher, authentication, quota, and
provider failures. Do not edit credentials you cannot verify.
