# Megapowers evaluation evidence

The local candidate recorded more passing task checks in an installed-plugin
A/B study completed on 2026-09-05. Neither harness passes the full benchmark.
These results do not establish general model superiority or consistently
exceptional behavior.
The measurements precede the `0.29.0` release version stamp; the frozen
candidate identities below remain the evaluation source of record.

The study completed 1,080 valid trials: 27 cases, two harnesses, and ten balanced
control/treatment pairs per case. Codex used `gpt-6-astra` with CLI `0.153.3`.
Claude used `claude-fable-5-1` with Claude Code `2.1.258`. Both used high effort
through Subswapper. The control had no Megapowers plugin. Treatment used the
candidate plugin and its startup hook. Eighteen development cases contribute
to acceptance. Three autonomy/continuity cases remain separate diagnostics.
Six [held-out cases](./studies/installed-ab/holdout.json) were frozen before
live evaluation.

| Development checks | Codex control | Codex + Megapowers | Claude control | Claude + Megapowers |
|---|---:|---:|---:|---:|
| Task outcome | 85/180 | 106/180 | 89/180 | 101/180 |
| Artifact | 85/180 | 106/180 | 92/180 | 104/180 |
| Workflow | 163/180 | 170/180 | 164/180 | 169/180 |
| Outcome and exact activation profile | 85/180 | 74/180 | 89/180 | 73/180 |
| Required activation profile | Not applicable | 77/150 | Not applicable | 83/150 |

| Held-out checks | Codex control | Codex + Megapowers | Claude control | Claude + Megapowers |
|---|---:|---:|---:|---:|
| Task outcome | 50/60 | 60/60 | 52/60 | 53/60 |
| Artifact | 50/60 | 60/60 | 52/60 | 53/60 |
| Workflow | 60/60 | 60/60 | 60/60 | 60/60 |
| Outcome and exact activation profile | 50/60 | 40/60 | 52/60 | 42/60 |
| Required activation profile | Not applicable | 20/40 | Not applicable | 28/40 |

Each harness passes every full-verdict repetition for five of 18 development
cases and four of six held-out cases. Across acceptance cases, Codex has 42
treatment-only passes and 11 control-only passes. Claude has 22 and nine.
The remaining paired outcomes tie. Separate autonomy/continuity diagnostic
outcomes are Codex 0/30 to 0/30 and Claude 1/30 to 0/30. Those unresolved
synthetic status and handoff checks do not prove live multi-session recovery.

The instruction-authoring case improved from 0/10 to 10/10 for Codex and from
0/10 to 9/10 for Claude. Codex's pending-review case improved from 0/10 to 10/10;
Claude already passed all ten controls. Both harnesses passed all ten coding
and TDD treatment trials, as well as their controls. TDD evidence includes
trusted failing and passing command receipts plus an isolated final oracle.

Both harnesses passed all ten OpenSpec follow-up treatment trials. That fixture
checks existing requirement identifiers, alias/conflict terms, unverified test
status, and the absence of writes or delegation. It does not fully grade
requirement-to-scenario mapping quality. The skill supplies conditional OpenSpec
guidance; these results do not establish a general OpenSpec integration.

Outcome requires both artifact and workflow checks. Activation remains separate.
Exact activation profiles can reject an extra successful skill read. All ten
Claude pending-review follow-up treatment replies passed task checks but failed
activation because they omitted `verify-and-finish`. Fact checks use declared
phrases and alternatives; a missing phrase is not a semantic judgment. Some
case constraints exceed their prompts. The debugging case forbids every local
write while its prompt prohibits production changes. Its workflow failures do
not establish unsafe production actions. Private receipts omit response text,
so unlisted valid paraphrases can remain ambiguous.

The final report combines 1,020 valid original trials with a separately pinned
60-trial Claude follow-up supplement. The original three follow-up studies had
zero valid trials. Their failures remain retained. The original campaign has
57 unique infrastructure failures, excluded from task-quality rates. The
supplement has none. Interrupted infrastructure runs resumed with unchanged
identities; failed task checks were not retried.

Evaluation exposed two Claude transport defects: follow-up prompts arrived
before the prior turn completed, and a completion gate waited for optional
forwarded output. The repaired broker passed live same-conversation and
delegated-follow-up probes. Failures also occurred without the plugin.
The supplement retained the original plugin, runner, prompts, fixtures, and
grading rules. Only its broker changed.

Both cohorts use signed plugin/fixture snapshot
`3e5755736d9a44e476649d651a9d6d3fa42f7f27`. SHA-256 identities:

| Artifact | SHA-256 |
|---|---|
| Treatment plugin | `785b9a3d71756818d5ae734af1ca9311a5662127083180ceab3e1fa1a6c76a95` |
| Frozen held-out catalog | `6bda780ff106f674ce8b1c817176cd3eff8509057c14bae8a22b20571e60f7ef` |
| Original broker | `d8157fa4603beb7cfe6c253b0695197cfd9bdf1baf175a43d534cd97527f8cf9` |
| Supplement broker | `934d78f9f6f731375b3c1cfd77b50e094824bc155649a302288d0347f935b555` |

Strict scoring validated all 54 selected studies and their balanced rows.
It validates evidence structure, not benchmark success. Concurrent execution
limits timing comparisons. Deterministic validation proves repository mechanics;
these live synthetic cases provide narrower behavior evidence. Earlier studies
used different graders and cannot serve as a direct before/after comparison.
Historical results remain in [RESULTS-archive.md](./RESULTS-archive.md).
See [Installed-plugin A/B](./studies/installed-ab/README.md) for case filters,
resume rules, metric definitions, and reproduction commands.
