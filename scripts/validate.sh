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

claude_mp=".claude-plugin/marketplace.json"
codex_mp=".agents/plugins/marketplace.json"

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

echo "== Antigravity manifests =="
while IFS= read -r pj; do
  [[ -z $pj ]] && continue
  dir="$(basename "$(dirname "$pj")")"
  if jq -e . "$pj" >/dev/null 2>&1; then
    pjn="$(jq -r '.name' "$pj")"
    if [[ $pjn == "$dir" ]]; then ok "Antigravity plugin $dir -> $pj"; else bad "$pj: name '$pjn' != dir '$dir'"; fi
    extra="$(jq -r 'keys[] | select(. != "$schema" and . != "name" and . != "description")' "$pj" 2>/dev/null | tr '\n' ' ')"
    if [[ -z $extra ]]; then ok "Antigravity plugin $dir schema-compatible keys"; else bad "$pj: unsupported keys: $extra"; fi
  else
    bad "$pj missing/invalid JSON"
  fi
done < <(find plugins -mindepth 2 -maxdepth 2 -name plugin.json 2>/dev/null)

echo "== hooks (shellcheck) =="
if command -v shellcheck >/dev/null 2>&1; then
  while IFS= read -r h; do
    [ -n "$h" ] || continue
    if shellcheck -S warning "$h" >/dev/null 2>&1; then ok "shellcheck $h"; else bad "shellcheck $h"; fi
  done < <(
    {
      find plugins scripts evals -type f -name '*.sh' 2>/dev/null
      # extensionless executable shell scripts under */scripts/ and */hooks/
      # (e.g. SDD helpers, the session-start hook) — shebang-detected
      find plugins -type f \( -path '*/scripts/*' -o -path '*/hooks/*' \) ! -name '*.*' 2>/dev/null | while IFS= read -r f; do
        head -1 "$f" 2>/dev/null | grep -qE '^#!.*sh' && printf '%s\n' "$f"
      done
    } | sort -u
  )
else
  echo "  (shellcheck not installed — skipped)"
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
    # run-hook.cmd <name> dispatches to <hooks_dir>/<name>; validate that payload
    # too, so a wrapper-invoked hook script can't be deleted while checks stay green.
    if [[ "$(basename "$cmd0")" == "run-hook.cmd" ]]; then
      rest="${cmdpath#"$cmd0"}"; rest="${rest# }"; payload="${rest%% *}"
      if [[ -n $payload ]]; then
        pres="$(dirname "$resolved")/$payload"
        if [[ -f $pres ]]; then ok "wrapped hook payload exists: $payload"; else bad "wrapped hook payload missing: $payload (in $hj)"; fi
      fi
    fi
  done < <(jq -r '.hooks[]?[]?.hooks[]?.command // empty' "$hj" 2>/dev/null)
done < <(find plugins -type f -path '*/hooks/hooks.json' 2>/dev/null | sort)

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

echo "== hook tests =="
ht_found=0
while IFS= read -r t; do
  [ -n "$t" ] || continue
  ht_found=1
  if bash "$t" >/dev/null 2>&1; then ok "hook test $t"; else bad "hook test $t"; fi
done < <(find plugins -type f -path '*/hooks/tests/*.test.sh' 2>/dev/null | sort)
[[ $ht_found -eq 1 ]] || bad "no hook tests found (expected plugins/*/hooks/tests/*.test.sh)"

echo "== evals =="
if [[ -d evals/scenarios ]]; then
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
else
  echo "  (no evals/ dir — skipped)"
fi

echo "== docs consistency =="
# prose drifts twice went stale within a day of a manifest change; assert the
# hand-written docs against the manifests they describe.
if [[ -f $claude_mp && -f $codex_mp ]] && command -v jq >/dev/null 2>&1; then
  cl_total="$(jq '.plugins|length' "$claude_mp")"
  cl_bundles="$(jq '[.plugins[]|select(has("skills")|not)]|length' "$claude_mp")"
  cl_standalone="$(jq '[.plugins[]|select(has("skills"))]|length' "$claude_mp")"
  cx_total="$(jq '.plugins|length' "$codex_mp")"
  # fold line wraps before matching: the doc sentence may wrap mid-pattern
  setup_flat="$(tr '\n' ' ' < docs/setup.md 2>/dev/null)"
  if printf '%s' "$setup_flat" | grep -q "currently ${cl_total}: ${cl_bundles} plugin bundles plus ${cl_standalone} standalone"; then
    ok "setup.md Claude marketplace count matches manifest (${cl_total} = ${cl_bundles}+${cl_standalone})"
  else
    bad "setup.md Claude marketplace count drifted (manifest: ${cl_total} = ${cl_bundles} bundles + ${cl_standalone} standalone)"
  fi
  # anchor with the closing paren so "currently 1" cannot false-match "currently 15: ..."
  if printf '%s' "$setup_flat" | grep -q "(currently ${cx_total})"; then
    ok "setup.md Codex marketplace count matches manifest (${cx_total})"
  else
    bad "setup.md Codex marketplace count drifted (manifest: ${cx_total})"
  fi
  # every plugin bundle must be mentioned in both user-facing docs (word-bounded:
  # "mega-go" must not be satisfied by a hypothetical "mega-golang" token)
  while IFS= read -r pname; do
    [[ -n $pname ]] || continue
    if grep -qE "(^|[^a-zA-Z0-9_-])${pname}([^a-zA-Z0-9_-]|$)" docs/setup.md 2>/dev/null; then ok "setup.md mentions plugin $pname"; else bad "setup.md never mentions plugin $pname"; fi
    if grep -qE "(^|[^a-zA-Z0-9_-])${pname}([^a-zA-Z0-9_-]|$)" README.md 2>/dev/null; then ok "README mentions plugin $pname"; else bad "README never mentions plugin $pname"; fi
  done < <(jq -r '.plugins[]|select(has("skills")|not)|.name' "$claude_mp")
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

echo "== required files =="
for f in LICENSE ATTRIBUTION.md README.md AGENTS.md CLAUDE.md docs/setup.md docs/tool-support.md docs/agent-install.md templates/settings.example.json "$codex_mp"; do
  if [[ -f $f ]]; then ok "$f present"; else bad "$f missing"; fi
done
if grep -q "Jesse Vincent" ATTRIBUTION.md 2>/dev/null; then ok "ATTRIBUTION credits Superpowers"; else bad "ATTRIBUTION missing Superpowers credit"; fi
if grep -qi "is not" README.md 2>/dev/null; then ok "README has scope (is NOT) section"; else bad "README missing 'is NOT' scope section"; fi
if jq -e . templates/settings.example.json >/dev/null 2>&1; then ok "settings.example.json valid JSON"; else bad "settings.example.json missing/invalid"; fi
# format guard only (huge max-age): every dated-opinion file must still carry a
# parseable review date. AGE is enforced by the scheduled freshness workflow.
if scripts/check-freshness.sh --max-age-days 36500 >/dev/null 2>&1; then ok "dated-opinion files carry parseable review dates"; else bad "dated-opinion date lines broken (run scripts/check-freshness.sh)"; fi

echo
echo "== summary: $pass passed, $fail failed =="
[[ $fail -eq 0 ]]
