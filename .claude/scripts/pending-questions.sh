#!/usr/bin/env bash
# pending-questions.sh: every question waiting on the user, from the two places they live:
#   1. the tracker: open items labelled needs-info, and open items of type question;
#   2. the ledger's "## Pending questions to user" section (questions not tied to an item).
# The supervisor runs it before every reply that ends with the pending-questions list, and
# restates each line in plain words with its default. "No questions pending" only on exit 0
# with nothing printed. Exit 3 when there is at least one question.
set -euo pipefail
main="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
ledger="$(dirname "$main")/$(basename "$main").worktrees/supervisor/ledger.md"
n=0
while IFS=$'\t' read -r id title; do
  n=$((n + 1))
  # the item's own questions: lines of its "Questions" / "Open questions" section
  q="$(cd "$main" && br show "$id" 2>/dev/null | awk '/^## .*[Qq]uestions/{f=1;next} /^## |^[A-Z][A-Za-z ]*:$/{f=0} f&&NF' | head -6)"
  printf '%d. [%s] %s\n%s\n' "$n" "$id" "$title" "${q:-   (no Questions section: write one with a default, then ask)}"
done < <(cd "$main" && br list --json 2>/dev/null | jq -r '
  (if type=="array" then . else .issues end) | flatten | .[]
  | select(.status != "closed")
  | select(((.labels // []) | index("needs-info")) or .issue_type == "question")
  | [.id, .title] | @tsv')
if [ -f "$ledger" ]; then
  while IFS= read -r line; do
    n=$((n + 1)); printf '%d. [ledger] %s\n' "$n" "$line"
  done < <(awk '/^## Pending questions to user/{f=1;next} /^## /{f=0} f' "$ledger" \
           | grep -E '^[0-9]+\.' || true)
fi
[ "$n" -eq 0 ] || exit 3
