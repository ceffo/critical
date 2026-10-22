#!/usr/bin/env bash
# Test of priest-effects-audit.py on a throwaway catalogue and mapping: the seed entries and the report
# are written, a rerun is a no-op and --check says so, the Wizard entries stay byte for byte, a Priest
# spell in no class or in two classes is refused, and a changed mapping replaces the entry.
set -euo pipefail
audit="$(dirname "$0")/priest-effects-audit.py"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
status=0
fail() { echo "FAIL: $1"; status=1; }

cat >"$tmp/catalogue.json" <<'J'
{"format": 9, "version": "1970-01-01", "spells": [
 {"id": "bless", "class": "Priest", "level": 1, "name": {"en": "Bless"}},
 {"id": "cure_light_wounds", "class": "Priest", "level": 1, "name": {"en": "Cure Light Wounds"}},
 {"id": "detect_magic_priest", "class": "Priest", "level": 1, "name": {"en": "Detect Magic"}},
 {"id": "flame_strike", "class": "Priest", "level": 5, "name": {"en": "Flame Strike"}},
 {"id": "armor", "class": "Wizard", "level": 1, "name": {"en": "Armor"}}
]}
J
cat >"$tmp/effects.json" <<'J'
{
  "bless": {"kind": "buff", "confidence": "high", "target": "area", "duration": {"kind": "rounds", "rounds": 6}, "modifiers": {"to_hit": 1}},
  "flame_strike": {"kind": "damage", "confidence": "high", "target": "area", "duration": {"kind": "instant"},
                   "save": {"type": "spell", "effect": "half"}, "damage": {"dice": "6d8", "type": "fire"}}
}
J
cat >"$tmp/classification.json" <<'J'
{"missing": {"hit point recovery at once": ["cure_light_wounds"]}, "utility": ["detect_magic_priest"]}
J
wizard='[
  {
    "id": "armor",
    "name": "Armor",
    "level": 1,
    "kind": "utility",
    "confidence": "high",
    "target": "none",
    "duration": {
      "kind": "permanent"
    },
    "condition_keys": ["save"]
  }
]'
printf '%s\n' "$wizard" >"$tmp/seeds.json"

run() { python3 "$audit" --catalogue "$tmp/catalogue.json" --effects "$tmp/effects.json" \
    --classification "$tmp/classification.json" --seeds "$tmp/seeds.json" --report "$tmp/report.md" "$@"; }

run >/dev/null || fail "first run"
python3 - "$tmp/seeds.json" <<'P' || fail "seed entries"
import json, sys
e = json.load(open(sys.argv[1]))
assert [x["id"] for x in e] == ["armor", "bless", "flame_strike"], [x["id"] for x in e]
assert e[1]["name"] == "Bless" and e[1]["level"] == 1 and e[2]["level"] == 5
P
grep -q '"condition_keys": \["save"\]' "$tmp/seeds.json" || fail "the Wizard entry changed"
grep -q 'missing kind: hit point recovery at once' "$tmp/report.md" || fail "report: missing kind"
grep -q 'utility: nothing to track' "$tmp/report.md" || fail "report: utility"
grep -q 'mapped: damage (damage dice, save half)' "$tmp/report.md" || fail "report: mapped damage"
grep -q '| all | 4 | 2 | 1 | 1 |' "$tmp/report.md" || fail "report: totals"
grep -q 'Armor' "$tmp/report.md" && fail "report: a Wizard spell is listed"

cp "$tmp/seeds.json" "$tmp/seeds.1"; cp "$tmp/report.md" "$tmp/report.1"
run >/dev/null || fail "rerun"
cmp -s "$tmp/seeds.json" "$tmp/seeds.1" || fail "rerun changed the seeds"
cmp -s "$tmp/report.md" "$tmp/report.1" || fail "rerun changed the report"
run --check || fail "--check on a fresh output"

sed -i 's/"to_hit": 1/"to_hit": 2/' "$tmp/effects.json"
if run --check 2>/dev/null; then fail "--check missed a changed mapping"; fi
run >/dev/null
grep -q '"to_hit": 2' "$tmp/seeds.json" || fail "a changed mapping did not replace the entry"
[ "$(grep -c '"id": "bless"' "$tmp/seeds.json")" = 1 ] || fail "the entry was duplicated"

# Refusals.
cp "$tmp/classification.json" "$tmp/classification.ok"
echo '{"missing": {}, "utility": ["detect_magic_priest"]}' >"$tmp/classification.json"
if run 2>"$tmp/err"; then fail "an unclassified Priest spell passed"; fi
grep -q 'cure_light_wounds' "$tmp/err" || fail "the unclassified spell is not named"
echo '{"missing": {"x": ["bless"]}, "utility": ["detect_magic_priest", "cure_light_wounds"]}' >"$tmp/classification.json"
if run 2>"$tmp/err"; then fail "a double classification passed"; fi
grep -q 'classified twice' "$tmp/err" || fail "the double is not named"
echo '{"missing": {}, "utility": ["detect_magic_priest", "cure_light_wounds", "armor"]}' >"$tmp/classification.json"
if run 2>"$tmp/err"; then fail "a Wizard spell passed as a Priest spell"; fi
cp "$tmp/classification.ok" "$tmp/classification.json"
if python3 "$audit" --catalogue "$tmp/none.json" 2>/dev/null; then fail "a missing catalogue passed"; fi
exit $status
