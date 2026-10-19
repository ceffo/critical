#!/usr/bin/env bash
# The equipment-model upgrade rehearsal (ADR 0033 steps 6, 7, 26 and 27, step 1.1): a database written
# by the release before the one equipment model is upgraded by this checkout's server, on a throwaway
# LOCAL compose PostgreSQL, then checked through the API and SQL.
#
#   scripts/equipment-upgrade-rehearsal.sh [old ref]   # default: the merge base of HEAD and origin/main
#
# 1. The old release (built from <old ref>, which must not hold 015_equipment.sql) migrates and seeds the
#    fixture character, then the API writes what a player had: an item whose id a weapon has (dagger),
#    "Arrows" with the ammo kind "arrows", a plain gem (Ruby), an attack with the bow and arrows (the
#    stored form keeps the bow), then a weapon_edit, an item_use of the dagger item and an item_remove.
# 2. This checkout's server upgrades it: pieces, review rows, the rename, names intact.
# 3. The three old entries pop and redo through the one model.
# 4. Two restarts: no second conversion, no duplicate review row, the seed only reconciles names.
# 5. The converted bow and arrows attack with the same dice and to-hit as before; the form keeps the bow.
# 6. The attack form picks a carried weapon (one equipment_state line), undo empties the hand, redo.
# 7. A second shield worn with swap: one journal line; undo restores both.
# 8. Chromatic Orb spends the converted gem and a gem added with the component code.
#
# Nothing touches the development database: the server is the `postgres` service of docker-compose.yml
# under its own compose project (criticalequip<pid>) on a free port with its own volume, removed at the
# end whatever the outcome. Fixture seeds and catalogue (api/testdata). Exit 0 only when every check
# passes. Needs docker, psql, jq, curl and go.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
fail() { echo "equipment-rehearsal: FAIL: $*" >&2; exit 1; }
ok() { echo "equipment-rehearsal: ok: $*"; }

old_ref="${1:-$(git -C "$root" merge-base HEAD origin/main)}"
git -C "$root" cat-file -e "$old_ref:api/internal/storage/postgres/migrations/015_equipment.sql" 2>/dev/null &&
    fail "$old_ref already holds the equipment migration: pass a ref of before it"

free_port() { python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }
work="$(mktemp -d)"
project="criticalequip$$"
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
        echo "equipment-rehearsal: compose project $project removed"
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
# The top entry's kind (what undo pops), the redo entry's kind; undo and redo answer the entry's kind.
last_kind() { call GET /api/log | jq -r '.topId as $t | (.entries[] | select(.id == $t) | .kind) // empty'; }
redo_kind() { call GET /api/log | jq -r '.redo.kind // empty'; }
undo() { call POST /api/log/undo "{\"expectedTopId\":$(call GET /api/log | jq .topId)}" | jq -r .kind; }
redo() { call POST /api/log/redo "{\"expectedRedoId\":$(call GET /api/log | jq .redo.id)}" | jq -r .kind; }

# --- 1. the old release writes a player's data ---
start "$work/old-srv" "$work/old/api/testdata"
call POST /api/items '{"name":{"en":"Dagger"},"type":"knife","behaviour":"stock","count":2}' >/dev/null
call POST /api/items '{"name":{"en":"Arrows"},"type":"arrows","behaviour":"stock","count":12,"ammoKind":"arrows"}' >/dev/null
call POST /api/items '{"name":{"en":"Ruby"},"type":"gem","behaviour":"stock","count":2}' >/dev/null
call PATCH /api/attack/form '{"weaponId":"shortbow","ammoItemId":"arrows_plus1","targetAc":5}' >/dev/null
old_attack="$(call POST /api/roll/attack '{"weapon_id":"shortbow","ammo_item_id":"arrows_plus1","target_ac":5}')"
call PATCH /api/weapons/longsword '{"magic_to_hit":3}' >/dev/null
call POST /api/items/dagger/use '' >/dev/null
call DELETE /api/items/fire_flask '' >/dev/null
expect "old release: the top entry is the item_remove" item_remove "$(last_kind)"
old_names="$(sql "SELECT 'item:' || i.id || '=' || t.body FROM items i JOIN localized_texts t ON t.text_id = i.name
                 UNION ALL SELECT 'weapon:' || w.id || '=' || t.body FROM weapons w JOIN localized_texts t ON t.text_id = w.name
                 ORDER BY 1")"
stop
echo "$old_attack" >"$work/old-attack.json"
echo "equipment-rehearsal: old release wrote its data ($old_ref)"

new_testdata="$root/api/testdata"
items() { call GET /api/items; }
piece() { items | jq -r --arg id "$1" ".[] | select(.id == \$id) | $2"; }

# --- 2. this checkout upgrades it ---
start "$work/new-srv" "$new_testdata"
expect "one conversion recorded" 1 "$(sql "SELECT COUNT(*) FROM schema_migrations WHERE name = '015_equipment.sql'")"
expect "every item and weapon is a piece" 14 "$(items | jq length)"
expect "the item whose id a weapon had is renamed (step 26.3)" "dagger=dagger_2" "$(sql "SELECT old_id || '=' || new_id FROM item_id_renames")"
expect "the dagger is the weapon, wielded and matched" "weapon wielded dagger" "$(piece dagger '"\(.category) \(.state) \(.catalogueCode)"')"
expect "the renamed item keeps its count and label" "gear carried 1 knife" "$(piece dagger_2 '"\(.category) \(.state) \(.count) \(.type)"')"
expect "Arrows (ammo kind arrows) is arrow ammunition with the bow's dice" "ammunition arrow 1d6 12" \
    "$(piece arrows '"\(.category) \(.ammoKind) \(.damageSm) \(.count)"')"
expect "the Power Gem is worn, magic misc, a gem" "magic misc worn gem" "$(piece power_gem_dispel '"\(.category) \(.magicKind) \(.state) \(.component)"')"
expect "the Ruby is a gem (step 1.1)" "gem carried" "$(piece ruby '"\(.component) \(.state)"')"
expect "the other_melee weapons are unmatched" "weapon_unmatched weapon_unmatched" "$(items | jq -r '[.[] | select(.weaponType == "other_melee") | .review[]] | join(" ")')"
reviews() { sql "SELECT item_id || ':' || reason FROM equipment_review ORDER BY 1" | paste -sd' '; }
expect "the review list" "bastard_sword:weapon_unmatched dagger_2:uncategorised mace:weapon_unmatched" "$(reviews)"
new_names="$(sql "SELECT id || '=' || t.body FROM items i JOIN localized_texts t ON t.text_id = i.name ORDER BY 1")"
want_names="$(sed -e 's/^item:dagger=/item:dagger_2=/' -e 's/^item://' -e 's/^weapon://' <<<"$old_names" | LC_ALL=C sort)"
expect "every name intact, the renamed item's included" "$want_names" "$(LC_ALL=C sort <<<"$new_names")"

# --- 3. the old entries pop and redo through the one model ---
expect "pop the old item_remove" item_remove "$(undo)"
expect "the removed flasks are back, as gear" "gear 5" "$(piece fire_flask '"\(.category) \(.count)"')"
expect "pop the old item_use of the renamed item" item_use "$(undo)"
expect "the renamed item has its count back" 2 "$(piece dagger_2 .count)"
expect "pop the old weapon_edit" weapon_edit "$(undo)"
expect "the long sword is back at +2, still a wielded weapon piece" "2 weapon wielded" "$(piece longsword '"\(.magicToHit) \(.category) \(.state)"')"
expect "redo the weapon_edit" weapon_edit "$(redo)"
expect "the long sword is +3 again" 3 "$(piece longsword .magicToHit)"
expect "redo the item_use" item_use "$(redo)"
expect "the renamed item is used again" 1 "$(piece dagger_2 .count)"
expect "redo the item_remove" item_remove "$(redo)"
expect "the flasks are gone again" "" "$(piece fire_flask .id)"

# --- 4. two restarts: no second conversion, no duplicate review row, names reconciled only ---
state() {
    sql "SELECT id || ' ' || category || ' ' || state || ' ' || count || ' ' || component || ' ' || name FROM items ORDER BY id"
    reviews
    sql "SELECT COUNT(*) FROM item_id_renames"
    sql "SELECT COUNT(*) FROM localized_texts"
    sql "SELECT COUNT(*) FROM seeded_name_languages"
    sql "SELECT COUNT(*) FROM schema_migrations"
}
before="$(state)"
for n in 1 2; do
    stop
    start "$work/new-srv" "$new_testdata"
    expect "restart $n leaves pieces, reviews, renames, names and migrations as they were" "$before" "$(state)"
done

# --- 5. the converted bow and arrows attack as before; the stored form keeps the bow ---
expect "the stored form still holds the bow" shortbow "$(call GET /api/attack/form | jq -r .form.weaponId)"
expect "the bow is wielded" wielded "$(piece shortbow .state)"
new_attack="$(call POST /api/roll/attack '{"weapon_id":"shortbow","ammo_item_id":"arrows_plus1","target_ac":5}')"
# targetACNeeded and the rolls follow the d20: the comparison is of what the rules derive.
same='{baseThac0, effectiveThac0, damageDice, modifiers: [.modifiers[] | {kind, value}],
       damageTerms: [.damageTerms[] | {kind, value}], weaponId, ammo: .ammo.itemId}'
expect "same to-hit and dice as the old release" "$(jq -c "$same" "$work/old-attack.json")" "$(jq -c "$same" <<<"$new_attack")"

# --- 6. the attack form picks a carried weapon: one equipment_state line, undo empties the hand ---
call POST /api/items '{"name":{"en":"Hand Axe"},"category":"weapon","weaponKind":"melee","damageSm":"1d6","damageL":"1d4","behaviour":"stock","count":1}' >/dev/null
expect "a new weapon is carried" carried "$(piece hand_axe .state)"
entries() { call GET /api/log | jq '.entries | length'; }
n="$(entries)"
call PATCH /api/attack/form '{"weaponId":"hand_axe"}' >/dev/null
expect "picking it is one journal line" "$((n + 1)) equipment_state" "$(entries) $(last_kind)"
expect "picking it wields it" wielded "$(piece hand_axe .state)"
expect "undo the pick" equipment_state "$(undo)"
expect "the axe is put away" carried "$(piece hand_axe .state)"
expect "the hand is empty" "" "$(call GET /api/attack/form | jq -r '.form.weaponId // ""')"
expect "redo the pick" equipment_state "$(redo)"
expect "the axe is wielded again, in the hand" "wielded hand_axe" "$(piece hand_axe .state) $(call GET /api/attack/form | jq -r .form.weaponId)"

# --- 7. a second shield worn with swap: one journal line, undo restores both ---
shield='"category":"shield","shieldKind":"small","acMelee":1,"acMissile":1,"behaviour":"stock","count":1'
call POST /api/items "{\"name\":{\"en\":\"Small Shield\"},$shield}" >/dev/null
call POST /api/items/small_shield/state '{"state":"worn"}' >/dev/null
call POST /api/items "{\"name\":{\"en\":\"Spare Shield\"},$shield}" >/dev/null
n="$(entries)"
call POST /api/items/spare_shield/state '{"state":"worn","swap":true}' >/dev/null
expect "the swap is one journal line" "$((n + 1)) equipment_state" "$(entries) $(last_kind)"
expect "the spare is worn, the first put away" "worn carried" "$(piece spare_shield .state) $(piece small_shield .state)"
expect "undo the swap" equipment_state "$(undo)"
expect "both are back" "carried worn" "$(piece spare_shield .state) $(piece small_shield .state)"

# --- 8. Chromatic Orb spends the converted gem and a gem added with the component code ---
call POST /api/items '{"name":{"en":"Opal"},"category":"gear","component":"gem","behaviour":"stock","count":1}' >/dev/null
call PUT /api/spells/prepared '{"spells":["chromatic_orb","chromatic_orb"]}' >/dev/null
expect "the spell offers both gems, not the daily Power Gem" "opal ruby" \
    "$(call GET /api/spells/preparation | jq -r '[.spellbook[] | select(.id == "chromatic_orb") | .items[].id] | sort | join(" ")')"
# The cast is of a slot, by its index in GET /api/spells/slots: the first unused Chromatic Orb.
orb_slot() { call GET /api/spells/slots | jq '[to_entries[] | select(.value.prepared_spell_id == "chromatic_orb" and (.value.used | not)) | .key][0]'; }
cast() { call POST "/api/spells/$(orb_slot)/cast" "{\"targetAc\":5,\"itemId\":\"$1\"}"; }
cast ruby >/dev/null
expect "casting spends the converted Ruby" 1 "$(piece ruby .count)"
cast opal >/dev/null
expect "casting spends the new Opal" 0 "$(piece opal .count)"

stop
echo "equipment-rehearsal: PASS"
