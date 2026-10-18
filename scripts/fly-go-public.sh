#!/usr/bin/env bash
# Gives a deployed fly app its public address (ADR 0024 step 8.2) and proves the login gate holds
# (scripts/gate-check.sh on https://<app>.fly.dev). Free: a shared IPv4 and an IPv6 (a dedicated IPv4
# costs money and is never allocated). If the gate check fails, every public address is released
# again (every public one, not only this run's), so the app is private as before and the script exits 1.
#
#   scripts/fly-go-public.sh <app>
#
# Run it after the first deploy of a release that carries the login (the server refuses to start on fly.io
# without the Google settings, so no release can serve without it, and /api/health reports login "google").
# Every later deploy re-checks the gate (scripts/fly-access-check.sh). Rerunnable. Operator only.
set -euo pipefail

app="${1:?usage: fly-go-public.sh <app>}"
[[ "$app" =~ ^[a-z][a-z0-9-]*$ ]] || { echo "fly-go-public: bad app name: $app" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
gate="${GATE_CHECK:-$here/gate-check.sh}"

[ "$("$fly" machine list -a "$app" --json | jq length)" != 0 ] || { echo "fly-go-public: $app has no machine: deploy it first (docs/deploy.md)" >&2; exit 1; }

ips() { "$fly" ips list -a "$app" --json; }
has() { ips | jq -e --arg t "$1" '.[] | select(.Type == $t)' >/dev/null; }
has shared_v4 || { "$fly" ips allocate-v4 --shared -a "$app" >/dev/null; }
has v6 || { "$fly" ips allocate-v6 -a "$app" >/dev/null; }
echo "fly-go-public: $app addresses: $(ips | jq -r '[.[] | "\(.Type) \(.Address)"] | join(", ")')"
# A dedicated IPv4 is a monthly charge ("nothing paid"): refuse to go on with one.
if has v4; then echo "fly-go-public: FAIL: $app has a dedicated IPv4 (paid): release it (fly ips release <address> -a $app)" >&2; exit 1; fi

if "$gate" "https://$app.fly.dev"; then
    echo "fly-go-public: OK ($app is public at https://$app.fly.dev behind the login)"
else
    echo "fly-go-public: the gate check FAILED: releasing every public address of $app" >&2
    left=0
    for a in $(ips | jq -r '.[] | select(.Type != "private_v6") | .Address'); do
        "$fly" ips release "$a" -a "$app" >/dev/null || { echo "fly-go-public: COULD NOT RELEASE $a: $app STILL HAS IT, release it by hand (fly ips release $a -a $app)" >&2; left=1; }
    done
    [ "$left" = 0 ] || echo "fly-go-public: $app IS STILL PUBLIC on the addresses above" >&2
    exit 1
fi
