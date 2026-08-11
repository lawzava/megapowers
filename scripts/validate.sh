#!/usr/bin/env bash
# validate.sh — structural validation for the megapowers marketplace.
# Deps: jq (required), shellcheck (optional). Run from repo root.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 2

pass=0
fail=0
ok()  { printf '  \033[32mPASS\033[0m %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf '  \033[31mFAIL\033[0m %s\n' "$1"; fail=$((fail + 1)); }

# shellcheck source=lib/validate-helpers.sh
source "$ROOT/scripts/lib/validate-helpers.sh"

# Render a SKILL.md frontmatter description to a single line, resolving both
# single-line values and folded/literal YAML block scalars (>, >-, |, |-, >+,
# |+). Used by the context-budget checks; the plain skills check only needs to
# know the field is present, so it keeps its cheaper first-line sed extraction.
skill_desc() {
  awk '
    BEGIN { st=0; done=0 }                       # st: 0=pre-fm 1=in-fm 2=in-block
    st==0 && /^---/ { st=1; next }
    st==1 && /^---/ { exit }
    st==1 && /^description:/ {
      val=$0; sub(/^description:[[:space:]]*/,"",val)
      if (val ~ /^[|>][+-]?$/) { st=2; next }    # block-scalar opener
      print val; done=1; exit
    }
    st==2 && /^---/          { print out; done=1; exit }
    st==2 && /^[^[:space:]]/ { print out; done=1; exit }   # dedent = next key
    st==2 { line=$0; sub(/^[[:space:]]+/,"",line); out=(out==""?line:out" "line); next }
    END   { if (!done && st==2) print out }
  ' "$1"
}

# Byte length of a string, locale-independent. The budgets below are stated in
# characters; bytes >= chars, so a byte budget is a conservative superset that
# never under-counts (descriptions are near-ASCII, so the two coincide at the
# peak anyway).
byte_len() { printf '%s' "$1" | LC_ALL=C wc -c | tr -d '[:space:]'; }

claude_mp=".claude-plugin/marketplace.json"
codex_mp=".agents/plugins/marketplace.json"

catalog_codex_model() {
  local tier="$1"
  awk -v tier="$tier" '
    /^\[providers\.codex\.tiers\]$/ { in_tiers=1; next }
    in_tiers && /^\[/ { exit }
    in_tiers && $1 == tier {
      value=$3
      gsub(/^"|"$/, "", value)
      print value
      exit
    }
  ' plugins/mega-orchestration/models.toml
}

catalog_opencode_model() {
  local provider="$1" tier="$2"
  awk -v provider="$provider" -v tier="$tier" '
    $0 == "[providers." provider ".tiers]" { in_tiers=1; next }
    in_tiers && /^\[/ { exit }
    in_tiers && $1 == tier {
      value=$3
      gsub(/^"|"$/, "", value)
      print value
      exit
    }
  ' plugins/mega-orchestration/models.toml
}

echo "== marketplace =="
if ! command -v jq >/dev/null 2>&1; then
  bad "jq is required"
else
  ok "jq present"
fi

if [[ -f $claude_mp ]] && jq -e . "$claude_mp" >/dev/null 2>&1; then
  ok "$claude_mp is valid JSON"
  if jq -e '.name and (.plugins | type == "array")' "$claude_mp" >/dev/null 2>&1; then
    ok "Claude marketplace has name + plugins[]"
  else
    bad "Claude marketplace missing name or plugins[]"
  fi
  if jq -e '(.plugins | length) == 7 and all(.plugins[]; has("skills") | not)' "$claude_mp" >/dev/null 2>&1; then
    ok "Claude marketplace publishes seven plugin bundles only"
  else
    bad "Claude marketplace must publish exactly seven plugin bundles and no standalone skill aliases"
  fi
else
  bad "$claude_mp missing or invalid JSON"
fi

if [[ -f $codex_mp ]] && jq -e . "$codex_mp" >/dev/null 2>&1; then
  ok "$codex_mp is valid JSON"
  if jq -e '.name and (.interface.displayName | type == "string") and (.plugins | type == "array")' "$codex_mp" >/dev/null 2>&1; then
    ok "Codex marketplace has name + interface.displayName + plugins[]"
  else
    bad "Codex marketplace missing name, interface.displayName, or plugins[]"
  fi
else
  bad "$codex_mp missing or invalid JSON"
fi

echo "== Claude plugins =="
if [[ -f $claude_mp ]]; then
  # real plugin entries: string source, no skills[] -> must have a matching plugin.json
  while IFS=$'\t' read -r name src; do
    [[ -z $name ]] && continue
    dir="${src#./}"
    pj="$dir/.claude-plugin/plugin.json"
    if [[ -f $pj ]] && jq -e . "$pj" >/dev/null 2>&1; then
      pjn="$(jq -r '.name' "$pj")"
      if [[ $pjn == "$name" ]]; then ok "plugin $name -> $pj"; else bad "$name: plugin.json name '$pjn' != entry '$name'"; fi
    else
      bad "$name: missing/invalid $pj"
    fi
  done < <(jq -r '.plugins[] | select((.source|type=="string") and (has("skills")|not)) | [.name, .source] | @tsv' "$claude_mp" 2>/dev/null)
  # skill-bundle entries (strict:false): have skills[] -> each referenced SKILL.md must exist
  while IFS=$'\t' read -r name src skill; do
    [[ -z $name ]] && continue
    base="${src#./}"; sk="${skill#./}"
    p="$base/$sk/SKILL.md"
    if [[ -f $p ]]; then ok "skill-bundle $name -> $p"; else bad "skill-bundle $name: missing $p"; fi
  done < <(jq -r '.plugins[] | select(has("skills")) | . as $e | $e.skills[] | [$e.name, $e.source, .] | @tsv' "$claude_mp" 2>/dev/null)
fi

echo "== Codex plugins =="
if [[ -f plugins/mega-guardrails/.codex-plugin/plugin.json ]]; then
  ok "mega-guardrails has Codex plugin metadata"
else
  bad "mega-guardrails missing Codex plugin metadata"
fi

codex_frontier_model="$(catalog_codex_model frontier)"
codex_strong_model="$(catalog_codex_model strong)"
if [[ -n $codex_frontier_model ]] && grep -qF "model = \"$codex_frontier_model\"" templates/codex-config.toml; then
  ok "Codex config lead model matches the catalog frontier tier ($codex_frontier_model)"
else
  bad "Codex config lead model must match the catalog frontier tier ($codex_frontier_model)"
fi
# Each native Codex role mirrors the delegates.toml role it stands in for, so it
# is pinned to that role's tier: builder is small_impl (strong), reviewer is
# code_review (frontier). Pinning both to one tier would hide a routing change.
for role in builder reviewer; do
  root_role="templates/codex-agents/$role.toml"
  plugin_role="plugins/mega-orchestration/assets/codex-agents/$role.toml"
  case "$role" in
    builder)  role_tier="strong";   role_model="$codex_strong_model" ;;
    reviewer) role_tier="frontier"; role_model="$codex_frontier_model" ;;
  esac
  if [[ -n $role_model ]] && grep -qF "model = \"$role_model\"" "$root_role" 2>/dev/null; then
    ok "Codex $role role matches the catalog $role_tier tier ($role_model)"
  else
    bad "Codex $role role must match the catalog $role_tier tier ($role_model)"
  fi
  if [[ -f $plugin_role ]] && cmp -s "$root_role" "$plugin_role"; then
    ok "Codex $role role ships with mega-orchestration"
  else
    bad "Codex $role role missing from mega-orchestration assets or differs from root template"
  fi
done
if [[ -f $codex_mp ]]; then
  while IFS=$'\t' read -r name source path install auth category; do
    [[ -z $name ]] && continue
    if [[ $source != "local" ]]; then bad "$name: Codex source '$source' != local"; fi
    if [[ $install != "AVAILABLE" && $install != "INSTALLED_BY_DEFAULT" && $install != "NOT_AVAILABLE" ]]; then bad "$name: invalid Codex installation policy '$install'"; fi
    if [[ $auth != "ON_INSTALL" && $auth != "ON_USE" ]]; then bad "$name: invalid Codex authentication policy '$auth'"; fi
    if [[ -z $category ]]; then bad "$name: missing Codex category"; fi
    dir="${path#./}"
    pj="$dir/.codex-plugin/plugin.json"
    if [[ -f $pj ]] && jq -e . "$pj" >/dev/null 2>&1; then
      pjn="$(jq -r '.name' "$pj")"
      if [[ $pjn == "$name" ]]; then ok "Codex plugin $name -> $pj"; else bad "$name: Codex plugin.json name '$pjn' != entry '$name'"; fi
      if jq -e '.version and .description and .author.name and .interface.displayName and .interface.shortDescription and .interface.longDescription and .interface.developerName and .interface.category and (.interface.capabilities | type == "array") and (.interface.defaultPrompt or .interface.default_prompt)' "$pj" >/dev/null 2>&1; then
        ok "Codex plugin $name has required interface metadata"
      else
        bad "$name: Codex plugin.json missing required interface metadata"
      fi
      # if the manifest declares a skills path, it must actually exist
      skp="$(jq -r '.skills // empty' "$pj" 2>/dev/null)"
      if [[ -n $skp ]]; then
        skdir="$dir/${skp#./}"; skdir="${skdir%/}"
        if [[ -d $skdir ]]; then ok "Codex plugin $name skills path exists: $skp"; else bad "$name: Codex skills path '$skp' missing"; fi
      fi
    else
      bad "$name: missing/invalid $pj"
    fi
  done < <(jq -r '.plugins[] | [.name, .source.source, .source.path, .policy.installation, .policy.authentication, .category] | @tsv' "$codex_mp" 2>/dev/null)
fi

echo "== OpenCode plugins =="
# The OpenCode agent templates carry model = "<opencode-provider-id>/<model-id>";
# the provider prefix is a deliberate placeholder for the reader to substitute
# (the catalog has no mapping to OpenCode's own provider ids), so only the
# suffix is checked against the catalog tier. builder mirrors small_impl on
# moonshot's strong tier; reviewer mirrors code_review on qwen's frontier tier,
# a different vendor on purpose. The same twin-drift risk as the Codex agent
# roles applies here: the plugin asset and the root template must stay
# byte-identical.
opencode_strong_model="$(catalog_opencode_model moonshot strong)"
opencode_frontier_model="$(catalog_opencode_model qwen frontier)"
for role in builder reviewer; do
  root_role="templates/opencode-agents/$role.md"
  plugin_role="plugins/mega-orchestration/assets/opencode-agents/$role.md"
  case "$role" in
    builder)  role_provider="moonshot"; role_tier="strong";   role_model="$opencode_strong_model" ;;
    reviewer) role_provider="qwen";     role_tier="frontier"; role_model="$opencode_frontier_model" ;;
  esac
  model_line="$(sed -n 's/^model:[[:space:]]*//p' "$root_role" 2>/dev/null | head -1)"
  if [[ -n $role_model && $model_line == *"$role_model" ]]; then
    ok "OpenCode $role role model pin ends with the catalog $role_provider $role_tier tier ($role_model)"
  else
    bad "OpenCode $role role model pin must end with the catalog $role_provider $role_tier tier ($role_model)"
  fi
  if [[ -f $plugin_role ]] && cmp -s "$root_role" "$plugin_role"; then
    ok "OpenCode $role role ships with mega-orchestration"
  else
    bad "OpenCode $role role missing from mega-orchestration assets or differs from root template"
  fi
done

# node is an optional external validator here, same as shellcheck and the
# claude CLI above: a stated skip line rather than a failure or a silent pass
# when it is not installed.
if command -v node >/dev/null 2>&1; then
  for js in plugins/megapowers/opencode/session-catalog.js plugins/mega-guardrails/opencode/deny-destructive.js; do
    if node --check "$js" >/dev/null 2>&1; then ok "node --check $js"; else bad "node --check failed: $js"; fi
  done
  # node --test with a directory argument silently misreports on this node
  # version; the glob form is what actually discovers and runs both suites.
  if node --test "plugins/*/opencode/tests/*.test.mjs" >/dev/null 2>&1; then
    ok "node --test plugins/*/opencode/tests/*.test.mjs"
  else
    bad "node --test failed: plugins/*/opencode/tests/*.test.mjs"
  fi
else
  echo "  (node not installed, skipped: OpenCode plugin syntax check and test run)"
fi

oc_json="templates/opencode.json"
if [[ -f $oc_json ]] && jq -e . "$oc_json" >/dev/null 2>&1; then
  ok "$oc_json is valid JSON"
  if jq -e '(.plugin // []) | contains(["<megapowers-checkout>/plugins/megapowers/opencode/session-catalog.js","<megapowers-checkout>/plugins/mega-guardrails/opencode/deny-destructive.js"])' "$oc_json" >/dev/null 2>&1; then
    ok "$oc_json plugin[] names both OpenCode module paths"
  else
    bad "$oc_json plugin[] must name both session-catalog.js and deny-destructive.js"
  fi
  if jq -e '(.permission.bash // {}) as $b | (["git reset --hard*","git clean -f*","git branch -D*","git push --force*","curl * | *sh"] | all(. as $k | $b[$k] == "ask"))' "$oc_json" >/dev/null 2>&1; then
    ok "$oc_json permission.bash carries all five ask patterns"
  else
    bad "$oc_json permission.bash missing one or more of the five ask patterns"
  fi
else
  bad "$oc_json missing or invalid JSON"
fi

opencode_md="templates/OPENCODE.md"
if [[ -f $opencode_md ]] && grep -qF -- '--caller-adapter opencode' "$opencode_md"; then
  ok "$opencode_md documents --caller-adapter opencode"
else
  bad "$opencode_md must document --caller-adapter opencode"
fi

echo "== scratch storage guidance =="
# Marker phrases are prose, so match them against the file with newlines
# collapsed. A line-anchored grep for a sentence passes or fails on where the
# paragraph happens to wrap, which makes a reflow look like a policy change.
flat_file() { tr '\n' ' ' < "$1" | tr -s ' '; }
for template in templates/CODEX.md templates/CLAUDE.md; do
  # Three semantic markers, not the paragraph verbatim: the rules have to be
  # there, the wording is the template author's to tune.
  flat="$(flat_file "$template")"
  if grep -q '^## Scratch storage$' "$template" &&
     [[ $flat == *"Honor \`\$TMPDIR\`"* ]] &&
     [[ $flat == *'writable in the current sandbox'* ]] &&
     [[ $flat == *"Do not silently fall back to \`/tmp\`"* ]]; then
    ok "$template has portable scratch storage guidance"
  else
    bad "$template must guide agents to a writable, capacity-checked \$TMPDIR"
  fi
done
review_rubric="plugins/megapowers/skills/requesting-code-review/review-rubric.md"
rubric_flat="$(flat_file "$review_rubric")"
if grep -qF "git worktree add \"\$TMPDIR/review-<sha>\" <sha>" "$review_rubric" &&
   [[ $rubric_flat == *"Do not silently fall back to \`/tmp\` for a large checkout."* ]] &&
   ! grep -qF '/tmp/review-<sha>' "$review_rubric"; then
  ok "review worktrees honor TMPDIR"
else
  bad "review worktrees must honor TMPDIR instead of hard-coding /tmp"
fi

echo "== skills =="
while IFS= read -r sk; do
  [[ -z $sk ]] && continue
  d="$(basename "$(dirname "$sk")")"
  fm="$(awk 'NR==1 && /^---/ {f=1; next} f && /^---/ {exit} f' "$sk")"
  n="$(printf '%s\n' "$fm" | sed -n 's/^name:[[:space:]]*//p' | head -1)"
  desc="$(printf '%s\n' "$fm" | sed -n 's/^description:[[:space:]]*//p' | head -1)"
  if [[ -n $n && -n $desc ]]; then
    if [[ $n == "$d" ]]; then ok "skill $sk"; else bad "skill $sk: name '$n' != dir '$d'"; fi
  else
    bad "skill $sk: missing name/description frontmatter"
  fi
  unknown="$(printf '%s\n' "$fm" | sed -n 's/^\([A-Za-z0-9_-]\+\):.*/\1/p' | grep -Ev '^(name|description|license|compatibility|disable-model-invocation|disable_model_invocation)$' || true)"
  if [[ -z $unknown ]]; then ok "skill $sk frontmatter portable"; else bad "skill $sk: unsupported frontmatter keys: $(printf '%s' "$unknown" | tr '\n' ' ')"; fi
done < <(find plugins skills -name SKILL.md 2>/dev/null)

echo "== .agents/skills discovery links =="
# Codex and OpenCode discover skills in a checkout through
# .agents/skills/<name> symlinks. A skill added under plugins/ without its link
# is invisible to those harnesses, which is how upgrading-megapowers went
# missing. Exemptions are deliberate, one line each, with the reason.
AGENTS_LINK_EXEMPT="wayfinding"   # explicit-only skill; stays out of implicit discovery
if [[ -d .agents/skills ]]; then
  al_bad=0
  while IFS= read -r sk; do
    [[ -z $sk ]] && continue
    name="$(basename "$(dirname "$sk")")"
    link=".agents/skills/$name"
    if grep -qx -- "$name" <<<"$AGENTS_LINK_EXEMPT"; then
      [[ -e $link ]] && { bad ".agents/skills: $name is exempt but linked anyway"; al_bad=1; }
      continue
    fi
    if [[ ! -L $link ]]; then
      bad ".agents/skills: no discovery symlink for skill '$name' (add: ln -s ../../${sk%/SKILL.md} $link)"
      al_bad=1
    elif [[ ! -f "$link/SKILL.md" ]]; then
      bad ".agents/skills/$name is a dangling symlink"
      al_bad=1
    fi
  done < <(find plugins -name SKILL.md 2>/dev/null | sort)
  # and no orphan links pointing at skills that no longer exist
  while IFS= read -r link; do
    [[ -z $link ]] && continue
    [[ -f "$link/SKILL.md" ]] || { bad ".agents/skills/$(basename "$link") does not resolve to a SKILL.md"; al_bad=1; }
  done < <(find .agents/skills -maxdepth 1 -type l 2>/dev/null | sort)
  [[ $al_bad -eq 0 ]] && ok "every shipped skill has a resolving .agents/skills link (exempt: $AGENTS_LINK_EXEMPT)"
else
  bad ".agents/skills directory missing"
fi

echo "== explicit-only skill reachability =="
# A skill whose sidecar sets allow_implicit_invocation: false is never surfaced by
# implicit discovery, so its ONLY route is another skill naming it. If that
# reference disappears the skill silently becomes unreachable, which is a
# functional regression rather than a wording change. Assert the reference exists;
# say nothing about how it is phrased.
eo_found=0
while IFS= read -r sc; do
  [ -n "$sc" ] || continue
  grep -Eq '^[[:space:]]*allow_implicit_invocation:[[:space:]]*false[[:space:]]*$' "$sc" || continue
  skdir="$(dirname "$(dirname "$sc")")"
  name="$(basename "$skdir")"
  plug="$(basename "$(dirname "$(dirname "$skdir")")")"
  eo_found=1
  # any OTHER shipped skill body naming it as <plugin>:<skill>
  if [[ -n "$(grep -rIl --include=SKILL.md -e "$plug:$name" plugins 2>/dev/null | grep -v "^$skdir/SKILL.md$")" ]]; then
    ok "explicit-only skill '$name' is reachable via a $plug:$name reference"
  else
    bad "explicit-only skill '$name' has no $plug:$name reference in any other skill: implicit discovery is off, so nothing can route to it"
  fi
done < <(find plugins -type f -path '*/skills/*/agents/openai.yaml' 2>/dev/null | sort)
[[ $eo_found -eq 0 ]] && ok "no explicit-only skills to check"

echo "== Codex skill metadata =="
if [[ -x scripts/validate-codex-skill-metadata ]]; then
  if metadata_result="$(scripts/validate-codex-skill-metadata "$ROOT" 2>&1)"; then
    ok "$metadata_result"
  else
    bad "optional agents/openai.yaml validation failed"
    printf '%s\n' "$metadata_result" | sed 's/^/    /'
  fi
else
  bad "scripts/validate-codex-skill-metadata missing or not executable"
fi

echo "== context budgets =="
# Skill descriptions consume discovery context. Claude lists all skills, while
# Codex can omit explicit-only skills from implicit discovery. writing-skills
# states token-efficiency limits for them; these checks make that budget
# visible to CI so drift stops passing silently. All measured in bytes
# (LC_ALL=C) for determinism across CI locales.

# 1. Per-skill description budget: writing-skills keeps each under ~500 chars
#    (Task 12 trimmed the one offender, orchestrating, to 494 — this pins it).
DESC_MAX=500
desc_over=0; desc_peak=0; desc_peak_name=""; desc_sum=0
while IFS= read -r sk; do
  [[ -z $sk ]] && continue
  n="$(byte_len "$(skill_desc "$sk")")"
  desc_sum=$((desc_sum + n))
  if (( n > desc_peak )); then desc_peak=$n; desc_peak_name="$(basename "$(dirname "$sk")")"; fi
  if (( n > DESC_MAX )); then bad "skill description over ${DESC_MAX}B: $(basename "$(dirname "$sk")") (${n}B)"; desc_over=1; fi
done < <(find plugins skills -name SKILL.md 2>/dev/null | sort)
(( desc_over == 0 )) && ok "every skill description within ${DESC_MAX}B (peak: ${desc_peak_name} ${desc_peak}B)"

# 2. Cross-harness upper bound: every skill description plus the real
#    SessionStart hook context. Keep explicit-only Codex skills in this total
#    because other harnesses can still discover them. Invoke the hook and parse
#    its output instead of duplicating its trimming and preface logic here.
ALWAYS_MAX=13800
skfile="plugins/megapowers/skills/using-megapowers/SKILL.md"
session_hook="plugins/megapowers/hooks/session-start"
if [[ -f $skfile && -x $session_hook ]]; then
  hook_json="$(MODELS_TOML="plugins/megapowers/models.toml" "$session_hook" 2>/dev/null || true)"
  hook_context="$(printf '%s' "$hook_json" | jq -er '.hookSpecificOutput.additionalContext' 2>/dev/null || true)"
  hook_bytes="$(byte_len "$hook_context")"
  catalog_block="$(MODELS_TOML="plugins/megapowers/models.toml" plugins/megapowers/hooks/render-model-catalog 2>/dev/null || true)"
  catalog_bytes="$(byte_len "$catalog_block")"
  # 980, raised from 900 on 2026-08-10 with the catalog's growth from two vendors to
  # four. Kept in step with plugins/megapowers/hooks/tests/render-model-catalog.test.sh
  # and 44B inside the hook's own 1024B clamp.
  if [[ -n $catalog_block ]] && (( catalog_bytes <= 980 )); then
    ok "session-start catalog block within 980B (${catalog_bytes}B)"
  else
    bad "session-start catalog block empty or over 980B (${catalog_bytes}B)"
  fi
  always_total=$((desc_sum + hook_bytes))
  if [[ -n $hook_context ]] && (( always_total <= ALWAYS_MAX )); then
    ok "always-loaded context within ${ALWAYS_MAX}B (${always_total}B = ${desc_sum}B descriptions + ${hook_bytes}B SessionStart context)"
  else
    bad "SessionStart context empty or always-loaded context over ${ALWAYS_MAX}B (${always_total}B)"
  fi
else
  bad "using-megapowers skill or SessionStart hook missing (cannot measure always-loaded budget)"
fi

# 3. Post-compaction survival budget. An invoked skill's rendered body stays in the
#    conversation for the whole session, and when Claude Code compacts it re-attaches
#    the most recent invocation of each skill AFTER the summary, keeping the first
#    5,000 tokens of each under a combined 25,000-token cap, filled newest-first.
#    Two failures follow from that and neither is visible while authoring:
#      - a body past the per-skill head is TRUNCATED after the first compaction, so a
#        rule written near the end silently stops applying mid-session;
#      - a deep stack of skills evicts the oldest ones entirely, which on this suite
#        is the always-first process skill.
#    Budgeted in bytes at 4 bytes/token with a 20% margin, because the tokenizer is
#    not ours to run and a budget that assumes the best case is not a budget.
SKILL_HEAD_MAX=16000     # 5,000 tokens * 4 B/token * 0.8
SKILL_STACK_MAX=80000    # 25,000 tokens * 4 B/token * 0.8
SKILL_STACK_DEPTH=6      # the deepest routine stack: using-megapowers -> orchestrating
                         # -> multi-agent-delegation -> a process skill -> TDD -> verification
head_over=0; body_peak=0; body_peak_name=""
while IFS= read -r sk; do
  [[ -z $sk ]] && continue
  n="$(validate_skill_body_bytes "$sk")"
  if (( n > body_peak )); then body_peak=$n; body_peak_name="$(basename "$(dirname "$sk")")"; fi
  if (( n > SKILL_HEAD_MAX )); then
    bad "skill body over the post-compaction head budget of ${SKILL_HEAD_MAX}B: $(basename "$(dirname "$sk")") (${n}B) — everything past it is dropped at the first compaction"
    head_over=1
  fi
done < <(find plugins skills -name SKILL.md 2>/dev/null | sort)
(( head_over == 0 )) && ok "every skill body survives compaction whole (budget ${SKILL_HEAD_MAX}B, peak: ${body_peak_name} ${body_peak}B)"

# The stack bound uses the largest bodies rather than a named route: which skills a
# session stacks is a runtime question, and the worst case is what has to fit.
stack_sum=0
while IFS= read -r n; do
  [[ -z $n ]] && continue
  stack_sum=$((stack_sum + n))
done < <(find plugins skills -name SKILL.md -exec wc -c {} + 2>/dev/null |
         grep -v ' total$' | awk '{print $1}' | sort -rn | head -"$SKILL_STACK_DEPTH")
if (( stack_sum <= SKILL_STACK_MAX )); then
  ok "the ${SKILL_STACK_DEPTH} largest skills co-load within ${SKILL_STACK_MAX}B (${stack_sum}B)"
else
  bad "the ${SKILL_STACK_DEPTH} largest skills exceed the ${SKILL_STACK_MAX}B re-attach budget (${stack_sum}B): a deep stack will evict the earliest process skill after compaction"
fi

echo "== hooks (shellcheck) =="
if command -v shellcheck >/dev/null 2>&1; then
  run_shellcheck_group < <(git ls-files --cached --others --exclude-standard -- plugins scripts evals | while IFS= read -r f; do
    [ -f "$f" ] || continue
    case "$f" in
      *.sh) printf '%s\n' "$f" ;;
      plugins/*/scripts/*|plugins/*/hooks/*|scripts/*)
        head -1 "$f" 2>/dev/null | grep -qE '^#!.*sh' && printf '%s\n' "$f" ;;
    esac
  done | sort -u)
else
  echo "  (shellcheck not installed — skipped)"
fi

echo "== security lint =="
# Deterministic malicious-skill-marker scan over skill bodies, hooks, and
# templates (fetch-in-exec, base64|sh, eval of fetched
# content, bidi/Trojan-Source chars, safety-off instructions). No external
# deps, so it always runs; SECURITY.md advertises it as a CI gate and this
# wires it in. Exit 0 clean, nonzero on any hit (file:line: reason on stdout).
if [[ -x scripts/security-lint.sh ]]; then
  if sl_out="$(scripts/security-lint.sh 2>/dev/null)"; then
    ok "security-lint clean"
  else
    bad "security-lint found markers (see below)"
    printf '%s\n' "$sl_out" | sed 's/^/    /'
  fi
else
  echo "  (scripts/security-lint.sh not present — skipped)"
fi

echo "== skill cross-references =="
ref_bad=0
# validate <plugin>:<skill> references for EVERY plugin namespace, not just megapowers,
# so a broken ref like mega-orchestration:best-of-n or mega-python:foo is caught too.
for pdir in plugins/*/; do
  ns="$(basename "$pdir")"
  while IFS= read -r ref; do
    [ -n "$ref" ] || continue
    if [[ ! -f "plugins/$ns/skills/$ref/SKILL.md" ]]; then
      bad "unresolved reference $ns:$ref"; ref_bad=1
    fi
  done < <(grep -rhoE "$ns:[a-z0-9-]+" plugins --include='*.md' 2>/dev/null | sed "s/^$ns://" | sort -u)
done
[[ $ref_bad -eq 0 ]] && ok "all <plugin>:skill references resolve"

echo "== skill support files (orphans) =="
orphan_bad=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  d="$(dirname "$f")"
  while [[ "$d" != "." && ! -f "$d/SKILL.md" ]]; do d="$(dirname "$d")"; done
  [[ -f "$d/SKILL.md" ]] || continue
  base="$(basename "$f")"; rel="${f#"$d"/}"
  if [[ -z "$(grep -rIl --exclude-dir=.git -e "$base" -e "$rel" "$d" 2>/dev/null | grep -v "^$f$")" ]]; then
    bad "orphaned (unreferenced) skill file: $f"; orphan_bad=1
  fi
done < <(find plugins -type f -path '*/skills/*' ! -name 'SKILL.md' 2>/dev/null)
[[ $orphan_bad -eq 0 ]] && ok "no orphaned skill support files"

echo "== hooks.json =="
legacy_codex_hooks="$(find plugins -type f -path '*/hooks/codex-hooks.json' -print 2>/dev/null)"
if [[ -z $legacy_codex_hooks ]]; then
  ok "no legacy manual Codex hook manifests remain"
else
  bad "legacy manual Codex hook manifests would duplicate plugin hooks: ${legacy_codex_hooks//$'\n'/, }"
fi
while IFS= read -r hj; do
  [ -n "$hj" ] || continue
  if ! jq -e . "$hj" >/dev/null 2>&1; then bad "$hj invalid JSON"; continue; fi
  ok "$hj valid JSON"
  pdir="$(cd "$(dirname "$hj")/.." && pwd)"   # plugin root (hooks/..)
  while IFS= read -r cmdpath; do
    [ -n "$cmdpath" ] || continue
    cmd0="${cmdpath%% *}"                          # executable path, without any args
    resolved="${cmd0/'${CLAUDE_PLUGIN_ROOT}'/$pdir}"
    if [[ -f $resolved ]]; then ok "hook command exists: $cmd0"; else bad "hook command missing: $cmd0 (in $hj)"; fi
    # run-hook.cmd (and dispatch.sh behind it) name sibling scripts as args;
    # validate every one so a wrapped hook script can't be deleted while
    # checks stay green.
    if [[ "$(basename "$cmd0")" == "run-hook.cmd" ]]; then
      rest="${cmdpath#"$cmd0"}"
      for payload in $rest; do
        pres="$(dirname "$resolved")/$payload"
        if [[ -f $pres ]]; then ok "wrapped hook payload exists: $payload"; else bad "wrapped hook payload missing: $payload (in $hj)"; fi
      done
    fi
  done < <(jq -r '.hooks[]?[]?.hooks[]?.command // empty' "$hj" 2>/dev/null)
done < <(find plugins -type f -path '*/hooks/hooks.json' 2>/dev/null | sort)
# dispatch.sh and run-hook.cmd ship as byte-twins in every hooks/ dir (plugins
# cannot locate each other at runtime); any drift is a release bug.
for twin in dispatch.sh run-hook.cmd; do
  twin_ref=""; twin_bad=0
  while IFS= read -r tf; do
    [ -n "$tf" ] || continue
    if [[ -z $twin_ref ]]; then twin_ref="$tf"; continue; fi
    cmp -s "$twin_ref" "$tf" || { bad "hook twin drift: $tf vs $twin_ref"; twin_bad=1; }
  done < <(find plugins -type f -path "*/hooks/$twin" 2>/dev/null | sort)
  [[ -n $twin_ref && $twin_bad -eq 0 ]] && ok "hook twin $twin identical across plugins"
done
# lib-toml.sh is the same kind of byte-twin, but it is not under hooks/ in both
# plugins: render-model-catalog sources it from hooks/, delegate-resolve from the
# multi-agent-delegation skill. Both parse the same models.toml, so a divergent
# copy means the session catalog and the resolver disagree about the same file.
lt_a="plugins/megapowers/hooks/lib-toml.sh"
lt_b="plugins/mega-orchestration/skills/multi-agent-delegation/scripts/lib-toml.sh"
if [[ -f $lt_a && -f $lt_b ]]; then
  if cmp -s "$lt_a" "$lt_b"; then ok "lib-toml.sh identical across plugins"; else bad "lib-toml.sh twin drift: $lt_a vs $lt_b"; fi
else
  bad "lib-toml.sh missing a copy ($lt_a / $lt_b)"
fi

echo "== cross-manifest consistency =="
# a plugin's version must agree across its Claude and Codex manifests
for cl in plugins/*/.claude-plugin/plugin.json; do
  [[ -f $cl ]] || continue
  dir="$(cd "$(dirname "$cl")/.." && pwd)"; base="$(basename "$dir")"
  cx="$dir/.codex-plugin/plugin.json"
  vcl="$(jq -r '.version // empty' "$cl" 2>/dev/null)"
  if [[ -f $cx ]]; then
    vcx="$(jq -r '.version // empty' "$cx" 2>/dev/null)"
    if [[ -n $vcl && $vcl == "$vcx" ]]; then ok "$base version agrees across Claude/Codex ($vcl)"; else bad "$base version drift: claude=$vcl codex=$vcx"; fi
  fi
done
# every Codex plugin manifest must be registered in the Codex marketplace
for cx in plugins/*/.codex-plugin/plugin.json; do
  [[ -f $cx ]] || continue
  n="$(jq -r '.name' "$cx" 2>/dev/null)"
  if jq -e --arg n "$n" '.plugins[]|select(.name==$n)' "$codex_mp" >/dev/null 2>&1; then ok "Codex plugin $n registered in marketplace"; else bad "Codex plugin $n has a manifest but is absent from $codex_mp"; fi
done

echo "== repository hygiene =="
if jq -e '.permissions.deny | all(test("\\*"; "") | not)' templates/settings.example.json >/dev/null 2>&1; then
  ok "secret-path permission denies use exact paths"
else
  bad "templates/settings.example.json permission denies must not use ineffective wildcard paths"
fi
validate_parallel_manifest

echo "== hook tests =="
run_test_group "hook test" "no hook tests found (expected plugins/*/hooks/tests/*.test.sh)" < <(
  find plugins -type f -path '*/hooks/tests/*.test.sh' 2>/dev/null | sort
)

echo "== skill script tests =="
run_test_group "skill script test" "no skill script tests found (expected plugins/*/skills/*/scripts/tests/*.test.sh)" < <(
  find plugins -type f -path '*/skills/*/scripts/tests/*.test.sh' 2>/dev/null | sort
)

echo "== repository script tests =="
scheduler_contract="scripts/tests/validate-helpers.test.sh"
if scheduler_output="$(mktemp)"; then
  scheduler_status=0
  bash "$scheduler_contract" >"$scheduler_output" 2>&1 || scheduler_status=$?
  validate_report_test "repository script test" "$scheduler_contract" "$scheduler_output" "$scheduler_status"
  rm -f "$scheduler_output"
else
  bad "cannot create temporary file for repository script test $scheduler_contract"
fi
run_test_group "repository script test" "no repository script tests found (expected scripts/tests/*.test.sh)" < <(
  find scripts/tests -type f -name '*.test.sh' ! -path "$scheduler_contract" 2>/dev/null | sort
)

echo "== evals =="
if [[ -d evals/scenarios ]]; then
  # A scenario suite without its harness contracts is an incomplete gate.
  run_test_group "eval harness test" "no eval harness tests found" < <(
    find evals/tests evals/studies/tests -type f -name '*.test.sh' 2>/dev/null | sort
  )
  ev_bad=0; ev_n=0
  for sd in evals/scenarios/*/; do
    [[ -d $sd ]] || continue
    id="$(basename "$sd")"; ev_n=$((ev_n + 1))
    [[ -f "$sd/scenario.toml" ]] || { bad "eval $id: missing scenario.toml"; ev_bad=1; }
    [[ -f "$sd/check.sh" ]]      || { bad "eval $id: missing check.sh"; ev_bad=1; }
    kind="$(sed -n 's/^[[:space:]]*kind[[:space:]]*=[[:space:]]*//p' "$sd/scenario.toml" 2>/dev/null | head -1 | tr -d '"' )"
    if [[ $kind == artifact && ! -f "$sd/solve.sh" ]]; then bad "eval $id: artifact scenario needs solve.sh"; ev_bad=1; fi
  done
  if [[ $ev_bad -eq 0 && $ev_n -gt 0 ]]; then ok "all $ev_n eval scenarios well-formed"; fi
  for f in evals/run.sh evals/run-all.sh evals/score.go; do
    [[ -f $f ]] && ok "eval harness present: $f" || bad "eval harness missing: $f"
  done
  # Study oracle mutation selftests are offline-deterministic and credential-free.
  for study_selftest in evals/studies/process-behavior/run-study.sh evals/studies/install-smoke/run-smoke.sh; do
    if [[ -x $study_selftest ]] && "$study_selftest" --selftest >/dev/null 2>&1; then ok "study selftest $study_selftest"; else bad "study selftest $study_selftest"; fi
  done
  if evals/studies/gauntlet/oracle.sh --selftest >/dev/null 2>&1; then ok "study selftest evals/studies/gauntlet/oracle.sh"; else bad "study selftest evals/studies/gauntlet/oracle.sh"; fi
  if evals/coverage-inventory.sh >/dev/null 2>&1; then ok "eval coverage inventory generated"; else bad "eval coverage inventory failed"; fi
else
  echo "  (no evals/ dir — skipped)"
fi

echo "== delegates.toml =="
dr="plugins/mega-orchestration/skills/multi-agent-delegation/scripts/delegate-resolve"
dt="plugins/mega-orchestration/skills/multi-agent-delegation/delegates.toml"
mc_core="plugins/megapowers/models.toml"
mc_orch="plugins/mega-orchestration/models.toml"
if [[ -f $mc_core && -f $mc_orch ]]; then
  # Twin shipped catalogs: plugins cannot locate each other at runtime, so the
  # same file ships in both plugin roots; any drift is a release bug.
  if cmp -s "$mc_core" "$mc_orch"; then ok "shipped models.toml twins identical"; else bad "models.toml twin drift: $mc_core vs $mc_orch"; fi
else
  bad "shipped models.toml missing ($mc_core / $mc_orch)"
fi
if [[ -x $dr && -f $dt ]]; then
  # Pin both stacks to the shipped files so local override layers cannot color CI.
  if DELEGATES_TOML="$dt" MODELS_TOML="$mc_orch" "$dr" --check >/dev/null 2>&1; then
    ok "delegate-resolve --check (shipped delegates.toml + models.toml)"
  else
    bad "delegate-resolve --check failed (run: DELEGATES_TOML=$dt MODELS_TOML=$mc_orch $dr --check)"
  fi
else
  bad "delegate-resolve or shipped delegates.toml missing"
fi

echo "== docs consistency =="
# prose drifts twice went stale within a day of a manifest change; assert the
# hand-written docs against the manifests they describe.
if [[ -f $claude_mp && -f $codex_mp ]] && command -v jq >/dev/null 2>&1; then
  cx_total="$(jq '.plugins|length' "$codex_mp")"
  if [[ $cx_total -eq 7 ]]; then ok "Codex marketplace publishes all seven plugin bundles"; else bad "Codex marketplace expected 7 plugin bundles, found $cx_total"; fi
  release_version="$(sed -n 's/^## \([0-9][0-9.]*\) -.*/\1/p' CHANGELOG.md | head -1)"
  version_drift=0
  while IFS= read -r manifest; do
    [[ "$(jq -r '.version // empty' "$manifest" 2>/dev/null)" == "$release_version" ]] || version_drift=1
  done < <(find plugins -type f \( -path '*/.claude-plugin/plugin.json' -o -path '*/.codex-plugin/plugin.json' \) | sort)
  if [[ -n $release_version && $version_drift -eq 0 ]]; then
    ok "all plugin manifests match latest changelog release ($release_version)"
  else
    bad "plugin manifest versions must match latest changelog release ($release_version)"
  fi
  if grep -qF "/v${release_version}/docs/agent-install.md" README.md && grep -qF "@v${release_version}" docs/agent-install.md && grep -qF "@v${release_version}" docs/setup.md; then
    ok "public install pins match latest changelog release (v$release_version)"
  else
    bad "README/setup/agent-install pins must match latest changelog release (v$release_version)"
  fi
  # every plugin bundle must be mentioned in both user-facing docs (word-bounded:
  # "mega-go" must not be satisfied by a hypothetical "mega-golang" token)
  while IFS= read -r pname; do
    [[ -n $pname ]] || continue
    if grep -qE "(^|[^a-zA-Z0-9_-])${pname}([^a-zA-Z0-9_-]|$)" docs/setup.md 2>/dev/null; then ok "setup.md mentions plugin $pname"; else bad "setup.md never mentions plugin $pname"; fi
    if grep -qE "(^|[^a-zA-Z0-9_-])${pname}([^a-zA-Z0-9_-]|$)" README.md 2>/dev/null; then ok "README mentions plugin $pname"; else bad "README never mentions plugin $pname"; fi
  done < <(jq -r '.plugins[]|select(has("skills")|not)|.name' "$claude_mp")

  codex_support="$(awk '/^## Codex$/{in_section=1;next} /^## /{in_section=0} in_section' docs/harness-support.md)"
  while IFS= read -r pname; do
    [[ -n $pname ]] || continue
    # Here-string, not a pipe. `grep -q` exits at the first match, and under the
    # `set -o pipefail` at the top of this file the unwritten remainder kills the
    # upstream `printf` with SIGPIPE, which pipefail then promotes to the
    # pipeline's status. A plugin listed EARLY in the section therefore reported
    # as missing, intermittently, depending on who won the race. A here-string
    # has no pipeline and no writer to signal. This same construct was the root
    # cause of the intermittent `delegate-resolve --check` failure.
    if grep -qE "(^|[^a-zA-Z0-9_-])${pname}([^a-zA-Z0-9_-]|$)" <<<"$codex_support"; then ok "Codex support matrix mentions $pname"; else bad "Codex support matrix omits $pname"; fi
  done < <(jq -r '.plugins[].name' "$codex_mp")

  while IFS= read -r pname; do
    [[ -n $pname ]] || continue
    if grep -qE "\| \`${pname}\` \|" SECURITY.md; then ok "SECURITY capability table mentions $pname"; else bad "SECURITY capability table omits $pname"; fi
  done < <(jq -r '.plugins[].name' "$codex_mp")
fi
# each plugin README must name every skill directory the plugin ships;
# a plugin that ships skills but has no README is itself a failure, not a skip
for pdir in plugins/*/; do
  rme="$pdir/README.md"
  [[ -d "$pdir/skills" ]] || continue
  if [[ ! -f $rme ]]; then bad "$(basename "$pdir") ships skills but has no README.md"; continue; fi
  miss=""
  for sd in "$pdir"skills/*/; do
    [[ -d $sd ]] || continue
    sname="$(basename "$sd")"
    grep -qE "(^|[^a-zA-Z0-9_-])${sname}([^a-zA-Z0-9_-]|$)" "$rme" || miss="$miss $sname"
  done
  if [[ -z $miss ]]; then ok "$(basename "$pdir") README lists all shipped skills"; else bad "$(basename "$pdir") README missing skills:$miss"; fi
done

echo "== baseline drift =="
# upgrading-megapowers fetches templates/ from raw.githubusercontent to diff the
# baseline across versions. A failed fetch and a clean baseline produce the same
# empty output, so a path rename here would read as "nothing changed" forever.
# Pin every fetched name to a file that actually exists.
drift_ref="plugins/megapowers/skills/upgrading-megapowers/references/channels.md"
skill_body="plugins/megapowers/skills/upgrading-megapowers/SKILL.md"
if grep -qF 'raw.githubusercontent.com/lawzava/megapowers' "$drift_ref" 2>/dev/null; then
  ok "upgrade channel reference fetches baselines from this repository"
else
  bad "upgrade channel reference must fetch baselines from raw.githubusercontent.com/lawzava/megapowers"
fi
# Parse the fetch list itself ("for f in A B C; do"), not any mention of the
# name elsewhere in the file: a stale name in the loop is exactly the typo this
# check exists to catch, and it appears in the jq examples further down too.
drift_names="$(sed -n 's/^for f in \(.*\); do$/\1/p' "$drift_ref" 2>/dev/null | head -1)"
if [[ -z $drift_names ]]; then
  bad "channels.md has no parseable baseline fetch list (expected: for f in <names>; do)"
else
  drift_missing=0
  for tname in $drift_names; do
    [[ -f "templates/$tname" ]] || { bad "channels.md fetches templates/$tname, which does not exist"; drift_missing=1; }
  done
  (( drift_missing == 0 )) && ok "every baseline fetched by channels.md exists in templates/ ($drift_names)"
  for tname in CLAUDE.md CODEX.md settings.example.json; do
    case " $drift_names " in
      *" $tname "*) ok "channels.md fetches baseline $tname" ;;
      *) bad "channels.md fetch list must include baseline $tname" ;;
    esac
  done
fi

if evals/check-portability-boundary.sh >/dev/null 2>&1; then ok "semantic skills keep harness mechanics at the routing boundary"; else bad "semantic skill contains harness-specific command or model ID"; fi
if grep -qF 'did not run' "$skill_body" 2>/dev/null; then
  ok "upgrading-megapowers distinguishes a failed drift check from no drift"
else
  bad "upgrading-megapowers must say the drift check did not run when the fetch fails"
fi

echo "== required files =="
for f in LICENSE ATTRIBUTION.md README.md AGENTS.md CLAUDE.md docs/setup.md docs/harness-support.md docs/agent-install.md templates/settings.example.json "$codex_mp"; do
  if [[ -f $f ]]; then ok "$f present"; else bad "$f missing"; fi
done
if grep -q "Jesse Vincent" ATTRIBUTION.md 2>/dev/null; then ok "ATTRIBUTION credits Superpowers"; else bad "ATTRIBUTION missing Superpowers credit"; fi
if grep -qi "is not" README.md 2>/dev/null; then ok "README has scope (is NOT) section"; else bad "README missing 'is NOT' scope section"; fi
if jq -e . templates/settings.example.json >/dev/null 2>&1; then ok "settings.example.json valid JSON"; else bad "settings.example.json missing/invalid"; fi
# format guard only (huge max-age): every dated-opinion file must still carry a
# parseable review date. AGE is enforced by the scheduled freshness workflow.
if scripts/check-freshness.sh --max-age-days 36500 >/dev/null 2>&1; then ok "dated-opinion files carry parseable review dates"; else bad "dated-opinion date lines broken (run scripts/check-freshness.sh)"; fi

# Every rule that can block a session is declared in a plugin's enforcement.toml,
# and the checker asserts each declaration is complete and wired: the named hook
# exists, the source skill exists, the state is one of the three legal values, the
# promotion is dated, and a named contract_test exists and names the rule.
#
# It does NOT inspect the hook body to prove the declared state is what runs. Two
# attempts to do that textually were defeated, first by a comment and then by an
# unused assignment, so the behavioral proof lives in each rule's contract_test
# and this check verifies the linkage instead. check-enforcement.sh carries the
# full reasoning. Drift between a gate's behavior and the file claiming to
# configure it is a release bug rather than a staleness problem, which is why this
# runs here and not only in the weekly freshness job.
if scripts/check-enforcement.sh >/dev/null 2>&1; then ok "enforcement lifecycle declarations are complete and wired"; else bad "enforcement lifecycle broken (run scripts/check-enforcement.sh)"; fi

echo "== native plugin validate (claude CLI) =="
# CI's plugin-validate job runs the native Claude Code manifest validator with
# --strict; v0.1.6 passed this script locally and then failed that job, forcing
# a second tag. Mirror it here when the CLI is installed so local green predicts
# CI green.
if command -v claude >/dev/null 2>&1; then
  if claude plugin validate --strict "$claude_mp" >/dev/null 2>&1; then
    ok "claude plugin validate marketplace"
  else
    bad "claude plugin validate marketplace (run: claude plugin validate --strict $claude_mp)"
  fi
  for pdir in plugins/*/; do
    if claude plugin validate --strict "$pdir" >/dev/null 2>&1; then
      ok "claude plugin validate $(basename "$pdir")"
    else
      bad "claude plugin validate $(basename "$pdir") (run: claude plugin validate --strict $pdir)"
    fi
  done
else
  echo "  (claude CLI not installed — skipped; CI runs this as the plugin-validate job)"
fi

echo
echo "== summary: $pass passed, $fail failed =="
[[ $fail -eq 0 ]]
