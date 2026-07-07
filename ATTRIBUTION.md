# Attribution

megapowers builds on the work of others. This file records upstream sources
and the licenses they are used under.

## Superpowers (the `megapowers` workflow plugin)

The `megapowers` workflow plugin is a fork and restyling of **Superpowers** by
Jesse Vincent (obra): https://github.com/obra/superpowers.

- Upstream license: MIT
- Required notice, retained here per that license:

  > MIT License
  >
  > Copyright (c) 2025 Jesse Vincent
  >
  > Permission is hereby granted, free of charge, to any person obtaining a copy
  > of this software and associated documentation files (the "Software"), to deal
  > in the Software without restriction, including without limitation the rights
  > to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
  > copies of the Software, and to permit persons to whom the Software is
  > furnished to do so, subject to the following conditions:
  >
  > The above copyright notice and this permission notice shall be included in all
  > copies or substantial portions of the Software.
  >
  > THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
  > IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
  > FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
  > AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
  > LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
  > OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
  > SOFTWARE.

The methodology (brainstorming, planning, TDD, systematic debugging, review,
worktrees, subagent orchestration) originates with Superpowers. megapowers
rewrites the prose style and renames the skills; the underlying process is
Jesse Vincent's.

Every Superpowers-derived skill also carries a one-line origin footer in its
`SKILL.md` body, so the notice travels with copies installed through the
bare-`SKILL.md` skills CLI channel (`npx skills add ...`), not only with the
plugin bundle that ships this file.

### Fork point and maintenance

megapowers vendored the Superpowers process core as a snapshot. That snapshot
has no git parent in this repo, and the pre-publication history is squashed (see
the initial public release commit). The fork tracked upstream as of the initial
vendoring in early July 2026; the exact upstream commit is not recorded. The
contemporaneous upstream release line was Superpowers v6.1.0 (dated 2026-06-30),
which is the baseline a future backport review should diff against.

Maintenance rule: upstream Superpowers releases are reviewed for backports on
each megapowers release.

## golang-patterns (in the `mega-go` plugin)

The `golang-patterns` skill is vendored from **Everything Claude Code** by
Affaan Mustafa: https://github.com/affaan-m/everything-claude-code
(path `.kiro/skills/golang-patterns`).

- Upstream license: MIT
- Required notice, retained per that license:

  > MIT License
  >
  > Copyright (c) 2026 Affaan Mustafa
  >
  > Permission is hereby granted, free of charge, to any person obtaining a copy
  > of this software and associated documentation files (the "Software"), to deal
  > in the Software without restriction, including without limitation the rights
  > to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
  > copies of the Software, and to permit persons to whom the Software is
  > furnished to do so, subject to the following conditions:
  >
  > The above copyright notice and this permission notice shall be included in all
  > copies or substantial portions of the Software.
  >
  > THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
  > IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
  > FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
  > AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
  > LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
  > OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
  > SOFTWARE.

## codex-plugin-cc (prompting reference in `mega-orchestration`)

`plugins/mega-orchestration/skills/multi-agent-delegation/references/prompting-codex.md`
adapts prompting guidance, the adversarial-review framing, and the review
output schema from **codex-plugin-cc** by OpenAI:
https://github.com/openai/codex-plugin-cc.

- Upstream license: Apache-2.0. Upstream NOTICE: "Copyright 2026 OpenAI"
  (retained here per that license).
- The material is rewritten, not copied: restyled to this repo's register and
  condensed, with the schema's field names preserved. This entry records the
  provenance and the changes.

## skill-creator (description-optimization guidance in `megapowers`)

The "Optimizing the Description" section of
`plugins/megapowers/skills/writing-skills/testing-skills-with-subagents.md`
adapts the description eval loop from **skill-creator** in Anthropic's skills
repo: https://github.com/anthropics/skills.

- Upstream license: Apache-2.0.
- Rewritten, not copied; this entry records the provenance.

## humanizer (the `humanizing-prose` skill)

`plugins/megapowers/skills/humanizing-prose/SKILL.md` adapts the AI-tell
taxonomy from **humanizer** by Siqi Chen (blader):
https://github.com/blader/humanizer.

- Upstream license: MIT, Copyright (c) 2025 Siqi Chen.
- humanizer itself derives from Wikipedia's "Signs of AI writing"
  (WikiProject AI Cleanup, CC BY-SA). The skill's wording here is re-derived
  against a measured baseline rather than copied; both sources are credited,
  and the skill carries a one-line origin footer.

## Other sources

Additional upstream credits are added as further modules land, once their
upstream licenses are confirmed.
