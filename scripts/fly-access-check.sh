#!/usr/bin/env bash
# The deploy jobs' access check (ADR 0024 step 8.3, as amended): a fly app may have a public IP only while
# the login gate reports itself configured. Replaces the no-public-IP check, which stays the rule for an app
# with no public address.
#
#   scripts/fly-access-check.sh <app>
#
# No public address: the private check (scripts/fly-private-check.sh: a Flycast address, nothing public).
# A public address: the gate check (scripts/gate-check.sh) against https://<app>.fly.dev, which fails
# unless /api/health reports "login":"google" and every route that carries content answers 401 or
# redirects to the login page. UNSERVED_OK=1 (the check before a deploy only): an app that serves nothing at
# all (the gate check exits 3: no response or a 5xx on /api/health, e.g. a crash-looping release) passes,
# since it exposes no content and the deploy that replaces it must be able to run; the check after the
# deploy never sets it (exit 4, an app that stopped answering mid-check, never passes), so the new release must hold the gate. There is no bypass, for staging or production. GATE_CHECK names the gate check
# script (the offline tests stub it); prints no secret.
set -euo pipefail

app="${1:?usage: fly-access-check.sh <app>}"
here="$(cd "$(dirname "$0")" && pwd)"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
gate="${GATE_CHECK:-$here/gate-check.sh}"

public="$("$fly" ips list -a "$app" --json | jq '[.[] | select(.Type != "private_v6")] | length')"
if [ "$public" = 0 ]; then
    FLY="$fly" "$here/fly-private-check.sh" "$app"
else
    echo "fly-access-check: $app has $public public address(es): the login gate must be live"
    rc=0
    "$gate" "https://$app.fly.dev" || rc=$?
    if [ "$rc" = 3 ] && [ "${UNSERVED_OK:-}" = 1 ]; then
        echo "fly-access-check: OK ($app: public address, but it serves nothing now; the deploy replaces it and the gate is checked after)"
    elif [ "$rc" = 3 ] || [ "$rc" = 4 ]; then
        echo "fly-access-check: FAIL: $app has a public address and does not answer, so the login gate could not be checked (unreachable, not an open gate)" >&2; exit 1
    elif [ "$rc" != 0 ]; then
        echo "fly-access-check: FAIL: $app has a public address and the login gate is OPEN or misconfigured (the gate check failed)" >&2; exit 1
    else
        echo "fly-access-check: OK ($app: public, login gate live)"
    fi
fi
