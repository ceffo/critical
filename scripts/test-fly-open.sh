#!/usr/bin/env bash
# Tests the local port scripts/fly-open.sh resolves (part of `just validate`): the login port of the
# staging and prod apps by default, an explicit port wins, a taken port is refused, a database or an
# unknown app gets a free port (the defaults are read without the taken check: a tunnel may hold them). Resolution only: no proxy, no fly.
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/fly-open.sh"
fails=0
fail() { echo "test-fly-open: FAIL: $*" >&2; fails=$((fails + 1)); }
resolve() { FLY_OPEN_PORT_ONLY=1 "$script" "$@" 2>&1; }
taken() { [ -n "$(ss -H -ltn "sport = :$1")" ]; }

for pair in critical-staging:8091 critical-prod:8092; do
    app="${pair%:*}" want="${pair#*:}"
    [ "$(FLY_OPEN_PORT_ONLY=default "$script" "$app")" = "$want" ] || fail "$app without a port should default to $want"
    [ "$(resolve "$app" 18123)" = 18123 ] || fail "$app with an explicit port should keep it"
done

[ "$(resolve some-other-app 18124)" = 18124 ] || fail "explicit port on another app"
free="$(resolve some-other-app)"
[[ "$free" =~ ^[0-9]+$ ]] && [ "$free" != 8091 ] && [ "$free" != 8092 ] || fail "unknown app should get a free port, got: $free"
free="$(resolve --db tournesol-db)"
[[ "$free" =~ ^[0-9]+$ ]] && [ "$free" != 8091 ] || fail "a database should get a free port, got: $free"

# A taken explicit port is refused (exit 2).
python3 -I -c 'import socket,time; s=socket.socket(); s.bind(("127.0.0.1",18125)); s.listen(); time.sleep(10)' &
holder=$!
trap 'kill $holder 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do taken 18125 && break; sleep 0.1; done
rc=0
FLY_OPEN_PORT_ONLY=1 "$script" critical-staging 18125 >/dev/null 2>&1 || rc=$?
[ "$rc" = 2 ] || fail "taken explicit port should exit 2, got $rc"

[ "$fails" = 0 ] || exit 1
echo "test-fly-open: ok"
