// OpenCode plugin: injects megapowers' model catalog into the system prompt,
// the same block Claude Code's session-start hook and Codex's
// codex-session-catalog.sh inject at session start. OpenCode has no
// SessionStart-equivalent hook, so this rides the system-prompt transform
// instead, appending on every chat request rather than once at start.
//
// `experimental.chat.system.transform` is UNDOCUMENTED upstream (present and
// triggered in opencode 1.18.16, absent from opencode.ai/docs/plugins) and may
// change shape or vanish in a later release. This module must never throw out
// of the hook: every filesystem/shell call is wrapped, and any failure is a
// silent no-op rather than a broken chat turn. See README.md for the loading
// and versioning caveats.
import { join } from "node:path";
import { existsSync } from "node:fs";

export const MegapowersSessionCatalog = async ({ $ }) => {
  const scriptPath = join(import.meta.dirname, "..", "hooks", "render-model-catalog");

  // Same copy-versus-symlink trap as the guardrail plugin: node resolves an ESM
  // specifier to its realpath, so a SYMLINK into ~/.config/opencode/plugins/ keeps
  // this relative path pointing into the checkout, while a COPY leaves it pointing
  // at a ../hooks/ that does not exist.
  //
  // This one warns and carries on where the guardrail throws, and the difference is
  // what the failure costs. A missing catalog is visible: the session simply does not
  // have the block, and nothing false has been claimed. A missing guardrail is
  // invisible and actively misleading, so it refuses to load at all. Do not make
  // these two consistent; they are calibrated to their blast radius.
  const scriptFound = existsSync(scriptPath);
  if (!scriptFound) {
    console.error(
      `megapowers: session-catalog cannot find ${scriptPath}, so no model catalog will be injected. ` +
        "Symlink the plugin instead of copying it, or load it from inside the checkout.",
    );
  }

  // Render the catalog block once per process and reuse it for every session
  // and every turn; the catalog does not change while the process is alive.
  // A failed render is cached too (as ""), so a broken script fails the same
  // silent way on every call instead of re-shelling out on each turn.
  let catalogPromise;
  const renderCatalog = () => {
    if (!catalogPromise) {
      catalogPromise = (async () => {
        // Do not spawn a subprocess that cannot succeed. The warning above already
        // said why, and shelling out to a path known to be absent would add a failed
        // exec to the noise without changing the outcome.
        if (!scriptFound) return "";
        try {
          const text = await $`${scriptPath}`.nothrow().text();
          return text.trim();
        } catch {
          return "";
        }
      })();
    }
    return catalogPromise;
  };

  // output.system is a fresh array per chat request (the host builds and
  // discards it per request; nothing carries over), so the transform must
  // append every time it's called. There is no "already announced this
  // session" state to keep. What IS cached is renderCatalog() above: the
  // append happens every request, but the script runs once per process.
  return {
    "experimental.chat.system.transform": async (input, output) => {
      try {
        const catalog = await renderCatalog();
        if (!catalog) return;

        // The SDK's `Model` carries the model id as `id`; only `chat.message` uses
        // the `{providerID, modelID}` shape. Read both, because this hook is
        // undocumented and its input shape is not a promise anyone made.
        const providerID = input?.model?.providerID;
        const modelID = input?.model?.id ?? input?.model?.modelID;

        // No id, no identity line. An earlier version defaulted to the string
        // "unknown" and shipped `--caller-model unknown` into live sessions, which
        // delegate-resolve rejects outright (exit 2: matches no model in any tier
        // map). A session that says nothing falls back to the catalog [lead] and is
        // merely wrong about who is calling; a session that declares "unknown"
        // cannot resolve a route at all. Silence is the better failure.
        const identity =
          modelID && providerID
            ? `This session runs ${providerID}/${modelID} on the opencode adapter. ` +
              `Route resolution takes --caller-model ${modelID} --caller-adapter opencode.`
            : "";

        output.system.push(identity ? `${catalog}\n\n${identity}` : catalog);
      } catch {
        // Undocumented hook, undocumented shell surface: never break a chat
        // turn over a missing catalog or a shape change upstream.
      }
    },
  };
};
