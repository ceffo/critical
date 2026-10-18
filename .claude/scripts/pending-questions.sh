#!/usr/bin/env bash
# pending-questions.sh: every question waiting on the user, from the two places they live:
#   1. the tracker: open items labelled needs-info, and open items of type question;
#   2. the ledger's "## Pending questions to user" section (questions not tied to an item).
# The supervisor runs it before every reply that ends with the pending-questions list, and
# restates each line in plain words with its default. "No questions pending" only on exit 0
# with nothing printed. Exit 3 when there is at least one question.
# CRITICAL_TRACKER=linear reads the tracker part from Linear (open items of team $LINEAR_TEAM, default CRI,
# labelled needs-info or question; the "Questions" section comes from the issue description) through
# scripts/linear.sh; the default stays beads until the cutover. A missing key or an API error is a message
# on stderr and exit 1, never exit 3 and never a silent "no questions".
set -euo pipefail
main="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
ledger="${CRITICAL_LEDGER:-$(dirname "$main")/$(basename "$main").worktrees/supervisor/ledger.md}"
n=0
# the item's own questions: lines of its "Questions" / "Open questions" section (stdin: the item's text)
questions() { awk '/^## .*[Qq]uestions/{f=1;next} /^## |^[A-Z][A-Za-z ]*:$/{f=0} f&&NF' | head -6; }
if [ "${CRITICAL_TRACKER:-beads}" = linear ]; then
  query='query($team: String!) {
    issues(first: 250, filter: {team: {key: {eq: $team}}, state: {type: {nin: ["completed", "canceled"]}},
           or: [{labels: {name: {eq: "needs-info"}}}, {labels: {name: {eq: "question"}}}]}) {
      nodes { identifier title description }
    }
  }'
  resp="$("$main/scripts/linear.sh" "$query" "$(jq -nc --arg t "${LINEAR_TEAM:-CRI}" '{team: $t}')")" || exit 1
  while IFS= read -r row; do
    n=$((n + 1))
    id="$(jq -r .identifier <<<"$row")"; title="$(jq -r .title <<<"$row")"
    q="$(jq -r '.description // ""' <<<"$row" | questions)"
    printf '%d. [%s] %s\n%s\n' "$n" "$id" "$title" "${q:-   (no Questions section: write one with a default, then ask)}"
  done < <(jq -c '.data.issues.nodes[]' <<<"$resp")
else
  while IFS=$'\t' read -r id title; do
    n=$((n + 1))
    q="$(cd "$main" && br show "$id" 2>/dev/null | questions)"
    printf '%d. [%s] %s\n%s\n' "$n" "$id" "$title" "${q:-   (no Questions section: write one with a default, then ask)}"
  done < <(cd "$main" && br list --json 2>/dev/null | jq -r '
    (if type=="array" then . else .issues end) | flatten | .[]
    | select(.status != "closed")
    | select(((.labels // []) | index("needs-info")) or .issue_type == "question")
    | [.id, .title] | @tsv')
fi
if [ -f "$ledger" ]; then
  while IFS= read -r line; do
    n=$((n + 1)); printf '%d. [ledger] %s\n' "$n" "$line"
  done < <(awk '/^## Pending questions to user/{f=1;next} /^## /{f=0} f' "$ledger" \
           | grep -E '^[0-9]+\.' || true)
fi
[ "$n" -eq 0 ] || exit 3
