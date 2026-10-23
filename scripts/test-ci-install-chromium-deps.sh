#!/usr/bin/env bash
# Tests scripts/ci-install-chromium-deps.sh offline (part of `just validate`): a fake apt-get process
# outlives `timeout` as a grandchild, as the real `sudo apt-get` does, and "holds the lock" for as long as it
# lives. The fake is named uniquely per run and the script is told to look only for that name, so neither a
# real apt-get on the machine nor a parallel run of this test (another worktree's gate) can be seen or
# killed. No wall-clock race: which attempt hangs is decided by a counter, not by a background sleep.
# Attempt 1 times out, the script must stop the leftover apt-get so attempt 2 gets the lock; a mirror
# that is really down must still end in a failure after the bounded attempts.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
script="$root/scripts/ci-install-chromium-deps.sh"
tmp="$(mktemp -d)"
fake="fakeapt$$"
trap 'pkill -f "^$tmp/bin/$fake" 2>/dev/null || true; rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-ci-install-chromium-deps: FAIL: $*" >&2; fails=$((fails + 1)); }

mkdir -p "$tmp/bin" "$tmp/state"
# The fake apt-get: a symlink to sleep, so the process really has this name (a script would show as bash).
# comm is cut at 15 characters; "fakeapt" + the pid stays under.
ln -s "$(command -v sleep)" "$tmp/bin/$fake"
# install: what npx would run. Fails at once while an apt-get is alive ("Could not get lock"); otherwise,
# when slow, starts apt-get in its own session (as sudo's pty does, so timeout's group signal does not reach
# it) and waits on it. Slow = the first attempt, or every attempt when state/always exists.
cat > "$tmp/bin/install" <<INST
#!/usr/bin/env bash
if pgrep -x "$fake" >/dev/null; then echo "E: Could not get lock" >&2; exit 100; fi
n=\$(cat "$tmp/state/n" 2>/dev/null || echo 0); echo \$((n + 1)) > "$tmp/state/n"
if [ "\$n" -eq 0 ] || [ -e "$tmp/state/always" ]; then
    setsid "$tmp/bin/$fake" 60 &
    wait \$!
fi
INST
chmod +x "$tmp/bin/install"
# dpkg stub, so the script's "dpkg --configure -a" touches nothing real.
printf '#!/usr/bin/env bash\ntouch "%s/state/configured"\n' "$tmp" > "$tmp/bin/dpkg"
chmod +x "$tmp/bin/dpkg"

run() {
    PATH="$tmp/bin:$PATH" SUDO="" INSTALL_CMD="$tmp/bin/install" ATTEMPT_TIMEOUT=2 SETTLE_SECONDS=8 \
        APT_PROCS="$fake" APT_KILL="$fake" \
        "$script" "$@"
}

# Attempt 1 hangs holding the lock; the script must free it so attempt 2 (not slow any more) succeeds.
if ! out="$(run 2>&1)"; then fail "attempt 2 did not proceed after a timed-out attempt: $out"; fi
grep -q "Could not get lock" <<<"$out" && fail "a retry hit the lock: $out"
[ -e "$tmp/state/configured" ] || fail "dpkg --configure -a was not run after stopping apt-get"
pgrep -x "$fake" >/dev/null && fail "an apt-get is still running after the run"

# The mirror is really down: every attempt hangs, the script fails after the bounded attempts.
touch "$tmp/state/always"
if ATTEMPTS=2 run >/dev/null 2>&1; then fail "a permanently stalled install reported success"; fi

[ "$fails" -eq 0 ] && echo "test-ci-install-chromium-deps: ok"
exit "$fails"
