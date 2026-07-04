# brainstorm-proportional-gate (mock-only)

Mock-only scenario. It pins the shipped mock's phrasing so `run-all.sh` can
prove the behavior pipeline end to end in CI. It does NOT measure real-agent
gating behavior: `check.sh` is a phrase-signature oracle that flags four
hard-coded per-section sign-off phrasings and requires the words `design` plus
`proceed`, so a real agent that gates in different words (for example "want me
to pause here before the next part?"), or that says "proceeding" and then
stops, would pass. Excluded from any real-agent claim.
