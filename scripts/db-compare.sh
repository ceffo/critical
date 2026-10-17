#!/usr/bin/env bash
# Compares the data of two PostgreSQL databases of the app, table by table and column by column,
# without printing any of it: for each table of the public schema it prints the row count on each
# side, then "same" or the names of the columns whose values differ (rows matched in the order of the
# first column, the key of every table of the app).
#
#   scripts/db-compare.sh <dsn A> <dsn B>
#
# Read-only (every session is read-only). Used to see whether a database's character sheet still
# equals what a fresh boot seeds (docs/deploy.md, "Production data"). Prints names and counts only.
set -euo pipefail

a="${1:?usage: db-compare.sh <dsn A> <dsn B>}"
b="${2:?usage: db-compare.sh <dsn A> <dsn B>}"
export PGOPTIONS="-c default_transaction_read_only=on"

q() { psql "$1" -X -At -v ON_ERROR_STOP=1 -c "$2"; }

# Any failing query (a connection or SQL error, on either side) stops the run with psql's message:
# a table or column absent from B's public schema is reported as missing, never guessed from an error.
list="SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename"
tables="$(q "$a" "$list")"
tables_b="$(q "$b" "$list")"
for t in $tables; do
    na="$(q "$a" "SELECT count(*) FROM \"$t\"")"
    if grep -qxF "$t" <<<"$tables_b"; then nb="$(q "$b" "SELECT count(*) FROM \"$t\"")"; else nb=missing; fi
    colsq="SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '$t' ORDER BY ordinal_position"
    cols="$(q "$a" "$colsq")"
    diff=()
    missing=()
    if [ "$nb" != missing ]; then
        cols_b="$(q "$b" "$colsq")"
        for c in $cols; do grep -qxF "$c" <<<"$cols_b" || missing+=("$c"); done
    fi
    if [ "$nb" != missing ] && [ "${#missing[@]}" = 0 ]; then
        # Rows in the order of the first column (the key in every table of the app), then of the rest.
        order="$(echo "$cols" | head -n1 | sed 's/.*/"&"/'),$(echo "$cols" | sed 's/.*/"&"::text/' | paste -sd, -)"
        for c in $cols; do
            sql="SELECT md5(coalesce(string_agg(coalesce(\"$c\"::text, '<null>'), '|' ORDER BY $order), '')) FROM \"$t\""
            [ "$(q "$a" "$sql")" = "$(q "$b" "$sql")" ] || diff+=("$c")
        done
    fi
    if [ "$nb" = missing ]; then verdict="missing in B"
    elif [ "${#missing[@]}" != 0 ]; then verdict="columns missing in B: ${missing[*]}"
    elif [ "${#diff[@]}" = 0 ]; then verdict="same"
    else verdict="differs: ${diff[*]}"; fi
    printf '%-24s A=%-6s B=%-6s %s\n' "$t" "$na" "$nb" "$verdict"
done
