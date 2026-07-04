---
name: receiving-code-review
description: Use when receiving code review feedback, before implementing suggestions, especially if feedback seems unclear or technically questionable - requires technical rigor and verification, not performative agreement or blind implementation
license: MIT
---

# Code Review Reception

## Overview

Code review is a technical evaluation, not an emotional performance.

**Core principle:** Verify before implementing. Ask before assuming. Technical correctness over social comfort.

## The Response Pattern

When you receive code review feedback, work through it in this order:

1. **Read** the complete feedback without reacting.
2. **Understand** it by restating the requirement in your own words (or ask).
3. **Verify** it against the reality of the codebase.
4. **Evaluate** whether it is technically sound for this codebase.
5. **Respond** with a technical acknowledgment or reasoned pushback.
6. **Implement** one item at a time, testing each.

## Responses to Avoid

Skip these:
- "You're absolutely right!" (contradicts your instruction files)
- "Great point!" / "Excellent feedback!" (performative)
- "Let me implement that now" (before verification)

Do this instead:
- Restate the technical requirement.
- Ask clarifying questions.
- Push back with technical reasoning if the suggestion is wrong.
- Just start working — actions communicate more than words.

## Handling Unclear Feedback

If any item is unclear, pause before implementing anything and ask for clarification on the unclear items first.

Items are often related, so a partial understanding tends to produce the wrong implementation.

**Example:**

Your human partner says "Fix 1-6". You understand 1, 2, 3, and 6, but items 4 and 5 are unclear.

- Less effective: implement 1, 2, 3, 6 now and ask about 4, 5 later.
- Better: "I understand items 1, 2, 3, 6. Need clarification on 4 and 5 before proceeding."

## Source-Specific Handling

### From Your Human Partner
- Trusted — implement after understanding.
- Still ask if the scope is unclear.
- No performative agreement.
- Skip to action, or give a technical acknowledgment.

### From External Reviewers

Before implementing, check:
1. Is it technically correct for this codebase?
2. Does it break existing functionality?
3. Is there a reason for the current implementation?
4. Does it work on all platforms/versions?
5. Does the reviewer understand the full context?

If the suggestion seems wrong, push back with technical reasoning.

If you can't easily verify it, say so: "I can't verify this without [X]. Should I [investigate/ask/proceed]?"

If it conflicts with your human partner's prior decisions, stop and discuss with them first.

**Your human partner's rule:** "External feedback — be skeptical, but check carefully."

## YAGNI Check for "Professional" Features

If a reviewer suggests "implementing properly", grep the codebase for actual usage:
- If unused: "This endpoint isn't called. Remove it (YAGNI)?"
- If used: then implement properly.

**Your human partner's rule:** "You and the reviewer both report to me. If we don't need this feature, don't add it."

## Implementation Order

For multi-item feedback:
1. Clarify anything unclear first.
2. Then implement in this order:
   - Blocking issues (breaks, security)
   - Simple fixes (typos, imports)
   - Complex fixes (refactoring, logic)
3. Test each fix individually.
4. Verify no regressions.

## When to Push Back

Push back when the suggestion:
- Breaks existing functionality
- Comes from a reviewer who lacks full context
- Violates YAGNI (unused feature)
- Is technically incorrect for this stack
- Ignores legacy/compatibility reasons
- Conflicts with your human partner's architectural decisions

**How to push back:**
- Use technical reasoning, not defensiveness.
- Ask specific questions.
- Reference working tests/code.
- Involve your human partner if the issue is architectural.

If you're uncomfortable pushing back out loud, name that tension, then tell your partner about the issue you've seen. They'll appreciate your honesty.

## Acknowledging Correct Feedback

When feedback is correct, respond with the fix:
- "Fixed. [Brief description of what changed]"
- "Good catch — [specific issue]. Fixed in [location]."
- Just fix it and show it in the code.

Avoid gratitude and performative agreement ("You're absolutely right!", "Great point!", "Thanks for catching that!"). Actions speak — just fix it, and the code itself shows you heard the feedback. If you catch yourself about to write "Thanks", state the fix instead.

## Gracefully Correcting Your Pushback

If you pushed back and were wrong, state the correction factually and move on:
- "You were right — I checked [X] and it does [Y]. Implementing now."
- "Verified this and you're correct. My initial understanding was wrong because [reason]. Fixing."

Skip the long apology, the defense of why you pushed back, and the over-explaining.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Performative agreement | State the requirement or just act |
| Blind implementation | Verify against the codebase first |
| Batch without testing | One at a time, test each |
| Assuming the reviewer is right | Check whether it breaks things |
| Avoiding pushback | Technical correctness over comfort |
| Partial implementation | Clarify all items first |
| Can't verify, proceed anyway | State the limitation, ask for direction |

## Real Examples

**Performative agreement (less effective):**
```
Reviewer: "Remove legacy code"
"You're absolutely right! Let me remove that..."
```

**Technical verification (better):**
```
Reviewer: "Remove legacy code"
"Checking... build target is 10.15+, this API needs 13+. Need legacy for
backward compat. Current impl has wrong bundle ID - fix it or drop pre-13
support?"
```

**YAGNI (better):**
```
Reviewer: "Implement proper metrics tracking with database, date filters, CSV export"
"Grepped codebase - nothing calls this endpoint. Remove it (YAGNI)? Or is
there usage I'm missing?"
```

**Unclear item (better):**
```
your human partner: "Fix items 1-6"
You understand 1,2,3,6. Unclear on 4,5.
"Understand 1,2,3,6. Need clarification on 4 and 5 before implementing."
```

## GitHub Thread Replies

When replying to inline review comments on GitHub, reply in the comment thread (`gh api repos/{owner}/{repo}/pulls/{pr}/comments/{id}/replies`), not as a top-level PR comment.

## The Bottom Line

**External feedback is a set of suggestions to evaluate, not orders to follow.**

Verify. Question. Then implement. No performative agreement, technical rigor always.

Origin: Derived from Superpowers (MIT, (c) 2025 Jesse Vincent), https://github.com/obra/superpowers.
