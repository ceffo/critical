#!/usr/bin/env bash
# Runs check-locale-duplicate-keys.py on a clean and on a planted-duplicate locale tree.
set -euo pipefail
cd "$(dirname "$0")/.."
check=scripts/check-locale-duplicate-keys.py
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/clean/fr" "$tmp/dup/fr" "$tmp/dup/en"
echo '{"a": {"x": 1}, "b": {"x": 2}}' > "$tmp/clean/fr/common.json"
echo '{"a": {"x": 1, "y": 2}}' > "$tmp/dup/en/common.json"
echo '{"a": {"x": 1, "y": 2, "x": 3}}' > "$tmp/dup/fr/other.json"

python3 "$check" "$tmp/clean" >/dev/null || { echo "FAIL: clean tree rejected"; exit 1; }
if python3 "$check" "$tmp/dup" >/dev/null; then echo "FAIL: duplicate accepted"; exit 1; fi
echo "check-locale-duplicate-keys: ok"
