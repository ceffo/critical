#!/usr/bin/env bash
# Fetches a promotion's restore point (ADR 0024 steps 9.2 and 9.3; docs/deploy.md, "Rollback"): finds the
# artifact of the Promote run that took it, downloads it, decrypts it and unpacks it as `just db-backup`
# lays a backup out, so the drill and the restore of "Backups" take it as they are:
#
#   .backups/<restore point>/critical_prod.dump   pg_dump custom format, --no-owner --no-acl
#   .backups/<restore point>/counts.txt           "<table> <rows>", taken right after the dump
#
#   scripts/restore-point-fetch.sh <X.Y.Z | vX.Y.Z | restore point name>
#
# A version names the restore point taken before that version was promoted, the data as it was before it
# (critical_prod_pre_vX_Y_Z_<UTC time>); the newest one when it was promoted more than once. Artifacts are kept
# 90 days: an expired one is not found. The passphrase is RESTORE_POINT_PASSPHRASE, else the file
# ${CRITICAL_PASSWORD_DIR:-~/.config/critical}/restore-point.passphrase (`just restore-point-key`); it reaches
# gpg through a file descriptor only. BACKUP_DIR moves .backups/ (a folder inside the repository that git does
# not ignore is refused). The tar must hold exactly the dump and counts.txt, and the dump must read back with
# `pg_restore --list`; a failed run keeps nothing. Needs gh (actions: read), gpg, tar and pg_restore 17 or
# newer. Prints names and sizes, never data or the passphrase.
set -euo pipefail
umask 077

usage="usage: restore-point-fetch.sh <X.Y.Z | vX.Y.Z | restore point name>"
fail() { echo "restore-point-fetch: FAIL: $*" >&2; exit 1; }
want="${1:?$usage}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
source "$here/prod-db.sh"
db="$prod_db"

if [[ "$want" =~ ^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
    prefix="${db}_pre_v${BASH_REMATCH[1]}_${BASH_REMATCH[2]}_${BASH_REMATCH[3]}_"
    exact=""
elif [[ "$want" =~ ^${db}_pre_v[0-9]+_[0-9]+_[0-9]+_[0-9]{14}$ ]]; then
    prefix="" exact="$want"
else
    fail "'$want' is not X.Y.Z, vX.Y.Z or a restore point name (${db}_pre_vX_Y_Z_<yyyymmddhhmmss>); $usage"
fi

passphrase="${RESTORE_POINT_PASSPHRASE:-}"
if [ -z "$passphrase" ]; then
    key="${CRITICAL_PASSWORD_DIR:-$HOME/.config/critical}/restore-point.passphrase"
    [ -r "$key" ] && passphrase="$(<"$key")"
fi
[ -n "$passphrase" ] || fail "no passphrase: set RESTORE_POINT_PASSPHRASE, or put it in ${CRITICAL_PASSWORD_DIR:-~/.config/critical}/restore-point.passphrase"

repo="${GH_REPO:-$(gh repo view --json nameWithOwner --jq .nameWithOwner)}"
# Every unexpired artifact of the repository: name, id, run. Restore point names sort by their UTC time.
listing="$(gh api --paginate "repos/$repo/actions/artifacts?per_page=100" \
    --jq '.artifacts[] | select(.expired | not) | "\(.name) \(.id) \(.workflow_run.id)"')" \
    || fail "cannot list the artifacts of $repo (gh needs actions: read)"
found="$(awk -v p="$prefix" -v e="$exact" '(e != "" && $1 == e) || (e == "" && index($1, p) == 1 && $1 ~ /_[0-9]+$/)' <<<"$listing" \
    | sort -k1,1 | tail -n1)"
[ -n "$found" ] || fail "no restore point ${exact:-$prefix*} among the unexpired artifacts of $repo (kept 90 days; docs/deploy.md, \"Rollback\")"
read -r point id run <<<"$found"
echo "restore-point-fetch: $point (artifact $id of run $run)"

out_root="${BACKUP_DIR:-$root/.backups}"
mkdir -p "$out_root"
chmod 700 "$out_root"
dir="$out_root/$point"
[ ! -e "$dir" ] || fail "$dir exists already"
if git -C "$out_root" rev-parse --git-dir >/dev/null 2>&1 && ! git -C "$out_root" check-ignore -q "$dir"; then
    fail "$dir is inside a git repository and not ignored: a dump must never be committed"
fi

work="$(mktemp -d)"
trap '[ ! -d "$work/gnupg" ] || GNUPGHOME="$work/gnupg" gpgconf --kill gpg-agent 2>/dev/null || true; rm -rf "$work"' EXIT
gh run download "$run" -R "$repo" -n "$point" -D "$work/dl" >/dev/null || fail "could not download the artifact $point of run $run"
enc="$work/dl/$point.tar.gpg"
[ -s "$enc" ] || fail "the artifact $point holds no $point.tar.gpg"
mkdir -m 700 "$work/gnupg"
GNUPGHOME="$work/gnupg" gpg --batch --quiet --no-tty --yes --pinentry-mode loopback --no-symkey-cache \
    --passphrase-fd 3 --output "$work/point.tar" --decrypt "$enc" 3<<<"$passphrase" 2>/dev/null \
    || fail "could not decrypt $point (wrong passphrase, or a damaged file)"
members="$(tar -tf "$work/point.tar" | LC_ALL=C sort | tr '\n' ' ')"
[ "$members" = "counts.txt $db.dump " ] || fail "the restore point holds '$members', not $db.dump and counts.txt"
mkdir "$work/point"
tar -C "$work/point" -xf "$work/point.tar" "$db.dump" counts.txt
[ -s "$work/point/counts.txt" ] || fail "the restore point has no table counts"
pg_restore --list "$work/point/$db.dump" >/dev/null || fail "the dump is not readable by pg_restore --list (pg_restore 17 or newer)"
mkdir "$dir"
mv "$work/point/$db.dump" "$work/point/counts.txt" "$dir/"
total="$(awk '{ n += $2 } END { print n + 0 }' "$dir/counts.txt")"
echo "restore-point-fetch: $point -> $dir/$db.dump ($(du -h "$dir/$db.dump" | cut -f1), $(wc -l <"$dir/counts.txt") tables, $total rows)"
echo "restore-point-fetch: private data (ADR 0024 step 7): keep it on this machine. Next: just db-restore-drill $dir/$db.dump"
