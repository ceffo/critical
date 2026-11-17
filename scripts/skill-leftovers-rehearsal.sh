#!/usr/bin/env bash
# The skill points drop rehearsal (ADR 0040 step 3.5 amended, migration 048): a database written by
# the release before is upgraded by this checkout's server, on a throwaway LOCAL compose
# PostgreSQL, then checked through the API and SQL.
#
#   scripts/skill-leftovers-rehearsal.sh [old ref]   # default: the parent of the commit that added 048
#
# 1. The old release (built from <old ref>, which must not hold 048_drop_skill_points.sql) seeds the
#    bard of the fixture and writes a thief with points, a bard with points and a thief with none;
#    every character's skills are read.
# 2. A character left with flat skill_points and no skill sets (what a release from before 034 wrote)
#    is planted: this checkout's server refuses to start, applies nothing (048 not recorded, the
#    column still there) and its log names the guard.
# 3. The points are moved into skill_sets by hand (what 034's copy did): the server upgrades, the
#    column is gone, and every skill point and percentage is the same as before.
# 4. Two restarts change nothing; a spend and its undo still work.
# 5. The fixture seed (character.json) loads on a fresh database too, without the column.
#
# Nothing touches the development database: its own compose project (criticalleft<pid>), free port,
# own volume, removed at the end whatever the outcome. Fixture seeds and catalogue (api/testdata).
# Exit 0 only when every check passes. Needs docker, psql, jq, curl and go.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
fail() { echo "skill-leftovers-rehearsal: FAIL: $*" >&2; exit 1; }
ok() { echo "skill-leftovers-rehearsal: ok: $*"; }

migration=api/internal/storage/postgres/migrations/048_drop_skill_points.sql
added="$(git -C "$root" log --diff-filter=A --format=%H -n1 -- "$migration")"
old_ref="${1:-${added:+$added^}}"
[ -n "$old_ref" ] || fail "no ref given and no commit adds the skill points drop migration"
git -C "$root" cat-file -e "$old_ref:$migration" 2>/dev/null &&
    fail "$old_ref already holds the skill points drop migration: pass a ref of before it"

free_port() { python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }
work="$(mktemp -d)"
project="criticalleft$$"
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
        echo "skill-leftovers-rehearsal: compose project $project removed"
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
# The current character's skills as the comparison reads them, one line per skill.
new_skills() { call GET /api/skills | jq -r '.skillSets[].skills[] | "\(.id) \(.points) \(.percent)"'; }
stored() { sql "SELECT skill_sets FROM characters WHERE id = ${id[$1]}" | jq -cS .; }
points_of() { call GET /api/skills | jq -r ".skillSets[] | select(.set == \"$1\") | .skills[] | select(.id == \"$2\") | .points"; }

# --- 1. the old release writes the characters ---
start "$work/old-srv" "$work/old/api/testdata"
create robin thief human '{"thief":{"open_locks":20,"climb_walls":15}}'
create lyra bard half_elf '{"bard":{"pick_pockets":10,"detect_noise":5}}'
create blank thief human '{}'
id[seed]="$(sql "SELECT id FROM characters WHERE name = 'Test Bard'")"
[ -n "${id[seed]}" ] || fail "the fixture bard is not stored"
names=(robin lyra blank seed)
for n in "${names[@]}"; do
    use "$n"
    new_skills >"$work/before-$n"
    [ -s "$work/before-$n" ] || fail "$n has no skills on the old release"
done
expect "old release: robin's points are per skill set" '{"thief":{"climb_walls":15,"open_locks":20}}' "$(stored robin)"
expect "old release: the column still exists" 1 "$(sql "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'characters' AND column_name = 'skill_points'")"
stop
echo "skill-leftovers-rehearsal: old release wrote its data ($old_ref)"

new_testdata="$root/api/testdata"

# --- 2. flat points no skill set holds: the guard refuses, nothing is applied ---
sql "UPDATE characters SET skill_sets = '{}', skill_points = '{\"open_locks\":20,\"climb_walls\":15}' WHERE id = ${id[robin]}" >/dev/null
CRITICAL_DSN="postgres://critical:critical@127.0.0.1:$pg_port/critical_fixture?sslmode=disable" \
    CRITICAL_SEEDS_DIR="$new_testdata/seeds" CRITICAL_CATALOGUE="$new_testdata/catalogue.json" CRITICAL_ENV=dev \
    CRITICAL_ADDR="127.0.0.1:$api_port" "$work/new-srv" >"$work/refused.log" 2>&1 &
srv_pid=$!
refused=""
for _ in $(seq 1 100); do
    kill -0 "$srv_pid" 2>/dev/null || { refused=1; break; }
    sleep 0.1
done
[ -n "$refused" ] || fail "the server started over flat points no skill set holds"
wait "$srv_pid" 2>/dev/null && fail "the server exited 0 on a refused migration"
srv_pid=""
grep -q "skill_points still holds points" "$work/refused.log" || fail "the refusal does not name the guard: $(tail -n 3 "$work/refused.log")"
ok "the guard refused the run"
expect "048 is not recorded" 0 "$(sql "SELECT COUNT(*) FROM schema_migrations WHERE name = '048_drop_skill_points.sql'")"
expect "the column is still there (the run rolled back)" 1 "$(sql "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'characters' AND column_name = 'skill_points'")"
expect "robin's flat points are untouched" '{"climb_walls":15,"open_locks":20}' "$(sql "SELECT skill_points FROM characters WHERE id = ${id[robin]}" | jq -cS .)"

# --- 3. the points moved into skill_sets (what 034's copy did): the upgrade drops the column ---
sql "UPDATE characters SET skill_sets = '{\"thief\":{\"climb_walls\":15,\"open_locks\":20}}' WHERE id = ${id[robin]}" >/dev/null
start "$work/new-srv" "$new_testdata"
expect "048 recorded once" 1 "$(sql "SELECT COUNT(*) FROM schema_migrations WHERE name = '048_drop_skill_points.sql'")"
expect "characters.skill_points is gone" 0 "$(sql "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'characters' AND column_name = 'skill_points'")"
for n in "${names[@]}"; do
    use "$n"
    expect "$n: every skill, point and percentage as before" "$(cat "$work/before-$n")" "$(new_skills)"
done

# --- 4. two restarts change nothing; a spend and its undo work ---
state() {
    sql "SELECT id || ' ' || skill_sets FROM characters ORDER BY id"
    sql "SELECT COUNT(*) FROM schema_migrations"
    sql "SELECT COUNT(*) FROM action_log"
}
before="$(state)"
for n in 1 2; do
    stop
    start "$work/new-srv" "$new_testdata"
    expect "restart $n leaves the characters, migrations and journal as they were" "$before" "$(state)"
done
use robin
call PUT /api/skills/points '{"points":{"open_locks":30}}' >/dev/null
expect "a spend lands in skill_sets" '{"thief":{"open_locks":30}}' "$(stored robin)"
expect "undo gives the points back" skill_points "$(undo)"
expect "the sheet shows them again" 20 "$(points_of thief open_locks)"
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
expect "fresh database: no characters.skill_points" 0 "$(sql "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'characters' AND column_name = 'skill_points'")"
stop

echo "skill-leftovers-rehearsal: PASS"
