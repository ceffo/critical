#!/usr/bin/env bash
# Provisions critical's databases on the fly PostgreSQL cluster (ADR 0024 steps 2.3, 2.4 and 9.1):
#
#   scripts/fly-db-provision.sh [--rotate] [--password-dir DIR] <cluster app> <env>...
#   e.g.  scripts/fly-db-provision.sh tournesol-db staging prod
#
# For each <env> (staging | prod): a login user critical_<env> owning the database critical_<env>, with
# CONNECT on it for that user only. Every other database of the cluster that accepts connections
# (template1 included) keeps CONNECT for the login users it has at run time and loses it for PUBLIC, so
# a critical user reaches only its own database. No critical user has the CREATEDB attribute (a rerun
# removes the one prod had for the old copy-database restore points: the promotion's restore point is now a
# pg_dump, ADR 0024 step 9.2). A user the other app gets LATER needs an explicit
# `GRANT CONNECT ON DATABASE <db> TO <user>` (docs/deploy.md). The cluster's volumes get the snapshot
# retention of step 9.1.
#
# Passwords: a user gets one when it is created, or with --rotate; an existing user's password and its
# app's secret are left alone otherwise, so a rerun changes nothing that runs. When a password is set,
# the app's fly secret CRITICAL_DSN (private .flycast address) is set right after it: staged when the
# app has no machine (applied by its first deploy), applied at once otherwise (fly restarts the
# machines; until each has restarted its new connections fail: a short outage, docs/deploy.md). If the
# run stops between the two, rerun with --rotate. --password-dir DIR also writes each new password to
# DIR/critical_<env>.password (mode 600), for the operator's one-off use (psql or pg_dump through
# `fly proxy`, docs/deploy.md).
#
# Checks: every run checks, as the cluster's operator, that each critical user holds CONNECT on its own
# database and on no other, but for the leftover copies of the old restore points it owns
# (critical_<env>_pre_*) until a superuser drops them (docs/deploy.md, "Promotion"). When this run set a user's password it also connects through `fly proxy` to
# the cluster's .flycast address (the apps' route): own database usable, every other one refused.
#
# Passwords are generated here, reach the server only as a SCRAM-SHA-256 verifier, and reach fly only
# through stdin; nothing secret is printed. Operator only: needs fly credentials (`fly ssh`) on the cluster.
set -euo pipefail

usage="usage: fly-db-provision.sh [--rotate] [--password-dir DIR] <cluster app> <staging|prod>..."
rotate=""
password_dir=""
while [ "$#" -gt 0 ]; do
    case "$1" in
        --rotate) rotate=1; shift ;;
        --password-dir) password_dir="${2:?$usage}"; shift 2 ;;
        -*) echo "$usage" >&2; exit 2 ;;
        *) break ;;
    esac
done
cluster="${1:?$usage}"
shift
[ "$#" -ge 1 ] || { echo "$usage" >&2; exit 2; }
for env in "$@"; do
    case "$env" in staging | prod) ;; *) echo "fly-db-provision: unknown env '$env'; $usage" >&2; exit 2 ;; esac
done
if [ -n "$password_dir" ]; then
    mkdir -p "$password_dir"
    chmod 700 "$password_dir"
fi
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
retention_days=30
for tool in jq psql python3 openssl; do
    command -v "$tool" >/dev/null || { echo "fly-db-provision: $tool is required" >&2; exit 1; }
done
fail() { echo "fly-db-provision: FAIL: $*" >&2; exit 1; }

# admin_sql runs the SQL on stdin as the cluster's operator user, inside the cluster's machine (the
# operator password never leaves it).
admin_sql() {
    local sql
    sql="$(cat)"
    {
        # shellcheck disable=SC2016 # $OPERATOR_PASSWORD expands in the machine's shell, not here
        printf '%s\n' 'PGPASSWORD="$OPERATOR_PASSWORD" exec psql -w -X -q -v ON_ERROR_STOP=1 -h localhost -p 5433 -U postgres -d postgres <<'"'"'CRITICAL_SQL'"'"''
        printf '%s\n' "$sql"
        printf '%s\n' 'CRITICAL_SQL'
    } | "$fly" ssh console -a "$cluster" -C "sh -s" 2>&1 | grep -v '^Connecting to ' || return "${PIPESTATUS[1]}"
}

# scram_verifier prints the SCRAM-SHA-256 verifier of the password on stdin (RFC 7677, PostgreSQL's
# format), so the plain password is never sent to the server nor written to its logs.
scram_verifier() {
    # shellcheck disable=SC2016 # Python source
    python3 -I -c '
import base64, hashlib, hmac, os, sys
pw = sys.stdin.read().encode()
salt, n = os.urandom(16), 4096
salted = hashlib.pbkdf2_hmac("sha256", pw, salt, n)
client = hmac.new(salted, b"Client Key", "sha256").digest()
server = hmac.new(salted, b"Server Key", "sha256").digest()
b = lambda x: base64.b64encode(x).decode()
print(f"SCRAM-SHA-256${n}:{b(salt)}${b(hashlib.sha256(client).digest())}:{b(server)}")
'
}

echo "== cluster $cluster: machines"
"$fly" machine list -a "$cluster" --json | jq -r '.[] | "\(.id) \(.state) \(.region)"'
for id in $("$fly" machine list -a "$cluster" --json | jq -r '.[] | select(.state != "started") | .id'); do
    "$fly" machine start "$id" -a "$cluster"
done
for _ in $(seq 1 30); do
    "$fly" ssh console -a "$cluster" -C "pg_isready -q -h localhost -p 5433" >/dev/null 2>&1 && break
    sleep 2
done

existing="$(admin_sql <<'SQL'
SELECT rolname FROM pg_roles WHERE rolname ~ '^critical_' \g (format=unaligned tuples_only)
SQL
)"

# set_secret sets the app's CRITICAL_DSN from stdin: staged when the app has no machine, applied at once
# (machines restarted) otherwise.
set_secret() {
    local app="$1" machines
    machines="$("$fly" machine list -a "$app" --json | jq length)"
    if [ "$machines" = 0 ]; then
        "$fly" secrets import -a "$app" --stage >/dev/null
        echo "$app: CRITICAL_DSN staged (no machine; the first deploy applies it)"
    else
        "$fly" secrets import -a "$app" >/dev/null
        echo "$app: CRITICAL_DSN applied ($machines machine(s) restarted)"
    fi
}

# app_of <env>: the fly app of an environment, read from its fly config (the databases keep the
# environment's name: critical_staging, critical_prod; the apps are named in deploy/fly.<env>.toml).
app_of() {
    sed -n 's/^app = "\(.*\)"$/\1/p' "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/deploy/fly.$1.toml"
}

declare -A password=()
for env in "$@"; do
    role="critical_$env"
    app="$(app_of "$env")"
    verifier=""
    if grep -qx "$role" <<<"$existing" && [ -z "$rotate" ]; then
        echo "== $role: exists, password and $app's secret unchanged (--rotate to change them)"
    else
        password[$env]="$(openssl rand -hex 32)"
        verifier="$(printf '%s' "${password[$env]}" | scram_verifier)"
        echo "== $role: user and database, new password"
    fi
    admin_sql <<SQL
\set role '$role'
\set verifier '$verifier'
SELECT format('CREATE ROLE %I LOGIN', :'role') WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'role') \gexec
ALTER ROLE :"role" WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
SELECT format('ALTER ROLE %I PASSWORD %L', :'role', :'verifier') WHERE :'verifier' <> '' \gexec
SELECT format('CREATE DATABASE %I OWNER %I', :'role', :'role') WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = :'role') \gexec
ALTER DATABASE :"role" OWNER TO :"role";
REVOKE ALL ON DATABASE :"role" FROM PUBLIC;
GRANT CONNECT, TEMPORARY ON DATABASE :"role" TO :"role";
SQL
    if [ -n "${password[$env]:-}" ]; then
        # sslmode=disable: the cluster's Flycast service does not complete a TLS handshake (it answers
        # the SSL request, then closes), and the app's driver (lib/pq) does not fall back to plain on
        # sslmode=prefer as libpq does. The traffic stays inside fly's WireGuard private network, which
        # encrypts it; `fly postgres attach` writes sslmode=disable too.
        printf 'CRITICAL_DSN=postgres://%s:%s@%s.flycast:5432/%s?sslmode=disable\n' \
            "$role" "${password[$env]}" "$cluster" "$role" | set_secret "$app" \
            || fail "$role has a new password but $app's secret was not set: rerun with --rotate $env"
        if [ -n "$password_dir" ]; then
            (umask 077; printf '%s\n' "${password[$env]}" >"$password_dir/$role.password")
            echo "$role: password written to $password_dir/$role.password (mode 600; delete it after use)"
        fi
    fi
    "$fly" secrets list -a "$app"
done

echo "== other databases: CONNECT kept for their current login users, revoked from PUBLIC"
admin_sql <<'SQL'
DO $$
DECLARE d record; r record;
BEGIN
  FOR d IN SELECT datname FROM pg_database
           WHERE datallowconn AND datname !~ '^critical_' AND has_database_privilege('public', datname, 'CONNECT') LOOP
    FOR r IN SELECT rolname FROM pg_roles WHERE rolcanlogin AND rolname !~ '^critical_' LOOP
      EXECUTE format('GRANT CONNECT ON DATABASE %I TO %I', d.datname, r.rolname);
    END LOOP;
    EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM PUBLIC', d.datname);
    RAISE NOTICE 'locked database %', d.datname;
  END LOOP;
END $$;
SELECT 'database ' || datname || ': ' || coalesce(array_to_string(datacl, ' '), '(default: PUBLIC may connect)')
FROM pg_database WHERE datallowconn ORDER BY datname \g (format=unaligned tuples_only)
SQL

echo "== check: CONNECT privileges of the critical users"
grants="$(admin_sql <<'SQL'
SELECT CASE WHEN has_database_privilege(r.rolname, d.datname, 'CONNECT')
                 = (d.datname = r.rolname OR starts_with(d.datname, r.rolname || '_pre_')) THEN 'ok' ELSE 'BAD' END
       || ': ' || r.rolname || ' -> ' || d.datname || ' '
       || CASE WHEN has_database_privilege(r.rolname, d.datname, 'CONNECT') THEN 'may connect' ELSE 'no CONNECT' END
FROM pg_roles r CROSS JOIN pg_database d
WHERE r.rolname ~ '^critical_' AND d.datallowconn ORDER BY 1 DESC, r.rolname, d.datname \g (format=unaligned tuples_only)
SQL
)"
echo "$grants"
for env in "$@"; do
    grep -q ": critical_$env -> critical_$env may connect" <<<"$grants" || fail "critical_$env cannot connect to its own database"
done
! grep -q '^BAD' <<<"$grants" || fail "a critical user holds CONNECT on a database not its own (nor a leftover restore point copy)"

if [ "${#password[@]}" -eq 0 ]; then
    echo "== connection checks skipped: no password set by this run (--rotate gives one)"
else
    echo "== checks through fly proxy to $cluster.flycast"
    port="$(python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
    log="$(mktemp)"
    "$fly" proxy "$port:5432" "$cluster.flycast" -a "$cluster" -q >"$log" 2>&1 &
    proxy=$!
    trap 'kill "$proxy" 2>/dev/null || true; rm -f "$log"' EXIT
    # The app's sslmode (above): with prefer, psql would fall back to plain and hide a failing TLS.
    conn() { printf 'host=127.0.0.1 port=%s user=%s dbname=%s sslmode=disable connect_timeout=10' "$port" "$1" "$2"; }
    for env in "${!password[@]}"; do
        role="critical_$env"
        ok=""
        for _ in $(seq 1 30); do
            kill -0 "$proxy" 2>/dev/null || { cat "$log" >&2; fail "fly proxy exited"; }
            ok="$(PGPASSWORD="${password[$env]}" psql -w -X -A -t -c \
                "SELECT current_user || ' ' || current_database() || ' ' || has_schema_privilege('public', 'CREATE')" \
                "$(conn "$role" "$role")" 2>/dev/null || true)"
            [ -n "$ok" ] && break
            sleep 2
        done
        [ "$ok" = "$role $role true" ] || fail "$role cannot use its own database (got '$ok')"
        echo "ok: $role connects to $role and may create in its public schema"
        others="$(PGPASSWORD="${password[$env]}" psql -w -X -A -t -c \
            "SELECT datname FROM pg_database WHERE datallowconn AND datname <> current_database() ORDER BY 1" \
            "$(conn "$role" "$role")")"
        for db in $others; do
            case "$db" in "${role}_pre_"*) continue ;; esac # a leftover copy of the old restore points
            if err="$(PGPASSWORD="${password[$env]}" psql -w -X -A -t -c 'SELECT 1' "$(conn "$role" "$db")" 2>&1)"; then
                fail "$role can connect to $db"
            fi
            case "$err" in *"permission denied for database"*) echo "ok: $role -> $db refused (permission denied)" ;;
                *) fail "$role -> $db failed for another reason: $err" ;; esac
        done
    done
fi

echo "== $cluster: volume snapshot retention $retention_days days"
for vol in $("$fly" volumes list -a "$cluster" --json | jq -r '.[].id'); do
    "$fly" volumes update "$vol" -a "$cluster" --snapshot-retention "$retention_days" >/dev/null
done
"$fly" volumes list -a "$cluster" --json | jq -r '.[] | "\(.id) \(.name) \(.size_gb)GB \(.region) snapshot_retention=\(.snapshot_retention)"'
echo "fly-db-provision: OK ($cluster: $*)"
