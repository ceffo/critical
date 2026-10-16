# syntax=docker/dockerfile:1
#
# The production image (ADR 0024 step 1): one container where the Go API serves /api and the built
# web app. Build from the repo root, passing the spell catalogue as the named build context
# `catalogue` (a directory holding catalogue.json):
#
#   docker build --build-context catalogue=api/testdata --build-arg COMMIT="$(git rev-parse HEAD)" .
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
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/ceffo/critical/api/internal/cmd/api.commit=${COMMIT}" \
      -o /out/critical-srv ./cmd/api

FROM alpine:3.24
RUN addgroup -S critical && adduser -S -G critical -H -h /app critical \
    && mkdir -p /data && chown critical:critical /data
WORKDIR /app
COPY --from=api /out/critical-srv /app/critical-srv
COPY --from=web /src/web/dist /app/web
COPY seeds/ /app/seeds/
COPY --from=catalogue catalogue.json /app/catalogue.json
ENV CRITICAL_ADDR=:8080 \
    CRITICAL_DSN=/data/critical.db \
    CRITICAL_SEEDS_DIR=/app/seeds \
    CRITICAL_CATALOGUE=/app/catalogue.json \
    CRITICAL_WEB_DIR=/app/web
USER critical
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s --retries=3 \
    CMD wget -qO /dev/null http://127.0.0.1:8080/api/health || exit 1
ENTRYPOINT ["/app/critical-srv"]
