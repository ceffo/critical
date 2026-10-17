#!/usr/bin/env bash
# Promotes a release tag to production (ADR 0024 steps 4, 8.3 and 10.4): the image staging ran for the
# tag's commit, by digest, never rebuilt. Used by .github/workflows/promote.yml (on the tag push, and to
# roll back by re-running an older tag); runnable by hand with the production deploy token.
#
#   scripts/promote.sh <tag>
#
# In order, each step stopping the promotion when it fails:
#   1. the guards (scripts/release-guards.sh all): vMAJOR.MINOR.PATCH, on main, a successful staging
#      deploy of the commit and its digest, that image in the registry;
#   2. the production app has no public IP (scripts/fly-private-check.sh). Mandatory: there is no bypass
#      until a public-access option with access control exists (ADR 0024 step 8);
#   3. the restore point: scripts/restore-point.sh in an ephemeral `fly console` machine of the
#      production app started from that image, a verified copy database <db>_pre_<tag>_<UTC time> of the
#      production database on its cluster (steps 4.2, 9.2). No restore point, no deploy. It also refuses
#      a production database that records a migration the tag's commit (the image's source) does not
#      embed: a rollback across a migration goes through the runbook instead;
#   4. `fly deploy` of registry.fly.io/<staging app>@<digest> to the production app (rolling);
#   5. the smoke check of every machine (scripts/fly-smoke.sh: health with the commit, a database read,
#      the web app), then every machine runs exactly that digest;
#   6. the GitHub Release of the tag, with generated notes (kept when it exists: a rollback re-runs an
#      older tag).
#
# Needs fly (FLY_API_TOKEN), docker (logged in to registry.fly.io by `fly auth docker`), gh (GH_TOKEN:
# deployments read, contents write) and the tag and origin/main fetched. PROD_CONFIG names the fly config
# (default: this checkout's deploy/fly.prod.toml, the tag's own in the workflow). Prints no secret.
set -euo pipefail

tag="${1:?usage: promote.sh <tag>}"
here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
config="${PROD_CONFIG:-$root/deploy/fly.prod.toml}"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
smoke="${FLY_SMOKE:-$here/fly-smoke.sh}"
fail() { echo "promote: FAIL: $*" >&2; exit 1; }

app="$(sed -n 's/^app = "\(.*\)"$/\1/p' "$config")"
staging_app="$(sed -n 's/^app = "\(.*\)"$/\1/p' "$root/deploy/fly.staging.toml")"
[ -n "$app" ] && [ -n "$staging_app" ] || fail "no app in $config or deploy/fly.staging.toml"
db="critical_${app#critical-}" # the database of the app (scripts/fly-db-provision.sh)

guards="$("$here/release-guards.sh" all "$tag")"
sha="$(sed -n 's/^sha=//p' <<<"$guards")"
digest="$(sed -n 's/^digest=//p' <<<"$guards")"
[ -n "$sha" ] && [ -n "$digest" ] || fail "the guards gave no commit or digest"
image="registry.fly.io/$staging_app@$digest"
echo "promote: $tag is $sha, staging ran $image"

FLY="$fly" "$here/fly-private-check.sh" "$app"

point="${db}_pre_${tag//./_}_$(date -u +%Y%m%d%H%M%S)"
# The migrations the image embeds: those in the tree of its commit (read from git, whatever is checked out).
migrations="$(git -C "$root" ls-tree --name-only "$sha" api/internal/storage/postgres/migrations/ | sed -n 's|.*/||p' \
    | grep -Ex '[0-9a-z_]+\.sql' | tr '\n' ' ')" || true
[ -n "$migrations" ] || fail "no migrations in the tree of $sha"
echo "promote: restore point $point (fly console on $app, image $digest)"
log="$(mktemp)"
trap 'rm -f "$log"' EXIT
# fly console may not pass the remote exit status through: the script's last line is the proof.
"$fly" console -a "$app" --image "$image" \
    --file-local "/tmp/restore-point.sh=$here/restore-point.sh" \
    -C "sh /tmp/restore-point.sh $point ${migrations% }" </dev/null 2>&1 | tee "$log" || true
tr -d '\r' <"$log" | grep -qx "restore-point: OK $point" \
    || fail "no restore point of $db, nothing deployed (the output above says why; docs/deploy.md, \"Promotion\")"

# From here a failure leaves production on a release to roll back to.
rollback_hint() {
    [ $? -eq 0 ] && return
    echo "promote: FAILED after the restore point $point. Releases of $app, newest first; roll back by" >&2
    echo "  re-running the promotion of the previous tag (docs/deploy.md, \"Rollback\"):" >&2
    "$fly" releases --image -a "$app" 2>&1 | head -n 6 >&2 || true
    rm -f "$log"
}
trap rollback_hint EXIT

"$fly" deploy -c "$config" -a "$app" --image "$image" --no-public-ips --yes
FLY="$fly" "$smoke" "$app" "$sha"
running="$("$fly" machine list -a "$app" --json | jq -r '[.[].image_ref.digest // empty] | unique | .[]')"
[ "$running" = "$digest" ] || fail "the machines of $app run '$running', not $digest"
echo "promote: $app runs $tag ($sha, $digest)"

notes="Image \`$image\` (commit $sha), the one staging ran and smoke-checked. Restore point taken before it: \`$point\`."
if gh release view "$tag" >/dev/null 2>&1; then
    echo "promote: the GitHub Release $tag exists (a re-run or a rollback): kept"
else
    gh release create "$tag" --verify-tag --title "$tag" --generate-notes --notes "$notes" >/dev/null
    echo "promote: GitHub Release $tag created"
fi
echo "promote: OK ($tag on $app)"
