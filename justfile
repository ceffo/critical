# Critical Project Justfile

_default:
    @just --list

# --- Global Commands ---

# Run full project validation (backend + frontend)
validate: check-api check-web
    @echo "--- Project Validation Successful ---"

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

# API with live reload (rebuilds on .go changes), SQLite at .dev/critical.db
dev-api:
    mkdir -p .dev
    cd api && CRITICAL_DSN=../.dev/critical.db CRITICAL_SEEDS_DIR=../seeds go tool air -c .air.toml

# Frontend dev server with HMR (needs the API from dev-api)
dev-web:
    cd web && npm run dev

# Delete the local dev database; the next dev-api start re-seeds from seeds/
dev-reset:
    rm -f .dev/critical.db

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