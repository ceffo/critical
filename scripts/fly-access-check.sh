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
# redirects to the login page. There is no bypass, for staging or production. GATE_CHECK names the gate check
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
    "$gate" "https://$app.fly.dev" || { echo "fly-access-check: FAIL: $app has a public address and the login gate check failed" >&2; exit 1; }
    echo "fly-access-check: OK ($app: public, login gate live)"
fi
