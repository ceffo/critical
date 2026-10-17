#!/usr/bin/env bash
# Test of check-pr-title.sh: valid titles pass, the others are rejected.
set -euo pipefail
check="$(dirname "$0")/check-pr-title.sh"
status=0

expect() { # expect <accept|reject> <title>
    if "$check" "$2" >/dev/null 2>&1; then got=accept; else got=reject; fi
    if [ "$got" != "$1" ]; then echo "FAIL: '$2' should be ${1}ed"; status=1; fi
}

expect accept 'feat: add spell search'
expect accept 'fix(api): reject an empty name'
expect accept 'feat!: drop the v1 routes'
expect accept 'refactor(web)!: rename the store'
for t in feat fix test docs chore refactor perf; do expect accept "$t: something"; done
expect reject 'chore/sync-beads'
expect reject 'chore/sync-beads (#227)'
expect reject 'wip: something'
expect reject 'Feat: capitalised type'
expect reject 'feat:'
expect reject 'feat: '
expect reject 'feat:no space'
expect reject 'feat(): empty scope'
expect reject 'feat(a b): spaced scope'
expect reject 'feature: unknown type'
expect reject ' feat: leading space'
expect reject ''
exit $status
