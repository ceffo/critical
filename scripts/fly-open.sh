#!/usr/bin/env bash
# Opens a private fly app, or a fly PostgreSQL cluster, on this machine through `fly proxy`, on a local
# port that cannot collide with local development (docs/deploy.md, "Reaching a private app"): `just dev`
# holds 8080 (API), 5173 (web) and 5432 (PostgreSQL) by default, so a fixed port would either fail to
# bind or silently reach the local stack instead of fly.
#
#   scripts/fly-open.sh <app> [local port]          # the app: prints the URL to open in the browser
#   scripts/fly-open.sh --db <cluster> [local port] # a cluster: prints the psql connection string
#
# No port: a free one. A given port that is already taken is refused. (The apps are public since the login
# went live, docs/deploy.md; this is for reaching an app behind its public ingress, or a cluster.) The proxy goes to the
# `.flycast` address (fly starts a stopped machine, balances between machines; the app answers on its
# service port 80, the cluster on 5432), binds 127.0.0.1 only, and runs until Ctrl-C. For an app it
# first waits (up to 90 s, a cold machine or database) until /api/health answers 200 and prints the
# commit. Read-only; prints no data, no password.
set -euo pipefail

usage="usage: fly-open.sh <app> [local port] | --db <cluster> [local port]"
kind=app
if [ "${1:-}" = --db ]; then kind=db; shift; fi
target="${1:?$usage}"
port="${2:-}"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"

taken() { [ -n "$(ss -H -ltn "sport = :$1")" ]; }
if [ -z "$port" ]; then
    port="$(python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
elif taken "$port"; then
    echo "fly-open: local port $port is already in use (just dev? \`just status\`): pick another or omit it" >&2
    exit 2
fi
if [ -n "${FLY_OPEN_PORT_ONLY:-}" ]; then echo "$port"; exit 0; fi # for the test: resolve the port, no proxy
remote=80
[ "$kind" = db ] && remote=5432

log="$(mktemp)"
"$fly" proxy "$port:$remote" "$target.flycast" -a "$target" -q >"$log" 2>&1 &
proxy=$!
trap 'kill "$proxy" 2>/dev/null || true; wait "$proxy" 2>/dev/null || true; rm -f "$log"' EXIT
trap 'exit 0' INT TERM
fail() { echo "fly-open: FAIL ($target): $*" >&2; exit 1; }

# Wait until the proxy listens (and, for an app, the app answers).
base="http://127.0.0.1:$port"
ready=""
for _ in $(seq 1 90); do
    kill -0 "$proxy" 2>/dev/null || { cat "$log" >&2; fail "fly proxy exited"; }
    if [ "$kind" = app ]; then
        health="$(curl -fsS -m 70 "$base/api/health" 2>/dev/null || true)"
        [[ "$health" == *'"status":"ok"'* ]] && { ready=1; break; }
    elif taken "$port"; then
        ready=1
        break
    fi
    sleep 1
done
[ -n "$ready" ] || fail "not reachable after 90 s on local port $port"

if [ "$kind" = app ]; then
    commit="$(sed -n 's/.*"commit":"\([^"]*\)".*/\1/p' <<<"$health")"
    echo "fly-open: $target (commit ${commit:-unknown}) is open at $base/"
else
    echo "fly-open: $target is open: psql \"host=127.0.0.1 port=$port user=<user> dbname=<database> sslmode=disable\""
fi
echo "fly-open: proxy running on 127.0.0.1:$port, Ctrl-C to stop it"
wait "$proxy" || { cat "$log" >&2; fail "fly proxy exited"; }
