#!/usr/bin/env bash
# Fetches the real spell catalogue for the release build (ADR 0024 steps 3.1, 7 and 10.3): one
# shallow, blob-less clone of the private critical-rulebooks repo at its default branch, through an
# ssh-agent holding the read-only deploy key, then only dist/catalogue.json is read from it.
#
#   RULEBOOKS_DEPLOY_KEY=<private key> scripts/release-catalogue.sh <dest dir> [<app source dir>]
#
# With <app source dir> (the checkout of the commit being released) the catalogue's `format` must be
# one the app there reads (scripts/catalogue-formats.sh): otherwise nothing is written, a notice names
# both sides on stderr and the exit status is 3, so the release can skip the build and the deploy of an
# image whose app would crash at boot (the rulebooks half of a format bump landing before the app half,
# or the reverse). Any other failure exits 1.
#
# Writes <dest dir>/catalogue.json (mode 644, for the image build) and prints the rulebooks commit sha,
# and nothing else, on stdout. The key is handed to the agent on stdin and never written to a file; the
# agent and the clone are removed on exit. Never prints the key nor the catalogue; never traces the
# shell. github.com's host key is pinned (its published ed25519 key), so no key is trusted on first
# use. RULEBOOKS_REMOTE overrides the remote (default: the one of `just rulebooks`).
set -euo pipefail

dest="${1:?usage: release-catalogue.sh <dest dir> [<app source dir>]}"
app_src="${2:-}"
remote="${RULEBOOKS_REMOTE:-git@github.com:ceffo/critical-rulebooks.git}"
[ -n "${RULEBOOKS_DEPLOY_KEY:-}" ] || { echo "release-catalogue: RULEBOOKS_DEPLOY_KEY is not set" >&2; exit 1; }

work="$(mktemp -d)"
agent_pid=""
cleanup() {
    [ -z "$agent_pid" ] || SSH_AGENT_PID="$agent_pid" ssh-agent -k >/dev/null 2>&1 || true
    rm -rf "$work"
}
trap cleanup EXIT

eval "$(ssh-agent -s)" >/dev/null
agent_pid="$SSH_AGENT_PID"
# A secret pasted without its final newline is refused by ssh-add; the here-string adds one.
ssh-add -q - <<<"$RULEBOOKS_DEPLOY_KEY" >/dev/null 2>&1 \
    || { echo "release-catalogue: ssh-add refused RULEBOOKS_DEPLOY_KEY (not a private key?)" >&2; exit 1; }

# https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints
echo "github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl" > "$work/known_hosts"
export GIT_SSH_COMMAND="ssh -o UserKnownHostsFile=$work/known_hosts -o StrictHostKeyChecking=yes -o BatchMode=yes"

git clone --quiet --depth 1 --filter=blob:none --no-checkout "$remote" "$work/rulebooks" >&2 \
    || { echo "release-catalogue: cannot clone $remote with the deploy key" >&2; exit 1; }
sha="$(git -C "$work/rulebooks" rev-parse HEAD)"

mkdir -p "$dest"
tmp="$(mktemp "$dest/catalogue.json.XXXXXX")"
if ! git -C "$work/rulebooks" show "HEAD:dist/catalogue.json" > "$tmp" 2>/dev/null || [ ! -s "$tmp" ]; then
    rm -f "$tmp"
    echo "release-catalogue: no dist/catalogue.json at rulebooks $sha" >&2; exit 1
fi
if [ -n "$app_src" ]; then
    format="$(jq -r '.format | tostring' "$tmp" 2>/dev/null)" \
        || { rm -f "$tmp"; echo "release-catalogue: dist/catalogue.json at rulebooks $sha is not readable JSON" >&2; exit 1; }
    readable="$("$(dirname "$0")/catalogue-formats.sh" "$app_src")" || { rm -f "$tmp"; exit 1; }
    if ! grep -qx -- "$format" <<<"$readable"; then
        rm -f "$tmp"
        echo "release-catalogue: rulebooks $sha has catalogue format $format; the app reads format $(tr '\n' ' ' <<<"$readable" | sed 's/ $//; s/ / and /g')" >&2
        exit 3
    fi
fi
chmod 644 "$tmp"
mv "$tmp" "$dest/catalogue.json"
echo "release-catalogue: catalogue.json from rulebooks $sha" >&2
echo "$sha"
