#!/usr/bin/env bash
# ready-queue.sh: the supervisor's dispatch queue, rebuilt from Linear every time (never from the ledger).
# Prints one line per item that needs the supervisor:
#   READY <id> P<prio> <title>      Todo, not an epic (children or label epic), not needs-info, every blocker Done: dispatch it now
#   STACK <id> P<prio> <title> on <ids>   Todo, its open blockers are all In Review (open PR): stack it
#   STALLED <id> <state> <title>    In Progress / In Review with no live dev-/rev- agent for it and no open PR
#                                   (In Progress) : work it or flag it to the user, never leave it silent
# Exit 0: nothing printed. 3: at least one line. 1: Linear error (stderr).
# Items listed in supervisor/held (one "<id> <reason>" per line, the reason being a question already put
# to the user) are skipped: holding is a question to the user, so it must be in the pending list too.
# (sup-70: the ledger's "not dispatchable" list was carried over unchecked and two ready items sat for 10h.)
set -euo pipefail
main="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
W=${SUPERVISOR_DIR:-$(dirname "$main")/$(basename "$main").worktrees/supervisor}
query='query($team: String!) {
  issues(first: 250, filter: {team: {key: {eq: $team}}, state: {type: {in: ["unstarted", "started"]}}}) {
    nodes { identifier title priority state { name type } labels { nodes { name } } children { nodes { id } }
            inverseRelations { nodes { type issue { identifier state { name type } } } } }
  }
}'
resp="$("$main/scripts/linear.sh" "$query" "$(jq -nc --arg t "${LINEAR_TEAM:-CRI}" '{team: $t}')")" || exit 1
held=$(awk '{print $1}' "$W/held" 2>/dev/null || true)
agents=$(herdr agent list 2>/dev/null | jq -r '.result.agents[].name' 2>/dev/null || true)
prs=$(gh pr list --state open --json title,body -q '.[]|.title+" "+.body' 2>/dev/null || true)
out=$(jq -r '
  .data.issues.nodes[]
  | select((.children.nodes|length)==0)
  | select(([.labels.nodes[].name]|(index("needs-info") or index("epic")))|not)
  | . as $i
  | [.inverseRelations.nodes[]|select(.type=="blocks")|.issue|select(.state.type!="completed" and .state.type!="canceled")] as $open
  | if .state.type=="unstarted" then
      if ($open|length)==0 then "READY \(.identifier) P\(.priority) \(.title)"
      elif ($open|all(.state.name=="In Review")) then "STACK \(.identifier) P\(.priority) \(.title) on \([$open[].identifier]|join(","))"
      else empty end
    else "CHECK \(.identifier) \(.state.name) \(.title)" end' <<<"$resp")
n=0
while IFS= read -r line; do
  [ -n "$line" ] || continue
  id=$(awk '{print $2}' <<<"$line")
  grep -qxF "$id" <<<"$held" && continue
  if [ "${line%% *}" = CHECK ]; then
    low=$(tr 'A-Z' 'a-z' <<<"$id")
    grep -qE "^(dev|rev|tester)-$low$" <<<"$agents" && continue
    grep -qwF "$id" <<<"$prs" && [[ "$line" == *" In Review "* ]] && continue
    line="STALLED ${line#CHECK }"
  fi
  echo "$line"; n=$((n + 1))
done <<<"$out"
[ "$n" -eq 0 ] || exit 3
