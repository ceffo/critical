#!/usr/bin/env python3
"""Fail on a duplicate key in any locale file (every language, every namespace).

JSON.parse keeps the last of two equal keys without a word, so a duplicate silently hides a text.
Usage: check-locale-duplicate-keys.py [locales dir]   (default: web/src/i18n/locales)
"""
import json
import sys
from pathlib import Path


def duplicates(path):
    found = []

    def pairs(items):
        seen = set()
        for key, _ in items:
            if key in seen:
                found.append(key)
            seen.add(key)
        return dict(items)

    json.loads(path.read_text(encoding="utf-8"), object_pairs_hook=pairs)
    return found


def main():
    root = Path(sys.argv[1] if len(sys.argv) > 1 else "web/src/i18n/locales")
    files = sorted(root.rglob("*.json"))
    if not files:
        print(f"error: no locale file under {root}")
        return 1
    bad = False
    for f in files:
        for key in duplicates(f):
            print(f"{f}: duplicate key {key!r}")
            bad = True
    if bad:
        print("error: duplicate locale key above. Keep one value per key.")
    return 1 if bad else 0


sys.exit(main())
