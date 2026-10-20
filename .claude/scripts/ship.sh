#!/usr/bin/env bash
# ship.sh <worktree> <title> <body-file> <CRI-n>... : push the branch, open the PR, move each Linear issue to
# "In Review" with a comment holding the PR link, print "PR <n> <mergeable>". Run from anywhere.
# It never closes an issue: Linear's GitHub integration does that when the PR merges.
# A PR closed unmerged goes back with: .claude/scripts/linear-state.sh CRI-n Todo "PR #n closed unmerged"
# If Linear fails for an issue the PR stays open, the failure is on stderr and the exit status is 1.
set -euo pipefail
wt=$1 title=$2 body=$3; shift 3
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
main="${CRITICAL_MAIN:-/home/moncef/code/critical}"
br=$(git -C "$wt" branch --show-current)
git -C "$wt" push -q -u origin "$br" 2>&1 | grep -v '^remote' || true
url=$(cd "$main" && gh pr create --head "$br" --title "$title" --body-file "$body" | tail -1)
n=${url##*/}
rc=0
sup=${SUPERVISOR_DIR:-$HOME/code/critical.worktrees/supervisor}
for id in "$@"; do
    # the report is handled: fleet-watch must not re-fire NEEDS-SUPERVISOR on it
    [ -d "$sup" ] && touch "$sup/.prompt-$id"
    "$here/linear-state.sh" "$id" "In Review" "PR $url" >/dev/null || { echo "ship: $id not moved to In Review" >&2; rc=1; }
done
sleep "${SHIP_SETTLE:-5}"; echo "PR $n $(cd "$main" && gh pr view "$n" --json mergeable -q .mergeable)"
exit $rc
