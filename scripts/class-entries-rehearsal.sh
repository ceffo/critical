#!/usr/bin/env bash
# The class entries upgrade rehearsal (ADR 0042 step 1, migration 035 and its data step): a database
# written by the release before is upgraded by this checkout's server, on a throwaway LOCAL compose
# PostgreSQL, then checked through the API and SQL.
#
#   scripts/class-entries-rehearsal.sh [old ref]   # default: origin/main while it lacks the migration, else the commit before the one that added it
#
# 1. The old release (built from <old ref>, which must not hold 035_character_classes.sql) writes
#    characters of several classes with a starting XP, an XP history (bonus and plain entries) and,
#    for some, a level-up (a realized level); one more has no level-up, so its level is its sheet's.
#    Each character's sheet is read.
# 2. This checkout's server upgrades it: every character has exactly one active entry with its class,
#    its realized level and its starting XP; the sheet's derived block (XP, THAC0, saves, hit points,
#    level-up, slots) is the same as before; the XP ledger is intact.
# 3. Two restarts: no second entry, nothing else changes.
#    A character with a negative XP and level 0 in its columns (the app never writes one, the columns
#    have no CHECK) must not abort the boot: its entry is clamped to 0.
#    On a migrated character a level-up then its undo move the column and the entry together.
# 4. The rolling deploy: a character the old release writes beside the migrated database (columns
#    only, no entry) is read by this checkout with the one entry its columns state; a level-up of it
#    works and invents no entry.
#
# Nothing touches the development database: the server is the `postgres` service of docker-compose.yml
# under its own compose project (criticalclasses<pid>) on a free port with its own volume, removed at
# the end whatever the outcome. Fixture seeds and catalogue (api/testdata). Exit 0 only when every
# check passes. Needs docker, psql, jq, curl and go.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
fail() { echo "class-entries-rehearsal: FAIL: $*" >&2; exit 1; }
ok() { echo "class-entries-rehearsal: ok: $*"; }

mig=api/internal/storage/postgres/migrations/035_character_classes.sql
# default: the commit before the one that added the migration (the release before, on this branch's base)
# (the migration was first numbered 034, before CRI-166's skill sets took that number: both names count)
mig034="${mig/035_/034_}"
# The release before is origin/main while it lacks the migration (it carries whatever else changed
# the wire meanwhile); once it holds it, the commit before the one that added it.
if git -C "$root" cat-file -e "origin/main:$mig" 2>/dev/null; then
    default_ref="$(git -C "$root" log --diff-filter=A --format=%H -- "$mig" "$mig034" | tail -n 1)^"
else
    default_ref="origin/main"
fi
old_ref="${1:-$default_ref}"
git -C "$root" cat-file -e "$old_ref:$mig" 2>/dev/null &&
    fail "$old_ref already holds the class entries migration: pass a ref of before it"

free_port() { python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }
work="$(mktemp -d)"
project="criticalclasses$$"
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
        echo "class-entries-rehearsal: compose project $project removed"
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

declare -A id
use() { call PUT /api/characters/current "{\"id\":${id[$1]}}" >/dev/null; }
# create <name> <class> <race> <xp> <hp>: its id in id[name], current.
create() {
    local out
    out="$(call POST /api/characters "{\"name\":\"$1\",\"class\":\"$2\",\"raceCode\":\"$3\",
        \"details\":{\"alignment\":\"true_neutral\",\"sex\":\"male\"},
        \"str\":12,\"dex\":14,\"con\":15,\"int\":13,\"wis\":11,\"cha\":16,\"hpMax\":$5,\"xp\":$4}")"
    id[$1]="$(jq -r .currentId <<<"$out")"
    [ "${id[$1]}" != null ] || fail "creating $1 set no current character: $out"
}
award() { call POST /api/xp "{\"base\":$1,\"bonus\":$2,\"reason\":\"rehearsal\"}" >/dev/null; }
# The sheet as both releases serve it, reduced to what must not move: the old shape's class and level
# against the new shape's first entry are compared on their own.
sheet() { call GET /api/character | jq -cS 'del(.id, .class, .level, .classes)'; }
first_class() { call GET /api/character | jq -r '(.classes[0].class // .class) + " " + (.derived.realizedLevel|tostring)'; }
xp_ledger() { call GET /api/xp | jq -cS .; }

# --- 1. the old release writes the characters ---
start "$work/old-srv" "$work/old/api/testdata"
create bard bard half_elf 40000 52
award 2500 true
award 900 false
create thief thief elf 2600 6
award 4000 true
create fighter fighter human 0 10
create mage mage human 2500 4
award 3000 false
create plain thief human 0 5
names=(bard thief fighter mage plain)
declare -A cls
for n in "${names[@]}"; do
    use "$n"
    sheet >"$work/sheet-$n"
    xp_ledger >"$work/xp-$n"
    cls[$n]="$(first_class)"
    [ -s "$work/sheet-$n" ] || fail "$n has no sheet on the old release"
done
roll() { # roll the lowest pending level of the current character; there must be one
    local lvl
    lvl="$(call GET /api/character | jq -r '.derived.levelUp.pending[0].level // empty')"
    [ -n "$lvl" ] || fail "no level pending for the current character"
    call POST /api/character/levelup/roll "{\"level\":$lvl}" >/dev/null
}
use thief
roll
use mage
roll
for n in thief mage; do # the level-ups changed the realized level and hit points
    use "$n"
    sheet >"$work/sheet-$n"
    xp_ledger >"$work/xp-$n"
    cls[$n]="$(first_class)"
done
declare -A cols
for n in "${names[@]}"; do
    cols[$n]="$(sql "SELECT class || ' ' || COALESCE(realized_level, level) || ' ' || xp_total FROM characters WHERE id = ${id[$n]}")"
done
sql "INSERT INTO characters (name, class, level, xp_total, str, dex, con, int, wis, cha, hp_max, thac0_base, owner_user_id)
     SELECT 'odd', 'thief', 0, -5, 12, 14, 15, 13, 11, 16, 6, 20, owner_user_id FROM characters WHERE id = ${id[plain]}" >/dev/null
id[odd]="$(sql "SELECT id FROM characters WHERE name = 'odd'")"
sql "SELECT to_regclass('character_classes')" | grep -qx '' || fail "the old release already has the entries table"
stop
echo "class-entries-rehearsal: old release wrote its data ($old_ref)"

new_testdata="$root/api/testdata"
entries() { sql "SELECT character_id, position, class, realized_level, starting_xp, state, left_at_level FROM character_classes ORDER BY 1, 2" | paste -sd' '; }

# --- 2. this checkout upgrades it ---
start "$work/new-srv" "$new_testdata"
expect "migration 035 recorded once" 1 "$(sql "SELECT COUNT(*) FROM schema_migrations WHERE name = '035_character_classes.sql'")"
for n in "${names[@]}"; do
    expect "$n: one entry" 1 "$(sql "SELECT COUNT(*) FROM character_classes WHERE character_id = ${id[$n]}")"
    expect "$n: the entry states the class, realized level and starting XP of its columns" \
        "${cols[$n]}" "$(sql "SELECT class || ' ' || realized_level || ' ' || starting_xp FROM character_classes WHERE character_id = ${id[$n]} AND position = 0 AND state = 'active' AND left_at_level = 0")"
    use "$n"
    expect "$n: class and level served as before" "${cls[$n]}" "$(first_class)"
    expect "$n: the sheet is the same" "$(cat "$work/sheet-$n")" "$(sheet)"
    expect "$n: the XP ledger is intact" "$(cat "$work/xp-$n")" "$(xp_ledger)"
done
expect "a negative XP and a level 0 booted, clamped to 0" "thief 0 0" \
    "$(sql "SELECT class || ' ' || realized_level || ' ' || starting_xp FROM character_classes WHERE character_id = ${id[odd]}")"
expect "no character without an entry" 0 "$(sql "SELECT COUNT(*) FROM characters c WHERE NOT EXISTS (SELECT 1 FROM character_classes e WHERE e.character_id = c.id)")"
expect "the roster serves the classes" "bard" "$(call GET /api/characters | jq -r --argjson i "${id[bard]}" '[.characters[]? // .[]? | select(.id == $i) | (.classes[0].class // .class)][0]')"
after="$(entries)"

# a level-up then its undo on a migrated character: the column and the entry move together
col_level() { sql "SELECT COALESCE(realized_level, level) FROM characters WHERE id = $1"; }
ent_level() { sql "SELECT realized_level FROM character_classes WHERE character_id = $1 AND position = 0"; }
wire_level() { call GET /api/character | jq -r '.classes[0].level'; }
same_level() { # same_level <what> <id>: column, entry and the wire agree; prints the level
    local c e w
    c="$(col_level "$2")" e="$(ent_level "$2")" w="$(wire_level)"
    [ "$c" = "$e" ] && [ "$e" = "$w" ] || fail "$1: column $c, entry $e, classes[0].level $w"
    ok "$1: column, entry and classes[0].level are $c"
    LEVEL="$c"
}
use plain
award 2600 false
same_level "plain before its level-up" "${id[plain]}"
before_level="$LEVEL"
roll
same_level "plain after the level-up" "${id[plain]}"
[ "$LEVEL" -gt "$before_level" ] || fail "the level-up did not raise the level ($before_level -> $LEVEL)"
top="$(call GET /api/log | jq -r .topId)"
call POST /api/log/undo "{\"expectedTopId\":$top}" >/dev/null
same_level "plain after the undo" "${id[plain]}"
expect "the undo gave the level back" "$before_level" "$LEVEL"
after="$(entries)"

# --- 3. two restarts ---
for k in 1 2; do
    stop
    start "$work/new-srv" "$new_testdata"
    expect "restart $k leaves the entries as they were" "$after" "$(entries)"
done
stop

# --- 4. the rolling deploy: a character the release before writes beside the migrated database ---
# (a started old binary refuses a database with a migration it does not embed, ADR 0025 step 6.2; the
# one already running keeps writing the columns only, which the SQL below does the same way.)
start "$work/new-srv" "$new_testdata"
sql "INSERT INTO characters (name, class, level, xp_total, str, dex, con, int, wis, cha, hp_max, thac0_base, owner_user_id)
     SELECT 'newcomer', 'thief', 1, 2600, 12, 14, 15, 13, 11, 16, 6, 20, owner_user_id FROM characters WHERE id = ${id[plain]}" >/dev/null
id[newcomer]="$(sql "SELECT id FROM characters WHERE name = 'newcomer'")"
expect "a character written meanwhile has no entry yet" 0 "$(sql "SELECT COUNT(*) FROM character_classes WHERE character_id = ${id[newcomer]}")"
use newcomer
expect "this checkout reads it with the one entry its columns state" "thief 1" "$(first_class)"
roll
expect "its level-up works, the level served is 2" 2 "$(wire_level)"
expect "its level-up invents no entry" 0 "$(sql "SELECT COUNT(*) FROM character_classes WHERE character_id = ${id[newcomer]}")"
expect "its column took the level" 2 "$(col_level "${id[newcomer]}")"
stop
echo "class-entries-rehearsal: PASS"
