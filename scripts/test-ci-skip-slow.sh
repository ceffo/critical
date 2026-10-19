#!/usr/bin/env bash
# Tests scripts/ci-skip-slow.sh on a fixture git repository (part of `just validate`, docs/agent-workflow.md
# "CI and PRs"): docs-only changes skip, a code change or a mixed one runs everything, the ci-skip label
# skips on pull_request events only, and an unreadable range runs everything.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
script="$root/scripts/ci-skip-slow.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-ci-skip-slow: FAIL: $*" >&2; fails=$((fails + 1)); }

repo="$tmp/repo"
git init -q "$repo"
git -C "$repo" config user.email t@example.com
git -C "$repo" config user.name t
git -C "$repo" config commit.gpgsign false
echo base > "$repo/seed.txt"
git -C "$repo" add -A
git -C "$repo" commit -q -m base
base="$(git -C "$repo" rev-parse HEAD)"

# change <name> <file>...: a commit on top of base touching the files, as branch <name>.
change() {
    local name="$1"
    shift
    git -C "$repo" checkout -q -B "$name" "$base"
    for f in "$@"; do
        mkdir -p "$repo/$(dirname "$f")"
        echo "$name" >> "$repo/$f"
    done
    git -C "$repo" add -A
    git -C "$repo" commit -q -m "$name"
}

# check <name> <want> <event> <labels> <base>: runs the script on the checked-out branch.
check() {
    local name="$1" want="$2" event="$3" labels="$4" from="$5" out status=0
    : > "$tmp/out"
    out="$(cd "$repo" && EVENT="$event" BASE="$from" LABELS="$labels" GITHUB_OUTPUT="$tmp/out" "$script" 2>&1)" || status=$?
    [ "$status" = 0 ] || { fail "$name: exit $status: $out"; return; }
    grep -qx "skip_slow=$want" "$tmp/out" || fail "$name: want skip_slow=$want, got: $(cat "$tmp/out") ($out)"
}

change readme README.md
check "README only" true pull_request "" "$base"
check "README only, on a push" true push "" "$base"

change docs-all docs/adr/0001-x.md web/README.md api/NOTES.md
check "docs, nested markdown and tracker only" true pull_request "bug,other" "$base"

change code api/main.go
check "code" false pull_request "" "$base"
check "code, on a push" false push "" "$base"
check "code with the ci-skip label" true pull_request "bug,ci-skip" "$base"
check "code with the ci-skip label as the only one" true pull_request "ci-skip" "$base"
check "code with the ci-skip label, on a push" false push "ci-skip" "$base"
check "code with a look-alike label" false pull_request "ci-skipped,no-ci-skip" "$base"
check "code with the no-release label" false pull_request "no-release" "$base"

change mixed README.md api/main.go
check "docs and code" false pull_request "" "$base"

change workflow .github/workflows/ci.yml
check "a workflow file" false pull_request "" "$base"

change lookalike docs.txt notes.mdx
check "files that only look like docs" false pull_request "" "$base"

change docs-label README.md
check "docs with the ci-skip label" true pull_request "ci-skip" "$base"

git -C "$repo" checkout -q -B empty "$base"
check "no change" false pull_request "" "$base"
check "no base" false push "" ""
check "a dispatched run on main (no base)" false workflow_dispatch "" ""
check "a dispatched run ignores a stray ci-skip" false workflow_dispatch "ci-skip" ""
check "a zero base (new branch)" false push "" 0000000000000000000000000000000000000000
check "an unknown base" false push "" 1111111111111111111111111111111111111111

# The output file is optional: stdout carries the decision.
out="$(cd "$repo" && git checkout -q readme && EVENT=push BASE="$base" "$script")"
[ "$out" = "skip_slow=true (only documentation changed)" ] || fail "stdout without GITHUB_OUTPUT: $out"

if [ "$fails" -ne 0 ]; then
    echo "test-ci-skip-slow: $fails failure(s)" >&2
    exit 1
fi
echo "test-ci-skip-slow: ok"
