#!/usr/bin/env bash
# Tests scripts/build-identity.sh on a fixture git repository (part of `just validate`, ADR 0024 step 1.4):
# the last v* tag plus the short commit, `.dirty` on uncommitted changes, 0.0.0 without a tag, and no
# ldflags outside a git checkout.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
script="$root/scripts/build-identity.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-build-identity: FAIL: $*" >&2; fails=$((fails + 1)); }

repo="$tmp/repo"
git init -q "$repo"
git -C "$repo" config user.email t@example.com
git -C "$repo" config user.name t
git -C "$repo" config commit.gpgsign false
echo a > "$repo/a"
git -C "$repo" add -A
git -C "$repo" commit -q -m one

# check <name> <want version>
check() {
    local out sha short
    out="$("$script" "$repo")"
    sha="$(git -C "$repo" rev-parse HEAD)"
    short="$(git -C "$repo" rev-parse --short=7 HEAD)"
    case "$out" in
    *"api.commit=$sha "*) ;;
    *) fail "$1: no full commit in: $out" ;;
    esac
    case "$out" in
    *"api.version=${2//SHORT/$short}") ;;
    *) fail "$1: want version ${2//SHORT/$short}, got: $out" ;;
    esac
}

check "no tag" "0.0.0+SHORT"
git -C "$repo" tag v0.61.1
check "on the tag" "0.61.1+SHORT"
echo b >> "$repo/a"
check "dirty tracked file" "0.61.1+SHORT.dirty"
git -C "$repo" commit -q -am two
check "commit after the tag" "0.61.1+SHORT"
echo c > "$repo/new"
check "untracked file" "0.61.1+SHORT.dirty"
git -C "$repo" tag beta-1
rm "$repo/new"
check "non-version tag ignored" "0.61.1+SHORT"

mkdir "$tmp/plain"
[ -z "$("$script" "$tmp/plain")" ] || fail "outside a checkout it printed ldflags"

[ "$fails" -eq 0 ] || exit 1
echo "test-build-identity: ok"
