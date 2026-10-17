#!/usr/bin/env bash
# Smoke-checks a deployed private fly app (ADR 0024 steps 3.3 and 10.5): opens `fly proxy` to the
# app's Flycast address (fly's proxy starts a stopped machine), then, on every machine of the app
# (each request pinned with fly's `fly-force-instance-id` header), checks that /api/health answers
# 200 with status ok and the expected commit, that /api/character answers 200 (a read through the
# database: the health check does not touch it) and that / serves the web app. A machine whose
# server is still waiting for a cold database is waited for (up to 90 s).
#
#   scripts/fly-smoke.sh <app> <commit> [version]
#
# With a version, /api/health must also report it (a staging release); without one only the commit is
# checked (the promotion, and images built before versions were baked in).
#
# Read-only on the app and its data; prints status codes and the health body only, never data. The
# proxy runs on a free local port and is stopped on exit.
set -euo pipefail

app="${1:?usage: fly-smoke.sh <app> <commit> [version]}"
commit="${2:?usage: fly-smoke.sh <app> <commit> [version]}"
version="${3:-}"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"

port="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
body="$(mktemp)"
log="$(mktemp)"
"$fly" proxy "$port:80" "$app.flycast" -a "$app" -q >"$log" 2>&1 &
proxy=$!
trap 'kill "$proxy" 2>/dev/null || true; rm -f "$body" "$log"' EXIT
fail() { echo "fly-smoke: FAIL ($app): $*" >&2; exit 1; }

base="http://127.0.0.1:$port"
machines="$("$fly" machine list -a "$app" --json | jq -r '.[].id')"
[ -n "$machines" ] || fail "no machine"

# get <machine> <path>: GET through the proxy pinned to one machine, body in $body, prints the code.
get() { curl -sS -m 70 -o "$body" -w '%{http_code}' -H "fly-force-instance-id: $1" "$base$2" 2>/dev/null || true; }

for m in $machines; do
    start=$SECONDS
    if [ "$m" = "$(echo "$machines" | head -n1)" ]; then
        # The pin must be honoured, or "every machine serves" proves nothing: wait until the app
        # answers at all, then a machine id that does not exist must not get a 200.
        while [ $((SECONDS - start)) -lt 90 ] && [ "$(curl -sS -m 70 -o /dev/null -w '%{http_code}' "$base/api/health" 2>/dev/null || true)" != 200 ]; do sleep 1; done
        [ "$(get 0000000000dead /api/health)" != 200 ] || fail "fly-force-instance-id is ignored: cannot pin a machine"
    fi
    code=000
    while [ $((SECONDS - start)) -lt 90 ]; do
        kill -0 "$proxy" 2>/dev/null || { cat "$log" >&2; fail "fly proxy exited"; }
        code="$(get "$m" /api/health)"
        [ "$code" = 200 ] && break
        sleep 1
    done
    [ "$code" = 200 ] || fail "machine $m /api/health: HTTP $code"
    grep -q '"status":"ok"' "$body" || fail "machine $m /api/health status: $(cat "$body")"
    grep -q "\"commit\":\"$commit\"" "$body" || fail "machine $m /api/health does not report commit $commit: $(cat "$body")"
    [ -z "$version" ] || grep -q "\"version\":\"$version\"" "$body" \
        || fail "machine $m /api/health does not report version $version: $(cat "$body")"
    echo "fly-smoke: machine $m /api/health 200 $(cat "$body") after $((SECONDS - start)) s"

    start=$SECONDS
    code="$(get "$m" /api/character)"
    [ "$code" = 200 ] || fail "machine $m /api/character: HTTP $code"
    echo "fly-smoke: machine $m /api/character 200 (database read) in $((SECONDS - start)) s"

    code="$(get "$m" /)"
    [ "$code" = 200 ] || fail "machine $m /: HTTP $code"
    grep -q '<div id="root">' "$body" || fail "machine $m / is not the web app"
done
echo "fly-smoke: OK ($app, commit $commit${version:+, version $version}, machines: $(echo $machines | tr '\n' ' '))"
