#!/usr/bin/env bash
# linear-state.sh <CRI-n> <State name> [comment] : move a Linear issue to a workflow state of its team and
# optionally comment on it, through scripts/linear.sh. Prints "<CRI-n> -> <State>". Exit 1 on an unknown issue
# or state, or any Linear error.
#
# The supervisor's tracker transitions (the GitHub integration closes the issue on merge, nobody closes by hand):
#   PR opened            linear-state.sh TST-12 "In Review" "PR https://github.com/…/pull/9"   (ship.sh does it)
#   PR closed unmerged   linear-state.sh TST-12 Todo "PR #9 closed unmerged"
#   work claimed         linear-state.sh TST-12 "In Progress"
set -euo pipefail

[ $# -ge 2 ] || { echo "usage: linear-state.sh <CRI-n> <State name> [comment]" >&2; exit 2; }
id=$1 state=$2 comment=${3:-}
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
L="${LINEAR_SH:-$here/../../scripts/linear.sh}"

resp="$("$L" 'query($id: String!) { issue(id: $id) { id identifier team { states { nodes { id name } } } } }' \
    "$(jq -nc --arg id "$id" '{id: $id}')")" || exit 1
uuid="$(jq -r '.data.issue.id // empty' <<<"$resp")"
[ -n "$uuid" ] || { echo "linear-state: no such issue $id" >&2; exit 1; }
sid="$(jq -r --arg s "$state" '[.data.issue.team.states.nodes[] | select(.name == $s) | .id][0] // empty' <<<"$resp")"
[ -n "$sid" ] || { echo "linear-state: $id's team has no state \"$state\"" >&2; exit 1; }

out="$("$L" 'mutation($id: String!, $s: String!) { issueUpdate(id: $id, input: {stateId: $s}) { success } }' \
    "$(jq -nc --arg id "$uuid" --arg s "$sid" '{id: $id, s: $s}')")" || exit 1
[ "$(jq -r '.data.issueUpdate.success' <<<"$out")" = true ] || { echo "linear-state: update of $id refused" >&2; exit 1; }

if [ -n "$comment" ]; then
    out="$("$L" 'mutation($id: String!, $b: String!) { commentCreate(input: {issueId: $id, body: $b}) { success } }' \
        "$(jq -nc --arg id "$uuid" --arg b "$comment" '{id: $id, b: $b}')")" || exit 1
    [ "$(jq -r '.data.commentCreate.success' <<<"$out")" = true ] || { echo "linear-state: comment on $id refused" >&2; exit 1; }
fi
echo "$id -> $state"
