#!/usr/bin/env bash
set -u
o="$WORKDIR/resolve.out"; [ -f "$o" ] || { echo "no output"; exit 1; }
grep -q "PROVIDER=codex" "$o" || { echo "code_review did not resolve to codex"; exit 1; }
awk '/=== visual ===/{f=1} /=== browser_test ===/{f=0} f&&/PROVIDER=codex/{ok=1} END{exit !ok}' "$o" || { echo "visual did not resolve to codex"; exit 1; }
awk '/=== browser_test ===/{f=1} /=== visual_verify ===/{f=0} f&&/PROVIDER=codex/{ok=1} END{exit !ok}' "$o" || { echo "browser_test did not resolve to codex"; exit 1; }
awk '/=== visual_verify ===/{f=1} /=== unknown ===/{f=0} f&&/PROVIDER=browser/{p=1} f&&/CHANNEL=playwright-cli/{c=1} END{exit !(p&&c)}' "$o" || { echo "visual_verify did not resolve to browser/playwright-cli"; exit 1; }
grep -q "FLOOR=strong:low" "$o" || { echo "resolver did not surface the [defaults] floor"; exit 1; }
awk '/=== unknown ===/{f=1} f&&/rc=3/{ok=1} END{exit !ok}' "$o" || { echo "unknown role should exit 3"; exit 1; }
awk '/=== disabled ===/{f=1} /=== hash-in-quoted-value ===/{f=0} f&&/ENABLED=false/{e=1} f&&/rc=4/{r=1} END{exit !(e&&r)}' "$o" || { echo "disabled provider should be flagged + exit 4"; exit 1; }
grep -q "NOTES=curl -H x#y keep this" "$o" || { echo "a '#' inside a quoted value was truncated (comment-strip bug)"; exit 1; }
awk '/=== missing-provider-section ===/{f=1} f&&/no \[providers.ghost\] section/{m=1} f&&/rc=2/{r=1} END{exit !(m&&r)}' "$o" || { echo "role routed to a missing provider section should error + exit 2"; exit 1; }
awk '/=== bad-args/{f=1} f&&/needs a file argument/{m=1} f&&/rc=2/{r=1} END{exit !(m&&r)}' "$o" || { echo "--config with no file should exit 2, not crash"; exit 1; }

# --- second vendor, fallbacks, --exclude, availability, presets, parse error ---
awk '/=== verify-primary ===/{f=1} /=== verify-exclude-openai ===/{f=0} f&&/PROVIDER=codex/{ok=1} END{exit !ok}' "$o" || { echo "verify did not resolve to its codex primary"; exit 1; }
awk '/=== verify-exclude-openai ===/{f=1} /=== verify-exclude-both ===/{f=0} f&&/PROVIDER=claude/{p=1} f&&/rc=0/{r=1} END{exit !(p&&r)}' "$o" || { echo "verify --exclude openai should fall through to the claude route (rc 0)"; exit 1; }
awk '/=== verify-exclude-both ===/{f=1} /=== fallback-skip-absent ===/{f=0} f&&/no available route/{m=1} f&&/rc=3/{r=1} END{exit !(m&&r)}' "$o" || { echo "verify with both vendors excluded should report no available route + exit 3"; exit 1; }
awk '/=== fallback-skip-absent ===/{f=1} /=== no-available-route ===/{f=0} f&&/PROVIDER=p_present/{p=1} f&&/rc=0/{r=1} END{exit !(p&&r)}' "$o" || { echo "fallback resolution should skip the absent-binary primary and pick the present fallback"; exit 1; }
awk '/=== no-available-route ===/{f=1} /=== preset ===/{f=0} f&&/no available route/{m=1} f&&/rc=3/{r=1} END{exit !(m&&r)}' "$o" || { echo "excluded + absent candidates should yield no available route + exit 3"; exit 1; }
awk '/=== preset ===/{f=1} /=== parse-error ===/{f=0} f&&/PRESET=read_only/{p=1} f&&/SANDBOX=read-only/{s=1} f&&/rc=0/{r=1} END{exit !(p&&s&&r)}' "$o" || { echo "--preset read_only should resolve the presets table (rc 0)"; exit 1; }
awk '/=== parse-error ===/{f=1} f&&/parse error/{m=1} f&&/line 2/{l=1} f&&/rc=2/{r=1} END{exit !(m&&l&&r)}' "$o" || { echo "unparseable TOML should exit 2 naming the line (not the exit-3 no-route)"; exit 1; }

echo "ok: routing resolver executes correctly"
