#!/usr/bin/env bash
# Stop hook: a supervisor session can't end its turn while CI it owns is pending with no watcher, or
# red and not handled, or while it has live agents and no fleet watcher (.claude/scripts/fleet-watch.sh), or with no Linear watcher (.claude/scripts/queue-watch.sh) at any time. CI = main's CI/Release/Promote runs and every open PR's checks
# (.claude/scripts/ci-state.sh). Other agents are not affected.
#   pending + no live ci-watch.sh  -> blocked: arm .claude/scripts/ci-watch.sh (run_in_background)
#   red, not in supervisor/ci-red-handled -> blocked: act on it, then record it there
# On the hook's own re-entry (stop_hook_active) it lets the turn end, so it can't loop.
set -uo pipefail
in=$(cat)
t=$(jq -r '.transcript_path // empty' <<<"$in")
[ -f "$t" ] && head -c 20000 "$t" | grep -q '"agentSetting":"supervisor"' || exit 0
[ "$(jq -r '.stop_hook_active // false' <<<"$in")" = true ] && exit 0
W=${SUPERVISOR_DIR:-$HOME/code/critical.worktrees/supervisor}
state=$(timeout 60 "$CLAUDE_PROJECT_DIR/.claude/scripts/ci-state.sh" 2>&1); rc=$?
pid=$(cat "$W/ci-watch.pid" 2>/dev/null)
alive=no; [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null && alive=yes
reason=""
if [ $rc = 20 ]; then
  reason="CI is RED and unhandled:
$state
CI is the supervisor's job. Act now (rework brief to the branch's developer, a fresh developer for main or a Dependabot PR, or a rerun for a flake), then append each handled id (run id or pr<n>@<sha7> as printed) with one line on what you did to $W/ci-red-handled, and keep .claude/scripts/ci-watch.sh running."
elif [ $rc = 10 ] && [ $alive = no ]; then
  reason="CI is running and no watcher is armed:
$state
Start .claude/scripts/ci-watch.sh with run_in_background (it replaces any previous watcher) before ending the turn."
fi
# Live agents (anything but sup-*) need the fleet watcher: an agent that finished and waits on the
# supervisor must wake it, whatever it wrote where.
agents=$(herdr agent list 2>/dev/null | jq -r '[.result.agents[]|select(.name|startswith("sup-")|not)|.name]|join(" ")')
fpid=$(cat "$W/fleet-watch.pid" 2>/dev/null)
if [ -n "$agents" ] && ! { [ -n "$fpid" ] && kill -0 "$fpid" 2>/dev/null; }; then
  reason="${reason:+$reason

}Live agents ($agents) and no fleet watcher: start .claude/scripts/fleet-watch.sh with run_in_background before ending the turn. An agent waiting on you must never sit idle."
fi
# Ready work never waits on the supervisor: a Todo item with every blocker Done (READY), or blocked only
# by open PRs (STACK), or an In Progress / In Review item nobody works on (STALLED) blocks the turn's end
# (sup-70: two ready items sat 10 hours behind a stale ledger list). Hold one only as a question to the
# user, listed in supervisor/held.
queue=$(timeout 60 "$CLAUDE_PROJECT_DIR/.claude/scripts/ready-queue.sh" 2>&1); qrc=$?
if [ $qrc = 3 ]; then
  reason="${reason:+$reason

}Work is waiting on you (.claude/scripts/ready-queue.sh):
$queue
Dispatch every READY item and stack every STACK item now (one lane each, per Parallelism); work or flag every STALLED one to the user. An item you can't start is a question to the user: add it to $W/held (\"<id> <reason>\") and to the ledger's pending questions."
fi
# Linear is watched at all times (user, 2026-10-23): no live queue-watch.sh blocks the turn's end, idle or not.
qpid=$(cat "$W/queue-watch.pid" 2>/dev/null)
if [ $qrc != 3 ] && ! { [ -n "$qpid" ] && kill -0 "$qpid" 2>/dev/null; }; then
  reason="${reason:+$reason

}No Linear watcher: start .claude/scripts/queue-watch.sh with run_in_background before ending the turn (it wakes you when an item becomes READY, STACK or STALLED)."
fi
[ -z "$reason" ] && exit 0
jq -n --arg r "$reason" '{decision:"block",reason:$r}'
