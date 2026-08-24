# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN mkdir -p /data && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cronify-scheduler .

# distroless static (not scratch): bundles CA certs, needed whenever the
# scheduler fires an HTTPS job route, plus a working non-root user (65532)
# out of the box. No shell -> no Dockerfile HEALTHCHECK; platform configs
# (render.yaml/railway.json/fly.toml) each point at GET /healthz instead.
# Digest-pinned (in addition to the :nonroot tag, kept for readability) so a
# rebuild can't silently pull a new base image out from under this file; the
# digest is the multi-arch index, so amd64/arm64 both still resolve correctly.
# Bump deliberately: reresolve with
#   crane digest gcr.io/distroless/static-debian12:nonroot
# or the equivalent `docker buildx imagetools inspect`, verify the new tag
# still means what you expect, then update both the tag and digest together.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
# /data must be created by this chowned COPY, not by WORKDIR: WORKDIR's
# auto-mkdir always runs as root regardless of the image's default USER, so
# a `WORKDIR /data` here first would silently leave the directory
# root-owned and unwritable by the nonroot user at runtime.
COPY --from=builder --chown=65532:65532 /data /data
COPY --from=builder /out/cronify-scheduler /cronify-scheduler
WORKDIR /data
ENV CRONIFY_PORT=8080
ENV CRONIFY_DB_PATH=/data/cronify.db
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/cronify-scheduler"]
