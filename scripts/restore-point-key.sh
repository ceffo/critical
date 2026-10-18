#!/usr/bin/env bash
# The passphrase that encrypts the promotion's restore points (ADR 0024 step 9.2; docs/deploy.md,
# "Promotion"). Operator, once (an admin of the repository, gh logged in):
#
#   scripts/restore-point-key.sh        # just restore-point-key
#
# Without the file ${CRITICAL_PASSWORD_DIR:-~/.config/critical}/restore-point.passphrase it generates a
# random passphrase (32 bytes, base64) into it (mode 600); then it sets the production environment's secret
# RESTORE_POINT_PASSPHRASE from that file, through stdin. An existing file is never overwritten: a rerun only
# sets the secret again from it. GitHub never shows a secret back, so this file (and a copy of it in a
# password manager) is the only way to decrypt a restore point: lose both and the restore points cannot be
# read. Changing the passphrase (move the file aside, rerun) leaves the older restore points readable only
# with the older passphrase. Prints no secret.
set -euo pipefail
umask 077

fail() { echo "restore-point-key: FAIL: $*" >&2; exit 1; }
dir="${CRITICAL_PASSWORD_DIR:-$HOME/.config/critical}"
file="$dir/restore-point.passphrase"

if [ -s "$file" ]; then
    echo "restore-point-key: $file exists: kept"
else
    mkdir -p "$dir"
    openssl rand -base64 32 >"$file.new" || fail "could not generate a passphrase"
    mv "$file.new" "$file"
    echo "restore-point-key: new passphrase in $file (mode 600)"
fi
gh secret set RESTORE_POINT_PASSPHRASE --env production <"$file" >/dev/null \
    || fail "could not set RESTORE_POINT_PASSPHRASE in the production environment (an admin, gh logged in)"
echo "restore-point-key: RESTORE_POINT_PASSPHRASE set in the production environment"
echo "restore-point-key: keep a copy of $file in your password manager: without it no restore point can be decrypted"
