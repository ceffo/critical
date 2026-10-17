#!/usr/bin/env bash
# The restore drill (ADR 0024 step 9; docs/deploy.md, "Backups"): restores a dump taken by
# scripts/db-backup.sh into a scratch database of a throwaway LOCAL compose PostgreSQL, compares the row
# count of every table with the counts recorded at dump time (counts.txt, next to the dump), then drops the
# scratch database and removes the compose project and its volume, whatever the outcome.
#
#   scripts/db-restore-drill.sh [dump file]     # default: the newest .backups/prod-*/critical_prod.dump
#
# Nothing touches fly, the cluster, or the development database: the server is the `postgres` service of
# docker-compose.yml under its own compose project (criticaldrill<pid>) on a free port, with its own
# volume. Prints table names and counts only. Exit 0 only when every table matches. Needs docker, psql and
# pg_restore.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
fail() { echo "db-restore-drill: FAIL: $*" >&2; exit 1; }

dump="${1:-}"
if [ -z "$dump" ]; then
    # shellcheck disable=SC2012 # names are ours: <env>-<timestamp>, sortable
    dump="$(ls -1d "${BACKUP_DIR:-$root/.backups}"/prod-*/critical_prod.dump 2>/dev/null | sort | tail -n1 || true)"
    [ -n "$dump" ] || fail "no prod dump in ${BACKUP_DIR:-$root/.backups}: run \`just db-backup prod\`, or pass a dump file"
fi
[ -f "$dump" ] || fail "no such dump: $dump"
expected="$(dirname "$dump")/counts.txt"
[ -s "$expected" ] || fail "no counts.txt next to the dump ($expected): the counts recorded at dump time are what the restore is compared with"
pg_restore --list "$dump" >/dev/null || fail "the dump is not readable by pg_restore --list"

work="$(mktemp -d)"
project="criticaldrill$$"
scratch="critical_drill_$(date -u +%Y%m%d%H%M%S)"
port="$(python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
export COMPOSE_PROJECT_NAME="$project" CRITICAL_PG_PORT="$port"
export PGPASSWORD=critical # the compose database's own throwaway account (docker-compose.yml)
admin="host=127.0.0.1 port=$port user=critical dbname=postgres"
started=""

cleanup() {
    status=$?
    set +e
    if [ -n "$started" ]; then
        psql "$admin" -X -q -c "DROP DATABASE IF EXISTS $scratch WITH (FORCE)" >/dev/null 2>&1
        (cd "$root" && docker compose down --volumes --remove-orphans >/dev/null 2>&1)
        echo "db-restore-drill: scratch database $scratch dropped, compose project $project removed"
    fi
    rm -rf "$work"
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

cd "$root"
started=1 # from here a failure must clean up whatever `up` managed to create
docker compose up -d postgres >/dev/null 2>&1 || fail "could not start the scratch PostgreSQL (docker running?)"
for _ in $(seq 1 60); do
    docker compose exec -T postgres pg_isready -U critical -d critical >/dev/null 2>&1 && break
    sleep 1
done
docker compose exec -T postgres pg_isready -U critical -d critical >/dev/null 2>&1 || fail "the scratch PostgreSQL did not accept connections"

psql "$admin" -X -q -v ON_ERROR_STOP=1 -c "CREATE DATABASE $scratch" || fail "could not create $scratch"
conn="host=127.0.0.1 port=$port user=critical dbname=$scratch"
pg_restore --no-owner --no-acl --exit-on-error --dbname="$conn" "$dump" || fail "pg_restore failed"
"$here/db-table-counts.sh" "$conn" >"$work/restored.txt" || fail "could not count the restored tables"
"$here/db-counts-compare.sh" "$expected" "$work/restored.txt"
