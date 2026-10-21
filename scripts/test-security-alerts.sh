#!/usr/bin/env bash
# Tests .claude/scripts/security-alerts.sh and its use by ci-watch.sh (part of `just validate`) with a stub
# gh and a stub Linear script: nothing here calls GitHub or Linear. Checks: new alerts of the three kinds are
# filed Urgent in Todo with link/severity/rule/location; an alert already in a title (any state) is not refiled
# ("#1" is not "#12"); a second run prints nothing; the secret value never reaches Linear; a gh or Linear
# failure is reported (exit 1) without hiding the alerts that could be filed; ci-watch.sh exits on a new
# alert report, keeps its CI exit semantics and keeps waiting when nothing is new.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
S="$root/.claude/scripts"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-security-alerts: FAIL: $*" >&2; fails=$((fails + 1)); }

# --- stub gh: serves fixtures from $FIX; GH_FAIL=<path fragment> makes that endpoint fail.
cat > "$tmp/gh" <<'SH'
#!/usr/bin/env bash
[ "$1" = api ] || exit 2
shift
args="$*"
[ -z "${GH_FAIL:-}" ] || case "$args" in *"$GH_FAIL"*) echo "HTTP 403: stub failure" >&2; exit 1;; esac
case "$args" in
  *dependabot/alerts*) cat "$FIX/dependabot.json";;
  *code-scanning/alerts*) cat "$FIX/code-scanning.json";;
  *secret-scanning/alerts/*/locations*) echo "src/config/.env";;
  *secret-scanning/alerts*) cat "$FIX/secret-scanning.json";;
  *) exit 2;;
esac
SH
cat > "$tmp/linear" <<'SH'
#!/usr/bin/env bash
echo "$1 $2" >> "$LOG"
case "$1" in
  *"issues(first"*) cat "$TITLES";;
  *"teams(filter"*) echo '{"data":{"teams":{"nodes":[{"id":"team-1","states":{"nodes":[{"id":"st-bl","name":"Backlog"},{"id":"st-todo","name":"Todo"}]}}]},"projects":{"nodes":[{"id":"proj-1"}]}}}';;
  *issueCreate*)
    [ -z "${LINEAR_FAIL:-}" ] || { echo "linear: HTTP 500" >&2; exit 1; }
    n=$(grep -c issueCreate "$LOG"); echo "{\"data\":{\"issueCreate\":{\"success\":true,\"issue\":{\"identifier\":\"TST-$((900 + n))\"}}}}";;
  *) exit 2;;
esac
SH
chmod +x "$tmp/gh" "$tmp/linear"
export FIX="$tmp/fix" LOG="$tmp/log" TITLES="$tmp/titles.json" GH="$tmp/gh" LINEAR_SH="$tmp/linear"
mkdir "$tmp/fix"
cat > "$FIX/dependabot.json" <<'J'
[{"number":56,"state":"open","html_url":"https://github.com/o/r/security/dependabot/56","dependency":{"manifest_path":"web/package-lock.json","package":{"name":"braces"}},
  "security_advisory":{"ghsa_id":"GHSA-aaaa","severity":"high","summary":"stack exhaustion"},"security_vulnerability":{"package":{"name":"braces"},"vulnerable_version_range":"<= 3.0.3","first_patched_version":null}},
 {"number":57,"state":"open","html_url":"https://github.com/o/r/security/dependabot/57","dependency":{"manifest_path":"api/go.mod","package":{"name":"example.com/lib"}},
  "security_advisory":{"ghsa_id":"GHSA-bbbb","severity":"critical","summary":"rce"},"security_vulnerability":{"package":{"name":"example.com/lib"},"vulnerable_version_range":"< 1.2.0","first_patched_version":{"identifier":"1.2.0"}}}]
J
cat > "$FIX/code-scanning.json" <<'J'
[{"number":1,"html_url":"https://github.com/o/r/security/code-scanning/1","rule":{"id":"js/identity-replacement","severity":"warning","description":"Replacement of a substring with itself"},"most_recent_instance":{"location":{"path":"web/a.test.ts","start_line":214},"message":{"text":"This replaces x with itself"}}},
 {"number":12,"html_url":"https://github.com/o/r/security/code-scanning/12","rule":{"id":"go/sql-injection","severity":"error","security_severity_level":"high","description":"Query built from user input"},"most_recent_instance":{"location":{"path":"api/db.go","start_line":9},"message":{"text":"flows here"}}}]
J
cat > "$FIX/secret-scanning.json" <<'J'
[{"number":7,"state":"open","html_url":"https://github.com/o/r/security/secret-scanning/7","secret_type":"github_pat","secret_type_display_name":"GitHub Personal Access Token","secret":"ghp_SUPERSECRETVALUE123"}]
J
titles() { jq -nc --argjson t "$1" '{data:{issues:{nodes:($t|map({title:.})),pageInfo:{hasNextPage:false,endCursor:null}}}}' > "$TITLES"; }
titles '["Security alert: dependabot #56 braces (web/package-lock.json)","Security alert: code-scanning #1 js/identity-replacement (web/a.test.ts)","Unrelated"]'
: > "$LOG"

# --- 1: files exactly the three unseen alerts
out="$("$S/security-alerts.sh")" || fail "run 1 exited $?"
[ "$(wc -l <<<"$out")" = 3 ] || fail "expected 3 FILED lines, got: $out"
grep -qxF 'FILED TST-901 Security alert: dependabot #57 example.com/lib (api/go.mod)' <<<"$out" || fail "dependabot line: $out"
grep -qxF 'FILED TST-902 Security alert: code-scanning #12 go/sql-injection (api/db.go:9)' <<<"$out" || fail "code-scanning #12 (a #1 prefix match must not hide it): $out"
grep -qxF 'FILED TST-903 Security alert: secret-scanning #7 GitHub Personal Access Token (src/config/.env)' <<<"$out" || fail "secret line: $out"
n="$(grep -c issueCreate "$LOG")"; [ "$n" = 3 ] || fail "expected 3 creations, got $n"
while IFS= read -r line; do
  v="$(sed 's/^.*issueCreate(input: \$i) { success issue { identifier } } } //' <<<"$line")"
  jq -e '.i.priority == 1 and .i.teamId == "team-1" and .i.projectId == "proj-1" and .i.stateId == "st-todo"' <<<"$v" >/dev/null || fail "create input wrong: $v"
done < <(grep issueCreate "$LOG")
desc57="$(grep issueCreate "$LOG" | grep 'dependabot #57' | sed 's/^.*} } } //' | jq -r .i.description)"
for want in 'critical' 'https://github.com/o/r/security/dependabot/57' 'GHSA-bbbb' 'api/go.mod' '1.2.0'; do
  grep -qF "$want" <<<"$desc57" || fail "dependabot description lacks $want"
done
grep -qF 'ghp_SUPERSECRETVALUE123' "$LOG" && fail "the secret value reached Linear"
grep issueCreate "$LOG" | grep 'secret-scanning #7' | sed 's/^.*} } } //' | jq -r .i.description | grep -qF 'security/secret-scanning/7' || fail "secret alert link missing"

# --- 2: everything is filed now: silence, no creation
titles '["Security alert: dependabot #56 braces (x)","Security alert: dependabot #57 y (z)","Security alert: code-scanning #1 r (p)","Security alert: code-scanning #12 r (p)","Security alert: secret-scanning #7 t (p)"]'
: > "$LOG"
out="$("$S/security-alerts.sh")" || fail "run 2 exited $?"
[ -z "$out" ] || fail "second run printed: $out"
grep -q issueCreate "$LOG" && fail "second run created an issue"

# --- 3: a gh failure is reported (exit 1); the alerts of the other kinds are still filed
titles '[]'; : > "$LOG"
rc=0; out="$(GH_FAIL=code-scanning "$S/security-alerts.sh")" || rc=$?
[ "$rc" = 1 ] || fail "gh failure exit $rc, want 1"
grep -q '^ERROR security-alerts: gh api code-scanning/alerts' <<<"$out" || fail "gh failure not reported: $out"
grep -q '^FILED .* dependabot #56' <<<"$out" || fail "other kinds not filed after a gh failure: $out"
# --- 4: a Linear failure is reported, exit 1
rc=0; out="$(LINEAR_FAIL=1 "$S/security-alerts.sh")" || rc=$?
[ "$rc" = 1 ] || fail "linear failure exit $rc, want 1"
grep -q '^ERROR security-alerts: Linear create' <<<"$out" || fail "linear failure not reported: $out"
grep -q '^FILED' <<<"$out" && fail "FILED printed although every creation failed"

# --- 5: ci-watch.sh, with a stub ci-state (exit code from $CI_RC, text from $CI_TEXT) and a stub alerts script
w="$tmp/w/.claude/scripts"; mkdir -p "$w"
cp "$S/ci-watch.sh" "$w/"
printf '#!/usr/bin/env bash\necho "$CI_TEXT"; exit "$CI_RC"\n' > "$w/ci-state.sh"
printf '#!/usr/bin/env bash\n[ -f "$ALERT_FILE" ] && { cat "$ALERT_FILE"; exit "${ALERT_RC:-0}"; }\nexit 0\n' > "$w/security-alerts.sh"
chmod +x "$w"/*.sh
export SUPERVISOR_DIR="$tmp/sup" CI_WATCH_INTERVAL=0.1 ALERT_FILE="$tmp/alert"
mkdir -p "$SUPERVISOR_DIR"
watch() { timeout 5 "$w/ci-watch.sh" 2>&1; }
rm -f "$ALERT_FILE"
out="$(CI_RC=20 CI_TEXT='RED main CI run 1 (failure) u' watch)"
[ "$out" = $'RED main CI run 1 (failure) u\nexit=20' ] || fail "red without alerts changed: $out"
out="$(CI_RC=0 CI_TEXT='' watch)"
[ "$out" = $'\nexit=0' ] || fail "green without alerts changed: $out"
echo 'FILED TST-9 Security alert: x' > "$ALERT_FILE"
out="$(CI_RC=0 CI_TEXT='' watch)"
[ "$out" = $'FILED TST-9 Security alert: x\n\nexit=0' ] || fail "alert on idle CI: $out"
out="$(CI_RC=10 CI_TEXT='PENDING PR #5 @abc: 1 check(s) running' watch)"
[ "$out" = $'FILED TST-9 Security alert: x\nPENDING PR #5 @abc: 1 check(s) running\nexit=10' ] || fail "alert while CI pending: $out"
echo 'ERROR security-alerts: gh api boom' > "$ALERT_FILE"
out="$(ALERT_RC=1 CI_RC=20 CI_TEXT='RED main CI run 1 (failure) u' watch)"
[ "$out" = $'ERROR security-alerts: gh api boom\nRED main CI run 1 (failure) u\nexit=20' ] || fail "alert error must not hide CI: $out"
rm -f "$ALERT_FILE"
rc=0; CI_RC=10 CI_TEXT='PENDING PR #5 @abc: 1 check(s) running' CI_WATCH_INTERVAL=0.1 timeout 1 "$w/ci-watch.sh" >/dev/null 2>&1 || rc=$?
[ "$rc" = 124 ] || fail "ci-watch must keep waiting with pending CI and nothing new (exit $rc)"

[ "$fails" = 0 ] && { echo "test-security-alerts: ok"; exit 0; }
exit 1
