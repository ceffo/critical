#!/usr/bin/env bash
# fleet-watch.sh: the supervisor's ONE watcher over its live agents (every herdr agent except sup-*).
# Run it with run_in_background. Polls `herdr agent list` every 30 s and exits when an agent:
#   - becomes blocked (a permission prompt or question in its tab; not scoper-*/designer-*, whose
#     questions are for the user)                                                 -> "BLOCKED <name>"
#   - ends a turn (working -> idle/done) and its last screen speaks to the supervisor
#     (supervisor / ship / push / PR / merge), or its report file is newer than   -> "NEEDS-SUPERVISOR <name>"
#     the last prompt it got
#   - disappears                                                                   -> "GONE <name>"
# An agent ending a turn that only talks to the user (live steps, a scoper's questions) doesn't wake it.
# Prints the agent's last lines. Records its pid in supervisor/fleet-watch.pid, which the Stop hook
# (.claude/hooks/supervisor-ci-guard.sh) checks: a supervisor can't end a turn with live agents and no
# fleet watcher. Starting a new one replaces (kills) the previous one.
set -uo pipefail
W=${SUPERVISOR_DIR:-$HOME/code/critical.worktrees/supervisor}
R=${REPORTS_DIR:-$(dirname "$W")/reports}
pidf="$W/fleet-watch.pid"
old=$(cat "$pidf" 2>/dev/null) && [ -n "$old" ] && [ "$old" != $$ ] && kill "$old" 2>/dev/null
echo $$ > "$pidf"
trap '[ "$(cat "$pidf" 2>/dev/null)" = $$ ] && rm -f "$pidf"' EXIT
declare -A seen state
snap() { herdr agent list 2>/dev/null | jq -r '.result.agents[]|select(.name|startswith("sup-")|not)|"\(.name) \(.agent_status) \(.state_change_seq)"'; }
while read -r n s q; do [ -n "$n" ] && { seen[$n]=$q; state[$n]=$s; }; done < <(snap)
fresh_report() { # the agent's report newer than the supervisor's last prompt marker for its item
  local item=${1#*-}; item=${item^^}; local f
  for f in "$R"/*"$item"*-dev.md "$R"/*"${item}"*.md; do
    [ -f "$f" ] || continue
    for p in "$W"/.prompt-"$item"*; do [ -f "$p" ] && [ "$f" -nt "$p" ] && return 0; done
  done; return 1
}
while sleep 30; do
  declare -A now=()
  while read -r n s q; do
    [ -n "$n" ] || continue; now[$n]=1
    [ "${seen[$n]:-}" = "$q" ] && continue
    prev=${state[$n]:-working}; seen[$n]=$q; state[$n]=$s
    tail=$(herdr agent read "$n" --source recent-unwrapped --lines 25 2>/dev/null | grep -v '^\s*$' | tail -15)
    # scoper-* / designer-* talk to the user by design: their questions (blocked) are for the user.
    if [ "$s" = blocked ] && [[ "$n" != scoper-* && "$n" != designer-* ]]; then echo "BLOCKED $n"; echo "$tail"; exit 0; fi
    if [ "$prev" = working ] && { [ "$s" = idle ] || [ "$s" = done ]; }; then
      if grep -qiE 'supervisor|ship|push|pull request|\bPR\b|merge' <<<"$tail" || fresh_report "$n"; then
        echo "NEEDS-SUPERVISOR $n"; echo "$tail"; exit 0
      fi
    fi
  done < <(snap)
  for n in "${!seen[@]}"; do [ -n "${now[$n]:-}" ] || { echo "GONE $n"; exit 0; }; done
  unset now
done
