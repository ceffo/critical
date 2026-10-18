#!/usr/bin/env bash
# Tests, with fly stubbed, the operator scripts that make an app public (part of `just validate`, ADR 0024
# step 8): fly-app-create.sh (the app, its private address, the DSN copied from the old app and never
# printed), fly-google-secrets.sh (the three secrets from the console's JSON, on stdin only, never printed)
# and fly-go-public.sh (free addresses only; released again when the gate check fails).
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-fly-public-setup: FAIL: $*" >&2; fails=$((fails + 1)); }
calls="$tmp/calls"
dsn='postgres://critical_staging:s3cr3t-pw@tournesol-db.flycast:5432/critical_staging?sslmode=disable'

echo "$dsn" > "$tmp/dsn"
stubs="$tmp/bin"
mkdir -p "$stubs"
cat > "$stubs/fly" <<STUB
#!/bin/sh
echo "fly \$*" >> "$calls"
case "\$1 \$2" in
    "apps list") cat "$tmp/apps" ;;
    "ips list") cat "$tmp/ips" ;;
    "ips allocate-v4") jq '. + [{"Type":"shared_v4","Address":"1.2.3.4"}]' "$tmp/ips" > "$tmp/ips.n" && mv "$tmp/ips.n" "$tmp/ips" ;;
    "ips allocate-v6") jq '. + [{"Type":"v6","Address":"2a09::1"}]' "$tmp/ips" > "$tmp/ips.n" && mv "$tmp/ips.n" "$tmp/ips" ;;
    "ips release") jq --arg a "\$3" 'map(select(.Address != \$a))' "$tmp/ips" > "$tmp/ips.n" && mv "$tmp/ips.n" "$tmp/ips" ;;
    "machine list") cat "$tmp/machines" ;;
    "ssh console") cat "$tmp/dsn" ;;
    "secrets import") cat >> "$tmp/stdin" ;;
esac
STUB
cat > "$stubs/gate" <<STUB
#!/bin/sh
echo "gate \$*" >> "$calls"
[ ! -f "$tmp/gate-fails" ]
STUB
chmod +x "$stubs/fly" "$stubs/gate"
run() { : > "$calls"; : > "$tmp/stdin"; status=0; out="$(FLY="$stubs/fly" GATE_CHECK="$stubs/gate" "$@" 2>&1)" || status=$?; }

# --- fly-app-create: a new app, the private address, the DSN copied and not printed ---
echo '[]' > "$tmp/apps"; echo '[]' > "$tmp/ips"
echo '[{"id":"m1","state":"started"}]' > "$tmp/machines"
run "$root/scripts/fly-app-create.sh" staging --dsn-from critical-staging
[ "$status" = 0 ] || fail "app-create: exit $status: $out"
grep -qx "fly apps create critical-hit-staging --org personal" "$calls" || fail "app-create: no app created: $(cat "$calls")"
grep -qx "fly ips allocate-v6 --private -a critical-hit-staging" "$calls" || fail "app-create: no private address"
grep -qx "fly secrets import -a critical-hit-staging --stage" "$calls" || fail "app-create: DSN not staged"
[ "$(cat "$tmp/stdin")" = "CRITICAL_DSN=$dsn" ] || fail "app-create: wrong secret on stdin"
[[ "$out$(cat "$calls")" != *s3cr3t-pw* ]] || fail "app-create: the DSN reached the log or a command line"
grep -qE 'allocate-v4' "$calls" && fail "app-create: allocated a public address"
# An existing app and address are kept.
echo '[{"Name":"critical-hit-staging"}]' > "$tmp/apps"; echo '[{"Type":"private_v6","Address":"fdaa::1"}]' > "$tmp/ips"
run "$root/scripts/fly-app-create.sh" staging
grep -qE 'apps create|allocate' "$calls" && fail "app-create: rerun created again"
[[ "$out" == *"CRITICAL_DSN not set"* ]] || fail "app-create: no --dsn-from should say the secret is not set: $out"
run "$root/scripts/fly-app-create.sh" qa
[ "$status" = 2 ] || fail "app-create: an unknown environment was accepted"
# A DSN that is not a database URL is refused, nothing staged.
echo "not a url" > "$tmp/dsn"
run "$root/scripts/fly-app-create.sh" staging --dsn-from critical-staging
[ "$status" != 0 ] && ! grep -q 'secrets import' "$calls" || fail "app-create: a bad DSN was staged"

# --- fly-google-secrets: from the console's JSON, on stdin only ---
echo '[]' > "$tmp/machines"
cat > "$tmp/client.json" <<'J'
{"web":{"client_id":"123-abc.apps.googleusercontent.com","client_secret":"GOCSPX-topsecret","redirect_uris":[]}}
J
run "$root/scripts/fly-google-secrets.sh" critical-hit-staging "$tmp/client.json"
[ "$status" = 0 ] || fail "google-secrets: exit $status: $out"
grep -qx "fly secrets import -a critical-hit-staging --stage" "$calls" || fail "google-secrets: not staged for an app with no machine: $(cat "$calls")"
[ "$(cat "$tmp/stdin")" = "CRITICAL_GOOGLE_CLIENT_ID=123-abc.apps.googleusercontent.com
CRITICAL_GOOGLE_CLIENT_SECRET=GOCSPX-topsecret
CRITICAL_PUBLIC_URL=https://critical-hit-staging.fly.dev" ] || fail "google-secrets: wrong secrets on stdin: $(cat "$tmp/stdin")"
[[ "$out$(cat "$calls")" != *topsecret* ]] || fail "google-secrets: the secret reached the log or a command line"
[[ "$out" == *"https://critical-hit-staging.fly.dev/api/auth/google/callback"* ]] || fail "google-secrets: no redirect URI printed: $out"
echo '[{"id":"m1"}]' > "$tmp/machines"
run "$root/scripts/fly-google-secrets.sh" critical-hit-staging "$tmp/client.json"
grep -qx "fly secrets import -a critical-hit-staging" "$calls" || fail "google-secrets: not applied at once for a running app"
echo '{"web":{"client_id":"nope","client_secret":"x"}}' > "$tmp/bad.json"
run "$root/scripts/fly-google-secrets.sh" critical-hit-staging "$tmp/bad.json"
[ "$status" != 0 ] && ! grep -q 'secrets import' "$calls" || fail "google-secrets: a bad client id was set"

# --- fly-go-public: free addresses, the gate check, release on failure ---
echo '[{"Type":"private_v6","Address":"fdaa::1"}]' > "$tmp/ips"
run "$root/scripts/fly-go-public.sh" critical-hit-staging
[ "$status" = 0 ] || fail "go-public: exit $status: $out"
grep -qx "fly ips allocate-v4 --shared -a critical-hit-staging" "$calls" || fail "go-public: no shared IPv4"
grep -qx "fly ips allocate-v6 -a critical-hit-staging" "$calls" || fail "go-public: no IPv6"
grep -qx "gate https://critical-hit-staging.fly.dev" "$calls" || fail "go-public: gate check not run on the public URL"
grep -q 'allocate-v4 -a\|allocate-v4$' "$calls" && fail "go-public: allocated a dedicated IPv4"
touch "$tmp/gate-fails"
# A failed gate check right after the script allocated the addresses releases exactly those.
echo '[{"Type":"private_v6","Address":"fdaa::1"}]' > "$tmp/ips"
run "$root/scripts/fly-go-public.sh" critical-hit-staging
[ "$status" != 0 ] || fail "go-public: a failing gate check was accepted after allocating"
grep -qx "fly ips release 1.2.3.4 -a critical-hit-staging" "$calls" && grep -qx "fly ips release 2a09::1 -a critical-hit-staging" "$calls" \
    || fail "go-public: did not release what it allocated: $(cat "$calls")"
[ "$(jq length "$tmp/ips")" = 1 ] || fail "go-public: the app is not private again"
echo '[{"Type":"private_v6","Address":"fdaa::1"},{"Type":"shared_v4","Address":"1.2.3.4"},{"Type":"v6","Address":"2a09::1"}]' > "$tmp/ips"
run "$root/scripts/fly-go-public.sh" critical-hit-staging
# A rerun that fails the gate releases every public address, also those of an earlier run.
[ "$status" != 0 ] || fail "go-public: a failing gate check was accepted"
grep -qx "fly ips release 1.2.3.4 -a critical-hit-staging" "$calls" && grep -qx "fly ips release 2a09::1 -a critical-hit-staging" "$calls" \
    || fail "go-public: a rerun did not release the earlier addresses: $(cat "$calls")"
[ "$(jq length "$tmp/ips")" = 1 ] || fail "go-public: the app is not private again after a rerun"
# A dedicated IPv4 (paid) fails the script before the gate check.
echo '[{"Type":"private_v6","Address":"fdaa::1"},{"Type":"v4","Address":"9.9.9.9"},{"Type":"shared_v4","Address":"1.2.3.4"},{"Type":"v6","Address":"2a09::1"}]' > "$tmp/ips"
run "$root/scripts/fly-go-public.sh" critical-hit-staging
[ "$status" != 0 ] && ! grep -q '^gate ' "$calls" || fail "go-public: a dedicated IPv4 was accepted: $(cat "$calls")"
echo '[{"Type":"private_v6","Address":"fdaa::1"}]' > "$tmp/ips"
echo '[]' > "$tmp/machines"
run "$root/scripts/fly-go-public.sh" critical-hit-staging
[ "$status" != 0 ] && ! grep -q 'ips allocate' "$calls" || fail "go-public: allocated for an app with no machine"

[ "$fails" = 0 ] || exit 1
echo "test-fly-public-setup: ok"
