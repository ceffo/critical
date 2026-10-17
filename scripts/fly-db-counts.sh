#!/usr/bin/env bash
# Prints the exact row count of every table of one database of a fly PostgreSQL cluster, and the
# total: e.g. to show what a deployed environment holds without showing any of it (ADR 0024 step 7).
#
#   scripts/fly-db-counts.sh <cluster> <database>     # e.g. tournesol-db critical_prod
#
# Read-only: runs psql on the cluster's machine through `fly ssh` as its operator, in a read-only
# transaction. The cluster's machine must be started (`fly machine start`, docs/deploy.md). Prints table
# names and counts only.
set -euo pipefail

cluster="${1:?usage: fly-db-counts.sh <cluster> <database>}"
db="${2:?usage: fly-db-counts.sh <cluster> <database>}"
[[ "$db" =~ ^[a-z_][a-z0-9_]*$ ]] || { echo "fly-db-counts: bad database name: $db" >&2; exit 2; }
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"

{
    printf '%s\n' "PGPASSWORD=\"\$OPERATOR_PASSWORD\" PGOPTIONS='-c default_transaction_read_only=on' exec psql -w -X -q -A -t -F ' ' -v ON_ERROR_STOP=1 -h localhost -p 5433 -U postgres -d $db <<'CRITICAL_SQL'"
    cat <<'SQL'
SELECT format('SELECT %L, count(*) FROM %I.%I', tablename, schemaname, tablename)
FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename \gexec
SQL
    printf '%s\n' 'CRITICAL_SQL'
} | "$fly" ssh console -a "$cluster" -C "sh -s" 2>&1 | grep -v '^Connecting to ' \
  | awk '{ printf "%-24s %s\n", $1, $2; total += $2 } END { printf "%-24s %s\n", "total", total }'
