#!/usr/bin/env bash
# Tests the promotion's scripts without the network, fly or GitHub (part of `just validate`, ADR 0024
# steps 4, 8.3 and 10.4), on fixture git repositories with fly, gh, docker and the smoke check stubbed:
#   - release-guards.sh: each guard (tag format, on main, the staging record read as docs/deploy.md says,
#     the image in the registry) and `all`, which prints the tag's commit and the digest staging ran (as
#     that tag's version when one commit carries two);
#   - promote.sh: guards, then the no-public-IP check, then the copy of the staged image by digest into the
#     production repository (and its digest checked), then the restore point in a console machine of it, then the deploy by digest, the smoke check, the digest check and the GitHub Release;
#     each failure stops everything after it (a refused guard calls fly not at all; no restore point, no
#     deploy; a failed smoke or a digest mismatch, no release);
#   - restore-point.sh: what it refuses without a database (its SQL runs for real in scripts/image-smoke.sh);
#   - release-guards.sh resolve: no version, the newest (by number) v* version on main that passed staging,
#     skipping higher ones that did not; a version asked (X.Y.Z or vX.Y.Z, the rollback) that must exist, be on
#     main and have passed staging; a malformed one refused;
#   - require-admin.sh: only repository admins pass (who may promote);
#   - github-environments.sh: production deployable from the branch main only (the old tag rule deleted);
#   - `just promote [version]`: the dispatch of promote.yml from main, with the version as its input;
#   - promote.yml, parsed: a run from another ref than main fails saying so; by hand only (no trigger on a tag
#     push), with an optional version input; the admin check before any secret in both jobs, the version
#     resolved without any secret, the production environment, serialized and never cancelled, the token
#     read only by the secret check (which fails on an empty token) and the promotion step.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-promotion: FAIL: $*" >&2; fails=$((fails + 1)); }

digest="sha256:$(printf 'b%.0s' {1..64})"
newer="sha256:$(printf 'd%.0s' {1..64})"
calls="$tmp/calls"
fx="$tmp/fixtures"
stubs="$tmp/bin"
mkdir -p "$fx" "$stubs"

# gh stub: `gh api <url> [--jq expr]` answers from $fx/<url with non-alphanumerics as _> ([] when absent).
cat > "$stubs/gh" <<STUB
#!/usr/bin/env bash
echo "gh \$*" >> "$calls"
case "\$1 \${2:-}" in
    "repo view") echo o/r ;;
    "release view") [ -f "$tmp/release-exists" ] ;;
    "release create") ;;
    "workflow run") ;;
    api\ *)
        url="\$2"; shift 2; expr=""
        while [ "\$#" -gt 0 ]; do case "\$1" in --jq) expr="\$2"; shift 2 ;; *) shift ;; esac; done
        f="$fx/\$(printf '%s' "\$url" | tr -c 'a-zA-Z0-9' '_')"
        [ ! -f "\$f.fail" ] || { echo "gh: HTTP 502" >&2; exit 1; }
        [ -f "\$f" ] || f="$fx/empty"
        if [ -n "\$expr" ]; then jq -r "\$expr" "\$f"; else cat "\$f"; fi ;;
    *) exit 1 ;;
esac
STUB
echo '[]' > "$fx/empty"
cat > "$stubs/docker" <<STUB
#!/bin/sh
echo "docker \$*" >> "$calls"
case "\$*" in
    "buildx imagetools create"*) [ ! -f "$tmp/copy-fails" ] ;;
    *critical-prod@*) [ ! -f "$tmp/copy-missing" ] ;;
    *) [ ! -f "$tmp/image-missing" ] ;;
esac
STUB
cat > "$stubs/fly" <<STUB
#!/bin/sh
echo "fly \$*" >> "$calls"
case "\$1 \$2" in
    "ips list") cat "$tmp/ips" ;;
    "machine list") [ ! -f "$tmp/machines-fail" ] || exit 1; cat "$tmp/machines" ;;
    "releases --image") echo "v3 complete registry.fly.io/critical-staging@previous" ;;
    "console -a") cat "$tmp/console-out" | sed "s/@POINT@/\$(echo "\$*" | sed -n 's/.*restore-point.sh \([a-z0-9_]*\).*/\1/p')/" ;;
esac
STUB
cat > "$stubs/smoke" <<STUB
#!/bin/sh
echo "smoke \$*" >> "$calls"
[ ! -f "$tmp/smoke-fails" ]
STUB
chmod +x "$stubs"/*
export PATH="$stubs:$PATH" GITHUB_REPOSITORY=o/r

# put <url> <json>: what the gh stub answers for that API path.
put() { printf '%s' "$2" > "$fx/$(printf '%s' "$1" | tr -c 'a-zA-Z0-9' '_')"; }
deployments() { echo "repos/o/r/deployments?environment=staging&task=deploy-staging&sha=$1&per_page=100"; }
statuses() { echo "repos/o/r/deployments/$1/statuses?per_page=100"; }

# --- fixture repositories: origin (bare) and a clone whose main holds two commits, plus a side branch ---
git init -q --bare -b main "$tmp/origin.git"
git clone -q "$tmp/origin.git" "$tmp/repo" 2>/dev/null
cd "$tmp/repo"
git config user.email t@example.com
git config user.name test
mkdir -p scripts deploy
cp "$root"/scripts/{release-guards,promote,restore-point,fly-private-check}.sh scripts/
cp "$root"/deploy/fly.{staging,prod}.toml deploy/
mkdir -p api/internal/storage/postgres/migrations
touch api/internal/storage/postgres/migrations/{001_baseline,002_more}.sql
git add -A && git commit -q -m one && git push -q origin main
git commit -q --allow-empty -m two && git push -q origin main
main_sha="$(git rev-parse HEAD)"
git switch -q -c side && git commit -q --allow-empty -m side
side_sha="$(git rev-parse HEAD)"
git switch -q main
git tag -a v1.2.3 -m r "$main_sha"
git tag -a v9.9.9 -m r "$side_sha"
guards="$tmp/repo/scripts/release-guards.sh"

# --- guard: tag format ---
for t in v1.2.3 v0.0.0 v10.20.30; do "$guards" tag "$t" >/dev/null 2>&1 || fail "tag $t refused"; done
for t in 1.2.3 v1.2 v01.2.3 v1.2.3-rc1 v1.2.3.4 V1.2.3 "v1.2.3 "; do
    "$guards" tag "$t" >/dev/null 2>&1 && fail "tag '$t' accepted"
done

# --- guard: on main ---
"$guards" on-main "$main_sha" >/dev/null 2>&1 || fail "a main commit is refused"
out="$("$guards" on-main "$side_sha" 2>&1)" && fail "a commit off main is accepted"
[[ "$out" == *"not on main"* ]] || fail "off-main message: $out"

# --- guard: the staging record (docs/deploy.md "Record") ---
staging() { status=0; out="$("$guards" staging "$@" 2>&1)" || status=$?; }
staging "$main_sha"
[ "$status" != 0 ] && [[ "$out" == *"no successful deploy"* ]] || fail "no deployment: accepted or wrong message: $out"
put "$(deployments "$main_sha")" "[{\"id\":11,\"created_at\":\"2026-10-17T01:00:00Z\",\"payload\":{\"digest\":\"$digest\"}}]"
put "$(statuses 11)" '[{"state":"failure"}]'
staging "$main_sha"
[ "$status" != 0 ] || fail "a failed staging deployment is accepted"
put "$(statuses 11)" '[{"state":"inactive"},{"state":"success"}]'
staging "$main_sha"
[ "$status" = 0 ] && [ "$out" = "$digest" ] || fail "success then inactive: exit $status, '$out' (the history counts, not the latest status)"
put "$(deployments "$main_sha")" "[{\"id\":12,\"created_at\":\"2026-10-17T03:00:00Z\",\"payload\":\"{\\\"digest\\\":\\\"$newer\\\"}\"},
  {\"id\":11,\"created_at\":\"2026-10-17T01:00:00Z\",\"payload\":{\"digest\":\"$digest\"}},
  {\"id\":13,\"created_at\":\"2026-10-17T04:00:00Z\",\"payload\":{\"digest\":\"sha256:$(printf 'e%.0s' {1..64})\"}}]"
put "$(statuses 12)" '[{"state":"inactive"},{"state":"success"},{"state":"in_progress"}]'
put "$(statuses 13)" '[{"state":"failure"}]'
staging "$main_sha"
[ "$status" = 0 ] && [ "$out" = "$newer" ] || fail "newest successful deployment: got '$out' (a string payload, a newer failure ignored)"
put "$(deployments "$main_sha")" "[{\"id\":14,\"created_at\":\"2026-10-17T05:00:00Z\",\"payload\":{}}]"
put "$(statuses 14)" '[{"state":"success"}]'
staging "$main_sha"
[ "$status" != 0 ] && [[ "$out" == *"no image digest"* ]] || fail "a success without a digest: accepted or wrong message: $out"
staging "${main_sha:0:7}"
[ "$status" != 0 ] || fail "a short sha is accepted"
put "$(deployments "$main_sha")" "[{\"id\":12,\"created_at\":\"2026-10-17T03:00:00Z\",\"payload\":{\"digest\":\"$newer\"}}]"

# --- guard: the image ---
: > "$calls"
"$guards" image "registry.fly.io/critical-staging@$newer" >/dev/null 2>&1 || fail "an image in the registry is refused"
grep -qx "docker manifest inspect registry.fly.io/critical-staging@$newer" "$calls" || fail "image: wrong docker call: $(cat "$calls")"
touch "$tmp/image-missing"
"$guards" image "registry.fly.io/critical-staging@$newer" >/dev/null 2>&1 && fail "a missing image is accepted"
rm -f "$tmp/image-missing"

# --- guard: all ---
out="$("$guards" all v1.2.3 2>&1)" || fail "all v1.2.3: $out"
[ "$out" = "$(printf 'sha=%s\ndigest=%s' "$main_sha" "$newer")" ] || fail "all v1.2.3 printed: $out"
"$guards" all v9.9.9 >/dev/null 2>&1 && fail "all: a tag off main is accepted"
"$guards" all v1.2.4 >/dev/null 2>&1 && fail "all: a missing tag is accepted"

# --- guard: two versions on one commit (a forced release of a tagged head): each tag gets its own image ---
older="sha256:$(printf '1%.0s' {1..64})"
forced="sha256:$(printf '2%.0s' {1..64})"
bare="sha256:$(printf '3%.0s' {1..64})"
git tag -a v1.3.0 -m r "$main_sha"
put "$(deployments "$main_sha")" "[{\"id\":21,\"created_at\":\"2026-10-17T01:00:00Z\",\"payload\":{\"digest\":\"$older\",\"version\":\"1.2.3\"}},
  {\"id\":22,\"created_at\":\"2026-10-17T02:00:00Z\",\"payload\":\"{\\\"digest\\\":\\\"$forced\\\",\\\"version\\\":\\\"1.3.0\\\"}\"},
  {\"id\":23,\"created_at\":\"2026-10-17T00:30:00Z\",\"payload\":{\"digest\":\"$bare\",\"version\":\"\"}},
  {\"id\":24,\"created_at\":\"2026-10-17T03:00:00Z\",\"payload\":{\"digest\":\"$digest\",\"version\":\"1.2.3\"}}]"
for id in 21 22 23; do put "$(statuses "$id")" '[{"state":"inactive"},{"state":"success"}]'; done
put "$(statuses 24)" '[{"state":"failure"}]'
out="$("$guards" all v1.2.3 2>&1)" || fail "all v1.2.3 (two versions): $out"
[ "$out" = "$(printf 'sha=%s\ndigest=%s' "$main_sha" "$older")" ] || fail "all from the OLDER tag of a commit got: $out (the newest digest of the sha, not its own; a failed deploy of it ignored)"
out="$("$guards" all v1.3.0 2>&1)" || fail "all v1.3.0 (two versions): $out"
[ "$out" = "$(printf 'sha=%s\ndigest=%s' "$main_sha" "$forced")" ] || fail "all from the newer tag of a commit got: $out"
staging "$main_sha" 1.4.0
[ "$status" = 0 ] && [ "$out" = "$bare" ] || fail "a version no deploy records: not the deploy without a version: '$out'"
staging "$main_sha"
[ "$status" = 0 ] && [ "$out" = "$forced" ] || fail "no version asked: not the newest successful deploy: '$out'"
put "$(statuses 23)" '[{"state":"failure"}]'
staging "$main_sha" 1.4.0
[ "$status" != 0 ] && [[ "$out" == *"no successful deploy of $main_sha as 1.4.0"* ]] \
    || fail "a version no deploy records, the others recorded as other versions: accepted or wrong message: $out"
git tag -d v1.3.0 >/dev/null
put "$(deployments "$main_sha")" "[{\"id\":12,\"created_at\":\"2026-10-17T03:00:00Z\",\"payload\":{\"digest\":\"$newer\"}}]"

# --- resolve: the version to promote ---
# main: one, two (v1.2.3, the bare record 12 above), three (v1.9.0, passed), four (v1.10.0, passed), five
# (v1.11.0, failed), six (v3.0.0-rc1, not a version, recorded as passed); side (v9.9.9, not on main).
d9="sha256:$(printf '9%.0s' {1..64})"
d10="sha256:$(printf 'a%.0s' {1..64})"
for n in three four five six; do
    git commit -q --allow-empty -m "$n" && git push -q origin main
    declare "sha_$n=$(git rev-parse HEAD)"
done
git tag -a v1.9.0 -m r "$sha_three"; git tag -a v1.10.0 -m r "$sha_four"; git tag v1.11.0 "$sha_five"
git tag -a v3.0.0-rc1 -m r "$sha_six"; git tag v2.0 "$sha_six"
put "$(deployments "$sha_three")" "[{\"id\":31,\"created_at\":\"2026-10-17T06:00:00Z\",\"payload\":{\"digest\":\"$d9\",\"version\":\"1.9.0\"}}]"
put "$(deployments "$sha_four")" "[{\"id\":32,\"created_at\":\"2026-10-17T07:00:00Z\",\"payload\":{\"digest\":\"$d10\",\"version\":\"1.10.0\"}}]"
put "$(deployments "$sha_five")" "[{\"id\":33,\"created_at\":\"2026-10-17T08:00:00Z\",\"payload\":{\"digest\":\"$digest\",\"version\":\"1.11.0\"}}]"
put "$(deployments "$sha_six")" "[{\"id\":34,\"created_at\":\"2026-10-17T09:00:00Z\",\"payload\":{\"digest\":\"$digest\",\"version\":\"3.0.0-rc1\"}}]"
put "$(deployments "$side_sha")" "[{\"id\":35,\"created_at\":\"2026-10-17T09:00:00Z\",\"payload\":{\"digest\":\"$digest\",\"version\":\"9.9.9\"}}]"
for id in 31 32 34 35; do put "$(statuses "$id")" '[{"state":"inactive"},{"state":"success"}]'; done
put "$(statuses 33)" '[{"state":"failure"}]'
resolve() { status=0; out="$("$guards" resolve "$@" 2>"$tmp/resolve-err")" || status=$?; err="$(cat "$tmp/resolve-err")"; }
resolved() { printf 'tag=%s\nsha=%s\ndigest=%s' "$@"; }

resolve
[ "$status" = 0 ] && [ "$out" = "$(resolved v1.10.0 "$sha_four" "$d10")" ] \
    || fail "resolve, no version: not the newest version that passed staging (v1.10.0, by number): exit $status, $out $err"
[[ "$err" == *"skipping v9.9.9: release-guards: FAIL: commit $side_sha is not on main"* ]] || fail "resolve: the skipped tag off main is not said: $err"
[[ "$err" == *"skipping v1.11.0: release-guards: FAIL: staging has no successful deploy"* ]] || fail "resolve: the skipped failed version is not said: $err"
[[ "$err" != *rc1* && "$err" != *v2.0* ]] || fail "resolve: a tag not in the vX.Y.Z format was considered: $err"
resolve ""
[ "$status" = 0 ] && [ "$out" = "$(resolved v1.10.0 "$sha_four" "$d10")" ] || fail "resolve with an empty version: $out $err"
for v in 1.9.0 v1.9.0; do
    resolve "$v"
    [ "$status" = 0 ] && [ "$out" = "$(resolved v1.9.0 "$sha_three" "$d9")" ] || fail "resolve $v (an older version, the rollback): exit $status, $out $err"
done
resolve 1.2.3
[ "$status" = 0 ] && [ "$out" = "$(resolved v1.2.3 "$main_sha" "$newer")" ] || fail "resolve 1.2.3 (recorded before versions): $out $err"
resolve 1.11.0
[ "$status" != 0 ] && [ -z "$out" ] && [[ "$err" == *"staging has no successful deploy of $sha_five as 1.11.0"* ]] \
    || fail "resolve: a version that did not pass staging is accepted or not said: $out $err"
resolve 1.4.0
[ "$status" != 0 ] && [ -z "$out" ] && [[ "$err" == *"there is no version v1.4.0"* ]] || fail "resolve: a version without a tag: $out $err"
resolve 9.9.9
[ "$status" != 0 ] && [ -z "$out" ] && [[ "$err" == *"not on main"* ]] || fail "resolve: a version off main: $out $err"
for v in 1.2 v1.2 2.0 3.0.0-rc1 v3.0.0-rc1 vv1.9.0 V1.9.0 01.9.0 1.9.0.0 " 1.9.0" "1.9.0 " "1.9.0;x" '$(true)'; do
    resolve "$v"
    [ "$status" != 0 ] && [ -z "$out" ] && [[ "$err" == *"is not X.Y.Z or vX.Y.Z"* ]] || fail "resolve: the malformed version '$v' is accepted or not said: $out $err"
done
# A failed API read on the newest version stops: it is no answer, and skipping it would promote v1.9.0.
fails_with() { f="$fx/$(printf '%s' "$1" | tr -c 'a-zA-Z0-9' '_').fail"; touch "$f"; resolve; rm -f "$f"; }
for url in "$(deployments "$sha_four")" "$(statuses 32)"; do
    fails_with "$url"
    [ "$status" = 3 ] && [ -z "$out" ] && [[ "$err" == *"cannot read the"* && "$err" == *"cannot tell whether v1.10.0"* ]] \
        || fail "resolve: a failed read of $url fell back or was not said: exit $status, $out $err"
done
f="$fx/$(printf '%s' "$(statuses 31)" | tr -c 'a-zA-Z0-9' '_').fail"; touch "$f"; resolve 1.9.0; rm -f "$f"
[ "$status" != 0 ] && [ -z "$out" ] && [[ "$err" == *"cannot read the statuses"* ]] || fail "resolve 1.9.0: a failed statuses read is not refused: $out $err"
# A broken record of a version below the one resolved changes nothing.
put "$(statuses 31)" 'not json'
resolve
[ "$status" = 0 ] && [ "$out" = "$(resolved v1.10.0 "$sha_four" "$d10")" ] || fail "resolve: a broken record of a lower version mattered: $out $err"
put "$(statuses 31)" '[{"state":"inactive"},{"state":"success"}]'
for answer in '[{"state":"inactive"},{"state":"success"}],' ''; do
    put "$(statuses 32)" "$answer"
    resolve
    [ "$status" = 3 ] && [ -z "$out" ] || fail "resolve: the statuses answer '$answer' on the newest version fell back: exit $status, $out $err"
done
put "$(statuses 32)" '[{"state":"inactive"},{"state":"success"}]'
put "$(deployments "$sha_four")" "[{\"id\":32,\"created_at\":\"2026-10-17T07:00:00Z\",\"payload\":{\"version\":\"1.10.0\"}}]"
resolve
[ "$status" = 3 ] && [ -z "$out" ] && [[ "$err" == *"records no image digest"* ]] || fail "resolve: a passed record without a digest on the newest version fell back: exit $status, $out $err"
put "$(deployments "$sha_four")" "[{\"id\":32,\"created_at\":\"2026-10-17T07:00:00Z\",\"payload\":{\"digest\":\"$d10\",\"version\":\"1.10.0\"}}]"

for id in 12 31 32 34; do put "$(statuses "$id")" '[{"state":"failure"}]'; done
resolve
[ "$status" != 0 ] && [ -z "$out" ] && [[ "$err" == *"no v* version is on main with a passed staging deploy"* ]] \
    || fail "resolve: no version passed staging, yet one is resolved or it is not said: $out $err"
put "$(statuses 12)" '[{"state":"inactive"},{"state":"success"}]'
git tag -d v1.9.0 v1.10.0 v1.11.0 v3.0.0-rc1 v2.0 >/dev/null

# --- promote.sh ---
private='[{"Type":"private_v6","Address":"fdaa::1","Region":"global"}]'
public='[{"Type":"v4","Address":"1.2.3.4","Region":"global"},{"Type":"private_v6","Address":"fdaa::1","Region":"global"}]'
label='"labels":{"org.opencontainers.image.version":"1.2.3"}'
machines="[{\"id\":\"m1\",\"image_ref\":{\"digest\":\"$newer\",\"tag\":\"$main_sha\",$label}},{\"id\":\"m2\",\"image_ref\":{\"digest\":\"$newer\",\"tag\":\"$main_sha\",$label}}]"
ok_console="$(printf 'Created an ephemeral machine\nrestore-point: dumping critical_prod\nrestore-point: OK @POINT@\r\n')"
reset() {
    echo "$private" > "$tmp/ips"; echo "$machines" > "$tmp/machines"; echo "$ok_console" > "$tmp/console-out"
    rm -f "$tmp/smoke-fails" "$tmp/release-exists" "$tmp/machines-fail" "$tmp/image-missing" "$tmp/copy-fails" "$tmp/copy-missing"
}
promote() { : > "$calls"; status=0; out="$(FLY="$stubs/fly" FLY_SMOKE="$stubs/smoke" "$tmp/repo/scripts/promote.sh" "$1" 2>&1)" || status=$?; }
line() { grep -n "$1" "$calls" | head -n1 | cut -d: -f1; }
staged="registry.fly.io/critical-staging@$newer"
image="registry.fly.io/critical-prod@$newer"

reset; promote v1.2.3
[ "$status" = 0 ] || fail "promote: exit $status: $out"
point="$(sed -n 's/.*restore-point.sh \([a-z0-9_]*\).*/\1/p' "$calls" | head -n1)"
[[ "$point" =~ ^critical_prod_pre_v1_2_3_[0-9]{14}$ ]] || fail "promote: restore point name '$point'"
grep -qx "docker buildx imagetools create --prefer-index=false --progress none -t registry.fly.io/critical-prod:$main_sha $staged" "$calls" \
    || fail "promote: wrong copy call: $(grep buildx "$calls")"
grep -qx "docker manifest inspect $image" "$calls" || fail "promote: the copy's digest is not checked"
grep -qx "fly console -a critical-prod --image $image --vm-size shared-cpu-1x --vm-memory 512 --file-local /tmp/restore-point.sh=$tmp/repo/scripts/restore-point.sh -C sh /tmp/restore-point.sh $point 001_baseline.sql 002_more.sql" "$calls" \
    || fail "promote: wrong console call: $(grep console "$calls")"
grep -qx "fly deploy -c $tmp/repo/deploy/fly.prod.toml -a critical-prod --image $image --no-public-ips --yes" "$calls" \
    || fail "promote: wrong deploy call: $(grep deploy "$calls")"
grep -qx "smoke critical-prod $main_sha" "$calls" || fail "promote: smoke not called with the app and the tag's commit"
grep -q "^gh release create v1.2.3 --verify-tag --title v1.2.3 --generate-notes --notes .*$newer" "$calls" || fail "promote: no GitHub Release with the digest"
[[ "$out" == *"promote: production (critical-prod) runs 1.2.3 (commit $main_sha, $newer); promoting v1.2.3"* ]] \
    || fail "promote: what production runs is not printed: $out"
[ "$(line '^fly machine list')" -lt "$(line '^fly ips')" ] || fail "promote: production's current version is not printed before the promotion starts"
reset; echo "[{\"id\":\"m1\",\"image_ref\":{\"digest\":\"$newer\",\"tag\":\"$main_sha\"}}]" > "$tmp/machines"; promote v1.2.3
[[ "$out" == *"runs no version (commit $main_sha, $newer)"* ]] || fail "promote: an image without a version label is not printed as such: $out"

# The newest version (none asked) never goes below production; a version asked (a rollback) may.
runs() { printf '[{"id":"m1","image_ref":{"digest":"%s","tag":"%s","labels":{"org.opencontainers.image.version":"%s"}}},{"id":"m2","image_ref":{"digest":"%s","tag":"%s","labels":{"org.opencontainers.image.version":"%s"}}}]' \
    "$newer" "$main_sha" "$1" "$newer" "$main_sha" "$2" > "$tmp/machines"; }
newest() { PROMOTE_NEWEST=true promote "$@"; }
for pair in "1.10.0 1.10.0" "1.2.3 1.2.4" "2.0.0 1.0.0" "1.2.10 1.2.3"; do
    reset; runs $pair; newest v1.2.3
    [ "$status" != 0 ] && [[ "$out" == *"the newest staging version v1.2.3 is older than production's"* ]] \
        || fail "promote: the newest version below production ($pair) is not refused: $out"
    grep -qE '^(fly (ips|console|deploy)|docker buildx)' "$calls" && fail "promote: the newest version below production ($pair) went on: $(cat "$calls")"
done
for pair in "1.2.3 1.2.3" "1.0.9 1.2.2" "0.9.10 1.1.0"; do
    reset; runs $pair; newest v1.2.3
    [[ "$out" != *"is older than production"* ]] && grep -q '^fly console' "$calls" || fail "promote: the newest version at or above production ($pair) is refused: $out"
done
reset; runs 1.10.0 1.10.0; promote v1.2.3
grep -q '^fly console' "$calls" || fail "promote: a version asked below production (a rollback) is refused: $out"
reset; touch "$tmp/machines-fail"; newest v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"what production runs could not be read"* ]] && ! grep -q '^fly ips' "$calls" \
    || fail "promote: the newest version with production unreadable is not refused before anything: $out"
reset; echo '[]' > "$tmp/machines"; newest v1.2.3
grep -q '^fly console' "$calls" && [[ "$out" == *"runs nothing (no machine)"* ]] || fail "promote: the newest version on an app without machines is refused: $out"
reset; echo "[{\"id\":\"m1\",\"image_ref\":{\"digest\":\"$newer\",\"tag\":\"$main_sha\"}}]" > "$tmp/machines"; newest v1.2.3
grep -q '^fly console' "$calls" || fail "promote: the newest version over an image without a version label is refused: $out"
[ "$(line '^docker manifest')" -lt "$(line '^fly ips')" ] && [ "$(line '^fly ips')" -lt "$(line '^docker buildx')" ] \
    && [ "$(line "^docker manifest inspect $image")" -lt "$(line '^fly console')" ] \
    && [ "$(line '^fly console')" -lt "$(line '^fly deploy')" ] && [ "$(line '^fly deploy')" -lt "$(line '^smoke')" ] \
    && [ "$(line '^smoke')" -lt "$(line '^gh release create')" ] || fail "promote: steps out of order: $(cat "$calls")"

# The embedded migrations are the tag's, not the checkout's: one added on a later main commit is not listed.
git switch -q --detach "$main_sha"
touch api/internal/storage/postgres/migrations/003_later.sql
git add -A && git commit -q -m later
reset; promote v1.2.3
grep -Eq "restore-point.sh critical_prod_pre_v1_2_3_[0-9]{14} 001_baseline.sql 002_more.sql\$" "$calls" \
    || fail "promote: the migrations are not read from the tag's tree: $(grep console "$calls")"
git switch -q main

reset; promote v9.9.9
[ "$status" != 0 ] || fail "promote: a tag off main is promoted"
grep -q '^fly' "$calls" && fail "promote: a refused guard still called fly: $(cat "$calls")"

reset; promote v1.2
[ "$status" != 0 ] && ! grep -q '^fly' "$calls" || fail "promote: a malformed tag called fly"

reset; touch "$tmp/image-missing"; promote v1.2.3
[ "$status" != 0 ] && ! grep -q '^fly' "$calls" || fail "promote: a missing image called fly"

reset; touch "$tmp/copy-fails"; promote v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"could not copy"* ]] || fail "promote: a failed copy is not refused: $out"
grep -qE '^fly (console|deploy)' "$calls" && fail "promote: a failed copy still took a restore point or deployed"
reset; touch "$tmp/copy-missing"; promote v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"is not $newer"* ]] || fail "promote: a copy under another digest is not refused: $out"
grep -qE '^fly (console|deploy)' "$calls" && fail "promote: a copy under another digest still took a restore point or deployed"

reset; echo "$public" > "$tmp/ips"; promote v1.2.3
[ "$status" != 0 ] || fail "promote: a public IP is promoted"
grep -qE '^(fly (console|deploy)|docker buildx)' "$calls" && fail "promote: a public IP still copied, took a restore point or deployed"
PUBLIC_ACCESS_OPTION=basic-auth promote v1.2.3
[ "$status" != 0 ] || fail "promote: PUBLIC_ACCESS_OPTION bypassed the mandatory no-public-IP check"

reset; printf 'restore-point: FAIL: the database user lacks CREATEDB\n' > "$tmp/console-out"; promote v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"no restore point"* ]] || fail "promote: a failed restore point is not refused: $out"
grep -q '^fly deploy' "$calls" && fail "promote: deployed without a restore point"
printf 'restore-point: OK critical_prod_pre_v1_2_3_20000101000000\n' > "$tmp/console-out"; promote v1.2.3
grep -q '^fly deploy' "$calls" && fail "promote: deployed on the OK line of another restore point"

reset; touch "$tmp/smoke-fails"; promote v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"registry.fly.io/critical-staging@previous"* ]] || fail "promote: a failed smoke check: exit $status, no releases listed: $out"
grep -q '^gh release create' "$calls" && fail "promote: released after a failed smoke check"

reset; echo "[{\"id\":\"m1\",\"image_ref\":{\"digest\":\"$newer\"}},{\"id\":\"m2\",\"image_ref\":{\"digest\":\"$digest\"}}]" > "$tmp/machines"
promote v1.2.3
[ "$status" != 0 ] && ! grep -q '^gh release create' "$calls" || fail "promote: machines on another digest still released"

reset; touch "$tmp/release-exists"; promote v1.2.3
[ "$status" = 0 ] && grep -q '^fly deploy' "$calls" && ! grep -q '^gh release create' "$calls" \
    || fail "promote: a re-run (existing release) did not redeploy, or created the release again"

cd "$root"

# --- restore-point.sh: refusals before any database call ---
# shellcheck disable=SC2086 # point_arg is the name and the migrations, split on purpose
rp() { status=0; out="$(env PATH="$stubs:/usr/bin:/bin" "$@" sh "$root/scripts/restore-point.sh" ${point_arg:-} 2>&1)" || status=$?; }
for point_arg in critical_prod_pre_v1_2_3 critical_prod_pre_v1_2_3_2026 'critical_prod_pre_v1_2_3_20261017000000;drop' Critical_pre_v1_2_3_20261017000000; do
    point_arg="$point_arg 001_x.sql"
    rp CRITICAL_DSN=postgres://u:p@h:5432/critical_prod; [ "$status" != 0 ] && [[ "$out" == *"malformed restore point"* ]] || fail "restore-point accepted '$point_arg': $out"
done
point_arg=critical_prod_pre_v1_2_3_20261017000000
rp CRITICAL_DSN=postgres://u:p@h:5432/critical_prod; [ "$status" != 0 ] && [[ "$out" == *"no embedded migration"* ]] || fail "restore-point without the embedded migrations: $out"
point_arg="critical_prod_pre_v1_2_3_20261017000000 001_x.sql"
rp A=1; [ "$status" != 0 ] && [[ "$out" == *"CRITICAL_DSN is not set"* ]] || fail "restore-point without a DSN: $out"
rp CRITICAL_DSN=postgres://u:p@h:5432/critical_prod RESTORE_POINT_KEEP=0; [ "$status" != 0 ] || fail "restore-point accepted a retention of 0"
[[ "$out" != *u:p@* ]] || fail "restore-point printed the DSN"
for m in "001_x.sql;drop" "001_x" "'001_x.sql'" "../001.sql"; do
    point_arg="critical_prod_pre_v1_2_3_20261017000000 $m"
    rp CRITICAL_DSN=postgres://u:p@h:5432/critical_prod; [ "$status" != 0 ] && [[ "$out" == *"malformed migration"* ]] || fail "restore-point accepted the migration '$m': $out"
done

# --- require-admin.sh: who may promote ---
perm() { put "repos/o/r/collaborators/$1/permission" "{\"permission\":\"$2\",\"role_name\":\"$2\"}"; }
perm boss admin; perm dev write; perm keeper maintain; perm reader read; perm 'github-actions[bot]' write
admin() { : > "$calls"; status=0; out="$("$root/scripts/require-admin.sh" "$@" 2>&1)" || status=$?; }
admin boss boss
[ "$status" = 0 ] && grep -qx "gh api repos/o/r/collaborators/boss/permission --jq .permission" "$calls" || fail "require-admin: an admin is refused: $out"
for who in dev keeper reader stranger 'github-actions[bot]'; do
    admin "$who"
    [ "$status" != 0 ] && [[ "$out" == *"$who is not an admin of o/r"* ]] || fail "require-admin: '$who' passes or is not named: $out"
done
admin boss dev
[ "$status" != 0 ] && [[ "$out" == *"dev is not an admin"* ]] || fail "require-admin: a re-run by a non-admin of an admin's run passes: $out"
admin dev boss
[ "$status" != 0 ] || fail "require-admin: a non-admin actor passes when the re-runner is an admin"
for who in 'boss;x' '../boss' '' 'boss/permission'; do
    admin "$who"
    [ "$status" != 0 ] || fail "require-admin: the user name '$who' passes"
done
admin
[ "$status" != 0 ] || fail "require-admin: no user passes"

# --- github-environments.sh: production deployable from main only ---
envstubs="$tmp/env-bin"
mkdir -p "$envstubs"
cat > "$envstubs/gh" <<STUB
#!/usr/bin/env bash
echo "gh \$*" >> "$calls"
case "\$*" in
    "repo view"*) echo o/r ;;
    "secret list"*) echo FLY_API_TOKEN ;;
    "api -X"*) ;;
    "api repos/o/r/environments/staging/deployment-branch-policies --jq"*)
        echo '{"branch_policies":[{"id":1,"name":"main","type":"branch"}]}' | jq -r "\$4" ;;
    "api repos/o/r/environments/production/deployment-branch-policies --jq"*)
        echo '{"branch_policies":[{"id":7,"name":"v*","type":"tag"},{"id":8,"name":"main","type":"tag"}]}' | jq -r "\$4" ;;
    *) exit 1 ;;
esac
STUB
chmod +x "$envstubs/gh"
: > "$calls"
status=0; out="$(env PATH="$envstubs:$PATH" FLY=/bin/false "$root/scripts/github-environments.sh" --no-tokens 2>&1)" || status=$?
prod="repos/o/r/environments/production/deployment-branch-policies"
[ "$status" = 0 ] || fail "github-environments --no-tokens: exit $status: $out"
grep -qx "gh api -X POST $prod -f name=main -f type=branch" "$calls" || fail "github-environments: production gets no main branch rule: $(cat "$calls")"
grep -qx "gh api -X DELETE $prod/7" "$calls" && grep -qx "gh api -X DELETE $prod/8" "$calls" \
    || fail "github-environments: production keeps a rule other than the branch main: $(cat "$calls")"
[ "$(grep -n "POST $prod" "$calls" | cut -d: -f1)" -lt "$(grep -n "DELETE $prod/7" "$calls" | cut -d: -f1)" ] \
    || fail "github-environments: the old rule is deleted before the new one exists"
grep -E "gh api -X (POST|DELETE) repos/o/r/environments/staging/deployment-branch-policies" "$calls" \
    && fail "github-environments: staging's main rule is changed"
grep -q "type=tag" "$calls" && fail "github-environments: a tag rule is still created"

# --- just promote: the dispatch from main ---
: > "$calls"
just -f "$root/justfile" -d "$root" promote 1.2.3 >/dev/null 2>&1 || fail "just promote 1.2.3 failed"
just -f "$root/justfile" -d "$root" promote >/dev/null 2>&1 || fail "just promote failed"
[ "$(cat "$calls")" = "$(printf 'gh workflow run promote.yml --ref main -f version=1.2.3\ngh workflow run promote.yml --ref main')" ] \
    || fail "just promote: wrong dispatches: $(cat "$calls")"

# --- the workflow, parsed ---
python3 -I - "$root/.github/workflows/promote.yml" <<'PY' || fails=$((fails + 1))
import subprocess, sys, yaml

wf = yaml.safe_load(open(sys.argv[1]))
errs = []
def check(ok, msg):
    if not ok:
        errs.append(msg)

on = wf.get(True) or wf.get("on")  # PyYAML reads the key `on` as True
check(set(on) == {"workflow_dispatch"}, "promote: not run by hand only (no tag push may start it)")
inputs = (on["workflow_dispatch"] or {}).get("inputs", {})
check(set(inputs) == {"version"} and inputs["version"].get("required") is False and inputs["version"].get("default") == "",
      "promote: the only input is an optional version, empty by default (the newest staging version)")
check(wf["permissions"] == {"contents": "read"}, "promote: workflow permissions are not read-only")
check(sorted(wf["jobs"]) == ["not-main", "promote", "resolve"], "promote: the jobs are not-main, resolve and promote")
main = "github.ref == 'refs/heads/main'"
refuse = wf["jobs"]["not-main"]
check(refuse.get("if") == "${{ github.ref != 'refs/heads/main' }}", "not-main: not the complement of the other jobs' condition")
check("environment" not in refuse and "secrets." not in str(refuse), "not-main: reads an environment or a secret")
for ref in ("refs/tags/v1.2.3", "refs/heads/feat"):
    r = subprocess.run(["bash", "-e", "-c", refuse["steps"][-1]["run"]], env={"REF": ref, "PATH": "/usr/bin:/bin"}, capture_output=True, text=True)
    check(r.returncode != 0 and "is not main" in r.stdout, f"not-main: a run from {ref} does not fail saying so")
for name, job in wf["jobs"].items():
    for st in job.get("steps", []):
        check("inputs." not in str(st.get("run", "")) and "github.event" not in str(st.get("run", "")),
              f"{name}: a run script interpolates an input (pass it through env)")
admin_run = 'scripts/require-admin.sh "$ACTOR" "$TRIGGERING_ACTOR"'
def admin_first(name, steps):
    names = [st.get("name", st.get("uses", "")) for st in steps]
    a = [i for i, st in enumerate(steps) if st.get("run", "").strip() == admin_run]
    check(len(a) == 1, f"{name}: no admin check")
    if a:
        e = steps[a[0]]["env"]
        check(e.get("ACTOR") == "${{ github.actor }}" and e.get("TRIGGERING_ACTOR") == "${{ github.triggering_actor }}"
              and e.get("GH_TOKEN") == "${{ github.token }}" and "secrets." not in str(e),
              f"{name}: the admin check does not check the actor and the re-runner with the workflow token")
        check(all(not st.get("run") and "secrets." not in str(st) for st in steps[:a[0]]),
              f"{name}: something runs before the admin check: {names[:a[0]]}")
    return a[0] if a else 0
res = wf["jobs"]["resolve"]
check(res.get("if") == main and "environment" not in res and "secrets." not in str(res) and "uses" not in res,
      "resolve: not main-only, or in an environment, or reads a secret")
check(res["permissions"] == {"contents": "read", "deployments": "read"}, "resolve: permissions are not minimal")
check(res["outputs"] == {"tag": "${{ steps.resolve.outputs.tag }}"}, "resolve: does not output the resolved tag")
admin_first("resolve", res["steps"])
rs = [st for st in res["steps"] if st.get("id") == "resolve"]
check(len(rs) == 1 and rs[0]["env"].get("VERSION") == "${{ inputs.version }}" and 'release-guards.sh resolve "$VERSION"' in rs[0]["run"],
      "resolve: the version is not resolved from the input by release-guards.sh resolve")
job = wf["jobs"]["promote"]
check(job.get("needs") == "resolve", "promote: does not wait for the admin check and the version")
check("uses" not in job, "promote: calls a reusable workflow, which cannot read the environment's secrets")
check(job.get("environment") == "production", "promote: not in the production environment")
check(job.get("if") == main, "promote: not limited to main")
check(job["concurrency"] == {"group": "deploy-production", "cancel-in-progress": False}, "promote: not serialized without cancelling")
check(job["permissions"] == {"contents": "write", "deployments": "read"}, "promote: permissions are not minimal")
steps = job["steps"]
a = admin_first("promote", steps)
readers = [i for i, st in enumerate(steps) if "secrets." in str(st)]
check([steps[i].get("name") for i in readers] == ["Check the deploy secret", "Promote"], f"promote: the token is read by {readers}")
check(all(i > a for i in readers), "promote: a secret is read before the admin check")
check(all("if" not in st for st in steps), "promote: a conditional step could skip the promotion")
for name, j in (("resolve", res), ("promote", job)):
    co = [st for st in j["steps"] if st.get("uses", "").startswith("actions/checkout@")]
    check(len(co) == 1 and co[0]["with"].get("fetch-depth") == 0 and co[0]["with"].get("persist-credentials") is False,
          f"{name}: checkout must fetch the whole history and the tags, without persisted credentials")
    check(bool(co) and "ref" not in co[0]["with"], f"{name}: the checkout must be main's (its scripts, its fly config)")
check(steps[-1]["env"].get("PROMOTE_NEWEST") == "${{ inputs.version == '' }}",
      "promote: promote.sh is not told when no version was asked (the newest never goes below production)")
check("scripts/promote.sh \"$TAG\"" in steps[-1]["run"] and steps[-1]["env"].get("TAG") == "${{ needs.resolve.outputs.tag }}",
      "promote: the last step does not promote the resolved tag")
secret = [st for st in steps if st.get("name") == "Check the deploy secret"]
run = secret[0]["run"] if secret else "exit 1"
for token, want_ok in (("", False), ("s3cret-value", True)):
    r = subprocess.run(["bash", "-e", "-c", run], env={"FLY_API_TOKEN": token, "PATH": "/usr/bin:/bin"}, capture_output=True, text=True)
    check((r.returncode == 0) == want_ok, f"promote: secret check with token {token!r} exited {r.returncode}")
    check(token == "" or token not in r.stdout + r.stderr, "promote: secret check printed the token")
if errs:
    print("\n".join("test-promotion: FAIL: " + e for e in errs), file=sys.stderr)
    sys.exit(1)
PY

[ "$fails" = 0 ] || exit 1
echo "--- Promotion scripts ---"
