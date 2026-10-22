#!/usr/bin/env bash
# Refuse a class, class group or race id literal in production code (ADR 0040 step 10): the code
# reads a definition's fields and feature kinds, ids live in catalogue data, seeds and tests.
# Scans api/internal/**/*.go and web/src/**/*.{ts,tsx}, but not tests (_test.go, *.test.ts(x),
# *.spec.ts), testdata/ or comments. A string literal equal to an id is refused.
# The ids are read from the catalogue fixture (classes, groups, races), plus the ones listed in
# extra_ids for definitions the fixture does not carry yet. scripts/class-ids-allowlist.txt holds the
# few literals that are allowed, each with a reason.
# Usage: check-no-class-ids.sh [root] [catalogue]   (default: the repo root, api/testdata/catalogue.json)
set -euo pipefail

root=${1:-$(git rev-parse --show-toplevel)}
here=$(cd "$(dirname "$0")" && pwd)
catalogue=${2:-$root/api/testdata/catalogue.json}
allowlist=${CLASS_IDS_ALLOWLIST:-$here/class-ids-allowlist.txt}

# Definitions not yet in the catalogue fixture (ADR 0043 adds the psionicist).
extra_ids=(psionicist)

ids=$( { jq -r '[.classes[].id, .groups[].id, .races[].id] | .[]' "$catalogue"; printf '%s\n' "${extra_ids[@]}"; } | sort -u)

# Words that are not class ids even when they spell one: none today. Spell lists are capitalised
# ("Wizard", "Priest") and compared case-sensitively, so they never match.
idre=$(printf '%s\n' "$ids" | paste -sd'|')

mapfile -t files < <(cd "$root" && {
    [ -d api/internal ] && find api/internal \( -name testdata -o -name node_modules \) -prune -o -type f -name '*.go' ! -name '*_test.go' -print
    [ -d web/src ] && find web/src \( -name node_modules -o -name testdata \) -prune -o -type f \( -name '*.ts' -o -name '*.tsx' \) ! -name '*.test.ts' ! -name '*.test.tsx' ! -name '*.spec.ts' ! -name '*.spec.tsx' -print
} | sort)

found=""
for f in "${files[@]}"; do
    # Blank the comments (keeping the line numbers), then list the string literals that are an id.
    hits=$(perl -0777 -ne '
        BEGIN { ($re, $go) = @ARGV; @ARGV = () }
        # A Go raw string is a struct tag (the catalogue adapter\x27s parsing of data) or SQL: not read.
        s/`[^`]*`/``/gs if $go;
        s{("(?:\\.|[^"\\\n])*"|\x27(?:\\.|[^\x27\\\n])*\x27|`[^`]*`)|//[^\n]*|/\*(.*?)\*/}{
            defined $1 ? $1 : (defined $2 ? ($2 =~ s/[^\n]//gr) : "")
        }gse;
        my $n = 0;
        for my $line (split /\n/, $_, -1) {
            $n++;
            while ($line =~ /(["\x27`])($re)\1/g) { print "$n:$2\n" }
        }
    ' "$idre" "$([[ $f == *.go ]] && echo 1 || echo 0)" <"$root/$f" || true)
    [ -z "$hits" ] && continue
    while IFS=: read -r line id; do
        key="$f:$id"
        if [ -f "$allowlist" ] && grep -v '^#' "$allowlist" | awk '{print $1}' | grep -qxF "$key"; then continue; fi
        found+="$f:$line: \"$id\""$'\n'
    done <<<"$hits"
done

if [ -n "$found" ]; then
    printf '%s' "$found"
    echo "error: class, group or race id literal in production code (ADR 0040 step 10). Read it from the definition or the feature kind; ids live in catalogue data, seeds and tests."
    exit 1
fi
