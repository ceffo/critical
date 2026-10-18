#!/usr/bin/env bash
# Tests scripts/linear.sh and scripts/linear-setup.sh against a local stub HTTP server (part of `just validate`);
# nothing here calls Linear and no real key is used:
#   - linear.sh: success (query and variables arrive, the key in Authorization), key from the key file,
#     HTTP error, GraphQL error, missing key, unreachable endpoint; the key never on stdout/stderr;
#   - linear-setup.sh: a wrong key is reported and nothing is stored; a good key is stored (file 600, folder 700),
#     the rc block is added once, GitHub confirmed through the API or by the user's answer; a rerun with
#     everything in place asks nothing and changes nothing; the key is never printed.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
server_pid=""
trap '[ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true; rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-linear: FAIL: $*" >&2; fails=$((fails + 1)); }
good="lin_api_good-$$-sentinel"
bad="lin_api_bad-$$-sentinel"

# Stub server: Authorization == $good answers by query content; any other key gets HTTP 401.
cat > "$tmp/server.py" <<'PY'
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer
good, log, portfile = sys.argv[1:4]
class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        with open(log, "a") as f:
            f.write(json.dumps({"auth": self.headers.get("Authorization"), "body": body}) + "\n")
        q = body["query"]
        if self.headers.get("Authorization") != good:
            code, out = 401, {"errors": [{"message": "Authentication required"}]}
        elif "httperror" in q:
            code, out = 500, {"errors": [{"message": "boom"}]}
        elif "graphqlerror" in q:
            code, out = 200, {"data": None, "errors": [{"message": "Field nope"}]}
        elif "notjson" in q:
            self.send_response(200); self.end_headers(); self.wfile.write(b"<html>"); return
        elif "integrations" in q:
            if open(portfile + ".github").read().strip() == "yes":
                code, out = 200, {"data": {"integrations": {"nodes": [{"service": "github"}]}}}
            else:
                code, out = 200, {"data": {"integrations": {"nodes": []}}}
        elif "viewer" in q:
            code, out = 200, {"data": {"viewer": {"name": "Ada Test", "email": "ada@example.com"},
                                       "teams": {"nodes": [{"key": "CRI", "name": "Critical"}]}}}
        else:
            code, out = 200, {"data": {"echo": body["variables"]}}
        self.send_response(code); self.send_header("Content-Type", "application/json"); self.end_headers()
        self.wfile.write(json.dumps(out).encode())
s = HTTPServer(("127.0.0.1", 0), H)
open(portfile, "w").write(str(s.server_address[1]))
s.serve_forever()
PY
echo no > "$tmp/port.github"
python3 -I "$tmp/server.py" "$good" "$tmp/requests" "$tmp/port" &
server_pid=$!
for _ in $(seq 50); do [ -s "$tmp/port" ] && break; sleep 0.1; done
[ -s "$tmp/port" ] || { echo "test-linear: stub server did not start" >&2; exit 1; }
export LINEAR_API_URL="http://127.0.0.1:$(cat "$tmp/port")/graphql"
export LINEAR_SETUP_NO_OPEN=1
export HOME="$tmp/home"
mkdir -p "$HOME"
export CRITICAL_PASSWORD_DIR="$HOME/.config/critical"
unset LINEAR_API_KEY
L="$root/scripts/linear.sh"

leaks() { grep -qF "$good" "$1" "${@:2}" 2>/dev/null || grep -qF "$bad" "$1" "${@:2}" 2>/dev/null; }

# --- linear.sh
LINEAR_API_KEY=$good "$L" 'query { echo }' '{"id":"x"}' >"$tmp/out" 2>"$tmp/err" || fail "success exited non-zero"
[ "$(jq -c .data.echo "$tmp/out")" = '{"id":"x"}' ] || fail "variables did not arrive: $(cat "$tmp/out")"
[ "$(tail -n1 "$tmp/requests" | jq -r .auth)" = "$good" ] || fail "key not sent in Authorization as is"
leaks "$tmp/out" "$tmp/err" && fail "success: key printed"

echo 'query { echo }' | LINEAR_API_KEY=$good "$L" >"$tmp/out" 2>&1 || fail "query on stdin failed"

LINEAR_API_KEY=$good "$L" 'query { httperror }' >"$tmp/out" 2>"$tmp/err" && fail "HTTP error exited 0"
grep -q 'HTTP 500' "$tmp/err" || fail "HTTP error not named: $(cat "$tmp/err")"

LINEAR_API_KEY=$bad "$L" 'query { viewer }' >"$tmp/out" 2>"$tmp/err" && fail "wrong key exited 0"
grep -q 'HTTP 401: Authentication required' "$tmp/err" || fail "wrong key not reported: $(cat "$tmp/err")"
leaks "$tmp/out" "$tmp/err" && fail "wrong key: key printed"

LINEAR_API_KEY=$good "$L" 'query { graphqlerror }' >"$tmp/out" 2>"$tmp/err" && fail "GraphQL error exited 0"
grep -q 'GraphQL error: Field nope' "$tmp/err" || fail "GraphQL error not named: $(cat "$tmp/err")"
[ "$(jq -r '.errors[0].message' "$tmp/out")" = "Field nope" ] || fail "GraphQL error response not on stdout"

LINEAR_API_KEY=$good "$L" 'query { notjson }' >"$tmp/out" 2>"$tmp/err" && fail "non-JSON exited 0"
grep -q 'not JSON' "$tmp/err" || fail "non-JSON not reported"

"$L" 'query { echo }' >"$tmp/out" 2>"$tmp/err" && fail "missing key exited 0"
grep -q 'no API key' "$tmp/err" || fail "missing key not reported: $(cat "$tmp/err")"

LINEAR_API_KEY=$good "$L" 'query { echo }' '{not json' >/dev/null 2>"$tmp/err" && fail "bad variables exited 0"
grep -q 'not valid JSON' "$tmp/err" || fail "bad variables not reported"

LINEAR_API_KEY=$good LINEAR_API_URL=http://127.0.0.1:1/graphql "$L" 'query { echo }' >/dev/null 2>"$tmp/err" && fail "unreachable exited 0"
grep -q 'cannot reach Linear' "$tmp/err" || fail "unreachable not reported"

mkdir -p "$CRITICAL_PASSWORD_DIR"
printf '%s\n' "$good" > "$CRITICAL_PASSWORD_DIR/linear-api-key"
"$L" 'query { echo }' '{"k":1}' >"$tmp/out" 2>&1 || fail "key from file failed: $(cat "$tmp/out")"
rm -rf "$CRITICAL_PASSWORD_DIR"

# --- linear-setup.sh
S="$root/scripts/linear-setup.sh"
touch "$HOME/.bashrc"
keyfile="$CRITICAL_PASSWORD_DIR/linear-api-key"

printf '%s\n' "$bad" | "$S" >"$tmp/out" 2>&1 && fail "setup with a wrong key exited 0"
grep -q 'refused that key' "$tmp/out" || fail "wrong key not reported by setup: $(cat "$tmp/out")"
[ ! -e "$keyfile" ] || fail "wrong key was stored"
[ ! -s "$HOME/.bashrc" ] || fail "wrong key touched .bashrc"
leaks "$tmp/out" && fail "setup printed the wrong key"

# good key, GitHub not connected, user answers n: key stored, exit 2
printf '%s\nn\n' "$good" | "$S" >"$tmp/out" 2>&1 && fail "setup without GitHub exited 0"
grep -q 'Ada Test' "$tmp/out" || fail "viewer name not printed"
grep -q 'team CRI: Critical' "$tmp/out" || fail "team CRI not printed"
grep -q 'NOT confirmed' "$tmp/out" || fail "unconfirmed GitHub not reported"
leaks "$tmp/out" && fail "setup printed the key"
[ "$(cat "$keyfile")" = "$good" ] || fail "key not stored"
[ "$(stat -c %a "$keyfile")" = 600 ] || fail "key file mode is $(stat -c %a "$keyfile")"
[ "$(stat -c %a "$CRITICAL_PASSWORD_DIR")" = 700 ] || fail "key folder mode is not 700"
[ "$(grep -c 'LINEAR_API_KEY' "$HOME/.bashrc")" = 1 ] || fail ".bashrc block missing or duplicated"
grep -qF "$good" "$HOME/.bashrc" && fail "key written into .bashrc"
# a new shell sees the key and the helper's viewer query succeeds from it
env -i HOME="$HOME" PATH="$PATH" LINEAR_API_URL="$LINEAR_API_URL" bash -c \
    'source "$HOME/.bashrc"; [ -n "$LINEAR_API_KEY" ] && '"$L"' "query { viewer { name } }"' >"$tmp/out" 2>&1 \
    || fail "a new shell does not get a working key: $(cat "$tmp/out")"

# GitHub answered y: confirmed by the user
printf 'y\n' | "$S" >"$tmp/out" 2>&1 || fail "setup with answer y failed: $(cat "$tmp/out")"
grep -q 'confirmed by you' "$tmp/out" || fail "answer y not taken"

# GitHub connected in Linear: a rerun asks nothing, changes nothing
echo yes > "$tmp/port.github"
before="$(cat "$keyfile" "$HOME/.bashrc"; stat -c '%a %Y' "$keyfile" "$HOME/.bashrc")"
"$S" </dev/null >"$tmp/out" 2>&1 || fail "rerun failed: $(cat "$tmp/out")"
grep -q 'GitHub integration: connected' "$tmp/out" || fail "rerun did not see GitHub"
after="$(cat "$keyfile" "$HOME/.bashrc"; stat -c '%a %Y' "$keyfile" "$HOME/.bashrc")"
[ "$before" = "$after" ] || fail "rerun changed the key file or .bashrc"
leaks "$tmp/out" && fail "rerun printed the key"

# a key set in the environment is checked, not stored
rm -rf "$CRITICAL_PASSWORD_DIR"
LINEAR_API_KEY=$good "$S" </dev/null >"$tmp/out" 2>&1 || fail "setup with env key failed: $(cat "$tmp/out")"
[ ! -e "$keyfile" ] || fail "env key was stored"
LINEAR_API_KEY=$bad "$S" </dev/null >"$tmp/out" 2>&1 && fail "setup with a wrong env key exited 0"
[ ! -e "$keyfile" ] || fail "wrong env key was stored"

[ "$fails" = 0 ] || { echo "test-linear: $fails failure(s)" >&2; exit 1; }
echo "test-linear: ok"
