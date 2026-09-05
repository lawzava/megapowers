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

Servers register at session start. After any configuration change, restart the
session before expecting new tools. A tool missing from a live session is not
evidence of a broken server.

Match the authentication flow to the execution mode. Browser OAuth completes
only in an interactive session; a headless or non-interactive session cannot
finish the grant. Provision a token, complete the grant interactively
beforehand, or route through a proxy command that owns its own authentication.
Record where the credential lives, and never write it into configuration
committed to a repository. When inspecting configuration, print the keys,
never the values.

Verify with a fresh probe, not the current session: start a new
non-interactive session, list the server's tools, and call one read-only tool.
Redact tokens and account identifiers before quoting probe output. A sandboxed
probe can report a healthy server as broken when the sandbox blocks its state
or socket files; re-run the probe outside the sandbox before concluding
failure.

On failure, report the harness, scope, configuration path, server name,
transport, authentication mode, and the exact error. Stop and report rather
than editing credentials you cannot verify.
