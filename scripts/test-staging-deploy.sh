#!/usr/bin/env bash
# Tests the staging deploy's scripts without the network, fly or GitHub (part of `just validate`, ADR 0024
# steps 3.2, 3.3, 8.3 and 10.4):
#   - staging-deploy.sh, with fly and the smoke check stubbed: a public IP fails before any deploy; a
#     private app is deployed from registry.fly.io/<app>:<sha> and the smoke check gets app and sha; a
#     failed smoke check fails the run and prints the releases; a configured public-access option skips
#     the IP check only; machines on different digests fail; the digest is written to $GITHUB_OUTPUT.
#   - record-deployment.sh, with gh stubbed: one deployment (task deploy-staging, ref the sha, digest in
#     the payload) and one status; a success without a digest, a short sha and an unknown state call nothing.
#   - the workflows, parsed: the deploy jobs (release.yml after a push, deploy-staging.yml by hand) have
#     the staging environment and pass its token to the composite action, never call a reusable workflow
#     (it cannot read the environment's secrets); the action reads the token only in its secret check and
#     deploy step, and its secret check fails (runs, not skips) on an empty token.
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
python3 -I - "$root/.github" <<'PY' || fails=$((fails + 1))
import subprocess, sys, yaml

d = sys.argv[1]
load = lambda n: yaml.safe_load(open(f"{d}/{n}"))
errs = []
def check(ok, msg):
    if not ok:
        errs.append(msg)

act = load("actions/staging-deploy/action.yml")
steps = act["runs"]["steps"]
byid = {st.get("id") or st["name"]: st for st in steps}
# The token reaches the action only as an input, and only the secret check and the deploy step read it.
readers = [st.get("id") or st["name"] for st in steps if "inputs.fly-api-token" in str(st)]
check(readers == ["Check the deploy secret", "deploy"], f"action: the token is read by {readers}, not only the check and the deploy")
check(all("if" not in st for st in steps[:-1]), "action: a step before the record is conditional, so a missing token could skip the deploy")
check(steps[0]["name"] == "Check the deploy secret", "action: the secret check is not the first step")
check(act["runs"]["using"] == "composite", "action: not a composite action")
rec = byid["Record the deployment"]
check(rec["if"].startswith("always()"), "action: the record is skipped when the deploy fails")

# The secret check fails on an empty token (the post-push skip of the first real run) and passes on a set one.
run = steps[0]["run"]
for token, want_ok in (("", False), ("s3cret-value", True)):
    r = subprocess.run(["bash", "-e", "-c", run], env={"FLY_API_TOKEN": token, "PATH": "/usr/bin:/bin"}, capture_output=True, text=True)
    check((r.returncode == 0) == want_ok, f"action: secret check with token {token!r} exited {r.returncode}")
    check(token == "" or token not in r.stdout + r.stderr, "action: secret check printed the token")

def uses_action(job):
    return [st for st in job["steps"] if st.get("uses") == "./.github/actions/staging-deploy"]

dep = load("workflows/deploy-staging.yml")
triggers = dep.get(True) or dep.get("on")  # PyYAML reads the key `on` as True
check(set(triggers) == {"workflow_dispatch"}, "deploy-staging: should be the manual entry only")
rel = load("workflows/release.yml")
jobs = rel["jobs"]

for name, job in (("deploy-staging", dep["jobs"]["deploy"]), ("release", jobs["deploy"])):
    check(job.get("environment") == "staging", f"{name}: deploy job not in the staging environment")
    check("uses" not in job, f"{name}: deploy job calls a reusable workflow, which cannot read the environment's secrets")
    check(job["concurrency"]["group"] == "deploy-staging" and job["concurrency"]["cancel-in-progress"] is False,
          f"{name}: not serialized without cancelling")
    check(job["permissions"] == {"contents": "read", "deployments": "write"}, f"{name}: permissions are not minimal")
    calls = uses_action(job)
    check(len(calls) == 1, f"{name}: does not call the staging deploy action exactly once")
    if calls:
        w = calls[0]["with"]
        check(w.get("fly-api-token") == "${{ secrets.FLY_API_TOKEN }}", f"{name}: the action does not get the environment's token")
        check("if" not in calls[0], f"{name}: the action step is conditional")
    check("FLY_API_TOKEN" not in str(job.get("env", "")), f"{name}: token in the job env")
    check(job["steps"][0].get("uses", "").startswith("actions/checkout@"), f"{name}: no checkout before the local action")

check("github.ref == 'refs/heads/main'" in str(dep["jobs"]["deploy"].get("if", "")), "deploy-staging: job not limited to main")
check(jobs["deploy"]["needs"] == "image", "release: deploy does not need the image job")
check(jobs["deploy"]["if"] == "needs.image.outputs.pushed == 'true'", "release: deploy not gated on a pushed image")
check("steps.build.outputs.pushed" in jobs["image"]["outputs"]["pushed"], "release: image job has no pushed output")
check("FLY_API_TOKEN" not in str(dep.get("env", "")) + str(rel.get("env", "")), "a workflow-level env holds the token")

if errs:
    print("\n".join("test-staging-deploy: FAIL: " + e for e in errs), file=sys.stderr)
    sys.exit(1)
PY

[ "$fails" = 0 ] || exit 1
echo "--- Staging deploy scripts ---"
