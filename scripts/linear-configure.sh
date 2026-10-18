#!/usr/bin/env bash
# Shapes the Linear workspace for this project (idempotent):  just linear-configure [--dry-run]
#
#   - project "critical" belongs to team CRI (CRI is added to its teams; nothing is removed),
#   - team CRI has every label of scripts/linear-mapping.json (Bug and Feature are the workspace labels
#     and are only checked),
#   - the states the mapping names exist in team CRI (the built-in ones; checked, never created).
#
# The mapping (priorities, states, types, labels) is scripts/linear-mapping.json, written once; docs/linear.md
# describes it. With --dry-run nothing is written: the script prints what it would change.
# It never deletes or renames anything. Preconditions that do not hold (key refused, team missing, project
# not visible, a mapped state missing) are reported and stop the run before any write (exit 1).
# Output ends with "already configured" when there was nothing to change. Needs $LINEAR_API_KEY or the key
# file (scripts/linear.sh); $LINEAR_MAPPING points at another mapping (tests).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
mapping="${LINEAR_MAPPING:-$here/linear-mapping.json}"
dry=0
case "${1:-}" in
    "") ;;
    --dry-run) dry=1 ;;
    *) echo "usage: linear-configure.sh [--dry-run]" >&2; exit 2 ;;
esac

say() { echo "linear-configure: $*"; }
fail() { echo "linear-configure: FAIL: $*" >&2; exit 1; }
gql() { "$here/linear.sh" "$@"; }

team_key="$(jq -r .team "$mapping")"
project_name="$(jq -r .project "$mapping")"

teams="$(gql 'query($k: String!) { teams(filter: {key: {eq: $k}}) { nodes { id key name } } }' \
    "$(jq -nc --arg k "$team_key" '{k: $k}')")" || exit 1
team_id="$(jq -r '.data.teams.nodes[0].id // empty' <<<"$teams")"
[ -n "$team_id" ] || fail "team $team_key not found: nothing changed"

# Every state the mapping names must exist (they are the built-in ones).
states="$(gql 'query($id: String!) { team(id: $id) { states { nodes { name } } } }' \
    "$(jq -nc --arg i "$team_id" '{id: $i}')")" || exit 1
missing_states="$(jq -rn --slurpfile m "$mapping" --argjson s "$states" \
    '[$m[0].status[].state] | unique - [$s.data.team.states.nodes[].name] | join(", ")')"
[ -z "$missing_states" ] || fail "team $team_key lacks the state(s) $missing_states: nothing changed"

projects="$(gql 'query($n: String!) { projects(first: 50, filter: {name: {eq: $n}}) { nodes { id name teams(first: 10) { nodes { id key } } } } }' \
    "$(jq -nc --arg n "$project_name" '{n: $n}')")" || exit 1
project="$(jq -c --arg n "$project_name" '[.data.projects.nodes[] | select(.name == $n)]' <<<"$projects")"
case "$(jq length <<<"$project")" in
    0) fail "project \"$project_name\" not visible to this key: nothing changed" ;;
    1) ;;
    *) fail "several projects are named \"$project_name\": nothing changed" ;;
esac
project_id="$(jq -r '.[0].id' <<<"$project")"
in_team="$(jq -r --arg k "$team_key" '.[0].teams.nodes | map(.key) | index($k) != null' <<<"$project")"

labels="$(gql 'query { issueLabels(first: 250) { nodes { name team { key } } } }')" || exit 1
# Labels usable in the team: its own and the workspace ones (no team).
have="$(jq -c --arg k "$team_key" '[.data.issueLabels.nodes[] | select(.team == null or .team.key == $k) | .name | ascii_downcase]' <<<"$labels")"
want_missing="$(jq -rn --slurpfile m "$mapping" --argjson have "$have" \
    '$m[0].labels[] | select((ascii_downcase) as $l | $have | index($l) | not)')"
# Every type label the mapping names must exist already or be created below (Bug and Feature are workspace labels).
for l in $(jq -r '.type[] | select(. != null)' "$mapping"); do
    jq -en --arg l "$l" --argjson have "$have" --slurpfile m "$mapping" \
        '($have | index($l | ascii_downcase)) != null or ($m[0].labels | index($l)) != null' >/dev/null \
        || fail "type label \"$l\" is neither in the workspace nor in the mapping's labels: nothing changed"
done

changes=0
if [ "$in_team" != true ]; then
    changes=$((changes + 1))
    if [ "$dry" = 1 ]; then
        say "would add team $team_key to project \"$project_name\""
    else
        teamids="$(jq -c --arg t "$team_id" '[.[0].teams.nodes[].id] + [$t] | unique' <<<"$project")"
        out="$(gql 'mutation($id: String!, $in: ProjectUpdateInput!) { projectUpdate(id: $id, input: $in) { success } }' \
            "$(jq -nc --arg id "$project_id" --argjson ids "$teamids" '{id: $id, in: {teamIds: $ids}}')")" || exit 1
        jq -e '.data.projectUpdate.success' <<<"$out" >/dev/null || fail "Linear refused to update project \"$project_name\""
        say "added team $team_key to project \"$project_name\""
    fi
fi

while IFS= read -r name; do
    [ -n "$name" ] || continue
    changes=$((changes + 1))
    if [ "$dry" = 1 ]; then
        say "would create label $name in team $team_key"
    else
        out="$(gql 'mutation($in: IssueLabelCreateInput!) { issueLabelCreate(input: $in) { success } }' \
            "$(jq -nc --arg n "$name" --arg t "$team_id" '{in: {name: $n, teamId: $t}}')")" || exit 1
        jq -e '.data.issueLabelCreate.success' <<<"$out" >/dev/null || fail "Linear refused to create label $name"
        say "created label $name in team $team_key"
    fi
done <<<"$want_missing"

if [ "$changes" = 0 ]; then
    say "already configured (team $team_key, project \"$project_name\", $(jq '.labels | length' "$mapping") labels)"
elif [ "$dry" = 1 ]; then
    say "dry run: $changes change(s) pending, nothing written"
else
    say "done: $changes change(s) applied"
fi
