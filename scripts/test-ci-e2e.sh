#!/usr/bin/env bash
# Tests scripts/ci-e2e.sh offline (part of `just validate`): two suites started together run one after the other,
# the exit status of the command comes through, a command past its limit is stopped and does not keep the slot, and a
# process that outlives the command does not hold the slot either. Every file is in a throwaway directory.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
script="$root/scripts/ci-e2e.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-ci-e2e: FAIL: $*" >&2; fails=$((fails + 1)); }
export E2E_LOCK="$tmp/slot.lock"

# 1. Two runs at once: start, end, start, end (not start, start, end, end).
export E2E_CMD="bash -c 'echo start >> $tmp/order; sleep 1; echo end >> $tmp/order'"
"$script" >"$tmp/a.log" 2>&1 &
pa=$!
"$script" >"$tmp/b.log" 2>&1 &
pb=$!
wait "$pa" "$pb"
if [ "$(tr '\n' ' ' <"$tmp/order")" != "start end start end " ]; then
    fail "two runs overlapped: $(tr '\n' ' ' <"$tmp/order")"
fi
grep -q 'e2e slot taken after' "$tmp/a.log" || fail "the log does not say when the slot was taken"
grep -q ' cpus, load ' "$tmp/a.log" || fail "the log does not show the host's cpus and load"

# 2. The command's status comes through.
E2E_CMD="bash -c 'exit 7'" "$script" >/dev/null 2>&1 && status=0 || status=$?
[ "$status" = 7 ] || fail "status 7 became $status"

# 3. Past the limit the command is stopped (124) and the slot is free for the next run; a child that outlives
# the command (as a leftover Chromium would) does not hold it.
E2E_TIMEOUT=1s E2E_CMD="bash -c '(sleep 30 &) ; sleep 30'" "$script" >/dev/null 2>&1 && status=0 || status=$?
[ "$status" = 124 ] || fail "a command past its limit ended with $status, not 124"
E2E_CMD="true" timeout 20 "$script" >/dev/null 2>&1 || fail "the next run did not get the slot after a timed-out one"

# 4. A run waiting for the slot does not count the wait against the limit: the holder takes 3 s, the waiter's
# limit is 2 s and its own command takes 1 s.
(E2E_CMD="sleep 3" "$script" >/dev/null 2>&1 &)
sleep 0.5
E2E_TIMEOUT=2s E2E_CMD="sleep 1" "$script" >/dev/null 2>&1 || fail "the wait for the slot counted against the limit"

[ "$fails" = 0 ] && echo "test-ci-e2e: ok"
exit "$fails"
