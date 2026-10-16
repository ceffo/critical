#!/usr/bin/env bash
# Refuse a per-language identifier (name_fr, NameFR, nameFr, "name_fr" JSON tag, SQL column): a text
# in a human language is a row of localized_texts, never a column or field per language (ADR 0023).
# Usage: check-no-language-fields.sh [dir]   (no dir: scan the tracked files of the repo)
set -euo pipefail

# snake_case suffix _fr/_en, or camelCase/PascalCase suffix Fr/FR/En/EN glued to a lowercase letter.
pattern='(_(fr|en)|[a-z](FR|Fr|EN|En))\b'

# Not scanned: docs (ADRs narrate the old names), the tracker, and the applied migrations that
# created and dropped the per-language columns (an applied migration never changes) with the
# tests that replay them.
excludes=(
    ':!.beads' ':!docs'
    ':!api/internal/storage/sqlite/migrations/027*' ':!api/internal/storage/sqlite/migrations/029*'
    ':!api/internal/storage/sqlite/migrations/030*' ':!api/internal/storage/sqlite/migrations/031*'
    ':!api/internal/storage/sqlite/migrate_030_test.go' ':!api/internal/storage/sqlite/migrate_031_test.go'
    ':!scripts/test-check-no-language-fields.sh'
)

if [ $# -ge 1 ]; then
    found=$(grep -rnIE "$pattern" "$1" || true)
else
    found=$(git grep -nIE "$pattern" -- . "${excludes[@]}" || true)
fi
if [ -n "$found" ]; then
    echo "$found"
    echo "error: per-language identifier above. Store the text as localized_texts rows and serve a language map (ADR 0023)."
    exit 1
fi
