#!/usr/bin/env bash
# The skill armour upgrade rehearsal (ADR 0033 step 14, migration 020 and its carry-over): a database
# written by the release before is upgraded by this checkout's server, on a throwaway LOCAL compose
# PostgreSQL, then checked through the API and SQL.
#
#   scripts/skill-armour-rehearsal.sh [old ref]   # default: the merge base of HEAD and origin/main
#
# 1. The old release (built from <old ref>, which must not hold 020_skill_armour_worn.sql) writes
#    thieves with every entered skill armour ('' set through SQL, as the column's default; leather,
#    padded_studded_leather, elven_chain, none) and a bard with metal; a thief already wearing a
#    studded leather; one carrying a leather armour; one that took off its worn leather (the take-off
#    logged); one whose last entry is a skill_armor change. Every character's skill percentages are
#    read.
# 2. This checkout's server upgrades it: every skill percentage is the same as before (except the
#    thief already wearing armour, whose skills now follow that armour), the right piece is worn or
#    added, guessed pieces are on the review list.
# 3. Two restarts: no second piece, no second review row, nothing else changes.
# 4. The take-off then undo of before 020 (an accepted upgrade gap, ADR 0033 step 14 as built): the
#    result is printed, the undo must succeed.
# 5. The old skill_armor entry pops: no error, the journal still lists it, the skills don't move.
#
# Nothing touches the development database: the server is the `postgres` service of docker-compose.yml
# under its own compose project (criticalskill<pid>) on a free port with its own volume, removed at
# the end whatever the outcome. Fixture seeds and catalogue (api/testdata). Exit 0 only when every
# check passes. Needs docker, psql, jq, curl and go.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
fail() { echo "skill-armour-rehearsal: FAIL: $*" >&2; exit 1; }
ok() { echo "skill-armour-rehearsal: ok: $*"; }
note() { echo "skill-armour-rehearsal: observed: $*"; }

old_ref="${1:-$(git -C "$root" merge-base HEAD origin/main)}"
git -C "$root" cat-file -e "$old_ref:api/internal/storage/postgres/migrations/020_skill_armour_worn.sql" 2>/dev/null &&
    fail "$old_ref already holds the skill armour migration: pass a ref of before it"

free_port() { python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }
work="$(mktemp -d)"
project="criticalskill$$"
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
        echo "skill-armour-rehearsal: compose project $project removed"
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

# The characters by name, and the current one.
declare -A id
use() { call PUT /api/characters/current "{\"id\":${id[$1]}}" >/dev/null; }
# create <name> <class> <race> <skill armour>: a level 1 character with the dexterity and race that move
# the skills; its id in id[name], current.
create() {
    local out
    out="$(call POST /api/characters "{\"name\":\"$1\",\"class\":\"$2\",\"raceCode\":\"$3\",
        \"details\":{\"alignment\":\"true_neutral\",\"sex\":\"male\"},
        \"str\":10,\"dex\":17,\"con\":12,\"int\":14,\"wis\":9,\"cha\":16,\"hpMax\":6,\"xp\":0,\"skillArmor\":\"$4\"}")"
    id[$1]="$(jq -r .currentId <<<"$out")"
    [ "${id[$1]}" != null ] || fail "creating $1 set no current character: $out"
}
armour() { # armour <name> <base AC> <skill column> <armour row>: a carried body armour of the current character
    call POST /api/items "{\"name\":{\"en\":\"$1\"},\"category\":\"armour\",\"baseAc\":$2,\"skillColumn\":\"$3\",
        \"armorRow\":\"$4\",\"behaviour\":\"stock\",\"count\":1}" | jq -r '.item.id // .id'
}
wear() { call POST "/api/items/$1/state" "{\"state\":\"$2\"}" >/dev/null; }
# The current character's skills as the comparison reads them, one line per skill.
skills() { call GET /api/skills | jq -r '(.thiefSkills + .bardSkills)[] | "\(.id) \(.percent) armour \(.armor)"'; }
items() { call GET /api/items; }
worn_armour() { items | jq -r '[.[] | select(.category == "armour" and .state == "worn") | .id] | join(" ")'; }
armour_ids() { items | jq -r '[.[] | select(.category == "armour") | .id] | join(" ")'; }

# --- 1. the old release writes the characters ---
start "$work/old-srv" "$work/old/api/testdata"
create blank thief human leather
create leather thief human leather
create padded thief halfling padded_studded_leather
create elven thief elf elven_chain
create none thief human none
create metal bard half_elf metal
create wearing thief human leather
studded="$(armour "Studded Leather" 7 padded_studded studded_leather)"
wear "$studded" worn
create carrying thief human leather
carried="$(armour "Leather Armour" 8 leather leather)"
create tookoff thief human elven_chain
off="$(armour "Leather Armour" 8 leather leather)"
wear "$off" worn
wear "$off" carried
expect "old release: the take-off is the top entry" equipment_state "$(last_kind)"
create popped thief human leather
call PATCH "/api/characters/${id[popped]}" '{"skillArmor":"elven_chain"}' >/dev/null
expect "old release: the skill armour change is the top entry" skill_armor "$(last_kind)"
sql "UPDATE characters SET skill_armor = '' WHERE id = ${id[blank]}" >/dev/null
names=(blank leather padded elven none metal wearing carrying tookoff popped)
for n in "${names[@]}"; do
    use "$n"
    skills >"$work/before-$n"
    [ -s "$work/before-$n" ] || fail "$n has no skills on the old release"
done
expect "old release: the entered armours" "'' leather padded_studded_leather elven_chain none metal leather leather elven_chain elven_chain" \
    "$(for n in "${names[@]}"; do sql "SELECT CASE skill_armor WHEN '' THEN '''''' ELSE skill_armor END FROM characters WHERE id = ${id[$n]}"; done | paste -sd' ')"
stop
echo "skill-armour-rehearsal: old release wrote its data ($old_ref)"

new_testdata="$root/api/testdata"

# --- 2. this checkout upgrades it ---
start "$work/new-srv" "$new_testdata"
expect "migration 020 recorded once" 1 "$(sql "SELECT COUNT(*) FROM schema_migrations WHERE name = '020_skill_armour_worn.sql'")"
for n in blank leather padded elven none metal carrying tookoff popped; do
    use "$n"
    expect "$n: every skill percentage as before" "$(cat "$work/before-$n")" "$(skills)"
done
check() { # check <name> <worn armour ids> <every armour id> <skill armour read>
    use "$1"
    expect "$1: wears '$2', holds '$3', skills read $4" "$2|$3|$4" "$(worn_armour)|$(armour_ids)|$(call GET /api/skills | jq -r .skillArmor)"
}
check blank leather leather leather
check leather leather leather leather
check padded studded_leather studded_leather padded_studded_leather
check elven elven_chain_mail elven_chain_mail elven_chain
check none "" "" none
check metal chain_mail chain_mail metal
check wearing "$studded" "$studded" padded_studded_leather
check carrying "$carried" "$carried" leather
check tookoff elven_chain_mail "$off elven_chain_mail" elven_chain
check popped elven_chain_mail elven_chain_mail elven_chain
use wearing
note "wearing: before (entered leather): $(paste -sd, "$work/before-wearing")"
note "wearing: after (worn studded leather): $(skills | paste -sd,)"
reviews() { sql "SELECT character_id || ':' || item_id || ':' || reason FROM equipment_review WHERE reason = 'armour_guessed' ORDER BY 1" | paste -sd' '; }
expect "the guessed pieces are on the review list" "${id[padded]}:studded_leather:armour_guessed ${id[metal]}:chain_mail:armour_guessed" "$(reviews)"
use leather
expect "the added piece is named in the catalogue's languages" "$(jq -c '[.equipment.armour[] | select(.code == "leather") | .name | to_entries[] | .key] | sort' "$new_testdata/catalogue.json")" \
    "$(items | jq -c '[.[] | select(.id == "leather") | .name | keys[]] | sort')"

# --- 3. two restarts: nothing added again ---
state() {
    sql "SELECT character_id || ' ' || id || ' ' || category || ' ' || state || ' ' || count FROM items ORDER BY 1"
    reviews
    sql "SELECT COUNT(*) FROM localized_texts"
    sql "SELECT COUNT(*) FROM schema_migrations"
    sql "SELECT COUNT(*) FROM action_log"
}
before="$(state)"
for n in 1 2; do
    stop
    start "$work/new-srv" "$new_testdata"
    expect "restart $n leaves the pieces, reviews, texts, migrations and journal as they were" "$before" "$(state)"
done

# --- 4. the take-off of before 020, undone (accepted upgrade gap, ADR 0033 step 14 as built) ---
use tookoff
expect "undo the old take-off" equipment_state "$(undo)"
note "tookoff after the undo: worn armour '$(worn_armour)', skills read $(call GET /api/skills | jq -r .skillArmor), AC $(call GET /api/ac | jq -r .ac)"
note "tookoff skills: $(skills | paste -sd,)"

# --- 5. the old skill_armor entry pops on the upgraded database ---
use popped
after="$(skills)"
expect "the journal still lists the old skill_armor entry" skill_armor "$(last_kind)"
expect "pop the old skill_armor entry" skill_armor "$(undo)"
expect "the entered column is written back" leather "$(sql "SELECT skill_armor FROM characters WHERE id = ${id[popped]}")"
expect "the skills don't move" "$after" "$(skills)"
expect "the popped entry is the redo" skill_armor "$(call GET /api/log | jq -r '.redo.kind // empty')"

stop
echo "skill-armour-rehearsal: PASS"
