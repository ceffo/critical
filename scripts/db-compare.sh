#!/usr/bin/env bash
# Compares the data of two PostgreSQL databases of the app, table by table and column by column,
# without printing any of it: for each table of the public schema it prints the row count on each
# side, then "same" or the names of the columns whose values differ (rows matched in the order of the
# first column, the key of every table of the app).
#
#   scripts/db-compare.sh <dsn A> <dsn B>
#
# Read-only (every session is read-only). Used to see whether a database's character sheet still
# equals what a fresh boot seeds (docs/deploy.md, "Seed production"). Prints names and counts only.
set -euo pipefail

a="${1:?usage: db-compare.sh <dsn A> <dsn B>}"
b="${2:?usage: db-compare.sh <dsn A> <dsn B>}"
export PGOPTIONS="-c default_transaction_read_only=on"

q() { psql "$1" -X -At -v ON_ERROR_STOP=1 -c "$2"; }

tables="$(q "$a" "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename")"
for t in $tables; do
    na="$(q "$a" "SELECT count(*) FROM \"$t\"")"
    nb="$(q "$b" "SELECT count(*) FROM \"$t\"" 2>/dev/null || echo missing)"
    cols="$(q "$a" "SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '$t' ORDER BY ordinal_position")"
    # Rows in the order of the first column (the key in every table of the app), then of the rest.
    order="$(echo "$cols" | head -n1 | sed 's/.*/"&"/'),$(echo "$cols" | sed 's/.*/"&"::text/' | paste -sd, -)"
    diff=()
    if [ "$nb" != missing ]; then
        for c in $cols; do
            sql="SELECT md5(coalesce(string_agg(coalesce(\"$c\"::text, '<null>'), '|' ORDER BY $order), '')) FROM \"$t\""
            [ "$(q "$a" "$sql")" = "$(q "$b" "$sql" 2>/dev/null || echo x)" ] || diff+=("$c")
        done
    fi
    if [ "$nb" = missing ]; then verdict="missing in B"
    elif [ "${#diff[@]}" = 0 ]; then verdict="same"
    else verdict="differs: ${diff[*]}"; fi
    printf '%-24s A=%-6s B=%-6s %s\n' "$t" "$na" "$nb" "$verdict"
done
