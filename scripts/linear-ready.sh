#!/usr/bin/env bash
# linear-ready.sh: the ready queue from Linear, the Linear counterpart of `br ready --json`.
#
#   scripts/linear-ready.sh          # one line per item: id, priority, title, parent (tab separated)
#   scripts/linear-ready.sh --json   # the same items as a JSON array
#
# Ready = team $LINEAR_TEAM (default CRI), state Todo, no blocker that is not Done or Canceled, no
# `needs-info` or `deferred` label, not an `epic` (parents are not dispatchable). Order: priority
# (Urgent, High, Medium, Low, None) then oldest first. Needs $LINEAR_API_KEY or the key file
# (scripts/linear.sh). A missing key or an API error is a message on stderr and exit 1; an empty queue is
# exit 0 with nothing printed on a text run (and `[]` with --json).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
team="${LINEAR_TEAM:-CRI}"
json=0
case "${1:-}" in
    "") ;;
    --json) json=1 ;;
    *) echo "usage: linear-ready.sh [--json]" >&2; exit 2 ;;
esac

query='query($team: String!, $after: String) {
  issues(first: 100, after: $after, filter: {team: {key: {eq: $team}}, state: {name: {eq: "Todo"}}}) {
    nodes {
      identifier title priority createdAt
      parent { identifier }
      labels { nodes { name } }
      inverseRelations { nodes { type issue { identifier state { type } } } }
    }
    pageInfo { hasNextPage endCursor }
  }
}'

nodes='[]'
after=null
while :; do
    vars="$(jq -nc --arg t "$team" --argjson a "$after" '{team: $t, after: $a}')"
    page="$("$here/linear.sh" "$query" "$vars")" || exit 1
    nodes="$(jq -c --argjson more "$(jq -c '.data.issues.nodes' <<<"$page")" '. + $more' <<<"$nodes")"
    [ "$(jq -r '.data.issues.pageInfo.hasNextPage' <<<"$page")" = true ] || break
    after="$(jq -c '.data.issues.pageInfo.endCursor' <<<"$page")"
done

# Linear's priority: 0 none, 1 urgent .. 4 low; none sorts last. A blocker is an inverse relation of type
# "blocks" whose issue is not in a completed or canceled state.
ready="$(jq -c '
  [ .[]
    | select(([.labels.nodes[].name] | any(. == "needs-info" or . == "deferred" or . == "epic")) | not)
    | select([.inverseRelations.nodes[]
              | select(.type == "blocks")
              | select(.issue.state.type | IN("completed", "canceled") | not)] | length == 0)
    | {id: .identifier, priority: .priority, title: .title, parent: (.parent.identifier // null),
       created: .createdAt}
  ]
  | sort_by([(if .priority == 0 then 5 else .priority end), .created])' <<<"$nodes")"

if [ "$json" = 1 ]; then
    echo "$ready"
else
    jq -r '.[] | [.id, ({"0":"None","1":"Urgent","2":"High","3":"Medium","4":"Low"}[.priority|tostring]),
                  .title, (.parent // "")] | @tsv' <<<"$ready"
fi
