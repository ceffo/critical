#!/usr/bin/env bash
# Tests scripts/linear-import.sh against a local stub Linear server that keeps state (part of `just validate`);
# nothing here calls Linear and no real key is used. A fixture beads export covers: a live epic, a closed epic
# (imported as Done), a child with comments and notes, a blocked feature, a bug whose parent is closed history
# (listed as text), a deferred item, a closed task and a tombstone (not imported). Checks:
#   - dry run: lists every issue, relation and comment, writes nothing (no mutation, no id map);
#   - run: creates what the mapping says (state, priority, labels, dates, parent, blocks direction, comments);
#   - the id map has every imported bead id exactly once; a second run creates nothing and rewrites the same map;
#   - a partial run (stopped by a hard failure) names the last step done, and the next run completes it without
#     duplicates; a transient HTTP 429 is retried; a missing label stops the run before any write.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
server_pid=""
trap '[ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true; rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-linear-import: FAIL: $*" >&2; fails=$((fails + 1)); }

cat > "$tmp/server.py" <<'PY'
import json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer
statefile, portfile = sys.argv[1:3]
class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def reply(self, code, out):
        self.send_response(code); self.send_header("Content-Type", "application/json"); self.end_headers()
        self.wfile.write(json.dumps(out).encode())
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        q, v = body["query"], body["variables"]
        st = json.load(open(statefile))
        write = "mutation" in q
        st["calls"].append("write" if write else "read")
        def done(code, out):
            json.dump(st, open(statefile, "w")); self.reply(code, out)
        def uuid(i): return next(x for x in st["issues"] if x["id"] == i)
        if write and st["transient"] > 0:
            st["transient"] -= 1
            return done(429, {"errors": [{"message": "Too many requests"}]})
        if "issueCreate" in q:
            i = v["in"]
            if st["failon"] and st["failon"] in i["title"]:
                return done(200, {"data": None, "errors": [{"message": "Argument Validation Error"}]})
            n = len(st["issues"]) + 1
            st["issues"].append({"id": "u%d" % n, "identifier": "CRI-%d" % n, "title": i["title"], "input": i,
                                 "parent": None, "relations": [], "comments": []})
            data = {"issueCreate": {"success": True, "issue": {"id": "u%d" % n, "identifier": "CRI-%d" % n}}}
        elif "issueUpdate" in q:
            uuid(v["id"])["parent"] = {"id": v["in"]["parentId"]}
            data = {"issueUpdate": {"success": True}}
        elif "issueRelationCreate" in q:
            uuid(v["in"]["issueId"])["relations"].append({"type": v["in"]["type"], "relatedIssue": {"id": v["in"]["relatedIssueId"]}})
            data = {"issueRelationCreate": {"success": True}}
        elif "commentCreate" in q:
            uuid(v["in"]["issueId"])["comments"].append({"body": v["in"]["body"], "input": v["in"]})
            data = {"commentCreate": {"success": True}}
        elif "issueLabels" in q:
            data = {"issueLabels": {"nodes": st["labels"]}}
        elif "projects(" in q:
            data = {"projects": {"nodes": [{"id": "p1", "name": "proj"}]}}
        elif "states" in q:
            data = {"team": {"states": {"nodes": [{"id": "s-" + n, "name": n} for n in st["states"]]}}}
        elif "issues(" in q:
            data = {"issues": {"nodes": [{"id": x["id"], "identifier": x["identifier"], "title": x["title"], "parent": x["parent"],
                                          "relations": {"nodes": x["relations"]}, "comments": {"nodes": x["comments"]}} for x in st["issues"]],
                               "pageInfo": {"hasNextPage": False, "endCursor": None}}}
        else:
            data = {"teams": {"nodes": [{"id": "t1", "key": "TST"}]}}
        done(200, {"data": data})
s = HTTPServer(("127.0.0.1", 0), H)
open(portfile, "w").write(str(s.server_address[1]))
s.serve_forever()
PY

d=. p=critical-fx   # ids built at run time: this script holds no work-item id (check-no-tracker-ids)
E="${p}a"; CE="${p}b"; C1="${p}c"; C2="${p}c${d}2"; F="${p}d"; X="${p}e"; B="${p}f"; DF="${p}g"; T="${p}h"; EC="${p}i"
dep() { jq -nc --arg i "$1" --arg o "$2" --arg t "$3" '{issue_id: $i, depends_on_id: $o, type: $t}'; }
bead() { jq -nc --arg id "$1" --arg ty "$2" --arg st "$3" --argjson p "$4" --argjson extra "$5" \
    '{id: $id, title: ("Title of " + $id), description: ("Description of " + $id + "\n\n## Section\n- [ ] item"), status: $st, priority: $p,
      issue_type: $ty, created_at: "2026-10-15T08:22:42.608089307Z", updated_at: "2026-10-15T08:22:42Z"} + $extra'; }
{
    bead "$E" epic open 2 '{}'
    bead "$CE" epic closed 3 '{"closed_at": "2026-10-16T09:00:00.123456789Z"}'
    bead "$EC" task closed 3 '{"closed_at": "2026-10-16T09:00:00Z"}'
    bead "$C1" task in_progress 1 "$(jq -nc --argjson d "[$(dep "$C1" "$E" parent-child)]" \
        '{labels: ["risky", "backend"], dependencies: $d, notes: "A note to keep.",
          comments: [{id: 7, issue_id: "x", author: "moncef", text: "First comment", created_at: "2026-10-16T10:00:00Z"}]}')"
    bead "$C2" feature open 3 "$(jq -nc --argjson d "[$(dep "$C2" "$E" parent-child), $(dep "$C2" "$C1" blocks), $(dep "$C2" "$X" blocks)]" \
        '{dependencies: $d, acceptance_criteria: "- [ ] must work"}')"
    bead "$F" chore open 4 "$(jq -nc --argjson d "[$(dep "$F" "$C2" blocks)]" '{dependencies: $d}')"
    bead "$X" task closed 3 '{"closed_at": "2026-10-16T09:00:00Z"}'
    bead "$B" bug open 2 "$(jq -nc --argjson d "[$(dep "$B" "$X" parent-child)]" '{dependencies: $d}')"
    bead "$DF" task deferred 3 '{}'
    bead "$T" task tombstone 3 '{"deleted_at": "2026-10-16T09:00:00Z"}'
} > "$tmp/beads.jsonl"
# Imported: E CE C1 C2 F B DF (7); not imported: EC X T.
# Parent-child pairs between imported items: C1 and C2 under E. Blocks: C1 -> C2, C2 -> F. X blocks C2 (not imported).

cat > "$tmp/mapping.json" <<'JSON'
{"team": "TST", "project": "proj",
 "priority": {"P0": {"linear": 1}, "P1": {"linear": 2}, "P2": {"linear": 3}, "P3": {"linear": 4}, "P4": {"linear": 0}},
 "status": {"open": {"state": "Todo"}, "in_progress": {"state": "In Progress"}, "closed": {"state": "Done"},
            "deferred": {"state": "Backlog", "label": "deferred"}},
 "type": {"task": null, "bug": "Bug", "feature": "Feature", "chore": "chore", "epic": "epic"},
 "labels": ["epic", "chore", "deferred", "risky", "backend"]}
JSON

# reset <transient 429 count> <fail-on title substring or "">
reset() {
    jq -n --argjson tr "$1" --arg fo "$2" '{calls: [], issues: [], transient: $tr, failon: $fo,
        states: ["Todo", "In Progress", "Done", "Backlog"],
        labels: ([{name: "Bug", team: null}, {name: "Feature", team: null}] + ([("epic","chore","deferred","risky","backend")] | map({name: ., team: {key: "TST"}})))}' >"$tmp/state.json"
    rm -f "$tmp/map.json"
}
writes() { jq '[.calls[] | select(. == "write")] | length' "$tmp/state.json"; }
count() { jq '.issues | length' "$tmp/state.json"; }
by_bead() { jq -c --arg b "$1" '.issues[] | select(.title | endswith("[" + $b + "]"))' "$tmp/state.json"; }

echo '{}' >"$tmp/state.json"
reset 0 ""
python3 -I "$tmp/server.py" "$tmp/state.json" "$tmp/port" &
server_pid=$!
for _ in $(seq 50); do [ -s "$tmp/port" ] && break; sleep 0.1; done
[ -s "$tmp/port" ] || { echo "test-linear-import: stub server did not start" >&2; exit 1; }
export LINEAR_API_URL="http://127.0.0.1:$(cat "$tmp/port")/graphql"
export LINEAR_API_KEY="lin_api_stub-$$"
export HOME="$tmp/home" CRITICAL_PASSWORD_DIR="$tmp/home/.config/critical"
export LINEAR_MAPPING="$tmp/mapping.json" LINEAR_IMPORT_BEADS="$tmp/beads.jsonl" LINEAR_IMPORT_MAP="$tmp/map.json"
export LINEAR_IMPORT_RETRY_SLEEP=0
mkdir -p "$HOME"
I="$root/scripts/linear-import.sh"

# --- dry run: everything listed, nothing written
"$I" >"$tmp/out" 2>&1 || fail "dry run exited non-zero: $(cat "$tmp/out")"
[ "$(writes)" = 0 ] || fail "dry run wrote to Linear"
[ ! -e "$tmp/map.json" ] || fail "dry run wrote the id map"
[ "$(grep -c 'create   issue' "$tmp/out")" = 7 ] || fail "dry run: expected 7 issues: $(cat "$tmp/out")"
for b in "$E" "$CE" "$C1" "$C2" "$F" "$B" "$DF"; do grep -q "issue $b:" "$tmp/out" || fail "dry run: $b not listed"; done
for b in "$EC" "$X" "$T"; do grep -q "issue $b:" "$tmp/out" && fail "dry run: $b listed but not in scope"; done
grep -q "relate   $C1 blocks $C2" "$tmp/out" || fail "dry run: relation C1 blocks C2 missing"
grep -q "relate   $C2 blocks $F" "$tmp/out" || fail "dry run: relation C2 blocks F missing"
grep -q "link     parent $E <- $C1" "$tmp/out" || fail "dry run: parent link missing"
grep -q 'comment  .*\[notes\]' "$tmp/out" || fail "dry run: notes comment missing"
grep -q 'dry run, nothing written: 7 issue(s) in scope (2 epic(s)); would create 7, 0 already imported; 2 parent link(s), 2 relation(s), 4 comment(s)' "$tmp/out" \
    || fail "dry run summary wrong: $(tail -n1 "$tmp/out")"

# --- run
"$I" --apply >"$tmp/out" 2>&1 || fail "run exited non-zero: $(cat "$tmp/out")"
[ "$(count)" = 7 ] || fail "expected 7 issues, got $(count)"
c1="$(by_bead "$C1")"
[ "$(jq -r .input.stateId <<<"$c1")" = "s-In Progress" ] || fail "C1 state wrong"
[ "$(jq -r .input.priority <<<"$c1")" = 2 ] || fail "C1 priority wrong (P1 -> 2)"
[ "$(jq -r .input.description <<<"$c1")" = "Description of $C1

## Section
- [ ] item" ] || fail "C1 description changed"
[ "$(jq -r .input.createdAt <<<"$c1")" = "2026-10-15T08:22:42.608Z" ] || fail "createdAt not cut to milliseconds: $(jq -r .input.createdAt <<<"$c1")"
[ "$(jq -r .input.teamId <<<"$c1")" = t1 ] && [ "$(jq -r .input.projectId <<<"$c1")" = p1 ] || fail "team/project missing"
[ "$(jq -r '.input.labelIds | sort | join(",")' <<<"$c1")" = "" ] && fail "C1 has no labels"
ce="$(by_bead "$CE")"
[ "$(jq -r .input.stateId <<<"$ce")" = "s-Done" ] || fail "closed epic not Done"
[ "$(jq -r .input.completedAt <<<"$ce")" = "2026-10-16T09:00:00.123Z" ] || fail "closed epic completedAt wrong"
[ "$(by_bead "$DF" | jq -r .input.stateId)" = "s-Backlog" ] || fail "deferred not in Backlog"
[ "$(by_bead "$DF" | jq -r '.input.labelIds | length')" = 1 ] || fail "deferred label missing"
[ "$(by_bead "$C2" | jq -r '.input.description | contains("## Acceptance criteria")')" = true ] || fail "acceptance criteria dropped"
uE="$(by_bead "$E" | jq -r .id)"; uC1="$(by_bead "$C1" | jq -r .id)"; uC2="$(by_bead "$C2" | jq -r .id)"; uF="$(by_bead "$F" | jq -r .id)"
[ "$(by_bead "$C1" | jq -r .parent.id)" = "$uE" ] || fail "C1 parent not E"
[ "$(by_bead "$C2" | jq -r .parent.id)" = "$uE" ] || fail "C2 parent not E"
[ "$(by_bead "$B" | jq -r '.parent')" = null ] || fail "B got a parent though its parent was not imported"
[ "$(by_bead "$C1" | jq -r --arg t "$uC2" '[.relations[] | select(.type == "blocks" and .relatedIssue.id == $t)] | length')" = 1 ] || fail "C1 does not block C2"
[ "$(by_bead "$C2" | jq -r --arg t "$uF" '[.relations[] | select(.type == "blocks" and .relatedIssue.id == $t)] | length')" = 1 ] || fail "C2 does not block F"
[ "$(by_bead "$C2" | jq -r --arg t "$uC1" '[.relations[] | select(.relatedIssue.id == $t)] | length')" = 0 ] || fail "blocks relation reversed"
[ "$(by_bead "$C1" | jq '.comments | length')" = 2 ] || fail "C1 should have comment + notes"
by_bead "$C1" | jq -e '[.comments[].body] | any(contains("First comment")) and any(contains("A note to keep."))' >/dev/null || fail "C1 comment/note text missing"
by_bead "$C1" | jq -e '.comments[] | select(.body | contains("First comment")) | .input.createdAt == "2026-10-16T10:00:00Z"' >/dev/null || fail "comment date not kept"
by_bead "$C2" | jq -e --arg x "$X" '.comments | length == 1 and (.[0].body | contains("Blocked by `" + $x + "` (closed, not imported)"))' >/dev/null || fail "relation to closed item not listed as text on C2"
by_bead "$B" | jq -e --arg x "$X" '.comments | length == 1 and (.[0].body | contains("Child of `" + $x + "`"))' >/dev/null || fail "closed parent of B not listed as text"
by_bead "$X" | grep -q . && fail "closed task imported"
[ "$(jq 'length' "$tmp/map.json")" = 7 ] || fail "id map should hold 7 entries: $(cat "$tmp/map.json")"
for b in "$E" "$CE" "$C1" "$C2" "$F" "$B" "$DF"; do
    [ "$(jq -r --arg b "$b" '.[$b]' "$tmp/map.json")" = "$(by_bead "$b" | jq -r .identifier)" ] || fail "id map: $b wrong"
    [ "$(grep -c "\"$b\"" "$tmp/map.json")" = 1 ] || fail "id map: $b not exactly once"
done
cp "$tmp/map.json" "$tmp/map.first"

# --- second run: nothing created, map unchanged; dry run now reports everything imported
before="$(writes)"
"$I" --apply >"$tmp/out" 2>&1 || fail "second run exited non-zero: $(cat "$tmp/out")"
[ "$(writes)" = "$before" ] || fail "second run wrote ($(writes) vs $before)"
[ "$(count)" = 7 ] || fail "second run duplicated issues"
cmp -s "$tmp/map.json" "$tmp/map.first" || fail "second run changed the id map"
"$I" 2>&1 | grep -q 'would create 0, 7 already imported; 0 parent link(s), 0 relation(s), 0 comment(s)' || fail "dry run after run not empty"

# --- hard failure stops naming the last step; the next run completes without duplicates
reset 0 "Title of $F"
rc=0; "$I" --apply >"$tmp/out" 2>&1 || rc=$?
[ "$rc" = 1 ] || fail "hard failure exited $rc"
grep -q "last step done: issue " "$tmp/out" || fail "hard failure did not name the last step: $(cat "$tmp/out")"
[ "$(count)" -lt 7 ] || fail "failure was not partial"
jq '.failon = ""' "$tmp/state.json" >"$tmp/s2" && mv "$tmp/s2" "$tmp/state.json"
"$I" --apply >"$tmp/out" 2>&1 || fail "resume exited non-zero: $(cat "$tmp/out")"
[ "$(count)" = 7 ] || fail "resume left $(count) issues"
[ "$(jq '[.issues[].title] | unique | length' "$tmp/state.json")" = 7 ] || fail "resume duplicated an issue"
[ "$(by_bead "$C1" | jq '.comments | length')" = 2 ] || fail "resume duplicated or lost comments"
[ "$(jq 'length' "$tmp/map.json")" = 7 ] || fail "resume: id map incomplete"

# --- a transient 429 is retried
reset 2 ""
"$I" --apply >"$tmp/out" 2>&1 || fail "run with 429s failed: $(cat "$tmp/out")"
[ "$(count)" = 7 ] || fail "429 run: $(count) issues"

# --- a missing label stops before any write
reset 0 ""
jq '.labels |= map(select(.name != "risky"))' "$tmp/state.json" >"$tmp/s2" && mv "$tmp/s2" "$tmp/state.json"
rc=0; "$I" --apply >"$tmp/out" 2>&1 || rc=$?
[ "$rc" = 1 ] && grep -q 'label "risky"' "$tmp/out" || fail "missing label not reported: $(cat "$tmp/out")"
[ "$(writes)" = 0 ] || fail "missing label: wrote"

"$I" --bogus >/dev/null 2>&1 && fail "bad option exited 0"
LINEAR_API_KEY= "$I" --apply >"$tmp/out" 2>&1 && fail "apply without a key exited 0"

[ "$fails" = 0 ] || { echo "test-linear-import: $fails failure(s)" >&2; exit 1; }
echo "test-linear-import: ok"
