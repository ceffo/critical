#!/bin/sh
# Takes the pre-promotion restore point of the production database (ADR 0024 steps 4.2 and 9.2): a copy
# database on the same PostgreSQL cluster, named after the release being promoted, made with pg_dump and
# pg_restore from the database CRITICAL_DSN names, then verified, then the oldest copies beyond the
# retention are dropped. The promotion (scripts/promote.sh) runs it inside an ephemeral `fly console`
# machine of the production app started from the release image (which ships the PostgreSQL 17 client), so
# it has the app's CRITICAL_DSN and the cluster's private network, and the running app is not touched.
#
#   sh restore-point.sh <base>_pre_v<major>_<minor>_<patch>_<UTC yyyymmddhhmmss> <migration>...
#   e.g. critical_prod_pre_v1_2_3_20261017153000 001_baseline.sql 002_spells.sql
#
# The <migration>s are the migration files the release image embeds (its commit's
# api/internal/storage/postgres/migrations/*.sql, listed by scripts/promote.sh). When the database records
# one that is not among them, the release is older than the schema (a rollback across a migration): its
# server would refuse to start (ADR 0024 step 6.2) and the rolling deploy would leave production on one
# machine, so that is refused too, pointing at the runbook (docs/deploy.md, "Rollback", "Across a
# migration").
#
# <base> is the database CRITICAL_DSN connects to. The copy is owned by the connecting user (it needs the
# CREATEDB attribute: docs/deploy.md, "Databases, users and the apps' secrets"), and CONNECT on it is
# revoked from PUBLIC at once, like every database of the cluster. Refused before anything is written:
# a malformed name, a name that exists, a database newer than the release's migrations (above), the server
# running in this machine (its boot would migrate the database before the copy), a missing CREATEDB, and a
# projected size of the database and its copies over RESTORE_POINT_BUDGET_MB (default 300; the cluster's
# 1 GB volume is shared). The budget counts critical's databases only: this user cannot size the other
# app's databases nor see the volume's free space, which the operator watches (docs/deploy.md,
# "Promotion"). RESTORE_POINT_KEEP complete copies
# are kept (default 3). A copy that fails its verification is dropped (its sessions ended first: the server
# may not have noticed yet that a dropped connection is gone). An incomplete copy left by an earlier run
# (no public.schema_migrations: a restore is one transaction, so a failed one leaves an empty database) is
# dropped at the start of the next run and never counts toward the retention or the budget, so it can never
# push a valid copy out. Promotions are serialized (concurrency: deploy-production), so none is mid-restore.
# A copy held by a session this user cannot end (another role's, e.g. a superuser's) is not dropped: it is
# skipped at once with a warning naming the holder, and the run goes on (the operator drops it,
# docs/deploy.md, "Promotion"). A lost connection to the cluster (its proxy blips while the VM is short of
# memory) is retried a few times (RESTORE_POINT_CONNECT_TRIES, default 5), and so is pg_dump; a failed
# statement or restore is not. Prints names, counts and sizes, never data or the DSN; the last line is
# `restore-point: OK <name>` on success.
#
# POSIX sh (the image's shell is busybox). Also run by scripts/image-smoke.sh against a throwaway database.
set -eu

usage="usage: restore-point.sh <base>_pre_v<major>_<minor>_<patch>_<yyyymmddhhmmss> <migration>..."
fail() { echo "restore-point: FAIL: $*" >&2; exit 1; }
name="${1:?$usage}"
shift
[ "$#" -gt 0 ] || fail "no embedded migration given; $usage"
keep="${RESTORE_POINT_KEEP:-3}"
budget_mb="${RESTORE_POINT_BUDGET_MB:-300}"
connect_tries="${RESTORE_POINT_CONNECT_TRIES:-5}" # 2+4+6+8 = 20 s of waiting by default
connect_wait="${RESTORE_POINT_CONNECT_WAIT:-2}"
dsn="${CRITICAL_DSN:?restore-point: CRITICAL_DSN is not set}"
for m in "$@"; do
    echo "$m" | grep -Eqx '[0-9a-z_]+\.sql' || fail "malformed migration name '$m'"
done

case "$keep" in '' | *[!0-9]* | 0) fail "RESTORE_POINT_KEEP must be a positive number" ;; esac
case "$budget_mb" in '' | *[!0-9]*) fail "RESTORE_POINT_BUDGET_MB must be a number" ;; esac
case "$connect_tries" in '' | *[!0-9]* | 0) fail "RESTORE_POINT_CONNECT_TRIES must be a positive number" ;; esac
case "$connect_wait" in '' | *[!0-9]*) fail "RESTORE_POINT_CONNECT_WAIT must be a number" ;; esac
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

# q1 <dsn> <sql>: one psql run. q <dsn> <sql>: the same, run again (a few times, waiting longer each time)
# when the connection itself fails (psql exits 2): the cluster's proxy marks the database down for some
# seconds when its small VM is short of memory, and a promotion must not fail on that. A statement that
# fails (exit 1 or 3) is never run again. Only statements that are safe to repeat go through q.
q1() { psql -X -w -q -A -t -v ON_ERROR_STOP=1 -d "$1" -c "$2"; }
q() {
    _qn=1
    while :; do
        _qs=0
        _qo="$(q1 "$1" "$2")" || _qs=$?
        if [ "$_qs" != 2 ] || [ "$_qn" -ge "$connect_tries" ]; then
            [ -z "$_qo" ] || printf '%s\n' "$_qo"
            return "$_qs"
        fi
        echo "restore-point: the connection to the database failed (try $_qn of $connect_tries); trying again in $((_qn * connect_wait))s" >&2
        sleep $((_qn * connect_wait))
        _qn=$((_qn + 1))
    done
}

# dsn_of <database>: the DSN of another database of the cluster.
dsn_of() { echo "${dsn_path%/*}/$1$dsn_query"; }
# drop_copy <database>: DROP DATABASE after ending this user's own sessions still on it (its restore
# connection may still be open on the server after the client died: DROP fails "being accessed by other
# users" until the server notices). A session of another role (a superuser's, a background process's) cannot
# be ended from here: the copy is left at once, its holders named (kind, role, since when: no query), and
# never retried in a loop that would load a cluster already short of memory. Autovacuum workers are not
# holders: DROP DATABASE ends them itself. A few tries otherwise.
drop_copy() {
    _di=1
    while :; do
        q "$dsn" "SELECT pg_terminate_backend(pid) FROM pg_stat_activity
                  WHERE datname = '$1' AND usename = current_user AND pid <> pg_backend_pid()" >/dev/null 2>&1 || true
        _dh="$(q "$dsn" "SELECT coalesce(string_agg(coalesce(backend_type, 'a session') || ' of ' || coalesce(usename, 'no role')
                             || coalesce(' since ' || to_char(backend_start AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI') || ' UTC', ''),
                             ', '), '')
                         FROM pg_stat_activity WHERE datname = '$1' AND pid <> pg_backend_pid()
                         AND usename IS DISTINCT FROM current_user AND backend_type IS DISTINCT FROM 'autovacuum worker'" 2>/dev/null)" \
            || _dh="unknown (the sessions could not be read)"
        if [ -n "$_dh" ]; then
            echo "restore-point: $1 is held by a session this user cannot end: $_dh" >&2
            return 1
        fi
        q "$dsn" "DROP DATABASE IF EXISTS \"$1\"" >/dev/null 2>&1 && return 0
        [ "$_di" -lt 3 ] || return 1
        _di=$((_di + 1))
        sleep 2
    done
}
# copy_state <database>: complete, incomplete (no recorded migrations table: a failed restore) or unknown
# (cannot connect: judged complete, never dropped on a guess).
copy_state() {
    case "$(q "$(dsn_of "$1")" "SELECT to_regclass('public.schema_migrations') IS NOT NULL" 2>/dev/null)" in
        t) echo complete ;;
        f) echo incomplete ;;
        *) echo unknown ;;
    esac
}

[ "$(q "$dsn" 'SELECT current_database()')" = "$base" ] || fail "CRITICAL_DSN does not connect to '$base'"
[ "$(q "$dsn" 'SELECT rolcreatedb FROM pg_roles WHERE rolname = current_user')" = t ] \
    || fail "the database user lacks CREATEDB (docs/deploy.md: rerun the provisioning for prod)"
[ "$(q "$dsn" "SELECT count(*) FROM pg_database WHERE datname = '$name'")" = 0 ] || fail "'$name' exists already"
# The names are checked above ([0-9a-z_]+.sql), so they are safe inside the SQL literal.
embedded="$(printf "'%s'," "$@")"
unknown="$(q "$dsn" "SELECT coalesce(string_agg(name, ' ' ORDER BY name), '') FROM schema_migrations
                     WHERE name NOT IN (${embedded%,})")"
[ -z "$unknown" ] || fail "$base records migrations this release does not embed ($unknown): it is older than \
the schema and would not start on it. Roll back across a migration as docs/deploy.md says (\"Rollback\", \"Across a migration\")"

# Incomplete copies of earlier failed runs go first (those that cannot be dropped are not tried again below): they hold no data, and must not count below.
skipped=""
for old in $(q "$dsn" "SELECT datname FROM pg_database WHERE datname ~ '$pattern'"); do
    if [ "$(copy_state "$old")" = incomplete ]; then
        if drop_copy "$old"; then
            echo "restore-point: dropped the incomplete copy $old (an earlier run failed)"
        else
            skipped="$skipped $old "
            echo "restore-point: WARNING: could not drop the incomplete copy $old; skipped, drop it by hand (docs/deploy.md, \"Promotion\")" >&2
        fi
    fi
done

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
        drop_copy "$name" \
            && echo "restore-point: dropped the incomplete copy $name" >&2 \
            || echo "restore-point: could not drop the incomplete copy $name: drop it by hand" >&2
    fi
    exit "$status"
}
trap cleanup EXIT

echo "restore-point: dumping $base"
# pg_dump only reads: a dump cut by a lost connection is taken again, twice at most.
dumped=0
for try in 1 2 3; do
    if pg_dump -w -Fc -f "$work/dump" -d "$dsn"; then dumped=1; break; fi
    [ "$try" = 3 ] || { echo "restore-point: pg_dump of $base failed (try $try of 3); trying again in $((try * 5))s" >&2; sleep $((try * 5)); }
done
[ "$dumped" = 1 ] || fail "pg_dump of $base failed"
[ -s "$work/dump" ] || fail "the dump of $base is empty"
entries="$(pg_restore --list "$work/dump" | grep -c ' TABLE DATA ' || true)"
[ "$entries" -gt 0 ] || fail "the dump of $base holds no table data"

echo "restore-point: creating $name"
# Not repeated (q1): a CREATE whose connection dropped may have run. created is set first, so a failure drops
# whatever it made (the name did not exist above).
created=1
q1 "$dsn" "CREATE DATABASE \"$name\" TEMPLATE template0" >/dev/null || fail "creating $name failed"
q "$dsn" "REVOKE ALL ON DATABASE \"$name\" FROM PUBLIC" >/dev/null
# pg_restore's error text can quote row values (a failed COPY line, a key): the log stays in this ephemeral
# machine. Printed: the TOC entry it names, and its error lines (its own class: the server's message, or the
# lost connection) with every quoted string blanked, since the server quotes values in double quotes; the
# DETAIL/CONTEXT lines (where "Key (id)=(5)" and the COPY line appear) are never printed.
if ! pg_restore -w --exit-on-error --single-transaction --no-owner --no-acl -d "$copy_dsn" "$work/dump" 2>"$work/restore.log"; then
    grep -E '^pg_restore: (from TOC entry|error:)' "$work/restore.log" | sed -e 's/"[^"]*"/"..."/g' -e "s/'[^']*'/'...'/g" | head -n 10 >&2 || true
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

# Retention: the newest $keep complete copies stay (an incomplete one is not counted: it is dropped, or
# reported when it cannot be). A copy that cannot be dropped (in use, or not this user's) is
# reported and kept; the budget above stops the next promotion before the volume fills.
seen=0
for old in $(q "$dsn" "SELECT datname FROM pg_database WHERE datname ~ '$pattern'
                       ORDER BY substring(datname FROM '[0-9]{14}\$') DESC"); do
    case "$skipped" in *" $old "*) continue ;; esac
    if [ "$(copy_state "$old")" = incomplete ]; then
        drop_copy "$old" && echo "restore-point: dropped the incomplete copy $old" \
            || echo "restore-point: WARNING: could not drop the incomplete copy $old; drop it by hand (docs/deploy.md)" >&2
        continue
    fi
    seen=$((seen + 1))
    [ "$seen" -gt "$keep" ] || continue
    if drop_copy "$old"; then
        echo "restore-point: dropped $old (keeping the newest $keep)"
    else
        echo "restore-point: WARNING: could not drop $old; drop it by hand (docs/deploy.md)" >&2
    fi
done
echo "restore-point: copies of $base: $(q "$dsn" "SELECT string_agg(datname, ' ' ORDER BY substring(datname FROM '[0-9]{14}\$') DESC) FROM pg_database WHERE datname ~ '$pattern'")"
echo "restore-point: OK $name"
