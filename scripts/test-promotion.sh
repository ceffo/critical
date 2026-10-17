#!/usr/bin/env bash
# Tests the promotion's scripts without the network, fly or GitHub (part of `just validate`, ADR 0024
# steps 4, 8.3 and 10.4), on fixture git repositories with fly, gh, docker and the smoke check stubbed:
#   - release-guards.sh: each guard (tag format, on main, the staging record read as docs/deploy.md says,
#     the image in the registry) and `all`, which prints the tag's commit and the digest staging ran;
#   - promote.sh: guards, then the no-public-IP check, then the restore point in a console machine of the
#     staged image, then the deploy by digest, the smoke check, the digest check and the GitHub Release;
#     each failure stops everything after it (a refused guard calls fly not at all; no restore point, no
#     deploy; a failed smoke or a digest mismatch, no release);
#   - release-tag.sh (`just release`): every refusal (version, branch, dirty tree, behind origin, staging
#     not passed) pushes nothing; the happy path pushes one annotated tag;
#   - restore-point.sh: what it refuses without a database (its SQL runs for real in scripts/image-smoke.sh);
#   - promote.yml, parsed: tags only, the production environment, serialized and never cancelled, the
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
[ ! -f "$tmp/image-missing" ]
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
cp "$root"/scripts/{release-guards,release-tag,promote,restore-point,fly-private-check}.sh scripts/
cp "$root"/deploy/fly.{staging,prod}.toml deploy/
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
staging() { status=0; out="$("$guards" staging "$1" 2>&1)" || status=$?; }
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

# --- promote.sh ---
private='[{"Type":"private_v6","Address":"fdaa::1","Region":"global"}]'
public='[{"Type":"v4","Address":"1.2.3.4","Region":"global"},{"Type":"private_v6","Address":"fdaa::1","Region":"global"}]'
machines="[{\"id\":\"m1\",\"image_ref\":{\"digest\":\"$newer\"}},{\"id\":\"m2\",\"image_ref\":{\"digest\":\"$newer\"}}]"
ok_console="$(printf 'Created an ephemeral machine\nrestore-point: dumping critical_prod\nrestore-point: OK @POINT@\r\n')"
reset() {
    echo "$private" > "$tmp/ips"; echo "$machines" > "$tmp/machines"; echo "$ok_console" > "$tmp/console-out"
    rm -f "$tmp/smoke-fails" "$tmp/release-exists" "$tmp/image-missing"
}
promote() { : > "$calls"; status=0; out="$(FLY="$stubs/fly" FLY_SMOKE="$stubs/smoke" "$tmp/repo/scripts/promote.sh" "$1" 2>&1)" || status=$?; }
line() { grep -n "$1" "$calls" | head -n1 | cut -d: -f1; }
image="registry.fly.io/critical-staging@$newer"

reset; promote v1.2.3
[ "$status" = 0 ] || fail "promote: exit $status: $out"
point="$(sed -n 's/.*restore-point.sh \([a-z0-9_]*\).*/\1/p' "$calls" | head -n1)"
[[ "$point" =~ ^critical_prod_pre_v1_2_3_[0-9]{14}$ ]] || fail "promote: restore point name '$point'"
grep -qx "fly console -a critical-prod --image $image --file-local /tmp/restore-point.sh=$tmp/repo/scripts/restore-point.sh -C sh /tmp/restore-point.sh $point" "$calls" \
    || fail "promote: wrong console call: $(grep console "$calls")"
grep -qx "fly deploy -c $tmp/repo/deploy/fly.prod.toml -a critical-prod --image $image --no-public-ips --yes" "$calls" \
    || fail "promote: wrong deploy call: $(grep deploy "$calls")"
grep -qx "smoke critical-prod $main_sha" "$calls" || fail "promote: smoke not called with the app and the tag's commit"
grep -q "^gh release create v1.2.3 --verify-tag --title v1.2.3 --generate-notes --notes .*$newer" "$calls" || fail "promote: no GitHub Release with the digest"
[ "$(line '^docker manifest')" -lt "$(line '^fly ips')" ] && [ "$(line '^fly ips')" -lt "$(line '^fly console')" ] \
    && [ "$(line '^fly console')" -lt "$(line '^fly deploy')" ] && [ "$(line '^fly deploy')" -lt "$(line '^smoke')" ] \
    && [ "$(line '^smoke')" -lt "$(line '^gh release create')" ] || fail "promote: steps out of order: $(cat "$calls")"

reset; promote v9.9.9
[ "$status" != 0 ] || fail "promote: a tag off main is promoted"
grep -q '^fly' "$calls" && fail "promote: a refused guard still called fly: $(cat "$calls")"

reset; promote v1.2
[ "$status" != 0 ] && ! grep -q '^fly' "$calls" || fail "promote: a malformed tag called fly"

reset; touch "$tmp/image-missing"; promote v1.2.3
[ "$status" != 0 ] && ! grep -q '^fly' "$calls" || fail "promote: a missing image called fly"

reset; echo "$public" > "$tmp/ips"; promote v1.2.3
[ "$status" != 0 ] || fail "promote: a public IP is promoted"
grep -qE '^fly (console|deploy)' "$calls" && fail "promote: a public IP still took a restore point or deployed"
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

# --- release-tag.sh (`just release`) ---
put "$(deployments "$main_sha")" "[{\"id\":12,\"created_at\":\"2026-10-17T03:00:00Z\",\"payload\":{\"digest\":\"$newer\"}}]"
release() { status=0; out="$(scripts/release-tag.sh "$1" 2>&1)" || status=$?; }
pushed() { git ls-remote --tags "$tmp/origin.git" | grep -c 'refs/tags/v[0-9.]*$' || true; }
git push -q origin v1.2.3
git tag -d v9.9.9 >/dev/null # a local tag off main, never pushed
before="$(pushed)"
for v in 1.2 v1.2.3.4 1.2.3-rc1 01.2.3 abc; do
    release "$v"; [ "$status" != 0 ] && [[ "$out" == *"not MAJOR.MINOR.PATCH"* ]] || fail "release $v: not refused as non-semver: $out"
done
for v in 1.2.3 1.2.2 0.9.9 v1.2.3; do
    release "$v"; [ "$status" != 0 ] && [[ "$out" == *"not greater than the last release v1.2.3"* ]] || fail "release $v: not refused as non-increasing: $out"
done
git switch -q side; release 1.3.0
[ "$status" != 0 ] && [[ "$out" == *"not main"* ]] || fail "release on a side branch: $out"
git switch -q main
echo dirty > scripts/dirty; release 1.3.0
[ "$status" != 0 ] && [[ "$out" == *"not clean"* ]] || fail "release with a dirty tree: $out"
rm scripts/dirty
git commit -q --allow-empty -m local; release 1.3.0
[ "$status" != 0 ] && [[ "$out" == *"not origin/main's head"* ]] || fail "release ahead of origin: $out"
git reset -q --hard origin/main
git push -q origin "$(git commit-tree -p "$main_sha" -m remote "$(git rev-parse HEAD^{tree})"):refs/heads/main"
release 1.3.0
[ "$status" != 0 ] && [[ "$out" == *"not origin/main's head"* ]] || fail "release behind origin: $out"
git reset -q --hard origin/main
release 1.3.0
[ "$status" != 0 ] && [[ "$out" == *"staging has not passed"* ]] || fail "release of a commit staging did not pass: $out"
[ "$(pushed)" = "$before" ] || fail "a refused release pushed a tag"
put "$(deployments "$(git rev-parse HEAD)")" "[{\"id\":15,\"created_at\":\"2026-10-17T06:00:00Z\",\"payload\":{\"digest\":\"$newer\"}}]"
put "$(statuses 15)" '[{"state":"success"}]'
release 1.3.0
[ "$status" = 0 ] || fail "release 1.3.0: exit $status: $out"
[ "$(git -C "$tmp/origin.git" cat-file -t refs/tags/v1.3.0 2>/dev/null)" = tag ] || fail "release: no annotated tag v1.3.0 on origin"
[ "$(git -C "$tmp/origin.git" rev-parse 'v1.3.0^{commit}')" = "$(git rev-parse HEAD)" ] || fail "release: v1.3.0 is not main's head"
release 1.10.0
[ "$status" = 0 ] && [ "$(git -C "$tmp/origin.git" rev-parse 'v1.10.0^{commit}' 2>/dev/null)" = "$(git rev-parse HEAD)" ] \
    || fail "release 1.10.0 after 1.3.0 (version order, not text order): $out"
release 1.9.0
[ "$status" != 0 ] || fail "release 1.9.0 after 1.10.0 is accepted"
cd "$root"

# --- restore-point.sh: refusals before any database call ---
rp() { status=0; out="$(env PATH="$stubs:/usr/bin:/bin" "$@" sh "$root/scripts/restore-point.sh" "${point_arg:-}" 2>&1)" || status=$?; }
for point_arg in critical_prod_pre_v1_2_3 critical_prod_pre_v1_2_3_2026 'critical_prod_pre_v1_2_3_20261017000000;drop' Critical_pre_v1_2_3_20261017000000; do
    rp CRITICAL_DSN=postgres://u:p@h:5432/critical_prod; [ "$status" != 0 ] && [[ "$out" == *"malformed"* ]] || fail "restore-point accepted '$point_arg': $out"
done
point_arg=critical_prod_pre_v1_2_3_20261017000000
rp A=1; [ "$status" != 0 ] && [[ "$out" == *"CRITICAL_DSN is not set"* ]] || fail "restore-point without a DSN: $out"
rp CRITICAL_DSN=postgres://u:p@h:5432/critical_prod RESTORE_POINT_KEEP=0; [ "$status" != 0 ] || fail "restore-point accepted a retention of 0"
[[ "$out" != *u:p@* ]] || fail "restore-point printed the DSN"

# --- the workflow, parsed ---
python3 -I - "$root/.github/workflows/promote.yml" <<'PY' || fails=$((fails + 1))
import subprocess, sys, yaml

wf = yaml.safe_load(open(sys.argv[1]))
errs = []
def check(ok, msg):
    if not ok:
        errs.append(msg)

on = wf.get(True) or wf.get("on")  # PyYAML reads the key `on` as True
check(on["push"] == {"tags": ["v*"]} and set(on) == {"push", "workflow_dispatch"}, "promote: not tags v* + dispatch")
check(wf["permissions"] == {"contents": "read"}, "promote: workflow permissions are not read-only")
check(list(wf["jobs"]) == ["promote"], "promote: one job expected")
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
