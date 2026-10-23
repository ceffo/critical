#!/usr/bin/env bash
# gate-stamp.sh write | check <worktree> : ties "the gate is green" to the exact tree it ran on (sup-84 retro:
# a round reported green while its committed HEAD was red, a fixture edited after the gate).
#   write  (last step of `just validate`): records the tree of the working copy as validated, tracked
#          and untracked-not-ignored files included, i.e. what a commit of everything would hold.
#   check  (ship.sh, before pushing): passes only when <worktree>'s HEAD tree is the validated one and
#          nothing is left uncommitted. Prints why it refuses.
# The stamp lives in the worktree's own git dir (git rev-parse --git-path), never in the tree.
set -euo pipefail
tree_now() { # tree of the working copy, through a throwaway index: the real index is untouched
  local idx; idx=$(mktemp); trap 'rm -f "$idx"' RETURN
  cp "$(git -C "$1" rev-parse --git-path index)" "$idx" 2>/dev/null || rm -f "$idx" # no index yet: git 2.56 refuses an empty file, a missing one is an empty index
  GIT_INDEX_FILE=$idx git -C "$1" add -A >/dev/null 2>&1
  GIT_INDEX_FILE=$idx git -C "$1" write-tree
}
stamp_of() { local p; p=$(git -C "$1" rev-parse --git-path validate-green); [[ $p = /* ]] || p="$1/$p"; echo "$p"; }
case ${1:-} in
  write) wt=$(git rev-parse --show-toplevel); tree_now "$wt" > "$(stamp_of "$wt")" ;;
  check)
    wt=${2:?usage: gate-stamp.sh check <worktree>}
    head=$(git -C "$wt" rev-parse 'HEAD^{tree}')
    [ "$(tree_now "$wt")" = "$head" ] || { echo "gate-stamp: uncommitted changes in $wt: commit them and rerun just validate" >&2; exit 1; }
    f=$(stamp_of "$wt")
    [ -f "$f" ] || { echo "gate-stamp: just validate never passed in $wt" >&2; exit 1; }
    [ "$(cat "$f")" = "$head" ] || { echo "gate-stamp: HEAD $(git -C "$wt" rev-parse --short HEAD) is not the tree just validate passed on: rerun it" >&2; exit 1; } ;;
  *) echo "usage: gate-stamp.sh write | check <worktree>" >&2; exit 2 ;;
esac
