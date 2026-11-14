#!/usr/bin/env bash
# Runs the e2e suite in CI with the VM's e2e slot held, and logs what the host looks like.
#
#   scripts/ci-e2e.sh                   # `just e2e`, 6 minutes at most once it has the slot
#   E2E_TIMEOUT=2m E2E_CMD="..." E2E_LOCK=<file> scripts/ci-e2e.sh   # the tests' knobs
#
# Why: the self-hosted runner services share one machine, and the suite is CPU-bound (3 Playwright workers on
# 4 vCPU: docs/ci-runner.md). Alone it takes about 4 minutes; with other e2e runs and the smoke and image jobs beside it,
# 5.6 to 6+ minutes, which tripped the 6-minute limit with every test green. So one suite runs per machine at a
# time (flock on a file under $HOME, which the runner services share; on a machine with a single service the
# lock never waits), and the 6-minute limit counts from getting the slot, so a queued run
# does not time out for waiting. The job's own timeout bounds the wait.
#
# The lock is released when this script ends: the command gets fd 9 closed, so a Chromium or Vite that outlives a
# timed-out run cannot keep the next run waiting. A flock held by a killed job is dropped by the kernel.
set -uo pipefail

lock="${E2E_LOCK:-$HOME/.cache/critical-e2e.lock}"
limit="${E2E_TIMEOUT:-6m}"
cmd="${E2E_CMD:-just e2e}"

host() { echo "e2e host ($1): $(nproc) cpus, load $(cut -d' ' -f1-3 /proc/loadavg), $(free -m | awk '/^Mem:/ {print $2 " MB, " $7 " MB available"}')"; }

mkdir -p "$(dirname "$lock")"
exec 9>"$lock"
host "before the slot" || true
queued="$(date +%s)"
flock 9
echo "e2e slot taken after $(($(date +%s) - queued)) s"
host "slot taken" || true

started="$(date +%s)"
timeout --kill-after=15s "$limit" bash -c "$cmd" 9>&-
status=$?
echo "e2e command ended with status $status after $(($(date +%s) - started)) s (limit $limit)"
host "after" || true
exit "$status"
