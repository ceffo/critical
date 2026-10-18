#!/usr/bin/env bash
# Imports the live beads into Linear:  just linear-import [--apply]
#
# Reads the beads export (.beads/issues.jsonl, read-only) and scripts/linear-mapping.json, and creates in team
# CRI / the project of the mapping: every epic (closed ones as Done, with label `epic`) and every item that is
# not closed, with title, description (unchanged), state, priority and labels per the mapping, the parent link
# of children of imported epics, `blocks` relations between imported items, comments and notes as comments, and
# the original created / completed dates. Tombstones and the other closed history are not imported; a relation
# to such an item is listed in a comment on the issue ("Relations to items not imported").
#
# Without --apply it is a dry run: it prints everything it would create and writes nothing (it reads Linear
# when a key is available, to skip what is already there; offline it assumes nothing is). With --apply it
# writes, and writes the id map (docs/beads-id-map.json: {"critical-xxx": "CRI-n"}) after every issue.
#
# Old id on the issue: the title ends with " [<bead id>]" (a search for the bead id finds it). That is also how
# the run recognizes issues it imported: re-running after a partial or a complete run creates no duplicate and
# fills in what is missing (parent, relations, comments; each comment carries an "imported from beads [...]"
# marker line). This is how the cutover does the final import.
# Transient failures (HTTP 429/5xx, rate limit, network) are retried with a growing pause; any other failure stops
# the run, naming the last step done. Needs $LINEAR_API_KEY or the key file (scripts/linear.sh).
# Test and path overrides: LINEAR_MAPPING, LINEAR_IMPORT_BEADS, LINEAR_IMPORT_MAP, LINEAR_IMPORT_RETRY_SLEEP.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
mapping="${LINEAR_MAPPING:-$here/linear-mapping.json}"
beads="${LINEAR_IMPORT_BEADS:-$here/../.beads/issues.jsonl}"
idmap="${LINEAR_IMPORT_MAP:-$here/../docs/beads-id-map.json}"
apply=0
case "${1:-}" in
    "") ;;
    --apply) apply=1 ;;
    *) echo "usage: linear-import.sh [--apply]" >&2; exit 2 ;;
esac

say() { echo "linear-import: $*"; }
last="nothing yet"
die() { echo "linear-import: FAIL: $*" >&2; [ "$apply" = 0 ] || echo "linear-import: last step done: $last; fix the cause and run again (it resumes)" >&2; exit 1; }
[ -r "$beads" ] || die "cannot read $beads"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

plan="$tmp/plan.json"
jq -s --argjson m "$(cat "$mapping")" -f "$here/linear-import.jq" "$beads" >"$plan" || die "cannot build the plan from $beads"

# --- Linear access. try_call keeps the response in $RESP; transient failures are retried.
RESP=""
retries=6
try_call() {
    local n=1 err
    while :; do
        if RESP="$("$here/linear.sh" "$@" 2>"$tmp/err")"; then return 0; fi
        err="$(cat "$tmp/err")"
        if [ "$n" -ge "$retries" ] || ! grep -qiE 'HTTP (429|5[0-9][0-9])|rate ?limit|cannot reach' <<<"$err"; then
            echo "$err" >&2
            return 1
        fi
        sleep "$(awk -v s="${LINEAR_IMPORT_RETRY_SLEEP:-2}" -v n="$n" 'BEGIN { print s * n }')"
        n=$((n + 1))
    done
}
call() { try_call "$@" || die "Linear call failed"; }

team_key="$(jq -r .team "$mapping")"
project_name="$(jq -r .project "$mapping")"
offline=0
team_id="" project_id="" states='{}' labels='{}'
existing='[]'

if [ "$apply" = 0 ]; then retries=1; fi
if try_call 'query($k: String!) { teams(filter: {key: {eq: $k}}) { nodes { id key } } }' "$(jq -nc --arg k "$team_key" '{k: $k}')" 2>"$tmp/probe-err"; then
    team_id="$(jq -r '.data.teams.nodes[0].id // empty' <<<"$RESP")"
    [ -n "$team_id" ] || die "team $team_key not found"
    [ "$apply" = 0 ] || retries=6
    call 'query($id: String!) { team(id: $id) { states { nodes { id name } } } }' "$(jq -nc --arg i "$team_id" '{id: $i}')"
    states="$RESP"
    call 'query($n: String!) { projects(first: 50, filter: {name: {eq: $n}}) { nodes { id name } } }' "$(jq -nc --arg n "$project_name" '{n: $n}')"
    project_id="$(jq -r --arg n "$project_name" '[.data.projects.nodes[] | select(.name == $n)][0].id // empty' <<<"$RESP")"
    [ -n "$project_id" ] || die "project \"$project_name\" not visible to this key"
    call 'query { issueLabels(first: 250) { nodes { id name team { key } } } }'
    labels="$(jq -c --arg k "$team_key" '[.data.issueLabels.nodes[] | select(.team == null or .team.key == $k)]' <<<"$RESP")"
    # What is already imported (by the bead id at the end of the title), with what it has.
    cursor=""
    while :; do
        call 'query($k: String!, $after: String) { issues(first: 100, after: $after, filter: {team: {key: {eq: $k}}, title: {contains: " [critical-"}}) { nodes { id identifier title parent { id } relations(first: 100) { nodes { type relatedIssue { id } } } comments(first: 250) { nodes { body } } } pageInfo { hasNextPage endCursor } } }' \
            "$(jq -nc --arg k "$team_key" --arg a "$cursor" '{k: $k, after: (if $a == "" then null else $a end)}')"
        existing="$(jq -c --argjson e "$existing" '$e + .data.issues.nodes' <<<"$RESP")"
        [ "$(jq -r '.data.issues.pageInfo.hasNextPage' <<<"$RESP")" = true ] || break
        cursor="$(jq -r '.data.issues.pageInfo.endCursor' <<<"$RESP")"
    done
    existing="$(jq -c '[.[] | . + {bead: (.title | capture("\\[(?<b>critical-[a-z0-9.]+)\\]$").b // null)} | select(.bead != null)]' <<<"$existing")"
    dups="$(jq -r 'group_by(.bead)[] | select(length > 1) | .[0].bead' <<<"$existing")"
    [ -z "$dups" ] || die "several Linear issues carry the same bead id: $dups"
elif [ "$apply" = 1 ]; then
    die "cannot reach Linear: $(cat "$tmp/probe-err")"
else
    offline=1
    say "offline ($(head -n1 "$tmp/probe-err")): assuming nothing is imported yet, checking nothing against Linear"
fi

# Every state and label the plan uses must exist before anything is written.
problems=""
if [ "$offline" = 0 ]; then
    for s in $(jq -r '[.issues[].state] | unique | .[] | gsub(" "; "_")' "$plan"); do
        s="${s//_/ }"
        jq -e --arg s "$s" '[.data.team.states.nodes[].name] | index($s)' <<<"$states" >/dev/null || problems="$problems state \"$s\";"
    done
    while IFS= read -r l; do
        [ -n "$l" ] || continue
        jq -e --arg l "$l" 'map(.name | ascii_downcase) | index($l | ascii_downcase)' <<<"$labels" >/dev/null || problems="$problems label \"$l\";"
    done < <(jq -r '[.issues[].labels[]] | unique | .[]' "$plan")
fi
if [ -n "$problems" ]; then
    if [ "$apply" = 1 ]; then die "missing in team $team_key:$problems run 'just linear-configure' first; nothing was written"; fi
    say "WARNING: missing in team $team_key:$problems 'just linear-configure' creates the labels"
fi

# --- helpers over the existing issues and the ids created or found in this run
uuid_file="$tmp/uuids"; : >"$uuid_file"   # "<bead id> <linear uuid> <CRI-n>"
remember() { echo "$1 $2 $3" >>"$uuid_file"; }
uuid_of() { awk -v b="$1" '$1 == b { print $2 }' "$uuid_file"; }
ident_of() { awk -v b="$1" '$1 == b { print $3 }' "$uuid_file"; }
ex() { jq -c --arg b "$1" '[.[] | select(.bead == $b)][0] // empty' <<<"$existing"; }

write_map() {
    [ "$apply" = 1 ] || return 0
    mkdir -p "$(dirname "$idmap")"
    awk '{ printf "%s\t%s\n", $1, $3 }' "$uuid_file" | jq -Rn '[inputs | split("\t") | {key: .[0], value: .[1]}] | sort_by(.key) | from_entries' >"$idmap.tmp"
    mv "$idmap.tmp" "$idmap"
}

state_id() { jq -r --arg s "$1" '.data.team.states.nodes[] | select(.name == $s) | .id' <<<"$states"; }
label_ids() { jq -c --argjson n "$1" '. as $l | [$n[] | ascii_downcase as $x | ($l | map(select((.name | ascii_downcase) == $x))[0].id)]' <<<"$labels"; }

n_issue=0 n_have=0 n_parent=0 n_rel=0 n_comment=0

# --- 1. issues
while IFS= read -r issue; do
    b="$(jq -r .id <<<"$issue")"
    if found="$(ex "$b")" && [ -n "$found" ]; then
        remember "$b" "$(jq -r .id <<<"$found")" "$(jq -r .identifier <<<"$found")"
        n_have=$((n_have + 1))
        say "exists   $b -> $(ident_of "$b")"
        continue
    fi
    n_issue=$((n_issue + 1))
    say "create   issue $b: $(jq -r .title <<<"$issue") | state=$(jq -r .state <<<"$issue") priority=$(jq -r .priority <<<"$issue") labels=[$(jq -r '.labels | join(",")' <<<"$issue")] created=$(jq -r .createdAt <<<"$issue")$(jq -r 'if .completedAt then " completed=" + .completedAt else "" end' <<<"$issue")"
    [ "$apply" = 1 ] || continue
    input="$(jq -c --arg team "$team_id" --arg project "$project_id" --arg state "$(state_id "$(jq -r .state <<<"$issue")")" \
        --argjson lids "$(label_ids "$(jq -c .labels <<<"$issue")")" \
        '{teamId: $team, projectId: $project, title, description, stateId: $state, priority, labelIds: $lids, createdAt}
         + (if .completedAt then {completedAt} else {} end)' <<<"$issue")"
    call 'mutation($in: IssueCreateInput!) { issueCreate(input: $in) { success issue { id identifier } } }' "$(jq -nc --argjson i "$input" '{in: $i}')"
    jq -e '.data.issueCreate.success' <<<"$RESP" >/dev/null || die "Linear refused to create $b"
    remember "$b" "$(jq -r .data.issueCreate.issue.id <<<"$RESP")" "$(jq -r .data.issueCreate.issue.identifier <<<"$RESP")"
    write_map
    last="issue $b -> $(ident_of "$b")"
done < <(jq -c '.issues[]' "$plan")
[ "$apply" = 0 ] || write_map

# --- 2. parent links
while IFS= read -r issue; do
    b="$(jq -r .id <<<"$issue")"; p="$(jq -r '.parent // empty' <<<"$issue")"
    [ -n "$p" ] || continue
    cur="$(ex "$b" | jq -r '.parent.id // empty' 2>/dev/null || true)"
    if [ -n "$cur" ] && [ "$cur" = "$(uuid_of "$p")" ]; then continue; fi
    n_parent=$((n_parent + 1))
    say "link     parent $p <- $b"
    [ "$apply" = 1 ] || continue
    call 'mutation($id: String!, $in: IssueUpdateInput!) { issueUpdate(id: $id, input: $in) { success } }' \
        "$(jq -nc --arg id "$(uuid_of "$b")" --arg p "$(uuid_of "$p")" '{id: $id, in: {parentId: $p}}')"
    jq -e '.data.issueUpdate.success' <<<"$RESP" >/dev/null || die "Linear refused to set the parent of $b"
    last="parent $p <- $b"
done < <(jq -c '.issues[]' "$plan")

# --- 3. blocks relations ("from" blocks "to")
while IFS= read -r rel; do
    from="$(jq -r .from <<<"$rel")"; to="$(jq -r .to <<<"$rel")"
    fu="$(uuid_of "$from")"; tu="$(uuid_of "$to")"
    if [ -n "$fu" ] && [ -n "$tu" ] && ex "$from" | jq -e --arg t "$tu" '[.relations.nodes[] | select(.type == "blocks" and .relatedIssue.id == $t)] | length > 0' >/dev/null 2>&1; then continue; fi
    n_rel=$((n_rel + 1))
    say "relate   $from blocks $to"
    [ "$apply" = 1 ] || continue
    call 'mutation($in: IssueRelationCreateInput!) { issueRelationCreate(input: $in) { success } }' \
        "$(jq -nc --arg f "$fu" --arg t "$tu" '{in: {issueId: $f, relatedIssueId: $t, type: "blocks"}}')"
    jq -e '.data.issueRelationCreate.success' <<<"$RESP" >/dev/null || die "Linear refused the relation $from blocks $to"
    last="relation $from blocks $to"
done < <(jq -c '.relations[]' "$plan")

# --- 4. comments and notes
while IFS= read -r issue; do
    b="$(jq -r .id <<<"$issue")"
    while IFS= read -r c; do
        [ -n "$c" ] || continue
        tok="$(jq -r .token <<<"$c")"
        if ex "$b" | jq -e --arg t "$tok" '[.comments.nodes[].body | select(contains($t))] | length > 0' >/dev/null 2>&1; then continue; fi
        n_comment=$((n_comment + 1))
        say "comment  $b: $tok"
        [ "$apply" = 1 ] || continue
        call 'mutation($in: CommentCreateInput!) { commentCreate(input: $in) { success } }' \
            "$(jq -nc --arg i "$(uuid_of "$b")" --argjson c "$c" '{in: ({issueId: $i, body: $c.body} + (if $c.createdAt then {createdAt: $c.createdAt} else {} end))}')"
        jq -e '.data.commentCreate.success' <<<"$RESP" >/dev/null || die "Linear refused a comment on $b"
        last="comment on $b ($tok)"
    done < <(jq -c '.comments[]' <<<"$issue")
done < <(jq -c '.issues[]' "$plan")

total="$(jq '.issues | length' "$plan")"
epics="$(jq '[.issues[] | select(.epic)] | length' "$plan")"
if [ "$apply" = 1 ]; then
    say "done: $n_issue issue(s) created, $n_have already there, $n_parent parent link(s), $n_rel relation(s), $n_comment comment(s); id map $idmap has $total entries"
else
    say "dry run, nothing written: $total issue(s) in scope ($epics epic(s)); would create $n_issue, $n_have already imported; $n_parent parent link(s), $n_rel relation(s), $n_comment comment(s) to add"
fi
