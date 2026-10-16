#!/usr/bin/env bash
# `just dev-smoke`: proves the local dev lifecycle of scripts/dev.sh against the real stack.
# Everything runs in a throwaway CRITICAL_DEV_DIR, on free ports, in a throwaway docker compose
# project, so a `just dev` running from this or another checkout is never touched. Scenarios:
#   1. start: API, web and database answer; air rebuilds -> the server shuts down gracefully
#   2. Ctrl-C (SIGINT to the process group of `just dev`): clean exit, nothing left, ports free,
#      database stopped (this run started it)
#   3. start, then `just stop` from another process: same end state; stop twice is fine
#   4. supervisor killed with SIGKILL, then `just stop`: same end state
#   5. a database already running is left running by Ctrl-C; `just stop` stops it
#   6. port taken / docker not running: clear message, nothing started
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
work="$(mktemp -d)"
free_port() { python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])'; }
export CRITICAL_DEV_DIR="$work/dev" CRITICAL_DEV_FIXTURE=1 COMPOSE_PROJECT_NAME="criticalsmoke$$"
export CRITICAL_API_PORT="$(free_port)" CRITICAL_WEB_PORT="$(free_port)" CRITICAL_PG_PORT="$(free_port)"
export CRITICAL_DEV_GRACE=20
bin="$CRITICAL_DEV_DIR/tmp/critical-api"
probe="$root/api/pkg/must/zz_devsmoke_probe.go"
pgid=""

fail() { echo "FAIL: $*" >&2; echo "--- log ---" >&2; tail -40 "$work/dev.log" >&2 2>/dev/null || true; exit 1; }
ok() { echo "ok: $*"; }
until_ok() { # until_ok <seconds> <description> <command...>: readiness poll, no fixed sleeps
    local n="$1" what="$2" i; shift 2
    for ((i = 0; i < n * 5; i++)); do "$@" >/dev/null 2>&1 && return 0; sleep 0.2; done
    fail "timed out waiting for: $what"
}
listening() { [ -n "$(ss -H -ltn "sport = :$1")" ]; }
api_up() { curl -fsS "http://127.0.0.1:$CRITICAL_API_PORT/api/health"; }
web_up() { curl -fsS "http://127.0.0.1:$CRITICAL_WEB_PORT/api/health"; } # through Vite's proxy
db_up() { docker compose exec -T postgres pg_isready -U critical -d critical; }
db_running() { [ -n "$(docker compose ps --status running --quiet postgres)" ]; }
left_over() { pgrep -f -x "$bin" || pgrep -f -- "--port $CRITICAL_WEB_PORT --strictPort" || pgrep -f -- "-tmp_dir $CRITICAL_DEV_DIR/tmp" || true; }

cleanup() {
    rm -f "$probe"
    [ -z "$pgid" ] || kill -KILL -- "-$pgid" 2>/dev/null || true
    scripts/dev.sh stop >/dev/null 2>&1 || true
    docker compose down --volumes >/dev/null 2>&1 || true
    rm -rf "$work"
}
trap cleanup EXIT
trap 'echo "--- log ---"; tail -30 "$work/dev.log"' ERR

start_dev() { # start_dev: `just dev` as its own process group, like a terminal's foreground job
    : >"$work/dev.log"
    setsid just dev >"$work/dev.log" 2>&1 &
    pgid=$!
    until_ok 240 "API answers" api_up
    until_ok 60 "web dev server answers (proxying the API)" web_up
    until_ok 60 "database accepts connections" db_up
    ok "API :$CRITICAL_API_PORT, web :$CRITICAL_WEB_PORT, database :$CRITICAL_PG_PORT answer"
}

assert_clean() { # assert_clean <db: stopped|running>
    local p
    for p in "$CRITICAL_API_PORT" "$CRITICAL_WEB_PORT"; do listening "$p" && fail "port $p still open"; done
    [ -z "$(left_over)" ] || fail "processes left: $(ps -o pid,args -p "$(left_over | paste -sd,)")"
    [ ! -e "$CRITICAL_DEV_DIR/run/api.pid" ] || fail "stale pid record"
    if [ "$1" = stopped ]; then
        db_running && fail "database still running"
        listening "$CRITICAL_PG_PORT" && fail "port $CRITICAL_PG_PORT still open"
    else
        db_running || fail "database was stopped"
    fi
    ok "no process left, API/web ports free, database $1"
}

# 1. start + graceful restart on rebuild
start_dev
before="$(grep -c 'shutdown requested' "$work/dev.log" || true)"
printf 'package must\n' >"$probe"
until_ok 120 "air restarts the server after a source change (graceful: it logged the shutdown)" \
    bash -c "[ \"\$(grep -c 'shutdown requested' '$work/dev.log')\" -gt $before ]"
until_ok 120 "API back after the rebuild" api_up
rm -f "$probe"
ok "rebuild restarted the API through SIGINT (graceful shutdown)"

# 2. Ctrl-C
kill -INT -- "-$pgid"
wait "$pgid" 2>/dev/null && code=0 || code=$?
pgid=""
[ "$code" = 0 ] || [ "$code" = 130 ] || fail "Ctrl-C: just dev exited $code"
# just itself ends a Ctrl-C'd recipe with this one line (exit 130); anything else is noise.
grep -v '^error: interrupted by SIGINT$' "$work/dev.log" | grep -qi 'terminated by signal\|^error\|recipe .* failed' && fail "Ctrl-C printed errors"
grep -q 'shutdown requested' "$work/dev.log" || fail "the API was not shut down gracefully"
assert_clean stopped
ok "Ctrl-C: exit 0, no error output"

# 3. just stop while just dev runs elsewhere
start_dev
just stop >"$work/stop.log" 2>&1 || fail "just stop failed"
wait "$pgid" 2>/dev/null || true
pgid=""
assert_clean stopped
just stop >>"$work/stop.log" 2>&1 || fail "second just stop (nothing runs) failed"
ok "just stop: stops the running stack; idempotent"

# 4. supervisor killed hard
start_dev
read -r sup _ <"$CRITICAL_DEV_DIR/run/supervisor.pid"
kill -KILL "$sup"
just stop >"$work/stop.log" 2>&1 || fail "just stop after a killed supervisor failed"
pgid=""
assert_clean stopped
ok "just stop after the supervisor was killed"

# 5. database already running is left alone by Ctrl-C
just dev-db >"$work/db.log" 2>&1 || fail "just dev-db failed"
start_dev
kill -INT -- "-$pgid"
wait "$pgid" 2>/dev/null || true
pgid=""
assert_clean running
just stop >"$work/stop.log" 2>&1
assert_clean stopped
ok "a database started beforehand survives Ctrl-C and is stopped by just stop"

# 6. clear refusals
python3 -c "import socket,time; s=socket.socket(); s.bind(('127.0.0.1',$CRITICAL_API_PORT)); s.listen(); time.sleep(60)" &
blocker=$!
until_ok 10 "blocker listening" listening "$CRITICAL_API_PORT"
out="$(just dev 2>&1)" && fail "just dev started on a taken port"
kill "$blocker" 2>/dev/null || true
grep -q "port $CRITICAL_API_PORT (API) is taken" <<<"$out" || fail "no clear message for a taken port: $out"
db_running && fail "database started despite the taken port"
out="$(DOCKER_HOST=unix:///nonexistent/docker.sock just dev 2>&1)" && fail "just dev started without docker"
grep -q "docker daemon is not running" <<<"$out" || fail "no clear message without docker: $out"
ok "taken port and missing docker refused with a clear message, nothing started"

echo "--- dev lifecycle smoke test passed ---"
