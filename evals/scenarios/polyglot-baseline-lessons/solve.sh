#!/usr/bin/env bash
P="$ROOT/plugins/mega-python/skills"; T="$ROOT/plugins/mega-ts/skills"
present(){ if grep -qiE "$1" "$2" 2>/dev/null; then echo "OK  $3"; else echo "BAD $3"; fi; }
{
  py="$P/greenfield-python-stack/SKILL.md"
  present 'block the event loop|to_thread'            "$py" "py: async no-block"
  present 'gather|TaskGroup'                          "$py" "py: concurrent gather"
  present 'StaticPool|shared connection|per-connection' "$py" "py: sqlite test-DB footgun"
  present 'outermost|added .last|logs the 429'        "$py" "py: logging outermost"
  present 'forwarded|proxy'                           "$py" "py: trusted proxy"
  ts="$T/greenfield-ts-stack/SKILL.md"
  present 'floating'                                  "$ts" "ts: no floating promises"
  present 'Promise.all'                               "$ts" "ts: concurrent Promise.all"
  present 'outermost|logger first|logs the'          "$ts" "ts: logging outermost"
  present 'forwarded|proxy'                           "$ts" "ts: trusted proxy"
  present 'share one connection|in-memory DB is tied|shared' "$ts" "ts: shared test-DB"
} > lint.out
cat lint.out
