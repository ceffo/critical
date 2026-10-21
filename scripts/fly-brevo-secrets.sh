#!/usr/bin/env bash
# Sets the invite mail's secrets on a fly app (ADR 0029 steps 2.5 to 2.10, docs/deploy.md "Invite emails"):
# CRITICAL_BREVO_API_KEY, CRITICAL_MAIL_FROM and CRITICAL_MAIL_FROM_NAME, in one command.
#
#   scripts/fly-brevo-secrets.sh <app> <sender address> ["Sender name"]
#
# The Brevo v3 API key is typed hidden (or taken from $BREVO_API_KEY when a password manager exports it). It
# reaches curl and fly on stdin only, never a command line, a file or the log. The sender is looked up through
# Brevo's API; when it is not there yet it is created, and Brevo mails it a confirmation link. The secrets are
# only set once the sender is verified (active): until then the script says what to click and stops, so run it
# again afterwards. Staged when the app has no machine, applied at once (machines restarted) otherwise.
# Rerunnable. Operator only (fly credentials).
set -euo pipefail

usage="usage: fly-brevo-secrets.sh <app> <sender address> [\"Sender name\"]"
app="${1:?$usage}"
from="${2:?$usage}"
name="${3:-Critical Hit}"
[[ "$app" =~ ^[a-z][a-z0-9-]*$ ]] || { echo "fly-brevo-secrets: bad app name: $app" >&2; exit 2; }
[[ "$from" =~ ^[^[:space:]@\"\\]+@[^[:space:]@\"\\]+\.[^[:space:]@\"\\]+$ ]] || { echo "fly-brevo-secrets: bad sender address: $from" >&2; exit 2; }
[[ "$name" != *[\"\\]* ]] || { echo "fly-brevo-secrets: the sender name may not contain quotes or backslashes" >&2; exit 2; }
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
api="${BREVO_API_URL:-https://api.brevo.com/v3}"

key="${BREVO_API_KEY:-}"
if [ -z "$key" ]; then
    read -r -s -p "Brevo v3 API key (not echoed): " key
    echo
fi
[ -n "$key" ] || { echo "fly-brevo-secrets: no API key" >&2; exit 1; }
[[ "$key" != *[[:space:]\"\\]* ]] || { echo "fly-brevo-secrets: that does not look like an API key" >&2; exit 1; }

# brevo METHOD PATH [JSON BODY]: the key and the body go to curl as a config on stdin; prints "<status>\n<body>".
brevo() {
    local method="$1" path="$2" body="${3:-}"
    {
        printf 'header = "api-key: %s"\nheader = "accept: application/json"\n' "$key"
        [ -z "$body" ] || printf 'header = "content-type: application/json"\ndata = "%s"\n' "${body//\"/\\\"}"
    } | curl -sS -K - -X "$method" -w '\n%{http_code}' "$api$path"
}
# check_status <label> <response>: fails on anything but 2xx, with Brevo's own message (never the key).
check_status() {
    local code="${2##*$'\n'}" body="${2%$'\n'*}"
    [[ "$code" == 2* ]] && return 0
    echo "fly-brevo-secrets: $1 failed (HTTP $code): $(jq -r '.message // .' <<<"$body" 2>/dev/null || echo "$body")" >&2
    [ "$code" != 401 ] || echo "fly-brevo-secrets: Brevo refused the key: it must be a v3 API key (SMTP & API > API Keys)." >&2
    exit 1
}

resp="$(brevo GET /senders)"
check_status "listing the senders" "$resp"
active="$(jq -r --arg e "${from,,}" '[.senders[]? | select((.email | ascii_downcase) == $e)] | if length == 0 then "missing" elif .[0].active then "yes" else "no" end' <<<"${resp%$'\n'*}")"

case "$active" in
missing)
    body="$(jq -nc --arg n "$name" --arg e "$from" '{name: $n, email: $e}')"
    resp="$(brevo POST /senders "$body")"
    check_status "creating the sender" "$resp"
    echo "fly-brevo-secrets: $from added as a sender. Brevo mailed it a confirmation: open the mail and confirm,"
    echo "fly-brevo-secrets: then run this command again (nothing was set on $app)."
    exit 3
    ;;
no)
    echo "fly-brevo-secrets: $from is a sender but not verified yet. Open Brevo's confirmation mail in that mailbox"
    echo "fly-brevo-secrets: (Brevo > Senders, Domains & Dedicated IPs > Senders lets you resend it), then run this again."
    exit 3
    ;;
esac

stage=()
[ "$("$fly" machine list -a "$app" --json | jq length)" != 0 ] || stage=(--stage)
printf 'CRITICAL_BREVO_API_KEY=%s\nCRITICAL_MAIL_FROM=%s\nCRITICAL_MAIL_FROM_NAME=%s\n' "$key" "$from" "$name" \
    | "$fly" secrets import -a "$app" "${stage[@]}" >/dev/null
unset key
echo "fly-brevo-secrets: $app: sender $from verified, the mail secrets set${stage[*]:+ (staged: the first deploy applies them)}"
