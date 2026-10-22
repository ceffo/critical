#!/usr/bin/env bash
# PostToolUse: in a supervisor session, once the context passes 150k tokens, remind it every tool
# call to hand off with .claude/scripts/supervisor-handoff.sh. In a developer session, past 120k,
# remind it to stop on a green commit and report (developers overshot to 155k-270k, sup-75).
# Other agents are not affected.
set -euo pipefail
t="$(jq -r '.transcript_path // empty')"
[ -f "$t" ] || exit 0
head="$(head -c 20000 "$t")"
if grep -q '"agentSetting":"supervisor"' <<<"$head"; then role=supervisor; limit=150000
elif grep -q '"agentSetting":"developer"' <<<"$head"; then role=developer; limit=120000
else exit 0; fi
used="$(tac "$t" | jq -r 'select(.type=="assistant" and .message.usage)|.message.usage
  |(.input_tokens+(.cache_read_input_tokens//0)+(.cache_creation_input_tokens//0))' 2>/dev/null | head -1)"
[ "${used:-0}" -ge "$limit" ] || exit 0
if [ "$role" = developer ]; then
  msg="Context at $((used / 1000))k (>=120k). Finish the current step, commit on green (rerun the gate if you committed after it), then write your report with what is left for the next round, and stop."
else
msg="Context at $((used / 1000))k (>=150k). At the next quiet point (no rework mid-commit): rewrite the ledger with a 'Notes for the next supervisor (sup-N handoff' section and a Retro, then run .claude/scripts/supervisor-handoff.sh (background). Do not wait for the user."
fi
jq -n --arg m "$msg" '{hookSpecificOutput:{hookEventName:"PostToolUse",additionalContext:$m}}'
