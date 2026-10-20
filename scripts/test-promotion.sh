#!/usr/bin/env bash
# Tests the promotion's scripts without the network, fly or GitHub (part of `just validate`, ADR 0024
# steps 4, 8.3 and 10.4), on fixture git repositories with fly, gh, docker and the smoke check stubbed:
#   - release-guards.sh: each guard (tag format, on main, the staging record read as docs/deploy.md says,
#     the image in the registry) and `all`, which prints the tag's commit and the digest staging ran (as
#     that tag's version when one commit carries two);
#   - promote.sh: guards, then the no-public-IP check, then the copy of the staged image by digest into the
#     production repository (and its digest checked), then the restore point in a console machine of it
#     (the dump block kept out of the log, checked, encrypted, decryptable with the passphrase), then the
#     deploy by digest, the smoke check, the digest check and the GitHub Release naming where the restore
#     point is kept; each failure stops everything after it (a refused guard calls fly not at all; no
#     passphrase, nothing; no restore point, or a cut or damaged one, no deploy; a failed smoke or a digest
#     mismatch, no release); `prepare` and `deploy` apart (the workflow's two steps around the upload):
#     prepare deploys nothing, deploy refuses without prepare's record, for another tag, or without
#     RESTORE_POINT_KEPT;
#   - restore-point.sh: what it refuses without a database (its SQL runs for real in scripts/image-smoke.sh);
#   - restore-point-fetch.sh: the newest unexpired restore point of a version (or the one named) downloaded,
#     decrypted and unpacked as a backup; a wrong passphrase, a missing one, a foreign tar member, an
#     existing folder and a malformed name refused, keeping nothing;
#   - restore-point-key.sh: a new passphrase file (mode 600) set as the production secret through stdin,
#     never overwritten;
#   - release-guards.sh resolve: no version, the newest (by number) v* version on main that passed staging,
#     skipping higher ones that did not; a version asked (X.Y.Z or vX.Y.Z, the rollback) that must exist, be on
#     main and have passed staging; a malformed one refused;
#   - release-guards.sh candidates (what `just promote` offers; one implementation with `resolve`): the versions
#     above production's recorded version up to staging's, newest first, each passed staging as that version;
#     production unknown (no record): the staged versions, at most 20; production at or above staging's: none;
#     staging unknown or any failed read: exit 3, nothing offered;
#   - record-deployment.sh --production, and promote.sh writing that record only after the deploy and smoke
#     check passed (a failed smoke check, a refused guard, no record), a failed write failing the run;
#   - scripts/promote-pick.sh (gum stubbed): the picker with staging preselected, Esc promoting nothing, prod
#     already at staging, an unknown production, a failed read refusing, gum missing, a named version never
#     prompting, no terminal and no version dispatching with none;
#   - require-admin.sh: only repository admins pass (who may promote);
#   - github-environments.sh: production deployable from the branch main only (the old tag rule deleted);
#   - `just promote [version]`: the dispatch of promote.yml from main, with the version as its input;
#   - promote.yml, parsed: a run from another ref than main fails saying so; by hand only (no trigger on a tag
#     push), with an optional version input; the admin check before any secret in both jobs, the version
#     resolved without any secret, the production environment, serialized and never cancelled, the secrets
#     read only by the secret check (which fails on an empty token or passphrase) and the two promote.sh
#     steps, the restore point uploaded (90 days, encrypted file, an error when missing) between them, the
#     deploy step refusing without an artifact id.
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
    "api -X")
        shift 2; echo "gh api -X \$*" >> "$calls"; cat >> "$tmp/gh_posts"; echo >> "$tmp/gh_posts"
        [ ! -f "$tmp/record-fails" ] || { echo "gh: HTTP 422" >&2; exit 1; }
        case "\$*" in POST\ repos/o/r/deployments\ *) echo 77 ;; esac ;;
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
cat > "$stubs/gate" <<STUB
#!/bin/sh
echo "gate \$*" >> "$calls"
[ ! -f "$tmp/gate-fails" ]
STUB
chmod +x "$stubs/gate"
echo '[]' > "$fx/empty"
cat > "$stubs/docker" <<STUB
#!/bin/sh
echo "docker \$*" >> "$calls"
case "\$*" in
    "buildx imagetools create"*) [ ! -f "$tmp/copy-fails" ] ;;
    *critical-hit@*) [ ! -f "$tmp/copy-missing" ] ;;
    *) [ ! -f "$tmp/image-missing" ] ;;
esac
STUB
cat > "$stubs/fly" <<STUB
#!/bin/sh
echo "fly \$*" >> "$calls"
case "\$1 \$2" in
    "ips list") cat "$tmp/ips" ;;
    "machine list") [ ! -f "$tmp/machines-fail" ] || exit 1; cat "$tmp/machines" ;;
    "releases --image") echo "v3 complete registry.fly.io/critical-hit-staging@previous" ;;
    "console -a")
        point="\$(echo "\$*" | sed -n 's/.*restore-point.sh \([a-z0-9_]*\).*/\1/p')"
        if [ -f "$tmp/console-out" ]; then sed "s/@POINT@/\$point/" "$tmp/console-out"; exit 0; fi
        sum="\$(sha256sum "$tmp/point.tar" | cut -d' ' -f1)"; bytes="\$(wc -c <"$tmp/point.tar" | tr -d ' ')"
        [ ! -f "$tmp/console-damage" ] || sum="\$(printf '0%.0s' \$(seq 64))"
        echo "Connecting to fdaa::2... complete" >&2
        {
            echo "restore-point: dumping critical_prod"
            echo "restore-point: BEGIN \$point"
            if [ -f "$tmp/console-cut" ]; then base64 "$tmp/point.tar" | head -n 2; exit 0; fi
            base64 "$tmp/point.tar"
            echo "restore-point: END \$point"
            echo "restore-point: OK \$point \$sum \$bytes"
        } | sed 's/\$/\r/' ;;
esac
STUB
cat > "$stubs/smoke" <<STUB
#!/bin/sh
echo "smoke \$*" >> "$calls"
[ ! -f "$tmp/smoke-fails" ]
STUB
chmod +x "$stubs"/*
export PATH="$stubs:$PATH" GITHUB_REPOSITORY=o/r
# The restore point the console machine sends back: a tar of a dump (random bytes) and its table counts.
mkdir -p "$tmp/pointsrc"
head -c 20000 /dev/urandom >"$tmp/pointsrc/critical_prod.dump"
printf 'characters 3\nschema_migrations 9\n' >"$tmp/pointsrc/counts.txt"
tar -C "$tmp/pointsrc" -cf "$tmp/point.tar" critical_prod.dump counts.txt
passphrase="test-passphrase-$RANDOM-not-a-secret"
# decrypt <file> [passphrase]: the plaintext on stdout, with a throwaway GNUPGHOME.
decrypt() {
    local home; home="$(mktemp -d)"
    GNUPGHOME="$home" gpg --batch --quiet --no-tty --pinentry-mode loopback --no-symkey-cache --passphrase-fd 3 \
        --decrypt "$1" 3<<<"${2:-$passphrase}" 2>/dev/null
    local s=$?; rm -rf "$home"; return "$s"
}

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
cp "$root"/scripts/{release-guards,promote,promote-pick,record-deployment,restore-point,prod-db,fly-private-check,fly-access-check}.sh scripts/
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
"$guards" image "registry.fly.io/critical-hit-staging@$newer" >/dev/null 2>&1 || fail "an image in the registry is refused"
grep -qx "docker manifest inspect registry.fly.io/critical-hit-staging@$newer" "$calls" || fail "image: wrong docker call: $(cat "$calls")"
touch "$tmp/image-missing"
"$guards" image "registry.fly.io/critical-hit-staging@$newer" >/dev/null 2>&1 && fail "a missing image is accepted"
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

# --- candidates: what `just promote` offers (release-guards.sh candidates) ---
# Here: v1.2.3 (the bare record 12), v1.9.0 and v1.10.0 passed, v1.11.0 failed, v9.9.9 off main.
cur() { echo "repos/o/r/deployments?environment=$1&task=deploy-$1&per_page=100"; }
# record <env> <id> <created> <version> <state>: one record of the environment, with its status.
records=""
set_current() { # <env> <json list>
    put "$(cur "$1")" "$2"
}
cands() { status=0; out="$("$guards" candidates 2>"$tmp/cand-err")" || status=$?; err="$(cat "$tmp/cand-err")"; }
versions() { sed -n 's/^version=//p' <<<"$out" | tr '\n' ' '; }
rec() { printf '{"id":%s,"created_at":"%s","payload":{"version":"%s","digest":"%s"}}' "$1" "$2" "$3" "$digest"; }
put "$(statuses 41)" '[{"state":"inactive"},{"state":"success"}]'
put "$(statuses 42)" '[{"state":"failure"}]'
put "$(statuses 51)" '[{"state":"success"}]'
put "$(statuses 52)" '[{"state":"failure"}]'
put "$(statuses 53)" '[{"state":"success"}]'

# production unknown (no production record): the staged versions up to staging's, newest first
set_current staging "[$(rec 41 2026-10-18T01:00:00Z 1.10.0)]"
cands
[ "$status" = 0 ] && [ "$(head -n2 <<<"$out" | tr '\n' ' ')" = "staging=1.10.0 production=unknown " ] && [ "$(versions)" = "1.10.0 1.9.0 1.2.3 " ] \
    || fail "candidates, production unknown: exit $status, '$out' $err"
[ "$("$guards" resolve | sed -n 's/^tag=v//p')" = "$(sed -n 's/^version=//p' <<<"$out" | head -n1)" ] \
    || fail "candidates and resolve disagree on the newest staged version"
# the floor: production's recorded version (a newer FAILED production record and an older one change nothing)
set_current production "[$(rec 52 2026-10-18T05:00:00Z 1.10.0),$(rec 51 2026-10-18T04:00:00Z 1.9.0),$(rec 53 2026-10-18T02:00:00Z 1.2.3)]"
cands
[ "$status" = 0 ] && [[ "$out" == *"production=1.9.0"* ]] && [ "$(versions)" = "1.10.0 " ] || fail "candidates, production at 1.9.0: '$out' $err"
# production at staging's version: nothing to offer
set_current production "[$(rec 51 2026-10-18T04:00:00Z 1.10.0)]"
cands
[ "$status" = 0 ] && [[ "$out" == *"production=1.10.0"* ]] && [ -z "$(versions)" ] || fail "candidates, production at staging's version: '$out' $err"
# production ahead of staging (a rollback is asked by name, never offered)
set_current production "[$(rec 51 2026-10-18T04:00:00Z 1.11.0)]"
cands
[ "$status" = 0 ] && [ -z "$(versions)" ] || fail "candidates, production ahead of staging: '$out' $err"
# the ceiling: staging's recorded version, not the highest tag
set_current production '[]'
set_current staging "[$(rec 41 2026-10-18T01:00:00Z 1.9.0)]"
cands
[ "$status" = 0 ] && [ "$(versions)" = "1.9.0 1.2.3 " ] || fail "candidates, staging at 1.9.0: '$out' $err"
# a version that did not pass staging is skipped, saying why (staging at 1.11.0, production at 1.2.3)
put "$(statuses 41)" '[{"state":"success"}]'
set_current staging "[$(rec 41 2026-10-18T06:00:00Z 1.11.0)]"
set_current production "[$(rec 51 2026-10-18T04:00:00Z 1.2.3)]"
cands
[ "$status" = 0 ] && [ "$(versions)" = "1.10.0 1.9.0 " ] && [[ "$err" == *"skipping v1.11.0"* ]] || fail "candidates, a failed version in range: '$out' $err"
# staging's latest record is the one that counts, and a failed one is not a record
set_current staging "[$(rec 42 2026-10-18T09:00:00Z 1.11.0),$(rec 41 2026-10-18T06:00:00Z 1.9.0)]"
cands
[ "$status" = 0 ] && [[ "$out" == staging=1.9.0* ]] || fail "candidates: a failed staging record is staging's current: '$out' $err"
# at most 20 versions
git tag -a v5.0.0 -m r "$sha_four"
list=""; for n in $(seq 1 25); do git tag -a "v5.$n.0" -m r "$sha_four"; list="$list${list:+,}$(rec $((100 + n)) 2026-10-17T07:00:00Z "5.$n.0")"; put "$(statuses $((100 + n)))" '[{"state":"success"}]'; done
git tag -d v5.0.0 >/dev/null
saved="$(cat "$fx/$(printf '%s' "$(deployments "$sha_four")" | tr -c 'a-zA-Z0-9' '_')")"
put "$(deployments "$sha_four")" "[$list]"
set_current staging "[$(rec 41 2026-10-18T06:00:00Z 5.25.0)]"
set_current production '[]'
cands
[ "$status" = 0 ] && [ "$(sed -n 's/^version=//p' <<<"$out" | wc -l)" = 20 ] && [ "$(versions | cut -d' ' -f1)" = 5.25.0 ] && [[ "$(versions)" == *"5.6.0 " ]] \
    || fail "candidates: not the newest 20: $(versions)"
for n in $(seq 1 25); do git tag -d "v5.$n.0" >/dev/null; done
put "$(deployments "$sha_four")" "$saved"
# staging unknown, or a failed read: exit 3, nothing offered (never a silent fallback)
set_current staging '[]'
cands
[ "$status" = 3 ] && [ -z "$out" ] && [[ "$err" == *"staging has no recorded version"* ]] || fail "candidates, no staging record: exit $status '$out' $err"
set_current staging "[$(rec 41 2026-10-18T06:00:00Z 1.10.0)]"
set_current production "[$(rec 51 2026-10-18T04:00:00Z 1.9.0)]"
for url in "$(cur staging)" "$(cur production)" "$(statuses 41)" "$(statuses 51)" "$(deployments "$sha_four")"; do
    f="$fx/$(printf '%s' "$url" | tr -c 'a-zA-Z0-9' '_').fail"; touch "$f"; cands; rm -f "$f"
    [ "$status" = 3 ] && [ -z "$(versions)" ] || fail "candidates: a failed read of $url went on: exit $status '$out' $err"
done
put "$(statuses 51)" 'not json'
cands
[ "$status" = 3 ] || fail "candidates: unreadable production statuses went on: exit $status '$out'"
put "$(statuses 51)" '[{"state":"success"}]'
cands
[ "$status" = 0 ] && [ "$(versions)" = "1.10.0 " ] || fail "candidates, restored: '$out' $err"

# --- the picker (scripts/promote-pick.sh), gum stubbed ---
cat > "$stubs/gum" <<STUB
#!/bin/sh
echo "gum \$*" >> "$calls"
[ ! -f "$tmp/gum-cancel" ] || exit 130
cat "$tmp/gum-pick"
STUB
chmod +x "$stubs/gum"
pick() { : > "$calls"; status=0; out="$(PROMOTE_NO_FETCH=1 "$@" "$tmp/repo/scripts/promote-pick.sh" 2>&1 </dev/null)" || status=$?; }
dispatches() { grep -c '^gh workflow run' "$calls" || true; }
# a terminal, production at 1.9.0, staging at 1.10.0: the choice goes to the workflow
set_current staging "[$(rec 41 2026-10-18T06:00:00Z 1.10.0)]"
set_current production "[$(rec 51 2026-10-18T04:00:00Z 1.2.3)]"
echo 1.9.0 > "$tmp/gum-pick"; rm -f "$tmp/gum-cancel"
pick env PROMOTE_FORCE_TTY=1
[ "$status" = 0 ] && grep -qx 'gum choose --header Promote to production (now: 1.2.3): --selected 1.10.0 --limit 1 1.10.0 1.9.0' "$calls" \
    && grep -qx 'gh workflow run promote.yml --ref main -f version=1.9.0' "$calls" && [ "$(dispatches)" = 1 ] \
    || fail "picker: the choice is not dispatched, or staging is not preselected: exit $status $out $(cat "$calls")"
# Esc / Ctrl-C: nothing promoted
touch "$tmp/gum-cancel"
pick env PROMOTE_FORCE_TTY=1
[ "$status" = 0 ] && [ "$(dispatches)" = 0 ] && [[ "$out" == *"cancelled, nothing promoted"* ]] || fail "picker: Esc promoted or failed: exit $status $out $(cat "$calls")"
rm -f "$tmp/gum-cancel"
# production already at staging's version
set_current production "[$(rec 51 2026-10-18T04:00:00Z 1.10.0)]"
pick env PROMOTE_FORCE_TTY=1
[ "$status" = 0 ] && [ "$(dispatches)" = 0 ] && ! grep -q '^gum' "$calls" && [[ "$out" == *"production already runs staging's version (1.10.0): nothing to promote"* ]] \
    || fail "picker: prod at staging's version: exit $status $out $(cat "$calls")"
# production version unknown: says so, lists the staged versions
set_current production '[]'
echo 1.10.0 > "$tmp/gum-pick"
pick env PROMOTE_FORCE_TTY=1
[ "$status" = 0 ] && [[ "$out" == *"production version unknown"* ]] && grep -qx 'gum choose --header Promote to production (now: unknown): --selected 1.10.0 --limit 1 1.10.0 1.9.0 1.2.3' "$calls" \
    && grep -qx 'gh workflow run promote.yml --ref main -f version=1.10.0' "$calls" || fail "picker: production unknown: exit $status $out $(cat "$calls")"
# a failed GitHub read: refused, suggests the explicit version, never a fallback dispatch
f="$fx/$(printf '%s' "$(cur production)" | tr -c 'a-zA-Z0-9' '_').fail"; touch "$f"
pick env PROMOTE_FORCE_TTY=1; rm -f "$f"
[ "$status" != 0 ] && [ "$(dispatches)" = 0 ] && ! grep -q '^gum' "$calls" && [[ "$out" == *"just promote X.Y.Z"* ]] || fail "picker: a failed read: exit $status $out $(cat "$calls")"
# gum missing on the interactive path: the install hint, nothing promoted; the other paths need no gum
mv "$stubs/gum" "$tmp/gum.off"
nogum="$tmp/nogum-bin"; mkdir -p "$nogum"; ln -sf "$(command -v git)" "$(command -v jq)" "$(command -v sed)" "$(command -v bash)" "$(command -v dirname)" "$(command -v mktemp)" "$(command -v rm)" "$(command -v cat)" "$(command -v tr)" "$nogum/" 2>/dev/null; ln -sf "$stubs/gh" "$nogum/gh"
: > "$calls"; status=0; out="$(PATH="$nogum" PROMOTE_FORCE_TTY=1 PROMOTE_NO_FETCH=1 "$(command -v bash)" "$tmp/repo/scripts/promote-pick.sh" 2>&1 </dev/null)" || status=$?
[ "$status" != 0 ] && [ "$(dispatches)" = 0 ] && [[ "$out" == *"gum is missing"*"Install it"* ]] || fail "picker: gum missing: exit $status $out"
: > "$calls"; status=0; PATH="$nogum" "$(command -v bash)" "$tmp/repo/scripts/promote-pick.sh" 1.9.0 >/dev/null 2>&1 </dev/null || status=$?
[ "$status" = 0 ] && grep -qx 'gh workflow run promote.yml --ref main -f version=1.9.0' "$calls" || fail "picker: a named version needs gum: exit $status $(cat "$calls")"
mv "$tmp/gum.off" "$stubs/gum"
# a named version never prompts or reads the records, even on a terminal; no terminal and no version: today's dispatch
: > "$calls"; status=0; PROMOTE_FORCE_TTY=1 "$tmp/repo/scripts/promote-pick.sh" v1.9.0 >/dev/null 2>&1 </dev/null || status=$?
[ "$status" = 0 ] && [ "$(cat "$calls")" = 'gh workflow run promote.yml --ref main -f version=v1.9.0' ] || fail "picker: a named version prompted or read records: $(cat "$calls")"
pick env PROMOTE_NO_TTY=1
[ "$status" = 0 ] && [ "$(cat "$calls")" = 'gh workflow run promote.yml --ref main' ] || fail "picker: no terminal, no version: not today's dispatch: $(cat "$calls")"

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
reset() {
    echo "$private" > "$tmp/ips"; echo "$machines" > "$tmp/machines"; rm -rf "$tmp/points" "$tmp/console-out"
    rm -f "$tmp/smoke-fails" "$tmp/release-exists" "$tmp/machines-fail" "$tmp/image-missing" "$tmp/copy-fails" "$tmp/copy-missing" \
        "$tmp/console-damage" "$tmp/console-cut" "$tmp/gate-fails"
}
# promote [phase] <tag>: promote.sh with the passphrase and $tmp/points, RESTORE_POINT_KEPT unset by default.
promote() {
    : > "$calls"; : > "$tmp/gh_posts"; status=0
    out="$(FLY="$stubs/fly" FLY_SMOKE="$stubs/smoke" GATE_CHECK="$stubs/gate" RESTORE_POINT_PASSPHRASE="${PASS-$passphrase}" RESTORE_POINT_DIR="$tmp/points" \
        "$tmp/repo/scripts/promote.sh" "$@" 2>&1)" || status=$?
}
b64line="$(base64 "$tmp/point.tar" | sed -n 2p)" # a line of the dump's block: never in the log
line() { grep -n "$1" "$calls" | head -n1 | cut -d: -f1; }
staged="registry.fly.io/critical-hit-staging@$newer"
image="registry.fly.io/critical-hit@$newer"

reset; promote v1.2.3
[ "$status" = 0 ] || fail "promote: exit $status: $out"
# the production record (ADR 0024 step 4): one deployment {commit, digest, version}, then success; after the smoke check, before the release
grep -qx 'gh api -X POST repos/o/r/deployments --input - --jq .id' "$calls" && grep -qx 'gh api -X POST repos/o/r/deployments/77/statuses --input -' "$calls"     || fail "promote: no production deployment record written: $(grep 'gh api' "$calls")"
jq -se --arg sha "$main_sha" --arg d "$newer" 'any(.[]; .task == "deploy-production" and .environment == "production" and .ref == $sha and .payload == {commit: $sha, digest: $d, version: "1.2.3"})' "$tmp/gh_posts" >/dev/null \
    || fail "promote: the production record is wrong: $(cat "$tmp/gh_posts")"
jq -se 'any(.[]; .state == "success" and .environment == "production")' "$tmp/gh_posts" >/dev/null || fail "promote: the production record is not a success"
[ "$(line '^smoke')" -lt "$(line 'gh api -X POST repos/o/r/deployments ')" ] && [ "$(line 'gh api -X POST repos/o/r/deployments ')" -lt "$(line '^gh release create')" ] \
    || fail "promote: the record is not between the smoke check and the release"
point="$(sed -n 's/.*restore-point.sh \([a-z0-9_]*\).*/\1/p' "$calls" | head -n1)"
[[ "$point" =~ ^critical_prod_pre_v1_2_3_[0-9]{14}$ ]] || fail "promote: restore point name '$point'"
grep -qx "docker buildx imagetools create --prefer-index=false --progress none -t registry.fly.io/critical-hit:$main_sha $staged" "$calls" \
    || fail "promote: wrong copy call: $(grep buildx "$calls")"
grep -qx "docker manifest inspect $image" "$calls" || fail "promote: the copy's digest is not checked"
grep -qx "fly console -a critical-hit -c $tmp/repo/deploy/fly.prod.toml --image $image --region yyz --vm-size shared-cpu-1x --vm-memory 512 --file-local /tmp/restore-point.sh=$tmp/repo/scripts/restore-point.sh -C sh /tmp/restore-point.sh $point 001_baseline.sql 002_more.sql" "$calls" \
    || fail "promote: wrong console call: $(grep console "$calls")"
grep -qx "fly deploy -c $tmp/repo/deploy/fly.prod.toml -a critical-hit --image $image --no-public-ips --yes" "$calls" \
    || fail "promote: wrong deploy call: $(grep deploy "$calls")"
grep -qx "smoke critical-hit $main_sha" "$calls" || fail "promote: smoke not called with the app and the tag's commit"
grep -q "^gh release create v1.2.3 --verify-tag --title v1.2.3 --generate-notes --notes .*$newer.*$point\`, kept on the machine that ran the promotion, $tmp/points/$point.tar.gpg" "$calls" \
    || fail "promote: no GitHub Release with the digest, the restore point and where it is kept: $(grep release "$calls")"
[ -s "$tmp/points/$point.tar.gpg" ] || fail "promote: the restore point is not kept in RESTORE_POINT_DIR: $(ls "$tmp/points" 2>&1)"
decrypt "$tmp/points/$point.tar.gpg" | cmp -s - "$tmp/point.tar" || fail "promote: the kept restore point does not decrypt to the dump sent"
decrypt "$tmp/points/$point.tar.gpg" wrong-passphrase >/dev/null && fail "promote: the restore point decrypts with another passphrase"
cmp -s "$tmp/points/$point.tar.gpg" "$tmp/point.tar" && fail "promote: the restore point is kept in clear"
[ "$(find "$tmp/points" -type f | sed "s|$tmp/points/||" | sort | tr '\n' ' ')" = "$point.tar.gpg promote.env " ] \
    || fail "promote: RESTORE_POINT_DIR holds more than the encrypted restore point and its record (a clear copy?): $(ls -a "$tmp/points")"
[[ "$out" != *"$b64line"* ]] || fail "promote: the dump's block reached the log"
[[ "$out" == *"restore-point: dumping critical_prod"* && "$out" == *"Connecting to fdaa::2"* && "$out" == *"restore-point: OK $point"* ]] \
    || fail "promote: the console machine's own lines are not printed: $out"
[[ "$out" == *"promote: production (critical-hit) runs 1.2.3 (commit $main_sha, $newer); promoting v1.2.3"* ]] \
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

reset; echo "$public" > "$tmp/ips"; touch "$tmp/gate-fails"; promote v1.2.3
[ "$status" != 0 ] || fail "promote: a public IP without the login gate is promoted"
grep -qE '^(fly (console|deploy)|docker buildx)' "$calls" && fail "promote: a public IP without the gate still copied, took a restore point or deployed"
grep -qx "gate https://critical-hit.fly.dev" "$calls" || fail "promote: the gate check was not run against the public URL: $(cat "$calls")"
PUBLIC_ACCESS_OPTION=basic-auth promote v1.2.3
[ "$status" != 0 ] || fail "promote: PUBLIC_ACCESS_OPTION bypassed the mandatory access check"
# With the gate live a public app is promoted, and the gate is checked again once the release runs.
rm -f "$tmp/gate-fails"; promote v1.2.3
[ "$status" = 0 ] || fail "promote: a public app with the gate live is refused: $out"
[ "$(grep -c '^gate ' "$calls")" = 2 ] || fail "promote: the gate check did not run before and after the deploy: $(cat "$calls")"

reset; printf 'restore-point: FAIL: pg_dump of critical_prod failed\n' > "$tmp/console-out"; promote v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"no restore point"* && "$out" == *"pg_dump of critical_prod failed"* ]] \
    || fail "promote: a failed restore point is not refused, or its reason not printed: $out"
grep -q '^fly deploy' "$calls" && fail "promote: deployed without a restore point"
printf 'restore-point: OK critical_prod_pre_v1_2_3_20000101000000 %s 10\n' "$(printf 'a%.0s' {1..64})" > "$tmp/console-out"; promote v1.2.3
grep -q '^fly deploy' "$calls" && fail "promote: deployed on the OK line of another restore point"
reset; touch "$tmp/console-cut"; promote v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"no restore point"* ]] && [[ "$out" != *"$b64line"* ]] && ! grep -q '^fly deploy' "$calls" \
    || fail "promote: a cut restore point is not refused, or its block reached the log: $out"
reset; touch "$tmp/console-damage"; promote v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"came back damaged"* ]] && ! grep -q '^fly deploy' "$calls" && [ ! -e "$tmp/points/promote.env" ] \
    || fail "promote: a damaged restore point is not refused: $out"
for pass in "" unset; do
    reset
    if [ "$pass" = unset ]; then out="$(unset RESTORE_POINT_PASSPHRASE; FLY="$stubs/fly" RESTORE_POINT_DIR="$tmp/points" "$tmp/repo/scripts/promote.sh" v1.2.3 2>&1)" && status=0 || status=$?
    else : > "$calls"; PASS="" promote v1.2.3; fi
    [ "$status" != 0 ] && [[ "$out" == *"RESTORE_POINT_PASSPHRASE is not set"* ]] && ! grep -qE '^(fly|docker)' "$calls" \
        || fail "promote: no passphrase ($pass) is not refused before anything: $out"
done

# The workflow's two steps around the upload: prepare deploys nothing; deploy needs prepare's record for
# the same tag and RESTORE_POINT_KEPT.
reset; out_file="$tmp/github-output"; : > "$out_file"
GITHUB_OUTPUT="$out_file" promote prepare v1.2.3
[ "$status" = 0 ] || fail "promote prepare: exit $status: $out"
grep -qE '^(fly deploy|smoke|gh release)' "$calls" && fail "promote prepare deployed or released: $(cat "$calls")"
point="$(sed -n 's/^point=//p' "$out_file")"
[[ "$point" =~ ^critical_prod_pre_v1_2_3_[0-9]{14}$ ]] && [ "$(sed -n 's/^file=//p' "$out_file")" = "$tmp/points/$point.tar.gpg" ] \
    || fail "promote prepare: GITHUB_OUTPUT has no point= and file=: $(cat "$out_file")"
promote deploy v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"RESTORE_POINT_KEPT does not say"* ]] && ! grep -q '^fly deploy' "$calls" \
    || fail "promote deploy without RESTORE_POINT_KEPT: $out"
RESTORE_POINT_KEPT="artifact 7" promote deploy v1.2.4
[ "$status" != 0 ] && [[ "$out" == *"taken for another tag"* ]] && ! grep -q '^fly deploy' "$calls" \
    || fail "promote deploy of another tag than prepare's: $out"
RESTORE_POINT_KEPT="encrypted, artifact 7 of https://run" promote deploy v1.2.3
[ "$status" = 0 ] && grep -qx "fly deploy -c $tmp/repo/deploy/fly.prod.toml -a critical-hit --image $image --no-public-ips --yes" "$calls" \
    && grep -q "^gh release create v1.2.3 .*$point\`, encrypted, artifact 7 of https://run" "$calls" \
    && ! grep -qE '^(fly console|docker)' "$calls" && [ "$(line '^fly deploy')" -lt "$(line '^fly ips')" ] \
    || fail "promote deploy after prepare: exit $status, $out; $(cat "$calls")"
rm -rf "$tmp/points"; RESTORE_POINT_KEPT="artifact 7" promote deploy v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"no restore point recorded"* ]] && ! grep -q '^fly deploy' "$calls" || fail "promote deploy without prepare: $out"
reset; status=0; out="$(FLY="$stubs/fly" RESTORE_POINT_PASSPHRASE=x "$tmp/repo/scripts/promote.sh" prepare v1.2.3 2>&1)" || status=$?
[ "$status" != 0 ] && [[ "$out" == *"RESTORE_POINT_DIR is not set"* ]] || fail "promote prepare without RESTORE_POINT_DIR: $out"

reset; touch "$tmp/smoke-fails"; : > "$tmp/gh_posts"; promote v1.2.3
grep -q 'gh api -X POST' "$calls" && fail "promote: a production record after a failed smoke check"
[ "$status" != 0 ] && [[ "$out" == *"registry.fly.io/critical-hit-staging@previous"* ]] || fail "promote: a failed smoke check: exit $status, no releases listed: $out"
grep -q '^gh release create' "$calls" && fail "promote: released after a failed smoke check"

reset; echo "[{\"id\":\"m1\",\"image_ref\":{\"digest\":\"$newer\"}},{\"id\":\"m2\",\"image_ref\":{\"digest\":\"$digest\"}}]" > "$tmp/machines"
promote v1.2.3
[ "$status" != 0 ] && ! grep -q '^gh release create' "$calls" || fail "promote: machines on another digest still released"

reset; touch "$tmp/record-fails"; promote v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"production deployment record could not be written"* ]] || fail "promote: a failed record write did not fail the run: exit $status $out"
rm -f "$tmp/record-fails"
reset; : > "$tmp/gh_posts"; promote v9.9.9
[ "$status" != 0 ] && [ ! -s "$tmp/gh_posts" ] || fail "promote: a refused guard wrote a record"
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
rp CRITICAL_DSN=postgres://u:p@h:5432/critical_prod RESTORE_POINT_CONNECT_TRIES=0; [ "$status" != 0 ] || fail "restore-point accepted 0 connection tries"
[[ "$out" != *u:p@* ]] || fail "restore-point printed the DSN"
rp CRITICAL_DSN=postgres://u:p@h:5432/critical_staging; [ "$status" != 0 ] && [[ "$out" == *"is not a restore point of the database 'critical_staging'"* ]] \
    || fail "restore-point accepted a name of another database: $out"
for m in "001_x.sql;drop" "001_x" "'001_x.sql'" "../001.sql"; do
    point_arg="critical_prod_pre_v1_2_3_20261017000000 $m"
    rp CRITICAL_DSN=postgres://u:p@h:5432/critical_prod; [ "$status" != 0 ] && [[ "$out" == *"malformed migration"* ]] || fail "restore-point accepted the migration '$m': $out"
done

# --- restore-point-fetch.sh: a restore point back from its artifact ---
fstubs="$tmp/fetch-bin"
mkdir -p "$fstubs" "$tmp/artifacts"
cat > "$fstubs/gh" <<STUB
#!/usr/bin/env bash
case "\$1" in
    api) shift; expr=""; url=""
        while [ "\$#" -gt 0 ]; do case "\$1" in --jq) expr="\$2"; shift 2 ;; --paginate) shift ;; *) url="\$1"; shift ;; esac; done
        echo "gh api \$url" >> "$calls"; jq -r "\$expr" "$tmp/artifacts.json" ;;
    run) echo "gh \$*" >> "$calls"; shift 2; run="\$1"; shift
        while [ "\$#" -gt 0 ]; do case "\$1" in -n) name="\$2"; shift 2 ;; -D) dir="\$2"; shift 2 ;; *) shift ;; esac; done
        [ -d "$tmp/artifacts/\$run-\$name" ] || exit 1
        mkdir -p "\$dir" && cp "$tmp/artifacts/\$run-\$name"/* "\$dir/" ;;
    *) exit 1 ;;
esac
STUB
printf '#!/bin/sh\necho "pg_restore $*" >> "%s"\n' "$calls" > "$fstubs/pg_restore"
chmod +x "$fstubs"/*
# artifact <run> <name> <tar>: the encrypted tar as the run's artifact <name>.
artifact() {
    local home; home="$(mktemp -d)"; mkdir -p "$tmp/artifacts/$1-$2"
    GNUPGHOME="$home" gpg --batch --quiet --no-tty --pinentry-mode loopback --no-symkey-cache --passphrase-fd 3 \
        --symmetric --output "$tmp/artifacts/$1-$2/$2.tar.gpg" "$3" 3<<<"$passphrase"
    rm -rf "$home"
}
p1=critical_prod_pre_v1_2_3_20261001000000; p2=critical_prod_pre_v1_2_3_20261002000000
p3=critical_prod_pre_v1_2_3_20261003000000; p4=critical_prod_pre_v1_2_30_20261004000000; p5=critical_prod_pre_v2_0_0_20261005000000
artifact 11 "$p1" "$tmp/point.tar"; artifact 12 "$p2" "$tmp/point.tar"; artifact 14 "$p4" "$tmp/point.tar"
cp "$tmp/point.tar" "$tmp/foreign.tar"; echo x > "$tmp/pointsrc/extra.sh"; tar -C "$tmp/pointsrc" -rf "$tmp/foreign.tar" extra.sh
artifact 15 "$p5" "$tmp/foreign.tar"
cat > "$tmp/artifacts.json" <<JSON
{"artifacts":[
 {"name":"$p1","id":1,"expired":false,"workflow_run":{"id":11}},
 {"name":"$p2","id":2,"expired":false,"workflow_run":{"id":12}},
 {"name":"$p3","id":3,"expired":true,"workflow_run":{"id":13}},
 {"name":"$p4","id":4,"expired":false,"workflow_run":{"id":14}},
 {"name":"$p5","id":5,"expired":false,"workflow_run":{"id":15}},
 {"name":"e2e-results","id":6,"expired":false,"workflow_run":{"id":16}}]}
JSON
fetch() {
    : > "$calls"; status=0
    out="$(env PATH="$fstubs:$PATH" GH_REPO=o/r BACKUP_DIR="$tmp/bk" CRITICAL_PASSWORD_DIR="$tmp/nokeys" RESTORE_POINT_PASSPHRASE="${FPASS-$passphrase}" \
        "$root/scripts/restore-point-fetch.sh" "$@" 2>&1)" || status=$?
}
fetch 1.2.3
[ "$status" = 0 ] && grep -qx "gh run download 12 -R o/r -n $p2 -D .*" "$calls" \
    && cmp -s "$tmp/bk/$p2/critical_prod.dump" "$tmp/pointsrc/critical_prod.dump" && cmp -s "$tmp/bk/$p2/counts.txt" "$tmp/pointsrc/counts.txt" \
    && grep -q "^pg_restore --list .*/critical_prod.dump$" "$calls" \
    || fail "restore-point-fetch 1.2.3: not the newest unexpired restore point of v1.2.3, unpacked and read back: exit $status, $out; $(cat "$calls")"
[ "$(stat -c %a "$tmp/bk/$p2/critical_prod.dump")" = 600 ] && [ "$(stat -c %a "$tmp/bk")" = 700 ] || fail "restore-point-fetch: the dump is readable by others"
[[ "$out" == *"2 tables, 12 rows"* && "$out" != *"$passphrase"* ]] || fail "restore-point-fetch: summary, or the passphrase printed: $out"
fetch v1.2.3
[ "$status" != 0 ] && [[ "$out" == *"exists already"* ]] || fail "restore-point-fetch: an existing folder is overwritten: $out"
fetch "$p1"
[ "$status" = 0 ] && grep -q "^gh run download 11 " "$calls" || fail "restore-point-fetch by name: $out"
fetch 1.2.30
[ "$status" = 0 ] && grep -q "^gh run download 14 " "$calls" || fail "restore-point-fetch 1.2.30 (not 1.2.3's): $out"
for v in 9.9.9 "$p3"; do
    fetch "$v"
    [ "$status" != 0 ] && [[ "$out" == *"among the unexpired artifacts"* ]] || fail "restore-point-fetch $v (none, or expired): $out"
done
for v in 1.2 v01.2.3 "critical_staging_pre_v1_2_3_20261001000000" "$p1;x" "../x"; do
    fetch "$v"
    [ "$status" != 0 ] && [[ "$out" == *"is not X.Y.Z, vX.Y.Z or a restore point name"* ]] && [ ! -s "$calls" ] || fail "restore-point-fetch accepted '$v': $out"
done
rm -rf "$tmp/bk"; FPASS=wrong fetch 1.2.3
[ "$status" != 0 ] && [[ "$out" == *"could not decrypt"* ]] && [ ! -e "$tmp/bk/$p2" ] || fail "restore-point-fetch: a wrong passphrase: $out"
FPASS="" fetch 1.2.3
[ "$status" != 0 ] && [[ "$out" == *"no passphrase"* ]] && [ ! -s "$calls" ] || fail "restore-point-fetch: no passphrase: $out"
mkdir -p "$tmp/keys"; printf '%s\n' "$passphrase" > "$tmp/keys/restore-point.passphrase"
: > "$calls"; status=0
out="$(env PATH="$fstubs:$PATH" GH_REPO=o/r BACKUP_DIR="$tmp/bk" CRITICAL_PASSWORD_DIR="$tmp/keys" RESTORE_POINT_PASSPHRASE= \
    "$root/scripts/restore-point-fetch.sh" 1.2.3 2>&1)" || status=$?
[ "$status" = 0 ] && [ -s "$tmp/bk/$p2/critical_prod.dump" ] || fail "restore-point-fetch: the passphrase file is not read: $out"
fetch 2.0.0
[ "$status" != 0 ] && [[ "$out" == *"not critical_prod.dump and counts.txt"* ]] && [ ! -e "$tmp/bk/$p5" ] || fail "restore-point-fetch: a foreign tar member: $out"

# --- restore-point-key.sh: the passphrase, made once, set as the production secret ---
kstubs="$tmp/key-bin"
mkdir -p "$kstubs"
printf '#!/bin/sh\necho "gh $*" >> "%s"\ncat > "%s"\n' "$calls" "$tmp/secret" > "$kstubs/gh"
chmod +x "$kstubs/gh"
key() { : > "$calls"; status=0; out="$(env PATH="$kstubs:$PATH" CRITICAL_PASSWORD_DIR="$tmp/newkeys" "$root/scripts/restore-point-key.sh" 2>&1)" || status=$?; }
key
made="$(cat "$tmp/newkeys/restore-point.passphrase" 2>/dev/null)"
[ "$status" = 0 ] && [ "${#made}" -ge 40 ] && [ "$(stat -c %a "$tmp/newkeys/restore-point.passphrase")" = 600 ] \
    && [ "$(cat "$calls")" = "gh secret set RESTORE_POINT_PASSPHRASE --env production" ] && [ "$(cat "$tmp/secret")" = "$made" ] \
    && [[ "$out" != *"$made"* ]] || fail "restore-point-key: exit $status, $out; $(cat "$calls")"
key
[ "$status" = 0 ] && [ "$(cat "$tmp/newkeys/restore-point.passphrase")" = "$made" ] && [ "$(cat "$tmp/secret")" = "$made" ] \
    || fail "restore-point-key: a rerun changed the passphrase or did not set the secret again: $out"

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
PROMOTE_NO_TTY=1 just -f "$root/justfile" -d "$root" promote >/dev/null 2>&1 </dev/null || fail "just promote failed"
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
check(job["permissions"] == {"contents": "write", "deployments": "write"}, "promote: permissions are not minimal")
steps = job["steps"]
a = admin_first("promote", steps)
readers = [i for i, st in enumerate(steps) if "secrets." in str(st)]
check([steps[i].get("name") for i in readers] == ["Check the deploy secrets", "Restore point", "Promote"], f"promote: the secrets are read by {readers}")
check(all(i > a for i in readers), "promote: a secret is read before the admin check")
check(all("if" not in st for st in steps), "promote: a conditional step could skip the promotion")
for name, j in (("resolve", res), ("promote", job)):
    co = [st for st in j["steps"] if st.get("uses", "").startswith("actions/checkout@")]
    check(len(co) == 1 and co[0]["with"].get("fetch-depth") == 0 and co[0]["with"].get("persist-credentials") is False,
          f"{name}: checkout must fetch the whole history and the tags, without persisted credentials")
    check(bool(co) and "ref" not in co[0]["with"], f"{name}: the checkout must be main's (its scripts, its fly config)")
byname = {st.get("name"): i for i, st in enumerate(steps)}
point, upload, deploy = (steps[byname.get(n, 0)] for n in ("Restore point", "Keep the restore point", "Promote"))
check(byname.get("Restore point", 0) < byname.get("Keep the restore point", 0) < byname.get("Promote", 0) == len(steps) - 1,
      "promote: not restore point, then its upload, then the deploy (last)")
check(point.get("id") == "point" and 'scripts/promote.sh prepare "$TAG"' in point.get("run", ""),
      "promote: the restore point step does not run promote.sh prepare")
check(point.get("env", {}).get("RESTORE_POINT_PASSPHRASE") == "${{ secrets.RESTORE_POINT_PASSPHRASE }}"
      and point["env"].get("FLY_API_TOKEN") == "${{ secrets.FLY_API_TOKEN }}", "promote: prepare lacks the token or the passphrase")
check("RESTORE_POINT_PASSPHRASE" not in str(deploy), "promote: the deploy step reads the passphrase")
check(point.get("env", {}).get("PROMOTE_NEWEST") == "${{ inputs.version == '' }}",
      "promote: promote.sh prepare is not told when no version was asked (the newest never goes below production)")
for st in (point, deploy):
    check(st.get("env", {}).get("TAG") == "${{ needs.resolve.outputs.tag }}", "promote: a promote.sh step does not take the resolved tag")
    check(st.get("env", {}).get("RESTORE_POINT_DIR") == "${{ runner.temp }}/restore-point", "promote: the two steps do not share RESTORE_POINT_DIR")
check(upload.get("id") == "upload" and upload.get("uses", "").startswith("actions/upload-artifact@")
      and len(upload["uses"].split("@")[1]) == 40, "promote: the restore point is not uploaded by a pinned actions/upload-artifact")
w = upload.get("with", {})
check(w.get("name") == "${{ steps.point.outputs.point }}" and w.get("path") == "${{ steps.point.outputs.file }}"
      and w.get("retention-days") == 90 and w.get("if-no-files-found") == "error",
      "promote: the upload is not the encrypted file, named after the restore point, kept 90 days, an error when missing")
check('scripts/promote.sh deploy "$TAG"' in deploy.get("run", "") and deploy.get("env", {}).get("ARTIFACT_ID") == "${{ steps.upload.outputs.artifact-id }}",
      "promote: the last step does not deploy the resolved tag after the upload")
r = subprocess.run(["bash", "-e", "-c", deploy.get("run", "true")], env={"ARTIFACT_ID": "", "TAG": "v1.2.3", "PATH": "/usr/bin:/bin"},
                   capture_output=True, text=True, cwd="/")
check(r.returncode != 0 and "not kept as an artifact" in r.stdout, "promote: the deploy step runs without an artifact id")
secret = [st for st in steps if st.get("name") == "Check the deploy secrets"]
run = secret[0]["run"] if secret else "exit 1"
for token, phrase, want_ok in (("", "p4ss", False), ("s3cret-value", "", False), ("s3cret-value", "p4ss-phrase", True)):
    r = subprocess.run(["bash", "-e", "-c", run], env={"FLY_API_TOKEN": token, "RESTORE_POINT_PASSPHRASE": phrase, "PATH": "/usr/bin:/bin"},
                       capture_output=True, text=True)
    check((r.returncode == 0) == want_ok, f"promote: secret check with token {token!r}, passphrase {phrase!r} exited {r.returncode}")
    check(all(v == "" or v not in r.stdout + r.stderr for v in (token, phrase)), "promote: secret check printed a secret")
if errs:
    print("\n".join("test-promotion: FAIL: " + e for e in errs), file=sys.stderr)
    sys.exit(1)
PY

[ "$fails" = 0 ] || exit 1
echo "--- Promotion scripts ---"
