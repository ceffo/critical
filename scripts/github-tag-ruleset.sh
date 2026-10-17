#!/usr/bin/env bash
# Creates (or updates) the repository's tag ruleset (ADR 0024 step 10.2, docs/deploy.md "GitHub"). On `v*`:
#   - "release tags": only a repository admin may move (update) or delete a tag `v*`.
# Creating a tag is not restricted: GitHub refuses, on a personal-account repository, a ruleset that lets the
# GitHub Actions app (the release job's workflow token) bypass it (HTTP 422), so the earlier rulesets "release tag
# creation" (v* creation) and "next-major marker" (next-major) cannot exist here. The script removes them by name
# if they are present. Nothing is lost that matters: the promotion runs main's scripts with the version as an
# input, so a `v*` tag no longer selects the code that reaches production, and the repository's only
# collaborator is its owner.
#
# Why restrict moving and deleting the `v*` tags: they name the versions the promotion resolves (the newest one
# that passed staging, or the one asked). Moving or deleting one would change what a version means.
#
#   scripts/github-tag-ruleset.sh [--dry-run]
#
# Needs admin rights on the repository (gh). Rerunnable: the ruleset is found by name and replaced.
# --dry-run prints the ruleset (JSON on stdout) and the names it would remove (stderr), and changes nothing.
set -euo pipefail

# actor_id 5 is the built-in repository role "admin".
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
names=("release tags")
bodies=(
    "$(ruleset "release tags" '[{"type": "update"}, {"type": "deletion"}]' "[$admin]")"
)
# Rulesets an earlier version of this script made; GitHub refuses their Actions-app bypass here.
obsolete=("release tag creation" "next-major marker")

if [ "${1:-}" = --dry-run ]; then
    printf '%s\n' "${bodies[@]}"
    for name in "${obsolete[@]}"; do echo "github-tag-ruleset: would remove ruleset '$name' if present" >&2; done
    exit 0
fi
[ $# -eq 0 ] || { echo "usage: github-tag-ruleset.sh [--dry-run]" >&2; exit 2; }

repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
for name in "${obsolete[@]}"; do
    id="$(gh api "repos/$repo/rulesets?includes_parents=false" --jq ".[] | select(.name == \"$name\") | .id")"
    if [ -n "$id" ]; then
        gh api -X DELETE "repos/$repo/rulesets/$id" >/dev/null
        echo "github-tag-ruleset: $repo ruleset '$name' ($id) removed"
    fi
done
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
