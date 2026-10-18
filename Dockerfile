# syntax=docker/dockerfile:1
#
# The production image (ADR 0024 step 1): one container where the Go API serves /api and the built
# web app. Build from the repo root, passing the spell catalogue as the named build context
# `catalogue` (a directory holding catalogue.json):
#
#   docker build --build-context catalogue=api/testdata --build-arg COMMIT="$(git rev-parse HEAD)" \
#       --build-arg VERSION=1.2.3 .
#
# COMMIT and VERSION are the build identity /api/health reports (ADR 0024 step 1.4); each is `unknown` when
# not given. The release build computes VERSION from the commits since the last version (docs/deploy.md).
#
# PR and CI builds pass the fixture (api/testdata); only the release build passes the real catalogue
# (ADR 0024 step 7). Without the `catalogue` context the build stops at the stage below.

# Overridden by `--build-context catalogue=<dir>`; reached only when that input is missing.
FROM alpine:3.24 AS catalogue
RUN echo "error: no spell catalogue given. Pass --build-context catalogue=<dir holding catalogue.json>" \
         "(api/testdata for the fixture, .catalogue for the real one)." >&2 && exit 1

FROM node:25-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# The tag must name the Go version of api/go.mod: the check below fails the build otherwise.
FROM golang:1.26.2-alpine AS api
WORKDIR /src/api
COPY api/go.mod api/go.sum ./
RUN want="go$(sed -n 's/^go //p' go.mod)"; have="$(go env GOVERSION)"; \
    [ "$want" = "$have" ] || { echo "error: Dockerfile builds with $have, api/go.mod wants $want" >&2; exit 1; }
RUN go mod download
COPY api/ ./
ARG COMMIT=unknown
ARG VERSION=unknown
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/ceffo/critical/api/internal/cmd/api.commit=${COMMIT} -X github.com/ceffo/critical/api/internal/cmd/api.version=${VERSION}" \
      -o /out/critical-srv ./cmd/api
# The root-admin script (ADR 0029 step 5.1), run in a machine by `just fly-root-admin` (docs/deploy.md).
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/critical-rootadmin ./cmd/rootadmin

FROM alpine:3.24
# The PostgreSQL client of the cluster's major version (17): the promotion's restore point runs pg_dump
# (and pg_restore --list) from this image in a console machine (scripts/restore-point.sh, ADR 0024 step 4.2).
RUN apk add --no-cache postgresql17-client
RUN addgroup -S critical && adduser -S -G critical -H -h /app critical
WORKDIR /app
COPY --from=api /out/critical-srv /app/critical-srv
COPY --from=api /out/critical-rootadmin /app/critical-rootadmin
COPY --from=web /src/web/dist /app/web
COPY seeds/ /app/seeds/
COPY --from=catalogue catalogue.json /app/catalogue.json
# No CRITICAL_DSN: the database is PostgreSQL (ADR 0025 step 1), its DSN given at run time (on fly a
# secret). The image holds no data and has no volume.
ENV CRITICAL_ADDR=:8080 \
    CRITICAL_SEEDS_DIR=/app/seeds \
    CRITICAL_CATALOGUE=/app/catalogue.json \
    CRITICAL_WEB_DIR=/app/web
USER critical
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
    CMD wget -qO /dev/null http://127.0.0.1:8080/api/health || exit 1
ENTRYPOINT ["/app/critical-srv"]
