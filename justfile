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

# --- Code Generation ---

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
    go run github.com/stephenafamo/bob/gen/bobgen-sqlite@v0.43.0 -c bobgen.yaml
    # Clean up the temp DB
    rm -f .tmp_schema.db
    go mod tidy

# --- Backend (API) Commands ---

# Build the backend
build-api:
    cd api && go build -o ../bin/critical-srv ./cmd/api

# Lint the backend
lint-api:
    cd api && golangci-lint run ./...

# Format the backend
fmt-api:
    cd api && golangci-lint fmt ./...

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

# Typecheck the frontend
typecheck-web:
    cd web && npm run typecheck

# Run all frontend checks
check-web: lint-web typecheck-web build-web
    @echo "--- Frontend Validation Successful ---"

# Format the frontend
fmt-web:
    cd web && npx prettier --write .