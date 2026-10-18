#!/usr/bin/env bash
# Tests scripts/linear-configure.sh against a local stub HTTP server that keeps state (part of `just
# validate`); nothing here calls Linear and no real key is used.
#   - dry run lists the changes and writes nothing; a real run applies them; a second run reports
#     "already configured" and creates nothing;
#   - existing labels (any case) and workspace labels (Bug, Feature) are not recreated;
#   - project not visible, team missing, a mapped state missing: exit 1 before any write.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
server_pid=""
trap '[ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true; rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-linear-configure: FAIL: $*" >&2; fails=$((fails + 1)); }

cat > "$tmp/server.py" <<'PY'
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer
statefile, portfile = sys.argv[1:3]
class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        q, v = body["query"], body["variables"]
        st = json.load(open(statefile))
        st["calls"].append("write" if "mutation" in q else "read")
        if "issueLabelCreate" in q:
            st["labels"].append({"name": v["in"]["name"], "team": {"key": "TST"}})
            data = {"issueLabelCreate": {"success": True}}
        elif "projectUpdate" in q:
            ids = v["in"]["teamIds"]
            st["project"]["teams"]["nodes"] = [t for t in st["teams"] if t["id"] in ids]
            data = {"projectUpdate": {"success": True}}
        elif "issueLabels" in q:
            data = {"issueLabels": {"nodes": st["labels"]}}
        elif "projects" in q:
            data = {"projects": {"nodes": [st["project"]] if st["project"] else []}}
        elif "states" in q:
            data = {"team": {"states": {"nodes": [{"name": n} for n in st["states"]]}}}
        else:
            data = {"teams": {"nodes": [t for t in st["teams"] if t["key"] == "TST"]}}
        json.dump(st, open(statefile, "w"))
        self.send_response(200); self.send_header("Content-Type", "application/json"); self.end_headers()
        self.wfile.write(json.dumps({"data": data}).encode())
s = HTTPServer(("127.0.0.1", 0), H)
open(portfile, "w").write(str(s.server_address[1]))
s.serve_forever()
PY

cat > "$tmp/mapping.json" <<'JSON'
{"team": "TST", "project": "proj",
 "status": {"open": {"state": "Todo"}, "closed": {"state": "Done"}, "deferred": {"state": "Backlog", "label": "deferred"}},
 "type": {"task": null, "bug": "Bug", "chore": "chore"},
 "labels": ["epic", "deferred", "chore", "backend"]}
JSON

# reset <project-teams-json> <labels-json> <states-json>
reset() {
    jq -n --argjson pt "$1" --argjson labels "$2" --argjson states "$3" '{
        calls: [], labels: $labels, states: $states,
        teams: [{id: "t-tst", key: "TST", name: "tst"}, {id: "t-oth", key: "OTH", name: "oth"}],
        project: {id: "p1", name: "proj", teams: {nodes: $pt}}}' >"$tmp/state.json"
}
writes() { jq '[.calls[] | select(. == "write")] | length' "$tmp/state.json"; }
ws='[{"name":"Bug","team":null},{"name":"Feature","team":null}]'
st='["Todo","Done","Backlog","In Progress"]'
oth='[{"id":"t-oth","key":"OTH"}]'

echo '{}' >"$tmp/state.json"
reset "$oth" "$ws" "$st"
python3 -I "$tmp/server.py" "$tmp/state.json" "$tmp/port" &
server_pid=$!
for _ in $(seq 50); do [ -s "$tmp/port" ] && break; sleep 0.1; done
[ -s "$tmp/port" ] || { echo "test-linear-configure: stub server did not start" >&2; exit 1; }
export LINEAR_API_URL="http://127.0.0.1:$(cat "$tmp/port")/graphql"
export LINEAR_API_KEY="lin_api_stub-$$"
export HOME="$tmp/home"
export CRITICAL_PASSWORD_DIR="$HOME/.config/critical"
export LINEAR_MAPPING="$tmp/mapping.json"
mkdir -p "$HOME"
C="$root/scripts/linear-configure.sh"

# --- dry run: reports, writes nothing
"$C" --dry-run >"$tmp/out" 2>&1 || fail "dry run exited non-zero: $(cat "$tmp/out")"
grep -q 'would add team TST to project "proj"' "$tmp/out" || fail "dry run: project change not listed"
for l in epic deferred chore backend; do grep -q "would create label $l in team TST" "$tmp/out" || fail "dry run: label $l not listed"; done
grep -q 'dry run: 5 change(s) pending' "$tmp/out" || fail "dry run: summary wrong: $(cat "$tmp/out")"
[ "$(writes)" = 0 ] || fail "dry run wrote"

# --- real run, then again
"$C" >"$tmp/out" 2>&1 || fail "run exited non-zero: $(cat "$tmp/out")"
grep -q 'done: 5 change(s) applied' "$tmp/out" || fail "run: summary wrong: $(cat "$tmp/out")"
jq -e '.project.teams.nodes | map(.key) | sort == ["OTH","TST"]' "$tmp/state.json" >/dev/null || fail "project did not gain TST (and keep OTH)"
jq -e '[.labels[].name] | sort == ["Bug","Feature","backend","chore","deferred","epic"]' "$tmp/state.json" >/dev/null || fail "labels wrong after run"
before="$(writes)"
"$C" >"$tmp/out" 2>&1 || fail "second run exited non-zero"
grep -q 'already configured' "$tmp/out" || fail "second run: not 'already configured': $(cat "$tmp/out")"
[ "$(writes)" = "$before" ] || fail "second run wrote"
"$C" --dry-run | grep -q 'already configured' || fail "dry run after run: not 'already configured'"

# --- existing labels in another case and workspace labels are not recreated
reset '[{"id":"t-tst","key":"TST"}]' '[{"name":"Bug","team":null},{"name":"Feature","team":null},{"name":"Epic","team":{"key":"TST"}},{"name":"chore","team":null},{"name":"other-team","team":{"key":"OTH"}}]' "$st"
"$C" >"$tmp/out" 2>&1 || fail "partial run exited non-zero"
grep -q 'created label epic' "$tmp/out" && fail "Epic recreated"
grep -q 'created label chore' "$tmp/out" && fail "workspace chore recreated"
grep -q 'created label deferred' "$tmp/out" || fail "deferred not created"
grep -q 'created label backend' "$tmp/out" || fail "backend not created"
grep -q 'project' "$tmp/out" && fail "project touched though already in the team"

# --- preconditions: exit 1 before any write
reset "$oth" "$ws" "$st"; jq '.project = null' "$tmp/state.json" >"$tmp/s2" && mv "$tmp/s2" "$tmp/state.json"
rc=0; "$C" >"$tmp/out" 2>&1 || rc=$?
[ "$rc" = 1 ] || fail "missing project exited $rc"
grep -q 'project "proj" not visible' "$tmp/out" || fail "missing project not named"
[ "$(writes)" = 0 ] || fail "missing project: wrote"

reset "$oth" "$ws" '["Todo","Done"]'
rc=0; "$C" >"$tmp/out" 2>&1 || rc=$?
[ "$rc" = 1 ] || fail "missing state exited $rc"
grep -q 'lacks the state(s) Backlog' "$tmp/out" || fail "missing state not named: $(cat "$tmp/out")"
[ "$(writes)" = 0 ] || fail "missing state: wrote"

reset "$oth" "$ws" "$st"
jq '.teams = []' "$tmp/state.json" >"$tmp/s2" && mv "$tmp/s2" "$tmp/state.json"
rc=0; "$C" >"$tmp/out" 2>&1 || rc=$?
[ "$rc" = 1 ] && grep -q 'team TST not found' "$tmp/out" || fail "missing team not reported"

"$C" --bogus >/dev/null 2>&1 && fail "bad option exited 0"
LINEAR_API_KEY= "$C" >"$tmp/out" 2>&1 && fail "missing key exited 0"

[ "$fails" = 0 ] || { echo "test-linear-configure: $fails failure(s)" >&2; exit 1; }
echo "test-linear-configure: ok"
