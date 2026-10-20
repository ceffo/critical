#!/usr/bin/env bash
# The build identity of a local build (`just dev`, `just build-api` and so the e2e stacks), as the
# `go build -ldflags` value that sets what /api/health reports (ADR 0024 step 1.4):
#   -X ...api.commit=<full sha> -X ...api.version=<last version>+<short sha>[.dirty]
# The version is the last `v*` tag (0.0.0 with none) plus the short commit as semver build metadata,
# `.dirty` when the tree has uncommitted changes. Release images get a plain X.Y.Z from the release
# workflow instead (Dockerfile build args). Usage: scripts/build-identity.sh [repo dir]
set -euo pipefail

dir="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
pkg=github.com/ceffo/critical/api/internal/cmd/api

if ! sha="$(git -C "$dir" rev-parse HEAD 2>/dev/null)"; then
    # Not a git checkout (a source archive): leave the defaults, "unknown".
    exit 0
fi
last="$(git -C "$dir" describe --tags --abbrev=0 --match 'v*' 2>/dev/null || true)"
last="${last#v}"
version="${last:-0.0.0}+$(git -C "$dir" rev-parse --short=7 HEAD)"
if [ -n "$(git -C "$dir" status --porcelain)" ]; then
    version="$version.dirty"
fi
printf -- '-X %s.commit=%s -X %s.version=%s\n' "$pkg" "$sha" "$pkg" "$version"
