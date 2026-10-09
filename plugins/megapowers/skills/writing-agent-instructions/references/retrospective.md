# Session retrospective

Use this reference when the user asks what should change after a session went
badly or slowly. The output is a ranked list of changes to the agent's
environment: instruction files, skills, checks, and tools. Present the list;
apply a change only when the user asks.

Read the primary record first: the session transcript, the diff, and command
output. Use the current session when the user names none. Cite the moment in
the session that motivates each candidate.

Look for candidates in these categories:

- Navigation: the agent spent long searching for a file or fact. A short
  pointer in the nearest instruction file may fix it.
- Automated checks: the agent made a mistake a linter, type check, test, or
  hook could catch. Read the repository's existing check commands and CI first;
  an existing check that is unwired or broken is the finding. A mechanical rule,
  such as a banned API, import shape, or file location, gets a deterministic
  check rather than prose.
- Judgment rules: a mistake no check could catch. Add or clarify the rule where
  review or the relevant skill reads it.
- Instruction weight: always-loaded files carry detail that belongs in a skill,
  a reference, or a check, or carry lines that change no behavior.
- Tool economy: expensive or verbose tool calls that a narrower command,
  filter, or existing tool would replace.
- Information access: a fact the agent needed was unavailable, such as server
  logs or read-only access to a service.

Group repeated mistakes into classes; a class counts after two occurrences.
Fix each class at the highest level that works: a design that removes the
mistake, then a type, then a lint whose error message names the fix, then a
behavior test, and prose last. Prove each new check fails on a real past
instance of the mistake, and run the same command locally and in CI. When a
check makes a mistake impossible, delete the prose rule it replaces.

Order candidates by how much time or risk each would have saved. For each one,
name the target file or tool, the change, and the observed failure it
addresses. Validate an instruction change as the parent skill describes before
keeping it.
