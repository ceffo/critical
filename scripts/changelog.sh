#!/usr/bin/env bash
# The in-app changelog (ADR 0039): per release version, the Features and the Fixes, made from the
# conventional-commit titles of the first-parent (squash) commits, with no network and no LLM.
#
#   scripts/changelog.sh gen <commit> [<version of the build>]
#   scripts/changelog.sh render < input
#   scripts/changelog.sh clean <title>
#
# gen prints the changelog as JSON, newest version first, for every vX.Y.Z tag reachable from <commit>:
#   {"versions":[{"version":"1.2.3","features":["..."],"fixes":["..."]}, ...]}
# A version's commits are the first-parent ones after the previous version tag up to its own (the first
# version: all history up to it), the same range the release reads for its bump (release-version.sh).
# <version of the build> (X.Y.Z) is the release being built: its tag does not exist yet (it is made after
# the staging deploy), so its range is the commits after the last tag up to <commit>. A build never lists
# a later version (ADR 0039 step 6.2). With no tag and no version (a clone without tags, the image check) the
# list is empty, not an error. Same history, same bytes: the order is the tags' version order then the
# commits' order, nothing depends on the clock or the locale.
#
# render is the pure rule, no git: lines on stdin, oldest version first,
#   @<version>          starts a version
#   <title>             one commit of that version, in the commits' order
# and it prints the same JSON. clean prints what render lists for one title (nothing when it lists none):
#   "Feat" or "Fix" TAB the cleaned line, e.g. `feat(web): added twelve new themes (CRI-1234) (#456)`
#   gives `feat<TAB>Added twelve new themes`.
#
# The rule (ADR 0039 steps 2 and 3): only `feat` (and `feat!`) is a Feature and `fix` a Fix; every other
# type and a non-conventional title is never listed. The title is cleaned in this order: drop the
# `type(scope)!:` prefix, drop every trailing parenthesised group, trim the spaces, capitalise the first
# letter. A title left empty is not listed. Prints no secret.
set -euo pipefail

LC_ALL=C
export LC_ALL
usage="usage: changelog.sh gen <commit> [<version>] | render < input | clean <title>"
semver='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
die() { echo "changelog: $*" >&2; exit 1; }
command -v jq >/dev/null || die "jq is needed"

# clean <title>: "<feat|fix>\t<line>", or nothing when the title lists nothing.
clean() {
    local title="$1" type rest
    local conventional='^([A-Za-z]+)(\([^()]*\))?(!)?:[[:space:]]+(.*)$'
    [[ "$title" =~ $conventional ]] || return 0
    type="${BASH_REMATCH[1],,}"
    rest="${BASH_REMATCH[4]}"
    case "$type" in feat | fix) ;; *) return 0 ;; esac
    local group='^(.*[^[:space:]])?[[:space:]]*\([^()]*\)[[:space:]]*$'
    rest="${rest%"${rest##*[![:space:]]}"}"
    while [[ "$rest" =~ $group ]] || [[ "$rest" =~ ^[[:space:]]*\([^()]*\)[[:space:]]*$ ]]; do
        rest="${BASH_REMATCH[1]:-}"
        rest="${rest%"${rest##*[![:space:]]}"}"
    done
    rest="${rest#"${rest%%[![:space:]]*}"}"
    [ -n "$rest" ] || return 0
    printf '%s\t%s\n' "$type" "${rest^}"
}

# render < input: the JSON of the lines (see the header).
render() {
    local line version="" cleaned
    {
        while IFS= read -r line || [ -n "$line" ]; do
            if [[ "$line" == @* ]]; then
                version="${line#@}"
                printf 'v\t%s\n' "$version"
            elif [ -n "$version" ] && cleaned="$(clean "$line")" && [ -n "$cleaned" ]; then
                printf '%s\n' "$cleaned"
            fi
        done
    } | jq -R -s -c '
        split("\n") | map(select(length > 0) | split("\t")) as $rows
        | reduce $rows[] as $r ([];
            if $r[0] == "v" then [{version: $r[1], features: [], fixes: []}] + . 
            elif $r[0] == "feat" then .[0].features += [$r[1]]
            else .[0].fixes += [$r[1]] end)
        | {versions: .}'
}

# range_titles <previous ref or empty> <ref>: the first-parent commit titles after previous up to ref, oldest first.
range_titles() {
    if [ -n "$1" ]; then
        git log --first-parent --reverse --format=%s "$1..$2"
    else
        git log --first-parent --reverse --format=%s "$2"
    fi
}

# gen <commit> [version]: render's input from the tags, then its JSON.
gen() {
    local commit tag prev="" build="${2:-}" tags
    if [ -n "$build" ] && ! [[ "v$build" =~ $semver ]]; then die "'$build' is not a version (MAJOR.MINOR.PATCH)"; fi
    commit="$(git rev-parse -q --verify "$1^{commit}")" || die "unknown commit '$1'"
    tags="$(git tag --merged "$commit" -l 'v*' | grep -E "$semver" | sort -V || true)"
    {
        while IFS= read -r tag; do
            [ -n "$tag" ] || continue
            echo "@${tag#v}"
            range_titles "$prev" "$tag^{commit}"
            prev="$tag^{commit}"
        done <<<"$tags"
        # The build's own tag is not made yet (it follows the staging deploy); if it exists already, it is listed above.
        if [ -n "$build" ] && ! grep -qx "v$build" <<<"$tags"; then
            echo "@$build"
            range_titles "$prev" "$commit"
        fi
    } | render
}

case "${1:-}" in
    gen)
        { [ $# -ge 2 ] && [ $# -le 3 ]; } || die "$usage"
        gen "$2" "${3:-}"
        ;;
    render)
        [ $# = 1 ] || die "$usage"
        render
        ;;
    clean)
        [ $# = 2 ] || die "$usage"
        clean "$2"
        ;;
    *) die "$usage" ;;
esac
