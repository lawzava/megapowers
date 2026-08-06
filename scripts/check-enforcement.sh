#!/usr/bin/env bash
# check-enforcement.sh: keep the enforcement lifecycle honest.
#
#   scripts/check-enforcement.sh [--root DIR]
#
# Every rule that can block a session, and every rule that only advises one,
# is declared in a plugin's enforcement.toml. This script asserts that those
# declarations still describe the code. It exists because the failure this
# repository actually shipped was drift, not a bad rule: the risky-logic gate's
# keyword list lived inline in the hook, nothing recorded that the rule was
# enforced, nothing recorded when it was promoted, and no check could tell that
# the hook and the prose describing it had diverged. A rule whose declaration
# is decorative is worse than no declaration, because a reader trusts it.
#
# Checks, per [rules.<id>] in every plugins/*/enforcement.toml:
#
#   state       present and one of "off", "advisory", "enforced". Anything else
#               is a typo that silently disables a gate, since a consumer
#               comparing against "enforced" treats every other value as off.
#   hook        present, and the path exists relative to the plugin root. A rule
#               naming a hook that is not there cannot be enforced by anything.
#   source      present, and the path exists. This is the skill a reader goes to
#               for the reasoning; a dangling one sends them nowhere.
#   date        an enforced rule carries `promoted`, an advisory rule carries
#               `declared`, both as YYYY-MM-DD. Promotion is the event worth
#               dating: it is when the rule started costing people time.
#   wiring      the named hook file mentions enforcement.toml. THIS is the
#               anti-drift check and the reason the file is worth having. A rule
#               can claim any state it likes; if the hook never reads the rules
#               file, the declaration is fiction and the real behavior is
#               whatever is hardcoded. Grep is enough here: the hook either
#               reaches for the file or it does not.
#   scope       when [rules.<id>.scope] exists it declares a non-empty
#               `keywords` array. An empty keyword list in a security scan
#               matches nothing while looking configured, which is the quiet
#               failure mode a gate must not have.
#
# Exits 0 when every rule passes, 1 on any failure, 2 on a usage error. Prints
# one line per check so a CI failure names the file and the rule to fix.
#
# --root DIR scans DIR instead of the repository root; the tests point it at
# fixture trees, which is also how each assertion here is mutation-tested.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

while [ $# -gt 0 ]; do
  case "$1" in
    --root) ROOT="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

cd "$ROOT" || exit 2

pass=0
fail=0
ok()  { printf '  \033[32mPASS\033[0m %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf '  \033[31mFAIL\033[0m %s\n' "$1"; fail=$((fail + 1)); }

# The restricted-TOML readers this repository already ships, rather than a
# second parser with its own bugs. Sourced from the megapowers copy; validate.sh
# separately asserts it is byte-identical to the mega-orchestration one, so
# which copy is read does not matter.
lib="plugins/megapowers/hooks/lib-toml.sh"
if [ ! -f "$lib" ]; then
  echo "check-enforcement: missing $lib, cannot parse rules files" >&2
  exit 2
fi
# shellcheck source=/dev/null
. "$lib"

echo "== enforcement lifecycle =="

files=()
while IFS= read -r f; do files+=("$f"); done < <(find plugins -maxdepth 2 -name enforcement.toml 2>/dev/null | sort)

if [ "${#files[@]}" -eq 0 ]; then
  # Not a soft warning. The consumers below read these files at runtime, so
  # their absence is a broken install, not an empty configuration.
  bad "no plugins/*/enforcement.toml found"
  echo "== summary: $pass passed, $fail failed =="
  exit 1
fi

for file in "${files[@]}"; do
  plugin_root="$(dirname "$file")"
  rel_plugin="${plugin_root#plugins/}"

  rules=()
  while IFS= read -r r; do rules+=("$r"); done < <(
    # Rule headers only. [rules.<id>.scope] and any other subsection is matched
    # by the second alternation and dropped, so a scope block never reads as a
    # rule with no state.
    awk '{h=$0; gsub(/[[:space:]]/,"",h)}
         h ~ /^\[rules\.[A-Za-z0-9_-]+\]$/ {n=h; sub(/^\[rules\./,"",n); sub(/\]$/,"",n); print n}' "$file"
  )

  if [ "${#rules[@]}" -eq 0 ]; then
    bad "$file declares no [rules.*]"
    continue
  fi

  for id in "${rules[@]}"; do
    sec="rules.$id"
    where="$rel_plugin/$id"

    state="$(toml_scalar_in "$file" "$sec" state)"
    case "$state" in
      off|advisory|enforced) ok "$where state=$state" ;;
      "")  bad "$where has no state (expected off, advisory, or enforced)" ;;
      *)   bad "$where state='$state' is none of off, advisory, enforced; a consumer reads any unknown value as off, so a typo silently disables the rule" ;;
    esac

    hook="$(toml_scalar_in "$file" "$sec" hook)"
    if [ -z "$hook" ]; then
      bad "$where names no hook"
    elif [ ! -f "$plugin_root/$hook" ]; then
      bad "$where names hook '$hook' which does not exist under $plugin_root"
    else
      ok "$where hook exists ($hook)"
      # THE LINKAGE ASSERTION. This checker does NOT try to prove that the hook
      # honors its declared state, because it cannot: that is a behavioral
      # property and every textual approximation of it has been defeated.
      #
      # The history is worth keeping, because the temptation to try again is
      # strong. First cut grepped the hook for this file's name; a COMMENT
      # satisfied it, and the checker's own fixture was that comment. Second cut
      # stripped comments first; an UNUSED ASSIGNMENT satisfied it, and again the
      # fixture demonstrated the hole by assigning the path, testing readability,
      # and then sourcing /dev/null. Both times CI reported the central guarantee
      # of the enforcement lifecycle as met while nothing honored the value. An
      # independent reviewer caught it twice.
      #
      # Text can always be shaped to pass a text test. So the proof moved to
      # where it can actually be made: each rule names a contract_test that
      # drives its hook with different declared states and asserts the behavior
      # differs. Those tests already existed per hook and run in validate.sh.
      # What is left here is the honest part, insisting the linkage exists and
      # points somewhere real, which is the drift a checker CAN detect.
      ctest="$(toml_scalar_in "$file" "$sec" contract_test)"
      if [ -z "$ctest" ]; then
        bad "$where names no contract_test, so nothing proves its declared state is the state that runs"
      elif [ ! -f "$plugin_root/$ctest" ]; then
        bad "$where names contract_test '$ctest' which does not exist under $plugin_root"
      else
        # `grep -c`, not `grep -q`. This file runs under `set -o pipefail`, and
        # `-q` exits on the first match, which kills the upstream reader with
        # SIGPIPE and makes the pipeline report that failure instead. An earlier
        # cut of this very check did exactly that and reported every compliant
        # hook as non-compliant. `-c` consumes all input, so nothing is signalled.
        # The rule ID specifically, NOT a generic enforcement.toml mention. The
        # first cut accepted either, which meant a suite written for a different
        # rule, or any file that merely names the rules file, satisfied the link.
        # A linkage check that any sibling test passes is not a linkage check.
        hits="$(grep -c -F "$id" "$plugin_root/$ctest" || true)"
        if [ "${hits:-0}" -gt 0 ]; then
          ok "$where contract_test names the rule ($ctest)"
        else
          bad "$where contract_test '$ctest' never names '$id', so nothing ties it to this rule"
        fi
      fi
    fi

    source_path="$(toml_scalar_in "$file" "$sec" source)"
    if [ -z "$source_path" ]; then
      bad "$where names no source skill"
    elif [ ! -f "$plugin_root/$source_path" ]; then
      bad "$where names source '$source_path' which does not exist under $plugin_root"
    else
      ok "$where source exists ($source_path)"
    fi

    # Promotion is the dated event. An advisory rule records when it was stated
    # so a later honor-rate reading has a start point to measure from.
    if [ "$state" = "enforced" ]; then datekey=promoted; else datekey=declared; fi
    dateval="$(toml_scalar_in "$file" "$sec" "$datekey")"
    if [[ "$dateval" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
      ok "$where $datekey=$dateval"
    else
      bad "$where has no parseable $datekey date (expected YYYY-MM-DD, got '$dateval')"
    fi

    if toml_section_exists_in "$file" "$sec.scope"; then
      kw_count="$(toml_array_in "$file" "$sec.scope" keywords | grep -c .)"
      if [ "$kw_count" -gt 0 ]; then
        ok "$where scope declares $kw_count keywords"
      else
        bad "$where declares a scope with no keywords; an empty list matches nothing while looking configured"
      fi
    fi
  done
done

echo "== summary: $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
