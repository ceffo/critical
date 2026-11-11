#!/usr/bin/env bash
# Decides which of CI's slow jobs (e2e, dev-smoke, image) are skipped, so a README tweak does not wait for them
# and a change that cannot alter what a browser sees does not pay for the e2e suite (ADR 0048, docs/agent-workflow.md,
# "CI and PRs"). Prints skip_slow=true|false and skip_e2e=true|false with their reasons, and appends both to
# $GITHUB_OUTPUT when set. validate and generated never depend on it.
#
# Environment:
#   EVENT   the GitHub event name (pull_request, push, ...).
#   ACTION  the pull_request action (opened, synchronize, labeled, ...), read on pull_request events only.
#   BASE    the PR's base commit (read on pull_request events only).
#   HEAD    the PR's own head commit (default HEAD). Not the merge commit CI checks out: that one carries every
#           commit main gained since the PR branched, and none of those is the PR's change.
#   BEFORE  the PR's head before this push (github.event.before, on a synchronize action only).
#   LABELS  comma-separated label names of the PR (read on pull_request events only).
#
# skip_slow (dev-smoke, image, and the base of skip_e2e) is true when:
#   - the PR carries the label ci-skip; or
#   - every file the PR changes is documentation: *.md anywhere, docs/**. The PR's changes are the diff from
#     the merge base of BASE and HEAD (BASE...HEAD), so a base that moved on does not count.
# skip_e2e is true when skip_slow is, and also when:
#   - the push only updated the branch: the PR's own diff before the push (BASE...BEFORE) and now (BASE...HEAD)
#     have the same patch id, so the PR keeps its last e2e result; or
#   - none of the changed files is on an e2e path (e2e_path below): the web app, the route and handler layer of
#     the API, the API fixture, the e2e stack and the CI workflow. Unit-test-only files do not count.
# Only a pull request skips. A push to main (and a dispatched run) always runs everything: Release deploys
# the commit that run gates, and a push's range can hold earlier commits whose own run was cancelled.
# Every decision defaults to running: no changed file found, an unreadable range or an unreadable BEFORE.
set -euo pipefail

event="${EVENT:-}"
action="${ACTION:-}"
base="${BASE:-}"
head="${HEAD:-HEAD}"
before="${BEFORE:-}"
labels="${LABELS:-}"

# emit <skip_slow> <slow reason> <skip_e2e> <e2e reason>
emit() {
    echo "skip_slow=$1 ($2)"
    echo "skip_e2e=$3 ($4)"
    if [ -n "${GITHUB_OUTPUT:-}" ]; then
        echo "skip_slow=$1" >> "$GITHUB_OUTPUT"
        echo "skip_e2e=$3" >> "$GITHUB_OUTPUT"
    fi
    exit 0
}

# e2e_path <file>: succeeds when a change to the file can alter what the e2e suite sees.
e2e_path() {
    case "$1" in
        # Not e2e: unit tests, docs, and the capture tooling that runs outside `just e2e`.
        *.md | *_test.go | web/*.test.ts | web/*.test.tsx | web/design/* | web/ui-review/* | web/readme-shots/* \
            | web/playwright.ui-review.config.ts | web/playwright.readme-shots.config.ts) return 1 ;;
        web/* | api/testdata/* | justfile | .github/workflows/ci.yml | .github/actions/* \
            | scripts/ci-install-chromium-deps.sh) return 0 ;;
        # The API's routes and handlers, their wire types and error mapping, and the server's start-up.
        api/internal/rest/* | api/internal/render/* | api/internal/composition/* | api/internal/cmd/* | api/cmd/*) return 0 ;;
    esac
    return 1
}

[ "$event" = pull_request ] || emit false "only a pull request skips the slow jobs" false "only a pull request skips e2e"

IFS=',' read -ra names <<< "$labels"
for name in "${names[@]}"; do
    [ "$name" = ci-skip ] && emit true "the pull request has the ci-skip label" true "the pull request has the ci-skip label"
done

if [ -z "$base" ] || [[ "$base" =~ ^0+$ ]]; then
    emit false "no base commit to compare with" false "no base commit to compare with"
fi
if ! files="$(git diff --name-only "$base...$head" -- 2>/dev/null)"; then
    emit false "cannot list the files the pull request changes" false "cannot list the files the pull request changes"
fi
if [ -z "$files" ]; then
    emit false "no changed file found" false "no changed file found"
fi

docs_only=true
while IFS= read -r f; do
    case "$f" in
        *.md | docs/*) ;;
        *) docs_only=false; slow_reason="$f is not documentation"; break ;;
    esac
done <<< "$files"
if [ "$docs_only" = true ]; then
    emit true "only documentation changed" true "only documentation changed"
fi

# A branch update (main merged into the PR) changes the checked-out commit but not the PR's own diff.
if [ "$action" = synchronize ] && [ -n "$before" ] && ! [[ "$before" =~ ^0+$ ]]; then
    now_id="$(git diff "$base...$head" -- 2>/dev/null | git patch-id --stable 2>/dev/null | cut -d' ' -f1 || true)"
    was_id="$(git diff "$base...$before" -- 2>/dev/null | git patch-id --stable 2>/dev/null | cut -d' ' -f1 || true)"
    if [ -n "$now_id" ] && [ "$now_id" = "$was_id" ]; then
        emit false "$slow_reason" true "the push only updated the branch: the pull request's own diff is unchanged"
    fi
fi

while IFS= read -r f; do
    if e2e_path "$f"; then
        emit false "$slow_reason" false "$f is on an e2e path"
    fi
done <<< "$files"
emit false "$slow_reason" true "no changed file is on an e2e path"
