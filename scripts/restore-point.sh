#!/bin/sh
# Takes the pre-promotion restore point of the production database (ADR 0024 steps 4.2 and 9.2): a copy
# database on the same PostgreSQL cluster, named after the release being promoted, made with pg_dump and
# pg_restore from the database CRITICAL_DSN names, then verified, then the oldest copies beyond the
# retention are dropped. The promotion (scripts/promote.sh) runs it inside an ephemeral `fly console`
# machine of the production app started from the release image (which ships the PostgreSQL 17 client), so
# it has the app's CRITICAL_DSN and the cluster's private network, and the running app is not touched.
#
#   sh restore-point.sh <base>_pre_v<major>_<minor>_<patch>_<UTC yyyymmddhhmmss>
#   e.g. critical_prod_pre_v1_2_3_20261017153000
#
# <base> is the database CRITICAL_DSN connects to. The copy is owned by the connecting user (it needs the
# CREATEDB attribute: docs/deploy.md, "Databases, users and the apps' secrets"), and CONNECT on it is
# revoked from PUBLIC at once, like every database of the cluster. Refused before anything is written:
# a malformed name, a name that exists, the server running in this machine (its boot would migrate the
# database before the copy), a missing CREATEDB, and a projected size of the database and its copies over
# RESTORE_POINT_BUDGET_MB (default 300; the cluster's 1 GB volume is shared). RESTORE_POINT_KEEP copies
# are kept (default 3). A copy that fails its verification is dropped. Prints names, counts and sizes,
# never data or the DSN; the last line is `restore-point: OK <name>` on success.
#
# POSIX sh (the image's shell is busybox). Also run by scripts/image-smoke.sh against a throwaway database.
set -eu

name="${1:?usage: restore-point.sh <base>_pre_v<major>_<minor>_<patch>_<yyyymmddhhmmss>}"
keep="${RESTORE_POINT_KEEP:-3}"
budget_mb="${RESTORE_POINT_BUDGET_MB:-300}"
dsn="${CRITICAL_DSN:?restore-point: CRITICAL_DSN is not set}"
fail() { echo "restore-point: FAIL: $*" >&2; exit 1; }

case "$keep" in '' | *[!0-9]* | 0) fail "RESTORE_POINT_KEEP must be a positive number" ;; esac
case "$budget_mb" in '' | *[!0-9]*) fail "RESTORE_POINT_BUDGET_MB must be a number" ;; esac
echo "$name" | grep -Eqx '[a-z][a-z0-9_]*_pre_v[0-9]+_[0-9]+_[0-9]+_[0-9]{14}' || fail "malformed restore point name '$name'"
[ "${#name}" -le 63 ] || fail "restore point name longer than 63 characters"
for tool in psql pg_dump pg_restore; do
    command -v "$tool" >/dev/null || fail "$tool is missing (the image ships postgresql17-client)"
done
# fly console starts its machine without the image's entrypoint; if the server ran here, its boot would
# already have migrated the database this copy must hold as it was.
! pidof critical-srv >/dev/null 2>&1 || fail "the server runs in this machine: the copy would hold a migrated database"

# The copy's DSN: the same one with the database name replaced (postgres://user:pass@host:port/<db>?query).
dsn_path="${dsn%%\?*}"
dsn_query="${dsn#"$dsn_path"}"
base="${dsn_path##*/}"
copy_dsn="${dsn_path%/*}/$name$dsn_query"
case "$name" in "${base}_pre_"*) ;; *) fail "'$name' is not a restore point of the database '$base'" ;; esac
pattern="^${base}_pre_v[0-9]+_[0-9]+_[0-9]+_[0-9]{14}\$"

q() { psql -X -w -q -A -t -v ON_ERROR_STOP=1 -d "$1" -c "$2"; }

[ "$(q "$dsn" 'SELECT current_database()')" = "$base" ] || fail "CRITICAL_DSN does not connect to '$base'"
[ "$(q "$dsn" 'SELECT rolcreatedb FROM pg_roles WHERE rolname = current_user')" = t ] \
    || fail "the database user lacks CREATEDB (docs/deploy.md: rerun the provisioning for prod)"
[ "$(q "$dsn" "SELECT count(*) FROM pg_database WHERE datname = '$name'")" = 0 ] || fail "'$name' exists already"

# Projected use: the database, the copies kept after this run (the newest keep-1), and the new copy.
projected="$(q "$dsn" "
    WITH kept AS (
        SELECT datname FROM pg_database WHERE datname ~ '$pattern'
        ORDER BY substring(datname FROM '[0-9]{14}\$') DESC LIMIT $keep - 1)
    SELECT (2 * pg_database_size(current_database())
            + coalesce((SELECT sum(pg_database_size(datname)) FROM kept), 0))::bigint / (1024 * 1024)")"
[ "$projected" -le "$budget_mb" ] || fail "the database and its copies would use $projected MB, over the $budget_mb MB budget"

work="$(mktemp -d)"
created=""
cleanup() {
    status=$?
    rm -rf "$work"
    if [ "$status" != 0 ] && [ -n "$created" ]; then
        q "$dsn" "DROP DATABASE IF EXISTS \"$name\"" >/dev/null 2>&1 \
            && echo "restore-point: dropped the incomplete copy $name" >&2 \
            || echo "restore-point: could not drop the incomplete copy $name: drop it by hand" >&2
    fi
    exit "$status"
}
trap cleanup EXIT

echo "restore-point: dumping $base"
pg_dump -w -Fc -f "$work/dump" -d "$dsn" || fail "pg_dump of $base failed"
[ -s "$work/dump" ] || fail "the dump of $base is empty"
entries="$(pg_restore --list "$work/dump" | grep -c ' TABLE DATA ' || true)"
[ "$entries" -gt 0 ] || fail "the dump of $base holds no table data"

echo "restore-point: creating $name"
q "$dsn" "CREATE DATABASE \"$name\" TEMPLATE template0" >/dev/null
created=1
q "$dsn" "REVOKE ALL ON DATABASE \"$name\" FROM PUBLIC" >/dev/null
# pg_restore's error text can quote row values (a failed COPY line, a key): only the object it names is
# printed, the log stays in this ephemeral machine.
if ! pg_restore -w --exit-on-error --single-transaction --no-owner --no-acl -d "$copy_dsn" "$work/dump" 2>"$work/restore.log"; then
    grep -E '^pg_restore: from TOC entry' "$work/restore.log" >&2 || true
    fail "restoring the dump into $name failed"
fi

# Verified: the same tables and the same recorded migrations as the dump's source, and rows in it.
tables="SELECT count(*) || ' ' || md5(coalesce(string_agg(tablename, ',' ORDER BY tablename), '')) FROM pg_tables WHERE schemaname = 'public'"
migrations="SELECT count(*) || ' ' || md5(coalesce(string_agg(name, ',' ORDER BY name), '')) FROM schema_migrations"
rows="SELECT coalesce(sum((xpath('/row/c/text()', query_to_xml(format('SELECT count(*) AS c FROM public.%I', tablename), false, true, '')))[1]::text::bigint), 0) FROM pg_tables WHERE schemaname = 'public'"
[ "$(q "$copy_dsn" "$tables")" = "$(q "$dsn" "$tables")" ] || fail "$name does not hold the tables of $base"
[ "$(q "$copy_dsn" "$migrations")" = "$(q "$dsn" "$migrations")" ] || fail "$name does not hold the migrations of $base"
copy_rows="$(q "$copy_dsn" "$rows")"
[ "$copy_rows" -gt 0 ] || fail "$name holds no rows"
[ "$(q "$dsn" "SELECT has_database_privilege('public', '$name', 'CONNECT')")" = f ] || fail "PUBLIC may connect to $name"
summary="$(q "$copy_dsn" "SELECT (SELECT count(*) FROM pg_tables WHERE schemaname = 'public') || ' tables, $copy_rows rows, '
    || (SELECT count(*) FROM schema_migrations) || ' migrations, ' || pg_size_pretty(pg_database_size(current_database()))")"
echo "restore-point: $name verified ($summary)"
created=""

# Retention: the newest $keep copies stay. A copy that cannot be dropped (in use, or not this user's) is
# reported and kept; the budget above stops the next promotion before the volume fills.
for old in $(q "$dsn" "SELECT datname FROM pg_database WHERE datname ~ '$pattern'
                       ORDER BY substring(datname FROM '[0-9]{14}\$') DESC OFFSET $keep"); do
    if q "$dsn" "DROP DATABASE \"$old\"" >/dev/null 2>&1; then
        echo "restore-point: dropped $old (keeping the newest $keep)"
    else
        echo "restore-point: WARNING: could not drop $old; drop it by hand (docs/deploy.md)" >&2
    fi
done
echo "restore-point: copies of $base: $(q "$dsn" "SELECT string_agg(datname, ' ' ORDER BY substring(datname FROM '[0-9]{14}\$') DESC) FROM pg_database WHERE datname ~ '$pattern'")"
echo "restore-point: OK $name"
