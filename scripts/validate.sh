#!/bin/bash
set -e

echo "--- Running Backend Validation ---"
cd api
go mod tidy
go build ./...
go test ./internal/...
cd ..

echo ""
echo "--- Running Frontend Validation ---"
cd web
npm run typecheck
npm run lint
npm run build
cd ..

echo ""
echo "--- Validation Successful ---"
