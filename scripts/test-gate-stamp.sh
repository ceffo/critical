#!/usr/bin/env bash
# Tests scripts/gate-stamp.sh (part of `just validate`) on a throwaway repo: no stamp refuses; a stamp on the
# committed tree passes; an edit after the gate refuses (dirty, then committed); untracked files count.
set -euo pipefail
S="$(cd "$(dirname "$0")" && pwd)/gate-stamp.sh"
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
fails=0; fail() { echo "test-gate-stamp: FAIL: $*" >&2; fails=$((fails + 1)); }
r=$tmp/repo; git init -q "$r"; git -C "$r" config user.email t@t; git -C "$r" config user.name t
echo a > "$r/f"; git -C "$r" add f; git -C "$r" commit -qm one
"$S" check "$r" 2>/dev/null && fail "no stamp passed"
(cd "$r" && "$S" write); "$S" check "$r" || fail "stamp on HEAD refused"
echo b > "$r/f"; "$S" check "$r" 2>/dev/null && fail "dirty tree passed"
git -C "$r" commit -qam two; "$S" check "$r" 2>/dev/null && fail "commit after the gate passed"
(cd "$r" && "$S" write); "$S" check "$r" || fail "rerun gate refused"
echo n > "$r/new"; "$S" check "$r" 2>/dev/null && fail "untracked file passed"
(cd "$r" && "$S" write); git -C "$r" add new; git -C "$r" commit -qm three
"$S" check "$r" || fail "gate with untracked file, then committed as is, refused"
[ $fails = 0 ] && echo "test-gate-stamp: ok" || exit 1
