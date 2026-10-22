#!/usr/bin/env bash
# Test of check-no-class-ids.sh: a class, group or race id literal planted in a production file is
# refused; the same ids in tests, testdata, comments, Go struct tags and the allowlist pass.
set -euo pipefail
check="$(dirname "$0")/check-no-class-ids.sh"
repo=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
status=0

# A tree with the fixture catalogue and one file; the allowlist is empty unless a fixture sets one.
expect() { # expect <reject|accept> <name> <path> <content> [allowlist line]
    local dir="$tmp/$2"
    mkdir -p "$dir/api/testdata" "$dir/$(dirname "$3")"
    cp "$repo/api/testdata/catalogue.json" "$dir/api/testdata/catalogue.json"
    printf '%s\n' "$4" >"$dir/$3"
    printf '%s\n' "${5:-}" >"$dir/allowlist.txt"
    if CLASS_IDS_ALLOWLIST="$dir/allowlist.txt" "$check" "$dir" >/dev/null 2>&1; then got=accept; else got=reject; fi
    if [ "$got" != "$1" ]; then echo "FAIL: $2 should be ${1}ed"; status=1; fi
}

go_prod=api/internal/domain/service/x/x.go
ts_prod=web/src/lib/x.ts

expect reject go_class "$go_prod" 'package x
func f(c string) bool { return c == "cleric" }'
expect reject go_group "$go_prod" 'package x
var g = []string{"rogue"}'
expect reject go_race "$go_prod" 'package x
var r = map[string]int{"half_elf": 1}'
expect reject go_psionicist "$go_prod" 'package x
const p = "psionicist"'
expect reject ts_class "$ts_prod" 'export const isThief = (c: string) => c === "thief";'
expect reject ts_single_quote "$ts_prod" "export const c = 'bard';"
expect reject ts_template "$ts_prod" 'export const c = `mage`;'
expect reject tsx_group web/src/components/X.tsx 'export const X = () => <i>{"warrior"}</i>;'
expect accept go_test_file api/internal/domain/service/x/x_test.go 'package x
var c = "cleric"'
expect accept go_testdata api/internal/domain/service/x/testdata/x.go 'package x
var c = "cleric"'
expect accept ts_test_file web/src/lib/x.test.ts 'export const c = "thief";'
expect accept ts_spec_file web/src/lib/x.spec.tsx 'export const c = "thief";'
expect accept go_line_comment "$go_prod" 'package x
// the "cleric" casts; "dwarf" too
var a = 1 // not "elf"'
expect accept go_block_comment "$go_prod" 'package x
/* "mage" and
   "gnome" */
var a = 1'
expect accept ts_comment "$ts_prod" '/** The "bard" reads the wizard list. */
export const a = 1; // "halfling"'
expect accept go_struct_tag "$go_prod" 'package x
type T struct { Warrior int `json:"warrior"` }'
expect accept go_substring "$go_prod" 'package x
var a = "clerical error"
var b = "elfin"'
expect accept spell_list_case "$go_prod" 'package x
var a = "Wizard"
var b = "Priest"'
expect accept allowlisted "$go_prod" 'package x
var a = "wizard"' "$go_prod:wizard  spell list"
expect reject allowlist_other_id "$go_prod" 'package x
var a = "wizard"' "$go_prod:priest  spell list"
expect reject allowlist_other_file "$go_prod" 'package x
var a = "wizard"' "api/internal/y.go:wizard  spell list"
exit $status
