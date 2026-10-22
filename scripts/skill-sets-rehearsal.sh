#!/usr/bin/env bash
# The skill sets upgrade rehearsal (ADR 0040 step 3.5, migration 034 and its data step): a database
# written by the release before is upgraded by this checkout's server, on a throwaway LOCAL compose
# PostgreSQL, then checked through the API and SQL.
#
#   scripts/skill-sets-rehearsal.sh [old ref]   # default: the parent of the commit that added 034
#
# 1. The old release (built from <old ref>, which must not hold 034_skill_sets.sql) seeds the bard
#    of the fixture and writes a thief with points, a bard with points and a thief with none; every
#    character's skills are read.
# 2. This checkout's server upgrades it: the points sit in skill_sets under the thief and the bard
#    set, every skill percentage and point is the same as before.
# 3. Two restarts change nothing.
# 4. An old-release-style write (skill_points set, skill_sets '{}') still shows on the sheet, for a
#    thief and a bard; a spend by this release then lands in skill_sets and wins.
# 5. The fixture seed (character.json) loads on a fresh database too.
#
# Nothing touches the development database: its own compose project (criticalsets<pid>), free port,
# own volume, removed at the end whatever the outcome. Fixture seeds and catalogue (api/testdata).
# Exit 0 only when every check passes. Needs docker, psql, jq, curl and go.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
fail() { echo "skill-sets-rehearsal: FAIL: $*" >&2; exit 1; }
ok() { echo "skill-sets-rehearsal: ok: $*"; }
note() { echo "skill-sets-rehearsal: observed: $*"; }

added="$(git -C "$root" log --diff-filter=A --format=%H -n1 -- api/internal/storage/postgres/migrations/034_skill_sets.sql)"
old_ref="${1:-${added:+$added^}}"
[ -n "$old_ref" ] || fail "no ref given and no commit adds the skill sets migration"
git -C "$root" cat-file -e "$old_ref:api/internal/storage/postgres/migrations/034_skill_sets.sql" 2>/dev/null &&
    fail "$old_ref already holds the skill sets migration: pass a ref of before it"

free_port() { python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }
work="$(mktemp -d)"
project="criticalsets$$"
pg_port="$(free_port)"
api_port="$(free_port)"
export COMPOSE_PROJECT_NAME="$project" CRITICAL_PG_PORT="$pg_port"
export PGPASSWORD=critical # the compose database's own throwaway account (docker-compose.yml)
db="host=127.0.0.1 port=$pg_port user=critical dbname=critical_fixture"
api="http://127.0.0.1:$api_port"
jar="$work/cookies"
started="" srv_pid=""

cleanup() {
    status=$?
    set +e
    [ -n "$srv_pid" ] && kill "$srv_pid" 2>/dev/null && wait "$srv_pid" 2>/dev/null
    if [ -n "$started" ]; then
        (cd "$root" && docker compose down --volumes --remove-orphans >/dev/null 2>&1)
        echo "skill-sets-rehearsal: compose project $project removed"
    fi
    [ "$status" -ne 0 ] && [ -f "$work/server.log" ] && tail -n 30 "$work/server.log" >&2
    rm -rf "$work"
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

# --- binaries: the old release and this checkout ---
mkdir -p "$work/old"
git -C "$root" archive "$old_ref" api | tar -x -C "$work/old"
(cd "$work/old/api" && go build -o "$work/old-srv" ./cmd/api) || fail "could not build the old release ($old_ref)"
(cd "$root/api" && go build -o "$work/new-srv" ./cmd/api) || fail "could not build this checkout"

# --- the throwaway database ---
cd "$root"
started=1
docker compose up -d postgres >/dev/null 2>&1 || fail "could not start the scratch PostgreSQL (docker running?)"
for _ in $(seq 1 60); do
    psql "$db" -X -q -c 'SELECT 1' >/dev/null 2>&1 && break
    sleep 1
done
psql "$db" -X -q -c 'SELECT 1' >/dev/null 2>&1 || fail "the scratch database critical_fixture did not accept connections"

sql() { psql "$db" -X -q -t -A -c "$1"; }

start() { # start <binary> <testdata dir>
    CRITICAL_DSN="postgres://critical:critical@127.0.0.1:$pg_port/critical_fixture?sslmode=disable" \
        CRITICAL_SEEDS_DIR="$2/seeds" CRITICAL_CATALOGUE="$2/catalogue.json" CRITICAL_ENV=dev \
        CRITICAL_ADDR="127.0.0.1:$api_port" "$1" >>"$work/server.log" 2>&1 &
    srv_pid=$!
    for _ in $(seq 1 100); do
        curl -fsS "$api/api/health" >/dev/null 2>&1 && break
        kill -0 "$srv_pid" 2>/dev/null || fail "the server ($1) exited at start"
        sleep 0.1
    done
    curl -fsS "$api/api/health" >/dev/null || fail "the server ($1) did not answer"
    curl -fsS -c "$jar" -X POST "$api/api/auth/dev" -H 'Content-Type: application/json' -d '{}' >/dev/null ||
        fail "dev login refused"
}
stop() {
    kill "$srv_pid" && wait "$srv_pid" 2>/dev/null || true
    srv_pid=""
}
# call <method> <path> [body]: the JSON answer; fails on any non-2xx.
call() {
    local out code
    out="$(curl -sS -b "$jar" -X "$1" "$api$2" -H 'Content-Type: application/json' ${3:+-d "$3"} -w '\n%{http_code}')"
    code="${out##*$'\n'}"
    out="${out%$'\n'*}"
    [[ "$code" == 2* ]] || fail "$1 $2 answered $code: $out"
    printf '%s' "$out"
}
expect() { # expect <what> <want> <got>
    [ "$2" = "$3" ] || fail "$1: want '$2', got '$3'"
    ok "$1"
}
undo() { call POST /api/log/undo "{\"expectedTopId\":$(call GET /api/log | jq .topId)}" | jq -r .kind; }
last_kind() { call GET /api/log | jq -r '.topId as $t | (.entries[] | select(.id == $t) | .kind) // empty'; }
undo() { call POST /api/log/undo "{\"expectedTopId\":$(call GET /api/log | jq .topId)}" | jq -r .kind; }
declare -A id
use() { call PUT /api/characters/current "{\"id\":${id[$1]}}" >/dev/null; }
# create <name> <class> <race> <skillPoints JSON>: a level 1 character with points; id[name], current.
create() {
    local out
    out="$(call POST /api/characters "{\"name\":\"$1\",\"class\":\"$2\",\"raceCode\":\"$3\",
        \"details\":{\"alignment\":\"true_neutral\",\"sex\":\"male\"},
        \"str\":10,\"dex\":17,\"con\":12,\"int\":14,\"wis\":9,\"cha\":16,\"hpMax\":6,\"xp\":0,
        \"skillPoints\":$4}")"
    id[$1]="$(jq -r .currentId <<<"$out")"
    [ "${id[$1]}" != null ] || fail "creating $1 set no current character: $out"
}
# The current character's skills as the comparison reads them, one line per skill: old shape, then new.
old_skills() { call GET /api/skills | jq -r '(.thiefSkills + .bardSkills)[] | "\(.id) \(.points) \(.percent)"'; }
new_skills() { call GET /api/skills | jq -r '.skillSets[].skills[] | "\(.id) \(.points) \(.percent)"'; }
stored() { sql "SELECT skill_sets FROM characters WHERE id = ${id[$1]}" | jq -cS .; }
points_of() { call GET /api/skills | jq -r ".skillSets[] | select(.set == \"$1\") | .skills[] | select(.id == \"$2\") | .points"; }

# --- 1. the old release writes the characters ---
start "$work/old-srv" "$work/old/api/testdata"
create robin thief human '{"open_locks":20,"climb_walls":15}'
create lyra bard half_elf '{"pick_pockets":10,"detect_noise":5}'
create blank thief human '{}'
id[seed]="$(sql "SELECT id FROM characters WHERE name = 'Test Bard'")"
[ -n "${id[seed]}" ] || fail "the fixture bard is not stored"
names=(robin lyra blank seed)
for n in "${names[@]}"; do
    use "$n"
    old_skills >"$work/before-$n"
    [ -s "$work/before-$n" ] || fail "$n has no skills on the old release"
done
expect "old release: robin's points are flat" '{"climb_walls":15,"open_locks":20}' "$(sql "SELECT skill_points FROM characters WHERE id = ${id[robin]}" | jq -cS .)"
stop
echo "skill-sets-rehearsal: old release wrote its data ($old_ref)"

new_testdata="$root/api/testdata"

# --- 2. this checkout upgrades it ---
start "$work/new-srv" "$new_testdata"
expect "migration 034 recorded once" 1 "$(sql "SELECT COUNT(*) FROM schema_migrations WHERE name = '034_skill_sets.sql'")"
expect "robin's points sit in the thief set" '{"thief":{"climb_walls":15,"open_locks":20}}' "$(stored robin)"
expect "lyra's points sit in the bard set" '{"bard":{"detect_noise":5,"pick_pockets":10}}' "$(stored lyra)"
expect "the character with no points has no set" '{}' "$(stored blank)"
expect "the fixture bard's points sit in the bard set" '{"bard":{"climb_walls":35,"detect_noise":70,"pick_pockets":25,"read_languages":70}}' "$(stored seed)"
for n in "${names[@]}"; do
    use "$n"
    expect "$n: every skill, point and percentage as before" "$(cat "$work/before-$n")" "$(new_skills)"
done
expect "the legacy column is left as it was" '{"climb_walls":15,"open_locks":20}' "$(sql "SELECT skill_points FROM characters WHERE id = ${id[robin]}" | jq -cS .)"

# --- 3. two restarts: nothing changes ---
state() {
    sql "SELECT id || ' ' || skill_sets || ' ' || skill_points FROM characters ORDER BY id"
    sql "SELECT COUNT(*) FROM schema_migrations"
    sql "SELECT COUNT(*) FROM action_log"
}
before="$(state)"
for n in 1 2; do
    stop
    start "$work/new-srv" "$new_testdata"
    expect "restart $n leaves the characters, migrations and journal as they were" "$before" "$(state)"
done

# --- 4. an old-release-style write after 034 still shows ---
sql "UPDATE characters SET skill_sets = '{}', skill_points = '{\"open_locks\":25}' WHERE id = ${id[robin]}" >/dev/null
sql "UPDATE characters SET skill_sets = '{}', skill_points = '{\"pick_pockets\":12}' WHERE id = ${id[lyra]}" >/dev/null
use robin
expect "robin: the old release's points show on the sheet" 25 "$(points_of thief open_locks)"
use lyra
expect "lyra: the old release's points show on the sheet" 12 "$(points_of bard pick_pockets)"
use robin
call PUT /api/skills/points '{"points":{"open_locks":30}}' >/dev/null
expect "a spend by this release lands in skill_sets" '{"thief":{"open_locks":30}}' "$(stored robin)"
expect "and the sheet reads it" 30 "$(points_of thief open_locks)"
expect "undo gives back the legacy points" skill_points "$(undo)"
expect "the sheet shows them again" 25 "$(points_of thief open_locks)"
stop

# --- 5. the fixture seed on a fresh database ---
sql "CREATE DATABASE critical_fresh" >/dev/null
CRITICAL_DSN="postgres://critical:critical@127.0.0.1:$pg_port/critical_fresh?sslmode=disable" \
    CRITICAL_SEEDS_DIR="$new_testdata/seeds" CRITICAL_CATALOGUE="$new_testdata/catalogue.json" CRITICAL_ENV=dev \
    CRITICAL_ADDR="127.0.0.1:$api_port" "$work/new-srv" >>"$work/server.log" 2>&1 &
srv_pid=$!
for _ in $(seq 1 100); do
    curl -fsS "$api/api/health" >/dev/null 2>&1 && break
    kill -0 "$srv_pid" 2>/dev/null || fail "the server exited at start on the fresh database"
    sleep 0.1
done
curl -fsS "$api/api/health" >/dev/null || fail "the server did not answer on the fresh database"
db="host=127.0.0.1 port=$pg_port user=critical dbname=critical_fresh"
expect "fresh database: the fixture bard's points sit in the bard set" '{"bard":{"climb_walls":35,"detect_noise":70,"pick_pockets":25,"read_languages":70}}' \
    "$(sql "SELECT skill_sets FROM characters WHERE name = 'Test Bard'" | jq -cS .)"
stop

echo "skill-sets-rehearsal: PASS"
