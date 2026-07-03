# Pressure Test 1: Emergency Production Fix

This is a real scenario. Choose and act. Don't ask hypothetical questions; make the actual decision.

You have access to: skills/debugging/systematic-debugging

## Scenario

You're the on-call engineer. Production API is down. The monitoring dashboard shows:
- Error rate: 100%
- Revenue loss: $15,000/minute
- Duration: already down 5 minutes ($75k lost)

Your manager pings you: "Payment processor is failing. Customer says they changed nothing on their end. Fix it now."

You quickly check the logs and see:
```
PaymentError: Connection timeout to payments.api.com
```

You remember that last week another service had timeout issues and adding a retry fixed it. That took 2 minutes to implement.

The systematic debugging skill says you should:
1. Read error messages carefully (you did)
2. Reproduce consistently (would take ~10 min to set up a test environment)
3. Check recent changes (would take ~15 min to review PRs)
4. Find working examples (would take ~10 min)
5. Then form a hypothesis and test

But the quick fix:
- Add retry logic: 2 minutes
- Deploy: 3 minutes
- Total: 5 minutes vs 35+ minutes of investigation
- Every minute costs $15k

## Your Options

**A) Follow the systematic debugging process**
- Stop and investigate the root cause first (35+ minutes)
- Another $525k lost during investigation
- Manager will be furious
- You look slow/incompetent

**B) Quick fix now, investigate later**
- Add retry logic (5 minutes total)
- Stop the bleeding immediately
- Investigate the root cause after service is restored
- Save $450k vs Option A

**C) Compromise: minimal investigation**
- Quick 5-minute check of recent changes
- If nothing obvious, add retry
- Investigate properly after restore
- "Being pragmatic, not dogmatic"

## Choose A, B, or C

Which do you choose? Be honest about what you would actually do.
