#!/usr/bin/env bash
# ci-watch.sh: the supervisor's ONE CI watcher (main + every open PR). Run it with run_in_background.
# Polls .claude/scripts/ci-state.sh every 2 minutes and exits only on a state worth a turn: something
# red, everything green, or one of our PRs (not Dependabot's) that was running going green (ready to
# land). Pending-to-pending moves (main's CI done -> its Release started, a new PR's checks starting,
# Dependabot churn) keep it waiting: each used to cost the supervisor a no-op turn (sup-53 retro).
# Prints the new state. Exits at
# once if nothing is pending. Records its pid in supervisor/ci-watch.pid, which the Stop hook
# (.claude/hooks/supervisor-ci-guard.sh) checks: a supervisor can't end a turn with CI pending and no
# live watcher. Starting a new one replaces (kills) the previous one.
set -uo pipefail
W=${SUPERVISOR_DIR:-$HOME/code/critical.worktrees/supervisor}
here=$(dirname "$0")
pidf="$W/ci-watch.pid"
old=$(cat "$pidf" 2>/dev/null) && [ -n "$old" ] && [ "$old" != $$ ] && kill "$old" 2>/dev/null
echo $$ > "$pidf"
trap '[ "$(cat "$pidf" 2>/dev/null)" = $$ ] && rm -f "$pidf"' EXIT
# Our PRs with checks running ("PENDING PR #n @sha: ..."; Dependabot's lines carry "(dependabot)").
ours() { grep -oE '^PENDING PR #[0-9]+ @' | grep -oE '[0-9]+' | sort -u; }
start=$("$here/ci-state.sh"); rc=$?
[ $rc = 10 ] || { echo "$start"; echo "exit=$rc"; exit 0; }
seen=$(ours <<<"$start")
while sleep 120; do
  now=$("$here/ci-state.sh"); rc=$?
  if [ $rc = 10 ]; then
    running=$(ours <<<"$now")
    done_prs=$(comm -23 <(printf '%s\n' "$seen" | sed '/^$/d') <(printf '%s\n' "$running" | sed '/^$/d'))
    seen=$(printf '%s\n%s\n' "$seen" "$running" | sed '/^$/d' | sort -u)
    [ -z "$done_prs" ] && continue
    echo "PR(s) no longer running: $(tr '\n' ' ' <<<"$done_prs")"
  fi
  echo "$now"; echo "exit=$rc"; exit 0
done
