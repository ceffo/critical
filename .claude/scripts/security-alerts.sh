#!/usr/bin/env bash
# security-alerts.sh: file every open GitHub security alert of the repo as an Urgent Linear item.
#   - lists the open Dependabot, code-scanning and secret-scanning alerts (gh api);
#   - an alert whose "<kind> #<n>" already appears in the title of a Linear issue of the team (any
#     state, archived included) is skipped; every other one becomes an issue in team CRI, project
#     critical-hit, priority Urgent, state Todo, titled
#     "Security alert: <kind> #<n> <rule-or-package> (<path>)" (kind: dependabot, code-scanning,
#     secret-scanning), with the alert link, severity, rule/package and location.
# Prints "FILED CRI-n <title>" per item created, and nothing when there is nothing new.
# A failure (gh, Linear) prints "ERROR security-alerts: ..." and exits 1; alerts that were fetched
# fine are still filed. Exit 0 otherwise.
# Secret-scanning alerts: only the whitelisted fields below are read (type, link, state, file path),
# never the `secret` field, so the secret value can't reach Linear.
# Env: LINEAR_SH (tests), GH (tests; default gh).
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
L="${LINEAR_SH:-$here/../../scripts/linear.sh}"
GH="${GH:-gh}"
mapping="$here/../../scripts/linear-mapping.json"
team_key=CRI project_name=critical-hit
if [ -r "$mapping" ]; then
  team_key="$(jq -r '.team // "CRI"' "$mapping")"
  project_name="$(jq -r '.project // "critical-hit"' "$mapping")"
fi
rc=0
err() { echo "ERROR security-alerts: $*"; rc=1; }
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Each alert becomes one JSON line {kind,n,name,loc,sev,url,what}.
fetch() { # fetch <kind> <api path> <jq program producing one object per alert>
  local out
  if ! out="$("$GH" api --paginate "repos/{owner}/{repo}/$2?state=open&per_page=100" 2>&1)"; then
    err "gh api $2: $(head -c 200 <<<"$out" | tr '\n' ' ')"; return
  fi
  # --paginate prints one JSON array per page; flatten them.
  jq -c --arg k "$1" ".[] | $3 | .kind = \$k" <<<"$out" >>"$tmp/alerts" 2>"$tmp/jqerr" \
    || err "unreadable $2 response: $(head -c 200 "$tmp/jqerr")"
}
: >"$tmp/alerts"
fetch dependabot dependabot/alerts '{n: .number, name: (.security_vulnerability.package.name // .dependency.package.name // "unknown"),
  loc: (.dependency.manifest_path // "unknown"), sev: (.security_advisory.severity // "unknown"), url: .html_url,
  what: ((.security_advisory.ghsa_id // "") + ": " + (.security_advisory.summary // "")),
  extra: ("versions `" + (.security_vulnerability.vulnerable_version_range // "?") + "`, patched: " + (.security_vulnerability.first_patched_version.identifier // "none yet"))}'
fetch code-scanning code-scanning/alerts '{n: .number, name: (.rule.id // "unknown"),
  loc: ((.most_recent_instance.location.path // "unknown") + (if .most_recent_instance.location.start_line then ":" + (.most_recent_instance.location.start_line|tostring) else "" end)),
  sev: (.rule.security_severity_level // .rule.severity // "unknown"), url: .html_url,
  what: ((.rule.description // "") + (if .most_recent_instance.message.text then " (" + .most_recent_instance.message.text + ")" else "" end)), extra: ""}'
fetch secret-scanning secret-scanning/alerts '{n: .number, name: (.secret_type_display_name // .secret_type // "unknown"),
  loc: "unknown", sev: "secret", url: .html_url, what: "A secret was detected. Its value is deliberately not copied here: revoke it and read it in GitHub.", extra: ""}'

# Secret-scanning location (file path only, never the detail's secret content).
if jq -e 'select(.kind=="secret-scanning")' "$tmp/alerts" >/dev/null 2>&1; then
  : >"$tmp/alerts2"
  while IFS= read -r a; do
    if [ "$(jq -r .kind <<<"$a")" = secret-scanning ]; then
      n="$(jq -r .n <<<"$a")"
      path="$("$GH" api "repos/{owner}/{repo}/secret-scanning/alerts/$n/locations" --jq '.[0].details | (.path // .blob_url // .html_url // empty)' 2>/dev/null | head -n1)"
      [ -n "$path" ] && a="$(jq -c --arg p "$path" '.loc = $p' <<<"$a")"
    fi
    echo "$a" >>"$tmp/alerts2"
  done <"$tmp/alerts"
  mv "$tmp/alerts2" "$tmp/alerts"
fi

[ -s "$tmp/alerts" ] || exit $rc

# Existing titles (any state, archived too): dedupe on "<kind> #<n>".
: >"$tmp/titles"
cursor=""
while :; do
  vars="$(jq -nc --arg t "$team_key" --arg c "$cursor" '{t: $t, c: (if $c == "" then null else $c end)}')"
  resp="$("$L" 'query($t: String!, $c: String) { issues(first: 250, after: $c, includeArchived: true, filter: {team: {key: {eq: $t}}, title: {startsWith: "Security alert:"}}) { nodes { title } pageInfo { hasNextPage endCursor } } }' "$vars" 2>&1)" \
    || { err "Linear title lookup: $(head -c 200 <<<"$resp" | tr '\n' ' ')"; exit $rc; }
  jq -r '.data.issues.nodes[].title' <<<"$resp" >>"$tmp/titles" || { err "Linear title lookup: unreadable response"; exit $rc; }
  [ "$(jq -r '.data.issues.pageInfo.hasNextPage' <<<"$resp")" = true ] || break
  cursor="$(jq -r '.data.issues.pageInfo.endCursor' <<<"$resp")"
done

new="$tmp/new"; : >"$new"
while IFS= read -r a; do
  key="$(jq -r '"\(.kind) #\(.n)"' <<<"$a")"
  # "#1" must not match "#12": the key is followed by a space in a title.
  grep -qE "^Security alert: $(sed 's/[][\.*^$/]/\\&/g' <<<"$key") " "$tmp/titles" || echo "$a" >>"$new"
done <"$tmp/alerts"
[ -s "$new" ] || exit $rc

ids="$("$L" 'query($t: String!, $p: String!) { teams(filter: {key: {eq: $t}}) { nodes { id states { nodes { id name } } } } projects(first: 50, filter: {name: {eq: $p}}) { nodes { id } } }' \
  "$(jq -nc --arg t "$team_key" --arg p "$project_name" '{t: $t, p: $p}')" 2>&1)" \
  || { err "Linear team lookup: $(head -c 200 <<<"$ids" | tr '\n' ' ')"; exit $rc; }
team_id="$(jq -r '.data.teams.nodes[0].id // empty' <<<"$ids")"
state_id="$(jq -r '[.data.teams.nodes[0].states.nodes[]? | select(.name == "Todo") | .id][0] // empty' <<<"$ids")"
project_id="$(jq -r '.data.projects.nodes[0].id // empty' <<<"$ids")"
[ -n "$team_id" ] && [ -n "$state_id" ] && [ -n "$project_id" ] \
  || { err "Linear has no team $team_key, its Todo state or project $project_name"; exit $rc; }

while IFS= read -r a; do
  title="$(jq -r '"Security alert: \(.kind) #\(.n) \(.name) (\(.loc))"' <<<"$a")"
  desc="$(jq -r '"GitHub \(.kind) alert, severity **\(.sev)**: `\(.name)` at `\(.loc)`.\n\n\(.what)\(if .extra != "" then "\n\n" + .extra else "" end)\n\nAlert: \(.url)\n\n## Acceptance\n\n* The alert is fixed at its cause (or dismissed on GitHub with a stated reason) and no longer open."' <<<"$a")"
  vars="$(jq -nc --arg team "$team_id" --arg proj "$project_id" --arg st "$state_id" --arg t "$title" --arg d "$desc" \
    '{i: {teamId: $team, projectId: $proj, stateId: $st, priority: 1, title: $t, description: $d}}')"
  out="$("$L" 'mutation($i: IssueCreateInput!) { issueCreate(input: $i) { success issue { identifier } } }' "$vars" 2>&1)" \
    || { err "Linear create for \"$title\": $(head -c 200 <<<"$out" | tr '\n' ' ')"; continue; }
  if [ "$(jq -r '.data.issueCreate.success' <<<"$out")" = true ]; then
    echo "FILED $(jq -r '.data.issueCreate.issue.identifier' <<<"$out") $title"
  else
    err "Linear refused to create \"$title\""
  fi
done <"$new"
exit $rc
