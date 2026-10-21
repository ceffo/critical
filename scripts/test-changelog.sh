#!/usr/bin/env bash
# Tests the changelog generator offline (part of `just validate`, ADR 0039 steps 1 to 4 and 6):
#   - changelog.sh clean, table-driven: the cleaning rule (prefix, scope, `!`, trailing groups, no ticket,
#     capitalisation) and the titles never listed (docs, chore, test, refactor, perf, non-conventional);
#   - changelog.sh render: versions newest first, Features and Fixes in the commits' order, a version with
#     neither listed with empty lists;
#   - changelog.sh gen on a fixture repository (no GitHub, no network): the ranges between tags (first
#     parent only), the build's own version (no tag yet), no later version listed, the same bytes twice, an
#     empty list without tags, refused inputs.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
script="$root/scripts/changelog.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-changelog: FAIL: $*" >&2; fails=$((fails + 1)); }
git_q() { git -c user.name=t -c user.email=t@t -c init.defaultBranch=main -c commit.gpgsign=false "$@" >/dev/null 2>&1; }

# --- clean ---
# clean_case <title> <want: "feat<TAB>line", "fix<TAB>line" or empty>
clean_case() {
    local got
    got="$("$script" clean "$1")"
    [ "$got" = "$2" ] || fail "clean '$1': got '$got', want '$2'"
}
T=$'\t'
clean_case 'feat(web): added twelve new themes (CRI-1234) (#456)' "feat${T}Added twelve new themes"
clean_case 'feat: a panel' "feat${T}A panel"
clean_case 'feat(ui): scope dropped (#1)' "feat${T}Scope dropped"
clean_case 'feat!: a breaking one (#2)' "feat${T}A breaking one"
clean_case 'feat(api)!: scoped and breaking (CRI-3) (#4)' "feat${T}Scoped and breaking"
clean_case 'fix(api): no ticket here' "fix${T}No ticket here"
clean_case 'fix: three groups (a) (CRI-5) (#6)' "fix${T}Three groups"
clean_case 'fix: keeps (inner) groups (CRI-7)' "fix${T}Keeps (inner) groups"
clean_case 'fix:   spaces around   (#8)   ' "fix${T}Spaces around"
clean_case 'FIX: type in capitals' "fix${T}Type in capitals"
clean_case 'fix: already Capitalised' "fix${T}Already Capitalised"
clean_case 'fix: ünicode first' "fix${T}ünicode first"
clean_case 'fix: only a group (CRI-9)' "fix${T}Only a group"
clean_case 'fix: (CRI-9)' ""
clean_case 'fix: (CRI-9) (#1)' ""
for t in 'docs: a page' 'docs(adr): x (CRI-1)' 'test: more' 'chore(deps): bump' 'refactor: x' 'perf: faster' 'ci: a job' \
    'build: deps' 'style: fmt' 'revert: feat: x' 'Add a thing' 'feat:no space' 'feat : spaced' 'feat(: open' \
    'Merge pull request #3 from a/b' 'chore/sync-tracker (#227)' 'features: not feat' 'feat' ''; do
    clean_case "$t" ""
done

# --- render ---
want='{"versions":[{"version":"1.1.0","features":["B one","B two"],"fixes":["A fix"]},{"version":"1.0.1","features":[],"fixes":[]},{"version":"1.0.0","features":["A feat"],"fixes":["Fix two","Fix one"]}]}'
got="$("$script" render <<'IN'
@1.0.0
feat: a feat (CRI-1) (#1)
docs: not listed
fix: fix two
fix: fix one
@1.0.1
chore: nothing user-facing
Merge branch x
@1.1.0
feat: b one
fix: a fix
feat(web): b two (#9)
IN
)"
[ "$got" = "$want" ] || fail "render: got $got, want $want"
[ "$("$script" render </dev/null)" = '{"versions":[]}' ] || fail "render: empty input is not an empty list"
printf 'fix: a title before any version\n' | "$script" render | grep -qx '{"versions":\[\]}' || fail "render: a title before any version was listed"
printf 'quote " and backslash \\ and <tag>\n' | sed 's/^/@1.0.0\nfeat: /' | "$script" render | grep -qF 'Quote \" and backslash \\ and <tag>' \
    || fail "render: a quote or a backslash is not escaped"

# --- gen on a fixture repository ---
repo="$tmp/repo"
mkdir "$repo"
git_q -C "$repo" init
c() { echo "$1" >> "$repo/f"; git_q -C "$repo" add -A; git_q -C "$repo" commit -m "$1"; }
c 'chore: first commit'
c 'feat: the first feature (CRI-1) (#1)'
git_q -C "$repo" tag v0.1.0
c 'fix(api): a first fix (#2)'
c 'docs: only docs (#3)'
# A side branch merged: only the merge commit (first parent) counts, not the branch's own commits.
git_q -C "$repo" checkout -b side
c 'feat: on the side branch'
git_q -C "$repo" checkout main
git_q -C "$repo" merge --no-ff -m 'feat: the merge title (#4)' side
git_q -C "$repo" tag v0.2.0
c 'chore: nothing for users (#5)'
git_q -C "$repo" tag v0.2.1
c 'fix: after the last tag (#6)'
head="$(git -C "$repo" rev-parse HEAD)"
c 'feat: a commit after HEAD of the build'
git_q -C "$repo" tag v0.3.0
git_q -C "$repo" tag not-a-version
git_q -C "$repo" tag v1.0
gen() { (cd "$repo" && "$script" gen "$@"); }

# The build of 0.2.2 at $head: its tag does not exist, its range is after v0.2.1; v0.3.0 is later, never listed.
want='{"versions":[{"version":"0.2.2","features":[],"fixes":["After the last tag"]},{"version":"0.2.1","features":[],"fixes":[]},{"version":"0.2.0","features":["The merge title"],"fixes":["A first fix"]},{"version":"0.1.0","features":["The first feature"],"fixes":[]}]}'
got="$(gen "$head" 0.2.2)"
[ "$got" = "$want" ] || fail "gen at the build: got $got, want $want"
[ "$(gen "$head" 0.2.2)" = "$got" ] || fail "gen: two runs on the same history differ"
[ "$(gen "$head" 0.2.2 | sha256sum)" = "$(LC_ALL=C.UTF-8 TZ=Asia/Tokyo gen "$head" 0.2.2 | sha256sum)" ] || fail "gen: the output depends on the locale or the zone"
# Without a build version, the tags reachable from the commit only.
got="$(gen "$head")"
[ "$(jq -r '.versions[0].version' <<<"$got")" = 0.2.1 ] || fail "gen without a build version: $got"
# A tag at the commit being built (a re-run) is listed once, from its tag.
got="$(gen v0.3.0 0.3.0)"
[ "$(jq -c '[.versions[].version]' <<<"$got")" = '["0.3.0","0.2.1","0.2.0","0.1.0"]' ] || fail "gen with an existing tag: $got"
[ "$(jq -c '.versions[0]' <<<"$got")" = '{"version":"0.3.0","features":["A commit after HEAD of the build"],"fixes":["After the last tag"]}' ] || fail "gen with an existing tag, 0.3.0: $got"
# The first tag lists all history up to it (the first commit is a chore).
[ "$(gen v0.1.0 | jq -c '.versions')" = '[{"version":"0.1.0","features":["The first feature"],"fixes":[]}]' ] || fail "gen at the first tag"

# No tag at all (a clone without history, the image check, a dev build): an empty list, success.
bare="$tmp/bare"
mkdir "$bare"
git_q -C "$bare" init
echo x > "$bare/f"; git_q -C "$bare" add -A; git_q -C "$bare" commit -m 'feat: untagged'
[ "$(cd "$bare" && "$script" gen HEAD)" = '{"versions":[]}' ] || fail "gen without tags is not an empty list"
[ "$(cd "$bare" && "$script" gen HEAD 0.0.1 | jq -c '[.versions[] | .version]')" = '["0.0.1"]' ] || fail "gen without tags, with a build version"

# Refused inputs.
for args in "nope" "HEAD v1" "HEAD 1.2" "HEAD 01.2.3" "HEAD 1.2.3 x"; do
    # shellcheck disable=SC2086 # one word per argument
    (cd "$repo" && "$script" gen $args >/dev/null 2>&1) && fail "gen accepted '$args'"
done
"$script" nope >/dev/null 2>&1 && fail "an unknown command was accepted"

[ "$fails" = 0 ] || exit 1
echo "--- Changelog script ---"
