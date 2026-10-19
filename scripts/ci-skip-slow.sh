#!/usr/bin/env bash
# Decides whether CI skips its slow jobs (e2e, dev-smoke, image), so a README tweak does not wait for them
# (docs/agent-workflow.md, "CI and PRs"). Prints skip_slow=true|false and the reason, and appends
# skip_slow to $GITHUB_OUTPUT when set. validate and generated never depend on it.
#
# Environment:
#   EVENT   the GitHub event name (pull_request, push, ...).
#   BASE    the commit to compare with (the PR's base, or the push's previous head).
#   HEAD    the commit to compare (default HEAD).
#   LABELS  comma-separated label names of the PR (read on pull_request events only).
#
# Skip when:
#   - the PR carries the label ci-skip (a pull_request event only: the push to main that merges it runs
#     everything, unless the path rule applies); or
#   - every changed file is documentation: *.md anywhere, docs/**.
# The decision defaults to running everything: no changed file found, an unreadable range (a new branch's
# zero BASE, a force-pushed-away commit) or a changed file that is not documentation.
set -euo pipefail

event="${EVENT:-}"
base="${BASE:-}"
head="${HEAD:-HEAD}"
labels="${LABELS:-}"

decide() {
    local skip="$1" reason="$2"
    echo "skip_slow=$skip ($reason)"
    if [ -n "${GITHUB_OUTPUT:-}" ]; then
        echo "skip_slow=$skip" >> "$GITHUB_OUTPUT"
    fi
    exit 0
}

if [ "$event" = pull_request ]; then
    IFS=',' read -ra names <<< "$labels"
    for name in "${names[@]}"; do
        [ "$name" = ci-skip ] && decide true "the pull request has the ci-skip label"
    done
fi

if [ -z "$base" ] || [[ "$base" =~ ^0+$ ]]; then
    decide false "no base commit to compare with"
fi
if ! files="$(git diff --name-only "$base" "$head" -- 2>/dev/null)"; then
    decide false "cannot list the files changed since $base"
fi
if [ -z "$files" ]; then
    decide false "no changed file found"
fi

while IFS= read -r f; do
    case "$f" in
        *.md | docs/*) ;;
        *) decide false "$f is not documentation" ;;
    esac
done <<< "$files"
decide true "only documentation changed"
