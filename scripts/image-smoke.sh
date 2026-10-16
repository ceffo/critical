#!/usr/bin/env bash
# Smoke-checks a production image (ADR 0024 step 1) built with the fixture catalogue: runs it on the
# fixture seeds and checks the build identity, the web app, the SPA fallback and the JSON 404.
#
#   scripts/image-smoke.sh <image> <commit the image was built from>
#
# The image holds no database (ADR 0025 step 1) and refuses the in-memory storage (step 8.3), so the
# smoke starts its own empty PostgreSQL next to it, on a private network: the boot migrates, seeds and
# imports the fixture catalogue into it. The containers and the network get unique names and are
# removed on exit (the database has no volume); nothing else is touched.
set -euo pipefail

image="${1:?usage: image-smoke.sh <image> <commit>}"
commit="${2:?usage: image-smoke.sh <image> <commit>}"
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
    "$image" >/dev/null
port="$(docker port "$name" 8080/tcp | head -n1 | sed 's/.*://')"
base="http://127.0.0.1:$port"

for _ in $(seq 1 120); do
    curl -fsS "$base/api/health" >/dev/null 2>&1 && break
    [ "$(docker inspect -f '{{.State.Running}}' "$name")" = true ] || fail "container exited"
    sleep 0.5
done

# get <path>: prints "<status> <content-type>" and leaves the body in $body_file.
body_file="$(mktemp)"
get() { curl -sS -o "$body_file" -w '%{http_code} %{content_type}' "$base$1"; }

out="$(get /api/health)"
[ "${out%% *}" = 200 ] || fail "/api/health: $out"
grep -q "\"commit\":\"$commit\"" "$body_file" || fail "/api/health does not report commit $commit: $(cat "$body_file")"
grep -q '"status":"ok"' "$body_file" || fail "/api/health status: $(cat "$body_file")"

for path in / /spells/deep/link; do
    out="$(get "$path")"
    [ "${out%% *}" = 200 ] || fail "$path: $out"
    grep -q '<div id="root">' "$body_file" || fail "$path is not index.html"
done

out="$(get /api/nope)"
[ "$out" = "404 application/json" ] || fail "/api/nope: $out"
grep -q '"code":"not_found"' "$body_file" || fail "/api/nope body: $(cat "$body_file")"

uid="$(docker exec "$name" id -u)"
[ "$uid" != 0 ] || fail "the server runs as root"
docker inspect -f '{{json .Config.Healthcheck.Test}}' "$name" | grep -q '/api/health' ||
    fail "the image HEALTHCHECK does not call /api/health"

echo "image-smoke: OK ($image, commit $commit, uid $uid, port $port)"
