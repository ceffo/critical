#!/usr/bin/env bash
# close-agent.sh <name>: the supervisor's way to close a finished fleet agent's herdr tab.
# Looks the tab up by agent name, refuses the caller's own tab and the user's sessions (sup-* other than a
# predecessor is fine; books-* and anything not dev-/rev-/scoper-/designer-/tester-/sup- is refused), records the name
# in supervisor/closed-agents so fleet-watch.sh doesn't wake the supervisor with "GONE <name>", then closes the tab.
set -euo pipefail
W=${SUPERVISOR_DIR:-$HOME/code/critical.worktrees/supervisor}
name=${1:?usage: close-agent.sh <agent-name>}
case "$name" in dev-*|rev-*|scoper-*|designer-*|tester-*|sup-*) ;; *) echo "refused: $name is not a fleet agent" >&2; exit 2;; esac
tab=$(herdr agent list | jq -r --arg n "$name" '.result.agents[]|select(.name==$n).tab_id')
[ -n "$tab" ] || { echo "no live agent named $name" >&2; exit 1; }
own=$(herdr agent list | jq -r --arg p "${HERDR_PANE_ID:-}" '.result.agents[]|select(.pane_id==$p).tab_id')
[ "$tab" != "$own" ] || { echo "refused: $name is in your own tab" >&2; exit 2; }
echo "$name" >> "$W/closed-agents"
herdr tab close "$tab" >/dev/null && echo "closed $name ($tab)"
