# Critical Project Justfile

_default:
    @just --list

# --- Global Commands ---

# Run full project validation (tracker ids, backend, frontend)
validate: check-no-tracker-ids check-linear check-linear-queue check-no-language-fields check-pr-title check-adr-tests check-no-sqlite check-release-scripts check-ci-skip test-fly-open check-api check-web
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

# scripts/linear.sh (Linear GraphQL helper), scripts/linear-setup.sh, scripts/linear-configure.sh and the supervisor's Linear scripts (.claude/scripts) against a local stub server; no real Linear call.
check-linear:
    scripts/test-linear.sh
    scripts/test-ship-linear.sh
    scripts/test-linear-configure.sh

# scripts/linear-ready.sh and the Linear mode of pending-questions.sh against a local stub server; no real Linear call.
check-linear-queue:
    scripts/test-linear-queue.sh

# The CI slow-job skip rule (scripts/test-ci-skip-slow.sh): docs-only paths and the ci-skip label.
check-ci-skip:
    scripts/test-ci-skip-slow.sh

# The release version's, the release build's, the staging deploy's and the promotion's scripts (ADR 0024
# steps 3, 4, 7, 8.3 and 10.4), and the backup and restore drill's (step 9), offline: docker, fly, gh and the
# PostgreSQL tools stubbed, a local rulebooks repo and a generated key, fixture git repos
# (scripts/test-release-version.sh, scripts/test-release-scripts.sh, scripts/test-staging-deploy.sh,
# scripts/test-promotion.sh, scripts/test-gate-check.sh, scripts/test-db-backup.sh, scripts/test-major.sh).
check-release-scripts:
    scripts/test-build-identity.sh
    scripts/test-release-version.sh
    scripts/test-release-scripts.sh
    scripts/test-staging-deploy.sh
    scripts/test-gate-check.sh
    scripts/test-fly-public-setup.sh
    scripts/test-promotion.sh
    scripts/test-db-backup.sh
    scripts/test-major.sh

# Fail on any work-item id (Linear CRI-<n>, or an old bead id) in code, tests, seeds or config. The test
# plants ids in throwaway trees; the id sources are in the script header (docs/agent-workflow.md,
# "Hard rules for implementers").
check-no-tracker-ids:
    scripts/test-check-no-tracker-ids.sh
    scripts/check-no-tracker-ids.sh

# The local port `just fly-open` resolves: the apps' login ports by default, an explicit port wins, a taken one is refused.
test-fly-open:
    scripts/test-fly-open.sh

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

# Set the one root admin (ADR 0029 step 5.1) by email, on the database at CRITICAL_DSN; unset, the local
# database `just dev` uses (critical with the real catalogue, else critical_fixture). The server must have run
# once on it (migrations). Idempotent; another email is refused once the root has logged in. Staging and
# production: `just fly-root-admin` (docs/deploy.md, "Root admin"). Local dev needs none: the dev login makes one.
root-admin email:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "${CRITICAL_DSN:-}" ]; then
        db=critical_fixture
        if [ -z "${CRITICAL_DEV_FIXTURE:-}" ] && [ -f .catalogue/catalogue.json ]; then db=critical; fi
        export CRITICAL_DSN="postgres://critical:critical@localhost:${CRITICAL_PG_PORT:-5432}/$db?sslmode=disable"
        echo "root-admin: CRITICAL_DSN is not set: using the local database $db (just dev's)"
    fi
    cd api && go run ./cmd/rootadmin "{{email}}"

# Set the root admin of a deployed app in one command, e.g. `just fly-root-admin critical-hit-staging you@gmail.com`:
# runs the image's script inside the app's machine with its own CRITICAL_DSN (never shown), starting a machine
# first when none runs. Operator only (fly credentials); docs/deploy.md ("Root admin").
fly-root-admin app email:
    scripts/fly-root-admin.sh {{app}} {{email}}

# Operator only (fly credentials), ADR 0024 step 8.5. Create the fly app of an environment (name from deploy/fly.<env>.toml),
# give it its private address, and copy its CRITICAL_DSN from the app it replaces (never shown). E.g. `just fly-app-create staging critical-staging`.
fly-app-create env old="":
    scripts/fly-app-create.sh {{env}} {{ if old != "" { "--dsn-from " + old } else { "" } }}

# Operator only. Set CRITICAL_GOOGLE_CLIENT_ID / _SECRET / CRITICAL_PUBLIC_URL on an app from the OAuth client JSON the
# Google console downloads (or typed when no file is given); prints the redirect URI to authorise. docs/deploy.md ("Google login").
fly-google-secrets app json="":
    scripts/fly-google-secrets.sh {{app}} {{json}}

# Operator only. Allocate the (free) public addresses of a deployed app and prove the login gate on its public URL;
# the addresses are released again when the gate check fails. docs/deploy.md ("Going public").
fly-go-public app:
    scripts/fly-go-public.sh {{app}}

# Read-only, no fly: what a visitor with no session gets from a URL (health reports the Google login; the catalogue,
# the data routes and every page but the login refuse). E.g. `just gate-check https://critical-hit.fly.dev`.
gate-check url:
    scripts/gate-check.sh {{url}}

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
    cd api && go build -ldflags "$(../scripts/build-identity.sh)" -o ../bin/critical-srv ./cmd/api

# Lint the backend, on the Go version go.mod names (CI's): gofmt output differs between Go versions
lint-api:
    cd api && GOTOOLCHAIN=go$(awk '$1=="go"{print $2}' go.mod) go tool golangci-lint run ./...

# Format the backend
fmt-api:
    cd api && GOTOOLCHAIN=go$(awk '$1=="go"{print $2}' go.mod) go tool golangci-lint fmt ./...

# Test the backend
test-api:
    cd api && go test -v ./...

# Rewrite the web journal fixtures (web/src/journal/fixtures) to what the services log; review the diff
journal-fixtures:
    cd api && UPDATE_JOURNAL_FIXTURES=1 go test -count=1 ./internal/domain/... -run TestJournalFixtures
    cd web && npx prettier --write src/journal/fixtures && npx vitest run src/journal

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

# There is no recipe that makes a version: CI is the only maker of `v*` tags. Each main commit that makes a
# release gets the next version from the conventional commits since the last one, baked into its image, and
# the tag once staging passed (docs/deploy.md, "How a release flows").

# Promote to production (ADR 0024 step 4): runs the Promote workflow from main. In a terminal with no version:
# a gum picker of the versions above production's, up to staging's (staging's preselected; Esc promotes
# nothing); no terminal and no version: the newest that passed staging; a version (1.2.3 or v1.2.3): that one,
# no prompt, also the rollback. Prints the run's URL. Repository admins only (the workflow checks it); the
# workflow's log says what it resolved and what production ran. Needs gum for the picker only.
promote $version="":
    scripts/promote-pick.sh "$version"

# Make the next staging release the next major version (ADR 0024 step 3): runs the Major workflow from main, which
# moves the `next-major` marker to main's head. Repository admins only (the workflow checks it). Only the next
# release that happens is a major (a docs-only range makes none and the marker waits); on 0.x the first major is
# 1.0.0. Prints what the next release will be.
major:
    gh workflow run major.yml --ref main
    git ls-remote --tags --refs origin 'v*' | sed 's|.*refs/tags/||' | scripts/next-major.sh preview

# Check that a fly app has no public IP and has a private Flycast address (ADR 0024 step 8). Read-only.
fly-private app:
    scripts/fly-private-check.sh {{app}}

# ADR 0024 steps 3.3, 10.5. Read-only. Default commit: HEAD.
# Smoke-check each machine of a fly app through fly proxy: health with the commit, a database read, the web app
fly-smoke app commit=`git rev-parse HEAD`:
    scripts/fly-smoke.sh {{app}} {{commit}}

# Through `fly proxy` on a free local port (never just dev's 8080/5173), until Ctrl-C; read-only.
# A free local port unless one is given (refused when taken). The apps are public now: this is for looking behind the public ingress. E.g. `just fly-open critical-hit-staging`.
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

# ADR 0024 step 9, docs/deploy.md "Backups". Operator only: fly credentials, and the database user's password
# in PGPASSWORD or ~/.config/critical/critical_<env>.password. pg_dump (custom format, PostgreSQL 17 or newer)
# through `fly proxy` into .backups/<env>-<UTC time>/ (gitignored, mode 600) with the table counts at dump
# time beside it. Prints the folder and sizes, never the password or any data. E.g. `just db-backup prod`.
# Dump one environment's database (staging or prod) into the gitignored .backups/
db-backup env:
    scripts/db-backup.sh {{env}}

# Local only, never touches fly: restores a dump (default: the newest prod one) into a scratch database of a
# throwaway compose PostgreSQL (own project and port, never `just dev`'s), compares every table's row count with
# the dump-time counts, drops the scratch database and the compose project. Fails unless every table matches.
# Prove a dump restores
db-restore-drill dump="":
    scripts/db-restore-drill.sh {{dump}}

# Local only: a database written by the release before the one equipment model (old ref, default the merge
# base with origin/main) is upgraded by this checkout's server on a throwaway compose PostgreSQL (own project
# and port, never `just dev`'s), then the conversion, old entries' pop/redo, restarts, the attack, Equip and
# the spell components are checked through the API. Removes the compose project. Needs docker, psql, jq, go.
# Rehearse the equipment upgrade
equipment-rehearsal ref="":
    scripts/equipment-upgrade-rehearsal.sh {{ref}}

# Local only: the skill armour carry-over (020, ADR 0033 step 14) on a database written by the release before
# (old ref, default the merge base with origin/main), in a throwaway compose PostgreSQL (own project and port,
# never `just dev`'s): every thief's and bard's skill percentages compared, restarts, the old entries popped.
# Removes the compose project. Needs docker, psql, jq, go.
# Rehearse the skill armour upgrade
skill-armour-rehearsal ref="":
    scripts/skill-armour-rehearsal.sh {{ref}}

# The AC override migration (017) on a copy of a real database: a read-only pg_dump of the source
# (default: the main checkout's dev database) restored into a throwaway compose PostgreSQL, upgraded
# by this checkout's server and checked through SQL and the API.
ac-rehearsal container="critical-postgres-1" database="critical":
    scripts/ac-override-rehearsal.sh {{container}} {{database}}

# The snapshot codec upgrade (019) on copies of a real database (default: the main checkout's dev
# database, only read by pg_dump) in a throwaway compose PostgreSQL: the rolling deploy beside the
# release before (old ref, default the merge base with origin/main), a data step failure, and every
# character's stack popped and replayed against that release. Removes the compose project.
snapshot-rehearsal container="critical-postgres-1" database="critical" ref="":
    scripts/snapshot-codec-rehearsal.sh {{container}} {{database}} {{ref}}

# ADR 0024 step 9.2, docs/deploy.md "Promotion". Admin, once: a random passphrase in
# ~/.config/critical/restore-point.passphrase (never overwritten), set as the production secret
# RESTORE_POINT_PASSPHRASE. Keep a copy in a password manager: GitHub never shows a secret back.
# Create the passphrase that encrypts the promotion's restore points
restore-point-key:
    scripts/restore-point-key.sh

# ADR 0024 step 9.3, docs/deploy.md "Rollback". Downloads the Promote run's artifact (kept 90 days), decrypts it
# with ~/.config/critical/restore-point.passphrase, unpacks it as a backup (.backups/<restore point>/: the dump
# and counts.txt, gitignored, mode 600). Then `just db-restore-drill` it. E.g. `just restore-point-fetch 1.2.3`.
# Fetch the restore point taken before a version was promoted
restore-point-fetch version:
    scripts/restore-point-fetch.sh {{version}}

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
# smoke-check it (scripts/image-smoke.sh). The real catalogue only enters the release build, and only the
# release build computes a real version: this one is built as 0.0.0, which the smoke expects back.
image tag="critical:local":
    #!/usr/bin/env bash
    set -euo pipefail
    commit="$(git rev-parse HEAD)"
    docker build -t "{{tag}}" --build-context catalogue=api/testdata --build-arg COMMIT="$commit" --build-arg VERSION=0.0.0 .
    scripts/image-smoke.sh "{{tag}}" "$commit" 0.0.0

# Build the design-direction pages (docs/design/ui-directions, static HTML on the real theme tokens)
# and screenshot every one at 1707x1300 and 1440x900 into <direction>/shots/ with Playwright's
# Chromium; fails when any text is clipped (wide or tall) or a page overflows. Needs web/node_modules and
# `just e2e-install`. `just design-shots --self-test` checks the audit itself on a page with known clipping.
design-shots *args:
    cd web && node e2e/check-browser.mjs
    node docs/design/ui-directions/shots.mjs {{args}}

# Onboarding design round (docs/design/onboarding/): the mock-ups of web/design/onboarding/ served on
# the app's Vite and screenshotted in French and English at 1280x800, 1920x1080 and 2560x1440 into
# docs/design/onboarding/shots/; fails on cut text or a page scroll (`--self-test` checks the audit).
# Not part of `just validate` nor `just e2e`. Needs web/node_modules and `just e2e-install` once.
onboarding-shots *args:
    cd web && node e2e/check-browser.mjs
    node docs/design/onboarding/shots.mjs {{args}}

# Equipment design round (docs/design/equipment/): the mock-ups of web/design/equipment/ served on the
# app's Vite and screenshotted in French and English at 1280x800, 1920x1080 and 2560x1440 into
# docs/design/equipment/shots/; fails on cut text or a page scroll (`--self-test` checks the audit,
# `--fr` shoots French only, `--only=a,b` some screens). Not part of `just validate` nor `just e2e`.
equipment-shots *args:
    cd web && node e2e/check-browser.mjs
    node docs/design/equipment/shots.mjs {{args}}

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

# The new-themes round (docs/design/new-themes/): the candidates painted over the app, then the
# app's theme rules (theme.test.ts) run over them; the tests pinning today's theme list fail by design.
ui-themes: build-api
    cd web && node e2e/check-browser.mjs
    cd web && npx playwright test -c playwright.ui-review.config.ts themes-round.spec.ts
    cd web && npx vitest run -c ui-review/vitest.candidates.ts || true

# Brand placement design round (docs/design/brand-placement/): the decided header drawn in each
# placement over the real app (French, the Arcane theme), on the e2e stack with the seeded table of
# web/ui-review/paint.ts. Fails on cut text, a page scroll, a header over 120 px or a hidden wordmark.
# Not part of `just validate` nor `just e2e`. Needs `just e2e-install` once.
ui-brand: build-api
    cd web && node e2e/check-browser.mjs
    cd web && npx playwright test -c playwright.ui-review.config.ts brand.spec.ts

# Once, by the user: creates and checks the Linear API key (stored in ~/.config/critical/linear-api-key, mode 600,
# exported to new shells) and confirms Linear's GitHub integration. A rerun checks and changes nothing.
# Set up the Linear API key and the GitHub integration
linear-setup:
    scripts/linear-setup.sh

# Shapes the Linear workspace (project in team CRI, process labels) from scripts/linear-mapping.json; idempotent.
# `just linear-configure --dry-run` prints what it would change without writing.
# Configure Linear for the project
linear-configure *args:
    scripts/linear-configure.sh {{args}}

