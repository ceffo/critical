#!/usr/bin/env bash
# Tests .claude/scripts/{linear-state,ship,land,linear-cutover}.sh (part of `just validate`) against a stub Linear
# server and stub gh: nothing here calls Linear or GitHub. Checks: ship moves each issue to In Review and comments
# the PR link and never closes (no Done/Canceled state sent); the unmerged path puts an issue back to Todo; an
# unknown issue or state fails; a Linear failure leaves the PR open and exits 1; land.sh merges without a tracker
# call only when CI is green, the PR is CLEAN, Greptile has reviewed the head and every Greptile thread has a reply;
# the cutover swaps the wrappers and removes the beads tooling.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
S="$root/.claude/scripts"
tmp="$(mktemp -d)"
server_pid=""
trap '[ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true; rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-ship-linear: FAIL: $*" >&2; fails=$((fails + 1)); }

cat > "$tmp/server.py" <<'PY'
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer
log, portfile = sys.argv[1:3]
STATES = [{"id": "s-todo", "name": "Todo"}, {"id": "s-prog", "name": "In Progress"},
          {"id": "s-rev", "name": "In Review"}, {"id": "s-done", "name": "Done"}]
class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        open(log, "a").write(json.dumps(body) + "\n")
        q, v = body["query"], body["variables"]
        if "issue(id" in q:
            if v["id"] == "TST-404":
                out = {"data": {"issue": None}}
            else:
                out = {"data": {"issue": {"id": "uuid-" + v["id"], "identifier": v["id"], "team": {"states": {"nodes": STATES}}}}}
        elif "issueUpdate" in q:
            out = {"data": {"issueUpdate": {"success": v["id"] != "uuid-TST-500"}}}
        elif "commentCreate" in q:
            out = {"data": {"commentCreate": {"success": True}}}
        else:
            out = {"errors": [{"message": "unexpected"}]}
        self.send_response(200); self.send_header("Content-Type", "application/json"); self.end_headers()
        self.wfile.write(json.dumps(out).encode())
s = HTTPServer(("127.0.0.1", 0), H)
open(portfile, "w").write(str(s.server_address[1]))
s.serve_forever()
PY
python3 -I "$tmp/server.py" "$tmp/requests" "$tmp/port" &
server_pid=$!
for _ in $(seq 50); do [ -s "$tmp/port" ] && break; sleep 0.1; done
[ -s "$tmp/port" ] || { echo "test-ship-linear: stub server did not start" >&2; exit 1; }
export LINEAR_API_URL="http://127.0.0.1:$(cat "$tmp/port")/graphql" LINEAR_API_KEY=lin_api_stub
reqs() { wc -l <"$tmp/requests" 2>/dev/null || echo 0; }
sent_states() { jq -r 'select(.query|test("issueUpdate")) | .variables.s' "$tmp/requests" | paste -sd' '; }

# --- linear-state.sh
out="$("$S/linear-state.sh" TST-12 "In Review" "PR https://x/pull/9")" || fail "linear-state failed"
[ "$out" = "TST-12 -> In Review" ] || fail "linear-state output: $out"
[ "$(sent_states)" = "s-rev" ] || fail "state not In Review: $(sent_states)"
jq -e 'select(.query|test("commentCreate")) | select(.variables.b=="PR https://x/pull/9" and .variables.id=="uuid-TST-12")' "$tmp/requests" >/dev/null || fail "comment not sent"
"$S/linear-state.sh" TST-404 Todo 2>"$tmp/err" && fail "unknown issue accepted"; grep -q 'no such issue' "$tmp/err" || fail "unknown issue message"
"$S/linear-state.sh" TST-12 Nope 2>"$tmp/err" && fail "unknown state accepted"; grep -q 'no state' "$tmp/err" || fail "unknown state message"
: > "$tmp/requests"
"$S/linear-state.sh" TST-12 Todo "PR #9 closed unmerged" >/dev/null || fail "back to Todo failed"
[ "$(sent_states)" = "s-todo" ] || fail "unmerged path did not send Todo"

# --- ship.sh with a real local remote and a stub gh
git init -q --bare "$tmp/remote.git"
git init -q -b feat "$tmp/wt"
git -C "$tmp/wt" -c user.name=t -c user.email=t@t commit -q --allow-empty -m init
git -C "$tmp/wt" remote add origin "$tmp/remote.git"
mkdir -p "$tmp/bin" "$tmp/main"
cat > "$tmp/bin/gh" <<'GH'
#!/usr/bin/env bash
echo "gh $*" >> "$GH_LOG"
# land.sh's gate reads: pr view --json (PR state and checks), the Greptile check run, the review comments.
case "$*" in
  *statusCheckRollup*) printf '{"state":"OPEN","mergeable":"MERGEABLE","mergeStateStatus":"%s","headRefOid":"abc1234","statusCheckRollup":[{"name":"validate","status":"COMPLETED","conclusion":"%s"}]}\n' \
    "${STUB_MERGE_STATE:-CLEAN}" "${STUB_CHECK:-SUCCESS}"; exit 0 ;;
  *check-runs*) echo "${STUB_GREPTILE:-completed}"; exit 0 ;;
  *pulls/77/comments*) echo "${STUB_COMMENTS:-[]}"; exit 0 ;;
esac
case "$1 $2" in
  "pr create") echo "https://github.com/o/r/pull/77" ;;
  "pr view") echo MERGEABLE ;;
  "pr merge") [ -z "${STUB_MERGE_FAIL:-}" ] || exit 1 ;;
esac
GH
chmod +x "$tmp/bin/gh"
export GH_LOG="$tmp/gh.log" PATH="$tmp/bin:$PATH" CRITICAL_MAIN="$tmp/main" SHIP_SETTLE=0
git -C "$tmp/main" init -q -b main; git -C "$tmp/main" remote add origin "$tmp/remote.git"
echo body > "$tmp/body"
: > "$tmp/requests"
out="$("$S/ship.sh" "$tmp/wt" "feat: x" "$tmp/body" TST-1 TST-2 2>"$tmp/err")" || fail "ship failed: $(cat "$tmp/err")"
[ "$out" = "PR 77 MERGEABLE" ] || fail "ship output: $out"
[ "$(sent_states)" = "s-rev s-rev" ] || fail "ship states: $(sent_states)"
[ "$(jq -r 'select(.query|test("commentCreate")) | .variables.b' "$tmp/requests" | sort -u)" = "PR https://github.com/o/r/pull/77" ] || fail "PR link comment"
grep -q 'Done\|s-done' "$tmp/requests" && fail "ship closed an issue"
git -C "$tmp/remote.git" rev-parse -q --verify refs/heads/feat >/dev/null || fail "branch not pushed"
: > "$tmp/requests"
out="$("$S/ship.sh" "$tmp/wt" "feat: x" "$tmp/body" TST-404 TST-3 2>"$tmp/err")" && fail "ship with a bad issue exited 0"
[ "$out" = "PR 77 MERGEABLE" ] || fail "PR line missing after a Linear failure: $out"
grep -q 'TST-404 not moved' "$tmp/err" || fail "bad issue not reported"
[ "$(sent_states)" = "s-rev" ] || fail "the good issue was skipped after the bad one"

# --- land.sh: merges only through its gate, no Linear call
: > "$tmp/requests"; : > "$GH_LOG"
git -C "$tmp/main" fetch -q origin; git -C "$tmp/main" checkout -q -B main origin/feat
git -C "$tmp/main" push -q origin main
"$S/land.sh" 77 >/dev/null 2>&1 || fail "land failed"
grep -q 'pr merge 77 .*--squash --delete-branch --match-head-commit abc1234' "$GH_LOG" || fail "land did not merge"
[ ! -s "$tmp/requests" ] || fail "land called Linear"
refused() { # $1 = case name, env set by the caller; land must exit 2, print REFUSED and not merge
  : > "$GH_LOG"
  out="$("$S/land.sh" 77 2>&1)"; rc=$?
  [ "$rc" = 2 ] || fail "land $1: exit $rc, want 2"
  case "$out" in REFUSED*) ;; *) fail "land $1: no REFUSED line: $out" ;; esac
  ! grep -q 'pr merge' "$GH_LOG" || fail "land $1: merged anyway"
}
set +e
STUB_CHECK=FAILURE refused "red check"
STUB_MERGE_STATE=BLOCKED refused "not clean"
STUB_GREPTILE=in_progress refused "greptile pending"
STUB_COMMENTS='[{"id":1,"in_reply_to_id":null,"user":{"login":"greptile-apps[bot]"}}]' refused "unanswered greptile thread"
: > "$GH_LOG"; STUB_MERGE_FAIL=1 "$S/land.sh" 77 >/dev/null 2>&1; [ $? = 1 ] || fail "land: a refused merge did not exit 1"
! grep -q "stack merge" "$GH_LOG" || fail "land fell back to an unchecked stack merge"
set -e
STUB_COMMENTS='[{"id":1,"in_reply_to_id":null,"user":{"login":"greptile-apps[bot]"}},{"id":2,"in_reply_to_id":1,"user":{"login":"ceffo"}}]' \
  "$S/land.sh" 77 >/dev/null 2>&1 || fail "land refused an answered greptile thread"

# --- cutover
sup="$tmp/sup"; mkdir -p "$sup"
touch "$sup/merge.sh" "$sup/sync-branch.sh" "$sup/beads-line.py"
mkdir -p "$tmp/main/.claude"; cp -r "$S" "$tmp/main/.claude/"
SUPERVISOR_DIR=$sup "$S/linear-cutover.sh" >/dev/null || fail "cutover failed"
SUPERVISOR_DIR=$sup "$S/linear-cutover.sh" >/dev/null || fail "cutover not idempotent"
for f in merge.sh sync-branch.sh beads-line.py; do [ ! -e "$sup/$f" ] || fail "$f still there"; done
grep -q "$tmp/main/.claude/scripts/ship.sh" "$sup/ship.sh" || fail "ship wrapper"
[ -x "$sup/pr-merge-watch.sh" ] || fail "watch wrapper"

[ "$fails" = 0 ] || { echo "test-ship-linear: $fails failure(s)" >&2; exit 1; }
echo "test-ship-linear: ok"
