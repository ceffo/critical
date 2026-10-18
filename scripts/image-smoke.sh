#!/usr/bin/env bash
# Smoke-checks a production image (ADR 0024 step 1) built with the fixture catalogue: runs it on the
# fixture seeds and checks the build identity, the web app, the SPA fallback and the JSON 404.
#
#   scripts/image-smoke.sh <image> <commit the image was built from> <version it was built as>
#
# The image holds no database (ADR 0025 step 1) and refuses the in-memory storage (step 8.3), so the
# smoke starts its own empty PostgreSQL next to it, on a private network: the boot migrates, seeds and
# imports the fixture catalogue into it. Then it takes restore points of that database with the image's
# own PostgreSQL client and shell (scripts/restore-point.sh, as the promotion does in a `fly console` machine
# of the release image, ADR 0024 steps 4.2 and 9.2): the dump comes back whole (checksum, size, the dump and
# its counts), restores into a scratch database with the same row counts, no database is created by it, a
# lost connection is retried, and the refusals. The containers and the network get unique names and are
# removed on exit (the database has no volume); nothing else is touched.
set -euo pipefail

usage="usage: image-smoke.sh <image> <commit> <version>"
image="${1:?$usage}"
commit="${2:?$usage}"
version="${3:?$usage}"
root="$(cd "$(dirname "$0")/.." && pwd)"
name="critical-smoke-$$-$RANDOM"
db="$name-db"
pg_image="postgres:17-alpine" # the docker compose service's image

body_file=""
cleanup() {
    [ -z "$body_file" ] || rm -f "$body_file"
    docker rm -f "$name" "$db" >/dev/null 2>&1 || true
    docker network rm "$name" >/dev/null 2>&1 || true
}
trap cleanup EXIT
fail() { echo "image-smoke: FAIL: $*" >&2; docker logs "$name" >&2 2>&1 || true; exit 1; }

docker network create "$name" >/dev/null
docker run -d --name "$db" --network "$name" \
    -e POSTGRES_USER=critical -e POSTGRES_PASSWORD=critical -e POSTGRES_DB=critical \
    "$pg_image" >/dev/null
for _ in $(seq 1 60); do
    # -h forces TCP: the entrypoint's init phase only listens on the socket.
    docker exec "$db" pg_isready -h 127.0.0.1 -U critical -d critical >/dev/null 2>&1 && break
    sleep 0.5
done
docker exec "$db" pg_isready -h 127.0.0.1 -U critical -d critical >/dev/null ||
    { docker logs "$db" >&2 2>&1; echo "image-smoke: FAIL: the database did not start" >&2; exit 1; }

# The fixture catalogue only knows the fixture's spells, so the campaign seeds baked into the image
# are swapped for the fixture seeds.
docker run -d --name "$name" --network "$name" -p 127.0.0.1::8080 \
    -e CRITICAL_DSN="postgres://critical:critical@$db:5432/critical?sslmode=disable" \
    -v "$root/api/testdata/seeds:/fixture-seeds:ro" -e CRITICAL_SEEDS_DIR=/fixture-seeds \
    -e CRITICAL_ENV=dev \
    "$image" >/dev/null
port="$(docker port "$name" 8080/tcp | head -n1 | sed 's/.*://')"
base="http://127.0.0.1:$port"

for _ in $(seq 1 120); do
    curl -fsS "$base/api/health" >/dev/null 2>&1 && break
    [ "$(docker inspect -f '{{.State.Running}}' "$name")" = true ] || fail "container exited"
    sleep 0.5
done

# get <path> [curl args]: prints "<status> <content-type>" and leaves the body in $body_file.
body_file="$(mktemp)"
# Extra arguments go to curl (headers).
get() {
    local path="$1"
    shift
    curl -sS -o "$body_file" -w '%{http_code} %{content_type}' "$@" "$base$path"
}

out="$(get /api/health)"
[ "${out%% *}" = 200 ] || fail "/api/health: $out"
grep -q "\"commit\":\"$commit\"" "$body_file" || fail "/api/health does not report commit $commit: $(cat "$body_file")"
grep -q "\"version\":\"$version\"" "$body_file" || fail "/api/health does not report version $version: $(cat "$body_file")"
grep -q '"status":"ok"' "$body_file" || fail "/api/health status: $(cat "$body_file")"

# The login gate (ADR 0029 step 6): without a session a page goes to the login page, which is served,
# and a data route, the catalogue and an asset of the bundle answer 401. The image runs without OAuth
# settings, outside fly and with CRITICAL_ENV=dev, so it is in dev login mode (step 8.1).
for path in / /spells/deep/link; do
    out="$(get "$path" -H "Accept: text/html")"
    [ "${out%% *}" = 302 ] || fail "$path without a session: $out (want a redirect to /login)"
done
out="$(get /login -H "Accept: text/html")"
[ "${out%% *}" = 200 ] || fail "/login: $out"
grep -q '<div id="root">' "$body_file" || fail "/login is not index.html"
asset="$(docker exec "$name" sh -c 'ls /app/web/assets | grep "\.js$" | head -n1')"
for path in /api/character /api/spells/names "/assets/$asset"; do
    out="$(get "$path")"
    [ "${out%% *}" = 401 ] || fail "$path without a session: $out"
done

out="$(curl -sS -o "$body_file" -D - -X POST -H 'Content-Type: application/json' -d '{}' "$base/api/auth/dev")"
# The session cookie is named per app: critical_session_dev_<listen port> in dev login mode.
cookie="$(sed -n 's/^[Ss]et-[Cc]ookie: \(critical_session_dev_8080=[^;]*\).*/\1/p' <<<"$out" | head -n1)"
[ -n "${cookie#*=}" ] || fail "dev login: no critical_session_dev_8080 cookie: $out"

for path in / /spells/deep/link; do
    out="$(get "$path" -H "Cookie: $cookie")"
    [ "${out%% *}" = 200 ] || fail "$path: $out"
    grep -q '<div id="root">' "$body_file" || fail "$path is not index.html"
done
out="$(get /api/character -H "Cookie: $cookie")"
[ "${out%% *}" = 200 ] || fail "/api/character with a session: $out"

out="$(get /api/nope -H "Cookie: $cookie")"
[ "$out" = "404 application/json" ] || fail "/api/nope: $out"
grep -q '"code":"not_found"' "$body_file" || fail "/api/nope body: $(cat "$body_file")"

# Fails closed (ADR 0029 step 8.2): the image with no OAuth settings, outside fly and without
# CRITICAL_ENV=dev refuses to start, before listening and before touching the database.
refuse_status=0
refuse_out="$(timeout 60 docker run --rm --network "$name" \
    -e CRITICAL_DSN="postgres://critical:critical@$db:5432/critical?sslmode=disable" "$image" 2>&1)" || refuse_status=$?
[ "$refuse_status" != 0 ] && [ "$refuse_status" != 124 ] ||
    fail "the image without OAuth settings and without CRITICAL_ENV=dev started (exit $refuse_status): $refuse_out"
[[ "$refuse_out" == *"CRITICAL_ENV=dev"* ]] || fail "the refusal does not name CRITICAL_ENV=dev: $refuse_out"
echo "image-smoke: without OAuth settings nor CRITICAL_ENV=dev the image refuses to start: $refuse_out"

uid="$(docker exec "$name" id -u)"
[ "$uid" != 0 ] || fail "the server runs as root"
docker inspect -f '{{json .Config.Healthcheck.Test}}' "$name" | grep -q '/api/health' ||
    fail "the image HEALTHCHECK does not call /api/health"

# restore_point <container> <name> [env...]: runs scripts/restore-point.sh in the container, as the
# promotion runs it in its console machine; stdout in $rp_out, stderr in $rp_err.
dsn="postgres://critical:critical@$db:5432/critical?sslmode=disable"
# The migrations the image embeds, as scripts/promote.sh lists them (RP_MIGRATIONS overrides them).
migrations="$(cd "$root/api/internal/storage/postgres/migrations" && ls -- *.sql | tr '\n' ' ')"
rp_dir="$(mktemp -d)"
restore_point() {
    local c="$1" n="$2"
    shift 2
    rp_status=0
    # shellcheck disable=SC2086 # one argument per migration
    rp_out="$(docker exec -i -e CRITICAL_DSN="$dsn" "$@" "$c" sh -s "$n" ${RP_MIGRATIONS:-$migrations} \
        <"$root/scripts/restore-point.sh" 2>"$rp_dir/err")" || rp_status=$?
    rp_err="$(cat "$rp_dir/err")"
}
# unpack <name>: the block of $rp_out decoded into $rp_dir/<name>.tar, checked against the OK line, unpacked.
unpack() {
    local ok sum bytes
    ok="$(grep -E "^restore-point: OK $1 [0-9a-f]{64} [0-9]+\$" <<<"$rp_out")" || fail "no OK line for $1: $rp_out $rp_err"
    read -r _ _ _ sum bytes <<<"$ok"
    awk -v b="restore-point: BEGIN $1" -v e="restore-point: END $1" '$0 == e { exit } f { print } $0 == b { f = 1 }' <<<"$rp_out" \
        | base64 -d >"$rp_dir/$1.tar" || fail "the block of $1 does not decode"
    [ "$(wc -c <"$rp_dir/$1.tar" | tr -d ' ')" = "$bytes" ] && [ "$(sha256sum "$rp_dir/$1.tar" | cut -d' ' -f1)" = "$sum" ] ||
        fail "$1 came back damaged (size or checksum)"
    [ "$(tar -tf "$rp_dir/$1.tar" | LC_ALL=C sort | tr '\n' ' ')" = "counts.txt critical.dump " ] || fail "$1 does not hold the dump and its counts"
    mkdir -p "$rp_dir/$1"
    tar -C "$rp_dir/$1" -xf "$rp_dir/$1.tar"
}
sql() { docker exec "$db" psql -X -q -A -t -U critical -d critical -c "$1"; }
databases() { sql "SELECT string_agg(datname, ' ' ORDER BY datname) FROM pg_database"; }
before="$(databases)"
# A machine started from the image without its entrypoint, like the console machine.
docker run -d --name "$name-console" --network "$name" --entrypoint sleep "$image" infinity >/dev/null
trap 'cleanup; docker rm -f "$name-console" >/dev/null 2>&1 || true; rm -rf "$rp_dir"' EXIT
point=critical_pre_v0_0_1_20260101000001
restore_point "$name-console" "$point"
[ "$rp_status" = 0 ] && [ "$(tail -n1 <<<"$rp_out" | cut -d' ' -f1-3)" = "restore-point: OK $point" ] ||
    fail "restore point: exit $rp_status: $rp_out $rp_err"
grep -v '^[A-Za-z0-9+/=]*$' <<<"$rp_out" | grep -v "^restore-point: " && fail "restore point: a line outside the block that is not the script's"
unpack "$point"
grep -E '^restore-point: ' <<<"$rp_out"
[ "$(databases)" = "$before" ] || fail "the restore point created or dropped a database: '$(databases)', was '$before'"
[ "$(wc -l <"$rp_dir/$point/counts.txt")" = "$(sql "SELECT count(*) FROM pg_tables WHERE schemaname = 'public'")" ] &&
    grep -qx "schema_migrations $(sql 'SELECT count(*) FROM schema_migrations')" "$rp_dir/$point/counts.txt" ||
    fail "counts.txt does not count every table: $(cat "$rp_dir/$point/counts.txt")"
# It restores (as the runbook's restore does: --no-owner --no-acl) with the counts it recorded.
sql "CREATE DATABASE smoke_restored TEMPLATE template0" >/dev/null
docker cp "$rp_dir/$point/critical.dump" "$db:/tmp/critical.dump"
docker exec "$db" pg_restore -U critical --no-owner --no-acl --exit-on-error --single-transaction -d smoke_restored /tmp/critical.dump ||
    fail "the restore point does not restore"
restored="$(docker exec -i "$db" psql -X -q -A -t -F ' ' -v ON_ERROR_STOP=1 -U critical -d smoke_restored <<'SQL' | LC_ALL=C sort
SELECT format('SELECT %L, count(*) FROM %I.%I', tablename, schemaname, tablename)
FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename \gexec
SQL
)"
[ "$restored" = "$(cat "$rp_dir/$point/counts.txt")" ] || fail "the restored counts differ from counts.txt: $restored"
sql "DROP DATABASE smoke_restored" >/dev/null
# Refusals, before any dump.
restore_point "$name" critical_pre_v0_0_2_20260101000002
[ "$rp_status" != 0 ] && [[ "$rp_err" == *"the server runs in this machine"* ]] || fail "a dump beside the running server was not refused: $rp_err"
# An older release (a rollback across a migration): the database records a migration it does not embed.
RP_MIGRATIONS=000_older.sql restore_point "$name-console" critical_pre_v0_0_3_20260101000003
[ "$rp_status" != 0 ] && [[ "$rp_err" == *"records migrations this release does not embed (001_baseline.sql"* ]] &&
    [[ "$rp_err" == *"Across a migration"* ]] && [[ "$rp_out" != *BEGIN* ]] || fail "a release older than the schema was not refused: $rp_err"
restore_point "$name-console" critical_staging_pre_v0_0_4_20260101000004
[ "$rp_status" != 0 ] && [[ "$rp_err" == *"is not a restore point of the database 'critical'"* ]] || fail "a name of another database was not refused: $rp_err"
[[ "$rp_out$rp_err" != *critical:critical* ]] || fail "restore-point printed the DSN"
# The cluster unreachable for a few seconds (its proxy marks it down when the VM is short of memory): the
# connection is tried again and the restore point is taken.
docker network disconnect "$name" "$db"
(sleep 3 && docker network connect "$name" "$db") &
reconnect=$!
point=critical_pre_v0_0_5_20260101000005
restore_point "$name-console" "$point"
wait "$reconnect"
[ "$rp_status" = 0 ] && [[ "$rp_err" == *"the connection to the database failed (try 1 of 5)"* ]] ||
    fail "restore point across a lost connection: exit $rp_status: $rp_out $rp_err"
unpack "$point"
echo "image-smoke: restore points OK (the dump back whole, restored with its counts, no database created, refusals, a release older than the schema, a lost connection)"

echo "image-smoke: OK ($image, commit $commit, uid $uid, port $port)"
