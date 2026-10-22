#!/usr/bin/env bash
# Deploys the release image of a main commit to the staging app and smoke-checks it (ADR 0024 steps
# 3.2, 3.3, 8.3 and 10.5): asserts the access check (no public IP, or the login gate live: scripts/fly-access-check.sh), `fly deploy`s registry.fly.io/<app>:<sha>,
# smoke-checks every machine through `fly proxy` (scripts/fly-smoke.sh: health with the commit, a
# database read, the web app; it waits out a cold machine and a cold shared database), then reads the
# digest the machines run. Used by .github/actions/staging-deploy (release.yml); runnable by hand with fly
# credentials.
#
#   scripts/staging-deploy.sh <full commit sha> [version X.Y.Z]
#
# The image is always registry.fly.io/<app>:<sha>, the build of that commit, never the version tag: runs of
# different commits can compute the same version (the tag step of an earlier one failed, so no tag moved the
# next number on), and the version tag then names whichever build pushed last. With a version (every
# release, and a redeploy of a version), the smoke check also wants /api/health to report it, so a build that
# does not carry that version fails the run instead of staying up.
#
# The digest, not the tag, is the image's identity (a rerun for a new catalogue pushes a new digest under
# the same tag): it is printed and written as `digest=sha256:...` to $GITHUB_OUTPUT. When the deploy or
# the smoke check fails, the app's releases are printed so the previous one can be redeployed
# (docs/deploy.md, "Rollback"). The access check has no bypass and runs before and after the deploy. STAGING_CONFIG names the
# fly config to deploy with (the workflow passes the one of the image's own commit; default: this
# checkout's deploy/fly.staging.toml). Prints no secret.
set -euo pipefail

sha="${1:?usage: staging-deploy.sh <full commit sha> [version]}"
version="${2:-}"
[[ "$sha" =~ ^[0-9a-f]{40}$ ]] || { echo "staging-deploy: the commit must be a full sha, got '$sha'" >&2; exit 2; }
[ -z "$version" ] || [[ "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] \
    || { echo "staging-deploy: the version must be MAJOR.MINOR.PATCH, got '$version'" >&2; exit 2; }

here="$(cd "$(dirname "$0")" && pwd)"
config="${STAGING_CONFIG:-$(cd "$here/.." && pwd)/deploy/fly.staging.toml}"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
smoke="${FLY_SMOKE:-$here/fly-smoke.sh}"
out="${GITHUB_OUTPUT:-/dev/stdout}"
app="$(sed -n 's/^app = "\(.*\)"$/\1/p' "$config")"
[ -n "$app" ] || { echo "staging-deploy: no app in $config" >&2; exit 1; }
image="registry.fly.io/$app:$sha"

# Before: a public IP is allowed only while the login gate is live (no bypass).
FLY="$fly" "$here/fly-access-check.sh" "$app"

# The image the machines run now, to put back when the new one fails (a release that crash-loops must not
# leave staging down). Empty when staging has no machine yet.
previous="$("$fly" machine list -a "$app" --json 2>/dev/null | jq -r '[.[].config.image // empty] | unique | if length == 1 then .[0] else empty end' 2>/dev/null || true)"

# From here a failure puts the previous image back and leaves a release to roll back to.
rollback_hint() {
    [ $? -eq 0 ] && return
    if [ -n "$previous" ] && [ "$previous" != "$image" ]; then
        echo "staging-deploy: FAILED. Putting back the previous image $previous" >&2
        "$fly" deploy -c "$config" -a "$app" --image "$previous" --no-public-ips --ha=false --yes >&2 \
            || echo "staging-deploy: the previous image could not be put back; redeploy it by hand" >&2
    fi
    echo "staging-deploy: FAILED. Releases of $app, newest first; redeploy the previous one's image with" >&2
    echo "  fly deploy -c deploy/fly.staging.toml --image <its image> (docs/deploy.md, \"Rollback\"):" >&2
    "$fly" releases --image -a "$app" 2>&1 | head -n 6 >&2 || true
}
trap rollback_hint EXIT

"$fly" deploy -c "$config" -a "$app" --image "$image" --no-public-ips --ha=false --yes
FLY="$fly" "$smoke" "$app" "$sha" ${version:+"$version"}
# After: the release now running must hold the gate too.
FLY="$fly" "$here/fly-access-check.sh" "$app"

digests="$("$fly" machine list -a "$app" --json | jq -r '[.[].image_ref.digest // empty] | unique | .[]')"
[ "$(wc -w <<<"$digests")" = 1 ] || { echo "staging-deploy: the machines do not run exactly one image digest: $digests" >&2; exit 1; }
echo "staging-deploy: OK ($app runs $sha${version:+ as $version}, digest $digests)"
echo "digest=$digests" >> "$out"
