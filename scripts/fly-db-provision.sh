#!/usr/bin/env bash
# Provisions critical's databases on the fly PostgreSQL cluster (ADR 0024 steps 2.3, 2.4 and 9.1):
#
#   scripts/fly-db-provision.sh <cluster app> <env>...      e.g.  tournesol-db staging prod
#
# For each <env> (staging | prod): a login user critical_<env> owning the database critical_<env>,
# CONNECT on it for that user only, then the fly secret CRITICAL_DSN on the app critical-<env>
# (private .flycast address, staged: no machine restarts). Every other database of the cluster keeps
# CONNECT for its current login users and loses it for PUBLIC, so a critical user reaches only its own
# database. The cluster's volumes get the snapshot retention of step 9.1.
#
# Checks, through `fly proxy` to the cluster's .flycast address (the same route the apps use): each
# user connects to its own database and may create in its public schema, and every connection to any
# other database of the cluster is refused. The secret is set only after the checks pass.
#
# Rerunnable: a run creates what is missing and gives each named <env> a new password (the old one
# stops working at once, so redeploy or restart that app after a rerun). Passwords are generated here,
# reach the server only as a SCRAM-SHA-256 verifier, and reach fly only through stdin; nothing secret
# is printed. Operator only: needs fly credentials (`fly ssh`) on the cluster.
set -euo pipefail

usage="usage: fly-db-provision.sh <cluster app> <staging|prod>..."
cluster="${1:?$usage}"
shift
[ "$#" -ge 1 ] || { echo "$usage" >&2; exit 2; }
for env in "$@"; do
    case "$env" in staging | prod) ;; *) echo "fly-db-provision: unknown env '$env'; $usage" >&2; exit 2 ;; esac
done
fly="${FLY:-$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)}"
retention_days=30
for tool in jq psql python3 openssl; do
    command -v "$tool" >/dev/null || { echo "fly-db-provision: $tool is required" >&2; exit 1; }
done

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

declare -A password
for env in "$@"; do
    role="critical_$env"
    password[$env]="$(openssl rand -hex 32)"
    verifier="$(printf '%s' "${password[$env]}" | scram_verifier)"
    echo "== $role: user and database"
    admin_sql <<SQL
\set role '$role'
\set verifier '$verifier'
SELECT format('CREATE ROLE %I LOGIN', :'role') WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'role') \gexec
ALTER ROLE :"role" WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD :'verifier';
SELECT format('CREATE DATABASE %I OWNER %I', :'role', :'role') WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = :'role') \gexec
ALTER DATABASE :"role" OWNER TO :"role";
REVOKE ALL ON DATABASE :"role" FROM PUBLIC;
GRANT CONNECT, TEMPORARY ON DATABASE :"role" TO :"role";
SQL
done

echo "== other databases: CONNECT kept for their current login users, revoked from PUBLIC"
admin_sql <<'SQL'
DO $$
DECLARE d record; r record;
BEGIN
  FOR d IN SELECT datname FROM pg_database
           WHERE NOT datistemplate AND datname !~ '^critical_' AND has_database_privilege('public', datname, 'CONNECT') LOOP
    FOR r IN SELECT rolname FROM pg_roles WHERE rolcanlogin AND rolname !~ '^critical_' LOOP
      EXECUTE format('GRANT CONNECT ON DATABASE %I TO %I', d.datname, r.rolname);
    END LOOP;
    EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM PUBLIC', d.datname);
    RAISE NOTICE 'locked database %', d.datname;
  END LOOP;
END $$;
SELECT 'database ' || datname || ': ' || coalesce(array_to_string(datacl, ' '), '(default: PUBLIC may connect)')
FROM pg_database WHERE NOT datistemplate ORDER BY datname \g (format=unaligned tuples_only)
SQL

echo "== checks through fly proxy to $cluster.flycast"
port="$(python3 -I -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
log="$(mktemp)"
"$fly" proxy "$port:5432" "$cluster.flycast" -a "$cluster" -q >"$log" 2>&1 &
proxy=$!
trap 'kill "$proxy" 2>/dev/null || true; rm -f "$log"' EXIT
fail() { echo "fly-db-provision: FAIL: $*" >&2; exit 1; }
conn() { printf 'host=127.0.0.1 port=%s user=%s dbname=%s sslmode=prefer connect_timeout=10' "$port" "$1" "$2"; }

for env in "$@"; do
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
        "SELECT datname FROM pg_database WHERE NOT datistemplate AND datname <> current_database() ORDER BY 1" \
        "$(conn "$role" "$role")")"
    for db in $others; do
        if err="$(PGPASSWORD="${password[$env]}" psql -w -X -A -t -c 'SELECT 1' "$(conn "$role" "$db")" 2>&1)"; then
            fail "$role can connect to $db"
        fi
        case "$err" in *"permission denied for database"*) echo "ok: $role -> $db refused (permission denied)" ;;
            *) fail "$role -> $db failed for another reason: $err" ;; esac
    done
done

for env in "$@"; do
    app="critical-$env"
    role="critical_$env"
    echo "== $app: secret CRITICAL_DSN (staged)"
    printf 'CRITICAL_DSN=postgres://%s:%s@%s.flycast:5432/%s?sslmode=prefer\n' \
        "$role" "${password[$env]}" "$cluster" "$role" | "$fly" secrets import -a "$app" --stage >/dev/null
    "$fly" secrets list -a "$app"
done

echo "== $cluster: volume snapshot retention $retention_days days"
for vol in $("$fly" volumes list -a "$cluster" --json | jq -r '.[].id'); do
    "$fly" volumes update "$vol" -a "$cluster" --snapshot-retention "$retention_days" >/dev/null
done
"$fly" volumes list -a "$cluster" --json | jq -r '.[] | "\(.id) \(.name) \(.size_gb)GB \(.region) snapshot_retention=\(.snapshot_retention)"'
echo "fly-db-provision: OK ($cluster: $*)"
