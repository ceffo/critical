#!/usr/bin/env bash
# The version of a staging release (ADR 0024 step 3, docs/deploy.md "Release version"): computed from the
# conventional-commit titles of ALL main commits since the last version, before the image is built.
#
#   scripts/release-version.sh bump <last tag vX.Y.Z> [--force]   < commits
#   scripts/release-version.sh next <commit> [--force]
#   scripts/release-version.sh resolve <X.Y.Z or vX.Y.Z>
#
# bump is the rule itself, pure (no git, no GitHub): one line per commit on stdin, oldest first,
#   <sha> TAB <PR number or empty> TAB <PR labels, comma-separated> TAB <1 if the body has a
#   BREAKING CHANGE footer, else 0> TAB <title>
# Each commit has a level: `feat` minor; `docs`, `test`, `chore` none; any other type (`fix`, `refactor`,
# `perf`, ...) and a title that is not `type(scope)!: description` patch. A breaking change (`!` after the
# type, or BREAKING CHANGE) is a minor, never a major (majors are made by hand), with a notice that one may
# be due. The next version takes the highest level of the range, every commit counted (one whose run was
# replaced in the queue, or that made no release, too). A release happens when at least one commit has a
# level and its PR does not carry the label `no-release`; otherwise none, with the reason. --force (the
# manual run) releases anyway, at least a patch.
#
# next gathers that input for the first-parent commits after the highest vX.Y.Z tag reachable from
# <commit> (labels of each commit's merged PR from the GitHub API, `gh` with GH_TOKEN and
# GITHUB_REPOSITORY), then runs bump. It needs the history and the tags (fetch-depth 0).
#
# resolve prints the full sha of the commit a version tag names, or fails with a message naming the
# version (an unknown version).
#
# Outputs (bump and next) go to $GITHUB_OUTPUT (default stdout), one key=value per line: release=true|false,
# bump=minor|patch|none, last=vX.Y.Z and, for a release, version=X.Y.Z. Notices are printed as GitHub
# workflow commands. Prints no secret.
set -euo pipefail

usage="usage: release-version.sh bump <vX.Y.Z> [--force] < commits | next <commit> [--force] | resolve <version>"
semver='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
out="${GITHUB_OUTPUT:-/dev/stdout}"
die() { echo "release-version: $*" >&2; exit 1; }

# level <type> <breaking 0|1>: 0 none, 1 patch, 2 minor.
level() {
    if [ "$2" = 1 ]; then echo 2; return; fi
    case "$1" in
        feat) echo 2 ;;
        docs | test | chore) echo 0 ;;
        *) echo 1 ;;
    esac
}

# bump <last tag> <force 0|1> < commits: prints the notices and the key=value outputs.
bump() {
    local last="$1" force="$2"
    [[ "$last" =~ $semver ]] || die "the last version '$last' is not vMAJOR.MINOR.PATCH"
    local major="${BASH_REMATCH[1]}" minor="${BASH_REMATCH[2]}" patch="${BASH_REMATCH[3]}"
    local top=0 releasing=0 count=0 labelled=()
    local line sha pr labels footer title type breaking lvl
    local conventional='^([A-Za-z]+)(\([^()]*\))?(!)?: [^ ]'
    while IFS= read -r line; do
        [ -n "$line" ] || continue
        # Five tab-separated fields, empty ones included (read would merge consecutive tabs).
        sha="${line%%$'\t'*}"; line="${line#*$'\t'}"
        pr="${line%%$'\t'*}"; line="${line#*$'\t'}"
        labels="${line%%$'\t'*}"; line="${line#*$'\t'}"
        footer="${line%%$'\t'*}"; title="${line#*$'\t'}"
        count=$((count + 1))
        type="" breaking=0
        if [[ "$title" =~ $conventional ]]; then
            type="${BASH_REMATCH[1],,}"
            [ -z "${BASH_REMATCH[3]}" ] || breaking=1
        fi
        if [ "$footer" = 1 ] || [[ "$title" == *"BREAKING CHANGE"* ]]; then breaking=1; fi
        if [ "$breaking" = 1 ]; then
            echo "::notice title=Major version?::${sha:0:7} \"$title\" is marked as a breaking change: it counts as a minor here; a major version is made by hand"
        fi
        lvl="$(level "$type" "$breaking")"
        if [ "$lvl" -gt "$top" ]; then top="$lvl"; fi
        if [ "$lvl" -gt 0 ]; then
            if [[ ",$labels," == *",no-release,"* ]]; then
                labelled+=("#${pr:-?}")
            else
                releasing=1
            fi
        fi
    done

    local reason=""
    if [ "$releasing" = 0 ]; then
        if [ "$count" = 0 ]; then
            reason="no commit since $last"
        elif [ "${#labelled[@]}" -gt 0 ]; then
            reason="no-release label on PR ${labelled[*]}"
        else
            reason="docs/test/chore only since $last"
        fi
        if [ "$force" != 1 ]; then
            echo "::notice title=No release::no release: $reason. Nothing is built, deployed or tagged."
            printf 'release=false\nbump=none\nlast=%s\n' "$last"
            return
        fi
        if [ "$top" = 0 ]; then top=1; fi
        echo "::notice title=Forced release::a manual run releases main's head although the rules make none ($reason)"
    fi

    local kind version
    if [ "$top" = 2 ]; then
        kind=minor version="$major.$((minor + 1)).0"
    else
        kind=patch version="$major.$minor.$((patch + 1))"
    fi
    echo "::notice title=Release version::$version ($kind after $last, $count commit(s))"
    printf 'release=true\nbump=%s\nlast=%s\nversion=%s\n' "$kind" "$last" "$version"
}

# emit <bump's output>: the notices to stdout, the key=value lines to $GITHUB_OUTPUT.
emit() {
    grep '^::' <<<"$1" || true
    grep -v '^::' <<<"$1" >> "$out"
}

# commits <last tag> <commit>: bump's input for the first-parent commits in last..commit.
commits() {
    local last="$1" commit="$2" repo list sha body footer title prs
    repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}"
    list="$(git rev-list --reverse --first-parent "$last..$commit")"
    for sha in $list; do
        title="$(git log -1 --format=%s "$sha")"
        body="$(git log -1 --format=%b "$sha")"
        footer=0
        if grep -qE '^BREAKING[ -]CHANGE:' <<<"$body"; then footer=1; fi
        # The merged PR whose merge commit this is (else any merged PR of the commit): its number and labels.
        prs="$(gh api "repos/$repo/commits/$sha/pulls" --jq "
            [.[] | select(.merged_at != null)] | ([.[] | select(.merge_commit_sha == \"$sha\")] + .) | first
            | select(. != null) | \"\(.number)\t\([.labels[].name] | join(\",\"))\"")" \
            || die "could not read the pull requests of $sha from GitHub"
        [ -n "$prs" ] || prs=$'\t'
        printf '%s\t%s\t%s\t%s\n' "$sha" "$prs" "$footer" "${title//$'\t'/ }"
    done
}

force_flag() { [ "${1:-}" = --force ] && echo 1 || echo 0; }

cmd="${1:-}"
case "$cmd" in
    bump)
        [ $# -ge 2 ] && [ $# -le 3 ] && { [ $# = 2 ] || [ "$3" = --force ]; } || die "$usage"
        result="$(bump "$2" "$(force_flag "${3:-}")")"
        emit "$result"
        ;;
    next)
        [ $# -ge 2 ] && [ $# -le 3 ] && { [ $# = 2 ] || [ "$3" = --force ]; } || die "$usage"
        commit="$(git rev-parse -q --verify "$2^{commit}")" || die "unknown commit '$2'"
        last="$(git tag --merged "$commit" -l 'v*' | grep -E "$semver" | sort -V | tail -n 1 || true)"
        [ -n "$last" ] || die "no vMAJOR.MINOR.PATCH tag before $commit: the first version is tagged by hand"
        input="$(commits "$last" "$commit")"
        result="$(bump "$last" "$(force_flag "${3:-}")" <<<"$input")"
        version="$(sed -n 's/^version=//p' <<<"$result")"
        if [ -n "$version" ] && git rev-parse -q --verify "refs/tags/v$version" >/dev/null; then
            die "v$version exists already, on $(git rev-parse "v$version^{commit}"), which is not before $commit"
        fi
        emit "$result"
        ;;
    resolve)
        [ $# = 2 ] || die "$usage"
        tag="v${2#v}"
        [[ "$tag" =~ $semver ]] || die "'$2' is not a version (MAJOR.MINOR.PATCH, e.g. 1.2.3)"
        git rev-parse -q --verify "refs/tags/$tag^{commit}" \
            || die "unknown version $tag: no such tag (the versions are the v* tags CI made after a passed staging deploy)"
        ;;
    *) die "$usage" ;;
esac
