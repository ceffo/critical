#!/usr/bin/env bash
# Smoke-checks a production image (ADR 0024 step 1) built with the fixture catalogue: runs it on the
# fixture seeds and checks the build identity, the web app, the SPA fallback and the JSON 404.
#
#   scripts/image-smoke.sh <image> <commit the image was built from>
#
# The container gets a unique name and is removed on exit; nothing else is touched.
set -euo pipefail

image="${1:?usage: image-smoke.sh <image> <commit>}"
commit="${2:?usage: image-smoke.sh <image> <commit>}"
root="$(cd "$(dirname "$0")/.." && pwd)"
name="critical-smoke-$$-$RANDOM"

fail() { echo "image-smoke: FAIL: $*" >&2; docker logs "$name" >&2 2>&1 || true; exit 1; }
trap 'docker rm -f "$name" >/dev/null 2>&1 || true' EXIT

# The fixture catalogue only knows the fixture's spells, so the campaign seeds baked into the image
# are swapped for the fixture seeds.
docker run -d --name "$name" -p 127.0.0.1::8080 \
    -v "$root/api/testdata/seeds:/fixture-seeds:ro" -e CRITICAL_SEEDS_DIR=/fixture-seeds \
    "$image" >/dev/null
port="$(docker port "$name" 8080/tcp | head -n1 | sed 's/.*://')"
base="http://127.0.0.1:$port"

for _ in $(seq 1 60); do
    curl -fsS "$base/api/health" >/dev/null 2>&1 && break
    [ "$(docker inspect -f '{{.State.Running}}' "$name")" = true ] || fail "container exited"
    sleep 0.5
done

# get <path>: prints "<status> <content-type>" and leaves the body in $body_file.
body_file="$(mktemp)"
trap 'rm -f "$body_file"; docker rm -f "$name" >/dev/null 2>&1 || true' EXIT
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
