#!/usr/bin/env bash
# Creates a fly app of the release flow (ADR 0024 step 8.5) with everything it needs before its first deploy,
# in one command: the app (named in deploy/fly.<env>.toml), its private Flycast address, and its database
# secret CRITICAL_DSN copied from the app it replaces (same cluster, same database, same user: the data
# stays). Nothing public and nothing paid: no IP but the private one, no volume.
#
#   scripts/fly-app-create.sh <staging|prod> [--dsn-from <old app>]
#
# The DSN is read inside the old app's machine (`fly ssh console`, starting one when none runs) and piped
# straight into `fly secrets import --stage` on the new app: it is never printed or written to a file. Without
# --dsn-from the secret is not set: run `just fly-db-provision` (docs/deploy.md). Rerunnable: an existing app
# and an existing private address are kept. Operator only (fly credentials).
set -euo pipefail

usage="usage: fly-app-create.sh <staging|prod> [--dsn-from <old app>]"
env="${1:?$usage}"
[[ "$env" =~ ^(staging|prod)$ ]] || { echo "fly-app-create: $usage" >&2; exit 2; }
shift
old=""
if [ "${1:-}" = --dsn-from ]; then old="${2:?$usage}"; shift 2; fi
[ "$#" = 0 ] || { echo "fly-app-create: $usage" >&2; exit 2; }
[ -z "$old" ] || [[ "$old" =~ ^[a-z][a-z0-9-]*$ ]] || { echo "fly-app-create: bad app name: $old" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
app="$(sed -n 's/^app = "\(.*\)"$/\1/p' "$here/../deploy/fly.$env.toml")"
[ -n "$app" ] || { echo "fly-app-create: no app in deploy/fly.$env.toml" >&2; exit 1; }

if "$fly" apps list --json | jq -e --arg a "$app" '.[] | select(.Name == $a)' >/dev/null; then
    echo "fly-app-create: $app exists"
else
    "$fly" apps create "$app" --org personal
fi
if [ "$("$fly" ips list -a "$app" --json | jq '[.[] | select(.Type == "private_v6")] | length')" = 0 ]; then
    "$fly" ips allocate-v6 --private -a "$app"
fi

if [ -n "$old" ]; then
    machines="$("$fly" machine list -a "$old" --json)"
    id="$(jq -r '[.[] | select(.state == "started")][0].id // empty' <<<"$machines")"
    if [ -z "$id" ]; then
        id="$(jq -r '.[0].id // empty' <<<"$machines")"
        [ -n "$id" ] || { echo "fly-app-create: $old has no machine to read its database secret from" >&2; exit 1; }
        echo "fly-app-create: no machine of $old is running: starting $id"
        "$fly" machine start "$id" -a "$old" >/dev/null
        for _ in $(seq 1 30); do
            [ "$("$fly" machine list -a "$old" --json | jq -r --arg id "$id" '.[] | select(.id == $id) | .state')" = started ] && break
            sleep 2
        done
    fi
    dsn="$("$fly" ssh console -a "$old" --machine "$id" -C 'printenv CRITICAL_DSN' | tr -d '\r\n')"
    [[ "$dsn" == postgres://* ]] || { echo "fly-app-create: $old gave no database URL (is CRITICAL_DSN set there?)" >&2; exit 1; }
    printf 'CRITICAL_DSN=%s\n' "$dsn" | "$fly" secrets import -a "$app" --stage >/dev/null
    unset dsn
    echo "fly-app-create: $app: CRITICAL_DSN staged, copied from $old (the same database; never printed)"
else
    echo "fly-app-create: CRITICAL_DSN not set (no --dsn-from): just fly-db-provision tournesol-db $env"
fi
echo "fly-app-create: OK ($app). Next: just fly-google-secrets $app, then the first deploy (docs/deploy.md)"
