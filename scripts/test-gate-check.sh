#!/usr/bin/env bash
# Tests scripts/gate-check.sh offline (part of `just validate`, ADR 0024 step 8.3): against a stub server
# (scripts/gate-stub-server.py) that answers like an app behind the login gate, the check passes; each rule
# the stub breaks (the dev login, the dev route, an open catalogue, an open POST, the app served to a
# visitor) makes it fail. A server that does not answer fails after GATE_CHECK_WAIT.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
pid=""
trap 'kill $pid 2>/dev/null || true; rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-gate-check: FAIL: $*" >&2; fails=$((fails + 1)); }

serve() { # <mode>: starts the stub, sets $url
    kill $pid 2>/dev/null || true
    python3 -I "$here/gate-stub-server.py" "$1" >"$tmp/port" &
    pid=$!
    for _ in $(seq 1 50); do [ -s "$tmp/port" ] && break; sleep 0.1; done
    url="http://127.0.0.1:$(<"$tmp/port")"
}

serve gated
out="$("$here/gate-check.sh" "$url" 2>&1)" || fail "a gated app was refused: $out"
[[ "$out" == *"gate-check: OK"* ]] || fail "a gated app: no OK line: $out"

for mode in devlogin devroute opencatalogue openpost openapp; do
    serve "$mode"
    rc=0; out="$("$here/gate-check.sh" "$url" 2>&1)" || rc=$?
    [ "$rc" = 1 ] || fail "$mode: want exit 1 (a server that answers without the gate), got $rc: $out"
    [[ "$out" == *"gate-check: FAIL"* ]] || fail "$mode: no FAIL line: $out"
done

kill $pid 2>/dev/null || true
rc=0; out="$(GATE_CHECK_WAIT=2 "$here/gate-check.sh" "$url" 2>&1)" || rc=$?
[ "$rc" = 3 ] || fail "a dead server must exit 3 (serves nothing), got $rc: $out"
if "$here/gate-check.sh" 'https://x/../y; rm' >/dev/null 2>&1; then fail "a bad url was accepted"; fi

[ "$fails" = 0 ] || exit 1
echo "test-gate-check: ok"
