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
# promoting is `just promote [version]`, a run of Promote from main (docs/deploy.md, "Promotion"). Creating a v* tag is
# not restricted by a ruleset (GitHub refuses the Actions-app bypass on a personal repository); the ruleset keeps
# only moving and deleting them admin-only (scripts/github-tag-ruleset.sh). An
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

# A transient GitHub API error (502, a secondary-rate-limit 403) must not leave main red after a passed
# staging deploy: every call is retried (TAG_ATTEMPTS, default 5, TAG_DELAY seconds apart, doubling), and
# the whole step is safe to repeat: a tag that already points at this commit (a retry whose first try
# reached GitHub but lost the answer, or a rerun) is a success; one that points at another commit is not.
attempts="${TAG_ATTEMPTS:-5}"
delay="${TAG_DELAY:-5}"
retry() {
    local n=1 wait="$delay"
    until "$@"; do
        [ "$n" -lt "$attempts" ] || return 1
        echo "version-tag: attempt $n/$attempts failed, retrying in ${wait}s" >&2
        sleep "$wait"
        n=$((n + 1)); wait=$((wait * 2))
    done
}

# The commit the tag $tag points at, empty when there is no such tag (a 404 is not an error to retry).
tag_commit() {
    local out
    out="$(gh api "repos/$repo/git/ref/tags/$tag" 2>&1)" || { [[ "$out" == *"404"* || "$out" == *"Not Found"* ]] && return 0; echo "$out" >&2; return 1; }
    local type obj
    type="$(jq -r .object.type <<<"$out")"; obj="$(jq -r .object.sha <<<"$out")"
    if [ "$type" = tag ]; then
        gh api "repos/$repo/git/tags/$obj" --jq .object.sha
    else
        echo "$obj"
    fi
}

make_tag() {
    local existing object
    existing="$(tag_commit)" || return 1
    if [ -n "$existing" ]; then
        [ "$existing" = "$sha" ] && return 0
        echo "version-tag: $tag exists already on $existing, not on $sha; a tag is never moved" >&2
        exit 1
    fi
    object="$(jq -n --arg tag "$tag" --arg sha "$sha" --arg digest "$digest" --arg date "$(date -u +%Y-%m-%dT%H:%M:%SZ)" '{
            tag: $tag, object: $sha, type: "commit",
            message: "Release \($tag)\n\nStaging ran and smoke-checked the image \($digest).",
            tagger: {name: "github-actions[bot]", email: "41898282+github-actions[bot]@users.noreply.github.com", date: $date}}' \
        | gh api -X POST "repos/$repo/git/tags" --input - --jq .sha)" || return 1
    [[ "$object" =~ ^[0-9a-f]{40}$ ]] || return 1
    jq -n --arg ref "refs/tags/$tag" --arg sha "$object" '{ref: $ref, sha: $sha}' \
        | gh api -X POST "repos/$repo/git/refs" --input - >/dev/null
}

retry make_tag || { echo "version-tag: GitHub refused to create $tag after $attempts attempts (the workflow token may not write the repository)" >&2; exit 1; }
echo "::notice title=Version::$tag tagged on $sha (staging digest $digest)"
