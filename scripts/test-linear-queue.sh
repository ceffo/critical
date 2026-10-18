#!/usr/bin/env bash
# Tests scripts/linear-ready.sh and .claude/scripts/pending-questions.sh against a
# local stub HTTP server (part of `just validate`); nothing here calls Linear and no real key is used.
# Fixture ids (TST-n) are made up.
#   - ready queue: blocked by an open item -> out; blocked only by Done/Canceled -> in; needs-info, deferred,
#     epic and non-Todo-page items out; order priority then age (None last); pagination; parent; --json;
#   - pending questions: lines, Questions section, missing section warning, ledger section, exit 3 / exit 0
#     silent; a missing key or an API error is exit 1 with a message, never 3.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
server_pid=""
trap '[ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true; rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-linear-queue: FAIL: $*" >&2; fails=$((fails + 1)); }

cat > "$tmp/server.py" <<'PY'
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer
fixtures, portfile = sys.argv[1:3]
class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        fx = json.load(open(fixtures))
        if fx.get("fail"):
            code, out = 500, {"errors": [{"message": "boom"}]}
        elif "inverseRelations" in body["query"]:
            pages = fx["ready"]
            idx = 0 if not body["variables"].get("after") else int(body["variables"]["after"])
            nodes = pages[idx]
            more = idx + 1 < len(pages)
            code, out = 200, {"data": {"issues": {"nodes": nodes,
                "pageInfo": {"hasNextPage": more, "endCursor": str(idx + 1) if more else None}}}}
        else:
            code, out = 200, {"data": {"issues": {"nodes": fx["questions"]}}}
        self.send_response(code); self.send_header("Content-Type", "application/json"); self.end_headers()
        self.wfile.write(json.dumps(out).encode())
s = HTTPServer(("127.0.0.1", 0), H)
open(portfile, "w").write(str(s.server_address[1]))
s.serve_forever()
PY

# issue(id, priority, created, labels, blockers[(type, state)], parent)
cat > "$tmp/gen.py" <<'PY'
import json, sys
def issue(i, prio, created, labels=(), blockers=(), parent=None):
    return {"identifier": i, "title": "title " + i, "priority": prio, "createdAt": created,
            "parent": {"identifier": parent} if parent else None,
            "labels": {"nodes": [{"name": l} for l in labels]},
            "inverseRelations": {"nodes": [{"type": t, "issue": {"identifier": "TST-0", "state": {"type": s}}}
                                           for t, s in blockers]}}
page1 = [
    issue("TST-1", 3, "2026-01-01"),                                   # medium, old
    issue("TST-2", 0, "2026-01-02"),                                   # none: last
    issue("TST-3", 2, "2026-01-05", parent="TST-9"),                   # high, newer
    issue("TST-4", 2, "2026-01-03"),                                   # high, older: before TST-3
    issue("TST-5", 1, "2026-01-09", blockers=[("blocks", "started")]), # blocked by an open item: out
    issue("TST-6", 1, "2026-01-08", blockers=[("blocks", "completed"), ("blocks", "canceled")]),  # urgent: first
]
page2 = [
    issue("TST-7", 1, "2026-01-01", labels=["needs-info"]),
    issue("TST-8", 1, "2026-01-01", labels=["deferred"]),
    issue("TST-9", 1, "2026-01-01", labels=["epic"]),
    issue("TST-10", 3, "2026-01-02", blockers=[("blocks", "unstarted"), ("blocks", "completed")]),  # one open: out
    issue("TST-11", 3, "2026-01-04", blockers=[("related", "started")]),  # a plain relation does not block
]
questions = [
    {"identifier": "TST-20", "title": "needs an answer", "description":
        "intro\n\n## Questions\n1. Which colour? Default: blue.\n2. How many?\n\n## Other\nnot this"},
    {"identifier": "TST-21", "title": "no section", "description": None},
]
json.dump({"ready": [page1, page2], "questions": questions}, open(sys.argv[1], "w"))
json.dump({"ready": [[]], "questions": []}, open(sys.argv[2], "w"))
json.dump({"fail": True}, open(sys.argv[3], "w"))
PY
python3 -I "$tmp/gen.py" "$tmp/full.json" "$tmp/empty.json" "$tmp/fail.json"
cp "$tmp/full.json" "$tmp/fixtures.json"
python3 -I "$tmp/server.py" "$tmp/fixtures.json" "$tmp/port" &
server_pid=$!
for _ in $(seq 50); do [ -s "$tmp/port" ] && break; sleep 0.1; done
[ -s "$tmp/port" ] || { echo "test-linear-queue: stub server did not start" >&2; exit 1; }
export LINEAR_API_URL="http://127.0.0.1:$(cat "$tmp/port")/graphql"
export LINEAR_API_KEY="lin_api_stub-$$"
export HOME="$tmp/home"
export CRITICAL_PASSWORD_DIR="$HOME/.config/critical"
mkdir -p "$HOME"
R="$root/scripts/linear-ready.sh"
P="$root/.claude/scripts/pending-questions.sh"

# --- ready queue
"$R" >"$tmp/out" 2>"$tmp/err" || fail "ready exited non-zero: $(cat "$tmp/err")"
got="$(cut -f1 "$tmp/out" | paste -sd' ')"
[ "$got" = "TST-6 TST-4 TST-3 TST-1 TST-11 TST-2" ] || fail "ready order/selection wrong: $got"
grep -q $'^TST-3\tHigh\ttitle TST-3\tTST-9$' "$tmp/out" || fail "ready line lacks priority/title/parent: $(cat "$tmp/out")"
grep -q $'^TST-2\tNone\t' "$tmp/out" || fail "priority None not named"
"$R" --json | jq -e 'map(.id) == ["TST-6","TST-4","TST-3","TST-1","TST-11","TST-2"] and .[2].parent == "TST-9"' >/dev/null \
    || fail "--json output wrong"
"$R" --bogus >/dev/null 2>&1 && fail "bad option exited 0"

cp "$tmp/empty.json" "$tmp/fixtures.json"
"$R" >"$tmp/out" 2>&1 || fail "empty queue exited non-zero"
[ ! -s "$tmp/out" ] || fail "empty queue printed something"
[ "$("$R" --json)" = "[]" ] || fail "empty --json is not []"

cp "$tmp/fail.json" "$tmp/fixtures.json"
"$R" >"$tmp/out" 2>"$tmp/err" && fail "API error exited 0"
grep -q 'HTTP 500' "$tmp/err" || fail "API error not named: $(cat "$tmp/err")"
LINEAR_API_KEY= "$R" >"$tmp/out" 2>"$tmp/err" && fail "missing key exited 0"
grep -q 'no API key' "$tmp/err" || fail "missing key not reported"

# --- pending questions
export CRITICAL_LEDGER="$tmp/ledger.md"
printf '# Ledger\n\n## Pending questions to user\n1. ledger question one\nnot numbered\n2. ledger two\n\n## Other\n3. not a question\n' > "$CRITICAL_LEDGER"
cp "$tmp/full.json" "$tmp/fixtures.json"
rc=0; "$P" >"$tmp/out" 2>"$tmp/err" || rc=$?
[ "$rc" = 3 ] || fail "pending with questions exited $rc, want 3"
grep -q '^1\. \[TST-20\] needs an answer$' "$tmp/out" || fail "item line missing: $(cat "$tmp/out")"
grep -q 'Which colour? Default: blue.' "$tmp/out" || fail "Questions section not extracted"
grep -q 'not this' "$tmp/out" && fail "Questions section ran into the next section"
grep -q '^2\. \[TST-21\] no section$' "$tmp/out" || fail "second item missing"
grep -q 'no Questions section' "$tmp/out" || fail "missing-section warning absent"
grep -q '^3\. \[ledger\] 1\. ledger question one$' "$tmp/out" || fail "ledger line missing"
grep -q '^4\. \[ledger\] 2\. ledger two$' "$tmp/out" || fail "second ledger line missing"
grep -q 'not a question' "$tmp/out" && fail "ledger read past its section"

cp "$tmp/empty.json" "$tmp/fixtures.json"
rm "$CRITICAL_LEDGER"
rc=0; "$P" >"$tmp/out" 2>"$tmp/err" || rc=$?
[ "$rc" = 0 ] || fail "no questions exited $rc, want 0"
[ ! -s "$tmp/out" ] || fail "no questions printed something"

cp "$tmp/fail.json" "$tmp/fixtures.json"
rc=0; "$P" >"$tmp/out" 2>"$tmp/err" || rc=$?
{ [ "$rc" != 0 ] && [ "$rc" != 3 ]; } || fail "API error exited $rc, want non-zero and not 3"
grep -q 'HTTP 500' "$tmp/err" || fail "API error not reported by pending-questions"
rc=0; LINEAR_API_KEY= "$P" >"$tmp/out" 2>"$tmp/err" || rc=$?
{ [ "$rc" != 0 ] && [ "$rc" != 3 ]; } || fail "missing key exited $rc, want non-zero and not 3"
grep -q 'no API key' "$tmp/err" || fail "missing key not reported by pending-questions"

[ "$fails" = 0 ] || { echo "test-linear-queue: $fails failure(s)" >&2; exit 1; }
echo "test-linear-queue: ok"
