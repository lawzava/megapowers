// Tests for the OpenCode session-catalog plugin. Run via
// `node --test "plugins/*/opencode/tests/*.test.mjs"` (also wired into
// scripts/validate.sh). The glob form is deliberate: passing the directory
// misreports on node 24.
//
// `$` is faked rather than shelling out to the real render-model-catalog
// script: the plugin only cares that `$` is a BunShell-shaped tagged
// template returning something with `.nothrow().text()`, so the fake models
// exactly that surface and nothing more.
import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, rmSync, copyFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { MegapowersSessionCatalog } from "../session-catalog.js";

const CATALOG = "Model catalog (models.toml; override layers project>user>shipped):\nlead: acme frontier (acme-max)\n";

function fakeShell(text) {
  return () => ({
    nothrow() {
      return this;
    },
    text() {
      return Promise.resolve(text);
    },
  });
}

function throwingShell() {
  return () => {
    throw new Error("render-model-catalog: command not found");
  };
}

// Counts invocations so the test below can assert the script is shelled out
// to only once, even though the hook itself fires on every chat request.
function countingShell(text) {
  const calls = { count: 0 };
  const shell = () => {
    calls.count++;
    return {
      nothrow() {
        return this;
      },
      text() {
        return Promise.resolve(text);
      },
    };
  };
  return { shell, calls };
}

test("appends exactly one system entry containing the catalog text", async () => {
  const hooks = await MegapowersSessionCatalog({ $: fakeShell(CATALOG), directory: "/repo" });
  const output = { system: [] };
  await hooks["experimental.chat.system.transform"](
    { sessionID: "s1", model: { providerID: "anthropic", id: "claude-opus-5" } },
    output,
  );
  assert.equal(output.system.length, 1);
  assert.match(output.system[0], /Model catalog \(models\.toml/);
});

// The lead line is rendered, not appended: the plugin tells the script which
// harness it is rendering for, so an OpenCode session reads "lead: opencode"
// instead of the catalog [lead] plus a correction buried under the block.
test("renders the catalog for the opencode adapter", async () => {
  const commands = [];
  const recordingShell = (strings, ...values) => {
    commands.push(String.raw({ raw: strings }, ...values));
    return fakeShell(CATALOG)();
  };
  const hooks = await MegapowersSessionCatalog({ $: recordingShell, directory: "/repo" });
  await hooks["experimental.chat.system.transform"](
    { sessionID: "s1", model: { providerID: "anthropic", id: "claude-opus-5" } },
    { system: [] },
  );
  assert.equal(commands.length, 1);
  assert.match(commands[0], /--caller opencode/);
});

test("the entry names the opencode adapter and the session model id", async () => {
  const hooks = await MegapowersSessionCatalog({ $: fakeShell(CATALOG), directory: "/repo" });
  const output = { system: [] };
  await hooks["experimental.chat.system.transform"](
    { sessionID: "s1", model: { providerID: "anthropic", id: "claude-opus-5" } },
    output,
  );
  assert.match(output.system[0], /--caller-adapter opencode/);
  assert.match(output.system[0], /claude-opus-5/);
});

test("appends to a fresh system[] on every call, but shells out to the catalog script only once", async () => {
  // output.system is rebuilt per chat request by the host (verified against
  // the 1.18.16 bundle), so "idempotent" cannot mean "append once ever" — it
  // means the expensive part (the script) runs once while the cheap part
  // (the append) happens on every request.
  const { shell, calls } = countingShell(CATALOG);
  const hooks = await MegapowersSessionCatalog({ $: shell, directory: "/repo" });
  const input = { sessionID: "s1", model: { providerID: "anthropic", id: "claude-opus-5" } };

  const outputA = { system: [] };
  await hooks["experimental.chat.system.transform"](input, outputA);
  const outputB = { system: [] };
  await hooks["experimental.chat.system.transform"](input, outputB);

  assert.equal(outputA.system.length, 1);
  assert.equal(outputB.system.length, 1);
  assert.match(outputA.system[0], /Model catalog \(models\.toml/);
  assert.match(outputB.system[0], /Model catalog \(models\.toml/);
  assert.equal(calls.count, 1);
});

test("resolves silently and leaves system[] untouched when $ throws", async () => {
  const hooks = await MegapowersSessionCatalog({ $: throwingShell(), directory: "/repo" });
  const output = { system: [] };
  await assert.doesNotReject(() =>
    hooks["experimental.chat.system.transform"](
      { sessionID: "s1", model: { providerID: "anthropic", id: "claude-opus-5" } },
      output,
    ),
  );
  assert.deepEqual(output.system, []);
});

// The mirror of the guardrail plugin's copied-install test. This plugin resolves
// ../hooks/render-model-catalog relative to its own file, so a copy into an opencode
// plugins directory cannot find it. Unlike the guardrail, a missing catalog is
// harmless and self-evident, so the contract here is warn once and carry on rather
// than refuse to load; this test pins that difference so nobody "fixes" it into
// throwing later.
test("a copied install warns once and degrades to injecting nothing", async () => {
  const dir = mkdtempSync(path.join(tmpdir(), "megapowers-copied-catalog-"));
  const errors = [];
  const realError = console.error;
  console.error = (...args) => errors.push(args.join(" "));
  try {
    const copied = path.join(dir, "session-catalog.js");
    copyFileSync(new URL("../session-catalog.js", import.meta.url), copied);
    const { MegapowersSessionCatalog: copiedFactory } = await import(pathToFileURL(copied).href);

    // The fake `$` below would happily hand back a catalog if it were called. It must
    // not be: with the script absent there is nothing to run, so the module skips the
    // shell-out entirely. Asserting the fake was never invoked is what makes this a
    // test of the module rather than a test of the fake.
    let shellCalls = 0;
    const countingShell = (...args) => {
      shellCalls++;
      return fakeShell(CATALOG)(...args);
    };
    const hooks = await copiedFactory({ $: countingShell });
    assert.equal(errors.length, 1, "exactly one warning, not one per turn");
    assert.match(errors[0], /cannot find/);
    assert.match(errors[0], /Symlink the plugin instead of copying it/);

    const output = { system: ["base"] };
    await hooks["experimental.chat.system.transform"](
      { sessionID: "s", model: { providerID: "acme", id: "acme-max" } },
      output,
    );
    assert.deepEqual(output.system, ["base"], "a copied install must inject nothing");
    assert.equal(shellCalls, 0, "no subprocess may be spawned for a script that is absent");
    assert.equal(errors.length, 1, "the transform must not warn again per request");
  } finally {
    console.error = realError;
    rmSync(dir, { recursive: true, force: true });
  }
});

// Regression: 0.11.0 read the model id as `input.model.modelID`, but the SDK's
// `Model` type carries it as `id` (only chat.message uses modelID). Live sessions
// therefore got `--caller-model unknown`, a value delegate-resolve refuses with
// exit 2, so the one feature that makes a BYO-model runtime declarable emitted a
// flag that could not resolve. The fixtures had invented the wrong shape, which is
// why the suite stayed green while production was broken. These three pin the
// contract against the real type.
test("reads the model id from the SDK's `id` field", async () => {
  const hooks = await MegapowersSessionCatalog({ $: fakeShell(CATALOG) });
  const output = { system: [] };
  await hooks["experimental.chat.system.transform"](
    { sessionID: "s1", model: { providerID: "opencode-go", id: "qwen3.8-max" } },
    output,
  );
  assert.match(output.system[0], /--caller-model qwen3\.8-max --caller-adapter opencode/);
  assert.doesNotMatch(output.system[0], /unknown/);
});

test("still reads the chat.message-style modelID shape", async () => {
  const hooks = await MegapowersSessionCatalog({ $: fakeShell(CATALOG) });
  const output = { system: [] };
  await hooks["experimental.chat.system.transform"](
    { sessionID: "s1", model: { providerID: "opencode", modelID: "kimi-k3" } },
    output,
  );
  assert.match(output.system[0], /--caller-model kimi-k3 --caller-adapter opencode/);
});

test("omits the identity line entirely when the model id is absent", async () => {
  const hooks = await MegapowersSessionCatalog({ $: fakeShell(CATALOG) });
  const output = { system: [] };
  await hooks["experimental.chat.system.transform"]({ sessionID: "s1", model: {} }, output);
  assert.equal(output.system.length, 1, "the catalog still goes in");
  assert.match(output.system[0], /Model catalog/);
  assert.doesNotMatch(output.system[0], /unknown/);
  assert.doesNotMatch(output.system[0], /--caller-model/);
});
