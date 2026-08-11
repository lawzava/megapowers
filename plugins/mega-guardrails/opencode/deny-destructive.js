// OpenCode port of the deny-destructive accident tripwire. Enforces ONLY the
// DENY tier of ../hooks/deny-destructive.sh: OpenCode 1.18.16 exposes no
// plugin hook capable of raising a confirmation prompt, so the ASK tier
// (destructive git, curl | bash) is out of scope here and is instead covered
// declaratively by the `permission.bash` patterns in templates/opencode.json.
// See ../README.md for the full breakdown.
import { fileURLToPath } from "node:url";
import path from "node:path";
import { existsSync } from "node:fs";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const GUARD_SCRIPT = path.join(HERE, "..", "hooks", "deny-destructive.sh");

export const MegapowersDenyDestructive = async ({ $ }) => {
  // A missing guard script is an INSTALLATION error and fails loudly, which is the
  // opposite of how this module treats a guard that runs and misbehaves. The
  // asymmetry is deliberate. A guard that cannot be found is indistinguishable, from
  // inside a session, from a guard that found nothing to complain about: every bash
  // call sails through and the operator believes a tripwire is armed. Silence there
  // is the one failure this file can make that is worse than being noisy.
  //
  // This is a live footgun, not a hypothetical. Node resolves an ESM specifier to its
  // realpath, so SYMLINKING this file into ~/.config/opencode/plugins/ keeps the
  // relative path above pointing back into the checkout and works. COPYING it there
  // does not: ../hooks/ resolves beside the copy and the script is absent. Both were
  // once documented as equivalent installs.
  if (!existsSync(GUARD_SCRIPT)) {
    throw new Error(
      `megapowers: deny-destructive cannot find its guard script at ${GUARD_SCRIPT}. ` +
        "This plugin shells out to the bash tripwire in mega-guardrails/hooks/ and is inert without it. " +
        "Symlink the plugin instead of copying it (node resolves the symlink back into the checkout), " +
        "or load it through the `plugin` array in opencode.json pointing at the file inside the checkout.",
    );
  }

  return {
    "tool.execute.before": async (input, output) => {
      if (input.tool !== "bash") return;
      const command = output?.args?.command;
      if (!command) return;

      let stdout;
      try {
        const payload = JSON.stringify({ tool_input: { command } });
        const result = await $`echo ${payload} | ${GUARD_SCRIPT}`.nothrow().quiet();
        stdout = result.text().trim();
      } catch {
        return; // fail open: a guard that cannot run must not block the agent
      }
      if (!stdout) return; // no output: allow (matches the bash hook's own contract)

      let parsed;
      try {
        parsed = JSON.parse(stdout);
      } catch {
        return; // fail open: unparseable output must never block the agent
      }

      const decision = parsed?.hookSpecificOutput?.permissionDecision;
      if (decision === "deny") {
        const reason = parsed?.hookSpecificOutput?.permissionDecisionReason;
        throw new Error("megapowers: " + reason);
      }
      // "ask" (and anything else) is out of scope for this plugin: no throw.
    },
  };
};
