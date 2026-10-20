#!/usr/bin/env bash
# `just promote [version]` (ADR 0024 step 4, docs/deploy.md "Promotion"): starts the Promote workflow.
#
#   scripts/promote-pick.sh X.Y.Z    # (or vX.Y.Z) that version: no prompt, no TTY needed, no gum needed
#   scripts/promote-pick.sh          # no terminal: the workflow with no version (the newest that passed staging)
#   scripts/promote-pick.sh          # a terminal: a gum picker of `release-guards.sh candidates`
#
# The picker lists the versions above what production runs, up to what staging runs, newest first, staging's
# preselected: Enter dispatches that version, Esc or Ctrl-C dispatches nothing (exit 0). Production already at
# staging's version: says so, dispatches nothing. Production's version unknown (no record yet): says so and
# lists the staged versions. A failed read of the records, or a missing gum: nothing is dispatched, with the
# message and `just promote X.Y.Z`. The candidates are release-guards.sh's, the same logic the workflow's
# resolution uses, so the picker never offers a version the workflow refuses; the workflow still decides
# (admin check, guards, restore point).
#
# Test seams: PROMOTE_FORCE_TTY=1 / PROMOTE_NO_TTY=1 (the terminal check), PROMOTE_NO_FETCH=1 (no git fetch).
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
die() { echo "promote: $*" >&2; exit 1; }

dispatch() { gh workflow run promote.yml --ref main ${1:+-f "version=$1"}; }

if [ -n "${1:-}" ]; then
    [ "$#" = 1 ] || die "usage: promote-pick.sh [version]"
    dispatch "$1"
    exit
fi

tty=""
if [ -n "${PROMOTE_FORCE_TTY:-}" ]; then tty=1
elif [ -z "${PROMOTE_NO_TTY:-}" ] && [ -t 0 ] && [ -t 1 ]; then tty=1; fi
if [ -z "$tty" ]; then dispatch; exit; fi

command -v gum >/dev/null || die "gum is missing: it draws the version picker. Install it (Arch: sudo pacman -S gum; macOS: brew install gum; others: https://github.com/charmbracelet/gum#installation), or name the version: just promote X.Y.Z"

# The tags and main's history, as the workflow has them (never forcing: a version tag never moves).
if [ -z "${PROMOTE_NO_FETCH:-}" ]; then
    git fetch -q origin main 'refs/tags/v*:refs/tags/v*' \
        || die "cannot fetch origin (tags and main): nothing promoted. Name the version instead: just promote X.Y.Z"
fi
out="$("$here/release-guards.sh" candidates)" \
    || die "cannot tell what staging and production run: nothing promoted. Name the version instead: just promote X.Y.Z"
staging="$(sed -n 's/^staging=//p' <<<"$out")"
prod="$(sed -n 's/^production=//p' <<<"$out")"
mapfile -t choices < <(sed -n 's/^version=//p' <<<"$out")

if [ "$prod" = unknown ]; then
    echo "promote: production version unknown (no promotion has recorded one yet); staging runs $staging"
else
    echo "promote: production runs $prod, staging runs $staging"
fi
if [ "${#choices[@]}" = 0 ]; then
    if [ "$prod" = "$staging" ]; then
        echo "promote: production already runs staging's version ($staging): nothing to promote"
    else
        echo "promote: no version above production's ($prod) has passed staging: nothing to promote (a rollback is just promote X.Y.Z)"
    fi
    exit 0
fi

# Staging's version preselected; when it is not offered (its tag does not exist yet, or it did not pass), the
# newest version that is.
selected="${choices[0]}"
for c in "${choices[@]}"; do [ "$c" != "$staging" ] || selected="$staging"; done
status=0
pick="$(gum choose --header "Promote to production (now: $prod):" --selected "$selected" --limit 1 "${choices[@]}")" || status=$?
if [ "$status" != 0 ] || [ -z "$pick" ]; then
    echo "promote: cancelled, nothing promoted"
    exit 0
fi
dispatch "$pick"
