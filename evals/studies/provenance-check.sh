#!/usr/bin/env bash
# provenance-check.sh <study-results-dir>
# Reject a scored cell assembled from runs with different deterministic inputs.
set -euo pipefail

DIR="${1:?usage: provenance-check.sh <study-results-dir>}"
rows="$(mktemp)"; trap 'rm -f "$rows"' EXIT
found=0
while IFS= read -r meta; do
  found=1
  p="$(dirname "$meta")/provenance.json"
  [ -f "$p" ] || { echo "provenance: missing provenance.json beside $meta" >&2; exit 1; }
  jq -e '
    .schema_version == 1 and
    ([.repo_sha, .source_sha256, .harness_sha256, .agent_cli, .model_identity, .prompt_sha256] | all(type == "string" and length > 0)) and
    ((.cli_version == null) or ((.cli_version | type) == "string")) and
    ((.effort == null) or ((.effort | type) == "string")) and
    (.skill_hashes | type == "array") and
    all(.skill_hashes[]; (.skill | type == "string" and length > 0) and (.sha256 | type == "string" and length > 0))
  ' "$p" >/dev/null || { echo "provenance: invalid schema in $p" >&2; exit 1; }
  key="$(jq -r '[.probe // .task // "default", .model // "unknown", .mode // "default"] | join("|")' "$meta")"
  fingerprint="$(jq -cS '{schema_version,repo_sha,source_sha256,harness_sha256,agent_cli,cli_version,model_identity,effort,prompt_sha256,skill_hashes}' "$p")"
  printf '%s\t%s\t%s\n' "$key" "$fingerprint" "$p" >> "$rows"
done < <(find "$DIR" -name meta.json -type f | sort)
[ "$found" -eq 1 ] || { echo "provenance: no scored meta.json files under $DIR" >&2; exit 1; }

awk -F '\t' '
  !seen[$1]++ { fingerprint[$1]=$2; next }
  $2 != fingerprint[$1] && !reported[$1]++ {
    print "provenance: incompatible deterministic inputs in cell " $1 > "/dev/stderr"
    bad=1
  }
  END { exit bad }
' "$rows"
