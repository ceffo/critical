#!/usr/bin/env bash
# Tests the staging deploy's scripts without the network, fly or GitHub (part of `just validate`, ADR 0024
# steps 3.2, 3.3, 8.3 and 10.4):
#   - staging-deploy.sh, with fly and the smoke check stubbed: a public IP fails before any deploy; a
#     private app is deployed from registry.fly.io/<app>:<sha> and the smoke check gets app and sha; a
#     failed smoke check fails the run and prints the releases; a configured public-access option skips
#     the IP check only; machines on different digests fail; the digest is written to $GITHUB_OUTPUT.
#   - record-deployment.sh, with gh stubbed: one deployment (task deploy-staging, ref the sha, digest in
#     the payload) and one status; a success without a digest, a short sha and an unknown state call nothing.
#   - the workflows, parsed: deploy-staging.yml only on main, its token only in the secret check and the
#     deploy step, every later step gated; release.yml calls it only when the image was pushed.
#   - STAGING_CONFIG: the deploy uses the config it names (the image's own commit).
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-staging-deploy: FAIL: $*" >&2; fails=$((fails + 1)); }

sha="$(printf 'a%.0s' {1..40})"
digest="sha256:$(printf 'b%.0s' {1..64})"
other="sha256:$(printf 'c%.0s' {1..64})"
calls="$tmp/calls"

# fly stub: logs every call; answers from files the test sets (ips, machines).
stubs="$tmp/bin"
mkdir -p "$stubs"
cat > "$stubs/fly" <<STUB
#!/bin/sh
echo "fly \$*" >> "$calls"
case "\$1 \$2" in
    "ips list") cat "$tmp/ips" ;;
    "machine list") cat "$tmp/machines" ;;
    "releases --image") echo "v7 complete registry.fly.io/critical-staging:previous" ;;
esac
STUB
cat > "$stubs/smoke" <<STUB
#!/bin/sh
echo "smoke \$*" >> "$calls"
[ ! -f "$tmp/smoke-fails" ]
STUB
chmod +x "$stubs/fly" "$stubs/smoke"

private='[{"Type":"private_v6","Address":"fdaa::1","Region":"global"}]'
public='[{"Type":"v4","Address":"1.2.3.4","Region":"global"},{"Type":"private_v6","Address":"fdaa::1","Region":"global"}]'
one="[{\"id\":\"m1\",\"image_ref\":{\"digest\":\"$digest\"}}]"
split="[{\"id\":\"m1\",\"image_ref\":{\"digest\":\"$digest\"}},{\"id\":\"m2\",\"image_ref\":{\"digest\":\"$other\"}}]"

# run_deploy [env assignment...]: runs the script against the stubs; sets $status, $out (stdout+stderr), $gh_out.
run_deploy() {
    : > "$calls"; : > "$tmp/gh_out"
    status=0
    out="$(env "$@" FLY="$stubs/fly" FLY_SMOKE="$stubs/smoke" GITHUB_OUTPUT="$tmp/gh_out" \
        "$root/scripts/staging-deploy.sh" "$sha" 2>&1)" || status=$?
    gh_out="$(cat "$tmp/gh_out")"
}

# --- a public IP fails before anything is deployed ---
echo "$public" > "$tmp/ips"; echo "$one" > "$tmp/machines"; rm -f "$tmp/smoke-fails"
run_deploy A=1
[ "$status" != 0 ] || fail "a public IP did not fail the run"
grep -q '^fly deploy' "$calls" && fail "deployed an app that has a public IP"
grep -q '^smoke' "$calls" && fail "smoke-checked an app that has a public IP"
[[ "$out" != *"FAILED. Releases"* ]] || fail "a refused deploy printed a rollback hint"

# --- a private app: deployed from the sha's image, smoke-checked with the sha, digest recorded ---
echo "$private" > "$tmp/ips"
run_deploy A=1
[ "$status" = 0 ] || fail "private app: exit $status: $out"
grep -qx "fly deploy -c $root/deploy/fly.staging.toml -a critical-staging --image registry.fly.io/critical-staging:$sha --yes" "$calls" \
    || fail "private app: wrong deploy call: $(cat "$calls")"
grep -qx "smoke critical-staging $sha" "$calls" || fail "private app: smoke check not called with app and sha"
[ "$gh_out" = "digest=$digest" ] || fail "private app: output is '$gh_out'"
ips_line="$(grep -n '^fly ips' "$calls" | head -n1 | cut -d: -f1)"
deploy_line="$(grep -n '^fly deploy' "$calls" | head -n1 | cut -d: -f1)"
[ "${ips_line:-99}" -lt "${deploy_line:-0}" ] || fail "private app: the IP check did not come before the deploy"

# --- a failed smoke check fails the run and prints the releases to roll back to ---
touch "$tmp/smoke-fails"
run_deploy A=1
[ "$status" != 0 ] || fail "failed smoke check: exit 0"
[[ "$out" == *"v7 complete registry.fly.io/critical-staging:previous"* ]] || fail "failed smoke check: no previous release printed: $out"
[ -z "$gh_out" ] || fail "failed smoke check: wrote a digest: '$gh_out'"
rm -f "$tmp/smoke-fails"

# --- a configured public-access option skips the IP check, not the deploy and the smoke check ---
echo "$public" > "$tmp/ips"
run_deploy PUBLIC_ACCESS_OPTION=basic-auth
[ "$status" = 0 ] || fail "access option: exit $status: $out"
grep -q '^fly ips' "$calls" && fail "access option: still listed the IPs"
grep -q '^fly deploy' "$calls" || fail "access option: no deploy"
grep -q '^smoke' "$calls" || fail "access option: no smoke check"
echo "$private" > "$tmp/ips"

# --- machines on two digests are not one release ---
echo "$split" > "$tmp/machines"
run_deploy A=1
[ "$status" != 0 ] || fail "two digests: exit 0"
echo "$one" > "$tmp/machines"

# --- a short sha deploys nothing ---
: > "$calls"
FLY="$stubs/fly" FLY_SMOKE="$stubs/smoke" "$root/scripts/staging-deploy.sh" "${sha:0:7}" >/dev/null 2>&1 && fail "accepted a short sha"
[ ! -s "$calls" ] || fail "short sha: called fly: $(cat "$calls")"

# --- record-deployment.sh with gh stubbed ---
cat > "$stubs/gh" <<STUB
#!/bin/sh
echo "gh \$*" >> "$calls"
cat >> "$tmp/gh_stdin"
echo 4242
STUB
chmod +x "$stubs/gh"
run_record() { : > "$calls"; : > "$tmp/gh_stdin"; status=0; PATH="$stubs:$PATH" GITHUB_REPOSITORY=o/r GITHUB_RUN_ID=9 "$root/scripts/record-deployment.sh" "$@" >/dev/null 2>&1 || status=$?; }

run_record "$sha" success "$digest"
[ "$status" = 0 ] || fail "record success: exit $status"
grep -qx 'gh api -X POST repos/o/r/deployments --input - --jq .id' "$calls" || fail "record: no deployment created: $(cat "$calls")"
grep -qx 'gh api -X POST repos/o/r/deployments/4242/statuses --input -' "$calls" || fail "record: no status posted on the deployment id"
body="$(cat "$tmp/gh_stdin")"
jq -e --arg sha "$sha" --arg d "$digest" 'select(.ref == $sha and .environment == "staging" and .task == "deploy-staging" and .payload.digest == $d)' <<<"$body" >/dev/null \
    || fail "record: deployment body wrong: $body"
jq -e --arg d "$digest" 'select(.state == "success" and .description == $d)' <<<"$body" >/dev/null || fail "record: status body wrong: $body"

run_record "$sha" failure
[ "$status" = 0 ] || fail "record failure: exit $status"
jq -e 'select(.state == "failure")' <<<"$(cat "$tmp/gh_stdin")" >/dev/null || fail "record: no failure status"

for args in "$sha success" "${sha:0:7} success $digest" "$sha pending $digest"; do
    # shellcheck disable=SC2086
    run_record $args
    [ "$status" != 0 ] || fail "record accepted: $args"
    [ ! -s "$calls" ] || fail "record called gh on a refused input: $args"
done

# --- the config of the image's commit is the one deployed ---
printf 'app = "other-app"\n' > "$tmp/image.toml"
run_deploy STAGING_CONFIG="$tmp/image.toml"
grep -qx "fly deploy -c $tmp/image.toml -a other-app --image registry.fly.io/other-app:$sha --yes" "$calls" \
    || fail "STAGING_CONFIG: not deployed with that config: $(cat "$calls")"

# --- the workflows, parsed (not grepped) ---
python3 -I - "$root/.github/workflows" <<'PY' || fails=$((fails + 1))
import sys, yaml

d = sys.argv[1]
load = lambda n: yaml.safe_load(open(f"{d}/{n}"))
errs = []
def check(ok, msg):
    if not ok:
        errs.append(msg)

dep = load("deploy-staging.yml")
triggers = dep.get(True) or dep.get("on")  # PyYAML reads the key `on` as True
check({"workflow_call", "workflow_dispatch"} <= set(triggers), "deploy-staging: needs workflow_call and workflow_dispatch")
job = dep["jobs"]["deploy"]
check("github.ref == 'refs/heads/main'" in str(job.get("if", "")), "deploy-staging: job not limited to main")
check(job.get("environment") == "staging", "deploy-staging: job not in the staging environment")
check(job["concurrency"]["group"] == "deploy-staging" and job["concurrency"]["cancel-in-progress"] is False,
      "deploy-staging: not serialized without cancelling")
check(job["permissions"] == {"contents": "read", "deployments": "write"}, "deploy-staging: permissions are not minimal")
check("FLY_API_TOKEN" not in str(dep.get("env", "")) + str(job.get("env", "")), "deploy-staging: token in a workflow or job env")
token_steps = [st.get("id") for st in job["steps"] if "secrets.FLY_API_TOKEN" in str(st)]
check(token_steps == ["secrets", "deploy"], f"deploy-staging: the token is read by {token_steps}, not only the check and the deploy")
ids = [st.get("id") for st in job["steps"]]
after = job["steps"][ids.index("secrets") + 1:]
for st in after:
    cond = st.get("if", "")
    check("steps.secrets.outputs.ready == 'true'" in cond or "steps.deploy.outcome" in cond,
          f"deploy-staging: step '{st.get('name')}' runs without the token")
rec = next(st for st in job["steps"] if "record-deployment.sh" in st.get("run", ""))
check(rec["if"].startswith("always()"), "deploy-staging: the record is skipped when the deploy fails")

rel = load("release.yml")
jobs = rel["jobs"]
check(jobs["deploy"]["needs"] == "image", "release: deploy does not need the image job")
check(jobs["deploy"]["if"] == "needs.image.outputs.pushed == 'true'", "release: deploy not gated on a pushed image")
check(jobs["deploy"]["uses"] == "./.github/workflows/deploy-staging.yml", "release: does not call the staging deploy")
check("steps.build.outputs.pushed" in jobs["image"]["outputs"]["pushed"], "release: image job has no pushed output")

if errs:
    print("\n".join("test-staging-deploy: FAIL: " + e for e in errs), file=sys.stderr)
    sys.exit(1)
PY

[ "$fails" = 0 ] || exit 1
echo "--- Staging deploy scripts ---"
