#!/usr/bin/env bash
# Test of check-no-tracker-ids.sh on throwaway git trees: planted Linear and bead ids must be
# rejected (bead ids from the archive and, once it is gone, from the id map), a clean tree passes.
set -euo pipefail
check="$(cd "$(dirname "$0")" && pwd)/check-no-tracker-ids.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
status=0
cri=CR'I'  # split so this file holds no id itself

# tree <name> <archive: yes|no> <idmap: yes|no> <file content>
tree() {
    local d="$tmp/$1"
    mkdir -p "$d/src" "$d/.beads" "$d/docs"
    printf '%s\n' "$4" >"$d/src/file.go"
    printf '%s\n' '{"id":"critical-abcde"}' '{"id":"critical-abcde.2"}' >"$d/.beads/issues.jsonl"
    [ "$2" = yes ] || rm -rf "$d/.beads"
    [ "$3" = no ] || printf '%s\n' "{\"critical-zzzzz\": \"${cri}-1\", \"critical-zzzzz.3\": \"${cri}-2\"}" >"$d/docs/beads-id-map.json"
    # excluded places may hold ids
    printf '%s\n' "see critical-abcde and ${cri}-5" >"$d/docs/adr.md"
    git -C "$d" init -q && git -C "$d" add -A
}

expect() { # expect <reject|accept> <name> <archive> <idmap> <content>
    tree "$2" "$3" "$4" "$5"
    if (cd "$tmp/$2" && "$check") >/dev/null 2>&1; then got=accept; else got=reject; fi
    if [ "$got" != "$1" ]; then echo "FAIL: $2 should be ${1}ed"; status=1; fi
}

expect reject linear_id yes yes "// fixes ${cri}-12"
expect reject linear_id_no_sources no no "// fixes ${cri}-12"
expect reject bead_id_archive yes no '// see critical-abcde'
expect reject bead_child_archive yes no '// see abcde.7'
expect reject bead_id_map_only no yes '// see critical-zzzzz'
expect reject bead_child_map_only no yes '// see critical-zzzzz.3'
expect accept clean yes yes 'critical-hit, CRI-, CRIME-12, ABCRI-3, MACRI-1, 1.5'
expect accept clean_no_sources no no 'nothing here'
exit $status
