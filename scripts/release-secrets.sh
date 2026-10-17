#!/usr/bin/env bash
# The missing-secret rule of the main-branch release jobs (ADR 0024 step 10.4): when a secret the job
# needs is not set, the job skips with a notice and never fails main. Reads the named environment
# variables (the workflow maps each secret to the variable of the same name) and never prints a value.
#
#   scripts/release-secrets.sh <variable>...
#
# Writes `ready=true` or `ready=false` to $GITHUB_OUTPUT (stdout when unset); when some are missing it
# emits a GitHub `::notice::` naming them and docs/deploy.md. Exits 0 either way; later steps run on
# `ready == 'true'`.
set -euo pipefail

[ $# -gt 0 ] || { echo "usage: release-secrets.sh <variable>..." >&2; exit 2; }
out="${GITHUB_OUTPUT:-/dev/stdout}"

missing=()
for name in "$@"; do
    [ -n "${!name:-}" ] || missing+=("$name")
done

if [ ${#missing[@]} -eq 0 ]; then
    echo "ready=true" >> "$out"
    exit 0
fi
list="$(printf '%s, ' "${missing[@]}")"
echo "::notice title=Release skipped::${list%, } not set in this job's GitHub environment, so nothing was built, pushed or deployed. Set it as in docs/deploy.md (GitHub); main stays green meanwhile."
echo "ready=false" >> "$out"
