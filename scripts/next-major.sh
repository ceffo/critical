#!/usr/bin/env bash
# The manual major version (ADR 0024 step 3, docs/deploy.md "A major version"): `just major` runs the Major
# workflow, which moves the marker tag `next-major` to main's head. The next staging release that happens
# (scripts/release-version.sh) is then vN+1.0.0 instead of the computed bump; only that one, because the
# marker is then no longer after the last version tag. The marker never makes a release by itself.
#
#   scripts/next-major.sh mark <full commit sha>   # the workflow: moves the marker (gh, GITHUB_REPOSITORY)
#   scripts/next-major.sh preview < tag names      # `just major`: what the next release will be
#
# mark refuses when the commit is not after the highest vX.Y.Z tag (nothing has landed since the last version,
# or a newer release exists): the marker would be ignored. It needs the history and the tags (fetch-depth 0).
# The tag `next-major` is not a `v*` tag and has no ruleset (GitHub refuses the Actions-app bypass on a personal
# repository, scripts/github-tag-ruleset.sh): the repository's only collaborator is its owner.
# preview reads the tag names (one per line, `git ls-remote --tags --refs origin 'v*'` cut to the name) and
# prints the next major after the highest vX.Y.Z. Prints no secret.
set -euo pipefail

usage="usage: next-major.sh mark <full commit sha> | preview < tag names"
semver='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
die() { echo "next-major: $*" >&2; exit 1; }

case "${1:-}" in
    mark)
        [ $# = 2 ] || die "$usage"
        sha="$2"
        [[ "$sha" =~ ^[0-9a-f]{40}$ ]] || die "the commit must be a full sha, got '$sha'"
        git rev-parse -q --verify "$sha^{commit}" >/dev/null || die "unknown commit $sha"
        highest="$(git tag -l 'v*' | grep -E "$semver" | sort -V | tail -n 1 || true)"
        [ -n "$highest" ] || die "no vMAJOR.MINOR.PATCH tag: the first version is tagged by hand"
        [[ "$highest" =~ $semver ]]
        major="${BASH_REMATCH[1]}"
        highest_sha="$(git rev-parse "$highest^{commit}")"
        git merge-base --is-ancestor "$highest_sha" "$sha" \
            || die "a newer release exists: $highest, on $highest_sha, is not before $sha, so nothing is marked"
        [ "$highest_sha" != "$sha" ] \
            || die "$sha is $highest itself: no commit has landed since the last version, so a marker here would be ignored. Run it again once a commit has landed on main"
        repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}"
        if gh api "repos/$repo/git/ref/tags/next-major" >/dev/null 2>&1; then
            jq -n --arg sha "$sha" '{sha: $sha, force: true}' \
                | gh api -X PATCH "repos/$repo/git/refs/tags/next-major" --input - >/dev/null \
                || die "GitHub refused to move next-major (does the workflow token have contents: write?)"
        else
            jq -n --arg sha "$sha" '{ref: "refs/tags/next-major", sha: $sha}' \
                | gh api -X POST "repos/$repo/git/refs" --input - >/dev/null \
                || die "GitHub refused to create next-major (does the workflow token have contents: write?)"
        fi
        echo "::notice title=Major version::next-major is on $sha: the next release that happens is $((major + 1)).0.0 (after $highest). A range that makes no release (docs/test/chore only, no-release) waits; the marker needs no deleting."
        ;;
    preview)
        [ $# = 1 ] || die "$usage"
        highest="$(grep -E "$semver" | sort -V | tail -n 1 || true)"
        [ -n "$highest" ] || die "no vMAJOR.MINOR.PATCH tag on the remote: the first version is tagged by hand"
        [[ "$highest" =~ $semver ]]
        echo "the next release will be $((BASH_REMATCH[1] + 1)).0.0 (after $highest), once a commit that makes a release lands on main; docs/test/chore only waits"
        ;;
    *) die "$usage" ;;
esac
