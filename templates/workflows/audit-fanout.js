// audit-fanout.js: fan an audit out across many targets through several review
//   lenses, adversarially verify each finding, then synthesize one ranked report.
//   The verify stage is what makes this more than "more agents": a second agent
//   tries to refute each finding before it is reported (cf.
//   mega-orchestration:cross-model-verification). Reference template; see README.md.
//
// Save to .claude/workflows/audit-fanout.js (project, shared via the repo) or
// ~/.claude/workflows/audit-fanout.js (personal); it then runs as /audit-fanout.
// A plugin CANNOT ship a workflow, so this is a file you copy in, not an installed
// component.
//
// Args: { question?: string, paths?: string[], lenses?: string[] }

export const meta = {
  name: 'audit-fanout',
  description: 'Audit many targets through review lenses, verify each finding, synthesize',
}

const question = args?.question ?? 'correctness, security, and reliability defects'
const lenses = args?.lenses ?? ['correctness', 'security', 'simplicity']

// 1. Discover the targets. The workflow itself has no filesystem access, so an
//    agent lists them.
const found = await agent(
  `List the files to audit for: ${question}.` +
  (args?.paths ? ` Restrict to: ${args.paths.join(', ')}.` : ''),
  { schema: { type: 'object', required: ['files'], properties: { files: { type: 'array', items: { type: 'string' } } } } },
)

// 2. Lens fan-out: one agent per (file x lens). Each returns only its findings.
//    parallel() takes an array of thunks and awaits them all, so map each job to
//    a zero-arg function that starts its agent.
const jobs = found.files.flatMap(file => lenses.map(lens => ({ file, lens })))
const raw = await parallel(jobs.map(({ file, lens }) => () =>
  agent(
    `Audit ${file} through the ${lens} lens for: ${question}. ` +
    `Return each finding with a file:line location and a one-line claim.`,
    {
      label: `${lens}:${file}`,
      schema: { type: 'object', required: ['findings'], properties: { findings: { type: 'array', items: { type: 'string' } } } },
    },
  ).then(r => r.findings.map(claim => ({ file, lens, claim }))),
))
const findings = raw.flat()

// 3. Verify stage: a different agent tries to REFUTE each finding against the
//    code. Unverifiable or refuted claims are dropped, so only reproduced defects
//    reach the report.
const verified = await parallel(findings.map(f => () =>
  agent(
    `Try to refute this ${f.lens} finding in ${f.file}: "${f.claim}". ` +
    `Reproduce it against the code and report survives=true only if it holds.`,
    { schema: { type: 'object', required: ['survives'], properties: { survives: { type: 'boolean' } } } },
  ).then(r => ({ ...f, survives: r.survives })),
))

// 4. Synthesis: one agent ranks and deduplicates the survivors into a report.
const report = await agent(
  `Merge these verified findings into one ranked, deduplicated audit report:\n` +
  JSON.stringify(verified.filter(f => f.survives)),
  { model: 'sonnet' },
)

return report
