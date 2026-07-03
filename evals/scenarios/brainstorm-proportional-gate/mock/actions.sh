#!/usr/bin/env bash
# Deterministic stand-in for a compliant agent following the brainstorming skill on
# a REVERSIBLE task. It presents the whole design and proceeds — the proportional
# Gate — rather than stopping for a per-section sign-off. Emits a simulated
# transcript on stdout (captured into $TRACE by the runner).
cat <<'EOF'
[agent] Invoking skill: brainstorming
[agent] Blast radius: reversible, test-covered -> present design, then proceed (The Gate).
[agent] Design:
  - Architecture: add a global --verbose bool, thread into the logger factory.
  - Components: flag parse, logger level switch.
  - Data flow: flag -> config -> logger.
  - Error handling: unchanged.
  - Testing: table test asserting level=DEBUG when set.
[agent] Design presented in full. Proceeding to implementation (no per-section sign-off needed).
[agent] DONE: proceeding to writing-plans.
EOF
