# syntax=docker/dockerfile:1
#
# Rainy — multi-stage, multi-arch image (linux/amd64, linux/arm64).
#
#   docker build -t rainy .
#   docker buildx build --platform linux/amd64,linux/arm64 -t <owner>/rainy .
#   make docker-build      # tags rainy:dev; VERSION defaults to the repository VERSION file
#
# Stages: web (node, builds web/dist) → build (Go, cross-compiled, embeds web/dist) → final
# (alpine + ffmpeg). The first two stages always run on the build host's platform; only the
# final `apk add` runs on the target platform (QEMU when cross-building).

ARG NODE_VERSION=24
ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.22

# ------------------------------------------------------------------------------ web
FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-alpine AS web
WORKDIR /src/web
ENV CI=true COREPACK_ENABLE_DOWNLOAD_PROMPT=0
# pnpm version comes from the "packageManager" field of web/package.json.
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN corepack enable && corepack install
RUN --mount=type=cache,id=rainy-pnpm-store,target=/pnpm/store \
    pnpm install --frozen-lockfile --store-dir /pnpm/store
COPY web/ ./
RUN pnpm build

# ------------------------------------------------------------------------------ go
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-mod=readonly
COPY go.mod go.sum ./
RUN --mount=type=cache,id=rainy-gomod,target=/go/pkg/mod \
    go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY web/embed.go ./web/embed.go
COPY VERSION ./VERSION
COPY --from=web /src/web/dist ./web/dist

ARG TARGETOS TARGETARCH TARGETVARIANT
# Defaults to the repository's VERSION file; release builds pass the tag explicitly.
ARG VERSION=
ARG COMMIT=
ARG BUILD_DATE=
RUN --mount=type=cache,id=rainy-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=rainy-gobuild,target=/root/.cache/go-build \
    set -eu; \
    if [ "$TARGETARCH" = "arm" ] && [ -n "$TARGETVARIANT" ]; then export GOARM="${TARGETVARIANT#v}"; fi; \
    pkg=rainy/internal/buildinfo; \
    VERSION="${VERSION:-$(tr -d '\r\n' < VERSION)}"; \
    GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build -trimpath -buildvcs=false \
      -ldflags "-s -w -X $pkg.Version=$VERSION -X $pkg.Commit=$COMMIT -X $pkg.BuildDate=$BUILD_DATE" \
      -o /out/rainy ./cmd/rainy

# ------------------------------------------------------------------------------ final
FROM alpine:${ALPINE_VERSION}

ARG VERSION=dev
ARG COMMIT=
ARG BUILD_DATE=
ARG SOURCE_URL=https://github.com/yexca/rainy
LABEL org.opencontainers.image.title="Rainy" \
      org.opencontainers.image.description="Self-hosted music server with a Subsonic/OpenSubsonic API, a PWA web player and in-app library management" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="${SOURCE_URL}" \
      org.opencontainers.image.url="${SOURCE_URL}" \
      org.opencontainers.image.documentation="${SOURCE_URL}#readme"

COPY --chmod=0755 docker/entrypoint.sh /usr/local/bin/docker-entrypoint.sh
# The entrypoint is normalised to LF in case the repo was checked out with CRLF (Windows).
RUN apk add --no-cache ffmpeg ca-certificates tzdata su-exec \
 && sed -i 's/\r$//' /usr/local/bin/docker-entrypoint.sh \
 && addgroup -g 1000 rainy \
 && adduser -D -H -u 1000 -G rainy -h /data -s /sbin/nologin rainy \
 && mkdir -p /data /music \
 && chown rainy:rainy /data

COPY --from=build /out/rainy /usr/local/bin/rainy

ENV RAINY_DATA_DIR=/data \
    RAINY_MUSIC_DIR=/music \
    RAINY_LOG_FORMAT=json \
    PUID=1000 \
    PGID=1000 \
    UMASK=022

WORKDIR /data
VOLUME /data
EXPOSE 7650

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD wget -q -T 4 -O /dev/null "127.0.0.1:${RAINY_PORT:-7650}/api/health" || exit 1

# Starts as root only long enough to fix /data ownership, then drops to PUID:PGID (see
# docker/entrypoint.sh). To never start as root at all, run with `--user <uid>:<gid>`.
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["rainy", "serve"]
