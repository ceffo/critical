#!/usr/bin/env bash
# Tests scripts/main-catchup.sh with a stub gh (part of `just validate`): which run the catch-up starts for
# main's head, given the CI and Release runs that exist.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-main-catchup: FAIL: $*" >&2; fails=$((fails + 1)); }

mkdir "$tmp/bin"
cat > "$tmp/bin/gh" <<'STUB'
#!/usr/bin/env bash
case "$1 $2" in
    "api repos/o/r/commits/main") echo abcdef1234567 ;;
    "run list")
        case "$*" in
            *ci.yml*) cat "$STUB_CI" ;;
            *release.yml*) cat "$STUB_RELEASE" ;;
        esac ;;
    "workflow run") echo "$*" >> "$STUB_CALLS" ;;
    *) echo "unexpected gh $*" >&2; exit 1 ;;
esac
STUB
chmod +x "$tmp/bin/gh"

fresh='2027-01-15T07:59:00Z'   # 60 s before now
old='2027-01-15T07:40:00Z'     # 1200 s before now
now="$(date -d 2027-01-15T08:00:00Z +%s)"

# check <name> <want: ci|release|none> <ci runs json> <release runs json>
check() {
    local name="$1" want="$2"
    echo "$3" > "$tmp/ci"
    echo "$4" > "$tmp/release"
    : > "$tmp/calls"
    STUB_CI="$tmp/ci" STUB_RELEASE="$tmp/release" STUB_CALLS="$tmp/calls" PATH="$tmp/bin:$PATH" \
        GH_TOKEN=x GITHUB_REPOSITORY=o/r NOW="$now" "$root/scripts/main-catchup.sh" > "$tmp/out" 2>&1 \
        || { fail "$name: exited non-zero: $(cat "$tmp/out")"; return; }
    local got=none
    grep -q 'ci.yml' "$tmp/calls" && got=ci
    if grep -q 'release.yml' "$tmp/calls"; then
        got=release
        grep -q 'commit=abcdef1234567' "$tmp/calls" || fail "$name: Release not dispatched for the head commit"
    fi
    [ "$got" = "$want" ] || fail "$name: started '$got', want '$want'"
}

run() { echo "[{\"event\":\"$1\",\"status\":\"$2\",\"conclusion\":\"$3\",\"updatedAt\":\"$4\"}]"; }

check "no CI run at all" ci '[]' '[]'
check "only a pull request run" ci "$(run pull_request completed success "$old")" '[]'
check "only a cancelled run" ci "$(run push completed cancelled "$old")" '[]'
check "CI running" none "$(run workflow_dispatch in_progress '' "$old")" '[]'
check "CI queued" none "$(run push queued '' "$old")" '[]'
check "CI red" none "$(run push completed failure "$old")" '[]'
check "green, own Release run exists" none "$(run push completed success "$old")" '[{"databaseId":1}]'
check "green just now, Release not yet" none "$(run workflow_dispatch completed success "$fresh")" '[]'
check "dispatched CI green, no Release" release "$(run workflow_dispatch completed success "$old")" '[]'
check "push CI green, Release never started" release "$(run push completed success "$old")" '[]'
check "red then green rerun, no Release" release \
    '[{"event":"push","status":"completed","conclusion":"failure","updatedAt":"2027-01-15T07:00:00Z"},{"event":"push","status":"completed","conclusion":"success","updatedAt":"2027-01-15T07:40:00Z"}]' '[]'

[ "$fails" = 0 ] || exit 1
echo "test-main-catchup: ok"
