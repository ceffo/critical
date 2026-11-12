#!/usr/bin/env bash
# Sweeps what cancelled CI jobs left in the runner's Docker daemon, before a job that creates networks.
#
#   scripts/ci-docker-sweep.sh
#
# Why: a run superseded by a newer push is cancelled (concurrency, cancel-in-progress), and a cancelled job can be
# killed before its cleanup trap runs, leaving its throwaway containers and compose/bridge network behind. The
# daemon has about 30 predefined network address pools; once leaked networks fill them, every later
# `docker network create` fails with "all predefined address pools have been fully subnetted".
#
# Safe beside other runner instances on the same daemon: only the smoke jobs' own leftovers (names
# critical-smoke-* and criticalsmoke*) older than $SWEEP_AGE_HOURS (default 1, far above the 10-15 min job
# timeouts) are removed; a running job's resources are younger than that. Never fails the job.
set -uo pipefail

command -v docker >/dev/null 2>&1 || exit 0
cutoff="$(date -u -d "-${SWEEP_AGE_HOURS:-1} hours" +%s)" || exit 0

# stale <container|network> <id>: true when it was created before the cutoff.
stale() {
    local created t
    created="$(docker "$1" inspect -f '{{.Created}}' "$2" 2>/dev/null)" || return 1
    t="$(date -u -d "${created%%.*}Z" +%s 2>/dev/null)" || return 1
    [ "$t" -lt "$cutoff" ]
}

containers=0 networks=0
for id in $(docker ps -aq --filter name='critical-smoke-' --filter name='criticalsmoke'); do
    if stale container "$id" && docker rm -f -v "$id" >/dev/null 2>&1; then containers=$((containers + 1)); fi
done
for id in $(docker network ls -q --filter name='critical-smoke-' --filter name='criticalsmoke'); do
    if stale network "$id" && docker network rm "$id" >/dev/null 2>&1; then networks=$((networks + 1)); fi
done
echo "ci-docker-sweep: removed $containers stale container(s) and $networks stale network(s)"
exit 0
