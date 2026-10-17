#!/usr/bin/env bash
# Tests the release build's scripts without the network, fly or a registry (part of `just validate`):
#   - release-secrets.sh: all set -> ready=true; one missing -> ready=false, a notice naming it, exit 0
#     (the green no-op of main before the secrets exist); no value is ever printed.
#   - release-catalogue.sh: from a local rulebooks repo through an ssh-agent holding a generated key,
#     stdout is the rulebooks sha only (never the catalogue nor the key), catalogue.json is copied;
#     no key or no dist/catalogue.json fails and leaves no file.
#   - release.yml, parsed: one job; the version computed first; every step after the secrets check runs
#     only on ready == 'true' (no flyctl, no fetch, no build when a secret is missing); build and tag only
#     for a release, the tag last, after the deploy; a manual run without its secrets fails.
#   - release-image.sh: with docker and fly stubbed, one build labelled with both commits and one push
#     per app tagged with the full commit sha; with VERSION, the version baked in, labelled and pushed as a
#     v<version> tag too; a dirty tree, a short rulebooks sha or a bad VERSION builds nothing.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-release-scripts: FAIL: $*" >&2; fails=$((fails + 1)); }
git_q() { git -c user.name=t -c user.email=t@t -c init.defaultBranch=main -c commit.gpgsign=false "$@" >/dev/null; }

# --- release-secrets.sh ---
out="$tmp/out"; : > "$out"
log="$(A=secret-a B=secret-b GITHUB_OUTPUT="$out" "$root/scripts/release-secrets.sh" A B)"
[ "$(cat "$out")" = "ready=true" ] || fail "secrets set: output is '$(cat "$out")'"
[ -z "$log" ] || fail "secrets set: printed '$log'"

: > "$out"
if log="$(A=secret-a B= GITHUB_OUTPUT="$out" "$root/scripts/release-secrets.sh" A B)"; then
    [ "$(cat "$out")" = "ready=false" ] || fail "secret missing: output is '$(cat "$out")'"
    [[ "$log" == "::notice "*"B not set"* ]] || fail "secret missing: no notice naming B: '$log'"
    [[ "$log" != *secret-a* ]] || fail "secret missing: printed a secret value"
else
    fail "secret missing: exited non-zero (main would go red)"
fi

# --- release.yml, parsed: the version first, then every step that builds, deploys or tags only once the
# secrets are there (no flyctl, no fetch, no build when a secret is missing), the tag only for a release and
# after the deploy (the deploy job's own checks are in test-staging-deploy.sh) ---
python3 -I - "$root/.github/workflows/release.yml" <<'PY' || fails=$((fails + 1))
import subprocess, sys, yaml

wf = yaml.safe_load(open(sys.argv[1]))
errs = []
def check(ok, msg):
    if not ok:
        errs.append("release.yml: " + msg)

job = wf["jobs"]["release"]
steps = job["steps"]
names = [st.get("id") or st.get("name") or st.get("uses") for st in steps]
idx = {n: i for i, n in enumerate(names)}
check(list(wf["jobs"]) == ["release"], "not one job from the version to the tag")
for need in ("version", "secrets", "Refuse a manual run without its secrets", "Set up flyctl",
             "Check out the release commit", "Build and push the release image", "deploy", "Tag the version"):
    check(need in idx, f"no step {need}")
if not errs:
    check(idx["version"] < idx["secrets"], "the version is not computed first")
    check(steps[0].get("uses", "").startswith("actions/checkout@") and steps[0]["with"].get("fetch-depth") == 0,
          "the first checkout has no history (the version needs the tags)")
    for i, st in enumerate(steps[idx["secrets"] + 1:], idx["secrets"] + 1):
        if names[i] == "Refuse a manual run without its secrets":
            continue
        check("steps.secrets.outputs.ready == 'true'" in st.get("if", ""), f"step {names[i]} runs without the secrets")
    for n in ("Check out the release commit", "Build and push the release image", "Tag the version"):
        check("steps.version.outputs.release == 'true'" in steps[idx[n]]["if"], f"step {n} runs without a release")
    check("release == 'true' || steps.version.outputs.redeploy == 'true'" in steps[idx["secrets"]]["if"],
          "the secrets check runs when nothing is released")
    check(idx["deploy"] < idx["Tag the version"] and names[-1] == "Tag the version", "the tag is not the last step, after the deploy")
    check("steps.deploy.outputs.digest" in str(steps[idx["Tag the version"]]["env"]), "the tag does not name the staging digest")
    readers = [names[i] for i, st in enumerate(steps) if "{{ secrets." in str(st)]
    check(readers == ["secrets", "Build and push the release image", "deploy"], f"the secrets are read by {readers}")
    check("RULEBOOKS_DEPLOY_KEY" not in str(steps[idx["deploy"]]), "the deploy step gets the rulebooks key")
    refuse = steps[idx["Refuse a manual run without its secrets"]]
    check("github.event_name == 'workflow_dispatch'" in refuse["if"], "an automatic run without secrets would fail main")
    r = subprocess.run(["bash", "-e", "-c", refuse["run"]], capture_output=True, text=True)
    check(r.returncode != 0, "the manual refusal does not fail")
check(job["concurrency"] == {"group": "release-staging", "cancel-in-progress": False}, "not serialized without cancelling")
check(job["permissions"] == {"contents": "write", "deployments": "write", "pull-requests": "read"}, f"permissions {job['permissions']}")
check(wf["permissions"] == {"contents": "read"}, "workflow permissions are not read-only")
on = wf.get(True) or wf.get("on")
check(set(on) == {"workflow_run", "workflow_dispatch"}, "triggers are not CI completion + dispatch")
check(on["workflow_run"] == {"workflows": ["CI"], "types": ["completed"], "branches": ["main"]}, "not after CI on main")
check(list(on["workflow_dispatch"]["inputs"]) == ["version"] and on["workflow_dispatch"]["inputs"]["version"]["required"] is False,
      "the manual run's only input is an optional version")
cond = job["if"]
for part in ("github.event.workflow_run.conclusion == 'success'", "github.event.workflow_run.event == 'push'",
             "github.event.workflow_run.head_branch == 'main'", "head_repository.full_name == github.repository",
             "github.event_name == 'workflow_dispatch' && github.ref == 'refs/heads/main'"):
    check(part in cond, f"job condition lacks {part}")
check("inputs.version" in job["env"]["REDEPLOY"] and "${{" not in steps[idx["version"]]["run"],
      "the version input reaches the script other than through the environment")
if errs:
    print("\n".join("test-release-scripts: FAIL: " + e for e in errs), file=sys.stderr)
    sys.exit(1)
PY

# --- release-catalogue.sh ---
books="$tmp/rulebooks"
mkdir -p "$books/dist"
echo '{"marker":"catalogue-content"}' > "$books/dist/catalogue.json"
git_q -C "$books" init
git_q -C "$books" add -A
git_q -C "$books" commit -m catalogue
git -C "$books" config uploadpack.allowFilter true
sha="$(git -C "$books" rev-parse HEAD)"
ssh-keygen -q -t ed25519 -N '' -C test -f "$tmp/key"
key="$(cat "$tmp/key")"

stdout="$(RULEBOOKS_DEPLOY_KEY="$key" RULEBOOKS_REMOTE="file://$books" "$root/scripts/release-catalogue.sh" "$tmp/cat" 2>"$tmp/err")" \
    || fail "catalogue: exited non-zero: $(cat "$tmp/err")"
[ "$stdout" = "$sha" ] || fail "catalogue: stdout is not exactly the rulebooks sha: '$stdout'"
cmp -s "$books/dist/catalogue.json" "$tmp/cat/catalogue.json" || fail "catalogue: catalogue.json not copied"
[ "$(stat -c %a "$tmp/cat/catalogue.json" 2>/dev/null)" = 644 ] || fail "catalogue: catalogue.json is not mode 644"
! grep -q -e catalogue-content -e PRIVATE <<<"$stdout" || fail "catalogue: printed the catalogue or the key on stdout"
! grep -q -e catalogue-content -e PRIVATE "$tmp/err" || fail "catalogue: printed the catalogue or the key on stderr"

if RULEBOOKS_DEPLOY_KEY= RULEBOOKS_REMOTE="file://$books" "$root/scripts/release-catalogue.sh" "$tmp/nokey" >/dev/null 2>&1; then
    fail "catalogue: succeeded without a key"
fi
[ ! -e "$tmp/nokey/catalogue.json" ] || fail "catalogue: wrote a file without a key"

git_q -C "$books" rm -q dist/catalogue.json
git_q -C "$books" commit -m "no catalogue"
if RULEBOOKS_DEPLOY_KEY="$key" RULEBOOKS_REMOTE="file://$books" "$root/scripts/release-catalogue.sh" "$tmp/none" >/dev/null 2>&1; then
    fail "catalogue: succeeded without dist/catalogue.json"
fi
[ -z "$(ls -A "$tmp/none" 2>/dev/null)" ] || fail "catalogue: left a file when dist/catalogue.json is missing"

# --- release-image.sh ---
stubs="$tmp/bin"
mkdir -p "$stubs"
printf '#!/bin/sh\necho "docker $*" >> "%s"\n' "$tmp/calls" > "$stubs/docker"
printf '#!/bin/sh\necho "fly $*" >> "%s"\n' "$tmp/calls" > "$stubs/fly"
chmod +x "$stubs/docker" "$stubs/fly"
src="$tmp/src"
mkdir -p "$src"
echo x > "$src/file"
git_q -C "$src" init
git_q -C "$src" add -A
git_q -C "$src" commit -m src
commit="$(git -C "$src" rev-parse HEAD)"

(cd "$src" && PATH="$stubs:$PATH" FLY="$stubs/fly" "$root/scripts/release-image.sh" "$tmp/cat" "$sha" app-a app-b >/dev/null) \
    || fail "image: exited non-zero"
calls="$(cat "$tmp/calls" 2>/dev/null || true)"
[ "$(grep -c '^docker build ' <<<"$calls")" = 1 ] || fail "image: not exactly one build: $calls"
grep -q -- "--build-context catalogue=$tmp/cat " <<<"$calls" || fail "image: catalogue context not passed"
grep -q -- "--build-arg COMMIT=$commit " <<<"$calls" || fail "image: COMMIT build arg not the full sha"
grep -q -- "--label org.opencontainers.image.revision=$commit " <<<"$calls" || fail "image: no commit label"
grep -q -- "--label critical.rulebooks.revision=$sha " <<<"$calls" || fail "image: no rulebooks label"
grep -qx "fly auth docker" <<<"$calls" || fail "image: no fly auth docker"
for app in app-a app-b; do
    grep -qx "docker push registry.fly.io/$app:$commit" <<<"$calls" || fail "image: no push of $app:<full sha>"
done

: > "$tmp/calls"
(cd "$src" && PATH="$stubs:$PATH" FLY="$stubs/fly" "$root/scripts/release-image.sh" "$tmp/cat" "${sha:0:7}" app-a >/dev/null 2>&1) \
    && fail "image: accepted a short rulebooks sha"
echo y > "$src/file"
(cd "$src" && PATH="$stubs:$PATH" FLY="$stubs/fly" "$root/scripts/release-image.sh" "$tmp/cat" "$sha" app-a >/dev/null 2>&1) \
    && fail "image: built a dirty work tree"
[ ! -s "$tmp/calls" ] || fail "image: called docker or fly on a refused input: $(cat "$tmp/calls")"
echo x > "$src/file"
! grep -q 'VERSION\|image.version\|:v' <<<"$calls" || fail "image: without VERSION, a version was baked or pushed: $calls"

# With VERSION: baked in, labelled, and pushed under the version tag too (the same build).
(cd "$src" && PATH="$stubs:$PATH" FLY="$stubs/fly" VERSION=1.2.3 "$root/scripts/release-image.sh" "$tmp/cat" "$sha" app-a >/dev/null) \
    || fail "image with VERSION: exited non-zero"
calls="$(cat "$tmp/calls")"
[ "$(grep -c '^docker build ' <<<"$calls")" = 1 ] || fail "image with VERSION: not exactly one build"
grep -q -- "--build-arg VERSION=1.2.3 " <<<"$calls" || fail "image: VERSION build arg not passed: $calls"
grep -q -- "--label org.opencontainers.image.version=1.2.3 " <<<"$calls" || fail "image: no version label"
grep -qx "docker push registry.fly.io/app-a:$commit" <<<"$calls" || fail "image with VERSION: no push of the sha tag"
grep -qx "docker push registry.fly.io/app-a:v1.2.3" <<<"$calls" || fail "image with VERSION: no push of the version tag"
: > "$tmp/calls"
for v in v1.2.3 1.2 01.2.3 'x y'; do
    (cd "$src" && PATH="$stubs:$PATH" FLY="$stubs/fly" VERSION="$v" "$root/scripts/release-image.sh" "$tmp/cat" "$sha" app-a >/dev/null 2>&1) \
        && fail "image: accepted VERSION '$v'"
done
[ ! -s "$tmp/calls" ] || fail "image: called docker or fly on a refused VERSION"

[ "$fails" = 0 ] || exit 1
echo "--- Release scripts ---"
