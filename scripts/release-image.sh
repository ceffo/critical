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
[ -z "$(git status --porcelain)" ] || { echo "release-image: the work tree is not clean; commit first" >&2; exit 1; }

fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
commit="$(git rev-parse HEAD)"
local_ref="critical:release-$commit"

docker build -t "$local_ref" \
    --build-context catalogue="$catalogue" \
    --build-arg COMMIT="$commit" \
    --label org.opencontainers.image.revision="$commit" \
    --label critical.rulebooks.revision="$rulebooks" \
    .
"$fly" auth docker
for app in "$@"; do
    ref="registry.fly.io/$app:$commit"
    docker tag "$local_ref" "$ref"
    docker push "$ref" | tail -n1   # "<tag>: digest: sha256:… size: …"
done
