#!/usr/bin/env bash
# Builds the release image of the clean HEAD with the REAL catalogue and pushes that one build to each
# given fly app's repository in the private fly registry, as registry.fly.io/<app>:<full commit sha>
# (ADR 0024 steps 1.5, 3.1 and 7). Every app gets the same digest. Used by `just release-image` (the
# operator, catalogue from the local rulebooks clone) and by the release workflow (catalogue fetched
# with the deploy key, `scripts/release-catalogue.sh`).
#
#   scripts/release-image.sh <catalogue dir> <rulebooks commit> <app>...
#
# <catalogue dir> holds catalogue.json; it is passed as the build context `catalogue` and only enters
# the image. <rulebooks commit> is the critical-rulebooks commit it was taken from, recorded as the
# image label `critical.rulebooks.revision` (the commit is `org.opencontainers.image.revision`).
# VERSION (optional, X.Y.Z: the release workflow sets the version it computed, scripts/release-version.sh)
# is baked in as the build arg VERSION (reported by /api/health), recorded as the label
# `org.opencontainers.image.version` and pushed as a second tag registry.fly.io/<app>:v<version> (the one
# a redeploy of that version uses); unset, the image reports `unknown` and gets the sha tag only.
# CHANGELOG_DIR (optional) holds changelog.json, made by scripts/changelog.sh for the version being built
# (ADR 0039 step 6): it is passed as the build context `changelog` and only enters the image. The release
# workflow makes it from the full history, which this script's checkout (the release commit alone) lacks;
# unset (`just release-image`), the image carries an empty changelog.
# Needs fly credentials (FLY_API_TOKEN, or `fly auth login`): `fly auth docker` logs docker in to the
# registry. Plain `docker build`: no build cache is exported (step 7.2). Never traces the shell.
set -euo pipefail

usage="usage: release-image.sh <catalogue dir> <rulebooks commit> <app>..."
catalogue="${1:?$usage}"
rulebooks="${2:?$usage}"
shift 2
[ $# -gt 0 ] || { echo "$usage" >&2; exit 2; }

[ -s "$catalogue/catalogue.json" ] || { echo "release-image: no catalogue.json in $catalogue" >&2; exit 1; }
[[ "$rulebooks" =~ ^[0-9a-f]{40}$ ]] || { echo "release-image: the rulebooks commit must be a full sha, got '$rulebooks'" >&2; exit 1; }
version="${VERSION:-}"
[ -z "$version" ] || [[ "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] \
    || { echo "release-image: VERSION must be MAJOR.MINOR.PATCH, got '$version'" >&2; exit 1; }
changelog_dir="${CHANGELOG_DIR:-}"
[ -z "$changelog_dir" ] || [ -s "$changelog_dir/changelog.json" ] \
    || { echo "release-image: no changelog.json in $changelog_dir" >&2; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "release-image: the work tree is not clean; commit first" >&2; exit 1; }

fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
commit="$(git rev-parse HEAD)"
local_ref="critical:release-$commit"

versioned=()
[ -z "$version" ] || versioned=(--build-arg VERSION="$version" --label org.opencontainers.image.version="$version")
with_changelog=()
[ -z "$changelog_dir" ] || with_changelog=(--build-context changelog="$changelog_dir")
docker build -t "$local_ref" \
    --build-context catalogue="$catalogue" \
    "${with_changelog[@]}" \
    --build-arg COMMIT="$commit" \
    "${versioned[@]}" \
    --label org.opencontainers.image.revision="$commit" \
    --label critical.rulebooks.revision="$rulebooks" \
    .
"$fly" auth docker
for app in "$@"; do
    for tag in "$commit" ${version:+"v$version"}; do
        ref="registry.fly.io/$app:$tag"
        docker tag "$local_ref" "$ref"
        docker push "$ref" | tail -n1   # "<tag>: digest: sha256:… size: …"
    done
done
