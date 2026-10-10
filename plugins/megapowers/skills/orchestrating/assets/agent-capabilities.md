# Agent capabilities

Personal model preferences for lead and delegated agents. Copy this file to
`~/.config/megapowers/agent-capabilities.md` and replace every `<placeholder>`.
User instructions, repository instructions, available tools, and permissions
take precedence. Apart from the review rule below, this file grants no access,
write, or effect authority.

```yaml
version: 2
refreshed_at: <YYYY-MM-DD>
expires_at: <YYYY-MM-DD>   # about a month out; an expired file is ignored

policy:
  capability_floor: strong             # weakest profile intelligence a ranked lane accepts
  optimize_after_floor: [speed, cost]  # tie-break order among eligible bindings
  escalate_on: [oracle_failure, high_risk, unresolved_ambiguity]

profiles:
  fast-read: { roles: [explorer], intelligence: strong, write: false }
  balanced-build: { roles: [writer], intelligence: strong, write: true }
  challenger: { roles: [challenger], intelligence: frontier }
  deep-consult: { roles: [consultant], intelligence: frontier, write: false }
  independent-review: { roles: [reviewer], write: false }

lead_preferences:
  <harness>: { model: <model-id>, effort: high }

bindings:
  <harness>:
    fast-read:
      model: <model-id>
      effort: low
      agent_type: <native-agent-type>
      family: <vendor-family>
      access: native
      rankable: true
    balanced-build:
      model: <model-id>
      effort: high
      agent_type: <native-agent-type>
      family: <vendor-family>
      access: native
      rankable: true
    independent-review:
      provider: <other-harness>
      model: <model-id>
      effort: high
      family: <different-vendor-family>
      access: manual
      status: operator-selected
      rankable: false

fallbacks:
  fast-read: { provider: <harness>, model: <model-id>, effort: medium, write: false }
```

## Rules

- An `independent-review` binding is your standing approval to send review
  packages to that reviewer through the independent-review tool. The agent
  decides when to review and dispatches without asking; the tool's secret
  scan still applies. Delete the binding to require approval again.
- An unavailable binding is reported, not replaced. Use a `fallbacks` entry
  only when access, disclosure, and permissions for it already exist.
- A failed task oracle calls for diagnosis, not a provider switch.
- `intelligence`, `speed`, and `cost` are relative operator judgments, not
  measurements. Record the evidence and date for any benchmark you rely on
  in a note below the block.
- Effort names do not mean the same amount of thinking across models. Set
  effort from the vendor's default for each new model and your own runs at
  several levels, not from the previous model's value.
- Keep credentials, command lines, private paths, and routing secrets out of
  this file.
