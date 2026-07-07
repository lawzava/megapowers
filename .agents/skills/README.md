# .agents/skills

Relative symlinks, one per skill, into `plugins/*/skills/*`. They give a Codex,
OpenCode, or Antigravity session running inside this checkout zero-install skill
discovery: `.agents/skills/` is a discovery path all three read, and each link
resolves to the canonical `SKILL.md` under `plugins/`. Git tracks them as
symlinks (mode 120000), so a clone or `git pull` keeps them pointing at the
current skill bodies with nothing to copy or regenerate.

These are for reading the repo's own skills in place. For installing skills into
another project or a global path, use the marketplaces or the skills CLI (see
[`docs/setup.md`](../../docs/setup.md)); do not also install from here, or a
skill registers twice.

The flat leaf names here share the `.agents/skills` namespace with any other
skill vendor installed the same way (for example the official `google/skills`
repo). As of 2026-07-04 none of the 28 names then present collide with
Google's set (theirs are domain-specific: `gke-*`, `bigquery-*`, `gemini-api`,
`gcloud`, and similar); `humanizing-prose` and `designing-frontends`, added
2026-07-07, are outside that naming space too.
