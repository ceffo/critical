#!/usr/bin/env bash
# One-shot Linear setup, run once by the user:  just linear-setup   (scripts/linear-setup.sh)
#
#   1. API key: with none set, opens Linear's API key page, reads the key you paste (not echoed), checks it
#      with a viewer query and only then stores it in ${CRITICAL_PASSWORD_DIR:-~/.config/critical}/linear-api-key
#      (mode 600, folder 700; never in the repo) and makes new shells export LINEAR_API_KEY from that file
#      (one marked block in ~/.bashrc and ~/.zshrc, the ones that exist). A wrong key is reported and nothing
#      is stored. With a key already set it only checks it.
#   2. Prints your name and the team CRI (a warning when the key cannot see it).
#   3. GitHub integration: confirmed through the API when Linear lists it; otherwise opens its page and asks
#      "done? [y/N]".
#
# A rerun with everything in place asks nothing and changes nothing. The key is never printed, logged or
# put in an argument. $LINEAR_API_URL points the checks at another endpoint (tests).
set -euo pipefail
umask 077

here="$(cd "$(dirname "$0")" && pwd)"
dir="${CRITICAL_PASSWORD_DIR:-$HOME/.config/critical}"
keyfile="$dir/linear-api-key"
keys_url="https://linear.app/settings/account/security"
github_url="https://linear.app/settings/integrations/github"
marker="# critical: Linear API key (scripts/linear-setup.sh)"

say() { echo "linear-setup: $*"; }
fail() { echo "linear-setup: FAIL: $*" >&2; exit 1; }
open_page() {
    say "open $1"
    if [ -z "${LINEAR_SETUP_NO_OPEN:-}" ] && command -v xdg-open >/dev/null 2>&1; then
        xdg-open "$1" >/dev/null 2>&1 || true
    fi
}

viewer_query='query { viewer { name email } teams { nodes { key name } } }'
# Checks the key in LINEAR_API_KEY against Linear; prints the response or returns 1.
check_key() { "$here/linear.sh" "$viewer_query" 2>"$tmp/err"; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

stored=""
if [ -z "${LINEAR_API_KEY:-}" ] && [ -s "$keyfile" ]; then
    IFS= read -r LINEAR_API_KEY <"$keyfile" || true
    stored=1
fi

new_key=""
if [ -z "${LINEAR_API_KEY:-}" ]; then
    open_page "$keys_url"
    say "create a personal API key there (label it 'critical', all permissions), copy it, paste it here:"
    IFS= read -rs -p "API key (hidden): " LINEAR_API_KEY || fail "no key read"
    echo
    [ -n "$LINEAR_API_KEY" ] || fail "empty key; nothing stored"
    new_key=1
fi
export LINEAR_API_KEY

if ! check_key >"$tmp/viewer"; then
    reason="$(cat "$tmp/err")"
    if [ -n "$new_key" ]; then
        fail "Linear refused that key ($reason); nothing stored"
    fi
    fail "the key already set is refused by Linear ($reason); fix or remove it (${stored:+$keyfile}${stored:-\$LINEAR_API_KEY}) and rerun"
fi
say "key accepted: $(jq -r '.data.viewer.name' "$tmp/viewer") <$(jq -r '.data.viewer.email' "$tmp/viewer")>"
if jq -e '.data.teams.nodes[] | select(.key == "CRI")' "$tmp/viewer" >/dev/null; then
    say "team CRI: $(jq -r '.data.teams.nodes[] | select(.key == "CRI") | .name' "$tmp/viewer")"
else
    say "WARNING: team CRI not visible to this key (teams: $(jq -r '[.data.teams.nodes[].key] | join(", ")' "$tmp/viewer")); the next item creates it"
fi

if [ -n "$new_key" ]; then
    mkdir -p "$dir"
    chmod 700 "$dir"
    printf '%s\n' "$LINEAR_API_KEY" >"$keyfile.new"
    mv "$keyfile.new" "$keyfile"
    chmod 600 "$keyfile"
    say "key stored in $keyfile (mode 600)"
fi

# New shells: export the key from the file. Only the rc files that exist; one marked block each.
for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
    [ -f "$rc" ] || continue
    if grep -qF "$marker" "$rc"; then continue; fi
    [ -s "$keyfile" ] || continue
    {
        echo
        echo "$marker"
        echo "[ -r \"$keyfile\" ] && export LINEAR_API_KEY=\"\$(head -n1 \"$keyfile\")\""
    } >>"$rc"
    say "new shells export LINEAR_API_KEY: added to $rc"
done

# GitHub integration: the API lists the workspace's integrations (needs the key to be allowed to see them).
github_connected() {
    "$here/linear.sh" 'query { integrations { nodes { service } } }' 2>/dev/null \
        | jq -e '[.data.integrations.nodes[].service] | map(ascii_downcase) | index("github") != null' >/dev/null 2>&1
}
if github_connected; then
    say "GitHub integration: connected"
else
    open_page "$github_url"
    say "connect GitHub there (Connect, pick the repository)."
    read -r -p "linear-setup: GitHub connected? [y/N] " answer || answer=""
    case "$answer" in
        y|Y|yes) github_connected && say "GitHub integration: connected" || say "GitHub integration: confirmed by you (the API does not list it)" ;;
        *) say "GitHub integration NOT confirmed: rerun 'just linear-setup' once it is connected"; exit 2 ;;
    esac
fi
say "done. Open a new shell (or: export LINEAR_API_KEY=\"\$(head -n1 $keyfile)\") to use scripts/linear.sh"
