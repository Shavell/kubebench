# syntax=docker/dockerfile:1

# KubeBench image: the Go server with the built frontend embedded, on a
# distroless nonroot base. The build stages run on the build host
# ($BUILDPLATFORM) and cross-compile for $TARGETOS/$TARGETARCH, so a
# multi-arch build needs no emulation:
#
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     --build-arg VERSION=0.7.0 --build-arg COMMIT="$(git rev-parse --short HEAD)" \
#     --build-arg DATE="$(git log -1 --format=%cI)" -t ghcr.io/shavell/kubebench:0.7.0 .
#
# `make image` does the same for the local platform.

# 1. Frontend (Vite) -> /src/dist
FROM --platform=$BUILDPLATFORM node:26-trixie-slim AS frontend
WORKDIR /src
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund
# CHANGELOG.md is bundled for What's new (src/core/updates/changelog.ts).
COPY index.html vite.config.ts tsconfig*.json CHANGELOG.md ./
COPY public/ public/
COPY src/ src/
RUN npm run build

# 2. Go server with the frontend embedded (go:embed of internal/web/dist).
# CGO stays off: SQLite is the pure-Go modernc driver.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY backend/ ./
COPY --from=frontend /src/dist/ internal/web/dist/
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
      -o /out/kubebench . \
 && mkdir /out/data

# 3. Runtime: static binary only; no shell, no package manager.
FROM gcr.io/distroless/static-debian13:nonroot
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
LABEL org.opencontainers.image.title="KubeBench" \
      org.opencontainers.image.licenses="GPL-3.0-only" \
      org.opencontainers.image.description="VSCode-style Kubernetes workbench: workloads, logs, shells, YAML, Helm and metrics for many clusters in one web UI" \
      org.opencontainers.image.url="https://github.com/Shavell/kubebench" \
      org.opencontainers.image.source="https://github.com/Shavell/kubebench" \
      org.opencontainers.image.documentation="https://github.com/Shavell/kubebench/blob/main/docs/deployment.md" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${DATE}"
COPY --from=backend /out/kubebench /kubebench
# GPL-3.0 §4: the image carries the license text.
COPY LICENSE /LICENSE
# An image-owned /data lets Docker seed named volumes with nonroot ownership.
COPY --from=backend --chown=65532:65532 /out/data /data
ENV PORT=9876 \
    KUBEBENCH_DATA_DIR=/data \
    KUBEBENCH_DB=/data/kubebench.db
EXPOSE 9876
VOLUME ["/data"]
USER 65532:65532
# distroless has no shell or curl: the binary probes its own /healthz.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD ["/kubebench", "healthcheck"]
ENTRYPOINT ["/kubebench"]
