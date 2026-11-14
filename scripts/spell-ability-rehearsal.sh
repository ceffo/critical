#!/usr/bin/env bash
# The spell item ability on PostgreSQL (ADR 0049 steps 9, 10 and 12, slice 9a), through the real API on a
# throwaway LOCAL compose database, checked through the API and SQL:
#
#   scripts/spell-ability-rehearsal.sh
#
# 1. A scroll of Cure Light Wounds (a spell effect, with_item): Use heals through the HP ledger and spends the
#    last scroll (the row is gone); undo gives the scroll and the points back; redo returns the use.
# 2. A wand of Magic Missile at caster level 5 (charges 5): Use spends one charge and no slot; undo gives it back.
# 3. A wand of Chromatic Orb: Use with no answers is a 400 and changes nothing (charge, gem, journal); with the
#    target AC it rolls the to-hit and spends the gem and the charge in one entry; one undo gives both back.
# 4. A wand whose second spell effect asks an AC nobody gave: the whole use is refused, the first effect's heal and
#    the charge included (HP, charge and journal unchanged).
# 5. A spell no catalogue knows is refused by the editor (400, nothing stored).
#
# Nothing touches the development database: the server is the `postgres` service of docker-compose.yml under its
# own compose project (criticalspell<pid>) on a free port with its own volume, removed at the end whatever the
# outcome. Fixture seeds and catalogue (api/testdata). Exit 0 only when every check passes. Needs docker, psql,
# jq, curl and go.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
fail() { echo "spell-rehearsal: FAIL: $*" >&2; exit 1; }
ok() { echo "spell-rehearsal: ok: $*"; }

free_port() { python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }
work="$(mktemp -d)"
project="criticalspell$$"
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
        echo "spell-rehearsal: compose project $project removed"
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

undo() { call POST /api/log/undo "{\"expectedTopId\":$(call GET /api/log | jq .topId)}" | jq -r .kind; }
redo() { call POST /api/log/redo "{\"expectedRedoId\":$(call GET /api/log | jq .redo.id)}" | jq -r .kind; }
rows() { sql "SELECT count(*) FROM items WHERE id = '$1'"; }
count() { sql "SELECT count FROM items WHERE id = '$1'"; }
left() { sql "SELECT abilities FROM items WHERE id = '$1'" | jq -r '.[0].limit.left'; }
hp() { call GET /api/hp | jq -r '.current // .hp // .currentHp'; }
entries() { call GET /api/log | jq '.entries | length'; }

item() { # item <json>
    call POST /api/items "$1" >/dev/null
}
scroll() { # scroll <name> <effects json>
    item "$(jq -nc --arg n "$1" --argjson e "$2" '{name:{en:$n},category:"magic",magicKind:"scroll",behaviour:"stock",count:1,
        abilities:[{id:"read",limit:{kind:"with_item"},effects:$e}]}')"
}
wand() { # wand <name> <effects json>
    item "$(jq -nc --arg n "$1" --argjson e "$2" '{name:{en:$n},category:"magic",magicKind:"wand",behaviour:"stock",count:1,
        abilities:[{id:"zap",limit:{kind:"charges",max:5,left:5,cost:1},effects:$e}]}')"
}
cure='{"kind":"spell","spellId":"cure_light_wounds","level":1}'
missile='{"kind":"spell","spellId":"magic_missile","level":5}'
orb='{"kind":"spell","spellId":"chromatic_orb","level":3}'

max_hp="$(hp)"
call POST /api/hp/event '{"delta":-9,"source":"rehearsal"}' >/dev/null
wounded=$((max_hp - 9))
expect "the character is wounded" "$wounded" "$(hp)"

# --- 1. a scroll of a heal spell: use, undo, redo ---
scroll "Scroll of cure light wounds" "[$cure]"
res="$(call POST /api/items/scroll_of_cure_light_wounds/abilities/read/use '')"
healed="$(jq -r '.effects[0].applied.hpDelta' <<<"$res")"
[ "$healed" -ge 1 ] && [ "$healed" -le 8 ] || fail "the d8 healed $healed"
ok "use: the heal is a d8 ($healed)"
expect "use: the answer carries the spell's own breakdown" "item cure_light_wounds" "$(jq -r '.effects[0].cast | "\(.action) \(.spellId)"' <<<"$res")"
expect "use: the HP went up by what the answer says" "$((wounded + healed))" "$(hp)"
expect "use: the last scroll is gone" 0 "$(rows scroll_of_cure_light_wounds)"
expect "undo pops the use" item_use "$(undo)"
expect "undo: the scroll is back" 1 "$(count scroll_of_cure_light_wounds)"
expect "undo: the HP is back" "$wounded" "$(hp)"
expect "redo returns the use" item_use "$(redo)"
expect "redo: the scroll is gone again" 0 "$(rows scroll_of_cure_light_wounds)"
expect "redo: the HP is healed again" "$((wounded + healed))" "$(hp)"
undo >/dev/null # the character is wounded again for the next checks

# --- 2. a wand of a spell at a fixed level: a charge, no slot ---
wand "Wand of missiles" "[$missile]"
res="$(call POST /api/items/wand_of_missiles/abilities/zap/use '')"
expect "wand: one charge spent" 4 "$(left wand_of_missiles)"
expect "wand: cast at the effect's level" 5 "$(jq -r '.effects[0].cast.casterLevel' <<<"$res")"
[ "$(jq -r '.effects[0].cast.damage.total' <<<"$res")" -ge 6 ] || fail "three missiles of 1d4+1 are at least 6"
ok "wand: three missiles rolled (level 5)"
expect "wand: undo pops the use" item_use "$(undo)"
expect "wand: undo gives the charge back" 5 "$(left wand_of_missiles)"

# --- 3. the asks: refused with none, answered, one undo ---
item '{"name":{"en":"Opal"},"category":"gear","component":"gem","behaviour":"stock","count":2}'
wand "Wand of orbs" "[$orb]"
entries_before="$(entries)"
code="$(status_of POST /api/items/wand_of_orbs/abilities/zap/use '')"
expect "asks: no answer is a 400" 400 "$code"
expect "asks: the charge is unchanged" 5 "$(left wand_of_orbs)"
expect "asks: the gem is unchanged" 2 "$(count opal)"
expect "asks: nothing was logged" "$entries_before" "$(entries)"
res="$(call POST /api/items/wand_of_orbs/abilities/zap/use '{"spells":[{"effect":0,"targetAc":12}]}')"
expect "asks: answered, the charge is spent" 4 "$(left wand_of_orbs)"
expect "asks: the gem is spent" 1 "$(count opal)"
expect "asks: the to-hit is shown" true "$(jq '.effects[0].cast.attack != null' <<<"$res")"
expect "asks: one entry" "$((entries_before + 1))" "$(entries)"
expect "asks: undo pops the use" item_use "$(undo)"
expect "asks: undo gives the charge back" 5 "$(left wand_of_orbs)"
expect "asks: undo gives the gem back" 2 "$(count opal)"

# --- the body: none, empty and malformed ---
expect "body: malformed JSON is a 400" 400 "$(status_of POST /api/items/wand_of_orbs/abilities/zap/use '{nope')"
expect "body: malformed JSON spent nothing" 5 "$(left wand_of_orbs)"
expect "body: an empty object is the defaults (a 400 here: the AC is asked)" 400 "$(status_of POST /api/items/wand_of_orbs/abilities/zap/use '{}')"

# --- 4. a failing second spell effect rolls the whole use back ---
wand "Wand of ruin" "[$cure,$orb]"
hp_before="$(hp)" entries_before="$(entries)"
code="$(status_of POST /api/items/wand_of_ruin/abilities/zap/use '')"
expect "rollback: the use is refused" 400 "$code"
expect "rollback: the charge is unchanged" 5 "$(left wand_of_ruin)"
expect "rollback: the HP is unchanged" "$hp_before" "$(hp)"
expect "rollback: nothing was logged" "$entries_before" "$(entries)"

# --- 5. an unknown spell is refused by the editor ---
code="$(status_of POST /api/items "$(jq -nc '{name:{en:"Wand of nothing"},category:"magic",magicKind:"wand",behaviour:"stock",count:1,
    abilities:[{id:"zap",limit:{kind:"unlimited"},effects:[{kind:"spell",spellId:"no_such_spell",level:5}]}]}')")"
expect "editor: an unknown spell is a 400" 400 "$code"
expect "editor: nothing stored" 0 "$(rows wand_of_nothing)"
echo "spell-rehearsal: all checks passed"
