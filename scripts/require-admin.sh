#!/usr/bin/env bash
# Who may promote (ADR 0024 step 10.2): every user named must be an admin of the repository, or it fails
# saying who is not. The Promote workflow runs it first, before any secret, with the user who started the
# run and the one who (re-)ran it (github.actor and github.triggering_actor): a re-run by a non-admin of an
# admin's run is refused too. Read-only: the API's permission of a collaborator, read with the workflow's own
# token (GH_TOKEN; its metadata permission is enough), never a secret.
#
#   scripts/require-admin.sh <user>...
#
# Needs `gh` and GITHUB_REPOSITORY (default: the repository gh sees). A user who is not a collaborator, or a
# failed API call, is refused like a non-admin.
set -euo pipefail

[ "$#" -gt 0 ] || { echo "usage: require-admin.sh <user>..." >&2; exit 2; }
repo="${GITHUB_REPOSITORY:-$(gh repo view --json nameWithOwner --jq .nameWithOwner)}"
for user in "$@"; do
    [[ "$user" =~ ^[A-Za-z0-9][A-Za-z0-9-]*(\[bot\])?$ ]] \
        || { echo "require-admin: FAIL: '$user' is not a GitHub user name: nothing is promoted" >&2; exit 1; }
    permission="$(gh api "repos/$repo/collaborators/$user/permission" --jq .permission 2>/dev/null)" || permission=""
    if [ "$permission" != admin ]; then
        echo "::error title=Promotion::$user is not an admin of $repo (permission: ${permission:-unknown}): only a repository admin may promote (docs/deploy.md, \"Promotion\"). Nothing was promoted."
        echo "require-admin: FAIL: $user is not an admin of $repo" >&2
        exit 1
    fi
done
echo "require-admin: OK ($(printf '%s ' "$@" | sed 's/ $//') admin of $repo)"
