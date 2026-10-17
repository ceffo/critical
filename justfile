# Critical Project Justfile

_default:
    @just --list

# --- Global Commands ---

# Run full project validation (tracker ids, backend, frontend)
validate: check-no-tracker-ids check-no-language-fields check-pr-title check-adr-tests check-no-sqlite check-release-scripts check-api check-web
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

# Fail when the server binary depends on a SQLite driver (ADR 0025 step 1.2: PostgreSQL only; the
# sqlc tool's own SQLite dependency is not linked into the server).
check-no-sqlite:
    #!/usr/bin/env bash
    set -euo pipefail
    cd api
    if go list -deps ./cmd/api | grep -i sqlite; then
        echo "error: the server depends on the SQLite packages above (ADR 0025 step 1.2)"; exit 1
    fi
    echo "--- No SQLite in the server ---"

# The release build's, the staging deploy's and the promotion's scripts (ADR 0024 steps 3, 4, 7, 8.3 and
# 10.4), offline: docker, fly and gh stubbed, a local rulebooks repo and a generated key, fixture git repos
# (scripts/test-release-scripts.sh, scripts/test-staging-deploy.sh, scripts/test-promotion.sh).
check-release-scripts:
    scripts/test-release-scripts.sh
    scripts/test-staging-deploy.sh
    scripts/test-promotion.sh

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

# Fail on a per-language identifier (a French or English twin of a column, field or JSON tag): texts in a
# human language are localized_texts rows (ADR 0023). The test runs the check on rejected fixtures.
check-no-language-fields:
    scripts/test-check-no-language-fields.sh
    scripts/check-no-language-fields.sh
    @echo "--- No per-language fields ---"

# PR titles are conventional-commit lines (the release reads the version bump from them). The test
# runs the checker on valid and rejected titles.
check-pr-title:
    scripts/test-check-pr-title.sh
    @echo "--- PR title check ---"

# Install all dependencies
install:
    cd api && go mod tidy
    cd web && npm install

# Format all code
fmt: fmt-api fmt-web

# --- Local Development ---

# Run the whole stack locally with live reload: the compose PostgreSQL, the API (air) on :8080 and
# the Vite dev server on :5173 (proxies /api). Spells come from the catalogue artifact (`just
# catalogue`) with the campaign seeds and the database `critical`; without it the API runs on the
# fixture catalogue and seeds (api/testdata) and its own database `critical_fixture`. Ctrl-C stops
# the API (gracefully), Vite and the database when this run started it. `just stop` stops it from
# another terminal. CRITICAL_API_PORT, CRITICAL_WEB_PORT and CRITICAL_PG_PORT move the ports.
[no-exit-message]
dev:
    scripts/dev.sh start

# Stop everything `just dev` started for this checkout, the compose database included (its data
# is kept). Safe to run when nothing runs.
stop:
    @scripts/dev.sh stop

# What runs for this checkout: API, web dev server, database.
status:
    @scripts/dev.sh status

# Start the development PostgreSQL (docker compose service `postgres`) unless it runs, and wait
# until it accepts connections. `just stop` stops it.
dev-db:
    scripts/dev.sh db

# Proof of the local dev lifecycle, on free ports and a throwaway compose project (never touches a
# running `just dev`): starts the real stack, checks API, web and database answer, then Ctrl-C and
# `just stop` leave no process and no open port. Needs docker. Run it after changing the justfile,
# docker-compose.yml, api/.air.toml, scripts/dev.sh or the storage start-up.
dev-smoke:
    scripts/test-dev-lifecycle.sh

# Delete the development databases (real and fixture): stops this checkout's compose services and
# deletes their volume `critical-pg`; the next `just dev` start migrates, re-seeds and imports the
# catalogue. Files in .dev/ are not touched.
dev-reset: stop
    docker compose down --volumes

# --- Code Generation ---

# Regenerate mockery mocks for all service ports (run from api/ so Go module resolves)
mocks:
    cd api && go tool mockery --config ../.mockery.yaml

# Regenerate the DAL code (sqlc) from the PostgreSQL migrations and the query files. Offline:
# sqlc parses the migration files itself, no database server is involved.
generate:
    cd api && go tool sqlc generate -f sqlc.yaml && go mod tidy

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

# Run the Playwright suite against a throwaway stack per test (own API on a free port in its
# in-memory storage mode, no database; api/testdata/seeds fixture). Extra args go to `playwright test`, e.g. `just e2e smoke.spec.ts -g "a|b"` (args are passed quoted, not through sh).
# Artifacts: web/e2e-results/ (screens/, report/, artifacts/ with traces on failure).
[positional-arguments]
e2e *args: build-api
    cd web && node e2e/check-browser.mjs
    cd web && npx playwright test "$@"

# UI review capture (docs/design/ui-review/): screenshots of the real app, every screen and state in
# every colour theme at 1707x1300 and 1440x900 (EN, plus FR for the full set), on the e2e stack
# (in-memory API on free ports, the api/testdata fixture; never ports 8080/5173/5432 or .dev/), and
# the theme contrast report (docs/design/ui-review/contrast.md). Everything lands in the gitignored
# web/e2e-results/ui-review/; the curated set (docs/design/ui-review/curated.txt) is then copied into
# docs/design/ui-review/shots/. Not part of `just validate` nor `just e2e`. Needs `just e2e-install` once.
ui-shots: build-api
    cd web && node e2e/check-browser.mjs
    cd web && npx playwright test -c playwright.ui-review.config.ts screens.spec.ts contrast.spec.ts
    node web/ui-review/curate.mjs

# UI review mock-up round (docs/design/ui-review/mockups/): each option of the review painted over the
# real app (proposed tokens from web/ui-review/mockup-tokens.ts, component rules from
# docs/design/ui-review/mockups/css/), on the same e2e stack and seeded table as `ui-shots`; Act, the
# lookup and the inventory at 1707x1300 and 1440x900, dark and light, plus each option's theme sheet
# and themes.md. Fails on cut text in English or French. Committed shots are lossless WebP (needs
# ImageMagick's `magick`). Not part of `just validate` nor `just e2e`. Needs `just e2e-install` once.
ui-mockups: build-api
    @command -v magick >/dev/null || { echo "ui-mockups needs ImageMagick (magick) for lossless WebP"; exit 1; }
    cd web && node e2e/check-browser.mjs
    cd web && npx playwright test -c playwright.ui-review.config.ts mockups.spec.ts
    node web/ui-review/to-webp.mjs

# --- Fly.io configuration (ADR 0024 steps 2, 5, 8, 9.1) ---

# Check deploy/fly.*.toml: the Go test (part of `just test-api`, so of `just validate`) parses them and
# compares them with the Dockerfile; when flyctl is installed, it also validates them offline.
check-fly:
    #!/usr/bin/env bash
    set -euo pipefail
    (cd api && go test -count=1 ./internal/flycheck/)
    fly=$(command -v fly || command -v flyctl || echo ~/.fly/bin/fly)
    if [ ! -x "$fly" ]; then echo "flyctl not installed: skipping 'fly config validate'"; exit 0; fi
    for f in deploy/fly.*.toml; do "$fly" config validate -c "$f"; done

# Operator only (needs fly credentials and the rulebooks clone; never part of validate or CI). Build the
# production image of the clean HEAD with the REAL catalogue (`just catalogue`, at CATALOGUE_REF) and push
# that one build to each app's repository in the private fly registry, as registry.fly.io/<app>:<commit
# sha>, labelled with the rulebooks commit. Every app gets the same digest (ADR 0024 step 1.5). The
# catalogue only enters the image and the private registry (step 7). The release workflow runs the same
# script on main (docs/deploy.md, "Release build"). Deploy with `fly deploy -c deploy/fly.<env>.toml --image <ref>`.
release-image +apps: catalogue
    scripts/release-image.sh .catalogue "$(git -C "${RULEBOOKS_DIR:-$HOME/code/critical-rulebooks}" rev-parse "${CATALOGUE_REF:-HEAD}^{commit}")" {{apps}}

# Refuses a non-semver or non-increasing version, a branch other than main, a dirty tree, a main behind
# origin/main and a commit staging has not passed (needs gh). Pushing the tag is the release: the Promote
# workflow takes the restore point, deploys staging's image to production and creates the GitHub Release
# (ADR 0024 step 4, docs/deploy.md "Promotion"). E.g. `just release 1.2.3`.
# Tag the head of main as a release (vMAJOR.MINOR.PATCH) and push the tag: promotes it to production
release version:
    scripts/release-tag.sh {{version}}

# Check that a fly app has no public IP and has a private Flycast address (ADR 0024 step 8). Read-only.
fly-private app:
    scripts/fly-private-check.sh {{app}}

# ADR 0024 steps 3.3, 10.5. Read-only. Default commit: HEAD.
# Smoke-check each machine of a fly app through fly proxy: health with the commit, a database read, the web app
fly-smoke app commit=`git rev-parse HEAD`:
    scripts/fly-smoke.sh {{app}} {{commit}}

# Through `fly proxy` on a free local port (never just dev's 8080/5173), until Ctrl-C; read-only.
# Optional fixed port, refused when taken. E.g. `just fly-open critical-staging`, `just fly-open critical-prod`.
# Open a private fly app in the browser: waits until it answers, then prints the URL
fly-open app port="":
    scripts/fly-open.sh {{app}} {{port}}

# Through `fly proxy` on a free local port (never the compose 5432), until Ctrl-C. Prints no password.
# E.g. `just fly-db-proxy tournesol-db`, then psql with the printed connection string.
# Reach a fly PostgreSQL cluster from this machine
fly-db-proxy cluster port="":
    scripts/fly-open.sh --db {{cluster}} {{port}}

# Read-only, through fly ssh as the cluster's operator; the cluster machine must be started. Names and counts only.
# Print the row count of every table of one database of a fly PostgreSQL cluster, e.g. `just fly-db-counts tournesol-db critical_prod`
fly-db-counts cluster db:
    scripts/fly-db-counts.sh {{cluster}} {{db}}

# `mark` writes one action-log entry (a 1d20 roll) and prints its id, `check <id>` fails unless it is
# still there; run one before and one after a restart or redeploy. Refuses non-staging apps.
# Show that a staging app keeps its data across a restart and a redeploy
fly-persistence app +args:
    scripts/fly-persistence-check.sh {{app}} {{args}}

# Operator only (fly credentials), ADR 0024 steps 2.3, 2.4, 9.1 and docs/deploy.md. A rerun changes no
# existing password; `--rotate` does and applies the secret at once (a short outage of the app's new
# connections). E.g. `just fly-db-provision tournesol-db staging prod`.
# Create critical's database users on the fly PostgreSQL cluster and set each new user's CRITICAL_DSN secret
fly-db-provision +args:
    scripts/fly-db-provision.sh {{args}}

# --- Production image (ADR 0024 step 1) ---

# Build the production image with the FIXTURE catalogue (api/testdata), tagged critical:local, and
# smoke-check it (scripts/image-smoke.sh). The real catalogue only enters the release build.
image tag="critical:local":
    #!/usr/bin/env bash
    set -euo pipefail
    commit="$(git rev-parse HEAD)"
    docker build -t "{{tag}}" --build-context catalogue=api/testdata --build-arg COMMIT="$commit" .
    scripts/image-smoke.sh "{{tag}}" "$commit"

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
# validate. Without it, `just dev` runs on the fixture api/testdata/catalogue.json.
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
