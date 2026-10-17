#!/usr/bin/env bash
# Creates (or updates) the repository's tag ruleset "release tags" (ADR 0024 step 10.2, docs/deploy.md
# "GitHub"): only a repository admin may create, move or delete a tag `v*`.
#
# Why: a pushed `v*` tag runs promote.yml AS IT IS IN THE TAGGED COMMIT, in the environment `production`
# (whose rule accepts any `v*` tag), with the production deploy token. The on-main guard lives in that
# commit's own scripts, so a tag on a commit that edited it out would deploy anything. The environment's
# tag rule cannot prevent this; restricting who may create the tag does.
#
#   scripts/github-tag-ruleset.sh [--dry-run]
#
# Needs admin rights on the repository (gh). Rerunnable: the ruleset is found by name and replaced.
# --dry-run prints the ruleset and changes nothing. The promotion itself creates no tag (it creates the
# Release of an existing one, `gh release create --verify-tag`), so the workflow token is not a bypass actor.
set -euo pipefail

name="release tags"
# actor_id 5 is the built-in repository role "admin".
ruleset="$(jq -n --arg name "$name" '{
    name: $name,
    target: "tag",
    enforcement: "active",
    conditions: {ref_name: {include: ["refs/tags/v*"], exclude: []}},
    rules: [{type: "creation"}, {type: "update"}, {type: "deletion"}],
    bypass_actors: [{actor_id: 5, actor_type: "RepositoryRole", bypass_mode: "always"}]
}')"

if [ "${1:-}" = --dry-run ]; then
    echo "$ruleset"
    exit 0
fi
[ $# -eq 0 ] || { echo "usage: github-tag-ruleset.sh [--dry-run]" >&2; exit 2; }

repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
id="$(gh api "repos/$repo/rulesets?includes_parents=false" --jq ".[] | select(.name == \"$name\") | .id")"
if [ -n "$id" ]; then
    gh api -X PUT "repos/$repo/rulesets/$id" --input - <<<"$ruleset" >/dev/null
    echo "github-tag-ruleset: $repo ruleset '$name' ($id) updated"
else
    id="$(gh api -X POST "repos/$repo/rulesets" --input - <<<"$ruleset" --jq .id)"
    echo "github-tag-ruleset: $repo ruleset '$name' ($id) created"
fi
gh api "repos/$repo/rulesets/$id" \
    --jq '"github-tag-ruleset: \(.enforcement), \(.conditions.ref_name.include | join(",")): \([.rules[].type] | join(", ")); bypass: \([.bypass_actors[] | "\(.actor_type) \(.actor_id)"] | join(", "))"'
