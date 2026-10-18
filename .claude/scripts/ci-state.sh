#!/usr/bin/env bash
# ci-state.sh: the state of every CI the supervisor owns, in one call.
#   - main: the latest run of each workflow that runs on main (CI, Release, Promote), and the Release
#     run that CI's success on main's head must trigger;
#   - every open PR (ours and Dependabot's): its checks.
# Prints one line per item that is not green: "PENDING <what>" or "RED <what> <url>".
# Exit 0: all green. 10: something pending, nothing red. 20: something red.
# A red item listed (its run id or "pr<n>@<sha7>") in supervisor/ci-red-handled is reported as
# "HANDLED ..." and doesn't count: append it there once you've acted on it (rework brief, rerun).
set -uo pipefail
W=${SUPERVISOR_DIR:-$HOME/code/critical.worktrees/supervisor}
handled="$W/ci-red-handled"
pending=0; red=0
is_handled() { [ -f "$handled" ] && grep -qF -- "$1" "$handled"; }
flag() { # flag <kind PENDING|RED> <key> <text>
  if [ "$1" = RED ]; then
    if is_handled "$2"; then echo "HANDLED $3"; else echo "RED $3"; red=1; fi
  else echo "PENDING $3"; pending=1; fi
}

runs=$(gh run list --branch main --limit 30 --json databaseId,workflowName,status,conclusion,headSha,event,url 2>&1) \
  || { echo "ERROR gh run list: $(head -c 200 <<<"$runs")"; exit 20; }
head_sha=$(gh api repos/{owner}/{repo}/commits/main --jq .sha 2>/dev/null)
for wf in CI Release Promote; do
  r=$(jq -c --arg w "$wf" '[.[]|select(.workflowName==$w and .event!="pull_request")][0] // empty' <<<"$runs")
  [ -n "$r" ] || continue
  id=$(jq -r .databaseId <<<"$r"); st=$(jq -r .status <<<"$r"); c=$(jq -r .conclusion <<<"$r"); url=$(jq -r .url <<<"$r")
  if [ "$st" != completed ]; then flag PENDING "$id" "main $wf run $id ($st)"
  elif [ "$c" != success ] && [ "$c" != skipped ] && [ "$c" != neutral ]; then flag RED "$id" "main $wf run $id ($c) $url"; fi
done
# CI green on main's head but its Release run not started yet (workflow_run gap).
ci_head=$(jq -r --arg s "$head_sha" '[.[]|select(.workflowName=="CI" and .headSha==$s and .event=="push")][0].conclusion // empty' <<<"$runs")
rel_head=$(jq -r --arg s "$head_sha" '[.[]|select(.workflowName=="Release" and .headSha==$s)]|length' <<<"$runs")
[ "$ci_head" = success ] && [ "$rel_head" = 0 ] && flag PENDING "release-$head_sha" "main Release for ${head_sha:0:7} not started yet"
[ -z "$ci_head" ] && [ -n "$head_sha" ] && ! jq -e --arg s "$head_sha" 'any(.[];.workflowName=="CI" and .headSha==$s)' <<<"$runs" >/dev/null \
  && flag PENDING "ci-$head_sha" "main CI for ${head_sha:0:7} not started yet"

prs=$(gh pr list --state open --json number,headRefOid,statusCheckRollup 2>&1) \
  || { echo "ERROR gh pr list: $(head -c 200 <<<"$prs")"; exit 20; }
while IFS= read -r p; do
  [ -n "$p" ] || continue
  n=$(jq -r .number <<<"$p"); sha=$(jq -r '.headRefOid[0:7]' <<<"$p")
  np=$(jq '[.statusCheckRollup[]|select((.status // "COMPLETED")!="COMPLETED")]|length' <<<"$p")
  bad=$(jq -r '[.statusCheckRollup[]|select((.status // "COMPLETED")=="COMPLETED")|select(((.conclusion // .state)|IN("SUCCESS","SKIPPED","NEUTRAL"))|not)|.name // .context]|join(",")' <<<"$p")
  [ -n "$bad" ] && flag RED "pr$n@$sha" "PR #$n @$sha failing: $bad"
  [ "$np" -gt 0 ] && [ -z "$bad" ] && flag PENDING "pr$n" "PR #$n @$sha: $np check(s) running"
done < <(jq -c '.[]' <<<"$prs")

[ "$red" = 1 ] && exit 20
[ "$pending" = 1 ] && exit 10
exit 0
