#!/usr/bin/env bash
# Validate one already-stamped release candidate. This script never mutates,
# tags, publishes, or runs post-publish smoke.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

usage() {
  printf 'usage: release.sh <X.Y.Z>\n' >&2
  exit 2
}

[[ $# -eq 1 ]] || usage
version="$1"
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || usage

command -v git >/dev/null 2>&1 || { printf 'release.sh: git is required\n' >&2; exit 2; }
command -v go >/dev/null 2>&1 || { printf 'release.sh: Go is required\n' >&2; exit 2; }
command -v jq >/dev/null 2>&1 || { printf 'release.sh: jq is required\n' >&2; exit 2; }
grep -q "^## ${version//./\\.} - " CHANGELOG.md || {
  printf "release.sh: CHANGELOG.md has no '## %s - ' entry; write it first\n" "$version" >&2
  exit 2
}
latest_version="$(awk '$1 == "##" { print $2; exit }' CHANGELOG.md)"
[[ $latest_version == "$version" ]] || {
  printf 'release.sh: %s is not the newest CHANGELOG.md version (%s)\n' "$version" "$latest_version" >&2
  exit 2
}
for manifest in plugins/megapowers/.claude-plugin/plugin.json plugins/megapowers/.codex-plugin/plugin.json; do
  jq -e --arg version "$version" '.version == $version' "$manifest" >/dev/null || {
    printf 'release.sh: %s must already declare version %s\n' "$manifest" "$version" >&2
    exit 2
  }
done

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || {
  printf 'release.sh: candidate is not a Git checkout\n' >&2
  exit 2
}
if git show-ref --verify --quiet "refs/tags/v$version"; then
  printf 'release.sh: tag v%s already exists\n' "$version" >&2
  exit 2
fi
untracked="$(mktemp)"
deterministic_results="$(mktemp)"
index_flags="$(mktemp)"
cleanup() { rm -f "$untracked" "$deterministic_results" "$index_flags"; }
trap cleanup EXIT HUP INT TERM
git ls-files --others --exclude-standard -z > "$untracked"
git ls-files -v > "$index_flags"
if ! git diff --quiet || ! git diff --cached --quiet || [[ -s $untracked ]]; then
  printf 'release.sh: candidate must be a clean HEAD before validation\n' >&2
  exit 2
fi
if grep -Eq '^([a-z]|S) ' "$index_flags"; then
  printf 'release.sh: candidate index hides tracked paths with assume-unchanged or skip-worktree\n' >&2
  exit 2
fi

go run evals/studies/installed-ab/run.go --hash-plugin --repo "$ROOT" >/dev/null || {
  printf 'release.sh: candidate plugin tree cannot be verified\n' >&2
  exit 2
}

scripts/validate.sh
bash evals/run-all.sh --json "$deterministic_results"
go run evals/score.go --strict "$deterministic_results" >/dev/null
scripts/check-freshness.sh

printf 'release.sh: validated already-stamped %s; no tag or publish action performed\n' "$version"
