#!/usr/bin/env bash
# The AC override migration rehearsal (ADR 0033 step 13, migration 017_ac_override.sql) on a COPY of a
# real database: a read-only pg_dump of the source is restored into a throwaway LOCAL compose
# PostgreSQL, this checkout's server upgrades it (every pending migration, the equipment conversion
# included when the source is older), then the data is checked through SQL and the API.
#
#   scripts/ac-override-rehearsal.sh [source container] [source database]
#       default: critical-postgres-1 critical (the main checkout's `just dev` database)
#
# 1. The copy keeps its characters (same ids) and every one reads no override (NULL, empty label).
# 2. 017 is recorded once; a restart applies nothing again.
# 3. The dev login's current character: GET /api/ac answers a derived AC; PUT /api/ac/override
#    stores it and logs one ac_override entry; undo clears it, redo sets it again; DELETE clears it.
# 4. A refused override (11) is a 400 on "value" and logs nothing.
#
# The source is only read (pg_dump). Nothing else touches it: the copy lives in its own compose
# project (criticalac<pid>) on a free port with its own volume, removed at the end whatever the
# outcome. Real seeds (seeds/) and catalogue (.catalogue/catalogue.json, `just catalogue`). Exit 0 only
# when every check passes. Needs docker, psql, pg_restore, jq, curl and go.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
fail() { echo "ac-rehearsal: FAIL: $*" >&2; exit 1; }
ok() { echo "ac-rehearsal: ok: $*"; }

source_container="${1:-critical-postgres-1}"
source_db="${2:-critical}"
catalogue="$root/.catalogue/catalogue.json"
[ -f "$catalogue" ] || fail "no $catalogue: run just catalogue"

free_port() { python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }
work="$(mktemp -d)"
project="criticalac$$"
pg_port="$(free_port)"
api_port="$(free_port)"
export COMPOSE_PROJECT_NAME="$project" CRITICAL_PG_PORT="$pg_port"
export PGPASSWORD=critical # the compose database's own throwaway account (docker-compose.yml)
db="host=127.0.0.1 port=$pg_port user=critical dbname=rehearsal"
api="http://127.0.0.1:$api_port"
jar="$work/cookies"
started="" srv_pid=""

cleanup() {
    status=$?
    set +e
    [ -n "$srv_pid" ] && kill "$srv_pid" 2>/dev/null && wait "$srv_pid" 2>/dev/null
    if [ -n "$started" ]; then
        (cd "$root" && docker compose down --volumes --remove-orphans >/dev/null 2>&1)
        echo "ac-rehearsal: compose project $project removed"
    fi
    [ "$status" -ne 0 ] && [ -f "$work/server.log" ] && tail -n 30 "$work/server.log" >&2
    rm -rf "$work"
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

# --- the copy ---
docker exec "$source_container" pg_dump -U critical -Fc "$source_db" >"$work/source.dump" ||
    fail "could not dump $source_db from $source_container"
(cd "$root/api" && go build -o "$work/srv" ./cmd/api) || fail "could not build this checkout"
cd "$root"
started=1
docker compose up -d postgres >/dev/null 2>&1 || fail "could not start the scratch PostgreSQL (docker running?)"
for _ in $(seq 1 60); do
    psql "host=127.0.0.1 port=$pg_port user=critical dbname=critical" -X -q -c 'SELECT 1' >/dev/null 2>&1 && break
    sleep 1
done
psql "host=127.0.0.1 port=$pg_port user=critical dbname=critical" -X -q -c 'CREATE DATABASE rehearsal' >/dev/null ||
    fail "the scratch PostgreSQL did not accept connections"
pg_restore --no-owner -d "postgresql://critical:critical@127.0.0.1:$pg_port/rehearsal" "$work/source.dump" ||
    fail "could not restore the copy"

sql() { psql "$db" -X -q -t -A -c "$1"; }
chars_before="$(sql "SELECT string_agg(id::text, ',' ORDER BY id) FROM characters")"
[ -n "$chars_before" ] || fail "the copy holds no character"
ok "copy restored: characters $chars_before, last migration $(sql 'SELECT max(name) FROM schema_migrations' 2>/dev/null || echo '?')"

start() {
    CRITICAL_DSN="postgres://critical:critical@127.0.0.1:$pg_port/rehearsal?sslmode=disable" \
        CRITICAL_SEEDS_DIR="$root/seeds" CRITICAL_CATALOGUE="$catalogue" CRITICAL_ENV=dev \
        CRITICAL_ADDR="127.0.0.1:$api_port" "$work/srv" >>"$work/server.log" 2>&1 &
    srv_pid=$!
    for _ in $(seq 1 300); do
        curl -fsS "$api/api/health" >/dev/null 2>&1 && break
        kill -0 "$srv_pid" 2>/dev/null || fail "the server exited at start"
        sleep 0.1
    done
    curl -fsS "$api/api/health" >/dev/null || fail "the server did not answer"
    curl -fsS -c "$jar" -X POST "$api/api/auth/dev" -H 'Content-Type: application/json' -d '{}' >/dev/null ||
        fail "dev login refused"
}
stop() {
    kill "$srv_pid" && wait "$srv_pid" 2>/dev/null || true
    srv_pid=""
}
# call <method> <path> [body]: the answer and its status on the last line.
call() { curl -sS -b "$jar" -X "$1" "$api$2" -H 'Content-Type: application/json' ${3:+-d "$3"} -w '\n%{http_code}'; }
body() { local out; out="$(call "$@")"; [[ "${out##*$'\n'}" == 2* ]] || fail "$1 $2 answered: $out"; printf '%s' "${out%$'\n'*}"; }
expect() { [ "$2" = "$3" ] || fail "$1: want '$2', got '$3'"; ok "$1"; }
entries() { body GET /api/log | jq '.entries | length'; }
top_kind() { body GET /api/log | jq -r '.topId as $t | (.entries[] | select(.id == $t) | .kind) // empty'; }

# --- 1 and 2: the upgrade ---
start
expect "017 recorded once" 1 "$(sql "SELECT count(*) FROM schema_migrations WHERE name = '017_ac_override.sql'")"
expect "the characters are kept" "$chars_before" "$(sql "SELECT string_agg(id::text, ',' ORDER BY id) FROM characters")"
expect "no character has an override" 0 "$(sql "SELECT count(*) FROM characters WHERE ac_override IS NOT NULL OR ac_override_label <> ''")"

# --- 3: the AC and its override on the dev login's current character ---
ac="$(body GET /api/ac)"
jq -e '(.ac | type) == "number" and (.rows | length) >= 4 and .override == null' <<<"$ac" >/dev/null ||
    fail "GET /api/ac: $ac"
ok "GET /api/ac: AC $(jq .ac <<<"$ac"), $(jq '.rows | length' <<<"$ac") rows"
n="$(entries)"
body PUT /api/ac/override '{"value":4,"label":"Sort Bouclier"}' >/dev/null
expect "the override is stored" "4|Sort Bouclier" "$(sql "SELECT ac_override || '|' || ac_override_label FROM characters WHERE ac_override IS NOT NULL")"
expect "one ac_override entry" "$((n + 1)) ac_override" "$(entries) $(top_kind)"
body POST /api/log/undo "{\"expectedTopId\":$(body GET /api/log | jq .topId)}" >/dev/null
expect "undo clears it" 0 "$(sql 'SELECT count(*) FROM characters WHERE ac_override IS NOT NULL')"
body POST /api/log/redo "{\"expectedRedoId\":$(body GET /api/log | jq .redo.id)}" >/dev/null
expect "redo sets it again" 4 "$(body GET /api/ac | jq .override.value)"

# --- 4: a refused override ---
n="$(entries)"
out="$(call PUT /api/ac/override '{"value":11}')"
expect "11 is a 400 on value" "400 value" "${out##*$'\n'} $(jq -r .field <<<"${out%$'\n'*}")"
expect "nothing logged" "$n" "$(entries)"
body DELETE /api/ac/override >/dev/null
expect "DELETE clears it" null "$(body GET /api/ac | jq .override)"

# --- 2: a restart applies nothing again ---
stop
start
expect "017 still recorded once after a restart" 1 "$(sql "SELECT count(*) FROM schema_migrations WHERE name = '017_ac_override.sql'")"
expect "the characters are still kept" "$chars_before" "$(sql "SELECT string_agg(id::text, ',' ORDER BY id) FROM characters")"
stop
echo "ac-rehearsal: every check passed"
