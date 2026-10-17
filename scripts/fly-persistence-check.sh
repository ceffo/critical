#!/usr/bin/env bash
# Shows that a staging app keeps its data across a machine restart and a redeploy (its data lives in
# PostgreSQL, the app has no volume: ADR 0024 step 5.1).
#
#   scripts/fly-persistence-check.sh <app> mark          # rolls 1d20: one new action-log entry, prints its id
#   scripts/fly-persistence-check.sh <app> check <id>    # fails unless the action log still holds entry <id>
#
# Run `mark`, then restart (`fly machine restart`) or redeploy the app, then `check`. It writes to the
# app's data, so it refuses any app whose name does not end in -staging. Prints ids and codes only.
set -euo pipefail

usage="usage: fly-persistence-check.sh <app> mark | check <entry id>"
app="${1:?$usage}"
mode="${2:?$usage}"
case "$app" in *-staging) ;; *) echo "fly-persistence-check: refuses $app: staging apps only" >&2; exit 2 ;; esac
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"

port="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
body="$(mktemp)"
log="$(mktemp)"
"$fly" proxy "$port:8080" "$app.flycast" -a "$app" -q >"$log" 2>&1 &
proxy=$!
trap 'kill "$proxy" 2>/dev/null || true; rm -f "$body" "$log"' EXIT
fail() { echo "fly-persistence-check: FAIL ($app): $*" >&2; exit 1; }
base="http://127.0.0.1:$port"

code=000
for _ in $(seq 1 90); do
    kill -0 "$proxy" 2>/dev/null || { cat "$log" >&2; fail "fly proxy exited"; }
    code="$(curl -sS -m 70 -o /dev/null -w '%{http_code}' "$base/api/health" 2>/dev/null || true)"
    [ "$code" = 200 ] && break
    sleep 1
done
[ "$code" = 200 ] || fail "/api/health: HTTP $code"

case "$mode" in
mark)
    code="$(curl -sS -m 70 -o "$body" -w '%{http_code}' -H 'Content-Type: application/json' \
        -d '{"formula":"1d20"}' "$base/api/roll/dice")"
    [ "$code" = 200 ] || fail "POST /api/roll/dice: HTTP $code"
    id="$(jq -r '.logEntryId' "$body")"
    [ "$id" -gt 0 ] 2>/dev/null || fail "no log entry id in the roll"
    echo "fly-persistence-check: marked: action-log entry $id"
    ;;
check)
    id="${3:?$usage}"
    code="$(curl -sS -m 70 -o "$body" -w '%{http_code}' "$base/api/log")"
    [ "$code" = 200 ] || fail "GET /api/log: HTTP $code"
    jq -e --argjson id "$id" 'any(.entries[]; .id == $id)' "$body" >/dev/null || fail "action-log entry $id is gone"
    echo "fly-persistence-check: OK: action-log entry $id is still there"
    ;;
*) echo "$usage" >&2; exit 2 ;;
esac
