#!/usr/bin/env bash
# Tests scripts/ci-skip-slow.sh on a fixture git repository (part of `just validate`, docs/agent-workflow.md
# "CI and PRs", ADR 0048): skip_slow: docs-only changes skip, a code change or a mixed one runs everything, the
# ci-skip label skips on pull_request events only, an unreadable range runs everything. skip_e2e: only a change
# on a UI or API path runs e2e, a pure branch update does not, main and dispatched runs always do.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
script="$root/scripts/ci-skip-slow.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-ci-skip-slow: FAIL: $*" >&2; fails=$((fails + 1)); }

repo="$tmp/repo"
git init -q "$repo"
git -C "$repo" config user.email t@example.com
git -C "$repo" config user.name t
git -C "$repo" config commit.gpgsign false
echo base > "$repo/seed.txt"
git -C "$repo" add -A
git -C "$repo" commit -q -m base
base="$(git -C "$repo" rev-parse HEAD)"

# change <name> <file>...: a commit on top of base touching the files, as branch <name>.
change() {
    local name="$1"
    shift
    git -C "$repo" checkout -q -B "$name" "$base"
    for f in "$@"; do
        mkdir -p "$repo/$(dirname "$f")"
        echo "$name" >> "$repo/$f"
    done
    git -C "$repo" add -A
    git -C "$repo" commit -q -m "$name"
}

# check <name> <want> <event> <labels> <base>: runs the script on the checked-out branch.
check() {
    local name="$1" want="$2" event="$3" labels="$4" from="$5" out status=0
    : > "$tmp/out"
    out="$(cd "$repo" && EVENT="$event" BASE="$from" LABELS="$labels" GITHUB_OUTPUT="$tmp/out" "$script" 2>&1)" || status=$?
    [ "$status" = 0 ] || { fail "$name: exit $status: $out"; return; }
    grep -qx "skip_slow=$want" "$tmp/out" || fail "$name: want skip_slow=$want, got: $(cat "$tmp/out") ($out)"
}

change readme README.md
check "README only" true pull_request "" "$base"
check "README only, on a push" false push "" "$base"

change docs-all docs/adr/0001-x.md web/README.md api/NOTES.md
check "docs, nested markdown and tracker only" true pull_request "bug,other" "$base"

change code api/main.go
check "code" false pull_request "" "$base"
check "code, on a push" false push "" "$base"
check "code with the ci-skip label" true pull_request "bug,ci-skip" "$base"
check "code with the ci-skip label as the only one" true pull_request "ci-skip" "$base"
check "code with the ci-skip label, on a push" false push "ci-skip" "$base"
check "code with a look-alike label" false pull_request "ci-skipped,no-ci-skip" "$base"
check "code with the no-release label" false pull_request "no-release" "$base"

change mixed README.md api/main.go
check "docs and code" false pull_request "" "$base"

change workflow .github/workflows/ci.yml
check "a workflow file" false pull_request "" "$base"

change lookalike docs.txt notes.mdx
check "files that only look like docs" false pull_request "" "$base"

change docs-label README.md
check "docs with the ci-skip label" true pull_request "ci-skip" "$base"

git -C "$repo" checkout -q -B empty "$base"
# A PR branched from an older main: main moved on (code), and the PR merged with it is the checked-out HEAD.
# The PR's own head and the moved base are what the script gets; only the PR's own files count.
git -C "$repo" checkout -q -B main-moved "$base"
echo code > "$repo/moved.go"; git -C "$repo" add -A; git -C "$repo" commit -q -m moved
moved="$(git -C "$repo" rev-parse HEAD)"
git -C "$repo" checkout -q -B stale-docs "$base"
echo doc >> "$repo/README.md"; git -C "$repo" add -A; git -C "$repo" commit -q -m docs
prhead="$(git -C "$repo" rev-parse HEAD)"
git -C "$repo" merge -q --no-edit main-moved
out="$(cd "$repo" && EVENT=pull_request BASE="$moved" HEAD="$prhead" "$script")"; out="${out%%$'\n'*}"
[ "$out" = "skip_slow=true (only documentation changed)" ] || fail "docs PR on a moved base: $out"
git -C "$repo" checkout -q -B stale-code "$base"
echo code > "$repo/own.go"; git -C "$repo" add -A; git -C "$repo" commit -q -m code
out="$(cd "$repo" && EVENT=pull_request BASE="$moved" HEAD="$(git rev-parse HEAD)" "$script")"; out="${out%%$'\n'*}"
[ "$out" = "skip_slow=false (own.go is not documentation)" ] || fail "code PR on a moved base: $out"

check "no change" false pull_request "" "$base"
check "no base" false push "" ""
check "a dispatched run on main (no base)" false workflow_dispatch "" ""
check "a dispatched run ignores a stray ci-skip" false workflow_dispatch "ci-skip" ""
check "a zero base (new branch)" false push "" 0000000000000000000000000000000000000000
check "an unknown base" false push "" 1111111111111111111111111111111111111111

# The output file is optional: stdout carries the decision.
out="$(cd "$repo" && git checkout -q readme && EVENT=pull_request BASE="$base" "$script")"; out="${out%%$'\n'*}"
[ "$out" = "skip_slow=true (only documentation changed)" ] || fail "stdout without GITHUB_OUTPUT: $out"

# --- skip_e2e ---
# A fake gh: prints $FAKE_GH, the conclusion (or status) of the latest e2e check run on BEFORE that the real
# call's jq filter would give; FAKE_GH=fail makes the call fail (a token without checks: read, an API error).
mkdir -p "$tmp/bin"
cat > "$tmp/bin/gh" <<'GH'
#!/usr/bin/env bash
[ "${FAKE_GH:-}" != fail ] || exit 1
echo "${FAKE_GH:-none}"
GH
chmod +x "$tmp/bin/gh"
# The jq filter against sample payloads: the latest run by start time, its conclusion, or its status.
jqf='.check_runs | sort_by(.started_at) | last | if . == null then "none" elif .status != "completed" then .status else .conclusion end'
jqcase() { [ "$(echo "$2" | jq -r "$jqf")" = "$1" ] || fail "jq filter: want $1 for $2"; }
jqcase none '{"check_runs":[]}'
jqcase success '{"check_runs":[{"started_at":"2026-01-01T00:00:01Z","status":"completed","conclusion":"success"}]}'
jqcase success '{"check_runs":[{"started_at":"2026-01-01T00:00:01Z","status":"completed","conclusion":"failure"},{"started_at":"2026-01-01T00:00:09Z","status":"completed","conclusion":"success"}]}'
jqcase failure '{"check_runs":[{"started_at":"2026-01-01T00:00:09Z","status":"completed","conclusion":"failure"},{"started_at":"2026-01-01T00:00:01Z","status":"completed","conclusion":"success"}]}'
jqcase in_progress '{"check_runs":[{"started_at":"2026-01-01T00:00:01Z","status":"in_progress","conclusion":null}]}'
jqcase cancelled '{"check_runs":[{"started_at":"2026-01-01T00:00:01Z","status":"completed","conclusion":"cancelled"}]}'
# e2e <name> <want> <event> <labels> <base> [action] [before] [head]: skip_e2e on the checked-out branch.
e2e() {
    local name="$1" want="$2" event="$3" labels="$4" from="$5" action="${6:-}" was="${7:-}" tip="${8:-HEAD}" out status=0
    : > "$tmp/out"
    out="$(cd "$repo" && PATH="$tmp/bin:$PATH" GITHUB_REPOSITORY=o/r EVENT="$event" ACTION="$action" BASE="$from" \
        BEFORE="$was" HEAD="$tip" LABELS="$labels" GITHUB_OUTPUT="$tmp/out" "$script" 2>&1)" || status=$?
    [ "$status" = 0 ] || { fail "$name: exit $status: $out"; return; }
    grep -qx "skip_e2e=$want" "$tmp/out" || fail "$name: want skip_e2e=$want, got: $(cat "$tmp/out") ($out)"
}

# Paths that run e2e.
for f in web/src/App.tsx web/e2e/hp.spec.ts web/package.json web/vite.config.ts api/testdata/catalogue.json \
    api/internal/rest/router/router.go api/internal/rest/handler/hphdl/hphdl.go api/internal/rest/wire/buff.go \
    api/internal/render/render.go api/internal/composition/wire.go api/internal/cmd/api/main.go api/cmd/api/main.go \
    justfile .github/workflows/ci.yml scripts/ci-install-chromium-deps.sh; do
    change ui "$f"
    e2e "$f runs e2e" false pull_request "" "$base"
done
# Paths that do not.
for f in api/internal/domain/hp.go api/internal/adapters/hpdal/hpdal.go api/internal/rest/router/router_hp_test.go \
    api/internal/rest/handler/hphdl/hphdl_test.go web/src/lib/dice.test.ts web/src/App.test.tsx web/README.md \
    web/design/psionics/index.html web/ui-review/curate.mjs web/readme-shots/shots.ts \
    scripts/dev.sh scripts/ci-skip-slow.sh seeds/spells.json docs/adr/0001-x.md README.md .claude/agents/x.txt \
    .github/workflows/release.yml api/go.mod; do
    change quiet "$f"
    e2e "$f skips e2e" true pull_request "" "$base"
done
change quiet api/internal/domain/hp.go api/internal/domain/hp_test.go
e2e "Go-only change with its test skips e2e" true pull_request "" "$base"
change quiet api/internal/domain/hp.go docs/x.md
e2e "Go internals and docs skip e2e" true pull_request "" "$base"
change quiet api/internal/domain/hp.go web/src/App.tsx
e2e "Go internals and a web file run e2e" false pull_request "" "$base"
# A skipped e2e does not skip dev-smoke and image.
change quiet scripts/dev.sh
check "a script change keeps the slow jobs" false pull_request "" "$base"
change quiet docs/x.md
e2e "docs only skips e2e" true pull_request "" "$base"

# The ci-skip label skips e2e, on a pull request only; main and a dispatched run always run it.
change ui web/src/App.tsx
e2e "ci-skip skips e2e" true pull_request "ci-skip" "$base"
e2e "ci-skip on a push still runs e2e" false push "ci-skip" "$base"
e2e "a push to main runs e2e" false push "" "$base"
e2e "a push to main runs e2e even with no web change" false push "" ""
change quiet scripts/dev.sh
e2e "a push to main runs e2e on a quiet range" false push "" "$base"
e2e "a dispatched run runs e2e" false workflow_dispatch "" ""
e2e "a dispatched run with a base runs e2e" false workflow_dispatch "" "$base"

# Branch updates: main moves on and is merged into the PR; the PR's own diff is the same.
git -C "$repo" checkout -q -B main-2 "$base"
echo more > "$repo/more.go"; git -C "$repo" add -A; git -C "$repo" commit -q -m main-2
main2="$(git -C "$repo" rev-parse HEAD)"
git -C "$repo" checkout -q -B pr-ui "$base"
mkdir -p "$repo/web/src"; echo ui > "$repo/web/src/App.tsx"; git -C "$repo" add -A; git -C "$repo" commit -q -m ui
was="$(git -C "$repo" rev-parse HEAD)"
git -C "$repo" merge -q --no-edit main-2
export FAKE_GH=success
e2e "a pure branch update skips e2e" true pull_request "" "$main2" synchronize "$was" "$(git -C "$repo" rev-parse HEAD)"
e2e "the same event on a labeled action runs e2e" false pull_request "" "$main2" labeled "$was" "$(git -C "$repo" rev-parse HEAD)"
e2e "the same event on opened runs e2e" false pull_request "" "$main2" opened "$was" "$(git -C "$repo" rev-parse HEAD)"
e2e "a branch update with no before runs e2e" false pull_request "" "$main2" synchronize "" "$(git -C "$repo" rev-parse HEAD)"
e2e "a branch update with a zero before runs e2e" false pull_request "" "$main2" synchronize 0000000000000000000000000000000000000000 "$(git -C "$repo" rev-parse HEAD)"
e2e "a branch update with an unknown before runs e2e" false pull_request "" "$main2" synchronize 1111111111111111111111111111111111111111 "$(git -C "$repo" rev-parse HEAD)"
# The previous head's e2e decides: only success or skipped keeps the result.
tipnow="$(git -C "$repo" rev-parse HEAD)"
for r in success skipped; do
    FAKE_GH=$r e2e "a pure update after e2e $r skips e2e" true pull_request "" "$main2" synchronize "$was" "$tipnow"
done
for r in failure cancelled in_progress queued none fail timed_out; do
    FAKE_GH=$r e2e "a pure update after e2e $r runs e2e" false pull_request "" "$main2" synchronize "$was" "$tipnow"
done
# A Go-only PR skips whatever the previous e2e was (no e2e path), and a labeled action on it skips too.
git -C "$repo" checkout -q -B go-only "$base"
echo g > "$repo/api-only.go"; git -C "$repo" add -A; git -C "$repo" commit -q -m go-only
FAKE_GH=failure e2e "a Go-only PR skips e2e after a red previous run" true pull_request "" "$base" synchronize "$base" "$(git -C "$repo" rev-parse HEAD)"
FAKE_GH=failure e2e "a labeled event on a Go-only PR skips e2e" true pull_request "" "$base" labeled "" "$(git -C "$repo" rev-parse HEAD)"
git -C "$repo" checkout -q pr-ui
# An update that is not a merge but a real new commit on the PR.
echo ui2 >> "$repo/web/src/App.tsx"; git -C "$repo" add -A; git -C "$repo" commit -q -m ui2
e2e "a branch update plus a real PR change runs e2e" false pull_request "" "$main2" synchronize "$was" "$(git -C "$repo" rev-parse HEAD)"
# A quiet PR that gains a web file.
git -C "$repo" checkout -q -B pr-go "$base"
echo go > "$repo/quiet.go"; git -C "$repo" add -A; git -C "$repo" commit -q -m go
was="$(git -C "$repo" rev-parse HEAD)"
mkdir -p "$repo/web"; echo ui > "$repo/web/x.ts"; git -C "$repo" add -A; git -C "$repo" commit -q -m go-ui
e2e "a quiet PR gaining a web file runs e2e" false pull_request "" "$base" synchronize "$was" "$(git -C "$repo" rev-parse HEAD)"
# A PR rebased onto the new main: same patch, new commits (a force push).
git -C "$repo" checkout -q -B pr-old "$base"
mkdir -p "$repo/web"; echo ui > "$repo/web/x.ts"; git -C "$repo" add -A; git -C "$repo" commit -q -m old
was="$(git -C "$repo" rev-parse HEAD)"
git -C "$repo" checkout -q -B pr-rebased main-2
mkdir -p "$repo/web"; echo ui > "$repo/web/x.ts"; git -C "$repo" add -A; git -C "$repo" commit -q -m rebased
e2e "a rebased PR with the same diff skips e2e" true pull_request "" "$main2" synchronize "$was" "$(git -C "$repo" rev-parse HEAD)"

if [ "$fails" -ne 0 ]; then
    echo "test-ci-skip-slow: $fails failure(s)" >&2
    exit 1
fi
echo "test-ci-skip-slow: ok"
