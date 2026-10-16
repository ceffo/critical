#!/usr/bin/env bash
# Smoke-checks a deployed private fly app (ADR 0024 steps 3.3 and 10.5): opens `fly proxy` to the
# app's Flycast address (fly's proxy starts a stopped machine), then checks that /api/health answers
# 200 with status ok and the expected commit, and that / serves the web app.
#
#   scripts/fly-smoke.sh <app> <commit>
#
# Read-only on the app. The proxy runs on a free local port and is stopped on exit.
set -euo pipefail

app="${1:?usage: fly-smoke.sh <app> <commit>}"
commit="${2:?usage: fly-smoke.sh <app> <commit>}"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"

port="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
body="$(mktemp)"
log="$(mktemp)"
"$fly" proxy "$port:8080" "$app.flycast" -a "$app" -q >"$log" 2>&1 &
proxy=$!
trap 'kill "$proxy" 2>/dev/null || true; rm -f "$body" "$log"' EXIT
fail() { echo "fly-smoke: FAIL ($app): $*" >&2; exit 1; }

base="http://127.0.0.1:$port"
code=000
for _ in $(seq 1 60); do
    kill -0 "$proxy" 2>/dev/null || { cat "$log" >&2; fail "fly proxy exited"; }
    code="$(curl -sS -m 10 -o "$body" -w '%{http_code}' "$base/api/health" 2>/dev/null || true)"
    [ "$code" = 200 ] && break
    sleep 1
done
[ "$code" = 200 ] || fail "/api/health: HTTP $code"
grep -q '"status":"ok"' "$body" || fail "/api/health status: $(cat "$body")"
grep -q "\"commit\":\"$commit\"" "$body" || fail "/api/health does not report commit $commit: $(cat "$body")"
echo "fly-smoke: /api/health 200 $(cat "$body")"

code="$(curl -sS -m 10 -o "$body" -w '%{http_code}' "$base/")"
[ "$code" = 200 ] || fail "/: HTTP $code"
grep -q '<div id="root">' "$body" || fail "/ is not the web app"
echo "fly-smoke: OK ($app, commit $commit)"
