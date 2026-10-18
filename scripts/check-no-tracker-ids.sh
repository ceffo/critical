#!/usr/bin/env bash
# Fail on any work-item id in tracked code, tests, seeds or config: a Linear id (CRI-<n>) or an old
# bead id (critical-<id>, critical-<id>.N, <id>.N). Code says what it does and why; the link to the
# work item lives in commits and PRs only (docs/agent-workflow.md, "Hard rules for implementers").
#
# Linear ids are matched by pattern, no tracker needed. Old bead ids are listed from the frozen
# archive .beads/issues.jsonl while it exists, and from the committed id map docs/beads-id-map.json
# (any critical-<id> in it) once the archive is gone; both are used when both exist.
# Not scanned: .beads, docs (ADR history, workflow examples, the id map) and lock files.
# Run from the repository root (or from any git work tree: the scan is of that tree).
set -euo pipefail

archive=.beads/issues.jsonl
idmap=docs/beads-id-map.json

ids=""
[ -f "$archive" ] && ids=$(grep -o '"id":"critical-[a-z0-9]*' "$archive" | sed 's/.*"critical-//' || true)
[ -f "$idmap" ] && ids="$ids"$'\n'$(grep -oE 'critical-[a-z0-9]+' "$idmap" | sed 's/^critical-//' || true)
ids=$(printf '%s\n' "$ids" | sed '/^$/d' | sort -u | paste -sd'|')

pattern="\bCRI-[0-9]+\b"
if [ -n "$ids" ]; then
    pattern="$pattern|\bcritical-($ids)\b|\bcritical-[a-z0-9]{3}\.[0-9]+\b|\b($ids)\.[0-9]+\b"
else
    echo "warning: no old bead ids ($archive and $idmap both missing or empty), checking CRI-<n> only" >&2
fi

if git grep -nIE "$pattern" -- . ':!.beads' ':!docs' ':!api/go.sum' ':!web/package-lock.json'; then
    echo "error: tracker ids above. Say what the code does; the work item goes in the commit message."
    exit 1
fi
echo "--- No tracker ids ---"
