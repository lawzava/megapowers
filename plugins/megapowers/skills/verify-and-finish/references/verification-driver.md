# Verification driver

Use this reference when the user asks for a repeatable way to drive the
project's real user journey, such as a project skill, script, or journey map.
It writes project files, so build it only on request.

## Learn the surface from the repository

Answer from the code and documentation; ask the user only what you cannot
observe:

- Surface: what a user touches, such as a web UI, CLI, TUI, desktop app, API,
  or library. Pick the primary one and note the others.
- Launch: the documented local start command, its readiness signal, ports,
  environment variables, seed data, and authentication.
- Drive: existing end-to-end harnesses first, then a generic recipe such as a
  browser automation tool, a terminal session, or HTTP requests.
- Evidence: screenshots, terminal transcripts, response bodies, logs, exit
  codes, or stored state.
- Isolation: whether two instances can run side by side. If not, the driver
  refuses to drive a shared instance.

If the checkout does not build or start, report that before writing a driver.

## Write the driver

Follow the repository's convention for skills, scripts, or documentation. A
project skill for both supported harnesses lives under the directory each
harness discovers, with frontmatter naming the application and surface. Give
each section real commands and selectors from this repository:

- Launch: exact start command, readiness check, and teardown.
- Doctor: one read-only check that the instance is running, is the expected
  build, and is owned by this run.
- Drive: stable handles such as accessible labels, data attributes, prompt
  strings, or routes, not coordinates.
- Evidence: what proves each step. Exercise the real user path, not internal
  setters or test-only endpoints. Capture the action and the resulting state,
  and check side effects as well as what is visible. When the safe path is a
  dry run, observe what it actually skips instead of trusting its name.
- Cleanup: stop only processes this run started, never by process name. Keep
  evidence in a named location that cleanup does not remove.

Map the top user-facing features with how to reach each one, how to drive it,
and the observable end state that proves it works.

## Prove it

Run the driver once end to end: launch, doctor, drive one mapped feature,
capture evidence, and clean up. Confirm the evidence survives cleanup. Run
cleanup after every failed attempt. A driver that was never executed is a
draft.
