# Critical Project Justfile

_default:
    @just --list

# --- Global Commands ---

# Run full project validation (tracker ids, backend, frontend)
validate: check-no-tracker-ids check-adr-tests check-api check-web
    @echo "--- Project Validation Successful ---"

# Fail when an ADR cites a Test* name that has no `func TestName` in the code. A cited name ending
# in `_` is a prefix (e.g. TestCrit_StepN_ stands for TestCrit_Step1_..., so the N placeholder is
# written as `StepN_` and matched by dropping it: only the part before `StepN_` is checked).
check-adr-tests:
    #!/usr/bin/env bash
    set -euo pipefail
    funcs=$(git grep -hoE '^func (\([^)]*\) )?Test[A-Za-z0-9_]+' -- '*_test.go' | sed -E 's/^func (\([^)]*\) )?//')
    missing=0
    for name in $(grep -rhoE '\bTest[A-Z][A-Za-z0-9_]*' docs/adr | sort -u); do
        prefix="${name%%StepN_*}"
        if [ "$prefix" != "$name" ] || [[ "$name" == *_ ]]; then
            grep -q "^${prefix}" <<<"$funcs" && continue
        else
            grep -qx "$name" <<<"$funcs" && continue
        fi
        echo "docs/adr cites $name: no such test in the code"; missing=1
    done
    exit $missing

# Fail on any work-item id (a bead: critical-<id>, <id>.N) in code, tests, seeds or config. Code
# says what it does and why; the link to the work item lives in commits and PRs only
# (docs/agent-workflow.md, "Hard rules for implementers"). The ids come from .beads/issues.jsonl;
# docs (ADR history, workflow examples) and lock files are not scanned.
check-no-tracker-ids:
    #!/usr/bin/env bash
    set -euo pipefail
    ids=$(grep -o '"id":"critical-[a-z0-9]*' .beads/issues.jsonl | sed 's/.*"critical-//' | sort -u | paste -sd'|')
    if [ -z "$ids" ]; then echo "no tracker ids in .beads/issues.jsonl, cannot check"; exit 1; fi
    pattern="\bcritical-($ids)\b|\bcritical-[a-z0-9]{3}\.[0-9]+\b|\b($ids)\.[0-9]+\b"
    if git grep -nIE "$pattern" -- . ':!.beads' ':!docs' ':!api/go.sum' ':!web/package-lock.json'; then
        echo "error: tracker ids above. Say what the code does; the work item goes in the commit message."
        exit 1
    fi
    echo "--- No tracker ids ---"

# Install all dependencies
install:
    cd api && go mod tidy
    cd web && npm install

# Format all code
fmt: fmt-api fmt-web

# --- Local Development ---

# Run the whole stack locally with live reload: API (air) on :8080 and the Vite dev server
# on :5173 (proxies /api). Local state lives in .dev/ (gitignored). Ctrl-C stops both.
dev:
    #!/usr/bin/env bash
    set -euo pipefail
    trap 'kill 0' EXIT
    just dev-api &
    just dev-web &
    wait

# API with live reload (rebuilds on .go changes). Spells come from the catalogue artifact
# (`just catalogue`), with the campaign seeds and the SQLite database .dev/critical.db. Without the
# artifact it runs on the fixture catalogue and seeds (api/testdata) and a database of their own,
# .dev/fixture.db: .dev/critical.db is only ever opened with the real catalogue.
dev-api:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p .dev
    root="$(pwd)"
    if [ -f .catalogue/catalogue.json ]; then
        catalogue="$root/.catalogue/catalogue.json"; seeds="$root/seeds"; db="../.dev/critical.db"
        echo "dev-api: real catalogue .catalogue/catalogue.json, campaign seeds, database .dev/critical.db" >&2
    else
        catalogue="$root/api/testdata/catalogue.json"; seeds="$root/api/testdata/seeds"; db="../.dev/fixture.db"
        echo "dev-api: ==================================================================================" >&2
        echo "dev-api: NO .catalogue/catalogue.json: running on the FIXTURE catalogue and api/testdata/seeds," >&2
        echo "dev-api: database .dev/fixture.db (.dev/critical.db is left untouched)." >&2
        echo "dev-api: run \`just catalogue\` to fetch the real catalogue and use the campaign database." >&2
        echo "dev-api: ==================================================================================" >&2
    fi
    cd api && CRITICAL_DSN="$db" CRITICAL_SEEDS_DIR="$seeds" CRITICAL_CATALOGUE="$catalogue" go tool air -c .air.toml

# Frontend dev server with HMR (needs the API from dev-api). Reinstalls web deps first when
# web/package-lock.json is newer than the last install (e.g. after pulling a new dependency).
dev-web:
    #!/usr/bin/env bash
    set -euo pipefail
    cd web
    if [ ! -f node_modules/.package-lock.json ] || [ package-lock.json -nt node_modules/.package-lock.json ]; then
        echo "web/package-lock.json changed since the last install: running npm install"
        npm install
        touch node_modules/.package-lock.json
    fi
    npm run dev

# Delete the local dev databases (real and fixture); the next dev-api start re-seeds and imports the catalogue
dev-reset:
    rm -f .dev/critical.db .dev/critical.db-wal .dev/critical.db-shm .dev/fixture.db .dev/fixture.db-wal .dev/fixture.db-shm

# --- Code Generation ---

# Regenerate mockery mocks for all service ports (run from api/ so Go module resolves)
mocks:
    cd api && go tool mockery --config ../.mockery.yaml

# Regenerate bob DAL code from SQL schema + query files
generate:
    #!/usr/bin/env bash
    set -euo pipefail
    cd api
    # Build a temp DB by applying all migrations via sqlite3
    rm -f .tmp_schema.db
    for f in internal/storage/sqlite/migrations/*.sql; do
        sqlite3 .tmp_schema.db < "$f"
    done
    # Run the bob code generator
    go tool bobgen-sqlite -c bobgen.yaml
    # Clean up the temp DB
    rm -f .tmp_schema.db
    go mod tidy

# --- Backend (API) Commands ---

# Build the backend
build-api:
    cd api && go build -o ../bin/critical-srv ./cmd/api

# Lint the backend
lint-api:
    cd api && go tool golangci-lint run ./...

# Format the backend
fmt-api:
    cd api && go tool golangci-lint fmt ./...

# Test the backend
test-api:
    cd api && go test -v ./...

# Fast vet (compile + static analysis) on the backend
vet-api:
    cd api && go vet ./...

# Run all backend checks
check-api: lint-api test-api build-api
    @echo "--- Backend Validation Successful ---"

# --- Frontend (Web) Commands ---

# Build the frontend
build-web:
    cd web && npm run build

# Lint the frontend
lint-web:
    cd web && npm run lint
    cd web && npx prettier --check .

# Run frontend unit tests (vitest)
test-web:
    cd web && npm test

# Typecheck the frontend
typecheck-web:
    cd web && npm run typecheck

# Run all frontend checks
check-web: lint-web typecheck-web test-web build-web
    @echo "--- Frontend Validation Successful ---"

# Format the frontend
fmt-web:
    cd web && npx prettier --write .

# --- End-to-end (Playwright) ---

# Install Playwright's Chromium (one-time per machine / Playwright version)
e2e-install:
    cd web && npx playwright install chromium

# Run the Playwright suite against a throwaway stack per test (own API + Vite on free ports,
# fresh temp DB, api/testdata/seeds fixture). Extra args go to `playwright test`, e.g. `just e2e smoke.spec.ts -g "a|b"` (args are passed quoted, not through sh).
# Artifacts: web/e2e-results/ (screens/, report/, artifacts/ with traces on failure).
[positional-arguments]
e2e *args: build-api
    cd web && node e2e/check-browser.mjs
    cd web && npx playwright test "$@"

# Build the design-direction pages (docs/design/ui-directions, static HTML on the real theme tokens)
# and screenshot every one at 1707x1300 and 1440x900 into <direction>/shots/ with Playwright's
# Chromium; fails when any text is clipped (wide or tall) or a page overflows. Needs web/node_modules and
# `just e2e-install`. `just design-shots --self-test` checks the audit itself on a page with known clipping.
design-shots *args:
    cd web && node e2e/check-browser.mjs
    node docs/design/ui-directions/shots.mjs {{args}}

# Clone (or fast-forward) the private rulebooks repo used as a rules reference. Not part of validate/dev.
# Override the target dir with RULEBOOKS_DIR and the remote with RULEBOOKS_REMOTE.
rulebooks:
    #!/usr/bin/env bash
    set -euo pipefail
    dir="${RULEBOOKS_DIR:-$HOME/code/critical-rulebooks}"
    remote="${RULEBOOKS_REMOTE:-git@github.com:ceffo/critical-rulebooks.git}"
    if [ -d "$dir/.git" ]; then
        git -C "$dir" pull --ff-only || { echo "rulebooks: could not update $dir (no access, or local changes diverge)" >&2; exit 1; }
    else
        mkdir -p "$(dirname "$dir")"
        git clone "$remote" "$dir" || { echo "rulebooks: cannot clone $remote (the repo is private; you need access). Do not guess rules without it." >&2; exit 1; }
    fi

# Fetch the spell catalogue artifact (ADR 0021) into the gitignored .catalogue/catalogue.json: the
# rulebooks repo is cloned or fast-forwarded (`just rulebooks`), then its dist/catalogue.json is taken
# at CATALOGUE_REF (default: the checked-out branch). Needs access to the private repo; never part of
# validate. Without it, dev-api runs on the fixture api/testdata/catalogue.json.
catalogue: rulebooks
    #!/usr/bin/env bash
    set -euo pipefail
    dir="${RULEBOOKS_DIR:-$HOME/code/critical-rulebooks}"
    ref="${CATALOGUE_REF:-HEAD}"
    mkdir -p .catalogue
    tmp="$(mktemp .catalogue/catalogue.json.XXXXXX)"
    if ! git -C "$dir" show "$ref:dist/catalogue.json" > "$tmp"; then
        rm -f "$tmp"
        echo "catalogue: no dist/catalogue.json at $ref in $dir" >&2; exit 1
    fi
    chmod 644 "$tmp"
    mv "$tmp" .catalogue/catalogue.json
    echo "catalogue: .catalogue/catalogue.json from $dir at $(git -C "$dir" rev-parse --short "$ref")"

# How the French UI words the six spell stats over a catalogue (ADR 0021 step 8): per field, how many
# are worded from the typed value, shown as a listed label, or shown as the book's English text,
# with the most common English texts left. Default: the real .catalogue/catalogue.json (`just
# catalogue`); pass api/testdata/catalogue.json for the fixture. STAT_COVERAGE_LIST=<field> (e.g.
# area_of_effect) also lists every distinct English text of that field with its wording. Never part
# of validate.
stat-coverage catalogue=".catalogue/catalogue.json":
    #!/usr/bin/env bash
    set -euo pipefail
    out="$(mktemp)"
    trap 'rm -f "$out"' EXIT
    cat="$(realpath "{{catalogue}}")"
    (cd api && go run ./cmd/spellstats "$cat") > "$out"
    cd web && STAT_COVERAGE_JSON="$out" npx vitest run src/lib/statCoverage.test.ts

# The tracker DB is the truth: it is exported again after the pull and every changed status is printed.
# Never `br sync --merge` here: it lets the pulled JSONL overwrite local tracker changes.
# Supervisor: pull main in the main checkout while .beads/issues.jsonl is dirty
sync-main:
    #!/usr/bin/env bash
    set -euo pipefail
    [ "$(git branch --show-current)" = main ] || { echo "sync-main: run it on main in the main checkout" >&2; exit 1; }
    if [ -n "$(git status --porcelain --untracked-files=no -- . ':!.beads/issues.jsonl')" ]; then
        echo "sync-main: changes other than .beads/issues.jsonl; commit or remove them first" >&2; exit 1
    fi
    br sync --flush-only >/dev/null 2>&1 || true
    before="$(mktemp)"; cp .beads/issues.jsonl "$before"
    git checkout -q -- .beads/issues.jsonl
    git pull -q --ff-only
    br sync --flush-only --force >/dev/null
    python3 - "$before" .beads/issues.jsonl <<'PY'
    import json, sys
    def load(p):
        return {j["id"]: j.get("status") for j in map(json.loads, open(p))}
    a, b = load(sys.argv[1]), load(sys.argv[2])
    diff = sorted((k, a.get(k), b.get(k)) for k in a.keys() | b.keys() if a.get(k) != b.get(k))
    for k, x, y in diff:
        print(f"CHANGED {k}: {x} -> {y}")
    print("sync-main: tracker export identical to before the pull" if not diff else "sync-main: review the changes above")
    PY
    rm -f "$before"
    git log --oneline -1
