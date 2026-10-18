#!/usr/bin/env bash
# Takes a pg_dump of one environment's database of the fly PostgreSQL cluster (ADR 0024 steps 7 and 9;
# docs/deploy.md, "Backups"), through `fly proxy`, into a new timestamped folder of the gitignored
# .backups/ (BACKUP_DIR moves it; a path inside the repository that git does not ignore is refused):
#
#   .backups/<env>-<UTC yyyymmddThhmmssZ>/critical_<env>.dump   pg_dump custom format, --no-owner --no-acl
#   .backups/<env>-<UTC ...>/counts.txt                          "<table> <count>" taken right after the dump
#
#   scripts/db-backup.sh <staging|prod>
#
# A dump holds the catalogue (book text): it stays on this machine, mode 700/600, and is never committed,
# attached to a workflow or printed (only the promotion's restore point is, encrypted: scripts/promote.sh). The database user is critical_<env> and its password comes from the
# operator's environment: PGPASSWORD, else the file $CRITICAL_PASSWORD_DIR/critical_<env>.password (default
# ~/.config/critical, where `just fly-db-provision --password-dir` writes it). It reaches pg_dump through the
# environment only: never an argument, never printed. pg_dump must be at least the server's version (17).
# The cluster is tournesol-db (CRITICAL_DB_CLUSTER); CRITICAL_DB_ENDPOINT=<host>:<port> uses an already
# reachable server instead of starting `fly proxy` (an open `just fly-db-proxy`, or a local one). The dump is
# read back with `pg_restore --list` before the folder is kept; a failed run leaves no folder. Prints the
# folder, the size and the table counts total.
set -euo pipefail
umask 077

usage="usage: db-backup.sh <staging|prod>"
fail() { echo "db-backup: FAIL: $*" >&2; exit 1; }
env="${1:?$usage}"
case "$env" in staging | prod) ;; *) echo "db-backup: unknown environment '$env'; $usage" >&2; exit 2 ;; esac

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(dirname "$here")"
cluster="${CRITICAL_DB_CLUSTER:-tournesol-db}"
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
db="critical_$env"
user="$db"

password="${PGPASSWORD:-}"
if [ -z "$password" ]; then
    file="${CRITICAL_PASSWORD_DIR:-$HOME/.config/critical}/$user.password"
    [ -r "$file" ] && password="$(<"$file")"
fi
[ -n "$password" ] || fail "no password for $user: set PGPASSWORD, or put it in ${CRITICAL_PASSWORD_DIR:-~/.config/critical}/$user.password (just fly-db-provision --password-dir)"
export PGPASSWORD="$password"
export PGSSLMODE=disable # the cluster has no TLS; the traffic stays in fly's private network (docs/deploy.md)
unset password

out_root="${BACKUP_DIR:-$root/.backups}"
mkdir -p "$out_root"
chmod 700 "$out_root"
dir="$out_root/$env-$(date -u +%Y%m%dT%H%M%SZ)"
if git -C "$out_root" rev-parse --git-dir >/dev/null 2>&1 && ! git -C "$out_root" check-ignore -q "$dir"; then
    fail "$dir is inside a git repository and not ignored: a dump must never be committed"
fi
mkdir "$dir"

proxy=""
cleanup() {
    status=$?
    [ -z "$proxy" ] || { kill "$proxy" 2>/dev/null || true; wait "$proxy" 2>/dev/null || true; }
    [ "$status" = 0 ] || { rm -rf "$dir"; echo "db-backup: no backup kept" >&2; }
    rm -f "${log:-}"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

taken() { [ -n "$(ss -H -ltn "sport = :$1")" ]; }
if [ -n "${CRITICAL_DB_ENDPOINT:-}" ]; then
    host="${CRITICAL_DB_ENDPOINT%:*}"
    port="${CRITICAL_DB_ENDPOINT##*:}"
else
    host=127.0.0.1
    port="$(python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
    log="$(mktemp)"
    "$fly" proxy "$port:5432" "$cluster.flycast" -a "$cluster" -q >"$log" 2>&1 &
    proxy=$!
    for _ in $(seq 1 90); do
        kill -0 "$proxy" 2>/dev/null || { cat "$log" >&2; fail "fly proxy exited"; }
        taken "$port" && break
        sleep 1
    done
    taken "$port" || fail "fly proxy not listening on local port $port after 90 s"
fi
conn="host=$host port=$port user=$user dbname=$db sslmode=disable"

dump="$dir/$db.dump"
# The cluster's machine may be stopped (scale to zero): the first connection starts it and can wait a while.
pg_dump "$conn" --format=custom --no-owner --no-acl --file="$dump" || fail "pg_dump of $db failed"
pg_restore --list "$dump" >/dev/null || fail "the dump is not readable by pg_restore --list"
"$here/db-table-counts.sh" "$conn" >"$dir/counts.txt" || fail "could not count the tables of $db"
[ -s "$dir/counts.txt" ] || fail "$db has no table: nothing to back up"

total="$(awk '{ n += $2 } END { print n }' "$dir/counts.txt")"
echo "db-backup: $db -> $dir/$db.dump ($(du -h "$dump" | cut -f1), $(wc -l <"$dir/counts.txt") tables, $total rows)"
echo "db-backup: private data (ADR 0024 step 7): keep it on this machine"
