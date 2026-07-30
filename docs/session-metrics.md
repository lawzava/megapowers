# Session metrics

`scripts/session-metrics` turns "sessions feel slow" into a number, so a
change to the guard, the delegate launcher, or a skill can be checked against
real transcripts instead of a hand rolled audit.

```
scripts/session-metrics [--since ISO8601] [--project DIR] [--json] [--include-all]
```

It reads Claude Code transcripts (`~/.claude/projects/<slug>/<session-uuid>.jsonl`,
JSON Lines; override the root with `SESSION_METRICS_ROOT`) and prints, per
session and pooled across sessions, the numbers below. It is a report, not a
gate: it exits 0 whenever it can read the transcript store and never writes
anything under it.

By default it skips sidechain sessions and sessions whose entrypoint is not
`cli` (SDK spawns, such as automated security review runs), so those do not
distort the aggregate. `entrypoint` and `isSidechain` are session-level
properties that only appear on some records within a file; each session's
value is resolved once, from whichever record carries it, and the whole
session is included or excluded on that basis (a session with no `entrypoint`
anywhere defaults to `cli`). Pass `--include-all` to include them.

A session with zero assistant turns after filtering is omitted from the
report entirely, on top of the entrypoint and sidechain filters above (and
even under `--include-all`), since it carries no information: no latency, no
tool wall clock, nothing to show but a row of `n/a`.

The aggregate row's span is the earliest `span_start` and latest `span_end`
across the sessions in the report, not a claim that the tool was busy for
that whole window. Sessions can overlap or leave gaps between them; the
aggregate span says what date range the report covers, not a continuous
duration.

## One model response is several records

Claude Code does not write one record per model response. It splits a single
response into one record per content block: each thinking block, the text
block, and every `tool_use` block gets its own `type == "assistant"` record,
and all the siblings carry the same `message.id`.

This matters enough to state plainly, because an earlier version of this
script got it wrong. A `type == "assistant"` record is a content block, not a
turn. Counting records overstates model responses by more than a factor of two
(42,575 records against 19,580 responses over the store measured below), and a
"more than one `tool_use` block in one record" test for batching can never
fire, because the harness puts each `tool_use` in its own record.

Every number below that means "per model response" therefore groups assistant
records by `message.id` first. Sibling records are always adjacent within the
assistant records of a file and always in timestamp order, so a response is a
contiguous run. A record with no `message.id` counts as its own response,
which is the conservative reading: never merge records that might belong to
different responses.

## What each number means

**Assistant turns.** One count per model response, that is, per group of
consecutive assistant records sharing a `message.id`. This is the denominator
for several ratios below.

**Model latency (median, p90, total).** For each model response, the gap
between the timestamp of the last `user` record before it and the timestamp of
its own first record. That preceding user record is the `tool_result` or human
prompt that completed the model's input, so this is composing time up to the
first streamed block, and it excludes tool execution. Measuring instead from
the previous response's last record would fold every tool run into it: on the
store below that alternative totals 865.39 hours against this definition's
68.02.

When no user record separates a response from the one before it (13 responses
in 19,593 on the store below), the previous response's last record is used
instead. A negative gap, which is sub-second record write-order jitter, counts
as zero.

Model latency is not where the time goes: an 8.1 second median composing time
next to a 78.73 hour tool wall clock says the serialization is downstream of
the model.

**Tool wall clock (total, by tool).** For each `tool_use` block, the gap
between the assistant record that issued it and the timestamp of the
matching `tool_result`. This is the number that shows serialization cost:
work the agent could have started earlier if it had not been waiting on one
tool call to finish before issuing the next.

A gap of one hour or more is excluded from this figure and reported
separately as an excluded count. A tool call that genuinely runs for an hour
inside an interactive session does not happen; a gap that long is a
permission prompt the human had not yet answered, or the session sitting
idle across a break. Counting that time as tool execution would swamp every
other number in the report with a single overnight pause.

**Bash p90.** The 90th percentile of Bash tool wall clock times, the same
inclusion rule as above. Bash is called out on its own because it is where
long, foreground, unbatched commands live: builds, test suites, searches.

**Batched-tool-call ratio.** Model responses that issue more than one
`tool_use` block, over model responses that issue at least one. The blocks are
summed across the response's records, since a response normally spreads them
over several. This is the headline number for the serialization problem: a
batch of independent tool calls in a single response runs concurrently;
unbatched calls run one round trip at a time. On the store below the ratio is
0.14, and the largest single response issued 23 tool calls.

**Backgrounded-Bash ratio.** Bash calls with `run_in_background: true`, over
all Bash calls. A long build or watch command backgrounded with `BashOutput`
polling later frees the turn to keep working; one that runs in the
foreground blocks it. On the store below this is used rarely: 0.02, or 273 of
11,421 Bash calls.

**Skill invocations.** Count of `tool_use` blocks naming the `Skill` tool.

**Hook failures.** Count of `attachment` records whose `attachment.durationMs`
is present and whose payload either carries a non-zero exit status or the
text `Plugin directory does not exist`. This is the automatic detector for a
stale or missing plugin cache breaking a hook mid-session.

## The three levers

The audit that produced this script found three concrete ways to cut
serialization cost, in order of expected impact:

1. **Batch independent tool calls.** Every read, search, or lookup that does
   not depend on another call's result belongs in the same turn as its
   siblings. The batched-tool-call ratio is the number to watch; the baseline
   below is 0.14, so a change meant to improve batching has to beat that
   number, not merely be above zero.
2. **Background long commands.** A Bash call expected to run past a few
   seconds (builds, test suites, long-running servers) should set
   `run_in_background` and be checked later with `BashOutput`, instead of
   blocking the turn. The backgrounded-Bash ratio is the number to watch.
3. **Cap the verify loop.** A review or verification round that repeats
   against the same artifact without new information is wall clock spent
   for no new evidence. This script does not measure verify-round count
   directly (see the delegate launcher's own round accounting), but a
   session whose tool wall clock is dominated by a handful of tools called
   many times each, visible in the by-tool breakdown, is worth checking for
   this pattern.

## Baseline

Every number quoted above comes from one run of this script with default
filters over the author's own transcript store. It is recorded here so a later
run has something to be compared against, and so a claim of improvement has to
beat a specific number instead of clearing zero.

```
scripts/session-metrics --json
```

| Number | Baseline |
| --- | --- |
| Sessions | 154 |
| Span | 2026-06-30 to 2026-07-30 |
| Model responses | 19,587 |
| Model latency, median | 8.1s |
| Model latency, p90 | 23.49s |
| Model latency, total | 68.02h |
| Tool wall clock, total | 78.73h |
| Long-gap calls excluded | 13 |
| Bash p90 | 19.95s |
| Batched-tool-call ratio | 0.14 |
| Backgrounded-Bash ratio | 0.02 |
| Skill invocations | 199 |
| Hook failures | 16 |

Tool wall clock concentrates in a few tools: Bash 45.48h over 11,434 calls,
Agent 14.84h over 832, AskUserQuestion 9.34h over 126. Those three are 89
percent of the total across 21,454 paired calls.

The numbers are specific to one machine and one month of work. Treat the
shape as the transferable part: composing time is a rounding error next to
tool wall clock, and Bash dominates the tool wall clock.

## Regenerating the evidence table

Before and after a change meant to reduce serialization, run this script
against the same session (or `--since` the same window) and compare the tool
wall clock total, Bash p90, and batched-tool-call ratio. Those three numbers
are what "sessions feel slow" now means in this repository.

Compare against the baseline table above rather than against zero. Both
ratios have a non-zero baseline, so "it moved off 0" is not evidence of
anything.
