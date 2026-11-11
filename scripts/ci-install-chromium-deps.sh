#!/usr/bin/env bash
# Installs Chromium's system packages in CI (the e2e job, .github/workflows/ci.yml) with bounded, retried
# attempts. Debs downloaded by a timed-out attempt stay in /var/cache/apt/archives, so a retry resumes.
#
# `timeout` only signals its direct child (npx); the `sudo apt-get` it spawned lives on, still holding
# /var/lib/apt/lists/lock, and every later attempt then fails at once on the lock. So after a failed
# attempt this script reaps the leftover apt processes and waits for the lock to be free before retrying.
#
# Cache hit, no network: when OFFLINE=1 the packages Chromium needs are installed first, once, from the debs
# already in /var/cache/apt/archives, by name and with `apt-get --no-download`, so no mirror is contacted and an
# Ubuntu archive or DNS outage cannot fail the job that has its packages. The names are the ones Playwright
# itself would install (read by running PACKAGES_CMD against a recording apt-get, nothing is installed by that),
# never "every cached .deb": a stray cached deb can depend on packages that are not cached (firefox, xfconf)
# and the install would then try to download them. If the offline install fails (no cache, a cache older than
# the package lists) the bounded online attempts below run as before: a cold cache still uses the archive.
#
# Dead default mirror: the runner's apt sources (/etc/apt/sources.list.d/ubuntu.sources) say
# `URIs: mirror+file:/etc/apt/apt-mirrors.txt`, a list holding the Azure Ubuntu mirror and the canonical
# archives. apt's mirror method does not try that list in file order (a fallback line added at the top was
# never used), and when those hosts are unreachable every attempt spends minutes timing out on them. So before
# each online attempt the first mirror of the list is probed; if it does not answer, the first fallback mirror
# (other official Ubuntu mirrors, FALLBACK_MIRRORS) that answers replaces the `mirror+file:` URI in the sources
# files themselves, and the log prints the URIs apt will use. A healthy default mirror leaves them untouched.
#
# Environment (defaults are the CI values; scripts/test-ci-install-chromium-deps.sh overrides them):
#   INSTALL_CMD      the install command                       (npx playwright install-deps chromium)
#   OFFLINE         1: try the cache-only install first       (unset: skip)
#   PACKAGES_CMD    command whose apt-get install line names  ($INSTALL_CMD)
#                   the packages
#   APT_GET         the apt-get used for the offline install   (apt-get)
#   MIRRORS_FILE    apt's mirror list, probed for its first    (/etc/apt/apt-mirrors.txt)
#                   mirror
#   SOURCES_FILES   apt sources files rewritten to the fallback (/etc/apt/sources.list.d/ubuntu.sources
#                                                                /etc/apt/sources.list)
#   FALLBACK_MIRRORS  mirror URLs tried when the first one is down (a built-in list of official mirrors)
#   PROBE_CMD       command run with a mirror's InRelease URL, success = the mirror answers (curl, 5 s)
#   OFFLINE_TIMEOUT seconds the offline install may run       (120)
#   ATTEMPTS         number of attempts                        (3)
#   ATTEMPT_TIMEOUT  seconds one attempt may run               (135)
#   SETTLE_SECONDS   seconds to wait for apt/dpkg to be gone   (15)
#   SUDO             privilege prefix, empty for none          (sudo)
#   APT_PROCS        process names that hold the apt locks     (apt-get apt dpkg)
#   APT_KILL         the subset of those that get stopped      (apt-get apt)
set -uo pipefail

INSTALL_CMD="${INSTALL_CMD:-npx playwright install-deps chromium}"
OFFLINE="${OFFLINE:-}"
APT_GET="${APT_GET:-apt-get}"
OFFLINE_TIMEOUT="${OFFLINE_TIMEOUT:-120}"
ATTEMPTS="${ATTEMPTS:-3}"
ATTEMPT_TIMEOUT="${ATTEMPT_TIMEOUT:-135}"
SETTLE_SECONDS="${SETTLE_SECONDS:-15}"
SUDO="${SUDO-sudo}"
APT_PROCS="${APT_PROCS:-apt-get apt dpkg}"
APT_KILL="${APT_KILL:-apt-get apt}"

PACKAGES_CMD="${PACKAGES_CMD:-$INSTALL_CMD}"
MIRRORS_FILE="${MIRRORS_FILE:-/etc/apt/apt-mirrors.txt}"
FALLBACK_MIRRORS="${FALLBACK_MIRRORS:-https://mirrors.edge.kernel.org/ubuntu/ https://mirror.math.princeton.edu/pub/ubuntu/ https://mirrors.xtom.com/ubuntu/ https://ubuntu.mirror.constant.com/ https://mirrors.mit.edu/ubuntu/ http://mirror.us.leaseweb.net/ubuntu/ https://ftp.fau.de/ubuntu/}"
SOURCES_FILES="${SOURCES_FILES:-/etc/apt/sources.list.d/ubuntu.sources /etc/apt/sources.list}"
PROBE_CMD="${PROBE_CMD:-curl -fsS -o /dev/null --connect-timeout 3 -m 5}"

# package_names: the package names of the `apt-get install` that PACKAGES_CMD would run. apt-get and sudo are
# replaced by recorders for that one run, so nothing is installed and no network is used.
package_names() {
    local shim rec
    shim="$(mktemp -d)"
    rec="$shim/args"
    printf '#!/usr/bin/env bash\n[ "$1" = install ] && printf "%%s\\n" "$@" > "%s"\nexit 0\n' "$rec" > "$shim/apt-get"
    printf '#!/usr/bin/env bash\nexec "$@"\n' > "$shim/sudo"
    chmod +x "$shim/apt-get" "$shim/sudo"
    # shellcheck disable=SC2086
    PATH="$shim:$PATH" $PACKAGES_CMD >/dev/null 2>&1 || true
    [ -s "$rec" ] && grep -v -e '^install$' -e '^-' "$rec"
    rm -rf "$shim"
}

# mirror_answers URL: does the mirror serve the release's metadata?
mirror_answers() {
    local codename
    codename="$(. /etc/os-release 2>/dev/null && echo "${VERSION_CODENAME:-}")"
    # shellcheck disable=SC2086
    $PROBE_CMD "${1%/}/dists/${codename:-stable}-updates/InRelease" >/dev/null 2>&1
}

# uses_mirror_list: do the sources still point at the runner's mirror list?
uses_mirror_list() {
    local f
    # shellcheck disable=SC2086
    for f in $SOURCES_FILES; do [ -r "$f" ] && grep -q 'mirror+file:' "$f" && return 0; done
    return 1
}

# ensure_mirror: see "Dead default mirror" above.
ensure_mirror() {
    local first m f
    [ -r "$MIRRORS_FILE" ] || return 0
    uses_mirror_list || return 0
    first="$(awk 'NF {print $1; exit}' "$MIRRORS_FILE")"
    [ -n "$first" ] || return 0
    mirror_answers "$first" && return 0
    echo "mirror $first does not answer; looking for another Ubuntu mirror"
    for m in $FALLBACK_MIRRORS; do
        if mirror_answers "$m"; then
            echo "using $m instead of the runner's mirror list"
            # shellcheck disable=SC2086
            for f in $SOURCES_FILES; do
                [ -r "$f" ] && $SUDO sed -i "s#mirror+file:[^[:space:]]*#${m%/}/#g" "$f"
            done
            echo "apt sources now use:"
            # shellcheck disable=SC2086
            cat $SOURCES_FILES 2>/dev/null | grep -E '^(URIs:|deb )' | sort -u
            return 0
        fi
        echo "mirror $m does not answer either"
    done
    echo "no fallback mirror answers; keeping the runner's list"
}

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

if [ "$OFFLINE" = 1 ]; then
    pkgs="$(package_names | tr '\n' ' ')"
    # shellcheck disable=SC2086
    if [ -n "$pkgs" ] && timeout "$OFFLINE_TIMEOUT" $SUDO $APT_GET install -y --no-install-recommends --no-download $pkgs; then
        echo "installed from the cached debs, no network needed"
        exit 0
    fi
    echo "offline install from the cached debs failed; falling back to the archive"
    free_locks
fi

for attempt in $(seq 1 "$ATTEMPTS"); do
    ensure_mirror
    # shellcheck disable=SC2086
    if timeout "$ATTEMPT_TIMEOUT" $INSTALL_CMD; then
        exit 0
    fi
    echo "install-deps attempt $attempt did not finish; retrying (downloaded debs are kept)"
    free_locks
done
exit 1
