#!/usr/bin/env bash
# The promotion's guards (ADR 0024 step 4.1): what must hold before anything is written for a release tag.
# Each fails with a message saying what is wrong; nothing here changes anything.
#
#   scripts/release-guards.sh tag <tag>             the tag is vMAJOR.MINOR.PATCH
#   scripts/release-guards.sh on-main <commit>      the commit is on origin/main (fetched by the caller)
#   scripts/release-guards.sh staging <full sha> [X.Y.Z]
#                                                   staging ran and passed that sha (as that version): prints
#                                                   the digest it ran
#   scripts/release-guards.sh image <image ref>     the registry holds the image (docker logged in to it)
#   scripts/release-guards.sh all <tag>             the four in that order, for the tag's commit; prints
#                                                   `sha=<full sha>` and `digest=sha256:...`
#   scripts/release-guards.sh resolve [version]     the version to promote (X.Y.Z or vX.Y.Z; none: the
#                                                   newest that passed staging); prints `tag=vX.Y.Z`,
#                                                   `sha=<full sha>` and `digest=sha256:...`
#   scripts/release-guards.sh candidates           what `just promote` offers: prints `staging=<X.Y.Z|unknown>`,
#                                                   `production=<X.Y.Z|unknown>` and one `version=X.Y.Z` per
#                                                   candidate, newest first (see below)
#
# `candidates` and `resolve` without a version are one implementation (`passed_versions`): the picker never
# offers a version the workflow would refuse. Candidates are the `v*` versions on main that passed staging
# as that version, above the version production runs (its latest successful `deploy-production` record, the
# same shape as the staging one: payload {commit, digest, version}, written by promote.sh once the smoke
# check passed) and up to the version staging runs (its latest successful `deploy-staging` record), at most
# 20. Production unknown (no record yet, before the first promotion after the record existed): all staged
# versions up to staging's, `production=unknown`. Staging unknown, or a failed read: exit 3, nothing offered.
# Production at or above staging's version: no candidate (a rollback is `just promote X.Y.Z`).
#
# `resolve` is what the Promote workflow runs before anything else, without any secret (docs/deploy.md,
# "Promotion"). With a version: it must be X.Y.Z or vX.Y.Z, its tag must exist, be on main and have passed
# staging as that version (the `staging` guard), or it is refused. Without one: the highest `v*` tag (by
# MAJOR, MINOR, PATCH as numbers; a tag not in that format is never a version) whose commit is on main and
# passed staging as that version; a higher tag that did not is skipped, saying why. Only a definite no skips a
# tag (off main; its records read and none passed): a failed API or git read stops with exit 3, never falling
# back to an older version. promote.sh, told the version was not asked (PROMOTE_NEWEST=true), also refuses
# one older than what production runs. The image check is
# promote.sh's (`all`), which needs the registry login.
#
# `staging` reads the GitHub deployment record of the staging deploy as docs/deploy.md ("Staging deploy",
# "Record") says: deployments of the environment `staging` with task `deploy-staging` and that sha; among
# those whose status HISTORY holds `success` (the latest status turns `inactive` when a newer deployment
# supersedes it), the newest by created_at; its payload.digest. With a version (`all` passes the tag's), only
# those whose payload.version is that version count: one commit can carry two versions (a forced release of
# a tagged head, docs/deploy.md "A new catalogue version"), each with its own image. When none has it, the
# newest successful one that records NO version counts (deploys from before versioned releases, v0.1.0);
# a deploy recorded as another version never does. Needs `gh` (GH_TOKEN with deployments
# read) and GITHUB_REPOSITORY (default: the repository gh sees). `image` uses `docker manifest inspect`.
# The staging app (whose registry repository holds the release images) is read from deploy/fly.staging.toml.
set -euo pipefail

usage="usage: release-guards.sh tag <tag> | on-main <commit> | staging <sha> [version] | image <ref> | all <tag> | resolve [version] | candidates"
here="$(cd "$(dirname "$0")" && pwd)"
fail() { echo "release-guards: FAIL: $*" >&2; exit 1; }
# A failure that is no answer about the version (an API or git read that failed, a broken record): exit 3, so
# `resolve` stops instead of skipping to an older version.
abort() { echo "release-guards: FAIL: $*" >&2; exit 3; }

guard_tag() {
    [[ "$1" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] \
        || fail "tag '$1' is not vMAJOR.MINOR.PATCH (e.g. v1.2.3): nothing is promoted"
}

guard_on_main() {
    local status=0
    git rev-parse -q --verify origin/main >/dev/null || abort "origin/main is not fetched"
    git merge-base --is-ancestor "$1" origin/main || status=$?
    [ "$status" = 0 ] && return
    [ "$status" = 1 ] || abort "cannot tell whether $1 is on main (git merge-base exited $status)"
    fail "commit $1 is not on main: only a main commit that passed staging is promoted"
}

# Prints the digest staging ran and smoke-checked for the sha.
guard_staging() {
    local sha="$1" want="${2:-}" repo list best="" best_at="" bare="" bare_at="" id at digest version passed
    [[ "$sha" =~ ^[0-9a-f]{40}$ ]] || fail "staging: the commit must be a full sha, got '$sha'"
    repo="${GITHUB_REPOSITORY:-}"
    [ -n "$repo" ] || repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)" || abort "staging: cannot tell the repository"
    # payload is the object the staging deploy recorded (GitHub returns a string for a string payload).
    list="$(gh api "repos/$repo/deployments?environment=staging&task=deploy-staging&sha=$sha&per_page=100" \
        --jq '.[] | (.payload | if type == "string" then (fromjson? // {}) else . end) as $p
              | "\(.id) \(.created_at) \($p.digest // "-") \(($p.version // "") | if . == "" then "-" else . end)"')" \
        || abort "staging: cannot read the deployments of $repo (GH_TOKEN needs deployments: read; or the API failed): nothing is promoted"
    while read -r id at digest version; do
        [ -n "$id" ] || continue
        passed="$(gh api "repos/$repo/deployments/$id/statuses?per_page=100" --jq '[.[].state] | index("success") != null')" \
            || abort "staging: cannot read the statuses of deployment $id of $sha (the API failed): nothing is promoted"
        case "$passed" in
            true) ;;
            false) continue ;;
            *) abort "staging: unexpected statuses of deployment $id of $sha: '$passed'" ;;
        esac
        if [[ -z "$want" || "$version" == "$want" ]]; then
            if [[ -z "$best_at" || "$at" > "$best_at" ]]; then best="$digest"; best_at="$at"; fi
        elif [ "$version" = - ]; then
            if [[ -z "$bare_at" || "$at" > "$bare_at" ]]; then bare="$digest"; bare_at="$at"; fi
        fi
    done <<<"$list"
    if [ -z "$best_at" ] && [ -n "$bare_at" ]; then best="$bare"; best_at="$bare_at"; fi
    [ -n "$best_at" ] || fail "staging has no successful deploy of $sha${want:+ as $want} (docs/deploy.md, \"Staging deploy\"): merge it to main and wait for the staging smoke check, or redeploy it there first"
    [[ "$best" =~ ^sha256:[0-9a-f]{64}$ ]] || abort "the staging deployment of $sha records no image digest"
    echo "$best"
}

guard_image() {
    docker manifest inspect "$1" >/dev/null 2>&1 \
        || fail "the registry does not hold $1, or this token cannot read it (docs/deploy.md, \"Promotion\")"
}

semver_re='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
# vgt A B: version A is higher than version B.
vgt() { [ "$1" != "$2" ] && [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -n 1)" = "$1" ]; }

# deploy_current <environment> <task>: the version the latest successful deployment record of the environment
# carries (its status history holds `success`, newest by created_at first); empty when there is none, or it
# records no version. A failed read aborts.
deploy_current() {
    local env="$1" task="$2" repo list id at version passed
    repo="${GITHUB_REPOSITORY:-}"
    [ -n "$repo" ] || repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)" || abort "cannot tell the repository"
    list="$(gh api "repos/$repo/deployments?environment=$env&task=$task&per_page=100" \
        --jq '[.[] | (.payload | if type == "string" then (fromjson? // {}) else . end) as $p
               | {id, created_at, v: ($p.version // "")}] | sort_by(.created_at) | reverse | .[]
               | "\(.id) \(.created_at) \(.v | if . == "" then "-" else . end)"')" \
        || abort "cannot read the $env deployments of $repo (GH_TOKEN needs deployments: read; or the API failed): nothing is offered"
    while read -r id at version; do
        [ -n "$id" ] || continue
        passed="$(gh api "repos/$repo/deployments/$id/statuses?per_page=100" --jq '[.[].state] | index("success") != null')" \
            || abort "cannot read the statuses of $env deployment $id (the API failed)"
        case "$passed" in
            true) [ "$version" = - ] || echo "$version"; return 0 ;;
            false) ;;
            *) abort "unexpected statuses of $env deployment $id: '$passed'" ;;
        esac
    done <<<"$list"
}

# passed_versions <ceiling> <floor> <limit>: the versions that passed staging as that version, newest first,
# one `X.Y.Z <sha> <digest>` line each: only the `v*` tags (by number) not above the ceiling and above the
# floor (either may be empty), on main, at most <limit>. A tag that is a definite no (off main; records read,
# none passed) is skipped, saying why on stderr; any other failure stops with exit 3 (skipping on it would
# offer an older version than the newest that is staged).
passed_versions() {
    local ceiling="$1" floor="$2" limit="$3" tags v tag sha digest err status n=0
    tags="$(git tag -l 'v*' | grep -Ex 'v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)' \
        | sed 's/^v//' | sort -t. -k1,1nr -k2,2nr -k3,3nr)" || true
    err="$(mktemp)"
    for v in $tags; do
        [ -z "$ceiling" ] || ! vgt "$v" "$ceiling" || continue
        [ -z "$floor" ] || vgt "$v" "$floor" || break
        tag="v$v"
        sha="$(git rev-parse -q --verify "refs/tags/$tag^{commit}")" || { rm -f "$err"; abort "cannot read the commit of $tag"; }
        status=0
        (guard_on_main "$sha") 2>"$err" || status=$?
        if [ "$status" = 0 ]; then
            digest="$(guard_staging "$sha" "$v" 2>"$err")" || status=$?
        fi
        if [ "$status" = 0 ]; then
            echo "$v $sha $digest"
            n=$((n + 1))
            [ "$n" -lt "$limit" ] || break
            continue
        fi
        [ "$status" = 1 ] || { cat "$err" >&2; rm -f "$err"; abort "cannot tell whether $tag, the newest version not yet ruled out, passed staging: nothing is promoted (run it again)"; }
        echo "release-guards: skipping $tag: $(cat "$err")" >&2
    done
    rm -f "$err"
}

candidates() {
    local staging production list
    staging="$(deploy_current staging deploy-staging)"
    [[ "$staging" =~ $semver_re ]] || abort "staging has no recorded version (no successful deploy-staging record with one): nothing is offered"
    production="$(deploy_current production deploy-production)"
    [[ "$production" =~ $semver_re ]] || production=""
    echo "staging=$staging"
    echo "production=${production:-unknown}"
    if [ -n "$production" ] && ! vgt "$staging" "$production"; then return 0; fi
    list="$(passed_versions "$staging" "$production" 20)" || return $?
    sed -n 's/^\([^ ]*\) .*/version=\1/p' <<<"$list"
}

# Prints tag=, sha= and digest= of the version to promote: the one asked, or the newest that passed staging.
resolve() {
    local want="$1" tag sha digest line v
    if [ -n "$want" ]; then
        tag="v${want#v}"
        [[ "$tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] \
            || fail "version '$want' is not X.Y.Z or vX.Y.Z (e.g. 1.2.3): nothing is promoted"
        sha="$(git rev-parse -q --verify "refs/tags/$tag^{commit}")" \
            || fail "there is no version $tag (no such tag): nothing is promoted. The versions are the v* tags CI made (docs/deploy.md, \"Release version\")"
        guard_on_main "$sha"
        digest="$(guard_staging "$sha" "${tag#v}")"
    else
        line="$(passed_versions "" "" 1)" || exit $?
        read -r v sha digest <<<"$line"
        tag="v$v"
        [ -n "$digest" ] || fail "no v* version is on main with a passed staging deploy: nothing is promoted (docs/deploy.md, \"Promotion\")"
    fi
    echo "tag=$tag"
    echo "sha=$sha"
    echo "digest=$digest"
}

cmd="${1:-}"
arg="${2:-}"
if [ "$cmd" = resolve ]; then resolve "$arg"; exit 0; fi
if [ "$cmd" = candidates ]; then candidates; exit $?; fi
[ -n "$arg" ] || { echo "$usage" >&2; exit 2; }
case "$cmd" in
    tag) guard_tag "$arg" ;;
    on-main) guard_on_main "$arg" ;;
    staging) guard_staging "$arg" "${3:-}" ;;
    image) guard_image "$arg" ;;
    all)
        guard_tag "$arg"
        sha="$(git rev-parse -q --verify "refs/tags/$arg^{commit}")" || fail "no tag $arg in this checkout"
        guard_on_main "$sha"
        digest="$(guard_staging "$sha" "${arg#v}")"
        app="$(sed -n 's/^app = "\(.*\)"$/\1/p' "$here/../deploy/fly.staging.toml")"
        [ -n "$app" ] || fail "no app in deploy/fly.staging.toml"
        guard_image "registry.fly.io/$app@$digest"
        echo "sha=$sha"
        echo "digest=$digest"
        ;;
    *) echo "$usage" >&2; exit 2 ;;
esac
