#!/usr/bin/env bash
# Every exported type of the domain packages (production code) must have a glossary entry, or a
# reason in scripts/glossary-allowlist.txt (ADR 0036 step 1).
# A type matches an entry when its name equals the entry's term, ignoring case, spaces and hyphens
# ("Class group" matches ClassGroup), or a name in the entry's `_Code_: A, B` line.
# Allowlist: one `Name  # reason` per line; `*Suffix` and `Prefix*` match by pattern.
# Usage: check-glossary.sh [domain dir [glossary file [allowlist file]]]   (defaults: the repo's)
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
domain=${1:-$root/api/internal/domain}
glossary=${2:-$root/GLOSSARY.md}
allowlist=${3:-$root/scripts/glossary-allowlist.txt}

norm() { tr 'A-Z' 'a-z' | tr -cd 'a-z0-9\n'; }

# Normalised terms and aliases of the glossary.
{
    { grep -oE '^\*\*[^*]+\*\*' "$glossary" || true; } | tr -d '*'
    { grep -E '^_Code_:' "$glossary" || true; } | sed 's/^_Code_://' | tr ',' '\n'
} | norm | sort -u >"${TMPDIR:-/tmp}/glossary-terms.$$"
types_file="${TMPDIR:-/tmp}/glossary-types.$$"
trap 'rm -f "${TMPDIR:-/tmp}/glossary-terms.$$" "$types_file"' EXIT

# Allowlist names (patterns kept as written).
allowed=$(sed -e 's/#.*//' -e 's/[[:space:]]*$//' -e 's/^[[:space:]]*//' "$allowlist" | grep -v '^$' || true)

# Exported type names: `type X` and the members of `type ( ... )` blocks.
types=$(find "$domain" -name '*.go' ! -name '*_test.go' ! -path '*/mocks/*' -print0 | xargs -0 awk '
    /^type \(/ { block = 1; next }
    block && /^\)/ { block = 0 }
    block && match($0, /^\t[A-Z][A-Za-z0-9_]*/) { print substr($0, 2, RLENGTH - 1); next }
    /^type [A-Z]/ { name = $2; sub(/\[.*/, "", name); print name }
' | sort -u)

# Membership is tested against a file: `printf ... | grep -q` under pipefail reports a name that is
# present as absent when grep exits on the match before printf has finished writing (SIGPIPE).
printf '%s\n' "$types" >"$types_file"

missing=()
for t in $types; do
    n=$(printf '%s\n' "$t" | norm)
    grep -qxF "$n" "${TMPDIR:-/tmp}/glossary-terms.$$" && continue
    ok=0
    while IFS= read -r a; do
        [ -z "$a" ] && continue
        # shellcheck disable=SC2254
        case "$t" in $a) ok=1; break ;; esac
    done <<<"$allowed"
    [ $ok = 1 ] || missing+=("$t")
done

# A stale allowlist line (an exact name that is no longer a domain type) is an error too.
stale=()
while IFS= read -r a; do
    [ -z "$a" ] && continue
    case "$a" in *'*'*) continue ;; esac
    grep -qxF "$a" "$types_file" || stale+=("$a")
done <<<"$allowed"
if [ ${#stale[@]} -gt 0 ]; then
    printf '%s\n' "${stale[@]}"
    echo "error: allowlist name(s) above are no domain type any more: remove them from scripts/glossary-allowlist.txt."
    exit 1
fi

if [ ${#missing[@]} -gt 0 ]; then
    printf '%s\n' "${missing[@]}"
    echo "error: exported domain type(s) above have no GLOSSARY.md entry. Add the term (and its"
    echo "docs/domain-model.md entry), a '_Code_: Name' alias line, or an allowlist line with a reason"
    echo "in scripts/glossary-allowlist.txt (ADR 0036 step 1)."
    exit 1
fi
