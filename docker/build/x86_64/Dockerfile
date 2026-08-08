# syntax=docker/dockerfile:1.7
# This dockerfile should be built with `make docker-build` from the stash root.

# Build Frontend
FROM node:24-alpine AS frontend
RUN apk add --no-cache make git
## cache node_modules separately
COPY ./ui/v2.5/package.json ./ui/v2.5/pnpm-lock.yaml /stash/ui/v2.5/
WORKDIR /stash
COPY Makefile /stash/
COPY ./graphql /stash/graphql/
COPY ./ui /stash/ui/
# pnpm install with npm
RUN npm install -g pnpm@9
ENV PNPM_STORE_PATH=/root/.pnpm-store
RUN --mount=type=cache,target=/root/.pnpm-store \
  make pre-ui
RUN --mount=type=cache,target=/root/.pnpm-store \
  make generate-ui
ARG GITHASH
ARG STASH_VERSION
RUN --mount=type=cache,target=/root/.pnpm-store \
  BUILD_DATE=$(date +"%Y-%m-%d %H:%M:%S") make ui-only

# Build Backend
FROM golang:1.24.3-alpine AS backend
RUN apk add --no-cache make alpine-sdk
WORKDIR /stash
COPY ./go* ./*.go Makefile gqlgen.yml .gqlgenc.yml /stash/
COPY ./graphql /stash/graphql/
COPY ./scripts /stash/scripts/
COPY ./pkg /stash/pkg/
COPY ./cmd /stash/cmd/
COPY ./internal /stash/internal/
# needed for generate-login-locale
COPY ./ui /stash/ui/
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  make generate-backend generate-login-locale
COPY --from=frontend /stash /stash/
ARG GITHASH
ARG STASH_VERSION
# Optional build-time Go flags. Empty by default, so this changes nothing for a
# normal build. It exists so a memory-constrained host can cap the compiler's
# parallelism, which is what drives peak memory: `go build` compiles NumCPU
# packages at once, and on a 4-CPU box already carrying a large dockerd and
# dozens of containers that spike is what reaches the OOM killer.
#
#   docker build --build-arg GOFLAGS=-p=2 ...
#
# Roughly halves peak compile memory for roughly double the compile time on
# this stage.
ARG GOFLAGS=""
ENV GOFLAGS=$GOFLAGS
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  make flags-release flags-pie stash

# Final Runnable Image
FROM alpine:latest
# libva + intel-media-driver (iHD) enable VAAPI/QSV hardware transcode on Intel
# iGPUs (this host is Alder Lake-N / i915). ffmpeg in this image already advertises
# vaapi/qsv; the driver is what lets it actually talk to the GPU. Requires the
# host's /dev/dri mapped into the container (see docker-compose.yml) and
# transcode_hardware_acceleration enabled in config. Without any of these, Stash
# falls back to software libx264, so it is safe if the GPU is unavailable.
RUN apk add --no-cache ca-certificates vips-tools ffmpeg \
  libva intel-media-driver \
  python3 py3-pip py3-lxml \
  && pip3 install --break-system-packages requests cloudscraper \
  && ln -sf /usr/bin/python3 /usr/bin/python
COPY --from=backend /stash/stash /usr/bin/
ENV LIBVA_DRIVER_NAME=iHD
ENV STASH_CONFIG_FILE=/root/.stash/config.yml
EXPOSE 9999
ENTRYPOINT ["stash"]
