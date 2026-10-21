#!/usr/bin/env bash
# Tests scripts/fly-brevo-secrets.sh against a fake Brevo API and a fake fly (part of `just validate`): an unknown
# sender is created and nothing is set; an unverified one sets nothing; a verified one sets the three secrets on
# stdin only; a refused key stops. No network, no real fly.
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/fly-brevo-secrets.sh"
tmp="$(mktemp -d)"
holder=""
trap '[ -z "$holder" ] || kill "$holder" 2>/dev/null || true; rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-fly-brevo-secrets: FAIL: $*" >&2; fails=$((fails + 1)); }

cat >"$tmp/fly" <<'FLY'
#!/usr/bin/env bash
if [ "$1 $2" = "machine list" ]; then echo '[{"id":"m"}]'; exit 0; fi
echo "$*" >"$FAKE_DIR/fly.args"
cat >"$FAKE_DIR/fly.stdin"
FLY
chmod +x "$tmp/fly"

cat >"$tmp/brevo.py" <<'PY'
import json, os, sys
from http.server import BaseHTTPRequestHandler, HTTPServer
state = os.environ["FAKE_DIR"]
class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def send(self, code, body):
        b = json.dumps(body).encode()
        self.send_response(code); self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(b))); self.end_headers(); self.wfile.write(b)
    def do_GET(self):
        if self.headers.get("api-key") != "goodkey": return self.send(401, {"message": "Key not found"})
        self.send(200, {"senders": json.load(open(state + "/senders.json"))})
    def do_POST(self):
        n = int(self.headers.get("content-length", 0)); body = json.loads(self.rfile.read(n))
        open(state + "/created.json", "w").write(json.dumps(body)); self.send(201, {"id": 1})
s = HTTPServer(("127.0.0.1", 0), H)
open(state + "/port", "w").write(str(s.server_port))
s.serve_forever()
PY
FAKE_DIR="$tmp" python3 -I "$tmp/brevo.py" &
holder=$!
for _ in $(seq 1 50); do [ -s "$tmp/port" ] && break; sleep 0.1; done
export FAKE_DIR="$tmp" FLY="$tmp/fly" BREVO_API_URL="http://127.0.0.1:$(cat "$tmp/port")"

run() { rm -f "$tmp/fly.args" "$tmp/fly.stdin" "$tmp/created.json"; rc=0; BREVO_API_KEY="${KEY:-goodkey}" "$script" critical-hit-staging hit@example.com "Critical Hit" >"$tmp/out" 2>&1 || rc=$?; }

echo '[]' >"$tmp/senders.json"
run; [ "$rc" = 3 ] || fail "unknown sender should exit 3, got $rc"
[ -f "$tmp/created.json" ] && [ ! -f "$tmp/fly.stdin" ] || fail "unknown sender should be created and nothing set"

echo '[{"email":"Hit@Example.com","active":false}]' >"$tmp/senders.json"
run; [ "$rc" = 3 ] || fail "unverified sender should exit 3, got $rc"
[ ! -f "$tmp/created.json" ] && [ ! -f "$tmp/fly.stdin" ] || fail "unverified sender should neither be created nor set secrets"

echo '[{"email":"hit@example.com","active":true}]' >"$tmp/senders.json"
run; [ "$rc" = 0 ] || fail "verified sender should exit 0, got $rc: $(cat "$tmp/out")"
grep -qx 'CRITICAL_BREVO_API_KEY=goodkey' "$tmp/fly.stdin" && grep -qx 'CRITICAL_MAIL_FROM=hit@example.com' "$tmp/fly.stdin" \
    && grep -qx 'CRITICAL_MAIL_FROM_NAME=Critical Hit' "$tmp/fly.stdin" || fail "the three secrets should reach fly on stdin"
grep -q goodkey "$tmp/fly.args" "$tmp/out" && fail "the key leaked to the fly arguments or the output"

KEY=badkey run; [ "$rc" = 1 ] && [ ! -f "$tmp/fly.stdin" ] || fail "a refused key should exit 1 and set nothing, got $rc"

[ "$fails" = 0 ] || exit 1
echo "test-fly-brevo-secrets: ok"
