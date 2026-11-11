#!/usr/bin/env bash
# Tests scripts/ci-install-chromium-deps.sh offline (part of `just validate`): a fake apt-get process
# outlives `timeout` as a grandchild, as the real `sudo apt-get` does, and "holds the lock" for as long as it
# lives. The fake is named uniquely per run and the script is told to look only for that name, so neither a
# real apt-get on the machine nor a parallel run of this test (another worktree's gate) can be seen or
# killed. No wall-clock race: which attempt hangs is decided by a counter, not by a background sleep.
# Attempt 1 times out, the script must stop the leftover apt-get so attempt 2 gets the lock; a mirror
# that is really down must still end in a failure after the bounded attempts. With cached debs the offline
# install runs first and, when it works, the mirrors are never contacted.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
script="$root/scripts/ci-install-chromium-deps.sh"
tmp="$(mktemp -d)"
fake="fakeapt$$"
trap 'pkill -f "^$tmp/bin/$fake" 2>/dev/null || true; rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-ci-install-chromium-deps: FAIL: $*" >&2; fails=$((fails + 1)); }

mkdir -p "$tmp/bin" "$tmp/state"
# The fake apt-get: a copy of bash under this name, so the process really has it (a script would show as
# bash) and it is a real executable, not a link into a multi-call binary: on ubuntu-26.04 sleep is a link to
# coreutils, which dispatches on the name it is called by and exits at once for an unknown one, so the fake
# never ran and the first attempt never timed out. comm is cut at 15 characters; "fakeapt" + the pid stays under.
cp "$(command -v bash)" "$tmp/bin/$fake"
fifo="$tmp/state/hold"; mkfifo "$fifo"
# install: what npx would run. Fails at once while an apt-get is alive ("Could not get lock"); otherwise,
# when slow, starts apt-get in its own session (as sudo's pty does, so timeout's group signal does not reach
# it) and waits on it. Slow = the first attempt, or every attempt when state/always exists.
cat > "$tmp/bin/install" <<INST
#!/usr/bin/env bash
if pgrep -x "$fake" >/dev/null; then echo "E: Could not get lock" >&2; exit 100; fi
n=\$(cat "$tmp/state/n" 2>/dev/null || echo 0); echo \$((n + 1)) > "$tmp/state/n"
if [ "\$n" -eq 0 ] && [ -e "$tmp/state/dies" ]; then exit 1; fi
if [ "\$n" -eq 0 ] || [ -e "$tmp/state/always" ]; then
    # Waits on a builtin read (no child process), so killing the fake leaves nothing behind.
    setsid "$tmp/bin/$fake" -c 'read -t 60 <> "$fifo"' &
    wait \$!
fi
INST
chmod +x "$tmp/bin/install"
# dpkg stub, so the script's "dpkg --configure -a" touches nothing real.
printf '#!/usr/bin/env bash\ntouch "%s/state/configured"\n' "$tmp" > "$tmp/bin/dpkg"
chmod +x "$tmp/bin/dpkg"

run() {
    PATH="$tmp/bin:$PATH" SUDO="" INSTALL_CMD="${INSTALL_CMD:-$tmp/bin/install}" ATTEMPT_TIMEOUT=2 SETTLE_SECONDS=8 \
        APT_PROCS="$fake" APT_KILL="$fake" \
        "$script" "$@"
}

# Attempt 1 hangs holding the lock; the script must free it so attempt 2 (not slow any more) succeeds.
if ! out="$(run 2>&1)"; then fail "attempt 2 did not proceed after a timed-out attempt: $out"; fi
grep -q "Could not get lock" <<<"$out" && fail "a retry hit the lock: $out"
[ -e "$tmp/state/configured" ] || fail "dpkg --configure -a was not run after stopping apt-get"
pgrep -x "$fake" >/dev/null && fail "an apt-get is still running after the run"

# Attempt 1 fails and leaves no process behind (apt-get died on its own, perhaps mid-unpack): the packages
# it left must still be configured before the retry.
rm -f "$tmp/state/n" "$tmp/state/configured"; touch "$tmp/state/dies"
if ! out="$(run 2>&1)"; then fail "a retry after an attempt that left no process failed: $out"; fi
[ -e "$tmp/state/configured" ] || fail "dpkg --configure -a was not run after an attempt that left no apt process"
rm -f "$tmp/state/dies"

# The mirror is really down: every attempt hangs, the script fails after the bounded attempts.
touch "$tmp/state/always"
if ATTEMPTS=2 run >/dev/null 2>&1; then fail "a permanently stalled install reported success"; fi

# Cache hit, archive unreachable: the online install (INSTALL_CMD, whose `apt-get update` needs the mirrors)
# always fails and records that it ran; the offline install works. The script must succeed without ever
# running the online one.
rm -f "$tmp/state/always" "$tmp/state/n" "$tmp/state/online-ran" "$tmp/state/offline-ran"
printf '#!/usr/bin/env bash\ntouch "%s/state/online-ran"; echo "Err: azure.archive.ubuntu.com unreachable" >&2; exit 100\n' "$tmp" > "$tmp/bin/online-down"
printf '#!/usr/bin/env bash\ntouch "%s/state/offline-ran"\n' "$tmp" > "$tmp/bin/offline-ok"
printf '#!/usr/bin/env bash\ntouch "%s/state/offline-ran"; exit 100\n' "$tmp" > "$tmp/bin/offline-bad"
chmod +x "$tmp/bin/online-down" "$tmp/bin/offline-ok" "$tmp/bin/offline-bad"
if ! out="$(INSTALL_CMD="$tmp/bin/online-down" OFFLINE_CMD="$tmp/bin/offline-ok" run 2>&1)"; then
    fail "a cache hit with the archive unreachable failed: $out"
fi
[ -e "$tmp/state/offline-ran" ] || fail "the offline install did not run on a cache hit"
[ -e "$tmp/state/online-ran" ] && fail "the online install ran although the offline one succeeded"

# Cold cache (the offline install fails) with the archive reachable: falls back to the online install.
rm -f "$tmp/state/online-ran" "$tmp/state/offline-ran" "$tmp/state/n"
if ! out="$(OFFLINE_CMD="$tmp/bin/offline-bad" run 2>&1)"; then fail "a cold cache did not fall back to the online install: $out"; fi
[ -e "$tmp/state/offline-ran" ] || fail "the offline install was not tried first"
grep -q "falling back to the archive" <<<"$out" || fail "the fallback was not announced: $out"

# Neither works: the script fails.
if INSTALL_CMD="$tmp/bin/online-down" OFFLINE_CMD="$tmp/bin/offline-bad" ATTEMPTS=1 run >/dev/null 2>&1; then
    fail "an install that failed offline and online reported success"
fi

[ "$fails" -eq 0 ] && echo "test-ci-install-chromium-deps: ok"
exit "$fails"
