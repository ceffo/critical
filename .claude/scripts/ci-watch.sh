#!/usr/bin/env bash
# ci-watch.sh: the supervisor's ONE CI watcher (main + every open PR). Run it with run_in_background.
# Polls .claude/scripts/ci-state.sh every 2 minutes and exits only on a state worth a turn: something
# red, everything green, or one of our PRs (not Dependabot's) that was running going green (ready to
# land). Pending-to-pending moves (main's CI done -> its Release started, a new PR's checks starting,
# Dependabot churn) keep it waiting: each used to cost the supervisor a no-op turn (sup-53 retro).
# Prints the new state. Exits at
# once if nothing is pending. Each poll also runs security-alerts.sh (files new GitHub security alerts
# as urgent Linear items); what it prints (FILED / ERROR lines) is a reason to exit too, printed before
# the CI state, and never changes the CI exit code or hides CI. A failing alert check never stops CI watching.
# Records its pid in supervisor/ci-watch.pid, which the Stop hook
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
alerts() { "$here/security-alerts.sh" 2>&1 || true; }
start=$("$here/ci-state.sh"); rc=$?
found=$(alerts)
if [ $rc != 10 ] || [ -n "$found" ]; then
  [ -n "$found" ] && echo "$found"
  echo "$start"; echo "exit=$rc"; exit 0
fi
seen=$(ours <<<"$start")
while sleep "${CI_WATCH_INTERVAL:-120}"; do
  now=$("$here/ci-state.sh"); rc=$?
  found=$(alerts)
  if [ $rc = 10 ]; then
    running=$(ours <<<"$now")
    done_prs=$(comm -23 <(printf '%s\n' "$seen" | sed '/^$/d') <(printf '%s\n' "$running" | sed '/^$/d'))
    seen=$(printf '%s\n%s\n' "$seen" "$running" | sed '/^$/d' | sort -u)
    [ -z "$done_prs" ] && [ -z "$found" ] && continue
    # One verdict line per PR whose checks stopped, so the supervisor acts without a second call
    # (sup-84 retro): GREEN = land it (land.sh re-checks), RED = rework, else merged/closed.
    for n in $done_prs; do
      st=$(gh pr view "$n" --json state -q .state 2>/dev/null)
      if [ "$st" != OPEN ]; then echo "${st:-UNKNOWN} PR #$n"
      elif gh pr checks "$n" >/dev/null 2>&1; then echo "GREEN PR #$n: ready to land"
      else echo "RED PR #$n: $(gh pr checks "$n" 2>/dev/null | awk -F'\t' '$2=="fail"{printf "%s; ", $1}')"
      fi
    done
  fi
  [ -n "$found" ] && echo "$found"
  echo "$now"; echo "exit=$rc"; exit 0
done
