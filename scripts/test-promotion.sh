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
#   - promote.yml, parsed: a run from a branch fails saying "use a tag"; the promotion runs by hand only (no trigger on a tag push), for tags only, the production environment, serialized and never cancelled, the
#     token read only by the secret check (which fails on an empty token) and the promotion step.
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
    api\ *)
        url="\$2"; shift 2; expr=""
        while [ "\$#" -gt 0 ]; do case "\$1" in --jq) expr="\$2"; shift 2 ;; *) shift ;; esac; done
        f="$fx/\$(printf '%s' "\$url" | tr -c 'a-zA-Z0-9' '_')"
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
    "machine list") cat "$tmp/machines" ;;
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

# --- promote.sh ---
private='[{"Type":"private_v6","Address":"fdaa::1","Region":"global"}]'
public='[{"Type":"v4","Address":"1.2.3.4","Region":"global"},{"Type":"private_v6","Address":"fdaa::1","Region":"global"}]'
machines="[{\"id\":\"m1\",\"image_ref\":{\"digest\":\"$newer\"}},{\"id\":\"m2\",\"image_ref\":{\"digest\":\"$newer\"}}]"
ok_console="$(printf 'Created an ephemeral machine\nrestore-point: dumping critical_prod\nrestore-point: OK @POINT@\r\n')"
reset() {
    echo "$private" > "$tmp/ips"; echo "$machines" > "$tmp/machines"; echo "$ok_console" > "$tmp/console-out"
    rm -f "$tmp/smoke-fails" "$tmp/release-exists" "$tmp/image-missing" "$tmp/copy-fails" "$tmp/copy-missing"
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
check(wf["permissions"] == {"contents": "read"}, "promote: workflow permissions are not read-only")
check(sorted(wf["jobs"]) == ["not-a-tag", "promote"], "promote: the jobs are promote and not-a-tag")
refuse = wf["jobs"]["not-a-tag"]
check(refuse.get("if") == "${{ !startsWith(github.ref, 'refs/tags/v') }}", "not-a-tag: not the complement of the promote job's condition")
check("environment" not in refuse and "secrets." not in str(refuse), "not-a-tag: reads an environment or a secret")
r = subprocess.run(["bash", "-e", "-c", refuse["steps"][-1]["run"]], env={"REF": "refs/heads/main", "PATH": "/usr/bin:/bin"}, capture_output=True, text=True)
check(r.returncode != 0 and "not a v* tag" in r.stdout, "not-a-tag: a run from main does not fail saying so")
job = wf["jobs"]["promote"]
check("uses" not in job, "promote: calls a reusable workflow, which cannot read the environment's secrets")
check(job.get("environment") == "production", "promote: not in the production environment")
check(job.get("if") == "startsWith(github.ref, 'refs/tags/v')", "promote: not limited to tags")
check(job["concurrency"] == {"group": "deploy-production", "cancel-in-progress": False}, "promote: not serialized without cancelling")
check(job["permissions"] == {"contents": "write", "deployments": "read"}, "promote: permissions are not minimal")
steps = job["steps"]
readers = [st.get("name", st.get("uses")) for st in steps if "secrets.FLY_API_TOKEN" in str(st)]
check(readers == ["Check the deploy secret", "Promote"], f"promote: the token is read by {readers}")
check(all("if" not in st for st in steps), "promote: a conditional step could skip the promotion")
co = [st for st in steps if st.get("uses", "").startswith("actions/checkout@")]
check(len(co) == 1 and co[0]["with"].get("fetch-depth") == 0 and co[0]["with"].get("persist-credentials") is False,
      "promote: checkout must fetch the whole history (the on-main guard), without persisted credentials")
check("ref" not in co[0]["with"] if co else False, "promote: the checkout must be the tag itself (its scripts, its fly config)")
check("scripts/promote.sh" in steps[-1]["run"], "promote: the last step does not run scripts/promote.sh")
run = steps[0]["run"]
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
