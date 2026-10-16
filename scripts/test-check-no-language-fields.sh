#!/usr/bin/env bash
# Test of check-no-language-fields.sh: each fixture must be rejected, a clean file must pass.
set -euo pipefail
check="$(dirname "$0")/check-no-language-fields.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
status=0

expect() { # expect <reject|accept> <name> <content>
    local dir="$tmp/$2"
    mkdir -p "$dir"
    printf '%s\n' "$3" >"$dir/fixture"
    if "$check" "$dir" >/dev/null 2>&1; then got=accept; else got=reject; fi
    if [ "$got" != "$1" ]; then echo "FAIL: $2 should be ${1}ed"; status=1; fi
}

expect reject sql_column 'ALTER TABLE spells ADD COLUMN name_fr TEXT;'
expect reject go_field 'NameFR string'
expect reject go_field_mixed 'NameFr string'
expect reject ts_property 'const x = { nameFr: "a" }'
expect reject json_tag 'Name string `json:"name_fr"`'
expect reject english_variant 'description_en TEXT'
expect accept clean 'names: { en: "a", fr: "b" }  // French, from, free, Africa'
exit $status
