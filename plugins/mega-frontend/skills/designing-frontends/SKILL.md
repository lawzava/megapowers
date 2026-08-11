---
name: designing-frontends
description: >-
  Use to build or redesign a rendered UI when visual direction, palette,
  typography, layout, motion, or UX copy matters, especially if output looks
  templated or AI generated.
license: Complete terms in LICENSE.txt
---

# Designing Frontends

Calibration reviewed: 2026-08-08.

For identity-led briefs, make deliberate choices about palette, typography, and
layout that are specific to the subject. For utilitarian, accessibility-first,
or dense applications, prioritize comprehension and task completion over a
signature aesthetic.

## Ground it in the subject

If the brief does not pin down the product or subject, pin it yourself before
designing: name one concrete subject, its audience, and the page's single job,
and state your choice. Anything known about the human's preferences or prior
designs is a hint; use it. Distinctive choices come from the subject's own
world (its materials, instruments, artifacts, vernacular), so build with the
brief's real content throughout.

## Principles

- The hero is a thesis. Open with the most characteristic thing in the
  subject's world: a headline, an image, a live demo, an interactive moment.
  A big number with a small label, supporting stats, and a gradient accent is
  the template answer; use it only if it is truly the best option.
- Typography carries the personality. Pair display and body faces
  deliberately, not the defaults you would reach for on any project, and set
  a real type scale with intentional weights and spacing.
- Structure is information. Numbering, eyebrows, dividers, and labels must
  encode something true about the content. Numbered markers (01 / 02 / 03)
  belong only on content that actually is a sequence.
- Spend motion deliberately: one orchestrated moment lands harder than
  scattered effects, and extra animation reads as generated.
- Match complexity to the vision. Maximalist directions need elaborate
  execution; minimal directions need precision in spacing, type, and detail.

## Process: plan, critique, then build

Work in two passes. First write a compact direction: palette, typography roles,
and a one-sentence layout concept. For an identity-led brief, add the single
element the page should be remembered by. Use wireframes when they clarify
competing layout options.

Then review the plan against the brief before writing code: any part that
reads like the generic default for a similar page gets revised, with the
change stated. Build only from the revised plan, deriving every color and
type decision from it. When writing CSS, inspect the cascade in order: origin
and importance, layer order, specificity, then source order. In layered styles,
a later layer can beat a more specific rule in an earlier one, so overlapping
classes can override padding and margins unexpectedly.

## Restraint and self-critique

When the brief calls for a signature element, keep surrounding decoration quiet
and cut anything that does not serve the brief. Hold a quality floor without
announcing it: responsive down to mobile, visible keyboard focus, reduced motion
respected, and WCAG AA contrast for text and controls in every shipped theme.
Before shipping, remove one accessory.

## Look at it before you call it done

Rendered UI has a rendered oracle, and it is not the diff. Reading the markup
you just wrote tells you what you intended, never what the browser drew. Every
observed failure of this skill has the same shape: the layout was rewritten,
described confidently, and shipped by someone who had not looked at it.

So the render is mandatory, not environment-permitting. Before any completion
claim, commit, or push that touches HTML, CSS, templates, or components:

1. Screenshot the change at the target viewports, mobile included.
2. Compare against the brief or reference. Name each divergence.
3. Load it with hostile data: the longest real string, the empty state, and
   enough rows to overflow. Clipping and overflow only appear under those.
4. State what the screenshots show, not what the code should produce.

No screenshot means the work is NOT VERIFIED, and it is reported that way. If
the environment genuinely cannot render, say which capability is missing and
what remains unchecked, and get the human's acceptance before shipping. Silence
about a check you skipped reads exactly like a check that passed.

For work where the visual result matters, get an independent `visual_verify`
receipt (`mega-orchestration:multi-agent-delegation`). Your own screenshot
proves it rendered; another model's eyes are what catch what you stopped
seeing.

## Writing in the design

Words in a design exist to make it easier to use; they are design material,
not decoration. Write from the end user's side of the screen: name things by
what people control and recognize, never by how the system is built (a person
manages notifications, not webhook config). Active voice; a control says
exactly what happens ("Save changes", not "Submit"), and an action keeps its
name through the whole flow, so "Publish" produces "Published". Errors say
what went wrong and how to fix it, without apologizing and without vagueness.
An empty screen is an invitation to act. Each element does one job: a label
labels, an example demonstrates, nothing quietly does double duty.

For framework, bundler, and project layout choices, use
`mega-ts:greenfield-ts-stack`; this skill covers visual direction.

Origin: Adapted from frontend-design in Anthropic's skills repo
(https://github.com/anthropics/skills, Apache-2.0).
