# Provider: browser (playwright-cli plus a vision-capable model)

Vendor-neutral visual verification and fallback driving. Depends only on
`playwright-cli` (a standalone CLI callable from any runtime's Bash) plus a
vision-capable model to read the screenshots, not on any one vendor's browser
agent.

## Channel

A capture, reason, act loop from Bash:

1. Capture: navigate and screenshot the current state with `playwright-cli`.
2. Reason: a vision-capable model reads the screenshot and decides the next
   action or the verdict. A vision-capable lead reads the image directly;
   otherwise route the screenshot to one.
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
