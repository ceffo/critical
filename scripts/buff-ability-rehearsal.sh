#!/usr/bin/env bash
# The buff item ability on PostgreSQL (ADR 0049 steps 9, 10 and 12, slice 8), through the real API on a
# throwaway LOCAL compose database, checked through the API and SQL:
#
#   scripts/buff-ability-rehearsal.sh
#
# 1. A potion with a heal effect (fill to max HP) and a buff effect: after Use the dose count, the HP and the
#    buff row in active_buffs; undo gives all three back; redo returns them.
# 2. A potion whose second effect fails (a negative rounds value written straight into items.abilities, past the
#    editor): the use is refused, no buff row, the dose count and the journal unchanged.
# 3. A second potion with a buff of the same name: one row in active_buffs, renewed (its rounds and effects).
#
# Nothing touches the development database: the server is the `postgres` service of docker-compose.yml under its
# own compose project (criticalbuff<pid>) on a free port with its own volume, removed at the end whatever the
# outcome. Fixture seeds and catalogue (api/testdata). Exit 0 only when every check passes. Needs docker, psql,
# jq, curl and go.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
fail() { echo "buff-rehearsal: FAIL: $*" >&2; exit 1; }
ok() { echo "buff-rehearsal: ok: $*"; }

free_port() { python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }
work="$(mktemp -d)"
project="criticalbuff$$"
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
        echo "buff-rehearsal: compose project $project removed"
    fi
    [ "$status" -ne 0 ] && [ -f "$work/server.log" ] && tail -n 30 "$work/server.log" >&2
    rm -rf "$work"
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

(cd "$root/api" && go build -o "$work/srv" ./cmd/api) || fail "could not build this checkout"

cd "$root"
started=1
docker compose up -d postgres >/dev/null 2>&1 || fail "could not start the scratch PostgreSQL (docker running?)"
for _ in $(seq 1 60); do
    psql "$db" -X -q -c 'SELECT 1' >/dev/null 2>&1 && break
    sleep 1
done
psql "$db" -X -q -c 'SELECT 1' >/dev/null 2>&1 || fail "the scratch database critical_fixture did not accept connections"
sql() { psql "$db" -X -q -t -A -c "$1"; }

CRITICAL_DSN="postgres://critical:critical@127.0.0.1:$pg_port/critical_fixture?sslmode=disable" \
    CRITICAL_SEEDS_DIR="$root/api/testdata/seeds" CRITICAL_CATALOGUE="$root/api/testdata/catalogue.json" CRITICAL_ENV=dev \
    CRITICAL_ADDR="127.0.0.1:$api_port" "$work/srv" >>"$work/server.log" 2>&1 &
srv_pid=$!
for _ in $(seq 1 100); do
    curl -fsS "$api/api/health" >/dev/null 2>&1 && break
    kill -0 "$srv_pid" 2>/dev/null || fail "the server exited at start"
    sleep 0.1
done
curl -fsS "$api/api/health" >/dev/null || fail "the server did not answer"
curl -fsS -c "$jar" -X POST "$api/api/auth/dev" -H 'Content-Type: application/json' -d '{}' >/dev/null || fail "dev login refused"

# call <method> <path> [body]: the JSON answer; fails on any non-2xx.
call() {
    local out code
    out="$(curl -sS -b "$jar" -X "$1" "$api$2" -H 'Content-Type: application/json' ${3:+-d "$3"} -w '\n%{http_code}')"
    code="${out##*$'\n'}"
    out="${out%$'\n'*}"
    [[ "$code" == 2* ]] || fail "$1 $2 answered $code: $out"
    printf '%s' "$out"
}
status_of() { curl -sS -o /dev/null -b "$jar" -X "$1" "$api$2" -H 'Content-Type: application/json' ${3:+-d "$3"} -w '%{http_code}'; }
expect() { [ "$2" = "$3" ] || fail "$1: want '$2', got '$3'"; ok "$1"; }
undo() { call POST /api/log/undo "{\"expectedTopId\":$(call GET /api/log | jq .topId)}" | jq -r .kind; }
redo() { call POST /api/log/redo "{\"expectedRedoId\":$(call GET /api/log | jq .redo.id)}" | jq -r .kind; }
count() { sql "SELECT count FROM items WHERE id = '$1'"; }
buffs() { sql "SELECT buffs FROM active_buffs"; }
nbuffs() { buffs | jq length; }
hp() { call GET /api/hp | jq -r '.current // .hp // .currentHp'; }
entries() { call GET /api/log | jq '.entries | length'; }

potion() { # potion <name> <effects json>
    call POST /api/items "$(jq -nc --arg n "$1" --argjson e "$2" '{name:{en:$n},category:"magic",magicKind:"potion",behaviour:"stock",count:3,
        abilities:[{id:"drink",limit:{kind:"with_item"},effects:$e}]}')" >/dev/null
}
heal='{"kind":"heal","full":true}'
heroism='{"kind":"buff","name":"Heroism","rounds":10,"buff":{"to_hit":1}}'
valor='{"kind":"buff","name":"Heroism","rounds":20,"buff":{"save_bonus":2}}'

max_hp="$(hp)"
call POST /api/hp/event '{"delta":-9,"source":"rehearsal"}' >/dev/null
expect "the character is wounded" "$((max_hp - 9))" "$(hp)"
expect "no buff yet" 0 "$(nbuffs)"

# --- 1. heal + buff: use, undo, redo ---
potion "Potion of heroism" "[$heal,$heroism]"
call POST /api/items/potion_of_heroism/abilities/drink/use '' >/dev/null
expect "use: the dose is spent" 2 "$(count potion_of_heroism)"
expect "use: the HP is back at max" "$max_hp" "$(hp)"
expect "use: one buff row, Heroism for 10 rounds" "Heroism 10 1" "$(buffs | jq -r '.[] | "\(.Name // .name) \(.Duration // .duration) \(.Effects.ToHit // .effects.to_hit)"')"
expect "undo pops the use" item_use "$(undo)"
expect "undo: the dose is back" 3 "$(count potion_of_heroism)"
expect "undo: the HP is back to wounded" "$((max_hp - 9))" "$(hp)"
expect "undo: no buff row" 0 "$(nbuffs)"
expect "redo returns the use" item_use "$(redo)"
expect "redo: the dose is spent again" 2 "$(count potion_of_heroism)"
expect "redo: the HP is at max" "$max_hp" "$(hp)"
expect "redo: the buff row is back" 1 "$(nbuffs)"

# --- 3. the same buff name from a second item: renewed ---
potion "Elixir of valor" "[$valor]"
call POST /api/items/elixir_of_valor/abilities/drink/use '' >/dev/null
expect "second item: still one buff row" 1 "$(nbuffs)"
expect "second item: renewed with its rounds and effects" "20 2 null" \
    "$(buffs | jq -r '.[0] | "\(.Duration // .duration) \(.Effects.SaveBonus // .effects.save_bonus) \(.Effects.ToHit // .effects.to_hit)"')"
expect "second item: its dose is spent" 2 "$(count elixir_of_valor)"

# --- 2. a failing second effect rolls the use back ---
potion "Potion of ruin" "[$heroism]"
sql "UPDATE items SET abilities = '[{\"id\":\"drink\",\"limit\":{\"kind\":\"with_item\"},\"effects\":[{\"kind\":\"buff\",\"name\":\"Ruin\",\"rounds\":5,\"buff\":{\"ac\":-1}},{\"kind\":\"buff\",\"name\":\"Broken\",\"rounds\":-1,\"buff\":{}}]}]' WHERE id = 'potion_of_ruin'" >/dev/null
buffs_before="$(buffs)" entries_before="$(entries)"
code="$(status_of POST /api/items/potion_of_ruin/abilities/drink/use '')"
[[ "$code" != 2* ]] || fail "the failing use answered $code"
ok "the failing use is refused ($code)"
expect "failure: no Ruin row, the buff list is as it was" "$buffs_before" "$(buffs)"
expect "failure: the dose count is unchanged" 3 "$(count potion_of_ruin)"
expect "failure: nothing was logged" "$entries_before" "$(entries)"
echo "buff-rehearsal: all checks passed"
