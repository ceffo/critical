#!/usr/bin/env bash
# Creates (or updates) the repository's tag rulesets (ADR 0024 step 10.2, docs/deploy.md "GitHub"). On `v*`:
#   - "release tags": only a repository admin may move (update) or delete a tag `v*`;
#   - "release tag creation": only a repository admin and the GitHub Actions app (the release job's
#     workflow token, which creates each version's tag once staging passed, scripts/version-tag.sh) may
#     create one.
# And on the marker tag `next-major` (the Major workflow, scripts/next-major.sh; not a version, so not `v*`):
#   - "next-major marker": only a repository admin and the GitHub Actions app may create, move or delete it,
#     so a collaborator cannot make the next release a major by pushing it.
# Two rulesets on `v*` because a bypass actor bypasses every rule of its ruleset: the workflow may create a version
# tag, never move or delete one.
#
# Why restrict the `v*` tags: a `v*` tag selects what the Promote workflow runs AS IT IS IN THE TAGGED COMMIT, in the
# environment `production` (whose rule accepts any `v*` tag), with the production deploy token. The
# on-main guard lives in that commit's own scripts, so a tag on a commit that edited it out would deploy
# anything. The environment's tag rule cannot prevent this; restricting who may make the tag does.
#
#   scripts/github-tag-ruleset.sh [--dry-run]
#
# Needs admin rights on the repository (gh). Rerunnable: each ruleset is found by name and replaced.
# --dry-run prints the rulesets and changes nothing.
set -euo pipefail

# actor_id 5 is the built-in repository role "admin"; 15368 is the GitHub Actions app (the identity of a
# workflow's GITHUB_TOKEN).
ruleset() {
    jq -n --arg name "$1" --argjson rules "$2" --argjson bypass "$3" --arg ref "${4:-refs/tags/v*}" '{
        name: $name,
        target: "tag",
        enforcement: "active",
        conditions: {ref_name: {include: [$ref], exclude: []}},
        rules: $rules,
        bypass_actors: $bypass
    }'
}
admin='{"actor_id": 5, "actor_type": "RepositoryRole", "bypass_mode": "always"}'
actions='{"actor_id": 15368, "actor_type": "Integration", "bypass_mode": "always"}'
names=("release tags" "release tag creation" "next-major marker")
bodies=(
    "$(ruleset "release tags" '[{"type": "update"}, {"type": "deletion"}]' "[$admin]")"
    "$(ruleset "release tag creation" '[{"type": "creation"}]' "[$admin, $actions]")"
    "$(ruleset "next-major marker" '[{"type": "creation"}, {"type": "update"}, {"type": "deletion"}]' "[$admin, $actions]" refs/tags/next-major)"
)

if [ "${1:-}" = --dry-run ]; then
    printf '%s\n' "${bodies[@]}"
    exit 0
fi
[ $# -eq 0 ] || { echo "usage: github-tag-ruleset.sh [--dry-run]" >&2; exit 2; }

repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
for i in "${!names[@]}"; do
    name="${names[$i]}"
    id="$(gh api "repos/$repo/rulesets?includes_parents=false" --jq ".[] | select(.name == \"$name\") | .id")"
    if [ -n "$id" ]; then
        gh api -X PUT "repos/$repo/rulesets/$id" --input - <<<"${bodies[$i]}" >/dev/null
        echo "github-tag-ruleset: $repo ruleset '$name' ($id) updated"
    else
        id="$(gh api -X POST "repos/$repo/rulesets" --input - <<<"${bodies[$i]}" --jq .id)"
        echo "github-tag-ruleset: $repo ruleset '$name' ($id) created"
    fi
    gh api "repos/$repo/rulesets/$id" \
        --jq '"github-tag-ruleset: \(.name): \(.enforcement), \(.conditions.ref_name.include | join(",")): \([.rules[].type] | join(", ")); bypass: \([.bypass_actors[] | "\(.actor_type) \(.actor_id)"] | join(", "))"'
done
