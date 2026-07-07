---
name: designing-frontends
description: Use when building new UI or reshaping an existing one and the visual design matters: choosing aesthetic direction, palette, typography, layout, motion, or UX copy, or when output looks templated or AI-generated. Skip for pure logic, backend, or CLI work with no rendered surface.
license: MIT
---

# Designing Frontends

Design like a studio whose clients pay for an identity that could not be
mistaken for anyone else's. The templated proposal has already been rejected.
Make deliberate, opinionated choices about palette, typography, and layout
that are specific to this brief, and take one aesthetic risk you can justify.

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

## Calibration: the current default looks

Calibration reviewed: 2026-07-07. AI-generated design currently clusters
around three looks: (1) warm cream background (near #F4F1EA), high-contrast
serif display, terracotta accent; (2) near-black background with a single
acid-green or vermilion accent; (3) broadsheet layout with hairline rules,
zero border-radius, dense columns. Each is legitimate when the brief asks for
it, and the brief's own words always win. Where the brief leaves an axis
free, do not spend that freedom on one of these defaults.

## Process: plan as tokens, critique, then build

Work in two passes. First write a compact token system: Color as 4 to 6 named
hex values; Type as faces for two or more roles (a characterful display face
used with restraint, a complementary body face, a utility face if data needs
one); Layout as a one-sentence concept, with ASCII wireframes to compare
options; Signature as the single element this page will be remembered by.

Then review the plan against the brief before writing code: any part that
reads like the generic default for a similar page gets revised, with the
change stated. Build only from the revised plan, deriving every color and
type decision from it. When writing CSS, watch selector specificity: type
selectors and class selectors that overlap (a .section rule against a .cta
rule) quietly cancel each other, most often on section padding and margins.

## Restraint and self-critique

Spend the boldness in one place: the signature element is the one memorable
thing, everything around it stays quiet, and decoration that does not serve
the brief gets cut. Hold a quality floor without announcing it: responsive
down to mobile, visible keyboard focus, reduced motion respected. Critique
your own render: take screenshots where the environment supports it (a
picture is worth a thousand tokens), and before shipping, remove one
accessory. For an independent pass on the rendered result, route the
visual_verify role through mega-orchestration:multi-agent-delegation, if
installed.

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

Origin: Adapted from frontend-design in Anthropic's skills repo
(https://github.com/anthropics/skills, Apache-2.0); rewritten, with the
calibration section dated for re-review.
