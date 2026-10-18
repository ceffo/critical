#!/usr/bin/env bash
# Admin step for the Dependabot auto-merge (.github/workflows/dependabot-auto-merge.yml):
#   1. allows GitHub auto-merge on the repository (it is off by default);
#   2. makes every CI job a required status check of main: validate, e2e, image, dev-smoke and
#      "generated code is up to date". The `changes` job and the PR-title check stay NOT required.
# A job CI skips (docs-only change, `ci-skip`) counts as a success, so it never blocks a merge.
# The other branch-protection settings (review count, conversation resolution, admins, ...) are read back and
# written unchanged: the status-checks endpoint only replaces the contexts.
#
#   scripts/github-branch-protection.sh [--dry-run]
#
# Needs admin rights on the repository (gh). Rerunnable. --dry-run prints what it would send and changes nothing.
set -euo pipefail

contexts=("validate" "e2e" "image" "dev-smoke" "generated code is up to date")

body="$(printf '%s\n' "${contexts[@]}" | jq -R . | jq -s '{strict: false, contexts: .}')"

if [ "${1:-}" = --dry-run ]; then
    echo "PATCH repos/<repo> allow_auto_merge=true"
    echo "PATCH repos/<repo>/branches/main/protection/required_status_checks:"
    echo "$body"
    exit 0
fi
[ $# -eq 0 ] || { echo "usage: github-branch-protection.sh [--dry-run]" >&2; exit 2; }

repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
gh api -X PATCH "repos/$repo" -F allow_auto_merge=true --jq '"github-branch-protection: \(.full_name) allow_auto_merge=\(.allow_auto_merge)"'
gh api -X PATCH "repos/$repo/branches/main/protection/required_status_checks" --input - <<<"$body" \
    --jq '"github-branch-protection: main requires: \([.checks[].context] | join(", "))"'
