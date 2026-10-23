#!/usr/bin/env bash
# Creates the GitHub environments of the release flow and their FLY_API_TOKEN secrets (ADR 0024 step
# 10, docs/deploy.md "GitHub"): `staging` and `production` each deployable from the branch `main` only (the
# promotion runs main's workflow, the version is its input; any other rule, such as the former `v*` tags of
# `production`, is removed). Each
# FLY_API_TOKEN is a new deploy token scoped to its one fly app, piped from `fly tokens create` into
# `gh secret set`: it is never printed nor written to a file.
#
#   scripts/github-environments.sh [--no-tokens]
#
# Needs admin rights on the repository (gh) and fly credentials. Rerunnable: the environments and the
# deployment rules are kept; every run without --no-tokens creates new deploy tokens and replaces the secrets
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
        # The token is captured and checked first (its first line, which starts "FlyV1 "), and
        # only then piped to gh: a failed or unexpected fly run sets nothing and keeps the old secret.
        # Neither the token nor fly's output is ever printed.
        local t
        if ! t="$("$fly" tokens create deploy -a "$app" -n "github-$env" 2>/dev/null)" \
            || t="${t%%$'\n'*}" && [[ "$t" != "FlyV1 "* ]]; then
            echo "github-environments: fly tokens create failed or printed no token line, $env FLY_API_TOKEN kept" >&2
            exit 1
        fi
        printf '%s' "$t" | gh secret set FLY_API_TOKEN --env "$env" --repo "$repo"
        unset t
        echo "github-environments: $env FLY_API_TOKEN set from a new deploy token of $app"
    fi
}

custom='{"deployment_branch_policy": {"protected_branches": false, "custom_branch_policies": true}}'
environment staging critical-hit-staging "$custom"
environment production critical-hit "$custom"

# policy <environment> <branch|tag> <name>: the one rule that environment accepts deployments from. It is
# added first, then every other rule of the environment is deleted, so no moment accepts nothing.
policy() {
    local url="repos/$repo/environments/$1/deployment-branch-policies" id existing
    # Captured, not piped into grep -q: an early exit would SIGPIPE gh and pipefail would call the rule missing.
    existing="$(gh api "$url" --jq ".branch_policies[] | select(.type == \"$2\" and .name == \"$3\") | .id")"
    if [ -z "$existing" ]; then
        gh api -X POST "$url" -f name="$3" -f type="$2" >/dev/null
    fi
    for id in $(gh api "$url" --jq ".branch_policies[] | select(.type != \"$2\" or .name != \"$3\") | .id"); do
        gh api -X DELETE "$url/$id" >/dev/null
    done
    echo "github-environments: $1 deployable from: $(gh api "$url" --jq '[.branch_policies[] | "\(.type) \(.name)"] | join(", ")')"
}
policy staging branch main
policy production branch main
for env in staging production; do
    echo "github-environments: $env secrets: $(gh secret list --env "$env" --repo "$repo" --json name --jq '[.[].name] | join(", ")')"
done
