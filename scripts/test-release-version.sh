#!/usr/bin/env bash
# Tests the staging release's version scripts without the network or GitHub (part of `just validate`, ADR
# 0024 step 3, docs/deploy.md "Release version"):
#   - release-version.sh bump, table-driven: each commit type's level, a breaking change (minor plus a
#     notice), docs/test/chore only and the no-release label (no release, with the reason), a mixed range
#     takes the highest, a feat earlier in the range still counts, --force, the first run after v0.1.0;
#   - release-version.sh next on a fixture repository with gh stubbed: the range after the last v* tag
#     reachable from the commit (first parent), each commit's PR labels, a BREAKING CHANGE footer, a
#     version whose tag exists elsewhere refused, no tag at all refused;
#   - the next-major marker (bump --major, and next on the fixture): after the last version and reachable -> the next
#     major (1.0.0 on 0.x); at or before the last version, or on a later commit -> ignored; docs-only -> no
#     release, still pending; used once;
#   - release-version.sh resolve: a version tag's commit, an unknown version refused with its name;
#   - version-tag.sh with gh stubbed: one annotated tag object on the commit, then its ref; refused inputs
#     call nothing; a ref GitHub refuses fails.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
script="$root/scripts/release-version.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-release-version: FAIL: $*" >&2; fails=$((fails + 1)); }

# c <title> [labels] [breaking footer 0|1] [pr]: one input line of bump.
n=0
c() { n=$((n + 1)); printf '%040d\t%s\t%s\t%s\t%s\n' "$n" "${4:-$n}" "${2:-}" "${3:-0}" "$1"; }

# bump_case <name> <last> <want release> <want version or -> <want bump> [notice substring] [--force] < commits
bump_case() {
    local name="$1" last="$2" want_release="$3" want_version="$4" want_bump="$5" want_notice="${6:-}" force="${7:-}"
    local out status=0
    : > "$tmp/out"
    # shellcheck disable=SC2086 # force is empty or one word
    out="$(GITHUB_OUTPUT="$tmp/out" "$script" bump "$last" $force 2>&1)" || status=$?
    [ "$status" = 0 ] || { fail "$name: exit $status: $out"; return; }
    grep -qx "release=$want_release" "$tmp/out" || fail "$name: release is not $want_release: $(cat "$tmp/out")"
    grep -qx "bump=$want_bump" "$tmp/out" || fail "$name: bump is not $want_bump: $(cat "$tmp/out")"
    grep -qx "last=$last" "$tmp/out" || fail "$name: last is not $last: $(cat "$tmp/out")"
    if [ "$want_version" = - ]; then
        ! grep -q '^version=' "$tmp/out" || fail "$name: a version without a release: $(cat "$tmp/out")"
    else
        grep -qx "version=$want_version" "$tmp/out" || fail "$name: version is not $want_version: $(cat "$tmp/out")"
    fi
    [ -z "$want_notice" ] || [[ "$out" == *"$want_notice"* ]] || fail "$name: no notice '$want_notice': $out"
    ! grep -q '^::' "$tmp/out" || fail "$name: a notice in the outputs"
}

# --- bump: one commit of each kind ---
c 'feat: a panel' | bump_case "feat" v1.4.2 true 1.5.0 minor
c 'feat(ui): a panel' | bump_case "feat with a scope" v1.4.2 true 1.5.0 minor
c 'Feat: capitalised' | bump_case "type case" v1.4.2 true 1.5.0 minor
for t in 'fix: a bug' 'refactor(dal): x' 'perf: faster' 'ci: a job' 'build: deps' 'style: fmt' 'revert: feat: x'; do
    c "$t" | bump_case "'$t'" v1.4.2 true 1.4.3 patch
done
for t in 'chore/sync-tracker (#227)' 'Add a thing' 'feat:no space' 'feat : spaced' 'feat(: open' 'Merge pull request #3 from a/b' ''; do
    c "$t" | bump_case "unparseable '$t'" v1.4.2 true 1.4.3 patch
done
for t in 'docs: a page' 'test: more' 'chore: tracker export' 'chore(deps): bump x' 'docs(adr): y'; do
    c "$t" | bump_case "'$t' alone" v1.4.2 false - none "docs/test/chore only since v1.4.2"
done

# --- bump: a breaking change is a minor, with a notice that a major may be due ---
c 'fix!: drop an endpoint' | bump_case "fix!" v1.4.2 true 1.5.0 minor "Major version?"
c 'feat(api)!: new shape' | bump_case "feat(scope)!" v1.4.2 true 1.5.0 minor "Major version?"
c 'fix: x' '' 1 | bump_case "BREAKING CHANGE footer" v1.4.2 true 1.5.0 minor "Major version?"
c 'refactor: BREAKING CHANGE in the title' | bump_case "BREAKING CHANGE title" v1.4.2 true 1.5.0 minor "Major version?"
out="$(c 'feat: x' | GITHUB_OUTPUT=/dev/null "$script" bump v1.4.2)"
[[ "$out" != *"Major version?"* ]] || fail "a plain feat gets the major notice"

# --- bump: ranges ---
{ c 'docs: a'; c 'fix: b'; c 'feat: c'; c 'test: d'; } | bump_case "mixed range takes the highest" v1.4.2 true 1.5.0 minor
{ c 'docs: a'; c 'fix: b'; c 'chore: c'; } | bump_case "fix among docs" v1.4.2 true 1.4.3 patch
{ c 'feat: replaced in the queue'; c 'fix: b'; } | bump_case "a skipped feat earlier still makes a minor" v1.4.2 true 1.5.0 minor
{ c 'feat: no release' 'no-release' 0 41; c 'docs: x'; c 'fix: y'; } \
    | bump_case "a feat that made no release still counts" v1.4.2 true 1.5.0 minor
{ c 'docs: a'; c 'chore: b'; c 'test: c'; } | bump_case "docs/test/chore only" v1.4.2 false - none "docs/test/chore only"

# --- bump: the no-release label ---
c 'feat: x' 'no-release' 0 77 | bump_case "no-release label" v1.4.2 false - none "no-release label on PR #77"
c 'fix: x' 'ui,no-release,deps' 0 78 | bump_case "no-release among labels" v1.4.2 false - none "no-release label on PR #78"
c 'fix: x' 'no-release-later' 0 79 | bump_case "a label that only starts with no-release" v1.4.2 true 1.4.3 patch
{ c 'docs: a'; c 'feat: b' 'no-release' 0 80; c 'fix: c' 'no-release' 0 81; } \
    | bump_case "every releasing commit labelled" v1.4.2 false - none "no-release label on PR #80 #81"

# --- bump: nothing since the last version; --force ---
bump_case "empty range" v1.4.2 false - none "no commit since v1.4.2" </dev/null
bump_case "forced, empty range" v1.4.2 true 1.4.3 patch "Forced release" --force </dev/null
c 'docs: a' | bump_case "forced, docs only" v1.4.2 true 1.4.3 patch "Forced release" --force
c 'feat: a' 'no-release' | bump_case "forced, labelled feat" v1.4.2 true 1.5.0 minor "Forced release" --force
c 'feat: a' | bump_case "forced when the rules release anyway" v1.4.2 true 1.5.0 minor "" --force

# --- bump: the first run after v0.1.0, version arithmetic ---
c 'feat(deploy): version each staging release (#228)' | bump_case "first run after v0.1.0" v0.1.0 true 0.2.0 minor
c 'fix: a' | bump_case "first fix after v0.1.0" v0.1.0 true 0.1.1 patch
c 'fix: a' | bump_case "patch 9 to 10" v1.9.9 true 1.9.10 patch
c 'feat: a' | bump_case "minor 9 to 10, patch reset" v1.9.9 true 1.10.0 minor
c 'feat: a' | bump_case "major kept" v3.0.7 true 3.1.0 minor

# --- bump --major: the marker is pending, so the release that happens is the next major ---
c 'fix: a bug' | bump_case "major over a patch" v1.4.2 true 2.0.0 major "major asked by hand" --major
c 'feat: a panel' | bump_case "major over a minor" v1.4.2 true 2.0.0 major "" --major
c 'fix!: breaking' | bump_case "major over a breaking change" v1.4.2 true 2.0.0 major "" --major
c 'fix: a bug' | bump_case "on 0.x the first major is 1.0.0" v0.7.3 true 1.0.0 major "" --major
c 'fix: a bug' | bump_case "major resets minor and patch" v3.9.9 true 4.0.0 major "" --major
c 'fix: a bug' | bump_case "major with --force" v1.4.2 true 2.0.0 major "" "--major --force"
c 'fix: a bug' | bump_case "major with --force, any order" v1.4.2 true 2.0.0 major "" "--force --major"
# A range that makes no release makes no major: the marker waits.
c 'docs: a page' | bump_case "major + docs only: no release, still pending" v1.4.2 false - none "next major (2.0.0) is still pending" --major
{ c 'docs: a'; c 'chore: b'; } | bump_case "major + docs/chore: no release" v1.4.2 false - none "still pending" --major
c 'feat: x' 'no-release' 0 82 | bump_case "major + no-release label: no release" v1.4.2 false - none "no-release label on PR #82" --major
bump_case "major + empty range: no release" v1.4.2 false - none "still pending" --major </dev/null
c 'docs: a page' | bump_case "major + docs only + force: a major release" v1.4.2 true 2.0.0 major "Forced release" "--major --force"
# Without the marker nothing is a major, and a plain run says nothing of one.
c 'docs: a page' | bump_case "docs only, no marker: no pending notice" v1.4.2 false - none "docs/test/chore only"
out="$(c 'docs: a page' | GITHUB_OUTPUT=/dev/null "$script" bump v1.4.2)"
[[ "$out" != *"still pending"* ]] || fail "a pending notice without the marker"

# --- bump: refused input ---
for last in 1.2.3 v1.2 v01.2.3 v1.2.3-rc1 ''; do
    : | "$script" bump "$last" >/dev/null 2>&1 && fail "bump accepted the last version '$last'"
done
: | "$script" bump v1.2.3 --other >/dev/null 2>&1 && fail "bump accepted an unknown flag"

# --- next: a fixture repository, gh stubbed ---
stubs="$tmp/bin"
mkdir -p "$stubs" "$tmp/prs"
# gh api repos/o/r/commits/<sha>/pulls --jq <expr>: answers $tmp/prs/<sha> (else []), through the real jq.
cat > "$stubs/gh" <<STUB
#!/usr/bin/env bash
echo "gh \$*" >> "$tmp/gh-calls"
[ "\$1" = api ] || exit 1
[ ! -f "$tmp/gh-fails" ] || exit 1
sha="\$(sed -n 's|^repos/o/r/commits/\([0-9a-f]*\)/pulls$|\1|p' <<<"\$2")"
[ -n "\$sha" ] || exit 1
f="$tmp/prs/\$sha"; [ -f "\$f" ] || f="$tmp/prs/none"
jq -r "\$4" "\$f"
STUB
echo '[]' > "$tmp/prs/none"
chmod +x "$stubs/gh"

repo="$tmp/repo"
git init -q -b main "$repo"
cd "$repo"
git config user.email t@example.com
git config user.name test
git config commit.gpgsign false
git config tag.gpgsign false
commit() { git commit -q --allow-empty -m "$1" ${2:+-m "$2"}; git rev-parse HEAD; }
commit 'feat: before the last version' >/dev/null
git tag -a v0.1.0 -m r
git tag -a v0.0.9 -m r HEAD~0      # an older version on the same commit
git tag nonsemver-v9 HEAD
c1="$(commit 'docs: a page')"
c2="$(commit 'fix: a bug (#12)')"
c3="$(commit 'feat: a panel (#13)' 'BREAKING CHANGE: the health shape')"
c4="$(commit 'chore: deps (#14)')"
# c2 is the merge commit of PR #12 (labelled no-release); another merged PR also lists it.
printf '[{"number": 9, "merged_at": "x", "merge_commit_sha": "other", "labels": [{"name": "ui"}]},
         {"number": 12, "merged_at": "x", "merge_commit_sha": "%s", "labels": [{"name": "no-release"}, {"name": "ui"}]},
         {"number": 5, "merged_at": null, "merge_commit_sha": "%s", "labels": []}]' "$c2" "$c2" > "$tmp/prs/$c2"
printf '[{"number": 13, "merged_at": "x", "merge_commit_sha": "%s", "labels": []}]' "$c3" > "$tmp/prs/$c3"

next() { : > "$tmp/out"; : > "$tmp/gh-calls"; status=0
    out="$(PATH="$stubs:$PATH" GITHUB_REPOSITORY=o/r GITHUB_OUTPUT="$tmp/out" "$script" next "$@" 2>&1)" || status=$?; }

next "$c4"
[ "$status" = 0 ] || fail "next: exit $status: $out"
[ "$(cat "$tmp/out")" = "$(printf 'release=true\nbump=minor\nlast=v0.1.0\nversion=0.2.0')" ] \
    || fail "next c4: outputs $(cat "$tmp/out")"
[[ "$out" == *"Major version?"*"${c3:0:7}"* || "$out" == *"${c3:0:7}"*"Major version?"* ]] || fail "next: the BREAKING CHANGE footer gave no notice: $out"
[ "$(grep -c 'gh api repos/o/r/commits/' "$tmp/gh-calls")" = 4 ] || fail "next: not one PR lookup per commit since v0.1.0: $(cat "$tmp/gh-calls")"

next "$c2"
[ "$(cat "$tmp/out")" = "$(printf 'release=false\nbump=none\nlast=v0.1.0')" ] || fail "next c2 (docs + labelled fix): $(cat "$tmp/out")"
[[ "$out" == *"no-release label on PR #12"* ]] || fail "next c2: the labelled PR is not named (its own merge commit's PR): $out"

next "$c1"
[[ "$(cat "$tmp/out")" == "release=false"* ]] && [[ "$out" == *"docs/test/chore only since v0.1.0"* ]] || fail "next c1: $out"
next "$c1" --force
grep -qx 'version=0.1.1' "$tmp/out" || fail "next c1 --force: $(cat "$tmp/out")"

# A new version tag moves the range; a merged side branch counts by its merge commit only (first parent).
git tag -a v0.2.0 -m r "$c4"
git switch -q -c side "$c4"
commit 'feat: on the side' >/dev/null
git switch -q main
git merge -q --no-ff side -m 'fix: merge the side branch (#15)'
m="$(git rev-parse HEAD)"
next "$m"
[ "$(cat "$tmp/out")" = "$(printf 'release=true\nbump=patch\nlast=v0.2.0\nversion=0.2.1')" ] || fail "next after v0.2.0 (first parent): $(cat "$tmp/out")"
next "$c4"
[[ "$out" == *"no commit since v0.2.0"* ]] || fail "next on a tagged commit: $out"

# The computed version tagged elsewhere (on a commit that is not before this one): refused.
git switch -q -c elsewhere "$c4"
commit 'fix: never merged' >/dev/null
git tag -a v0.2.1 -m r
git switch -q main
next "$m"
[ "$status" != 0 ] && [[ "$out" == *"a newer release exists: v0.2.1"* ]] && [ ! -s "$tmp/out" ] || fail "next: a version tagged elsewhere is accepted: $out"
git tag -d v0.2.1 >/dev/null

# An older failed run re-run after a newer one passed: m (a fix, 0.2.1) failed, then n (a feat) was tagged
# v0.3.0. Re-running m's run must not stage m's image nor tag v0.2.1 after v0.3.0: refused, nothing output.
n="$(commit 'feat: newer (#16)')"
git tag -a v0.3.0 -m r "$n"
next "$m"
[ "$status" != 0 ] && [[ "$out" == *"a newer release exists: v0.3.0, on $n, is not before $m"* ]] && [ ! -s "$tmp/out" ] \
    || fail "next: an older run re-run after a newer release is accepted: $out"
next "$m" --force
[ "$status" != 0 ] && [ ! -s "$tmp/out" ] || fail "next --force: an older run re-run after a newer release is accepted: $out"
next "$n"
[ "$status" = 0 ] && [[ "$out" == *"no commit since v0.3.0"* ]] || fail "next on the newest tagged commit: $out"
git tag -d v0.3.0 >/dev/null

# --- next: the next-major marker (a lightweight tag on a commit; the last version is v0.2.0 on c4) ---
# Pending: after the last version and reachable from the released commit. Whatever the range's level.
git tag next-major "$n"
next "$n"
[ "$(cat "$tmp/out")" = "$(printf 'release=true\nbump=major\nlast=v0.2.0\nversion=1.0.0')" ] || fail "next: marker on the released commit: $(cat "$tmp/out")"
git tag -f next-major "$m" >/dev/null
next "$n"
grep -qx 'version=1.0.0' "$tmp/out" || fail "next: marker earlier in the range (after the last version): $(cat "$tmp/out")"
next "$m"
[ "$(cat "$tmp/out")" = "$(printf 'release=true\nbump=major\nlast=v0.2.0\nversion=1.0.0')" ] || fail "next: marker on the released commit, a fix: $(cat "$tmp/out")"
git tag -d next-major >/dev/null
git tag -a next-major -m marker "$m"          # annotated: the commit it names counts
next "$n"
grep -qx 'version=1.0.0' "$tmp/out" || fail "next: an annotated marker is not read: $(cat "$tmp/out")"
git tag -d next-major >/dev/null
# Ignored: on the last version's commit (not newer), before it, or on a commit the released one does not contain.
for at in "$c4" "$c1" "$c2"; do
    git tag next-major "$at"
    next "$n"
    grep -qx 'bump=minor' "$tmp/out" && grep -qx 'version=0.3.0' "$tmp/out" || fail "next: a marker at/before the last version ($at) is not ignored: $(cat "$tmp/out")"
    git tag -d next-major >/dev/null
done
git tag next-major "$n"
next "$m"
grep -qx 'bump=patch' "$tmp/out" && grep -qx 'version=0.2.1' "$tmp/out" || fail "next: a marker on a later commit than the released one is not left for later: $(cat "$tmp/out")"
git tag -d next-major >/dev/null
next "$n"
grep -qx 'bump=minor' "$tmp/out" || fail "next: a deleted marker still counts: $(cat "$tmp/out")"
# Docs only: no release and the marker still pending; the next releasing commit takes it; one release only.
git tag -a v0.3.0 -m r "$n"
d="$(commit 'docs: a page (#17)')"
git tag next-major "$d"
next "$d"
[ "$(cat "$tmp/out")" = "$(printf 'release=false\nbump=none\nlast=v0.3.0')" ] && [[ "$out" == *"still pending"* ]] || fail "next: marker + docs only: $(cat "$tmp/out") / $out"
[ "$(git rev-parse next-major)" = "$d" ] || fail "next moved the marker"
e="$(commit 'fix: a bug (#18)')"
next "$e"
[ "$(cat "$tmp/out")" = "$(printf 'release=true\nbump=major\nlast=v0.3.0\nversion=1.0.0')" ] || fail "next: the pending marker is not taken by the next release: $(cat "$tmp/out")"
next "$e" --force
grep -qx 'version=1.0.0' "$tmp/out" || fail "next --force: marker: $(cat "$tmp/out")"
git tag -a v1.0.0 -m r "$e"                   # the major is released: the marker is now at or before the last version
f="$(commit 'feat: after the major (#19)')"
next "$f"
[ "$(cat "$tmp/out")" = "$(printf 'release=true\nbump=minor\nlast=v1.0.0\nversion=1.1.0')" ] || fail "next: a used marker makes a second major: $(cat "$tmp/out")"
git tag -d v1.0.0 v0.3.0 next-major >/dev/null

touch "$tmp/gh-fails"
next "$m"
[ "$status" != 0 ] && [[ "$out" == *"could not read the pull requests"* ]] && [ ! -s "$tmp/out" ] || fail "next: a GitHub failure is not a failure: $out"
rm -f "$tmp/gh-fails"
next deadbeef
[ "$status" != 0 ] && [[ "$out" == *"unknown commit"* ]] || fail "next: an unknown commit: $out"
next "$c1" --other
[ "$status" != 0 ] || fail "next: an unknown flag is accepted"

git init -q -b main "$tmp/untagged"
git -C "$tmp/untagged" -c user.email=t@e -c user.name=t -c commit.gpgsign=false commit -q --allow-empty -m 'feat: x'
status=0; out="$(cd "$tmp/untagged" && GITHUB_REPOSITORY=o/r "$script" next HEAD 2>&1)" || status=$?
[ "$status" != 0 ] && [[ "$out" == *"no vMAJOR.MINOR.PATCH tag"* ]] || fail "next without any version tag: $out"

# --- resolve ---
[ "$("$script" resolve 0.2.0)" = "$c4" ] || fail "resolve 0.2.0 is not its commit"
[ "$("$script" resolve v0.1.0)" = "$(git rev-parse 'v0.1.0^{commit}')" ] || fail "resolve v0.1.0"
status=0; out="$("$script" resolve 9.9.9 2>&1)" || status=$?
[ "$status" != 0 ] && [[ "$out" == *"unknown version v9.9.9"* ]] || fail "resolve of an unknown version: $out"
for v in 1.2 abc 'v1.2.3; rm -rf /' ''; do
    "$script" resolve "$v" >/dev/null 2>&1 && fail "resolve accepted '$v'"
done
cd "$root"

# --- version-tag.sh, gh stubbed ---
cat > "$stubs/gh" <<STUB
#!/usr/bin/env bash
echo "gh \$*" >> "$tmp/gh-calls"
cat >> "$tmp/gh-stdin"; echo >> "$tmp/gh-stdin"
case "\$4" in
    repos/o/r/git/tags) echo "$(printf 'f%.0s' {1..40})" ;;
    repos/o/r/git/refs) [ ! -f "$tmp/ref-exists" ] ;;
    *) exit 1 ;;
esac
STUB
sha="$(printf 'a%.0s' {1..40})"
digest="sha256:$(printf 'b%.0s' {1..64})"
tagrun() { : > "$tmp/gh-calls"; : > "$tmp/gh-stdin"; status=0
    out="$(PATH="$stubs:$PATH" GITHUB_REPOSITORY=o/r "$root/scripts/version-tag.sh" "$@" 2>&1)" || status=$?; }

tagrun 1.2.3 "$sha" "$digest"
[ "$status" = 0 ] || fail "version-tag: exit $status: $out"
[ "$(cat "$tmp/gh-calls")" = "$(printf 'gh api -X POST repos/o/r/git/tags --input - --jq .sha\ngh api -X POST repos/o/r/git/refs --input -')" ] \
    || fail "version-tag: calls $(cat "$tmp/gh-calls")"
jq -s -e --arg sha "$sha" --arg d "$digest" --arg obj "$(printf 'f%.0s' {1..40})" '
    .[0].tag == "v1.2.3" and .[0].object == $sha and .[0].type == "commit" and (.[0].message | contains($d))
    and .[0].tagger.name == "github-actions[bot]" and .[1] == {ref: "refs/tags/v1.2.3", sha: $obj}' "$tmp/gh-stdin" >/dev/null \
    || fail "version-tag: bodies $(cat "$tmp/gh-stdin")"
[[ "$out" == *"::notice title=Version::v1.2.3 tagged on $sha"* ]] || fail "version-tag: no notice: $out"

touch "$tmp/ref-exists"
tagrun 1.2.3 "$sha" "$digest"
[ "$status" != 0 ] && [[ "$out" == *"refused to create v1.2.3"* ]] || fail "version-tag: a refused ref is not a failure: $out"
rm -f "$tmp/ref-exists"

for args in "v1.2.3 $sha $digest" "1.2 $sha $digest" "1.2.3 ${sha:0:7} $digest" "1.2.3 $sha sha256:abc" "1.2.3 $sha"; do
    # shellcheck disable=SC2086
    tagrun $args
    [ "$status" != 0 ] || fail "version-tag accepted: $args"
    [ ! -s "$tmp/gh-calls" ] || fail "version-tag called gh on a refused input: $args"
done

[ "$fails" = 0 ] || exit 1
echo "--- Release version scripts ---"
