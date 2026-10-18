#!/usr/bin/env bash
# Linear GraphQL helper: sends one query to Linear's API and prints the JSON response.
#
#   scripts/linear.sh '<query>' ['<variables as JSON>']      # or the query on stdin
#   scripts/linear.sh 'query { viewer { name } }' | jq -r .data.viewer.name
#
# Authentication: $LINEAR_API_KEY, else the first line of
# ${CRITICAL_PASSWORD_DIR:-~/.config/critical}/linear-api-key (written by scripts/linear-setup.sh, mode 600),
# so a shell that never sourced the environment still works. The key is handed to curl through a private
# temporary file, never in an argument, and is never printed. $LINEAR_API_URL overrides the endpoint (tests).
#
# Exit codes: 0 response without errors; 1 anything else, with one clear line on stderr: no key, no query,
# unreachable endpoint, HTTP error (a wrong key is "HTTP 401" or "HTTP 400"), invalid JSON, or a GraphQL
# `errors` field (the response is still printed on stdout, so a caller can read it).
set -euo pipefail
umask 077

fail() { echo "linear: $*" >&2; exit 1; }

key="${LINEAR_API_KEY:-}"
if [ -z "$key" ]; then
    keyfile="${CRITICAL_PASSWORD_DIR:-$HOME/.config/critical}/linear-api-key"
    [ ! -r "$keyfile" ] || IFS= read -r key <"$keyfile" || true
fi
[ -n "$key" ] || fail "no API key: run 'just linear-setup' (or export LINEAR_API_KEY)"

if [ $# -ge 1 ]; then query="$1"; else query="$(cat)"; fi
[ -n "$query" ] || fail "no query given"
variables="${2:-{\}}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
jq -n --arg q "$query" --argjson v "$variables" '{query: $q, variables: $v}' >"$tmp/body" \
    || fail "the variables are not valid JSON"
# Personal API keys go in Authorization as is, without "Bearer".
printf 'Authorization: %s\n' "$key" >"$tmp/headers"

code="$(curl -sS --max-time 30 -o "$tmp/response" -w '%{http_code}' \
    -H @"$tmp/headers" -H 'Content-Type: application/json' --data-binary @"$tmp/body" \
    "${LINEAR_API_URL:-https://api.linear.app/graphql}" 2>"$tmp/curl-err")" \
    || fail "cannot reach Linear: $(sed 's/Authorization:.*//' "$tmp/curl-err" | head -n1)"

if [ "$code" != 200 ]; then
    msg="$(jq -r '[.errors[]?.message] | join("; ")' "$tmp/response" 2>/dev/null || true)"
    fail "HTTP $code${msg:+: $msg}"
fi
jq -e . "$tmp/response" >/dev/null 2>&1 || fail "the response is not JSON"
cat "$tmp/response"
if jq -e '.errors | type == "array" and length > 0' "$tmp/response" >/dev/null; then
    fail "GraphQL error: $(jq -r '[.errors[].message] | join("; ")' "$tmp/response")"
fi
