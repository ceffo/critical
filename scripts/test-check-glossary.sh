#!/usr/bin/env bash
# Test of check-glossary.sh on throwaway trees: a missing term is rejected; a term, an alias, an
# allowlist line (exact or pattern) and non-domain files are accepted; a stale allowlist line is rejected.
set -euo pipefail
check="$(dirname "$0")/check-glossary.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
status=0

# expect <reject|accept> <name> <go source> <glossary> <allowlist>
expect() {
    local dir="$tmp/$2"
    mkdir -p "$dir/domain/sub/mocks"
    printf '%s\n' "$3" >"$dir/domain/sub/sub.go"
    printf 'type MockOnly struct{}\n' >"$dir/domain/sub/mocks/mock.go"
    printf 'package sub\ntype OnlyInTest struct{}\n' >"$dir/domain/sub/sub_test.go"
    printf '%s\n' "$4" >"$dir/GLOSSARY.md"
    printf '%s\n' "$5" >"$dir/allow.txt"
    if "$check" "$dir/domain" "$dir/GLOSSARY.md" "$dir/allow.txt" >/dev/null 2>&1; then got=accept; else got=reject; fi
    if [ "$got" != "$1" ]; then echo "FAIL: $2 should be ${1}ed"; status=1; fi
}

src=$'package sub\ntype SpellSlot struct{}'
expect reject missing_term "$src" '**Spell**:' ''
expect accept by_term "$src" '**Spell slot** (fr: emplacement):' ''
expect accept by_alias "$src" $'**Slot**:\n_Code_: SpellSlot, Other' ''
expect accept by_allowlist "$src" '' 'SpellSlot  # port'
expect accept by_pattern "$src" '' '*Slot  # suffix'
expect reject stale_allowlist "$src" '**Spell slot**:' 'Gone  # no longer a type'
expect accept grouped $'package sub\ntype (\n\tFoo struct{}\n\tbar int\n)' '**Foo**:' ''
expect reject grouped_missing $'package sub\ntype (\n\tFoo struct{}\n)' '**Bar**:' ''
expect accept unexported_only $'package sub\ntype spellSlot struct{}' '' ''

# A long type list with the allowlisted names first: grep -q finds them while a piped producer is still
# writing, which made a present name look stale (a timing race, so repeated).
pad=$(printf 'X%.0s' $(seq 1 120))
big=$'package sub\ntype (\n\tAaFirst struct{}\n\tAbSecond struct{}'
for i in $(seq 1 700); do big+=$'\n\tFiller'$i$pad' struct{}'; done
big+=$'\n)'
terms='**Aa first**:'$'\n''**Ab second**:'
for i in $(seq 1 700); do terms+=$'\n**Filler'$i$pad'**:'; done
for i in 1 2 3; do
    expect accept "big_allowlist_$i" "$big" "$terms" $'AaFirst  # a\nAbSecond  # b'
done
exit $status
