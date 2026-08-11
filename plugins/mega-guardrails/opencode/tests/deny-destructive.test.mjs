import { test } from "node:test";
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync, existsSync, copyFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { pathToFileURL } from "node:url";
import path from "node:path";
import { MegapowersDenyDestructive } from "../deny-destructive.js";

// POSIX single-quote escaping: wraps the value so bash treats it as one
// literal argument, matching what a real BunShell `$` template does when it
// interpolates an expression.
function shellEscape(value) {
  return "'" + String(value).replace(/'/g, `'\\''`) + "'";
}

// A minimal shim of the BunShell tagged-template runner, backed by a real
// `bash -c` child process. This drives the ACTUAL deny-destructive.sh policy
// script through the same `$` interface the plugin uses in production, so
// these tests exercise the real classifier rather than a mock of it.
function makeRealShell({ onCall } = {}) {
  return function $(strings, ...values) {
    onCall?.();
    let cmd = strings[0];
    for (let i = 0; i < values.length; i++) {
      cmd += shellEscape(values[i]) + strings[i + 1];
    }
    const state = { throws: true };
    const promise = new Promise((resolve, reject) => {
      const child = spawn("bash", ["-c", cmd]);
      let stdout = "";
      let stderr = "";
      child.stdout.on("data", (d) => (stdout += d));
      child.stderr.on("data", (d) => (stderr += d));
      child.on("error", reject);
      child.on("close", (exitCode) => {
        const result = { stdout, stderr, exitCode, text: () => stdout };
        if (exitCode !== 0 && state.throws) {
          reject(Object.assign(new Error(`shell exited ${exitCode}`), result));
        } else {
          resolve(result);
        }
      });
    });
    promise.nothrow = () => {
      state.throws = false;
      return promise;
    };
    promise.quiet = () => promise;
    return promise;
  };
}

test("rm -rf / throws and the message carries the hook's reason", async () => {
  const hooks = await MegapowersDenyDestructive({ $: makeRealShell() });
  await assert.rejects(
    hooks["tool.execute.before"](
      { tool: "bash", sessionID: "s", callID: "c" },
      { args: { command: "rm -rf /" } },
    ),
    (err) => {
      assert.match(err.message, /^megapowers: /);
      assert.match(err.message, /recursive rm of a root, home, or system directory/);
      return true;
    },
  );
});

test("rm -rf ./dist does not throw", async () => {
  const hooks = await MegapowersDenyDestructive({ $: makeRealShell() });
  await assert.doesNotReject(
    hooks["tool.execute.before"](
      { tool: "bash", sessionID: "s", callID: "c" },
      { args: { command: "rm -rf ./dist" } },
    ),
  );
});

test("a non-bash tool never invokes the guard script", async () => {
  let calls = 0;
  const hooks = await MegapowersDenyDestructive({
    $: makeRealShell({ onCall: () => calls++ }),
  });
  await hooks["tool.execute.before"](
    { tool: "read", sessionID: "s", callID: "c" },
    { args: { filePath: ".env" } },
  );
  assert.equal(calls, 0);
});

test("script failure or unparseable stdout does not throw (fails open)", async () => {
  // Unparseable stdout: the guard "ran" but returned garbage.
  const garbageShell = () => {
    const p = Promise.resolve({ text: () => "not json {{{" });
    p.nothrow = () => p;
    p.quiet = () => p;
    return p;
  };
  const hooksGarbage = await MegapowersDenyDestructive({ $: garbageShell });
  await assert.doesNotReject(
    hooksGarbage["tool.execute.before"](
      { tool: "bash", sessionID: "s", callID: "c" },
      { args: { command: "rm -rf /" } },
    ),
  );

  // Script/shell failure: the `$` call itself rejects.
  const failingShell = () => {
    const p = Promise.reject(new Error("boom"));
    p.nothrow = () => p;
    p.quiet = () => p;
    return p;
  };
  const hooksFailing = await MegapowersDenyDestructive({ $: failingShell });
  await assert.doesNotReject(
    hooksFailing["tool.execute.before"](
      { tool: "bash", sessionID: "s", callID: "c" },
      { args: { command: "rm -rf /" } },
    ),
  );
});

test("a deny-class command with shell metacharacters cannot break out of the guard invocation", async () => {
  // Proves the `echo ${payload} | ${GUARD_SCRIPT}` interpolation in
  // deny-destructive.js cannot become an execution vector: the command string
  // itself is untrusted agent input, and it is fed to a guard that runs on
  // EVERY bash call. If interpolation were ever broken, a `$(...)` embedded
  // in the classified command would run in the shell that invokes the guard,
  // not just get read as data by it.
  const dir = mkdtempSync(path.join(tmpdir(), "megapowers-injection-"));
  const witness = path.join(dir, "witness");
  try {
    const command = [
      "rm -rf /", // must still classify as deny
      `"double" 'single'`, // embedded quotes of both kinds
      `$(touch ${witness})`, // a substitution aimed at a side effect we can check
      ";", // a top-level separator
      "\n", // a literal newline
      `$(touch ${witness})`,
    ].join(" ");

    const hooks = await MegapowersDenyDestructive({ $: makeRealShell() });
    await assert.rejects(
      hooks["tool.execute.before"](
        { tool: "bash", sessionID: "s", callID: "c" },
        { args: { command } },
      ),
      (err) => {
        assert.match(err.message, /^megapowers: /);
        assert.match(err.message, /recursive rm of a root, home, or system directory/);
        return true;
      },
    );
    assert.equal(existsSync(witness), false, "the $(...) substitution must never execute");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

// The installation the docs used to call equivalent to a symlink. Node resolves an
// ESM specifier to its realpath, so a symlinked plugin still finds ../hooks/; a COPIED
// one does not, and before this check that copy loaded happily and allowed every
// destructive command it was supposed to stop. A guardrail that cannot find its policy
// must refuse to load, because from inside a session "no guard" and "guard found
// nothing wrong" look identical.
test("a copied install with no sibling hooks/ refuses to load", async () => {
  const dir = mkdtempSync(path.join(tmpdir(), "megapowers-copied-install-"));
  try {
    const copied = path.join(dir, "deny-destructive.js");
    copyFileSync(new URL("../deny-destructive.js", import.meta.url), copied);
    const { MegapowersDenyDestructive: copiedFactory } = await import(pathToFileURL(copied).href);

    await assert.rejects(
      copiedFactory({ $: makeRealShell() }),
      (err) => {
        assert.match(err.message, /cannot find its guard script/);
        assert.match(err.message, /Symlink the plugin instead of copying it/);
        return true;
      },
    );
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
