#!/usr/bin/env bash
# land.sh <pr> [tab] : squash-merge the PR, close the dev tab, fast-forward the main checkout, print the new
# main sha. No tracker step: Linear's GitHub integration closes the issue on merge.
set -euo pipefail
main="${CRITICAL_MAIN:-/home/moncef/code/critical}"
cd "$main"
gh pr merge "$1" --squash --delete-branch >/dev/null 2>&1 || gh stack merge "$1" --squash --yes >/dev/null 2>&1 || { echo "MERGE FAILED"; exit 1; }
git worktree prune
[ -z "${2:-}" ] || herdr tab close "$2" >/dev/null 2>&1 || true
git fetch -q origin && git merge -q --ff-only origin/main
git log --oneline -1 | cut -c1-90
