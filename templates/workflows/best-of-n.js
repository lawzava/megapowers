// best-of-n.js: generate N independent candidates, then SELECT one: executable
//   oracle first, blind judge second. Codifies mega-orchestration:best-of-n as a
//   Claude Code dynamic workflow. Reference template; see README.md.
//
// Save to .claude/workflows/best-of-n.js (project, shared via the repo) or
// ~/.claude/workflows/best-of-n.js (personal); it then runs as /best-of-n. A
// plugin CANNOT ship a workflow, so megapowers distributes this as a file you
// copy in, not an installed component.
//
// Args: { task: string, n?: number, oracle?: string }
//   oracle = a shell check that decides "correct" (e.g. "npm test"); omit it when
//   no oracle can exist and a blind judge must decide.

export const meta = {
  name: 'best-of-n',
  description: 'Generate N independent candidates, select one by oracle then blind judge',
}

const task = args?.task ?? 'the task described to you'
const n = args?.n ?? 3
const oracle = args?.oracle // undefined => the judge decides

// 1. Generate N candidates independently, each in its own worktree, blind to the
//    others. Shared context collapses N into 1, so give them no cross-visibility.
// parallel() takes an array of THUNKS (zero-arg functions returning a promise)
// and is a barrier: it awaits them all. Build one thunk per candidate.
const candidates = await parallel(
  Array.from({ length: n }, (_, i) => () => {
    const k = i + 1
    return agent(
      `Candidate ${k}. Working in your own isolated git worktree, implement: ${task}\n` +
      `You are blind to every other attempt. Return the worktree path and a one-line summary.`,
      {
        label: `candidate-${k}`,
        schema: {
          type: 'object',
          required: ['worktree', 'summary'],
          properties: { worktree: { type: 'string' }, summary: { type: 'string' } },
        },
      },
    )
  }),
)

// 2. Select, oracle first. Run the executable check in each worktree; the
//    passers survive. The lead re-runs the oracle later; never trust a
//    self-reported pass.
let survivors = candidates
if (oracle) {
  const graded = await parallel(candidates.map(c => () =>
    agent(
      `In worktree ${c.worktree}, run \`${oracle}\` and report only whether it passed.`,
      { label: `oracle-${c.worktree}`, schema: { type: 'object', required: ['passed'], properties: { passed: { type: 'boolean' } } } },
    ).then(r => ({ ...c, passed: r.passed })),
  ))
  survivors = graded.filter(c => c.passed)
  if (survivors.length === 1) return survivors[0] // sole passer wins; done
}

// 3. Select, blind judge second (no oracle, or several passers). One judge agent
//    blinds the set with best-of-n's anonymize-candidates, then ranks the
//    anonymized worktrees on the brief's criteria. Prefer a different-vendor judge
//    and mitigate position bias by swapping order.
const winner = await agent(
  `Rank these candidate worktrees for the task: ${task}\n` +
  survivors.map(c => c.worktree).join('\n') + '\n' +
  `First run mega-orchestration:best-of-n's scripts/anonymize-candidates to strip ` +
  `authorship markers, then judge blind on the criteria (not on length). Return the ` +
  `winning worktree path and a one-line reason.`,
  {
    model: 'sonnet',
    schema: {
      type: 'object',
      required: ['worktree', 'reason'],
      properties: { worktree: { type: 'string' }, reason: { type: 'string' } },
    },
  },
)

return winner // the lead integrates the winner as single writer
