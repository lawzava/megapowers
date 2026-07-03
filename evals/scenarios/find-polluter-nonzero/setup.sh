#!/usr/bin/env bash
set -e
mkdir -p src/a/b tests/unit bin
: > src/foo.test.ts          # direct child (glob terminal)
: > src/a/b/deep.test.ts     # nested (glob terminal)
: > tests/foo.test.ts        # direct child (literal terminal)
: > tests/unit/foo.test.ts   # nested (literal terminal) — the case a cross-model pass caught
# stub npm so find-polluter's `npm test <file>` is a harmless no-op that creates nothing
printf '#!/bin/sh\nexit 0\n' > bin/npm
chmod +x bin/npm
