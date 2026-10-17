#!/usr/bin/env bash
# Tests the backup and restore-drill scripts without fly, docker or a database (part of `just validate`,
# ADR 0024 step 9), with fly, docker, psql, pg_dump and pg_restore stubbed:
#   - db-counts-compare.sh: equal counts pass; a differing, a missing and an extra table, an empty listing
#     each fail and are named;
#   - db-backup.sh: refuses an unknown environment and a missing password before touching anything; reads
#     the password from the environment or the operator's file and never prints it nor puts it in an
#     argument; keeps the dump (mode 600, folder 700) with the counts beside it; keeps nothing when pg_dump
#     fails or the dump is unreadable; refuses a folder inside a git repository that does not ignore it;
#     the repository's own .backups/ is ignored;
#   - db-restore-drill.sh: matching counts exit 0, a count that differs exits 1, and in both cases (and when
#     the restore itself fails) the scratch database is dropped and the compose project removed with its
#     volume; with no dump or no counts.txt it refuses before starting docker.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fails=0
fail() { echo "test-db-backup: FAIL: $*" >&2; fails=$((fails + 1)); }
secret="s3cret-$$-sentinel"

calls="$tmp/calls"
stubs="$tmp/bin"
mkdir -p "$stubs"
cat > "$stubs/fly" <<STUB
#!/bin/sh
echo "fly \$*" >> "$calls"
# fly proxy <local>:5432 ...: listen on the local port like the real proxy
exec python3 -I -c 'import socket,sys,time; s=socket.socket(); s.bind(("127.0.0.1", int(sys.argv[1].split(":")[0]))); s.listen(); time.sleep(300)' "\$2"
STUB
cat > "$stubs/pg_dump" <<STUB
#!/bin/sh
echo "pg_dump \$*" >> "$calls"
echo "pg_dump PGPASSWORD=\${PGPASSWORD:-unset}" >> "$tmp/env"
[ ! -f "$tmp/dump-fails" ] || exit 1
for a in "\$@"; do case "\$a" in --file=*) printf 'PGDMP fake' > "\${a#--file=}" ;; esac; done
STUB
cat > "$stubs/pg_restore" <<STUB
#!/bin/sh
echo "pg_restore \$*" >> "$calls"
case "\$*" in
    --list*) [ ! -f "$tmp/dump-unreadable" ] ;;
    *) [ ! -f "$tmp/restore-fails" ] ;;
esac
STUB
cat > "$stubs/psql" <<STUB
#!/bin/sh
echo "psql \$*" >> "$calls"
case "\$*" in *" -c "*) exit 0 ;; esac
case "\$(cat)" in *gexec*) cat "\$COUNTS_FIXTURE" ;; esac
STUB
cat > "$stubs/docker" <<STUB
#!/bin/sh
echo "docker \$* [project=\$COMPOSE_PROJECT_NAME]" >> "$calls"
[ ! -f "$tmp/docker-down" ] || exit 1
STUB
chmod +x "$stubs"/*
export PATH="$stubs:$PATH"
export FLY="$stubs/fly"

reset() { : > "$calls"; : > "$tmp/env"; rm -f "$tmp"/dump-fails "$tmp"/dump-unreadable "$tmp"/restore-fails "$tmp"/docker-down; }
count() { grep -c -- "$1" "$calls" || true; }

# --- db-counts-compare.sh ---
printf 'items 6\nspells 35\nweapons 6\n' > "$tmp/want"
out="$("$root/scripts/db-counts-compare.sh" "$tmp/want" "$tmp/want" 2>&1)" || fail "compare: equal counts refused: $out"
for case in "differs:items 6\nspells 36\nweapons 6\n:spells.*DIFFERS" \
            "missing:items 6\nspells 35\n:weapons.*MISSING" \
            "extra:items 6\nspells 35\nweapons 6\nxp 0\n:xp.*EXTRA"; do
    name="${case%%:*}"; rest="${case#*:}"; body="${rest%:*}"; pat="${rest##*:}"
    printf "$body" > "$tmp/got"
    if out="$("$root/scripts/db-counts-compare.sh" "$tmp/want" "$tmp/got" 2>&1)"; then fail "compare ($name): accepted"; fi
    grep -q "$pat" <<<"$out" || fail "compare ($name): not named: $out"
done
: > "$tmp/empty"
! "$root/scripts/db-counts-compare.sh" "$tmp/empty" "$tmp/want" >/dev/null 2>&1 || fail "compare: empty expected counts accepted"

# --- db-backup.sh ---
bk="$tmp/backups"
export BACKUP_DIR="$bk" COUNTS_FIXTURE="$tmp/want"
unset PGPASSWORD CRITICAL_PASSWORD_DIR || true
export HOME="$tmp/home"; mkdir -p "$HOME"
reset
! "$root/scripts/db-backup.sh" qa >/dev/null 2>&1 || fail "backup: unknown environment accepted"
! "$root/scripts/db-backup.sh" staging >/dev/null 2>&1 || fail "backup: no password accepted"
[ ! -s "$calls" ] || fail "backup: refused runs still called a tool: $(cat "$calls")"
[ ! -e "$bk" ] || [ -z "$(ls -A "$bk")" ] || fail "backup: refused runs left a folder"

out="$(PGPASSWORD="$secret" "$root/scripts/db-backup.sh" staging 2>&1)" || fail "backup: failed: $out"
dir="$(ls -d "$bk"/staging-* 2>/dev/null | head -n1)"
[ -f "$dir/critical_staging.dump" ] && [ -s "$dir/counts.txt" ] || fail "backup: dump or counts missing in '$dir'"
[ "$(stat -c %a "$dir/critical_staging.dump")" = 600 ] || fail "backup: dump is not mode 600"
[ "$(stat -c %a "$dir")" = 700 ] || fail "backup: folder is not mode 700"
grep -q 'proxy .*tournesol-db.flycast -a tournesol-db' "$calls" || fail "backup: no fly proxy to the cluster: $(cat "$calls")"
grep -q -- "--format=custom" "$calls" || fail "backup: not the custom format"
grep -q "PGPASSWORD=$secret" "$tmp/env" || fail "backup: password not passed to pg_dump through the environment"
! grep -q "$secret" "$calls" || fail "backup: password in a command line"
! grep -q "$secret" <<<"$out" || fail "backup: password printed"
pgrep -f "listen().*time.sleep(300)" >/dev/null && fail "backup: the fly proxy was left running" || true

# the operator's password file
reset; rm -rf "$bk"
mkdir -p "$tmp/pw"; printf '%s\n' "$secret" > "$tmp/pw/critical_prod.password"
CRITICAL_PASSWORD_DIR="$tmp/pw" "$root/scripts/db-backup.sh" prod >/dev/null 2>&1 || fail "backup: password file refused"
grep -q "PGPASSWORD=$secret" "$tmp/env" || fail "backup: password file not used"

# failures keep nothing
for broken in dump-fails dump-unreadable; do
    reset; rm -rf "$bk"; touch "$tmp/$broken"
    if PGPASSWORD="$secret" "$root/scripts/db-backup.sh" staging >/dev/null 2>&1; then fail "backup ($broken): reported success"; fi
    [ -z "$(ls -A "$bk" 2>/dev/null)" ] || fail "backup ($broken): a folder was kept"
done

# a folder inside a repository that does not ignore it
reset; rm -rf "$bk"
git init -q "$tmp/repo"
if BACKUP_DIR="$tmp/repo/out" PGPASSWORD="$secret" "$root/scripts/db-backup.sh" staging >/dev/null 2>&1; then fail "backup: dumped into an unignored repository folder"; fi
[ -z "$(find "$tmp/repo/out" -type f 2>/dev/null)" ] || fail "backup: left files in the repository"
git -C "$root" check-ignore -q .backups/staging-x/critical_staging.dump || fail "the repository does not ignore .backups/"

# --- db-restore-drill.sh ---
unset BACKUP_DIR
mkdir -p "$tmp/dump" && printf 'PGDMP fake' > "$tmp/dump/critical_prod.dump"
cp "$tmp/want" "$tmp/dump/counts.txt"
down="docker compose down --volumes"
cleaned() { # cleaned <what>: scratch dropped and project removed
    [ "$(count 'DROP DATABASE IF EXISTS critical_drill_')" = 1 ] || fail "drill ($1): scratch database not dropped"
    [ "$(count "$down")" = 1 ] || fail "drill ($1): compose project not removed with its volume"
    grep -q 'project=criticaldrill' "$calls" || fail "drill ($1): not its own compose project"
}
reset; COUNTS_FIXTURE="$tmp/want"
out="$("$root/scripts/db-restore-drill.sh" "$tmp/dump/critical_prod.dump" 2>&1)" || fail "drill: matching counts failed: $out"
cleaned match
grep -q 'restore --no-owner' "$calls" || grep -q 'pg_restore --no-owner' "$calls" || fail "drill: no restore ran"

reset; printf 'items 6\nspells 34\nweapons 6\n' > "$tmp/other"
if COUNTS_FIXTURE="$tmp/other" "$root/scripts/db-restore-drill.sh" "$tmp/dump/critical_prod.dump" >"$tmp/o" 2>&1; then fail "drill: differing counts passed"; fi
grep -q 'spells.*DIFFERS' "$tmp/o" || fail "drill: difference not named"
cleaned differ

reset; touch "$tmp/restore-fails"
if COUNTS_FIXTURE="$tmp/want" "$root/scripts/db-restore-drill.sh" "$tmp/dump/critical_prod.dump" >/dev/null 2>&1; then fail "drill: failed restore passed"; fi
cleaned restore-fails

# the newest prod dump is the default
reset; mkdir -p "$tmp/bk2/prod-20260101T000000Z" "$tmp/bk2/prod-20260202T000000Z"
for d in "$tmp"/bk2/prod-*; do printf x > "$d/critical_prod.dump"; cp "$tmp/want" "$d/counts.txt"; done
BACKUP_DIR="$tmp/bk2" COUNTS_FIXTURE="$tmp/want" "$root/scripts/db-restore-drill.sh" >/dev/null 2>&1 || fail "drill: default dump failed"
grep -q "prod-20260202T000000Z/critical_prod.dump" "$calls" || fail "drill: not the newest dump: $(grep restore "$calls")"

# refusals happen before docker starts
reset
! "$root/scripts/db-restore-drill.sh" "$tmp/nope.dump" >/dev/null 2>&1 || fail "drill: missing dump accepted"
printf x > "$tmp/lonely.dump"
! "$root/scripts/db-restore-drill.sh" "$tmp/lonely.dump" >/dev/null 2>&1 || fail "drill: dump without counts accepted"
! BACKUP_DIR="$tmp/none" "$root/scripts/db-restore-drill.sh" >/dev/null 2>&1 || fail "drill: no dump at all accepted"
[ "$(count '^docker')" = 0 ] || fail "drill: docker started for a refused run"

[ "$fails" = 0 ] || { echo "test-db-backup: $fails failure(s)" >&2; exit 1; }
echo "test-db-backup: ok"
