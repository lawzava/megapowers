#!/usr/bin/env bash
# Stamp one already-certified release candidate. This script never tags,
# publishes, or runs post-publish smoke.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

usage() {
  printf 'usage: release.sh <X.Y.Z> --certificate-root <path>\n' >&2
  exit 2
}

[[ $# -eq 3 && $2 == --certificate-root ]] || usage
version="$1"
certificate_root="$3"
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || usage

command -v git >/dev/null 2>&1 || { printf 'release.sh: git is required\n' >&2; exit 2; }
command -v go >/dev/null 2>&1 || { printf 'release.sh: Go is required\n' >&2; exit 2; }
command -v jq >/dev/null 2>&1 || { printf 'release.sh: jq is required\n' >&2; exit 2; }
grep -q "^## ${version//./\\.} - " CHANGELOG.md || {
  printf "release.sh: CHANGELOG.md has no '## %s - ' entry; write it first\n" "$version" >&2
  exit 2
}

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || {
  printf 'release.sh: candidate is not a Git checkout\n' >&2
  exit 2
}
head_revision="$(git rev-parse HEAD)"
untracked="$(mktemp)"
deterministic_results="$(mktemp)"
cleanup() { rm -f "$untracked" "$deterministic_results"; }
trap cleanup EXIT HUP INT TERM
git ls-files --others --exclude-standard -z > "$untracked"
if ! git diff --quiet || ! git diff --cached --quiet || [[ -s $untracked ]]; then
  printf 'release.sh: candidate must be a clean HEAD before certification and stamping\n' >&2
  exit 2
fi

[[ -d $certificate_root ]] || {
  printf 'release.sh: certificate root is missing: %s\n' "$certificate_root" >&2
  exit 2
}

validate_certificate() {
  local harness="$1"
  local publish="$certificate_root/$harness/publish"
  local manifest="$publish/manifest.json"
  local results="$publish/results.jsonl"
  local gates="evals/studies/installed-ab/gates.json"
  local cases="evals/studies/installed-ab/cases.json"
  local entry entry_count=0 model effort treatment_hash control_hash minimum_pairs minimum_lift confidence

  [[ -d $publish && ! -L $publish ]] || {
    printf 'release.sh: %s publish directory is missing or unsafe\n' "$harness" >&2
    return 1
  }
  while IFS= read -r -d '' entry; do
    entry_count=$((entry_count + 1))
    case "$entry" in
      "$manifest"|"$results") ;;
      *) printf 'release.sh: unexpected %s publish artifact: %s\n' "$harness" "${entry#"$publish/"}" >&2; return 1 ;;
    esac
  done < <(find "$publish" -mindepth 1 -maxdepth 1 -print0)
  [[ $entry_count -eq 2 && -f $manifest && ! -L $manifest && -f $results && ! -L $results ]] || {
    printf 'release.sh: %s publish bundle must contain only manifest.json and results.jsonl\n' "$harness" >&2
    return 1
  }

  minimum_pairs="$(jq -er '.code_quality.minimum_paired_runs' "$gates")"
  minimum_lift="$(jq -er '.code_quality.minimum_absolute_lift' "$gates")"
  confidence="$(jq -er '.code_quality.confidence_level' "$gates")"
  jq -e --arg harness "$harness" \
    --argjson minimum_pairs "$minimum_pairs" \
    --argjson minimum_lift "$minimum_lift" \
    --argjson confidence "$confidence" '
    . as $manifest |
    type == "object" and
    .schema_version == "1" and
    .study == "installed-plugin-ab" and
    .evidence == "credentialed-behavioral-run" and
    .harness == $harness and
    (.model | type == "string" and length > 0) and
    (.effort | type == "string" and length > 0) and
    (.broker_hash | test("^sha256:[0-9a-f]{64}$")) and
    (.treatment_plugin_hash | test("^sha256:[0-9a-f]{64}$")) and
    .empty_control_plugin_hash == "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" and
    .treatment_plugin_hash != .empty_control_plugin_hash and
    .publishability.publishable == true and
    .publishability.minimum_paired_runs == $minimum_pairs and
    .publishability.minimum_absolute_lift == $minimum_lift and
    .publishability.confidence_level == $confidence and
    (.arms | type == "array" and length > 0) and
    all(.arms[];
      .evidence == "credentialed-behavioral-run" and
      ((.arm == "treatment" and .plugin_inventory == ["megapowers"] and
        .inventory_hash == "sha256:88e40f75e81a425ed2408db9ee50732561aef45d592f1dc5dc6d7a977260903e" and
        .plugin_hash == $manifest.treatment_plugin_hash) or
       (.arm == "control" and .plugin_inventory == [] and
        .inventory_hash == "sha256:37517e5f3dc66819f61f5a7bb8ace1921282415f10551d2defa5c3eb0985b570" and
        .plugin_hash == $manifest.empty_control_plugin_hash)))
  ' "$manifest" >/dev/null || {
    printf 'release.sh: %s manifest is not credentialed installed-plugin A/B evidence\n' "$harness" >&2
    return 1
  }

  model="$(jq -er .model "$manifest")"
  effort="$(jq -er .effort "$manifest")"
  treatment_hash="$(jq -er .treatment_plugin_hash "$manifest")"
  control_hash="$(jq -er .empty_control_plugin_hash "$manifest")"
  jq -se --slurpfile manifest "$manifest" --slurpfile cases "$cases" \
    --arg harness "$harness" --arg revision "$head_revision" \
    --arg model "$model" --arg effort "$effort" \
    --arg treatment_hash "$treatment_hash" --arg control_hash "$control_hash" '
    ($manifest[0]) as $m |
    length > 0 and
    length == ($m.arms | length) and
    ([.[].case_id] | unique | sort) == ([$cases[0].cases[].id] | unique | sort) and
    all(.[]; . as $row |
        .study == "installed-plugin-ab" and
        .evidence_class == "behavioral" and
        .harness.name == $harness and
        .harness.model == $model and
        .harness.effort == $effort and
        .source.revision == $revision and
        ((.arm == "treatment" and .plugin_hash == $treatment_hash) or
         (.arm == "control" and .plugin_hash == $control_hash)) and
        ((.metrics.report_only // 0) != 1 or
         any($cases[0].cases[]; .id == $row.case_id and .kind == "autonomy_status"))) and
    all(.[];
        . as $row | any($m.arms[];
            .case_id == $row.case_id and
            .block_id == $row.block_id and
            .arm == $row.arm and
            .prompt_hash == $row.prompt_hash and
            .fixture_hash == $row.fixture_hash and
            .plugin_hash == $row.plugin_hash))
  ' "$results" >/dev/null || {
    printf 'release.sh: %s results do not match the manifest or clean HEAD %s\n' "$harness" "$head_revision" >&2
    return 1
  }

  go run evals/score.go --strict --publishable-gates "$gates" "$results" >/dev/null || {
    printf 'release.sh: %s results failed strict publishability scoring\n' "$harness" >&2
    return 1
  }
}

# Candidate evidence first. No deterministic gate or file mutation happens
# until both harness certificates match the frozen clean revision.
validate_certificate claude
validate_certificate codex

bash evals/run-all.sh --json "$deterministic_results"
go run evals/score.go --strict "$deterministic_results" >/dev/null
scripts/check-freshness.sh

stamp_manifest() {
  local manifest="$1" tmp
  tmp="$(mktemp)"
  jq --arg version "$version" '.version = $version' "$manifest" > "$tmp"
  mv "$tmp" "$manifest"
}
stamp_manifest plugins/megapowers/.claude-plugin/plugin.json
stamp_manifest plugins/megapowers/.codex-plugin/plugin.json

printf 'release.sh: certified and stamped %s; no tag or publish action performed\n' "$version"
