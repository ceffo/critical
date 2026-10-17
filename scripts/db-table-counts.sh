#!/usr/bin/env bash
# Prints the exact row count of every table of the public schema of one PostgreSQL database, one
# "<table> <count>" line per table in byte order, nothing else: no row, no password. The counts a backup
# records at dump time and the drill compares after the restore (docs/deploy.md, "Backups").
#
#   scripts/db-table-counts.sh "<libpq connection string or URI>"    # the password, if any, in PGPASSWORD
#
# Read-only (the session is).
set -euo pipefail

conn="${1:?usage: db-table-counts.sh <connection string>}"
export PGOPTIONS="-c default_transaction_read_only=on"
psql "$conn" -X -q -A -t -F ' ' -v ON_ERROR_STOP=1 <<'SQL' | LC_ALL=C sort
SELECT format('SELECT %L, count(*) FROM %I.%I', tablename, schemaname, tablename)
FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename \gexec
SQL
