#!/usr/bin/env bash
# Sets the root admin (ADR 0029 step 5.1) of a deployed app, in one command:
#
#   scripts/fly-root-admin.sh <app> <email>     # e.g. critical-staging you@gmail.com
#
# Runs the image's /app/critical-rootadmin inside the app's own machine through `fly ssh console`, so it
# uses the machine's CRITICAL_DSN secret: the DSN never reaches this machine. If no machine of the app is
# started (staging stops when idle), it starts one first. Idempotent (docs/deploy.md, "Root admin").
set -euo pipefail

usage="usage: fly-root-admin.sh <app> <email>"
app="${1:?$usage}"
email="${2:?$usage}"
[[ "$app" =~ ^[a-z][a-z0-9-]*$ ]] || { echo "fly-root-admin: bad app name: $app" >&2; exit 2; }
[[ "$email" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+$ ]] || { echo "fly-root-admin: bad email: $email" >&2; exit 2; }
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"

machines="$("$fly" machine list -a "$app" --json)"
id="$(jq -r '[.[] | select(.state == "started")][0].id // empty' <<<"$machines")"
if [ -z "$id" ]; then
    id="$(jq -r '.[0].id // empty' <<<"$machines")"
    [ -n "$id" ] || { echo "fly-root-admin: $app has no machine (deploy it first: docs/deploy.md)" >&2; exit 1; }
    echo "fly-root-admin: no machine of $app is running: starting $id"
    "$fly" machine start "$id" -a "$app" >/dev/null
    for _ in $(seq 1 30); do
        [ "$("$fly" machine list -a "$app" --json | jq -r --arg id "$id" '.[] | select(.id == $id) | .state')" = started ] && break
        sleep 2
    done
fi

# fly ssh console runs the command with the machine's environment, secrets included. The command is run
# without a shell; the email was checked above.
"$fly" ssh console -a "$app" --machine "$id" -C "/app/critical-rootadmin $email"
