#!/usr/bin/env python3
"""Audit of the Priest spells against the effect kinds the engine has.

Deterministic and rerunnable, no network, no spell text: it reads the spell catalogue artifact's
structured fields (id, class, level, name) and two hand-edited files in scripts/priest-effects/:

  effects.json         spell id -> the seed entry (without id, name and level) of every spell the
                       existing effect kinds can model;
  classification.json  every other Priest spell: "missing" (kind label -> spell ids, a mechanic the
                       engine has no kind for) or "utility" (nothing the engine tracks numerically).

Every Priest spell of the catalogue must be in exactly one of them. The script then writes
  - the entries of the mapped spells at the end of seeds/spell_effects.json (the Wizard entries
    above them are kept byte for byte), in catalogue order (level, then id);
  - docs/priest-effects-audit.md, the report: every Priest spell, mapped or missing a kind.

  scripts/priest-effects-audit.py            rewrite the seed entries and the report
  scripts/priest-effects-audit.py --check    change nothing; exit 1 when they would change
  --catalogue PATH   default .catalogue/catalogue.json (`just catalogue`)

`just priest-effects-audit` runs it. The catalogue is the private artifact, so CI does not run
the audit on it; the seed entries are validated by the Go loader's tests, and
scripts/test-priest-effects-audit.sh tests this script on a throwaway catalogue.
"""
import argparse
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
HERE = Path(__file__).resolve().parent / "priest-effects"

# Key order of an entry, as in the Wizard entries of the seed file.
ENTRY_KEYS = [
    "id", "name", "level", "kind", "confidence", "note", "target", "duration", "charges", "save",
    "damage", "modes", "exposure", "modifiers", "ability_gain", "ability_set", "condition",
    "condition_keys", "grant_slots", "consume_item", "attack_roll", "end_effects",
]
MAPPED_KINDS = ("buff", "damage")


def fail(msg):
    print("priest-effects-audit: " + msg, file=sys.stderr)
    sys.exit(2)


def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def priest_spells(catalogue_path):
    cat = load(catalogue_path)
    out = [
        {"id": s["id"], "name": s["name"]["en"], "level": s["level"]}
        for s in cat["spells"]
        if s["class"] == "Priest"
    ]
    out.sort(key=lambda s: (s["level"], s["id"]))
    return out


def classify(spells, effects, classification):
    """id -> ("mapped", entry) | ("missing", label) | ("utility", None); fails on a gap or a double."""
    ids = {s["id"] for s in spells}
    status = {}

    def put(sid, st):
        if sid not in ids:
            fail("%r is not a Priest spell of the catalogue" % sid)
        if sid in status:
            fail("%r is classified twice" % sid)
        status[sid] = st

    for sid, e in effects.items():
        for k in ("id", "name", "level"):
            if k in e:
                fail("%s: effects.json takes %r from the catalogue, remove it" % (sid, k))
        unknown = sorted(set(e) - set(ENTRY_KEYS))
        if unknown:
            fail("%s: unknown entry keys %s" % (sid, unknown))
        if e.get("kind") not in MAPPED_KINDS:
            fail("%s: a mapped spell is a buff or a damage spell (a utility spell goes in classification.json)" % sid)
        put(sid, ("mapped", e))
    for label, sids in classification.get("missing", {}).items():
        if not label.strip():
            fail("a missing-kind label is empty")
        for sid in sids:
            put(sid, ("missing", label))
    for sid in classification.get("utility", []):
        put(sid, ("utility", None))
    left = sorted(ids - set(status))
    if left:
        fail("Priest spells in no class (add each to effects.json or classification.json): " + ", ".join(left))
    return status


def entry_of(spell, e):
    full = {"id": spell["id"], "name": spell["name"], "level": spell["level"], **e}
    return {k: full[k] for k in ENTRY_KEYS if k in full}


# ---- serialisation, in the layout of the seed file (2 spaces, short scalar lists on one line) ----

def dump(v, depth=0):
    pad = "  " * depth
    if isinstance(v, dict):
        if not v:
            return "{}"
        items = [
            "%s  %s: %s" % (pad, json.dumps(k, ensure_ascii=False), dump(x, depth + 1))
            for k, x in v.items()
        ]
        return "{\n" + ",\n".join(items) + "\n" + pad + "}"
    if isinstance(v, list):
        if all(not isinstance(x, (dict, list)) for x in v):
            return "[" + ", ".join(json.dumps(x, ensure_ascii=False) for x in v) + "]"
        items = ["%s  %s" % (pad, dump(x, depth + 1)) for x in v]
        return "[\n" + ",\n".join(items) + "\n" + pad + "]"
    return json.dumps(v, ensure_ascii=False)


def split_seed(raw):
    """(text up to the first Priest entry or the end of the array, entries before it as objects)."""
    dec = json.JSONDecoder()
    i = raw.index("[") + 1
    spans = []
    while True:
        while raw[i] in " \n\r\t,":
            i += 1
        if raw[i] == "]":
            break
        obj, end = dec.raw_decode(raw, i)
        spans.append((obj, i, end))
        i = end
    return spans, i


def new_seed(raw, priest_ids, entries):
    spans, close = split_seed(raw)
    keep = [s for s in spans if s[0]["id"] not in priest_ids]
    cut = keep[-1][2] if keep else raw.index("[") + 1
    body = ",\n".join("  " + dump(e, 1) for e in entries)
    head = raw[:cut]
    if not body:
        return head + "\n]\n"
    return head + ("," if keep else "") + "\n" + body + "\n]\n"


# ---- report ----

def features(e):
    """What of the existing kinds an entry uses, from its fields."""
    f = []
    if e["kind"] == "buff":
        f += sorted(e.get("modifiers", {}))
        for k in ("ability_gain", "ability_set", "charges"):
            if k in e:
                f.append(k)
    else:
        d = e.get("damage")
        if d:
            f.append("damage " + next(k for k in ("steps", "parts", "by_level", "dice") if k in d))
        if "modes" in e:
            f.append("modes")
        if "exposure" in e:
            f.append("exposure")
        if e.get("attack_roll"):
            f.append("attack_roll")
    sv = e.get("save")
    if sv and sv["effect"] != "none":
        f.append("save " + sv["effect"])
    return f


def report(spells, status):
    by_level = {}
    for s in spells:
        by_level.setdefault(s["level"], []).append(s)
    counts = {"mapped": 0, "utility": 0, "missing": 0}
    for st, _ in status.values():
        counts[st] += 1
    labels = {}
    for s in spells:
        st, v = status[s["id"]]
        if st == "missing":
            labels.setdefault(v, []).append(s["id"])
    out = [
        "# Priest spell effects audit",
        "",
        "Generated by `scripts/priest-effects-audit.py` from the spell catalogue's structured fields and",
        "the hand-edited `scripts/priest-effects/effects.json` and `classification.json`; do not edit it",
        "by hand. Rerun with `just priest-effects-audit` (needs `just catalogue`).",
        "",
        "Each Priest spell is exactly one of:",
        "",
        "- **mapped**: the existing effect kinds model it; its entry is in `seeds/spell_effects.json`;",
        "- **utility**: nothing the engine tracks numerically, so no entry (a cast shows its text only);",
        "- **missing kind**: it has a mechanic the engine has no effect kind for; the kind is named.",
        "",
        "| Level | Spells | Mapped | Utility | Missing kind |",
        "|---|---|---|---|---|",
    ]
    for lv in sorted(by_level):
        c = {"mapped": 0, "utility": 0, "missing": 0}
        for s in by_level[lv]:
            c[status[s["id"]][0]] += 1
        out.append("| %d | %d | %d | %d | %d |" % (lv, len(by_level[lv]), c["mapped"], c["utility"], c["missing"]))
    out.append("| all | %d | %d | %d | %d |" % (len(spells), counts["mapped"], counts["utility"], counts["missing"]))
    out += ["", "## Missing kinds", "", "A new effect kind is a separate item. Spells per missing kind:", ""]
    for label in sorted(labels, key=lambda k: (-len(labels[k]), k)):
        out.append("- **%s** (%d): %s" % (label, len(labels[label]), ", ".join("`%s`" % i for i in labels[label])))
    for lv in sorted(by_level):
        out += ["", "## Level %d" % lv, "", "| Id | Name | Status |", "|---|---|---|"]
        for s in by_level[lv]:
            st, v = status[s["id"]]
            if st == "mapped":
                text = "mapped: %s (%s)" % (v["kind"], ", ".join(features(v)) or "-")
            elif st == "utility":
                text = "utility: nothing to track"
            else:
                text = "missing kind: " + v
            out.append("| `%s` | %s | %s |" % (s["id"], s["name"].replace("|", "\\|"), text))
    return "\n".join(out) + "\n"


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--catalogue", default=str(ROOT / ".catalogue" / "catalogue.json"))
    ap.add_argument("--effects", default=str(HERE / "effects.json"))
    ap.add_argument("--classification", default=str(HERE / "classification.json"))
    ap.add_argument("--seeds", default=str(ROOT / "seeds" / "spell_effects.json"))
    ap.add_argument("--report", default=str(ROOT / "docs" / "priest-effects-audit.md"))
    ap.add_argument("--check", action="store_true")
    a = ap.parse_args()

    if not Path(a.catalogue).is_file():
        fail("no catalogue at %s (run `just catalogue`)" % a.catalogue)
    spells = priest_spells(a.catalogue)
    status = classify(spells, load(a.effects), load(a.classification))
    entries = [entry_of(s, status[s["id"]][1]) for s in spells if status[s["id"]][0] == "mapped"]

    raw = Path(a.seeds).read_text(encoding="utf-8")
    want = {a.seeds: new_seed(raw, {s["id"] for s in spells}, entries), a.report: report(spells, status)}
    stale = [p for p, text in want.items() if not Path(p).is_file() or Path(p).read_text(encoding="utf-8") != text]
    if a.check:
        if stale:
            print("priest-effects-audit: out of date: " + ", ".join(stale) + " (run `just priest-effects-audit`)", file=sys.stderr)
            sys.exit(1)
        return
    for p in stale:
        Path(p).write_text(want[p], encoding="utf-8")
        print("wrote " + p)


if __name__ == "__main__":
    main()
