# Attribution

megapowers builds on the following work.

## Superpowers

The workflow core descends from
[Superpowers](https://github.com/obra/superpowers) by Jesse Vincent. The
planning, test-first, debugging, verification, worktree, and delegation methods
were rewritten and consolidated into the current task-level skills.

Upstream license: MIT, Copyright (c) 2025 Jesse Vincent.

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

## humanizer

`humanizing-prose` adapts the editing approach from
[humanizer](https://github.com/blader/humanizer) by Siqi Chen. The current skill
keeps the fact-preservation and direct-writing contract while removing a fixed
vocabulary checklist.

Upstream license: MIT, Copyright (c) 2025 Siqi Chen.

## Skills for Real Engineers

`grill-me` adapts the round-based grilling interview from
[mattpocock/skills](https://github.com/mattpocock/skills) by Matt Pocock. The
current skill rewrites the design-tree and frontier protocol in repository
style, makes it topic-agnostic, and resolves facts itself following the
`design-and-plan` approach.

Upstream license: MIT, Copyright (c) 2026 Matt Pocock.

The instruction-authoring skill also draws on Matt Pocock's progressive
disclosure and removal-testing approach. Its guidance is written for this
repository's supported harnesses.

## OpenSpec

The behavior-specification lifecycle uses concepts from
[OpenSpec](https://github.com/Fission-AI/OpenSpec): requirement deltas,
scenarios, task mapping, verification, and reconciliation into baseline specs.
Megapowers implements these as Markdown guidance. It bundles no OpenSpec code,
CLI, Node package, or dependency.

## Official skill guidance

`writing-agent-instructions` synthesizes the official OpenAI and Anthropic
skill and repository-instruction guidance linked from its references. It uses
progressive disclosure, scoped discovery, and task-based evaluation.

## Everything Claude Code

The Go reference in `test-first-implementation` retains stable context, error, and
goroutine guidance adapted from `golang-patterns` in
[Everything Claude Code](https://github.com/affaan-m/everything-claude-code) by
Affaan Mustafa.

Upstream license: MIT, Copyright (c) 2026 Affaan Mustafa.

## codex-plugin-cc

The adversarial-review framing and structured review outcome concepts were
informed by [codex-plugin-cc](https://github.com/openai/codex-plugin-cc) by
OpenAI. The current review package, disclosure, and receipt implementation is a
new Go standard-library tool with a narrower explicit-input contract.

Upstream license: Apache-2.0. Upstream notice: Copyright 2026 OpenAI.

## caveman

The treatment and terse-control measurement design in `evals/` was informed by
[caveman](https://github.com/JuliusBrussee/caveman) by Julius Brussee. No text or
code was copied.

Upstream license: MIT, Copyright (c) 2026 Julius Brussee.
