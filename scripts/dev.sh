#!/usr/bin/env bash
# Local development lifecycle of one checkout: `just dev` (start), `just stop`, `just status`,
# `just dev-db`. Docs: docs/backend.md ("Local development").
#
#   dev.sh start    PostgreSQL (docker compose) + API (air) + Vite; Ctrl-C / SIGTERM stops them
#   dev.sh stop     stops everything this checkout started (also after a crashed `start`), the
#                   compose database included (data kept); idempotent
#   dev.sh status   what runs, on which port
#   dev.sh db       starts the compose database only and waits until it accepts connections
#
# Every process `start` launches is recorded in $RUN (pid + start time) so `stop` finds exactly
# those, never a process by name. Each runs in its own session: a terminal's Ctrl-C reaches only
# this script, which then shuts everything down in order (API first: air interrupts the server and
# waits for its graceful shutdown), instead of every process dying at once with interleaved errors.
#
# Environment: CRITICAL_API_PORT (8080), CRITICAL_WEB_PORT (5173), CRITICAL_PG_PORT (5432),
# CRITICAL_DEV_DIR (<repo>/.dev: pid records, the API binary), CRITICAL_DEV_FIXTURE=1 (fixture
# catalogue even when .catalogue/ exists), COMPOSE_PROJECT_NAME (docker compose).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
dev_dir="${CRITICAL_DEV_DIR:-$root/.dev}"
run="$dev_dir/run"
bin="$dev_dir/tmp/critical-api"
api_port="${CRITICAL_API_PORT:-8080}"
web_port="${CRITICAL_WEB_PORT:-5173}"
pg_port="${CRITICAL_PG_PORT:-5432}"
export CRITICAL_PG_PORT="$pg_port"
grace="${CRITICAL_DEV_GRACE:-15}" # seconds a process gets to stop before SIGKILL

say() { echo "dev: $*" >&2; }
die() { say "error: $*"; exit 1; }

# --- processes ---------------------------------------------------------------------------------

proc_start() { sed 's/^.*) //' "/proc/$1/stat" 2>/dev/null | cut -d' ' -f20; }

# record <name> <pid>: remember the process with its start time (guards against pid reuse).
record() { mkdir -p "$run"; echo "$2 $(proc_start "$2")" >"$run/$1.pid"; }

# recorded_pid <name>: prints the pid when the recorded process is still the same one.
recorded_pid() {
    local pid st
    [ -f "$run/$1.pid" ] || return 1
    read -r pid st <"$run/$1.pid"
    [ -n "$pid" ] && [ "$(proc_start "$pid")" = "$st" ] && [ -n "$st" ] && echo "$pid"
}

wait_gone() { # wait_gone <pid> <seconds>
    local i
    for ((i = 0; i < $2 * 10; i++)); do kill -0 "$1" 2>/dev/null || return 0; sleep 0.1; done
    return 1
}

# stop_group <name>: SIGINT to the recorded process group, SIGKILL after the grace period.
stop_group() {
    local pid
    if pid="$(recorded_pid "$1")"; then
        kill -INT -- "-$pid" 2>/dev/null || true
        if ! wait_gone "$pid" "$grace"; then
            say "$1 did not stop in ${grace}s: killing it"
            kill -KILL -- "-$pid" 2>/dev/null || true
            wait_gone "$pid" 5 || true
        fi
    fi
    rm -f "$run/$1.pid"
}

# stop_stray_api: the API binary air starts lives in its own process group; if air died without
# stopping it, it is found by its exact command line (this checkout's binary path).
stop_stray_api() {
    local pids
    pids="$(pgrep -f -x "$bin" || true)"
    [ -n "$pids" ] || return 0
    say "stopping the API binary left behind by air ($(echo $pids))"
    # shellcheck disable=SC2086
    kill -INT $pids 2>/dev/null || true
    # shellcheck disable=SC2086
    for p in $pids; do wait_gone "$p" "$grace" || kill -KILL "$p" 2>/dev/null || true; done
}

port_busy() { [ -n "$(ss -H -ltn "sport = :$1" 2>/dev/null)" ]; }

# --- docker compose PostgreSQL -----------------------------------------------------------------

need_docker() {
    command -v docker >/dev/null 2>&1 || die "docker is not installed (the development database runs in docker compose)"
    docker info >/dev/null 2>&1 || die "the docker daemon is not running or not reachable: start it (e.g. \`sudo systemctl start docker\`) and run this again"
}

db_running() { [ -n "$(docker compose ps --status running --quiet postgres 2>/dev/null)" ]; }

# db_up: starts the database unless it runs; sets db_started=1 when this call started it.
db_up() {
    need_docker
    db_started=0
    if db_running; then
        say "database: already running on 127.0.0.1:$pg_port"
        return 0
    fi
    if port_busy "$pg_port"; then
        die "port $pg_port is taken by another process: free it, or set CRITICAL_PG_PORT to a free port (e.g. CRITICAL_PG_PORT=5433 just dev)"
    fi
    say "database: starting PostgreSQL (docker compose, 127.0.0.1:$pg_port)"
    if ! docker compose up --detach --wait postgres >&2; then
        die "the database did not become ready (see the output above; \`docker compose logs postgres\`)"
    fi
    db_started=1
}

db_down() {
    command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1 || {
        say "database: docker is not running, nothing to stop"
        return 0
    }
    if db_running; then
        say "database: stopping PostgreSQL (the data stays in the volume)"
        docker compose stop postgres >&2
    fi
}

# --- commands ----------------------------------------------------------------------------------

web_deps() {
    if [ ! -f web/node_modules/.package-lock.json ] || [ web/package-lock.json -nt web/node_modules/.package-lock.json ]; then
        say "web/package-lock.json changed since the last install: running npm install"
        (cd web && npm install >&2 && touch node_modules/.package-lock.json)
    fi
}

cmd_start() {
    if recorded_pid supervisor >/dev/null; then
        die "\`just dev\` already runs for this checkout (pid $(recorded_pid supervisor)): \`just stop\` first"
    fi
    for pair in "API:$api_port:CRITICAL_API_PORT" "web:$web_port:CRITICAL_WEB_PORT"; do
        IFS=: read -r what port var <<<"$pair"
        port_busy "$port" && die "port $port ($what) is taken by another process: free it, or set $var (e.g. $var=$((port + 1)) just dev)"
    done
    web_deps

    db_up
    mkdir -p "$run" "$dev_dir/tmp"
    echo "$$ $(proc_start $$)" >"$run/supervisor.pid"
    stopping=0
    trap 'stopping=1; exit 0' INT TERM HUP
    trap cleanup EXIT

    local catalogue seeds db
    if [ -z "${CRITICAL_DEV_FIXTURE:-}" ] && [ -f .catalogue/catalogue.json ]; then
        catalogue="$root/.catalogue/catalogue.json"; seeds="$root/seeds"; db=critical
        say "api: real catalogue .catalogue/catalogue.json, campaign seeds, database critical"
    else
        catalogue="$root/api/testdata/catalogue.json"; seeds="$root/api/testdata/seeds"; db=critical_fixture
        say "api: NO real catalogue: running on the FIXTURE catalogue and api/testdata/seeds, database critical_fixture"
        say "api: (the database critical is left untouched; \`just catalogue\` fetches the real catalogue)"
    fi

    # The version /api/health reports is computed at each rebuild (build-identity.sh): a commit or an edit
    # in a file air does not watch shows only after the next .go rebuild.
    # air: rebuilds on .go changes; the server gets SIGINT (graceful shutdown) on every restart
    # and on exit, SIGKILL only after kill_delay (api/.air.toml).
    (
        cd api
        export CRITICAL_DSN="postgres://critical:critical@localhost:$pg_port/$db?sslmode=disable"
        export CRITICAL_SEEDS_DIR="$seeds" CRITICAL_CATALOGUE="$catalogue" CRITICAL_ADDR=":$api_port"
        # The dev login (ADR 0029 step 8.2): without OAuth settings the server only starts in dev.
        export CRITICAL_ENV=dev
        exec setsid go tool air -c .air.toml -tmp_dir "$dev_dir/tmp" \
            -build.cmd "go build -ldflags \"\$($root/scripts/build-identity.sh)\" -o $bin ./cmd/api" -build.entrypoint "$bin"
    ) &
    record api $!

    (
        cd web
        export CRITICAL_API_URL="http://localhost:$api_port"
        exec setsid npm run dev -- --port "$web_port" --strictPort
    ) &
    record web $!

    say "ready soon: API http://localhost:$api_port, app http://localhost:$web_port. Ctrl-C stops everything."

    # Leave when either process dies on its own (a build error does not kill air).
    while kill -0 "$(recorded_pid api || echo 0)" 2>/dev/null && kill -0 "$(recorded_pid web || echo 0)" 2>/dev/null; do
        sleep 1 &
        wait $! || true
    done
    [ "$stopping" = 1 ] || { say "a process exited by itself: shutting down"; exit_code=1; }
    exit "${exit_code:-0}"
}

cleanup() {
    local code=$?
    trap - EXIT INT TERM HUP
    [ "${exit_code:-0}" = 0 ] || code="$exit_code"
    echo >&2
    say "stopping..."
    stop_group web
    stop_group api
    stop_stray_api
    # The database is left as found: stopped again only when this run started it.
    if [ "${db_started:-0}" = 1 ]; then db_down; fi
    rm -f "$run/supervisor.pid"
    say "stopped"
    exit "$code"
}

cmd_stop() {
    local sup
    if sup="$(recorded_pid supervisor)"; then
        say "stopping \`just dev\` (pid $sup)"
        kill -TERM "$sup" 2>/dev/null || true
        wait_gone "$sup" $((grace * 2 + 10)) || say "the supervisor did not exit; stopping its processes directly"
    fi
    # Whatever is left (supervisor killed, crashed): the recorded processes, then the database.
    stop_group web
    stop_group api
    stop_stray_api
    rm -f "$run/supervisor.pid"
    db_down
    say "stopped: nothing of this checkout runs (database data kept; \`just dev-reset\` deletes it)"
}

cmd_status() {
    local pid
    for n in api web; do
        port="$api_port"; [ "$n" = web ] && port="$web_port"
        if pid="$(recorded_pid "$n")"; then
            if port_busy "$port"; then echo "$n: running (pid $pid), listening on :$port"
            else echo "$n: starting or not listening yet (pid $pid), expected on :$port"; fi
        else
            echo "$n: stopped"
            port_busy "$port" && echo "     (port $port is taken by a process that is not this checkout's)"
        fi
    done
    if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
        echo "database: unknown (docker is not running)"
    elif db_running; then
        echo "database: running (docker compose), 127.0.0.1:$pg_port"
    else
        echo "database: stopped (data kept in the volume critical-pg)"
    fi
}

case "${1:-}" in
    start) cmd_start ;;
    stop) cmd_stop ;;
    status) cmd_status ;;
    db) db_up ;;
    *) echo "usage: $0 start|stop|status|db" >&2; exit 2 ;;
esac
