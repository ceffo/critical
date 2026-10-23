#!/usr/bin/env bash
# Prints the catalogue formats the API source in <app source dir> reads, one per line: the
# `Formats` list of api/internal/domain/catalogue/catalogue.go with its named constants resolved.
# The release build asks it of the commit it builds (scripts/release-catalogue.sh), so a catalogue
# the app at that commit cannot read is caught before the deploy. A Go test keeps this parse equal
# to the list the API really uses (api/internal/domain/catalogue/formats_script_test.go).
#
#   scripts/catalogue-formats.sh [<app source dir>]    (default: the repository root)
set -euo pipefail

root="${1:-$(cd "$(dirname "$0")/.." && pwd)}"
src="$root/api/internal/domain/catalogue/catalogue.go"
[ -f "$src" ] || { echo "catalogue-formats: no $src" >&2; exit 1; }

names="$(sed -n 's/^var Formats = \[\]int{\(.*\)}$/\1/p' "$src" | tr -d ' ' | tr ',' '\n' | sed '/^$/d')"
[ -n "$names" ] || { echo "catalogue-formats: no 'var Formats = []int{...}' on one line in $src" >&2; exit 1; }
while IFS= read -r name; do
    if [[ "$name" =~ ^[0-9]+$ ]]; then
        echo "$name"
        continue
    fi
    value="$(sed -n "s/^const $name = \([0-9][0-9]*\)\$/\1/p" "$src")"
    [[ "$value" =~ ^[0-9]+$ ]] || { echo "catalogue-formats: cannot resolve $name in $src" >&2; exit 1; }
    echo "$value"
done <<<"$names"
