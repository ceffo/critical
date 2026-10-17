#!/usr/bin/env bash
# Tests the manual major version without the network or GitHub (part of `just validate`, ADR 0024 step 3,
# docs/deploy.md "A major version"):
#   - next-major.sh mark, gh stubbed on a fixture repository: creates the marker tag when there is none, moves
#     it (force) when there is one, refuses a commit that is not after the highest version, a newer release,
#     no version tag at all and a bad sha; calls nothing on a refused input; fails when GitHub refuses;
#   - next-major.sh preview: the next major after the highest tag (1.0.0 on 0.x);
#   - require-admin.sh worded for the Major workflow, and still refusing a non-admin;
#   - `just major`: dispatches major.yml from main, then prints what the next release will be;
#   - major.yml parsed: dispatch only, no secret, no environment, main only, the admin check before anything
#     else, the marker written last with the workflow's token only;
#   - github-tag-ruleset.sh: only "release tags" (update, deletion, admin bypass) on `v*`; no ruleset for
#     creation or for the marker (GitHub refuses the Actions-app bypass on a personal repository); a real run
#     (gh stubbed) removes the two obsolete rulesets by name and writes only "release tags".
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-major: FAIL: $*" >&2; fails=$((fails + 1)); }

stubs="$tmp/bin"; calls="$tmp/calls"; stdin_log="$tmp/stdin"
mkdir -p "$stubs"
# gh: logs the call and its stdin; `git/ref/tags/next-major` exists when $tmp/marker-exists does; any write
# fails when $tmp/gh-refuses does; collaborators/<user>/permission is admin for "boss" only.
cat > "$stubs/gh" <<STUB
#!/usr/bin/env bash
echo "gh \$*" >> "$calls"
case "\$*" in
    "api repos/o/r/git/ref/tags/next-major") [ -f "$tmp/marker-exists" ]; exit \$? ;;
    "api -X PATCH "* | "api -X POST "*) cat >> "$stdin_log"; echo >> "$stdin_log"; [ ! -f "$tmp/gh-refuses" ]; exit \$? ;;
    "api repos/o/r/collaborators/boss/permission --jq .permission") echo admin ;;
    "api repos/o/r/collaborators/"*) echo write ;;
    "workflow run "*) ;;
    *) exit 1 ;;
esac
STUB
chmod +x "$stubs/gh"
export PATH="$stubs:$PATH" GITHUB_REPOSITORY=o/r
script="$root/scripts/next-major.sh"

repo="$tmp/repo"
git init -q -b main "$repo"
cd "$repo"
git config user.email t@example.com; git config user.name test
git config commit.gpgsign false; git config tag.gpgsign false
commit() { git commit -q --allow-empty -m "$1"; git rev-parse HEAD; }
commit 'feat: first' >/dev/null
git tag -a v0.4.2 -m r
last_sha="$(git rev-parse HEAD)"
sha1="$(commit 'fix: after the last version')"

mark() { : > "$calls"; : > "$stdin_log"; status=0; out="$("$script" mark "$@" 2>&1)" || status=$?; }

# No marker yet: created (POST), pointing at the commit, with the version it will make named.
mark "$sha1"
[ "$status" = 0 ] || fail "mark: exit $status: $out"
[ "$(cat "$calls")" = "$(printf 'gh api repos/o/r/git/ref/tags/next-major\ngh api -X POST repos/o/r/git/refs --input -')" ] || fail "mark: calls $(cat "$calls")"
jq -e --arg sha "$sha1" '. == {ref: "refs/tags/next-major", sha: $sha}' "$stdin_log" >/dev/null || fail "mark: create body $(cat "$stdin_log")"
[[ "$out" == *"::notice title=Major version::next-major is on $sha1: the next release that happens is 1.0.0 (after v0.4.2)"* ]] || fail "mark: notice $out"

# A marker exists: moved (PATCH, force).
touch "$tmp/marker-exists"
sha2="$(commit 'feat: more')"
mark "$sha2"
[ "$status" = 0 ] && [ "$(sed -n 2p "$calls")" = "gh api -X PATCH repos/o/r/git/refs/tags/next-major --input -" ] || fail "mark: no PATCH of the existing marker: $(cat "$calls")"
jq -e --arg sha "$sha2" '. == {sha: $sha, force: true}' "$stdin_log" >/dev/null || fail "mark: move body $(cat "$stdin_log")"
rm -f "$tmp/marker-exists"

# Refused: the last version's own commit (the marker would be ignored), a newer release, a bad or unknown sha.
mark "$last_sha"
[ "$status" != 0 ] && [[ "$out" == *"no commit has landed since the last version"* ]] && [ ! -s "$calls" ] || fail "mark: the last version's commit is accepted: $out"
git switch -q -c elsewhere "$last_sha"
other="$(commit 'fix: never merged')"
git tag -a v0.4.3 -m r
git switch -q main
mark "$sha2"
[ "$status" != 0 ] && [[ "$out" == *"a newer release exists: v0.4.3"* ]] && [ ! -s "$calls" ] || fail "mark: a newer release elsewhere is accepted: $out"
git tag -d v0.4.3 >/dev/null
for bad in "${sha2:0:7}" 'main' "$(printf 'g%.0s' {1..40})" "$(printf 'a%.0s' {1..40})" ''; do
    mark "$bad"
    [ "$status" != 0 ] && [ ! -s "$calls" ] || fail "mark accepted '$bad'"
done
status=0; "$script" mark >/dev/null 2>&1 || status=$?; [ "$status" != 0 ] || fail "mark without a commit"
status=0; "$script" mark "$sha2" extra >/dev/null 2>&1 || status=$?; [ "$status" != 0 ] || fail "mark with an extra argument"
# GitHub refuses (the token may not write the marker): a failure, not a green run.
touch "$tmp/gh-refuses"
mark "$sha2"
[ "$status" != 0 ] && [[ "$out" == *"GitHub refused to create next-major"* ]] || fail "mark: a refused write is not a failure: $out"
touch "$tmp/marker-exists"
mark "$sha2"
[ "$status" != 0 ] && [[ "$out" == *"GitHub refused to move next-major"* ]] || fail "mark: a refused move is not a failure: $out"
rm -f "$tmp/gh-refuses" "$tmp/marker-exists"
# No version tag at all.
git init -q -b main "$tmp/untagged"
untagged="$(git -C "$tmp/untagged" -c user.email=t@e -c user.name=t -c commit.gpgsign=false commit -q --allow-empty -m 'feat: x' && git -C "$tmp/untagged" rev-parse HEAD)"
: > "$calls"; status=0; out="$(cd "$tmp/untagged" && "$script" mark "$untagged" 2>&1)" || status=$?
[ "$status" != 0 ] && [[ "$out" == *"no vMAJOR.MINOR.PATCH tag"* ]] && [ ! -s "$calls" ] || fail "mark without any version tag: $out"

# --- preview ---
preview() { printf '%s\n' "$@" | "$script" preview; }
[[ "$(preview v0.4.2 v0.9.0 v0.10.1 nonsemver)" == *"next release will be 1.0.0 (after v0.10.1)"* ]] || fail "preview on 0.x"
[[ "$(preview v1.2.3 v1.10.0 v2.0.0 v1.99.9)" == *"next release will be 3.0.0 (after v2.0.0)"* ]] || fail "preview picks the highest by number"
status=0; printf 'nonsemver\n' | "$script" preview >/dev/null 2>&1 || status=$?; [ "$status" != 0 ] || fail "preview without a version tag"
status=0; "$script" other >/dev/null 2>&1 || status=$?; [ "$status" != 0 ] || fail "an unknown command is accepted"

# --- require-admin.sh worded for the Major workflow ---
cd "$root"
: > "$calls"; status=0
out="$(ADMIN_ACTION='make the next release a major version' ADMIN_NOTHING=marked scripts/require-admin.sh dev 2>&1)" || status=$?
[ "$status" != 0 ] && [[ "$out" == *"dev is not an admin of o/r"* && "$out" == *"may make the next release a major version"* && "$out" == *"Nothing was marked"* ]] \
    || fail "require-admin: a non-admin is not refused in the Major wording: $out"
ADMIN_ACTION='make the next release a major version' scripts/require-admin.sh boss boss >/dev/null 2>&1 || fail "require-admin: an admin is refused"
status=0; scripts/require-admin.sh boss dev >/dev/null 2>&1 || status=$?; [ "$status" != 0 ] || fail "require-admin: a re-run by a non-admin passes"

# --- just major: dispatch from main, then the preview of the next release ---
fx="$tmp/just"
git init -q --bare -b main "$tmp/origin.git"
git init -q -b main "$fx"
ln -s "$root/scripts" "$fx/scripts"
git -C "$fx" -c user.email=t@e -c user.name=t -c commit.gpgsign=false -c tag.gpgsign=false commit -q --allow-empty -m 'feat: x'
git -C "$fx" remote add origin "$tmp/origin.git"
git -C "$fx" tag v0.6.1 && git -C "$fx" tag v0.7.0 && git -C "$fx" tag unrelated
git -C "$fx" push -q origin main v0.6.1 v0.7.0 unrelated
: > "$calls"; status=0
out="$(just -f "$root/justfile" -d "$fx" major 2>&1)" || status=$?
[ "$status" = 0 ] || fail "just major: exit $status: $out"
[ "$(cat "$calls")" = "gh workflow run major.yml --ref main" ] || fail "just major: dispatch $(cat "$calls")"
[[ "$out" == *"the next release will be 1.0.0 (after v0.7.0)"* ]] || fail "just major: does not say what the next release will be: $out"

# --- the workflow, parsed ---
python3 -I - "$root/.github/workflows/major.yml" <<'PY' || fails=$((fails + 1))
import subprocess, sys, yaml
wf = yaml.safe_load(open(sys.argv[1]))
errs = []
def check(ok, msg):
    if not ok: errs.append(msg)
on = wf.get(True, wf.get("on"))
check(on == {"workflow_dispatch": None}, "major: the only trigger is a manual dispatch, with no input")
check(wf["permissions"] == {"contents": "read"}, "major: workflow permissions are not read-only")
check(sorted(wf["jobs"]) == ["mark", "not-main"], "major: the jobs are not-main and mark")
check("secrets." not in open(sys.argv[1]).read(), "major: reads a secret")
refuse = wf["jobs"]["not-main"]
check(refuse.get("if") == "${{ github.ref != 'refs/heads/main' }}", "not-main: not the complement of mark's condition")
r = subprocess.run(["bash", "-e", "-c", refuse["steps"][-1]["run"]], env={"REF": "refs/heads/feat", "PATH": "/usr/bin:/bin"}, capture_output=True, text=True)
check(r.returncode != 0 and "is not main" in r.stdout, "not-main: a run from another branch does not fail saying so")
job = wf["jobs"]["mark"]
check(job.get("if") == "github.ref == 'refs/heads/main'", "mark: not limited to main")
check("environment" not in job and "uses" not in job and "concurrency" not in job,
      "mark: has an environment, calls a workflow or shares the release's concurrency group (it would replace a queued release)")
check(job["permissions"] == {"contents": "write"}, "mark: permissions are not just contents: write")
steps = job["steps"]
check(steps[0].get("uses", "").startswith("actions/checkout@") and steps[0]["with"].get("fetch-depth") == 0
      and steps[0]["with"].get("persist-credentials") is False and "ref" not in steps[0]["with"],
      "mark: the checkout is not main's whole history and tags, without persisted credentials")
admin = [i for i, st in enumerate(steps) if st.get("run", "").strip() == 'scripts/require-admin.sh "$ACTOR" "$TRIGGERING_ACTOR"']
check(admin == [1], "mark: the admin check is not the first step after the checkout")
if admin:
    e = steps[admin[0]]["env"]
    check(e.get("ACTOR") == "${{ github.actor }}" and e.get("TRIGGERING_ACTOR") == "${{ github.triggering_actor }}"
          and e.get("GH_TOKEN") == "${{ github.token }}", "mark: the admin check does not check the actor and the re-runner")
last = steps[-1]
check(len(steps) == 3 and 'scripts/next-major.sh mark "$SHA"' in last["run"] and last["env"].get("SHA") == "${{ github.sha }}"
      and last["env"].get("GH_TOKEN") == "${{ github.token }}", "mark: the marker is not written last, from main's head, with the workflow token")
check(all("if" not in st for st in steps), "mark: a conditional step could skip the admin check")
for name, j in wf["jobs"].items():
    for st in j.get("steps", []):
        check("github.event" not in str(st.get("run", "")) and "inputs." not in str(st.get("run", "")),
              f"{name}: a run script interpolates an event field (pass it through env)")
if errs:
    print("\n".join("test-major: FAIL: " + e for e in errs), file=sys.stderr)
    sys.exit(1)
PY

# --- the tag ruleset: only "release tags"; the creation and marker rulesets are gone ---
rs="$("$root/scripts/github-tag-ruleset.sh" --dry-run 2>"$tmp/rs.err")"
jq -s -e '
    length == 1
    and (.[0] | .name == "release tags" and .target == "tag" and .enforcement == "active"
         and .conditions.ref_name.include == ["refs/tags/v*"]
         and (.rules | map(.type) | sort) == ["deletion", "update"]
         and (.bypass_actors | map(.actor_type + (.actor_id | tostring)) == ["RepositoryRole5"]))' <<<"$rs" >/dev/null \
    || fail "github-tag-ruleset: the ruleset is not 'release tags' alone (update, deletion, admin bypass): $rs"
for n in "release tag creation" "next-major marker"; do
    grep -q "would remove ruleset '$n'" "$tmp/rs.err" || fail "github-tag-ruleset --dry-run: does not announce removing '$n'"
done

# A real run against a stubbed gh: the obsolete rulesets are deleted by name, 'release tags' is replaced, nothing else is written.
mkdir -p "$tmp/rsbin"
cat >"$tmp/rsbin/gh" <<'STUB'
#!/usr/bin/env bash
echo "gh $*" >>"$RS_LOG"
case "$*" in
    "repo view"*) echo o/r ;;
    "api repos/o/r/rulesets?includes_parents=false"*)
        case "$*" in
            *'"release tags"'*) echo 11 ;;
            *'"release tag creation"'*) echo 22 ;;
            *'"next-major marker"'*) echo 33 ;;
        esac ;;
    "api -X "*) cat >/dev/null 2>&1 </dev/null || true ;;
esac
STUB
chmod +x "$tmp/rsbin/gh"
: >"$tmp/rs.log"
RS_LOG="$tmp/rs.log" PATH="$tmp/rsbin:$PATH" "$root/scripts/github-tag-ruleset.sh" >/dev/null </dev/null \
    || fail "github-tag-ruleset: the real run (gh stubbed) failed"
writes="$(grep -E '^gh api -X ' "$tmp/rs.log" | sort)"
want="$(printf '%s\n' 'gh api -X DELETE repos/o/r/rulesets/22' 'gh api -X DELETE repos/o/r/rulesets/33' 'gh api -X PUT repos/o/r/rulesets/11 --input -' | sort)"
[ "$writes" = "$want" ] || fail "github-tag-ruleset: unexpected writes: $writes"

[ "$fails" = 0 ] || exit 1
echo "--- Major version scripts ---"
