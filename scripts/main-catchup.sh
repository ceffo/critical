#!/usr/bin/env bash
# Gives main's head the CI run and the Release run a merge made with a workflow's GITHUB_TOKEN did not start
# (the Dependabot auto-merge): GitHub starts no workflow from an event that token caused, except
# workflow_dispatch, and a CI run dispatched that way does not start the Release workflow_run either
# (.github/workflows/main-catchup.yml runs this on a schedule; docs/deploy.md "How a release flows").
#
#   scripts/main-catchup.sh          needs GH_TOKEN and GITHUB_REPOSITORY (gh)
#   GRACE=<seconds>                  how long a green CI run waits for its own Release run (default 300)
#
# Looks at main's head only, and acts at most once per call:
#   - a CI run on it is queued or running: wait.
#   - no CI run on it finished green or red: dispatch CI (a cancelled run counts as none).
#   - CI is green, older than GRACE, and no Release run exists for the commit: dispatch Release for exactly
#     that commit (input `commit`: the release rules apply, nothing is forced, and Release re-checks the
#     green CI itself).
#   - anything else (red CI, a Release run already there): nothing; red main is the supervisor's.
# Idempotent: a second call right after finds the run it just started.
set -euo pipefail

repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}"
grace="${GRACE:-300}"
now="${NOW:-$(date +%s)}"

head="$(gh api "repos/$repo/commits/main" --jq .sha)"
ci="$(gh run list --repo "$repo" --workflow ci.yml --commit "$head" --limit 50 --json status,conclusion,event,updatedAt)"
release="$(gh run list --repo "$repo" --workflow release.yml --commit "$head" --limit 50 --json databaseId)"

# Only the runs that belong to main's head as a push or a dispatch (a pull request run is not main's).
main_ci='[.[] | select(.event == "push" or .event == "workflow_dispatch")]'

if jq -e "$main_ci | any(.[]; .status != \"completed\")" <<<"$ci" >/dev/null; then
    echo "CI is running on ${head:0:7}: waiting"
    exit 0
fi
if ! jq -e "$main_ci | any(.[]; .conclusion == \"success\" or .conclusion == \"failure\")" <<<"$ci" >/dev/null; then
    gh workflow run ci.yml --repo "$repo" --ref main
    echo "no CI run on ${head:0:7}: dispatched CI"
    exit 0
fi
if ! jq -e "$main_ci | any(.[]; .conclusion == \"success\")" <<<"$ci" >/dev/null; then
    echo "CI is red on ${head:0:7}: nothing to do here"
    exit 0
fi
if jq -e 'length > 0' <<<"$release" >/dev/null; then
    echo "${head:0:7} already has a Release run"
    exit 0
fi
done_at="$(jq -r "$main_ci | map(select(.conclusion == \"success\") | .updatedAt) | max" <<<"$ci")"
age=$((now - $(date -d "$done_at" +%s)))
if [ "$age" -lt "$grace" ]; then
    echo "CI went green ${age}s ago on ${head:0:7}: giving its own Release run ${grace}s to start"
    exit 0
fi
gh workflow run release.yml --repo "$repo" --ref main -f "commit=$head"
echo "CI is green on ${head:0:7} with no Release run: dispatched Release"
