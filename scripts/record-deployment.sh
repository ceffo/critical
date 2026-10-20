#!/usr/bin/env bash
# Records a deployment as a GitHub deployment (ADR 0024 steps 3 and 4), so the promotion can check "staging ran
# this sha" and compare the digest, and `just promote` can tell which version production runs. Staging (default):
# the environment `staging`, a deployment with task `deploy-staging`, ref the full commit sha and payload {"commit","digest","version"}
# (version: the release version X.Y.Z the image was built as, empty for an image without one),
# then a status `success` or `failure` carrying the digest in its description. Needs `gh` with GH_TOKEN
# (permission `deployments: write`) and GITHUB_REPOSITORY.
#
#   scripts/record-deployment.sh [--production] <full commit sha> success|failure [digest] [version]
#
# --production: the same record for the environment `production`, task `deploy-production`, written by
# promote.sh once the deploy and its smoke check passed (a `success` needs the version too); nothing is
# written for a promotion that failed before that.
#
# GitHub also creates a deployment of its own for every job that names the environment, with the
# workflow's ref; only the ones with task `deploy-staging` are a staging release. Those deployments
# turn this one `inactive` soon after (a newer deployment of the environment supersedes it), so `success`
# stays in its status history but is not its latest status: how the promotion reads it is in
# docs/deploy.md ("Staging deploy", "Record"). This status itself leaves other deployments alone.
set -euo pipefail

env=staging
task=deploy-staging
if [ "${1:-}" = --production ]; then env=production; task=deploy-production; shift; fi
usage="usage: record-deployment.sh [--production] <full commit sha> success|failure [digest] [version]"
sha="${1:?$usage}"
state="${2:?$usage}"
digest="${3:-}"
version="${4:-}"
[ -z "$version" ] || [[ "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] \
    || { echo "record-deployment: the version must be MAJOR.MINOR.PATCH, got '$version'" >&2; exit 2; }
[[ "$sha" =~ ^[0-9a-f]{40}$ ]] || { echo "record-deployment: the commit must be a full sha, got '$sha'" >&2; exit 2; }
case "$state" in
    success) { [ "$env" != production ] || [ -n "$version" ]; } || { echo "record-deployment: a production success needs the version" >&2; exit 2; }
        [[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]] || { echo "record-deployment: a success needs the image digest" >&2; exit 2; } ;;
    failure) ;;
    *) echo "$usage" >&2; exit 2 ;;
esac
repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}"
log_url=""
[ -z "${GITHUB_RUN_ID:-}" ] || log_url="${GITHUB_SERVER_URL:-https://github.com}/$repo/actions/runs/$GITHUB_RUN_ID"

id="$(jq -n --arg sha "$sha" --arg digest "$digest" --arg version "$version" --arg env "$env" --arg task "$task" '{
        ref: $sha, environment: $env, task: $task, auto_merge: false, required_contexts: [],
        description: "\($env) deploy of \($sha)", payload: {commit: $sha, digest: $digest, version: $version}}' \
    | gh api -X POST "repos/$repo/deployments" --input - --jq .id)"
[[ "$id" =~ ^[0-9]+$ ]] || { echo "record-deployment: no deployment id from GitHub" >&2; exit 1; }
description="${digest:-smoke or deploy failed}"
jq -n --arg state "$state" --arg d "$description" --arg url "$log_url" --arg env "$env" \
    '{state: $state, environment: $env, description: $d, log_url: $url, auto_inactive: false}' \
    | gh api -X POST "repos/$repo/deployments/$id/statuses" --input - >/dev/null
echo "record-deployment: $env deployment $id of $sha${version:+ ($version)}: $state ${digest}"
