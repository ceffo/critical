#!/usr/bin/env bash
# Compares two "<table> <count>" listings (scripts/db-table-counts.sh): the counts recorded when a dump was
# taken and the counts of a database restored from it. Prints one line per table, `ok`, `DIFFERS`, `MISSING`
# (in the restored database) or `EXTRA` (only there), then the verdict; exits 1 unless every table is `ok`
# and there is at least one. Names and counts only.
#
#   scripts/db-counts-compare.sh <expected file> <restored file>
set -euo pipefail

expected="${1:?usage: db-counts-compare.sh <expected file> <restored file>}"
restored="${2:?usage: db-counts-compare.sh <expected file> <restored file>}"
[ -s "$expected" ] || { echo "db-counts-compare: no counts in $expected" >&2; exit 1; }

LC_ALL=C awk '
    FNR == NR { want[$1] = $2; next }
    { got[$1] = $2 }
    END {
        bad = 0; n = 0
        for (t in want) names[t] = 1
        for (t in got) names[t] = 1
        m = 0
        for (t in names) order[++m] = t
        # byte-order listing without relying on asort (portable awk)
        for (i = 2; i <= m; i++) { v = order[i]; j = i - 1; while (j > 0 && order[j] > v) { order[j + 1] = order[j]; j-- } order[j + 1] = v }
        for (i = 1; i <= m; i++) {
            t = order[i]
            if (!(t in got)) { verdict = "MISSING"; bad++ }
            else if (!(t in want)) { verdict = "EXTRA"; bad++ }
            else if (want[t] != got[t]) { verdict = "DIFFERS"; bad++ }
            else { verdict = "ok"; n++ }
            printf "%-24s dump=%-8s restored=%-8s %s\n", t, (t in want) ? want[t] : "-", (t in got) ? got[t] : "-", verdict
        }
        if (bad) { printf "restore drill: FAIL, %d of %d tables differ\n", bad, m; exit 1 }
        printf "restore drill: every one of the %d tables has the dump-time row count\n", n
    }' "$expected" "$restored"
