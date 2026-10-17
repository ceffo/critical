#!/usr/bin/env bash
# Promotes a release tag to production (ADR 0024 steps 4, 8.3 and 10.4): the image staging ran for the
# tag's commit, by digest, never rebuilt. Used by .github/workflows/promote.yml (run by hand from main,
# with the tag `scripts/release-guards.sh resolve` gave: the newest version that passed staging, or the one
# asked, also to roll back); runnable by hand with the production deploy token.
#
#   scripts/promote.sh <tag>
#
# In order, each step stopping the promotion when it fails:
#   1. the guards (scripts/release-guards.sh all): vMAJOR.MINOR.PATCH, on main, a successful staging
#      deploy of the commit and its digest, that image in the registry; then it prints what production
#      runs now (its machines' image: version label, commit tag and digest; read-only); with
#      PROMOTE_NEWEST=true (the workflow, no version asked) a tag older than the highest version production
#      runs, or production unreadable, is refused here;
#   2. the production app has no public IP (scripts/fly-private-check.sh). Mandatory: there is no bypass
#      until a public-access option with access control exists (ADR 0024 step 8);
#   3. the copy of that image, by digest, into the production app's repository,
#      registry.fly.io/<prod app>:<sha> (`docker buildx imagetools create --prefer-index=false`: the same
#      manifest, so the same digest; nothing is rebuilt), then registry.fly.io/<prod app>@<digest> must
#      exist. fly's API gives an app's deploy token the images of that app only (`fly console` and
#      `fly deploy` refuse another app's image), while the registry lets it read the staging repository;
#   4. the restore point: scripts/restore-point.sh in an ephemeral `fly console` machine of the
#      production app started from the copied image, a verified copy database <db>_pre_<tag>_<UTC time> of the
#      production database on its cluster (steps 4.2, 9.2). No restore point, no deploy. It also refuses
#      a production database that records a migration the tag's commit (the image's source) does not
#      embed: a rollback across a migration goes through the runbook instead;
#   5. `fly deploy` of registry.fly.io/<prod app>@<digest> to the production app (rolling);
#   6. the smoke check of every machine (scripts/fly-smoke.sh: health with the commit, a database read,
#      the web app), then every machine runs exactly that digest;
#   7. the GitHub Release of the tag, with generated notes (kept when it exists: a rollback re-runs an
#      older tag).
#
# Needs fly (FLY_API_TOKEN), docker with buildx (logged in to registry.fly.io by `fly auth docker`), gh (GH_TOKEN:
# deployments read, contents write) and the tag and origin/main fetched. PROD_CONFIG names the fly config
# (default: this checkout's deploy/fly.prod.toml, main's in the workflow: the workflow, the scripts and the
# fly config are main's, the image and its migrations the tag's). Prints no secret.
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
# The console machine's size: the app's, from its fly config (fly console does not read it without -c).
vm_size="$(sed -n 's/^ *size = "\(.*\)"$/\1/p' "$config")"
vm_memory="$(sed -n 's/^ *memory = "\([0-9]*\)mb"$/\1/p' "$config")"
[ -n "$vm_size" ] && [ -n "$vm_memory" ] || fail "no [[vm]] size and memory (in mb) in $config"

guards="$("$here/release-guards.sh" all "$tag")"
sha="$(sed -n 's/^sha=//p' <<<"$guards")"
digest="$(sed -n 's/^digest=//p' <<<"$guards")"
[ -n "$sha" ] && [ -n "$digest" ] || fail "the guards gave no commit or digest"
staged="registry.fly.io/$staging_app@$digest"
image="registry.fly.io/$app@$digest"
echo "promote: $tag is $sha, staging ran $staged"
# What production runs now, for the log (read-only; an app without machines says so).
machines_now="" current=""
if machines_now="$("$fly" machine list -a "$app" --json 2>/dev/null)" && current="$(jq -r '[.[].image_ref
    | "\(.labels["org.opencontainers.image.version"] // "no version") (commit \(.tag // "?"), \(.digest // "?"))"]
    | unique | if length == 0 then "nothing (no machine)" else join(", ") end' <<<"$machines_now" 2>/dev/null)"; then
    echo "promote: production ($app) runs $current; promoting $tag"
else
    current=""
    echo "promote: production ($app) runs unknown (its machines could not be read); promoting $tag"
fi
# The newest version (none was asked, PROMOTE_NEWEST=true) never goes below what production runs: that is a
# rollback, asked by naming the version. Unknown is refused too; machines without a version label (images
# from before versions) or no machine at all set no floor.
if [ "${PROMOTE_NEWEST:-false}" = true ]; then
    [ -n "$current" ] || fail "promoting the newest staging version, but what production runs could not be read: nothing promoted (run it again, or name the version: just promote X.Y.Z)"
    floor="$(jq -r '.[].image_ref.labels["org.opencontainers.image.version"] // empty' <<<"$machines_now" \
        | grep -Ex '(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)' | sort -t. -k1,1n -k2,2n -k3,3n | tail -n1)" || floor=""
    if [ -n "$floor" ]; then
        top="$(printf '%s\n%s\n' "$floor" "${tag#v}" | sort -t. -k1,1n -k2,2n -k3,3n | tail -n1)"
        [ "$top" = "${tag#v}" ] || fail "the newest staging version $tag is older than production's $floor: nothing promoted. A rollback names its version (just promote ${tag#v}; docs/deploy.md, \"Rollback\")"
    fi
fi

FLY="$fly" "$here/fly-private-check.sh" "$app"

echo "promote: copying it to registry.fly.io/$app:$sha (same digest, no rebuild)"
docker buildx imagetools create --prefer-index=false --progress none -t "registry.fly.io/$app:$sha" "$staged" \
    || fail "could not copy $staged to the repository of $app: nothing deployed"
"$here/release-guards.sh" image "$image" || fail "the copy in the repository of $app is not $digest: nothing deployed"

point="${db}_pre_${tag//./_}_$(date -u +%Y%m%d%H%M%S)"
# The migrations the image embeds: those in the tree of its commit (read from git, whatever is checked out).
migrations="$(git -C "$root" ls-tree --name-only "$sha" api/internal/storage/postgres/migrations/ | sed -n 's|.*/||p' \
    | grep -Ex '[0-9a-z_]+\.sql' | tr '\n' ' ')" || true
[ -n "$migrations" ] || fail "no migrations in the tree of $sha"
echo "promote: restore point $point (fly console on $app, image $digest)"
log="$(mktemp)"
trap 'rm -f "$log"' EXIT
# fly console may not pass the remote exit status through: the script's last line is the proof.
"$fly" console -a "$app" --image "$image" --vm-size "$vm_size" --vm-memory "$vm_memory" \
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

notes="Image \`$image\` (commit $sha), the one staging ran and smoke-checked as \`$staged\`. Restore point taken before it: \`$point\`."
if gh release view "$tag" >/dev/null 2>&1; then
    echo "promote: the GitHub Release $tag exists (a re-run or a rollback): kept"
else
    gh release create "$tag" --verify-tag --title "$tag" --generate-notes --notes "$notes" >/dev/null
    echo "promote: GitHub Release $tag created"
fi
echo "promote: OK ($tag on $app)"
