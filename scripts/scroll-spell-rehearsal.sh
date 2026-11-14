#!/usr/bin/env bash
# The scroll spell conversion rehearsal (ADR 0049 step 9, slice 9b): the boot step that folds a scroll's
# spell (items.spell_id) into a spell effect of its with_item ability, at the caster level DMG p.145 gives
# it (one above the minimum level that casts the spell, never below 6th), on a COPY of a real database
# and on a scratch one, through this checkout's real server, checked through SQL and the API.
#
#   scripts/scroll-spell-rehearsal.sh [source container] [source database]
#       default: critical-postgres-1 critical (the main checkout's `just dev` database)
#
# On the copy (real seeds and catalogue) and on a scratch database (fixture seeds and catalogue):
# 1. The copy is started: every migration and the abilities conversion run, no scroll is touched when the
#    database holds none (the log line says scrolls=0). The source is only read (pg_dump).
# 2. Scrolls are added through the API: a spell effect sent with no level takes the book's (the editor's
#    default), then put back in the state a database converted by the release before leaves them (the
#    ability with no effect, spell_id set), one more as a row an older release wrote after a rollback
#    (abilities NULL, spell_id set), one with a spell effect whose level the player set, one with a spell
#    the catalogue does not know, one without a spell.
# 3. The restart folds them: each spell effect is on the first with_item ability at the level of the book's
#    own examples (a 6th-level wizard spell at 13th; the 7th-level one, 15th, is the unit test's), spell_id is untouched, the player's
#    level, the unknown spell and the other items are as they were, the log line counts what it did.
# 4. A second restart changes no row of the items table (a checksum) and the log line says scrolls=0.
# 5. The release before (built from the merge base with origin/main) starts on the converted database and
#    serves the scroll with its spell_id and its effect, and a use of it works.
#
# Nothing touches the development database: both databases live in the `postgres` service of
# docker-compose.yml under its own compose project (criticalscroll<pid>) on a free port with its own
# volume, removed at the end whatever the outcome. Exit 0 only when every check passes. Needs docker, psql,
# pg_restore, jq, curl and go; the copy needs the real catalogue (.catalogue/catalogue.json, `just catalogue`).
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
fail() { echo "scroll-rehearsal: FAIL: $*" >&2; exit 1; }
ok() { echo "scroll-rehearsal: ok: $*"; }

source_container="${1:-critical-postgres-1}"
source_db="${2:-critical}"
catalogue="${CRITICAL_REAL_CATALOGUE:-$root/.catalogue/catalogue.json}"
if [ ! -f "$catalogue" ]; then # a linked worktree has none: the main checkout's
    main="$(dirname "$(git -C "$root" rev-parse --path-format=absolute --git-common-dir)")"
    catalogue="$main/.catalogue/catalogue.json"
fi
[ -f "$catalogue" ] || fail "no real catalogue: run just catalogue"
old_ref="$(git -C "$root" merge-base HEAD origin/main)"

free_port() { python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }
work="$(mktemp -d)"
project="criticalscroll$$"
pg_port="$(free_port)"
api_port="$(free_port)"
export COMPOSE_PROJECT_NAME="$project" CRITICAL_PG_PORT="$pg_port"
export PGPASSWORD=critical # the compose database's own throwaway account (docker-compose.yml)
api="http://127.0.0.1:$api_port"
jar="$work/cookies"
started="" srv_pid=""

cleanup() {
    status=$?
    set +e
    [ -n "$srv_pid" ] && kill "$srv_pid" 2>/dev/null && wait "$srv_pid" 2>/dev/null
    if [ -n "$started" ]; then
        (cd "$root" && docker compose down --volumes --remove-orphans >/dev/null 2>&1)
        echo "scroll-rehearsal: compose project $project removed"
    fi
    [ "$status" -ne 0 ] && [ -f "$work/server.log" ] && tail -n 30 "$work/server.log" >&2
    rm -rf "$work"
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

# --- binaries: this checkout and the release before ---
mkdir -p "$work/old"
git -C "$root" archive "$old_ref" api | tar -x -C "$work/old"
(cd "$work/old/api" && go build -o "$work/old-srv" ./cmd/api) || fail "could not build the release before ($old_ref)"
(cd "$root/api" && go build -o "$work/new-srv" ./cmd/api) || fail "could not build this checkout"

# --- the databases: the copy of the source, and a scratch one ---
docker exec "$source_container" pg_dump -U critical -Fc "$source_db" >"$work/source.dump" ||
    fail "could not dump $source_db from $source_container"
cd "$root"
started=1
docker compose up -d postgres >/dev/null 2>&1 || fail "could not start the scratch PostgreSQL (docker running?)"
admin="host=127.0.0.1 port=$pg_port user=critical dbname=critical"
for _ in $(seq 1 60); do
    psql "$admin" -X -q -c 'SELECT 1' >/dev/null 2>&1 && break
    sleep 1
done
psql "$admin" -X -q -c 'CREATE DATABASE copy' >/dev/null || fail "the scratch PostgreSQL did not accept connections"
pg_restore --no-owner -d "postgresql://critical:critical@127.0.0.1:$pg_port/copy" "$work/source.dump" ||
    fail "could not restore the copy"
psql "$admin" -X -q -c 'CREATE DATABASE scratch' >/dev/null || fail "could not create the scratch database"

start() { # start <binary> <database> <seeds> <catalogue>
    CRITICAL_DSN="postgres://critical:critical@127.0.0.1:$pg_port/$2?sslmode=disable" \
        CRITICAL_SEEDS_DIR="$3" CRITICAL_CATALOGUE="$4" CRITICAL_ENV=dev \
        CRITICAL_ADDR="127.0.0.1:$api_port" "$1" >>"$work/server.log" 2>&1 &
    srv_pid=$!
    for _ in $(seq 1 200); do
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
call() { # call <method> <path> [body]: the JSON answer; fails on any non-2xx
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
# The log line of the last start: "items converted to abilities ... scrolls=N scrollsSkipped=M".
last_line() { grep 'items converted to abilities' "$work/server.log" | tail -n 1; }
field() { sed -n "s/.* $1=\([0-9]*\).*/\1/p" <<<"$(last_line)"; }

scenario() { # scenario <name> <database> <seeds> <catalogue>
    local name="$1" database="$2" seeds="$3" cat="$4"
    local sql=(psql "host=127.0.0.1 port=$pg_port user=critical dbname=$database" -X -q -t -A -c)
    echo "scroll-rehearsal: === $name ==="

    # 1. the first start: migrations and the abilities conversion; the database holds no scroll
    start "$work/new-srv" "$database" "$seeds" "$cat"
    expect "$name: no scroll in the database, none folded" "0" "$(field scrolls)"
    [ -n "$("${sql[@]}" "SELECT 1 FROM items LIMIT 1")" ] || fail "$name: the database holds no item"
    scroll() { # scroll <name> <spell> [level]
        call POST /api/items "$(jq -nc --arg n "$1" --arg s "$2" --arg l "${3:-}" '{name:{en:$n},category:"magic",magicKind:"scroll",
            behaviour:"stock",count:2,abilities:[{id:"use",limit:{kind:"with_item"},
            effects:[{kind:"spell",spellId:$s} + (if $l == "" then {} else {level:($l|tonumber)} end)]}]}')" >/dev/null
    }
    # 2. scrolls through the API: the level is the book's when none is sent
    scroll "Scroll of chain lightning" chain_lightning
    scroll "Scroll of incendiary cloud" incendiary_cloud
    scroll "Scroll of fireball" fireball
    scroll "Scroll of cure light wounds" cure_light_wounds
    scroll "Scroll of flame strike" flame_strike
    scroll "Scroll of meteor swarm" meteor_swarm
    scroll "Scroll of magic missile" magic_missile 9
    effect() { "${sql[@]}" "SELECT abilities::jsonb #>> '{0,effects,0,level}' FROM items WHERE id = '$1'"; }
    expect "$name: a spell effect sent with no level takes the book's (6th wizard: 13)" 13 "$(effect scroll_of_chain_lightning)"
    expect "$name: an 8th-level wizard spell is written at 17 (its first slot is at 16)" 17 "$(effect scroll_of_incendiary_cloud)"
    expect "$name: a 3rd-level wizard spell is at the 6th-level floor" 6 "$(effect scroll_of_fireball)"
    expect "$name: a 1st-level priest spell is at the floor" 6 "$(effect scroll_of_cure_light_wounds)"
    expect "$name: a 5th-level priest spell is written at 10" 10 "$(effect scroll_of_flame_strike)"
    expect "$name: a 9th-level wizard spell is written at 19" 19 "$(effect scroll_of_meteor_swarm)"
    expect "$name: the level a player sets is kept" 9 "$(effect scroll_of_magic_missile)"
    expect "$name: spell_id mirrors the effect" "chain_lightning" "$("${sql[@]}" "SELECT spell_id FROM items WHERE id = 'scroll_of_chain_lightning'")"
    # a scroll with no spell, and one whose spell is not in the catalogue (stored by hand: the editor refuses it)
    call POST /api/items '{"name":{"en":"Blank scroll"},"category":"magic","magicKind":"scroll","behaviour":"stock","count":1,"abilities":[{"id":"use","limit":{"kind":"with_item"},"effects":[]}]}' >/dev/null
    stop

    # the states a converted database and an older release leave: no effect, spell_id kept (the six first
    # scrolls); abilities NULL with spell_id (one more); an unknown spell with no effect
    local bare='[{"id":"use","limit":{"kind":"with_item"},"effects":[]}]'
    "${sql[@]}" "UPDATE items SET abilities = '$bare' WHERE id IN ('scroll_of_chain_lightning','scroll_of_incendiary_cloud','scroll_of_fireball','scroll_of_cure_light_wounds','scroll_of_flame_strike','scroll_of_meteor_swarm')" >/dev/null
    "${sql[@]}" "UPDATE items SET abilities = NULL WHERE id = 'scroll_of_flame_strike'" >/dev/null
    "${sql[@]}" "UPDATE items SET spell_id = 'gone_from_the_catalogue', abilities = '$bare' WHERE id = 'blank_scroll'" >/dev/null
    others_before="$("${sql[@]}" "SELECT md5(string_agg(i::text, '|' ORDER BY character_id, id)) FROM items i WHERE magic_kind <> 'scroll'")"
    missile_before="$("${sql[@]}" "SELECT abilities FROM items WHERE id = 'scroll_of_magic_missile'")"
    spells_before="$("${sql[@]}" "SELECT string_agg(id || '=' || spell_id, ',' ORDER BY id) FROM items WHERE magic_kind = 'scroll'")"

    # 3. the restart folds them
    start "$work/new-srv" "$database" "$seeds" "$cat"
    expect "$name: the log line counts the folded scrolls" "6" "$(field scrolls)"
    expect "$name: and the one it cannot fold (an unknown spell)" "1" "$(field scrollsSkipped)"
    folded() { "${sql[@]}" "SELECT abilities::jsonb -> 0 -> 'effects' FROM items WHERE id = '$1'" | jq -c .; }
    expect "$name: a 6th-level wizard spell: level 13" '[{"kind":"spell","level":13,"spellId":"chain_lightning"}]' "$(folded scroll_of_chain_lightning | jq -cS .)"
    expect "$name: an 8th-level wizard spell: level 17" 17 "$(effect scroll_of_incendiary_cloud)"
    expect "$name: a 3rd-level one: the floor, 6" 6 "$(effect scroll_of_fireball)"
    expect "$name: a priest's 1st-level spell: the floor, 6" 6 "$(effect scroll_of_cure_light_wounds)"
    expect "$name: a priest's 5th-level spell: 10 (the row an older release wrote, NULL, is converted and folded)" 10 "$(effect scroll_of_flame_strike)"
    expect "$name: a 9th-level wizard spell: 19" 19 "$(effect scroll_of_meteor_swarm)"
    expect "$name: the ability stays with the item, whatever it held" with_item "$("${sql[@]}" "SELECT abilities::jsonb #>> '{0,limit,kind}' FROM items WHERE id = 'scroll_of_flame_strike'")"
    expect "$name: the player's level is not rewritten" "$missile_before" "$("${sql[@]}" "SELECT abilities FROM items WHERE id = 'scroll_of_magic_missile'")"
    expect "$name: an unknown spell is left as stored" "$bare" "$("${sql[@]}" "SELECT abilities FROM items WHERE id = 'blank_scroll'")"
    expect "$name: spell_id is untouched on every scroll" "$spells_before" "$("${sql[@]}" "SELECT string_agg(id || '=' || spell_id, ',' ORDER BY id) FROM items WHERE magic_kind = 'scroll'")"
    expect "$name: the quantity is untouched" 2 "$("${sql[@]}" "SELECT count FROM items WHERE id = 'scroll_of_chain_lightning'")"
    expect "$name: no other item is changed" "$others_before" "$("${sql[@]}" "SELECT md5(string_agg(i::text, '|' ORDER BY character_id, id)) FROM items i WHERE magic_kind <> 'scroll'")"
    stop

    # 4. the second restart changes nothing
    items_sum() { "${sql[@]}" "SELECT md5(string_agg(i::text, '|' ORDER BY character_id, id)) FROM items i"; }
    sum_before="$(items_sum)"
    start "$work/new-srv" "$database" "$seeds" "$cat"
    expect "$name: a second start folds nothing" "0" "$(field scrolls)"
    expect "$name: a second start leaves every row of items as it was" "$sum_before" "$(items_sum)"
    stop

    # 5. the release before on the converted database
    start "$work/old-srv" "$database" "$seeds" "$cat"
    stored="$(call GET /api/items | jq -c '.[] | select(.id == "scroll_of_chain_lightning") | [.spellId, .abilities[0].effects[0].spellId, .abilities[0].effects[0].level]')"
    expect "$name: the release before serves the scroll with spell_id and its spell effect" '["chain_lightning","chain_lightning",13]' "$stored"
    stop
}

scenario "copy of $source_db" copy "$root/seeds" "$catalogue"
scenario "scratch (fixture)" scratch "$root/api/testdata/seeds" "$root/api/testdata/catalogue.json"
echo "scroll-rehearsal: all checks passed"
