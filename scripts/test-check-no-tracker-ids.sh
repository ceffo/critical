#!/usr/bin/env bash
# Test of check-no-tracker-ids.sh on throwaway git trees: planted Linear and bead ids must be
# rejected (bead ids from the id map), a clean tree passes.
set -euo pipefail
check="$(cd "$(dirname "$0")" && pwd)/check-no-tracker-ids.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
status=0
cri=CR'I'  # split so this file holds no id itself

# tree <name> <idmap: yes|no> <file content>
tree() {
    local d="$tmp/$1"
    mkdir -p "$d/src" "$d/docs"
    printf '%s\n' "$3" >"$d/src/file.go"
    [ "$2" = no ] || printf '%s\n' "{\"critical-zzzzz\": \"${cri}-1\", \"critical-zzzzz.3\": \"${cri}-2\"}" >"$d/docs/beads-id-map.json"
    # excluded places may hold ids
    printf '%s\n' "see critical-zzzzz and ${cri}-5" >"$d/docs/adr.md"
    git -C "$d" init -q && git -C "$d" add -A
}

expect() { # expect <reject|accept> <name> <idmap> <content>
    tree "$2" "$3" "$4"
    if (cd "$tmp/$2" && "$check") >/dev/null 2>&1; then got=accept; else got=reject; fi
    if [ "$got" != "$1" ]; then echo "FAIL: $2 should be ${1}ed"; status=1; fi
}

expect reject linear_id yes "// fixes ${cri}-12"
expect reject linear_id_no_map no "// fixes ${cri}-12"
expect reject bead_id yes '// see critical-zzzzz'
expect reject bead_child yes '// see critical-zzzzz.3'
expect reject bead_bare_child yes '// see zzzzz.3'
expect accept clean yes 'critical-hit, CRI-, CRIME-12, ABCRI-3, MACRI-1, 1.5'
expect accept clean_no_map no 'nothing here'
exit $status
