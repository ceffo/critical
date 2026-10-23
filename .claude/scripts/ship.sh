#!/usr/bin/env bash
# ship.sh <worktree> <title> <body-file> <CRI-n>... : push the branch, open the PR, move each Linear issue to
# "In Review" with a comment holding the PR link, print "PR <n> <mergeable>". Run from anywhere.
# It never closes an issue: Linear's GitHub integration does that when the PR merges.
# A PR closed unmerged goes back with: .claude/scripts/linear-state.sh CRI-n Todo "PR #n closed unmerged"
# Refuses (exit 2, nothing pushed) a HEAD without a green gate stamp, or a stray tracker id: see below.
# If Linear fails for an issue the PR stays open, the failure is on stderr and the exit status is 1.
set -euo pipefail
wt=$1 title=$2 body=$3; shift 3
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
main="${CRITICAL_MAIN:-/home/moncef/code/critical}"
br=$(git -C "$wt" branch --show-current)
# Refused before anything is published (sup-84 retro):
# - a HEAD that `just validate` did not pass on (scripts/gate-stamp.sh), in repos whose justfile has a validate
#   recipe; SHIP_NO_GATE=1 skips it for a PR with nothing to validate.
if [ -z "${SHIP_NO_GATE:-}" ] && grep -qE '^validate:' "$wt/justfile" 2>/dev/null; then
    "$here/../../scripts/gate-stamp.sh" check "$wt" || { echo "ship: refused, nothing pushed" >&2; exit 2; }
fi
# - a tracker id in the title, body or branch that this PR does not close: Linear links it and moves that
#   item (an epic or a later round to In Review, a multi-round item to Done; sup-81, sup-82). Name it in words.
prefixes=$({ echo "${SHIP_ID_PREFIX:-CRI}"; for id in "$@"; do echo "${id%-*}"; done; } | sort -u | paste -sd'|')
stray=$({ echo "$title"; cat "$body"; echo "$br"; } | grep -oiE "\b($prefixes)-[0-9]+\b" | tr '[:lower:]' '[:upper:]' | sort -u \
    | grep -vxF -f <(for id in "$@"; do echo "${id^^}"; done; echo "") || true)
[ -z "$stray" ] || { echo "ship: refused, nothing pushed: $(paste -sd' ' <<<"$stray") named but not closed by this PR (pass it as an argument, or name it in words)" >&2; exit 2; }
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
