#!/usr/bin/env bash
# The gate check (ADR 0024 step 8.3, ADR 0029 step 6): what a visitor with no session gets from an app,
# read from the public URL. Passes only when the login gate reports itself configured and every route
# that carries content refuses the visitor. Read-only: plain GETs and one POST that must be refused, no
# cookie, prints status codes only, never a body.
#
#   scripts/gate-check.sh <base url>        # e.g. https://critical-hit.fly.dev
#
# Checked, each failing the script:
#   /api/health           200 and "login":"google" (the Google login is configured, not the dev login)
#   /api/auth/options     200, Google offered, the dev login not
#   /api/auth/dev         refused (404/405: the dev login route is not mounted on a hosted app)
#   /api/character, /api/characters, /api/spells/names, /api/spells/filters   401
#   a POST to /api/characters   refused (401 or 403), nothing created
#   / and a page (/characters)  a browser (Accept: text/html) is redirected to /login, the page shows nothing
#   /login                200, the login page (the one public page; its static bundle under /assets/ is
#                         public by ADR 0029 "the static bundle is public": it holds no data)
# Exit codes: 0 gate live; 1 a check failed; 2 bad usage; 3 the app serves nothing at all (/api/health never
# answered 200 and the last answer was no response or a 5xx: a crash-looping or stopped app), which a caller
# that is about to replace the release may accept (scripts/fly-access-check.sh, UNSERVED_OK).
# Exit 4: the app answered /api/health and then stopped answering (HTTP 000 on a later request, still so after
# GATE_CHECK_RETRY seconds): an unreachable app, not an open gate. A single refused or reset connection does not
# fail the check: a request that got no answer at all (HTTP 000) is repeated for up to GATE_CHECK_RETRY seconds
# (default 20). Any answer is judged at once and never repeated, so an open gate fails on its first answer.
# GATE_CHECK_WAIT (seconds, default 90) is how long a cold machine is waited for on the first request.
set -euo pipefail

base="${1:?usage: gate-check.sh <base url>}"
base="${base%/}"
[[ "$base" =~ ^https?://[A-Za-z0-9.:_-]+$ ]] || { echo "gate-check: bad base url: $base" >&2; exit 2; }
wait="${GATE_CHECK_WAIT:-90}"
retry="${GATE_CHECK_RETRY:-20}"
body="$(mktemp)"
hdr="$(mktemp)"
trap 'rm -f "$body" "$hdr"' EXIT
fail() { echo "gate-check: FAIL ($base): $*" >&2; exit 1; }

# get_once <path> [curl args]: status code on stdout (000: no answer); the body in $body, the headers in $hdr; no redirects.
get_once() {
    local path="$1"
    shift
    curl -sS -m 30 -o "$body" -D "$hdr" -w '%{http_code}' "$@" "$base$path" 2>/dev/null || true
}
# get <path> [curl args]: as get_once, but no answer at all is repeated for up to $retry seconds; still none:
# the app is unreachable (exit 4), which is not a verdict on the gate.
get() {
    local begin=$SECONDS code
    while :; do
        code="$(get_once "$@")"
        [ "$code" != 000 ] && break
        if [ $((SECONDS - begin)) -ge "$retry" ]; then
            echo "gate-check: UNREACHABLE ($base): $1: no answer (HTTP 000) for ${retry}s after the app had answered; the gate was not judged" >&2
            exit 4
        fi
        sleep 2
    done
    echo "$code"
}
location() { tr -d '\r' <"$hdr" | sed -n 's/^[Ll]ocation: //p' | head -n1; }

# The first request waits out a stopped machine and a cold database.
start=$SECONDS
code=000
while [ $((SECONDS - start)) -lt "$wait" ]; do
    code="$(get_once /api/health)"
    [ "$code" = 200 ] && break
    sleep 2
done
if [ "$code" != 200 ]; then
    case "$code" in
        000 | 5??) echo "gate-check: UNREACHABLE ($base): /api/health: HTTP $code for ${wait}s, the app serves nothing (the gate was not judged)" >&2; exit 3 ;;
    esac
    fail "/api/health: HTTP $code"
fi
jq -e '.login == "google"' "$body" >/dev/null 2>&1 || fail "/api/health does not report the Google login (the dev login, or a release before the gate): $(cat "$body")"
echo "gate-check: /api/health 200, login google"

code="$(get /api/auth/options)"
[ "$code" = 200 ] || fail "/api/auth/options: HTTP $code"
jq -e '.google == true and .devLogin == false' "$body" >/dev/null 2>&1 \
    || fail "/api/auth/options must offer Google and not the dev login: $(cat "$body")"
echo "gate-check: /api/auth/options 200, Google only"

code="$(get /api/auth/dev -X POST)"
case "$code" in 404 | 405 | 401 | 403) echo "gate-check: POST /api/auth/dev $code (refused)" ;; *) fail "POST /api/auth/dev: HTTP $code, the dev login must not be reachable" ;; esac

for path in /api/character /api/characters /api/spells/names /api/spells/filters; do
    code="$(get "$path")"
    [ "$code" = 401 ] || fail "GET $path without a session: HTTP $code, want 401"
    jq -e '.code == "unauthenticated"' "$body" >/dev/null 2>&1 || fail "GET $path 401 without the gate's code"
    echo "gate-check: GET $path 401"
done

code="$(get /api/characters -X POST -H 'Content-Type: application/json' -d '{}')"
case "$code" in 401 | 403) echo "gate-check: POST /api/characters $code (refused)" ;; *) fail "POST /api/characters without a session: HTTP $code, want 401 or 403" ;; esac

for path in / /characters; do
    code="$(get "$path" -H 'Accept: text/html')"
    [ "$code" = 302 ] || fail "GET $path as a browser without a session: HTTP $code, want a redirect to /login"
    case "$(location)" in /login | /login\?* | "$base"/login | "$base"/login\?*) ;; *) fail "GET $path redirects to '$(location)', want /login" ;; esac
    grep -q '<div id="root">' "$body" && fail "GET $path redirects but carries the app"
    echo "gate-check: GET $path 302 /login"
done

code="$(get /login -H 'Accept: text/html')"
[ "$code" = 200 ] || fail "/login: HTTP $code"
grep -q '<div id="root">' "$body" || fail "/login is not the web app's login page"
echo "gate-check: /login 200"
echo "gate-check: OK ($base)"
