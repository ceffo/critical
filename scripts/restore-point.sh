#!/bin/sh
# Takes the pre-promotion restore point of the production database (ADR 0024 steps 4.2 and 9.2): a pg_dump
# of the database CRITICAL_DSN names, with the row count of every table, sent back on stdout so the
# promotion (scripts/promote.sh) keeps it, encrypted, as an artifact of its run. Nothing is written on the
# cluster: no copy database (a pg_restore into one ran the shared cluster's small VM out of memory). The
# promotion runs it inside an ephemeral `fly console` machine of the production app started from the
# release image (which ships the PostgreSQL 17 client), so it has the app's CRITICAL_DSN and the cluster's
# private network, and the running app is not touched.
#
#   sh restore-point.sh <base>_pre_v<major>_<minor>_<patch>_<UTC yyyymmddhhmmss> <migration>...
#   e.g. critical_prod_pre_v1_2_3_20261017153000 001_baseline.sql 002_spells.sql
#
# The <migration>s are the migration files the release image embeds (its commit's
# api/internal/storage/postgres/migrations/*.sql, listed by scripts/promote.sh). When the database records
# one that is not among them, the release is older than the schema (a rollback across a migration): its
# server would refuse to start (ADR 0024 step 6.2) and the rolling deploy would leave production on one
# machine, so that is refused, pointing at the runbook (docs/deploy.md, "Rollback", "Across a migration").
#
# Refused before the dump: a malformed name or migration, a name that is not a restore point of the
# database CRITICAL_DSN connects to, a database newer than the release's migrations (above), and the server
# running in this machine (its boot would migrate the database before the dump). The dump is pg_dump's
# custom format (--no-owner --no-acl: it restores under any user), read back with pg_restore --list (table
# data, and the recorded migrations among it). counts.txt holds "<table> <rows>" per table, taken right after
# the dump, as `just db-backup` writes it (the drill compares them).
#
# Output, stdout: progress lines, then the restore point, a tar of <base>.dump and counts.txt, in base64
# between the lines `restore-point: BEGIN <name>` and `restore-point: END <name>`, then the last line
# `restore-point: OK <name> <sha256 of the tar> <bytes>`. The promotion keeps the block out of its log
# (the dump holds catalogue rows and player data: ADR 0024 step 7) and checks the checksum and the size.
# Nothing else prints data or the DSN. A lost connection to the cluster (its proxy blips while the VM is
# short of memory) is retried a few times (RESTORE_POINT_CONNECT_TRIES, default 5), and so is pg_dump; a
# failed statement is not.
#
# POSIX sh (the image's shell is busybox). Also run by scripts/image-smoke.sh against a throwaway database.
set -eu

usage="usage: restore-point.sh <base>_pre_v<major>_<minor>_<patch>_<yyyymmddhhmmss> <migration>..."
fail() { echo "restore-point: FAIL: $*" >&2; exit 1; }
name="${1:?$usage}"
shift
[ "$#" -gt 0 ] || fail "no embedded migration given; $usage"
connect_tries="${RESTORE_POINT_CONNECT_TRIES:-5}" # 2+4+6+8 = 20 s of waiting by default
connect_wait="${RESTORE_POINT_CONNECT_WAIT:-2}"
dsn="${CRITICAL_DSN:?restore-point: CRITICAL_DSN is not set}"
for m in "$@"; do
    echo "$m" | grep -Eqx '[0-9a-z_]+\.sql' || fail "malformed migration name '$m'"
done

case "$connect_tries" in '' | *[!0-9]* | 0) fail "RESTORE_POINT_CONNECT_TRIES must be a positive number" ;; esac
case "$connect_wait" in '' | *[!0-9]*) fail "RESTORE_POINT_CONNECT_WAIT must be a number" ;; esac
echo "$name" | grep -Eqx '[a-z][a-z0-9_]*_pre_v[0-9]+_[0-9]+_[0-9]+_[0-9]{14}' || fail "malformed restore point name '$name'"
dsn_path="${dsn%%\?*}"
base="${dsn_path##*/}"
case "$name" in "${base}_pre_"*) ;; *) fail "'$name' is not a restore point of the database '$base'" ;; esac
for tool in psql pg_dump pg_restore tar base64 sha256sum; do
    command -v "$tool" >/dev/null || fail "$tool is missing (the image ships postgresql17-client and busybox)"
done
# fly console starts its machine without the image's entrypoint; if the server ran here, its boot would
# already have migrated the database this dump must hold as it was.
! pidof critical-srv >/dev/null 2>&1 || fail "the server runs in this machine: the dump would hold a migrated database"

# q <sql>: one read-only psql run, run again (a few times, waiting longer each time) when the connection
# itself fails (psql exits 2): the cluster's proxy marks the database down for some seconds when its small VM
# is short of memory, and a promotion must not fail on that. A statement that fails (exit 1 or 3) is not.
q() {
    _qn=1
    while :; do
        _qs=0
        _qo="$(PGOPTIONS="-c default_transaction_read_only=on" psql -X -w -q -A -t -v ON_ERROR_STOP=1 -d "$dsn" -c "$1")" || _qs=$?
        if [ "$_qs" != 2 ] || [ "$_qn" -ge "$connect_tries" ]; then
            [ -z "$_qo" ] || printf '%s\n' "$_qo"
            return "$_qs"
        fi
        echo "restore-point: the connection to the database failed (try $_qn of $connect_tries); trying again in $((_qn * connect_wait))s" >&2
        sleep $((_qn * connect_wait))
        _qn=$((_qn + 1))
    done
}

[ "$(q 'SELECT current_database()')" = "$base" ] || fail "CRITICAL_DSN does not connect to '$base'"
# The names are checked above ([0-9a-z_]+.sql), so they are safe inside the SQL literal.
embedded="$(printf "'%s'," "$@")"
unknown="$(q "SELECT coalesce(string_agg(name, ' ' ORDER BY name), '') FROM schema_migrations
              WHERE name NOT IN (${embedded%,})")"
[ -z "$unknown" ] || fail "$base records migrations this release does not embed ($unknown): it is older than \
the schema and would not start on it. Roll back across a migration as docs/deploy.md says (\"Rollback\", \"Across a migration\")"

umask 077
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir "$work/point"

echo "restore-point: dumping $base"
# pg_dump only reads (one snapshot, light on the server): a dump cut by a lost connection is taken again,
# twice at most.
dumped=0
for try in 1 2 3; do
    if pg_dump -w -Fc --no-owner --no-acl -f "$work/point/$base.dump" -d "$dsn"; then dumped=1; break; fi
    [ "$try" = 3 ] || { echo "restore-point: pg_dump of $base failed (try $try of 3); trying again in $((try * 5))s" >&2; sleep $((try * 5)); }
done
[ "$dumped" = 1 ] || fail "pg_dump of $base failed"
[ -s "$work/point/$base.dump" ] || fail "the dump of $base is empty"
pg_restore --list "$work/point/$base.dump" >"$work/toc" || fail "the dump of $base is not readable by pg_restore --list"
grep -Eq ' TABLE DATA public schema_migrations( |$)' "$work/toc" || fail "the dump of $base holds no recorded migrations"
entries="$(grep -c ' TABLE DATA ' "$work/toc" || true)"

# The row count of every table, "<table> <rows>" in byte order (scripts/db-table-counts.sh's format).
q "SELECT tablename || ' ' || (xpath('/row/c/text()', query_to_xml(format('SELECT count(*) AS c FROM public.%I', tablename), false, true, '')))[1]::text
   FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename COLLATE \"C\"" >"$work/point/counts.txt" \
    || fail "could not count the tables of $base"
tables="$(wc -l <"$work/point/counts.txt" | tr -d ' ')"
[ "$tables" -gt 0 ] || fail "$base has no table"
rows="$(awk '{ n += $2 } END { print n + 0 }' "$work/point/counts.txt")"

tar -C "$work/point" -cf "$work/point.tar" "$base.dump" counts.txt
sum="$(sha256sum "$work/point.tar" | cut -d' ' -f1)"
bytes="$(wc -c <"$work/point.tar" | tr -d ' ')"
echo "restore-point: $name taken ($tables tables, $entries with data, $rows rows, $bytes bytes)"
echo "restore-point: BEGIN $name"
base64 "$work/point.tar"
echo "restore-point: END $name"
echo "restore-point: OK $name $sum $bytes"
