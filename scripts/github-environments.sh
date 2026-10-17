#!/usr/bin/env bash
# Creates the GitHub environments of the release flow and their FLY_API_TOKEN secrets (ADR 0024 step
# 10, docs/deploy.md "GitHub"): `staging`, and `production` deployable from tags `v*` only. Each
# FLY_API_TOKEN is a new deploy token scoped to its one fly app, piped from `fly tokens create` into
# `gh secret set`: it is never printed nor written to a file.
#
#   scripts/github-environments.sh [--no-tokens]
#
# Needs admin rights on the repository (gh) and fly credentials. Rerunnable: the environments and the
# tag rule are kept; every run without --no-tokens creates new deploy tokens and replaces the secrets
# (the old tokens stay valid until revoked: `fly tokens list -a <app>`, `fly tokens revoke <id>`).
# RULEBOOKS_DEPLOY_KEY is a user step (docs/deploy.md), never set here.
set -euo pipefail

tokens=1
[ "${1:-}" = --no-tokens ] && tokens=0
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"

# environment <name> <fly app> [json body]
environment() {
    local env="$1" app="$2" body="${3:-}"
    if [ -n "$body" ]; then
        gh api -X PUT "repos/$repo/environments/$env" --input - <<<"$body" >/dev/null
    else
        gh api -X PUT "repos/$repo/environments/$env" >/dev/null
    fi
    echo "github-environments: $repo environment $env"
    if [ "$tokens" = 1 ]; then
        # Only a token line goes on (fly's tokens start "FlyV1 "): anything else stops the run unseen.
        "$fly" tokens create deploy -a "$app" -n "github-$env" \
            | { read -r t; [[ "$t" == "FlyV1 "* ]] || { echo "github-environments: unexpected fly output, nothing set" >&2; exit 1; }; printf '%s' "$t"; } \
            | gh secret set FLY_API_TOKEN --env "$env" --repo "$repo"
        echo "github-environments: $env FLY_API_TOKEN set from a new deploy token of $app"
    fi
}

environment staging critical-staging
environment production critical-prod \
    '{"deployment_branch_policy": {"protected_branches": false, "custom_branch_policies": true}}'

policies="repos/$repo/environments/production/deployment-branch-policies"
if ! gh api "$policies" --jq '.branch_policies[] | select(.type == "tag" and .name == "v*") | .id' | grep -q .; then
    gh api -X POST "$policies" -f name='v*' -f type=tag >/dev/null
fi
echo "github-environments: production deployable from: $(gh api "$policies" --jq '[.branch_policies[] | "\(.type) \(.name)"] | join(", ")')"
for env in staging production; do
    echo "github-environments: $env secrets: $(gh secret list --env "$env" --repo "$repo" --json name --jq '[.[].name] | join(", ")')"
done
