#!/usr/bin/env bash
# The snapshot codec upgrade rehearsal (ADR 0018 steps 8 and 9, migration 019_snapshot_envelopes.sql,
# ADR 0024 step 5.2) on COPIES of a real database: a read-only pg_dump of the source is restored into
# databases of a throwaway LOCAL compose PostgreSQL, and the release before the codec (old ref) and
# this checkout's server run on them, side by side where a rolling deploy has them side by side.
#
#   scripts/snapshot-codec-rehearsal.sh [source container] [source database] [old ref]
#       default: critical-postgres-1 critical <merge base of HEAD and origin/main>
#
# 1. Rolling deploy, one database. The old release migrates the copy; it creates a weapon piece with a
#    French name, and an entry recorded before the equipment model (a "weapons:<id>" snapshot whose
#    name is a plain string, the old weapon service's encoding) is planted on top. This checkout
#    starts on the same database (019 and the re-encoding data step), the old one still running.
#    - The pre-model weapon entry: undo keeps the piece's French name and puts back the English one
#      the snapshot holds; redo likewise.
#    - The old machine logs an action (Next Round): it succeeds, its snapshots have no envelopes.
#      On the new machine GET /api/log answers (null before/after), undo and redo of that entry are
#      refused (409 snapshot_not_reencoded) and change nothing; the old machine undoes it.
#    - The follow-up release is simulated: this checkout plus a migration making the envelopes
#      NOT NULL with the data step run again just before it (what the later release does). It
#      re-encodes that row, and its redo then replays the entry.
#    - Undo and redo of an initiative roll (the redo stamps the initiative with its entry), and of a
#      use spending an item's last charge (gone after; back with 1 charge on undo).
# 2. Data step failure: on a second copy migrated by the old release, a snapshot no restorer reads
#    is planted. This checkout's start fails, the run rolls back (019 not recorded, no envelope
#    column) and the old machine keeps serving (GET /api/log, Next Round).
# 3. Whole stacks: two more copies, one served by the old release, one by this checkout. Every
#    character's log is popped to the bottom, then replayed to the top; the game state (every table
#    but the log's own, timestamps left out) is identical on both at the start, at the bottom and
#    at the end.
#
# The source is only read (pg_dump). Nothing else touches it: the copies live in their own compose
# project (criticalsnap<pid>) on a free port with its own volume, removed at the end whatever the
# outcome. Real seeds (seeds/) and catalogue (.catalogue/catalogue.json, `just catalogue`). Exit 0 only
# when every check passes. Needs docker, psql, pg_restore, jq, curl, git and go.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
fail() { echo "snapshot-rehearsal: FAIL: $*" >&2; exit 1; }
ok() { echo "snapshot-rehearsal: ok: $*"; }

source_container="${1:-critical-postgres-1}"
source_db="${2:-critical}"
old_ref="${3:-$(git -C "$root" merge-base HEAD origin/main)}"
envelopes="019_snapshot_envelopes.sql"
git -C "$root" cat-file -e "$old_ref:api/internal/storage/postgres/migrations/$envelopes" 2>/dev/null &&
    fail "$old_ref already holds $envelopes: pass a ref of before it"
catalogue="$root/.catalogue/catalogue.json"
[ -f "$catalogue" ] || fail "no $catalogue: run just catalogue"

free_port() { python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }
work="$(mktemp -d)"
project="criticalsnap$$"
pg_port="$(free_port)"
export COMPOSE_PROJECT_NAME="$project" CRITICAL_PG_PORT="$pg_port"
export PGPASSWORD=critical # the compose database's own throwaway account (docker-compose.yml)
started=""
declare -A pid=() port=()

cleanup() {
    status=$?
    set +e
    for name in "${!pid[@]}"; do
        kill "${pid[$name]}" 2>/dev/null && wait "${pid[$name]}" 2>/dev/null
    done
    if [ -n "$started" ]; then
        (cd "$root" && docker compose down --volumes --remove-orphans >/dev/null 2>&1)
        echo "snapshot-rehearsal: compose project $project removed"
    fi
    if [ "$status" -ne 0 ]; then
        for log in "$work"/*.log; do [ -f "$log" ] && { echo "--- $log" >&2; tail -n 20 "$log" >&2; }; done
    fi
    rm -rf "$work"
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

# --- the three servers ---
# old: the release before the codec. new: this checkout. next: this checkout plus the follow-up
# migration (envelopes NOT NULL, the data step run again just before it), as the later release has it.
mkdir -p "$work/old" "$work/next"
git -C "$root" archive "$old_ref" api seeds | tar -x -C "$work/old"
(cd "$work/old/api" && go build -o "$work/old-srv" ./cmd/api) || fail "could not build the old release ($old_ref)"
(cd "$root/api" && go build -o "$work/new-srv" ./cmd/api) || fail "could not build this checkout"
cp -r "$root/api" "$work/next/api"
follow_up="020_rehearsal_envelopes_required.sql"
printf '%s\n' "ALTER TABLE action_snapshots ALTER COLUMN before_envelope SET NOT NULL;" \
    "ALTER TABLE action_snapshots ALTER COLUMN after_envelope SET NOT NULL;" \
    >"$work/next/api/internal/storage/postgres/migrations/$follow_up"
steps="$work/next/api/internal/storage/postgres/migrations.go"
awk -v line="		\"$follow_up\": reencodeSnapshots," '{ print } /^\t\townerKeysMigration: func/ { print line }' \
    "$steps" >"$steps.new" && mv "$steps.new" "$steps"
grep -q "\"$follow_up\": reencodeSnapshots" "$steps" || fail "could not register the follow-up's data step"
(cd "$work/next/api" && go build -o "$work/next-srv" ./cmd/api) || fail "could not build the follow-up"
ok "built the old release ($old_ref), this checkout and the follow-up"

# --- the copies ---
docker exec "$source_container" pg_dump -U critical -Fc "$source_db" >"$work/source.dump" ||
    fail "could not dump $source_db from $source_container"
cd "$root"
started=1
docker compose up -d postgres >/dev/null 2>&1 || fail "could not start the scratch PostgreSQL (docker running?)"
admin="host=127.0.0.1 port=$pg_port user=critical dbname=critical"
for _ in $(seq 1 60); do psql "$admin" -X -q -c 'SELECT 1' >/dev/null 2>&1 && break; sleep 1; done
for copy in rolling failure stack_old stack_new; do
    psql "$admin" -X -q -c "CREATE DATABASE $copy" >/dev/null || fail "the scratch PostgreSQL did not accept connections"
    pg_restore --no-owner -d "postgresql://critical:critical@127.0.0.1:$pg_port/$copy" "$work/source.dump" ||
        fail "could not restore the copy $copy"
done
ok "four copies of $source_db restored"

sql() { psql "host=127.0.0.1 port=$pg_port user=critical dbname=$1" -X -q -t -A -v ON_ERROR_STOP=1 -c "$2"; }

# serve <name> <binary> <database>: starts a server and logs in with the dev login.
serve() {
    local name=$1 bin=$2 dbname=$3
    port[$name]="$(free_port)"
    CRITICAL_DSN="postgres://critical:critical@127.0.0.1:$pg_port/$dbname?sslmode=disable" \
        CRITICAL_SEEDS_DIR="$4" CRITICAL_CATALOGUE="$catalogue" CRITICAL_ENV=dev \
        CRITICAL_ADDR="127.0.0.1:${port[$name]}" "$bin" >>"$work/$name.log" 2>&1 &
    pid[$name]=$!
    for _ in $(seq 1 600); do
        curl -fsS "http://127.0.0.1:${port[$name]}/api/health" >/dev/null 2>&1 && break
        kill -0 "${pid[$name]}" 2>/dev/null || return 1
        sleep 0.1
    done
    curl -fsS "http://127.0.0.1:${port[$name]}/api/health" >/dev/null || fail "$name did not answer"
    curl -fsS -c "$work/$name.jar" -X POST "http://127.0.0.1:${port[$name]}/api/auth/dev" \
        -H 'Content-Type: application/json' -d '{}' >/dev/null || fail "$name: dev login refused"
}
old() { serve "$1" "$work/old-srv" "$2" "$work/old/seeds" || fail "$1 (old release) exited at start"; }
new() { serve "$1" "$work/new-srv" "$2" "$root/seeds" || fail "$1 (this checkout) exited at start"; }
halt() {
    kill "${pid[$1]}" 2>/dev/null && wait "${pid[$1]}" 2>/dev/null || true
    unset "pid[$1]"
}
# call <server> <method> <path> [body]: the answer and its status on the last line.
call() {
    curl -sS -b "$work/$1.jar" -X "$2" "http://127.0.0.1:${port[$1]}$3" -H 'Content-Type: application/json' \
        ${4:+-d "$4"} -w '\n%{http_code}'
}
status() { local out; out="$(call "$@")"; printf '%s' "${out##*$'\n'}"; }
body() { local out; out="$(call "$@")"; [[ "${out##*$'\n'}" == 2* ]] || fail "$1 $2 $3 answered: $out"; printf '%s' "${out%$'\n'*}"; }
expect() { [ "$2" = "$3" ] || fail "$1: want '$2', got '$3'"; ok "$1"; }
top() { body "$1" GET /api/log | jq -r '.topId // empty'; }
redo_id() { body "$1" GET /api/log | jq -r '.redo.id // empty'; }
undo() { call "$1" POST /api/log/undo "{\"expectedTopId\":$(top "$1")}"; }
redo() { call "$1" POST /api/log/redo "{\"expectedRedoId\":$(redo_id "$1")}"; }
answer() { printf '%s %s' "${1##*$'\n'}" "$(jq -r '.code // .kind // empty' <<<"${1%$'\n'*}")"; }
round() { body "$1" GET /api/round | jq -c '[.round, .initiative]'; }

# ============ 1. rolling deploy on one database ============
old A rolling
ok "the old release migrated the copy to $(sql rolling 'SELECT max(name) FROM schema_migrations')"
# item_id <server> <English name>: the id of the piece of that name.
item_id() { body "$1" GET /api/items | jq -r --arg n "$2" '[.[] | select(.name.en == $n) | .id] | first // empty'; }
body A POST /api/items '{"name":{"en":"Rehearsal Sword","fr":"Épée de répétition"},"category":"weapon","weaponKind":"melee","damageSm":"1d8","damageL":"1d12","behaviour":"stock","count":1}' >/dev/null
sword="$(item_id A "Rehearsal Sword")"
[ -n "$sword" ] || fail "the weapon piece was not created"
char="$(sql rolling 'SELECT character_id FROM action_log ORDER BY id DESC LIMIT 1')"
legacy() {
    printf '{"id":"%s","position":1,"name":"%s","damageSm":"1d8","damageL":"1d12","magicToHit":%s,"magicDamage":%s,"kind":"melee","twoHanded":false,"ammoKind":"","weaponType":"long_sword"}' \
        "$sword" "$1" "$2" "$2"
}
weapon_entry="$(sql rolling "INSERT INTO action_log (character_id, kind, summary, payload) VALUES ($char, 'weapon_edit', '', '{}') RETURNING id" | head -1)"
sql rolling "UPDATE action_log SET transaction_id = id WHERE id = $weapon_entry" >/dev/null
sql rolling "INSERT INTO action_snapshots (action_id, state_key, before_state, after_state) VALUES ($weapon_entry, 'weapons:$sword', '$(legacy 'Rehearsal Sword +1' 1)', '$(legacy 'Rehearsal Sword +2' 2)')" >/dev/null
ok "planted a pre-model weapon entry #$weapon_entry on $sword (character $char)"

new B rolling
expect "019 recorded once" 1 "$(sql rolling "SELECT count(*) FROM schema_migrations WHERE name = '$envelopes'")"
expect "every snapshot re-encoded" 0 "$(sql rolling 'SELECT count(*) FROM action_snapshots WHERE before_envelope IS NULL OR after_envelope IS NULL')"
expect "the plain-string name became a partial one" '{"en":"Rehearsal Sword +1"} true' \
    "$(sql rolling "SELECT before_envelope FROM action_snapshots WHERE action_id = $weapon_entry" | jq -c '.state.name, .state.namePartial' | tr '\n' ' ' | sed 's/ $//')"

# The pre-model weapon entry, on the new machine.
piece_name() { body "$1" GET /api/items | jq -c --arg id "$sword" '.[] | select(.id == $id) | .name'; }
expect "undo of the pre-model weapon entry" "200 weapon_edit" "$(answer "$(undo B)")"
expect "the French name is kept, the English one is the snapshot's" '{"en":"Rehearsal Sword +1","fr":"Épée de répétition"}' "$(piece_name B)"
expect "redo of the pre-model weapon entry" "200 weapon_edit" "$(answer "$(redo B)")"
expect "the French name is still kept" '{"en":"Rehearsal Sword +2","fr":"Épée de répétition"}' "$(piece_name B)"

# The old machine logs an action during the deploy.
expect "the old machine logs Next Round" 200 "$(status A POST /api/round/advance)"
advanced="$(round A)"
pending="$(sql rolling 'SELECT string_agg(DISTINCT action_id::text, $$,$$) FROM action_snapshots WHERE before_envelope IS NULL')"
[ -n "$pending" ] || fail "the old machine's snapshots have envelopes"
ok "its snapshots have no envelopes (entries $pending)"
log="$(body B GET /api/log)"
expect "GET /api/log on the new machine shows them with null before/after" true \
    "$(jq --arg ids "$pending" '[.entries[] | select((.id | tostring) as $i | $ids | split(",") | index($i)) | .snapshots[] | .before == null and .after == null] | (length > 0 and all)' <<<"$log")"
expect "the new machine refuses to undo it" "409 snapshot_not_reencoded" "$(answer "$(undo B)")"
expect "nothing changed" "$advanced" "$(round B)"
expect "the old machine undoes its own entry" "200 round_advance" "$(answer "$(undo A)")"
expect "the new machine refuses to redo it" "409 snapshot_not_reencoded" "$(answer "$(redo B)")"
popped="$(round B)"
[ "$popped" != "$advanced" ] || fail "the old machine's undo changed nothing"
expect "GET /api/log still answers" 200 "$(status B GET /api/log)"
halt A
halt B

# The follow-up release re-encodes the row, then replays the entry.
serve C "$work/next-srv" rolling "$root/seeds" || fail "the follow-up exited at start"
expect "the follow-up re-encoded the old machine's rows" 0 "$(sql rolling 'SELECT count(*) FROM action_snapshots WHERE before_envelope IS NULL OR after_envelope IS NULL')"
expect "the envelopes are required" "NO NO" "$(sql rolling "SELECT string_agg(is_nullable, ' ') FROM information_schema.columns WHERE table_name = 'action_snapshots' AND column_name LIKE '%_envelope'")"
expect "its redo replays the entry" "200 round_advance" "$(answer "$(redo C)")"
expect "the state is the old machine's Next Round" "$advanced" "$(round C)"

# An initiative roll: undo, then redo stamps the initiative with its entry.
body C POST /api/round/advance >/dev/null
rolled="$(round C)"
initiative_entry="$(top C)"
expect "the initiative names its entry" "$initiative_entry" "$(jq '.[1].logEntryId' <<<"$rolled")"
expect "undo of Next Round" "200 round_advance" "$(answer "$(undo C)")"
expect "the previous round and initiative are back" "$advanced" "$(round C)"
expect "redo of Next Round" "200 round_advance" "$(answer "$(redo C)")"
expect "the same initiative, stamped with its entry" "$rolled" "$(round C)"

# A use spending an item's last charge.
body C POST /api/items '{"name":{"en":"Rehearsal Wand","fr":"Baguette de répétition"},"category":"gear","behaviour":"charges","count":1,"maxCount":7}' >/dev/null
wand="$(item_id C "Rehearsal Wand")"
[ -n "$wand" ] || fail "the wand was not created"
count() { body C GET /api/items | jq -r --arg id "$wand" '[.[] | select(.id == $id) | .count] | first // "gone"'; }
expect "the use spends the last charge" true "$(body C POST "/api/items/$wand/use" | jq .removed)"
expect "the wand is gone" gone "$(count)"
expect "undo of the use" "200 item_use" "$(answer "$(undo C)")"
expect "the wand is back with 1 charge" 1 "$(count)"
expect "redo of the use" "200 item_use" "$(answer "$(redo C)")"
expect "the wand is gone again" gone "$(count)"
halt C

# ============ 2. a data step failure ============
old F failure
sql failure "INSERT INTO action_snapshots (action_id, state_key, before_state, after_state) VALUES ((SELECT max(id) FROM action_log), 'hit_points', '{\"hp\":3}', '{\"hp\":2}')" >/dev/null
last="$(sql failure 'SELECT max(name) FROM schema_migrations')"
if serve G "$work/new-srv" failure "$root/seeds"; then fail "this checkout started on a snapshot it cannot read"; fi
unset "pid[G]"
# The log quotes the key (\"hit_points\").
refusal='before_state: state key \"hit_points\": no restorer reads this family'
grep -qF "$refusal" "$work/G.log" || fail "the start failed for another reason"
ok "this checkout's start fails on the snapshot: $(grep -o 'migrate: after [^ ]* action_snapshots [0-9]*' "$work/G.log" | head -1) $refusal"
expect "the run rolled back: 019 not recorded" "$last" "$(sql failure 'SELECT max(name) FROM schema_migrations')"
expect "no envelope column" 0 "$(sql failure "SELECT count(*) FROM information_schema.columns WHERE table_name = 'action_snapshots' AND column_name LIKE '%_envelope'")"
expect "the old machine still lists the log" 200 "$(status F GET /api/log)"
expect "the old machine still logs Next Round" 200 "$(status F POST /api/round/advance)"
halt F

# ============ 3. whole stacks, old release against this checkout ============
# The game state: every table but the log's own and the sessions, timestamps left out, sorted.
state() {
    local tables t
    tables="$(sql "$1" "SELECT string_agg(table_name, ' ' ORDER BY table_name) FROM information_schema.tables WHERE table_schema = 'public' AND table_name NOT IN ('action_log', 'action_snapshots', 'action_corrections', 'schema_migrations', 'sessions')")"
    for t in $tables; do
        sql "$1" "SELECT '$t ' || (to_jsonb(x) - 'created_at' - 'updated_at' - 'last_seen_at' - 'last_login' - 'expires_at')::text FROM $t x" | sort
    done
}
same() {
    state stack_old >"$work/old.$1"
    state stack_new >"$work/new.$1"
    diff -q "$work/old.$1" "$work/new.$1" >/dev/null || { diff "$work/old.$1" "$work/new.$1" | head -20 >&2; fail "the state differs $1"; }
    ok "the state is identical $1 ($(wc -l <"$work/old.$1") rows)"
}
old O stack_old
new N stack_new
same "at the start"
characters="$(sql stack_old 'SELECT string_agg(id::text, $$ $$ ORDER BY id) FROM characters')"
for c in $characters; do
    for s in O N; do body "$s" PUT /api/characters/current "{\"id\":$c}" >/dev/null; done
    pops=0
    while [ -n "$(top O)" ]; do
        expect_old="$(answer "$(undo O)")" expect_new="$(answer "$(undo N)")"
        [ "$expect_old" = "$expect_new" ] && [[ "$expect_old" == 200* ]] || fail "character $c pop $pops: old $expect_old, new $expect_new"
        pops=$((pops + 1))
    done
    [ -z "$(top N)" ] || fail "character $c: this checkout has entries left to pop"
    ok "character $c: $pops transactions popped on both"
done
same "at the bottom"
for c in $characters; do
    for s in O N; do body "$s" PUT /api/characters/current "{\"id\":$c}" >/dev/null; done
    redos=0
    while [ -n "$(redo_id O)" ]; do
        expect_old="$(answer "$(redo O)")" expect_new="$(answer "$(redo N)")"
        [ "$expect_old" = "$expect_new" ] && [[ "$expect_old" == 200* ]] || fail "character $c redo $redos: old $expect_old, new $expect_new"
        redos=$((redos + 1))
    done
    [ -z "$(redo_id N)" ] || fail "character $c: this checkout has entries left to redo"
    ok "character $c: $redos transactions replayed on both"
done
same "at the end"
halt O
halt N
echo "snapshot-rehearsal: every check passed"
