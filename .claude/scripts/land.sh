#!/usr/bin/env bash
# land.sh <pr> [tab] : THE supervisor's only merge path for ceffo/critical (user-approved 2026-10-18).
# It enforces the review gate itself and refuses (exit 2, prints why) unless ALL hold on the PR's head:
#   - every check has completed and is SUCCESS / SKIPPED / NEUTRAL, at least one check exists;
#   - GitHub says MERGEABLE and CLEAN (required checks reported, not behind or blocked);
#   - every Greptile inline comment thread, if any, has a reply (handled: fixed, or answered why not).
# No wait for a Greptile review (user, 2026-10-19: the trial ran out; "drop the wait").
# Then: squash-merge (pinned to the checked head sha), close the dev tab if given, prune worktrees,
# fast-forward the main checkout. Call it as ONE plain command (absolute path, nothing chained) so the
# permission allow rule matches. No tracker step: Linear's GitHub integration closes the issue on merge.
set -euo pipefail
pr="${1:?usage: land.sh <pr> [tab]}"
main="${CRITICAL_MAIN:-/home/moncef/code/critical}"
repo="${CRITICAL_REPO:-ceffo/critical}"
cd "$main"
refuse() { echo "REFUSED #$pr: $*"; exit 2; }
j=$(gh pr view "$pr" -R "$repo" --json state,mergeable,mergeStateStatus,headRefOid,statusCheckRollup)
[ "$(jq -r .state <<<"$j")" = OPEN ] || refuse "state $(jq -r .state <<<"$j")"
sha=$(jq -r .headRefOid <<<"$j")
[ "$(jq '.statusCheckRollup|length' <<<"$j")" -gt 0 ] || refuse "no checks reported yet"
# A check can report several runs on one head (a concurrency group cancels the first): judge each by its
# latest run only, or a cancelled duplicate refuses a green PR.
latest='[.statusCheckRollup|group_by(.name // .context)[]|max_by(.startedAt // .createdAt // "")]'
pending=$(jq -r "$latest"'|[.[]|select((.status // "COMPLETED")!="COMPLETED")|.name]|join(",")' <<<"$j")
[ -z "$pending" ] || refuse "checks pending: $pending"
bad=$(jq -r "$latest"'|[.[]|select(((.conclusion // .state)|IN("SUCCESS","SKIPPED","NEUTRAL"))|not)|(.name // .context)]|join(",")' <<<"$j")
[ -z "$bad" ] || refuse "checks not green: $bad"
[ "$(jq -r .mergeable <<<"$j")" = MERGEABLE ] || refuse "mergeable=$(jq -r .mergeable <<<"$j")"
[ "$(jq -r .mergeStateStatus <<<"$j")" = CLEAN ] || refuse "mergeStateStatus=$(jq -r .mergeStateStatus <<<"$j")"
unanswered=$(gh api --paginate "repos/$repo/pulls/$pr/comments" | jq -s 'add // [] | . as $c
  | [$c[]|select(.in_reply_to_id==null and (.user.login|test("greptile")))|.id as $r
     |select([$c[]|select(.in_reply_to_id==$r)]|length==0)]|length')
[ "$unanswered" = 0 ] || refuse "$unanswered Greptile comment thread(s) without a reply"
# Pinned to the checked head: if the head moved after the checks, the merge fails and nothing is merged.
if ! err=$(gh pr merge "$pr" -R "$repo" --squash --delete-branch --match-head-commit "$sha" 2>&1 >/dev/null); then
  # A PR in a GitHub stack can only merge through the stack merge. Only the stack's bottom PR (based on
  # main), and only if its head is still the checked sha: a higher PR would merge unchecked parents.
  grep -q "part of a stack" <<<"$err" || { echo "MERGE FAILED #$pr (head moved from ${sha:0:7}, or GitHub refused)"; exit 1; }
  h=$(gh pr view "$pr" -R "$repo" --json headRefOid,baseRefName --jq '.headRefOid+" "+.baseRefName')
  [ "$h" = "$sha main" ] || { echo "MERGE FAILED #$pr (stacked: head/base now $h; only the bottom PR at ${sha:0:7} lands)"; exit 1; }
  gh stack merge "$pr" --yes --squash >/dev/null 2>&1 || { echo "MERGE FAILED #$pr (stack merge refused)"; exit 1; }
fi
git worktree prune
[ -z "${2:-}" ] || herdr tab close "$2" >/dev/null 2>&1 || true
if [ "$(git branch --show-current)" = main ]; then git fetch -q origin && git merge -q --ff-only origin/main; fi
echo "MERGED #$pr (head ${sha:0:7})"
