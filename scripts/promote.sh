#!/usr/bin/env bash
# Promotes a release tag to production (ADR 0024 steps 4, 8.3, 9.2 and 10.4): the image staging ran for the
# tag's commit, by digest, never rebuilt. Used by .github/workflows/promote.yml (run by hand from main,
# with the tag `scripts/release-guards.sh resolve` gave: the newest version that passed staging, or the one
# asked, also to roll back); runnable by hand with the production deploy token.
#
#   scripts/promote.sh prepare <tag>   steps 1 to 4: the restore point, encrypted, in RESTORE_POINT_DIR
#   scripts/promote.sh deploy <tag>    steps 5 to 7, once the restore point is kept (RESTORE_POINT_KEPT)
#   scripts/promote.sh <tag>           both, by hand: the restore point stays in RESTORE_POINT_DIR
#                                      (default .backups/restore-points, gitignored) on this machine
#
# The workflow runs `prepare`, uploads the restore point as an artifact of its run, then runs `deploy` with
# RESTORE_POINT_KEPT naming that artifact: no restore point kept, nothing deployed. In order, each step
# stopping the promotion when it fails:
#   1. the guards (scripts/release-guards.sh all): vMAJOR.MINOR.PATCH, on main, a successful staging
#      deploy of the commit and its digest, that image in the registry; then it prints what production
#      runs now (its machines' image: version label, commit tag and digest; read-only); with
#      PROMOTE_NEWEST=true (the workflow, no version asked) a tag older than the highest version production
#      runs, or production unreadable, is refused here;
#   2. the production app has no public IP, or the login gate is live (scripts/fly-access-check.sh,
#      scripts/gate-check.sh). Mandatory: there is no bypass (ADR 0024 step 8.3);
#   3. the copy of that image, by digest, into the production app's repository,
#      registry.fly.io/<prod app>:<sha> (`docker buildx imagetools create --prefer-index=false`: the same
#      manifest, so the same digest; nothing is rebuilt), then registry.fly.io/<prod app>@<digest> must
#      exist. fly's API gives an app's deploy token the images of that app only (`fly console` and
#      `fly deploy` refuse another app's image), while the registry lets it read the staging repository;
#   4. the restore point <db>_pre_<tag>_<UTC time> (steps 4.2, 9.2): scripts/restore-point.sh in an ephemeral
#      `fly console` machine of the production app started from the copied image, in the app's primary
#      region, pg_dumps the production database and sends it back (a tar of the dump and its table counts,
#      in base64 between two marker lines). Here the block is kept out of the log, decoded, checked against
#      the checksum and size the script printed, and encrypted with gpg (AES-256, the passphrase
#      RESTORE_POINT_PASSPHRASE) into RESTORE_POINT_DIR/<name>.tar.gpg, which is decrypted once to check
#      it; the plaintext is deleted. The repository is public, and so are its run artifacts: only the
#      encrypted file leaves this machine (ADR 0024 step 7). It also refuses a production database that
#      records a migration the tag's commit (the image's source) does not embed: a rollback across a
#      migration goes through the runbook instead. RESTORE_POINT_DIR/promote.env records the tag, commit,
#      digest and restore point for `deploy`, and GITHUB_OUTPUT (when set) gets point= and file=;
#   5. `fly deploy` of registry.fly.io/<prod app>@<digest> to the production app (rolling);
#   6. the smoke check of every machine (scripts/fly-smoke.sh: health with the commit, a database read,
#      the web app), then every machine runs exactly that digest;
#   7. the GitHub Release of the tag, with generated notes naming the restore point and where it is kept
#      (kept when it exists: a rollback re-runs an older tag).
#
# Needs fly (FLY_API_TOKEN), docker with buildx (logged in to registry.fly.io by `fly auth docker`), gh (GH_TOKEN:
# deployments read, contents write), gpg, and the tag and origin/main fetched. PROD_CONFIG names the fly config
# (default: this checkout's deploy/fly.prod.toml, main's in the workflow: the workflow, the scripts and the
# fly config are main's, the image and its migrations the tag's). Prints no secret and no data.
set -euo pipefail

usage="usage: promote.sh [prepare|deploy] <tag>"
phase=all
case "${1:-}" in prepare | deploy) phase="$1"; shift ;; esac
tag="${1:?$usage}"
[ "$#" = 1 ] || { echo "$usage" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
config="${PROD_CONFIG:-$root/deploy/fly.prod.toml}"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
smoke="${FLY_SMOKE:-$here/fly-smoke.sh}"
fail() { echo "promote: FAIL: $*" >&2; exit 1; }

app="$(sed -n 's/^app = "\(.*\)"$/\1/p' "$config")"
staging_app="$(sed -n 's/^app = "\(.*\)"$/\1/p' "$root/deploy/fly.staging.toml")"
[ -n "$app" ] && [ -n "$staging_app" ] || fail "no app in $config or deploy/fly.staging.toml"
db="critical_prod" # the database keeps the environment name; the app was renamed (ADR 0024 step 8.5, scripts/fly-db-provision.sh)
points="${RESTORE_POINT_DIR:-}"
if [ -z "$points" ]; then
    [ "$phase" = all ] || fail "RESTORE_POINT_DIR is not set: where the restore point is written and read"
    points="$root/.backups/restore-points"
fi

prepare() {
# The restore point leaves this machine encrypted only: no passphrase or no gpg, nothing starts.
[ -n "${RESTORE_POINT_PASSPHRASE:-}" ] || fail "RESTORE_POINT_PASSPHRASE is not set: the restore point could not be encrypted, nothing promoted (docs/deploy.md, \"Promotion\")"
command -v gpg >/dev/null || fail "gpg is missing: the restore point could not be encrypted, nothing promoted"
# The console machine's size: the app's, from its fly config (fly console does not read it without -c).
vm_size="$(sed -n 's/^ *size = "\(.*\)"$/\1/p' "$config")"
vm_memory="$(sed -n 's/^ *memory = "\([0-9]*\)mb"$/\1/p' "$config")"
[ -n "$vm_size" ] && [ -n "$vm_memory" ] || fail "no [[vm]] size and memory (in mb) in $config"
# The console machine runs where the app does (and so the database): fly picks the nearest region otherwise,
# and a long dump/restore across regions through fly's proxy has dropped mid-restore.
region="$(sed -n 's/^primary_region = "\(.*\)"$/\1/p' "$config")"
[ -n "$region" ] || fail "no primary_region in $config"

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

FLY="$fly" "$here/fly-access-check.sh" "$app"

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
# fly console may not pass the remote exit status through: the script's last line is the proof. Its stdout
# carries the dump: it goes to a file, never to the log.
"$fly" console -a "$app" --image "$image" --region "$region" --vm-size "$vm_size" --vm-memory "$vm_memory" \
    --file-local "/tmp/restore-point.sh=$here/restore-point.sh" \
    -C "sh /tmp/restore-point.sh $point ${migrations% }" </dev/null >"$work/console.out" 2>"$work/console.err" || true
tr -d '\r' <"$work/console.out" >"$work/console.txt"
rm -f "$work/console.out"
# Printed: every line outside the block (fly's own lines, the script's progress and failures); the block's
# lines go to a file. An unterminated block (a cut output) prints nothing after its first line.
awk -v begin="restore-point: BEGIN $point" -v end="restore-point: END $point" -v data="$work/point.b64" '
    $0 == begin { inside = 1; next }
    $0 == end { inside = 0; next }
    inside { print > data; next }
    { print }' "$work/console.txt"
cat "$work/console.err" >&2
ok="$(grep -E "^restore-point: OK $point [0-9a-f]{64} [0-9]+\$" "$work/console.txt" | tail -n1)" \
    || fail "no restore point of $db, nothing deployed (the output above says why; docs/deploy.md, \"Promotion\")"
rm -f "$work/console.txt"
read -r _ _ _ sum bytes <<<"$ok"
[ -s "$work/point.b64" ] && base64 -d "$work/point.b64" >"$work/point.tar" 2>/dev/null \
    || fail "the restore point $point did not come back whole: nothing deployed"
rm -f "$work/point.b64"
[ "$(wc -c <"$work/point.tar" | tr -d ' ')" = "$bytes" ] && [ "$(sha256sum "$work/point.tar" | cut -d' ' -f1)" = "$sum" ] \
    || fail "the restore point $point came back damaged (size or checksum): nothing deployed"

mkdir -p "$points"
chmod 700 "$points"
file="$points/$point.tar.gpg"
GNUPGHOME="$work/gnupg" gpg_run --symmetric --cipher-algo AES256 --output "$file" "$work/point.tar" \
    || fail "could not encrypt the restore point $point: nothing deployed"
# Read back: the file must decrypt, with the passphrase, to the very tar the console machine sent.
[ "$(GNUPGHOME="$work/gnupg" gpg_run --decrypt "$file" | sha256sum | cut -d' ' -f1)" = "$sum" ] \
    || fail "the encrypted restore point $file does not decrypt to the dump: nothing deployed"
rm -f "$work/point.tar"
printf 'tag=%s\nsha=%s\ndigest=%s\npoint=%s\nsum=%s\n' "$tag" "$sha" "$digest" "$point" "$sum" >"$points/promote.env"
echo "promote: restore point $point kept encrypted in $file ($(wc -c <"$file" | tr -d ' ') bytes; sha256 of the tar $sum)"
if [ -n "${GITHUB_OUTPUT:-}" ]; then
    printf 'point=%s\nfile=%s\n' "$point" "$file" >>"$GITHUB_OUTPUT"
fi
}

# gpg_run <gpg args>: gpg in batch mode with the passphrase on file descriptor 3 (never an argument, never
# in a file), no agent cache, in the throwaway GNUPGHOME the caller sets.
gpg_run() {
    mkdir -p -m 700 "$GNUPGHOME"
    gpg --batch --quiet --no-tty --yes --pinentry-mode loopback --no-symkey-cache --passphrase-fd 3 "$@" \
        3<<<"$RESTORE_POINT_PASSPHRASE"
}

deploy() {
[ -s "$points/promote.env" ] || fail "no restore point recorded in $points/promote.env: nothing deployed (run prepare first)"
state="$(cat "$points/promote.env")"
[ "$(sed -n 's/^tag=//p' <<<"$state")" = "$tag" ] || fail "the restore point in $points was taken for another tag: nothing deployed"
sha="$(sed -n 's/^sha=//p' <<<"$state")"
digest="$(sed -n 's/^digest=//p' <<<"$state")"
point="$(sed -n 's/^point=//p' <<<"$state")"
[ -n "$sha" ] && [ -n "$digest" ] && [ -n "$point" ] || fail "$points/promote.env is incomplete: nothing deployed"
[ -s "$points/$point.tar.gpg" ] || fail "the restore point $points/$point.tar.gpg is missing: nothing deployed"
kept="${RESTORE_POINT_KEPT:-}"
[ -n "$kept" ] || fail "RESTORE_POINT_KEPT does not say where the restore point $point is kept: nothing deployed"
staged="registry.fly.io/$staging_app@$digest"
image="registry.fly.io/$app@$digest"

# From here a failure leaves production on a release to roll back to.
rollback_hint() {
    [ $? -eq 0 ] && return
    echo "promote: FAILED after the restore point $point ($kept). Releases of $app, newest first; roll back by" >&2
    echo "  re-running the promotion of the previous tag (docs/deploy.md, \"Rollback\"):" >&2
    "$fly" releases --image -a "$app" 2>&1 | head -n 6 >&2 || true
}
trap 'rollback_hint; cleanup_work' EXIT

"$fly" deploy -c "$config" -a "$app" --image "$image" --no-public-ips --yes
FLY="$fly" "$smoke" "$app" "$sha"
FLY="$fly" "$here/fly-access-check.sh" "$app" # the release now running must hold the gate too
running="$("$fly" machine list -a "$app" --json | jq -r '[.[].image_ref.digest // empty] | unique | .[]')"
[ "$running" = "$digest" ] || fail "the machines of $app run '$running', not $digest"
echo "promote: $app runs $tag ($sha, $digest)"

notes="Image \`$image\` (commit $sha), the one staging ran and smoke-checked as \`$staged\`. Restore point taken before it: \`$point\`, $kept (docs/deploy.md, \"Rollback\")."
if gh release view "$tag" >/dev/null 2>&1; then
    echo "promote: the GitHub Release $tag exists (a re-run or a rollback): kept"
else
    gh release create "$tag" --verify-tag --title "$tag" --generate-notes --notes "$notes" >/dev/null
    echo "promote: GitHub Release $tag created"
fi
echo "promote: OK ($tag on $app)"
}

umask 077
work="$(mktemp -d)"
# cleanup_work: the throwaway gpg home's agent stopped, then everything in $work (no clear dump stays).
cleanup_work() { [ ! -d "$work/gnupg" ] || GNUPGHOME="$work/gnupg" gpgconf --kill gpg-agent 2>/dev/null || true; rm -rf "$work"; }
trap cleanup_work EXIT
case "$phase" in
    prepare) prepare ;;
    deploy) deploy ;;
    all)
        prepare
        RESTORE_POINT_KEPT="${RESTORE_POINT_KEPT:-kept on the machine that ran the promotion, $points/$point.tar.gpg}" deploy ;;
esac
