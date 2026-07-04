#!/usr/bin/env bash
set -u
o="$WORKDIR/resolve.out"; [ -f "$o" ] || { echo "no output"; exit 1; }
grep -q "PROVIDER=codex" "$o" || { echo "code_review did not resolve to codex"; exit 1; }
awk '/=== visual ===/{f=1} /=== browser_test ===/{f=0} f&&/PROVIDER=codex/{ok=1} END{exit !ok}' "$o" || { echo "visual did not resolve to codex"; exit 1; }
awk '/=== browser_test ===/{f=1} /=== visual_verify ===/{f=0} f&&/PROVIDER=codex/{ok=1} END{exit !ok}' "$o" || { echo "browser_test did not resolve to codex"; exit 1; }
awk '/=== visual_verify ===/{f=1} /=== unknown ===/{f=0} f&&/PROVIDER=browser/{p=1} f&&/CHANNEL=playwright-cli/{c=1} END{exit !(p&&c)}' "$o" || { echo "visual_verify did not resolve to browser/playwright-cli"; exit 1; }
grep -q "FLOOR=sonnet:low" "$o" || { echo "resolver did not surface the [defaults] floor"; exit 1; }
awk '/=== unknown ===/{f=1} f&&/rc=3/{ok=1} END{exit !ok}' "$o" || { echo "unknown role should exit 3"; exit 1; }
awk '/=== disabled ===/{f=1} f&&/ENABLED=false/{e=1} f&&/rc=4/{r=1} END{exit !(e&&r)}' "$o" || { echo "disabled provider should be flagged + exit 4"; exit 1; }
grep -q "NOTES=curl -H x#y keep this" "$o" || { echo "a '#' inside a quoted value was truncated (comment-strip bug)"; exit 1; }
awk '/=== missing-provider-section ===/{f=1} f&&/no \[providers.ghost\] section/{m=1} f&&/rc=2/{r=1} END{exit !(m&&r)}' "$o" || { echo "role routed to a missing provider section should error + exit 2"; exit 1; }
awk '/=== bad-args/{f=1} f&&/needs a file argument/{m=1} f&&/rc=2/{r=1} END{exit !(m&&r)}' "$o" || { echo "--config with no file should exit 2, not crash"; exit 1; }
echo "ok: routing resolver executes correctly"
