#!/usr/bin/env bash
# queue-watch.sh: the supervisor's Linear watcher, running AT ALL TIMES (user, 2026-10-23: "monitoring linear
# should be active at all times"), even with no agent and no CI in flight: when the user moves an item to Todo
# or a blocker lands, the supervisor wakes and dispatches it. Run it with run_in_background, one at a time
# (a new one replaces the previous one). Polls .claude/scripts/ready-queue.sh every QUEUE_WATCH_INTERVAL
# seconds (default 180) and exits, printing its lines, as soon as it prints anything (READY / STACK / STALLED).
# A Linear error is printed and retried, never an exit. Records its pid in supervisor/queue-watch.pid, which
# the Stop hook (.claude/hooks/supervisor-ci-guard.sh) checks.
set -uo pipefail
W=${SUPERVISOR_DIR:-$HOME/code/critical.worktrees/supervisor}
here=$(dirname "$0")
pidf="$W/queue-watch.pid"
old=$(cat "$pidf" 2>/dev/null) && [ -n "$old" ] && [ "$old" != $$ ] && kill "$old" 2>/dev/null
echo $$ > "$pidf"
trap '[ "$(cat "$pidf" 2>/dev/null)" = $$ ] && rm -f "$pidf"' EXIT
while :; do
  out=$("$here/ready-queue.sh" 2>&1); rc=$?
  [ $rc = 3 ] && { echo "$out"; exit 0; }
  [ $rc = 0 ] || echo "queue-watch: ready-queue.sh failed (rc $rc), retrying: $(tail -1 <<<"$out")" >&2
  sleep "${QUEUE_WATCH_INTERVAL:-180}"
done
