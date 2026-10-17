#!/usr/bin/env bash
# Creates the version tag of a staging release once its staging deploy and smoke check passed (ADR 0024
# step 3.4, docs/deploy.md "Release version"): the annotated tag vX.Y.Z on the commit, through the GitHub
# API with the workflow's token (`gh`, GH_TOKEN with contents: write, and GITHUB_REPOSITORY). A failed
# staging run never gets here, so its version takes no tag (the next build computes it again).
#
#   scripts/version-tag.sh <X.Y.Z> <full commit sha> <staging digest sha256:...>
#
# A tag created with the workflow's token starts no workflow (GitHub: "events triggered by the
# GITHUB_TOKEN ... will not create a new workflow run"), so this tag does not start the Promote workflow;
# promoting is running Promote from the tag (docs/deploy.md, "Promotion"). The tag ruleset lets the
# GitHub Actions app create v* tags, never move or delete them (scripts/github-tag-ruleset.sh). An
# existing tag is never moved: GitHub refuses to create the ref, and so does this script.
set -euo pipefail

usage="usage: version-tag.sh <X.Y.Z> <full commit sha> <sha256:digest>"
version="${1:?$usage}"
sha="${2:?$usage}"
digest="${3:?$usage}"
[[ "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || { echo "version-tag: '$version' is not MAJOR.MINOR.PATCH" >&2; exit 2; }
[[ "$sha" =~ ^[0-9a-f]{40}$ ]] || { echo "version-tag: the commit must be a full sha, got '$sha'" >&2; exit 2; }
[[ "$digest" =~ ^sha256:[0-9a-f]{64}$ ]] || { echo "version-tag: the staging digest must be sha256:<64 hex>, got '$digest'" >&2; exit 2; }
repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}"
tag="v$version"

object="$(jq -n --arg tag "$tag" --arg sha "$sha" --arg digest "$digest" --arg date "$(date -u +%Y-%m-%dT%H:%M:%SZ)" '{
        tag: $tag, object: $sha, type: "commit",
        message: "Release \($tag)\n\nStaging ran and smoke-checked the image \($digest).",
        tagger: {name: "github-actions[bot]", email: "41898282+github-actions[bot]@users.noreply.github.com", date: $date}}' \
    | gh api -X POST "repos/$repo/git/tags" --input - --jq .sha)"
[[ "$object" =~ ^[0-9a-f]{40}$ ]] || { echo "version-tag: no tag object from GitHub" >&2; exit 1; }
jq -n --arg ref "refs/tags/$tag" --arg sha "$object" '{ref: $ref, sha: $sha}' \
    | gh api -X POST "repos/$repo/git/refs" --input - >/dev/null \
    || { echo "version-tag: GitHub refused to create $tag (it exists already, or the tag ruleset does not let the workflow create it)" >&2; exit 1; }
echo "::notice title=Version::$tag tagged on $sha (staging digest $digest)"
