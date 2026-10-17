#!/usr/bin/env bash
# Deploys the release image of a main commit to the staging app and smoke-checks it (ADR 0024 steps
# 3.2, 3.3, 8.3 and 10.5): asserts the app has no public IP, `fly deploy`s registry.fly.io/<app>:<sha>,
# smoke-checks every machine through `fly proxy` (scripts/fly-smoke.sh: health with the commit, a
# database read, the web app; it waits out a cold machine and a cold shared database), then reads the
# digest the machines run. Used by .github/workflows/deploy-staging.yml; runnable by hand with fly
# credentials.
#
#   scripts/staging-deploy.sh <full commit sha>
#
# The digest, not the tag, is the image's identity (a rerun for a new catalogue pushes a new digest under
# the same tag): it is printed and written as `digest=sha256:...` to $GITHUB_OUTPUT. When the deploy or
# the smoke check fails, the app's releases are printed so the previous one can be redeployed
# (docs/deploy.md, "Rollback"). The no-public-IP check is skipped only when PUBLIC_ACCESS_OPTION names a
# chosen public-access option (the repository variable of ADR 0024 step 8.4). Prints no secret.
set -euo pipefail

sha="${1:?usage: staging-deploy.sh <full commit sha>}"
[[ "$sha" =~ ^[0-9a-f]{40}$ ]] || { echo "staging-deploy: the commit must be a full sha, got '$sha'" >&2; exit 2; }

here="$(cd "$(dirname "$0")" && pwd)"
config="$(cd "$here/.." && pwd)/deploy/fly.staging.toml"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
smoke="${FLY_SMOKE:-$here/fly-smoke.sh}"
out="${GITHUB_OUTPUT:-/dev/stdout}"
app="$(sed -n 's/^app = "\(.*\)"$/\1/p' "$config")"
[ -n "$app" ] || { echo "staging-deploy: no app in $config" >&2; exit 1; }
image="registry.fly.io/$app:$sha"

if [ -n "${PUBLIC_ACCESS_OPTION:-}" ]; then
    echo "::notice title=Staging access::public-access option '$PUBLIC_ACCESS_OPTION' is configured: the no-public-IP check is skipped"
else
    FLY="$fly" "$here/fly-private-check.sh" "$app"
fi

# From here a failure leaves a release to roll back to.
rollback_hint() {
    [ $? -eq 0 ] && return
    echo "staging-deploy: FAILED. Releases of $app, newest first; redeploy the previous one's image with" >&2
    echo "  fly deploy -c deploy/fly.staging.toml --image <its image> (docs/deploy.md, \"Rollback\"):" >&2
    "$fly" releases --image -a "$app" 2>&1 | head -n 6 >&2 || true
}
trap rollback_hint EXIT

"$fly" deploy -c "$config" -a "$app" --image "$image" --yes
FLY="$fly" "$smoke" "$app" "$sha"

digests="$("$fly" machine list -a "$app" --json | jq -r '[.[].image_ref.digest // empty] | unique | .[]')"
[ "$(wc -w <<<"$digests")" = 1 ] || { echo "staging-deploy: the machines do not run exactly one image digest: $digests" >&2; exit 1; }
echo "staging-deploy: OK ($app runs $sha, digest $digests)"
echo "digest=$digests" >> "$out"
