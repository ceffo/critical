#!/usr/bin/env bash
# Sets the Google login's three secrets on a fly app (ADR 0029 step 8, docs/deploy.md "Google login"):
# CRITICAL_GOOGLE_CLIENT_ID, CRITICAL_GOOGLE_CLIENT_SECRET and CRITICAL_PUBLIC_URL (https://<app>.fly.dev),
# in one command. The OAuth client comes from the JSON file the Google console offers ("Download JSON" on
# the client), or is typed when no file is given (the secret is not echoed).
#
#   scripts/fly-google-secrets.sh <app> [client_secret_*.json]
#
# Secrets reach fly on stdin only, never the command line or the log. Staged when the app has no machine
# (the first deploy applies them), applied at once (machines restarted) otherwise. Prints the redirect URI the
# console needs, never a secret. Rerunnable (replaces the values). Operator only (fly credentials).
set -euo pipefail

usage="usage: fly-google-secrets.sh <app> [client_secret_*.json]"
app="${1:?$usage}"
file="${2:-}"
[[ "$app" =~ ^[a-z][a-z0-9-]*$ ]] || { echo "fly-google-secrets: bad app name: $app" >&2; exit 2; }
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"

if [ -n "$file" ]; then
    id="$(jq -r '(.web // .installed // {}).client_id // empty' "$file")"
    secret="$(jq -r '(.web // .installed // {}).client_secret // empty' "$file")"
else
    read -r -p "Google OAuth client id: " id
    read -r -s -p "Google OAuth client secret (not echoed): " secret
    echo
fi
[[ "$id" == *.apps.googleusercontent.com ]] || { echo "fly-google-secrets: that is not a Google client id (…apps.googleusercontent.com)" >&2; exit 1; }
[ -n "$secret" ] || { echo "fly-google-secrets: no client secret" >&2; exit 1; }
url="https://$app.fly.dev"

stage=()
[ "$("$fly" machine list -a "$app" --json | jq length)" != 0 ] || stage=(--stage)
printf 'CRITICAL_GOOGLE_CLIENT_ID=%s\nCRITICAL_GOOGLE_CLIENT_SECRET=%s\nCRITICAL_PUBLIC_URL=%s\n' "$id" "$secret" "$url" \
    | "$fly" secrets import -a "$app" "${stage[@]}" >/dev/null
unset secret
echo "fly-google-secrets: $app: the three login secrets set${stage[*]:+ (staged: the first deploy applies them)}; public URL $url"
echo "fly-google-secrets: the console's authorised redirect URI for this client: $url/api/auth/google/callback"
