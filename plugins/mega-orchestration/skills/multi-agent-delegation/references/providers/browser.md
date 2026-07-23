# Driver: browser capture with playwright-cli

Vendor-neutral evidence capture for visual verification. This is a driver, not
a model provider: it gathers screenshots but cannot satisfy a review role,
vendor-independence requirement, tier, effort, or verdict.

## Channel

A capture, reason, act loop from Bash:

1. Capture: navigate and screenshot the current state with `playwright-cli`.
2. Reason: the independently resolved vision-capable model reads the screenshot
   and decides the next action or verdict.
3. Act: apply the action with `playwright-cli` (click, type, fill, select,
   navigate), repeat until verified, then capture a final screenshot.

Sessions: `playwright-cli state-save auth.json` after authenticating, then
`state-load auth.json` on later runs. Long-lived identities use a persistent
profile (`open --persistent` or `--profile=dir`). Headed vs headless changes
the rendered pixels, so verify in the mode the lead expects.

## Evidence

Every visual claim (pass or fail) carries a screenshot under
`.megapowers/evidence/`, and the lead re-reads the images before accepting the
claim; a text-only visual verdict is not evidence. The full driver procedure is
the browser-delegate agent (../../../../agents/browser-delegate.md).
