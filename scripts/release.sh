#!/usr/bin/env bash
# release.sh <version> — stamp a release version everywhere it is duplicated.
# Writes .version into every plugin manifest (Claude + Codex) and rewrites the
# public install pins in README.md, docs/agent-install.md, and docs/setup.md.
# Requires a matching "## <version> - " heading in CHANGELOG.md first, so the
# changelog entry is written before the stamp. Idempotent. Deps: jq.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

version="${1:-}"
if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: release.sh <X.Y.Z>" >&2
  exit 2
fi
if ! grep -q "^## ${version//./\\.} - " CHANGELOG.md; then
  echo "release.sh: CHANGELOG.md has no '## $version - ' entry; write it first" >&2
  exit 2
fi
command -v jq >/dev/null 2>&1 || { echo "release.sh: jq is required" >&2; exit 2; }

# In-place sed portably: GNU sed takes -i with no argument, BSD sed needs an
# explicit (empty) backup suffix.
if sed --version >/dev/null 2>&1; then
  sedi() { sed -Ei "$@"; }
else
  sedi() { sed -Ei '' "$@"; }
fi

for manifest in plugins/*/.claude-plugin/plugin.json plugins/*/.codex-plugin/plugin.json; do
  [[ -f $manifest ]] || continue
  tmp="$(mktemp)"
  jq --arg v "$version" '.version = $v' "$manifest" > "$tmp"
  mv "$tmp" "$manifest"
done

# Public install pins. Patterns match any prior X.Y.Z so restamping is safe.
sedi "s|/v[0-9]+\.[0-9]+\.[0-9]+/docs/agent-install\.md|/v${version}/docs/agent-install.md|g" README.md
sedi "s|@v[0-9]+\.[0-9]+\.[0-9]+|@v${version}|g" docs/agent-install.md docs/setup.md
sedi "s|\"ref\": \"v[0-9]+\.[0-9]+\.[0-9]+\"|\"ref\": \"v${version}\"|g" docs/setup.md
sedi "s|through \`v[0-9]+\.[0-9]+\.[0-9]+\` are the release pin range|through \`v${version}\` are the release pin range|" docs/setup.md

echo "release.sh: stamped $version into plugin manifests and doc pins"
