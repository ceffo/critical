#!/usr/bin/env bash
# linear-cutover.sh : switch the supervisor's scripts from beads to Linear (run once, at cutover, after this
# directory is on the main checkout). In the supervisor directory it replaces ship.sh and pr-merge-watch.sh with
# one-line wrappers around the versions in .claude/scripts, and deletes the beads conflict tooling (merge.sh,
# sync-branch.sh, beads-line.py). Idempotent. $SUPERVISOR_DIR and $CRITICAL_MAIN override the locations.
set -euo pipefail
sup="${SUPERVISOR_DIR:-/home/moncef/code/critical.worktrees/supervisor}"
main="${CRITICAL_MAIN:-/home/moncef/code/critical}"
for f in ship.sh pr-merge-watch.sh land.sh linear-state.sh; do
    [ -x "$main/.claude/scripts/$f" ] || { echo "linear-cutover: $main/.claude/scripts/$f missing (pull main first)" >&2; exit 1; }
done
for f in ship.sh pr-merge-watch.sh; do
    printf '#!/bin/bash\nexec "%s/.claude/scripts/%s" "$@"\n' "$main" "$f" >"$sup/$f"
    chmod +x "$sup/$f"
done
rm -f "$sup/merge.sh" "$sup/sync-branch.sh" "$sup/beads-line.py"
echo "supervisor scripts now use Linear: ship.sh, pr-merge-watch.sh -> $main/.claude/scripts; merge.sh, sync-branch.sh, beads-line.py removed"
