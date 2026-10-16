#!/usr/bin/env bash
# Checks that a fly app has no public IP (ADR 0024 step 8): every address must be a private Flycast
# IPv6, and there must be at least one so the app is reachable through `fly proxy` / WireGuard.
#
#   scripts/fly-private-check.sh <app>
#
# Read-only. Prints the address types; exits 1 on a public address or when there is no Flycast one.
set -euo pipefail

app="${1:?usage: fly-private-check.sh <app>}"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"

ips="$("$fly" ips list -a "$app" --json)"
echo "$ips" | jq -r --arg app "$app" '.[] | "\($app): \(.Type) \(.Address) \(.Region)"'
public="$(echo "$ips" | jq '[.[] | select(.Type != "private_v6")] | length')"
private="$(echo "$ips" | jq '[.[] | select(.Type == "private_v6")] | length')"
[ "$public" = 0 ] || { echo "fly-private-check: FAIL: $app has $public public address(es)" >&2; exit 1; }
[ "$private" -ge 1 ] || { echo "fly-private-check: FAIL: $app has no private Flycast address" >&2; exit 1; }
echo "fly-private-check: OK ($app: private only)"
