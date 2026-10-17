#!/usr/bin/env bash
# Tests the release build's scripts without the network, fly or a registry (part of `just validate`):
#   - release-secrets.sh: all set -> ready=true; one missing -> ready=false, a notice naming it, exit 0
#     (the green no-op of main before the secrets exist); no value is ever printed.
#   - release-catalogue.sh: from a local rulebooks repo through an ssh-agent holding a generated key,
#     stdout is the rulebooks sha only (never the catalogue nor the key), catalogue.json is copied;
#     no key or no dist/catalogue.json fails and leaves no file.
#   - release.yml: every step after the secrets check runs only on ready == 'true' (no flyctl, no fetch,
#     no build when a secret is missing).
#   - release-image.sh: with docker and fly stubbed, one build labelled with both commits and one push
#     per app tagged with the full commit sha; a dirty tree or a short rulebooks sha builds nothing.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-release-scripts: FAIL: $*" >&2; fails=$((fails + 1)); }
git_q() { git -c user.name=t -c user.email=t@t -c init.defaultBranch=main -c commit.gpgsign=false "$@" >/dev/null; }

# --- release-secrets.sh ---
out="$tmp/out"; : > "$out"
log="$(A=secret-a B=secret-b GITHUB_OUTPUT="$out" "$root/scripts/release-secrets.sh" A B)"
[ "$(cat "$out")" = "ready=true" ] || fail "secrets set: output is '$(cat "$out")'"
[ -z "$log" ] || fail "secrets set: printed '$log'"

: > "$out"
if log="$(A=secret-a B= GITHUB_OUTPUT="$out" "$root/scripts/release-secrets.sh" A B)"; then
    [ "$(cat "$out")" = "ready=false" ] || fail "secret missing: output is '$(cat "$out")'"
    [[ "$log" == "::notice "*"B not set"* ]] || fail "secret missing: no notice naming B: '$log'"
    [[ "$log" != *secret-a* ]] || fail "secret missing: printed a secret value"
else
    fail "secret missing: exited non-zero (main would go red)"
fi

# --- release.yml: the steps after the secrets check are gated on its output ---
ungated="$(awk '
    /^      - / { if (block != "" && after && block !~ /if: steps\.secrets\.outputs\.ready == .true./) print head
                  block = ""; head = $0; if (seen) after = 1 }
    /name: Check the release secrets/ { seen = 1 }
    { block = block "\n" $0 }
    END { if (block != "" && after && block !~ /if: steps\.secrets\.outputs\.ready == .true./) print head }
' "$root/.github/workflows/release.yml")"
grep -q 'name: Check the release secrets' "$root/.github/workflows/release.yml" || fail "release.yml: no secrets check step"
[ "$(grep -c "if: steps.secrets.outputs.ready == 'true'" "$root/.github/workflows/release.yml")" -ge 2 ] \
    || fail "release.yml: the flyctl and build steps are not gated"
[ -z "$ungated" ] || fail "release.yml: steps after the secrets check run without a secret: $ungated"

# --- release-catalogue.sh ---
books="$tmp/rulebooks"
mkdir -p "$books/dist"
echo '{"marker":"catalogue-content"}' > "$books/dist/catalogue.json"
git_q -C "$books" init
git_q -C "$books" add -A
git_q -C "$books" commit -m catalogue
git -C "$books" config uploadpack.allowFilter true
sha="$(git -C "$books" rev-parse HEAD)"
ssh-keygen -q -t ed25519 -N '' -C test -f "$tmp/key"
key="$(cat "$tmp/key")"

stdout="$(RULEBOOKS_DEPLOY_KEY="$key" RULEBOOKS_REMOTE="file://$books" "$root/scripts/release-catalogue.sh" "$tmp/cat" 2>"$tmp/err")" \
    || fail "catalogue: exited non-zero: $(cat "$tmp/err")"
[ "$stdout" = "$sha" ] || fail "catalogue: stdout is not exactly the rulebooks sha: '$stdout'"
cmp -s "$books/dist/catalogue.json" "$tmp/cat/catalogue.json" || fail "catalogue: catalogue.json not copied"
[ "$(stat -c %a "$tmp/cat/catalogue.json" 2>/dev/null)" = 644 ] || fail "catalogue: catalogue.json is not mode 644"
! grep -q -e catalogue-content -e PRIVATE <<<"$stdout" || fail "catalogue: printed the catalogue or the key on stdout"
! grep -q -e catalogue-content -e PRIVATE "$tmp/err" || fail "catalogue: printed the catalogue or the key on stderr"

if RULEBOOKS_DEPLOY_KEY= RULEBOOKS_REMOTE="file://$books" "$root/scripts/release-catalogue.sh" "$tmp/nokey" >/dev/null 2>&1; then
    fail "catalogue: succeeded without a key"
fi
[ ! -e "$tmp/nokey/catalogue.json" ] || fail "catalogue: wrote a file without a key"

git_q -C "$books" rm -q dist/catalogue.json
git_q -C "$books" commit -m "no catalogue"
if RULEBOOKS_DEPLOY_KEY="$key" RULEBOOKS_REMOTE="file://$books" "$root/scripts/release-catalogue.sh" "$tmp/none" >/dev/null 2>&1; then
    fail "catalogue: succeeded without dist/catalogue.json"
fi
[ -z "$(ls -A "$tmp/none" 2>/dev/null)" ] || fail "catalogue: left a file when dist/catalogue.json is missing"

# --- release-image.sh ---
stubs="$tmp/bin"
mkdir -p "$stubs"
printf '#!/bin/sh\necho "docker $*" >> "%s"\n' "$tmp/calls" > "$stubs/docker"
printf '#!/bin/sh\necho "fly $*" >> "%s"\n' "$tmp/calls" > "$stubs/fly"
chmod +x "$stubs/docker" "$stubs/fly"
src="$tmp/src"
mkdir -p "$src"
echo x > "$src/file"
git_q -C "$src" init
git_q -C "$src" add -A
git_q -C "$src" commit -m src
commit="$(git -C "$src" rev-parse HEAD)"

(cd "$src" && PATH="$stubs:$PATH" FLY="$stubs/fly" "$root/scripts/release-image.sh" "$tmp/cat" "$sha" app-a app-b >/dev/null) \
    || fail "image: exited non-zero"
calls="$(cat "$tmp/calls" 2>/dev/null || true)"
[ "$(grep -c '^docker build ' <<<"$calls")" = 1 ] || fail "image: not exactly one build: $calls"
grep -q -- "--build-context catalogue=$tmp/cat " <<<"$calls" || fail "image: catalogue context not passed"
grep -q -- "--build-arg COMMIT=$commit " <<<"$calls" || fail "image: COMMIT build arg not the full sha"
grep -q -- "--label org.opencontainers.image.revision=$commit " <<<"$calls" || fail "image: no commit label"
grep -q -- "--label critical.rulebooks.revision=$sha " <<<"$calls" || fail "image: no rulebooks label"
grep -qx "fly auth docker" <<<"$calls" || fail "image: no fly auth docker"
for app in app-a app-b; do
    grep -qx "docker push registry.fly.io/$app:$commit" <<<"$calls" || fail "image: no push of $app:<full sha>"
done

: > "$tmp/calls"
(cd "$src" && PATH="$stubs:$PATH" FLY="$stubs/fly" "$root/scripts/release-image.sh" "$tmp/cat" "${sha:0:7}" app-a >/dev/null 2>&1) \
    && fail "image: accepted a short rulebooks sha"
echo y > "$src/file"
(cd "$src" && PATH="$stubs:$PATH" FLY="$stubs/fly" "$root/scripts/release-image.sh" "$tmp/cat" "$sha" app-a >/dev/null 2>&1) \
    && fail "image: built a dirty work tree"
[ ! -s "$tmp/calls" ] || fail "image: called docker or fly on a refused input: $(cat "$tmp/calls")"

[ "$fails" = 0 ] || exit 1
echo "--- Release scripts ---"
