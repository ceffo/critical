#!/usr/bin/env bash
# Installs Chromium's system packages in CI (the e2e job, .github/workflows/ci.yml) with bounded, retried
# attempts. Debs downloaded by a timed-out attempt stay in /var/cache/apt/archives, so a retry resumes.
#
# `timeout` only signals its direct child (npx); the `sudo apt-get` it spawned lives on, still holding
# /var/lib/apt/lists/lock, and every later attempt then fails at once on the lock. So after a failed
# attempt this script reaps the leftover apt processes and waits for the lock to be free before retrying.
#
# Cache hit, no network: when OFFLINE_CMD is set it runs first, once. It installs the cached debs without
# contacting any mirror (apt-get --no-download), so an Ubuntu archive or DNS outage cannot fail the job that
# has its packages already. If it fails (no cache, a stale one missing a package) the bounded online
# attempts below run as before: a cold cache still uses the archive.
#
# Environment (defaults are the CI values; scripts/test-ci-install-chromium-deps.sh overrides them):
#   INSTALL_CMD      the install command                       (npx playwright install-deps chromium)
#   OFFLINE_CMD     cache-only install, tried once first      (empty: skip)
#   OFFLINE_TIMEOUT seconds the offline install may run       (120)
#   ATTEMPTS         number of attempts                        (3)
#   ATTEMPT_TIMEOUT  seconds one attempt may run               (135)
#   SETTLE_SECONDS   seconds to wait for apt/dpkg to be gone   (15)
#   SUDO             privilege prefix, empty for none          (sudo)
#   APT_PROCS        process names that hold the apt locks     (apt-get apt dpkg)
#   APT_KILL         the subset of those that get stopped      (apt-get apt)
set -uo pipefail

INSTALL_CMD="${INSTALL_CMD:-npx playwright install-deps chromium}"
OFFLINE_CMD="${OFFLINE_CMD:-}"
OFFLINE_TIMEOUT="${OFFLINE_TIMEOUT:-120}"
ATTEMPTS="${ATTEMPTS:-3}"
ATTEMPT_TIMEOUT="${ATTEMPT_TIMEOUT:-135}"
SETTLE_SECONDS="${SETTLE_SECONDS:-15}"
SUDO="${SUDO-sudo}"
APT_PROCS="${APT_PROCS:-apt-get apt dpkg}"
APT_KILL="${APT_KILL:-apt-get apt}"

# shellcheck disable=SC2086 # APT_PROCS and SUDO are word lists on purpose.
holders() { for p in $APT_PROCS; do pgrep -x "$p"; done; }

# free_locks: stop what a timed-out attempt left running, then wait until nothing holds an apt lock.
free_locks() {
    local p waited=0
    if [ -n "$(holders)" ]; then
        echo "apt processes still running after the timed-out attempt; stopping them"
        # shellcheck disable=SC2086
        for p in $APT_KILL; do $SUDO pkill -TERM -x "$p" || true; done
        while [ -n "$(holders)" ] && [ "$waited" -lt "$SETTLE_SECONDS" ]; do
            sleep 1
            waited=$((waited + 1))
            # After a few seconds of grace, stop asking politely.
            if [ "$waited" -eq 3 ]; then
                # shellcheck disable=SC2086
                for p in $APT_KILL; do $SUDO pkill -KILL -x "$p" || true; done
            fi
        done
    fi
    # An apt-get that was killed here, or died on its own, may have left packages unpacked but unconfigured;
    # finish them before the next run, whether or not a process was left to stop.
    # shellcheck disable=SC2086
    $SUDO dpkg --configure -a || true
}

if [ -n "$OFFLINE_CMD" ]; then
    # shellcheck disable=SC2086
    if timeout "$OFFLINE_TIMEOUT" $OFFLINE_CMD; then
        echo "installed from the cached debs, no network needed"
        exit 0
    fi
    echo "offline install from the cached debs failed; falling back to the archive"
    free_locks
fi

for attempt in $(seq 1 "$ATTEMPTS"); do
    # shellcheck disable=SC2086
    if timeout "$ATTEMPT_TIMEOUT" $INSTALL_CMD; then
        exit 0
    fi
    echo "install-deps attempt $attempt did not finish; retrying (downloaded debs are kept)"
    free_locks
done
exit 1
