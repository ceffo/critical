#!/usr/bin/env bash
# `just release <version>`: tags the head of main as a release and pushes the tag, which starts the
# promotion to production (.github/workflows/promote.yml, docs/deploy.md "Promotion").
#
#   scripts/release-tag.sh <version>      e.g. 1.2.3 or v1.2.3 (tag v1.2.3)
#
# Refuses, before tagging anything: a version that is not MAJOR.MINOR.PATCH, one that is not greater than
# the highest v* tag, a branch other than main, a dirty tree, a main that is not origin/main's head (after
# a fetch), and a commit staging has not run and passed (scripts/release-guards.sh staging, which needs
# gh). Creates an annotated tag and pushes it to origin. RELEASE_REMOTE names another remote (tests).
set -euo pipefail

usage="usage: release-tag.sh <MAJOR.MINOR.PATCH>"
version="${1:?$usage}"
here="$(cd "$(dirname "$0")" && pwd)"
remote="${RELEASE_REMOTE:-origin}"
fail() { echo "release: refused: $*" >&2; exit 1; }

tag="v${version#v}"
[[ "$tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || fail "'$version' is not MAJOR.MINOR.PATCH (e.g. 1.2.3)"

branch="$(git symbolic-ref -q --short HEAD || true)"
[ "$branch" = main ] || fail "on '${branch:-a detached HEAD}', not main: git switch main && git pull"
[ -z "$(git status --porcelain)" ] || fail "the working tree is not clean: commit or stash first"
git fetch -q "$remote" main --tags
head="$(git rev-parse HEAD)"
[ "$head" = "$(git rev-parse "$remote/main")" ] || fail "main is not $remote/main's head: git pull (or push your commits through a PR)"

last="$(git tag -l 'v*' | grep -E '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' | sort -V | tail -n 1 || true)"
if [ -n "$last" ]; then
    [ "$(printf '%s\n%s\n' "$last" "$tag" | sort -V | tail -n 1)" = "$tag" ] && [ "$last" != "$tag" ] \
        || fail "$tag is not greater than the last release $last"
fi
! git rev-parse -q --verify "refs/tags/$tag" >/dev/null || fail "the tag $tag exists"

digest="$("$here/release-guards.sh" staging "$head")" || fail "staging has not passed $head yet (see above)"

git tag -a "$tag" -m "Release $tag" "$head"
git push -q "$remote" "refs/tags/$tag"
echo "release: $tag ($head, staging digest $digest) pushed${last:+, after $last}: the promotion runs in GitHub Actions (Promote)"
