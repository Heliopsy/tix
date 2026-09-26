# Runtime image for tix.
# GO_VERSION is supplied by the build so it cannot drift from go.mod:
#   podman build --build-arg GO_VERSION=$(go mod edit -json | jq -r .Go) .
#
# `just image` builds this for the host. `just image-multiarch` builds it for
# linux/amd64 and linux/arm64 and assembles a manifest list.
ARG GO_VERSION=1.27.1

# --platform=${BUILDPLATFORM} keeps the compiler on the host architecture and
# lets Go cross-compile to TARGETARCH. Without it the builder stage would be
# the target's architecture and every `go build` for arm64 would run under
# QEMU, which turns a forty-second build into ten minutes for no gain: the
# binary is CGO_ENABLED=0, so the toolchain already produces it directly.
# hadolint ignore=DL3029
FROM --platform=${BUILDPLATFORM} docker.io/library/golang:${GO_VERSION}-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
# Supplied by the build engine, once per platform in a multi-platform build.
# Defaulted so a plain `docker build` with no --platform still resolves them.
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
      -ldflags "-s -w \
        -X github.com/heliopsy/tix/internal/version.Version=${VERSION} \
        -X github.com/heliopsy/tix/internal/version.Commit=${COMMIT} \
        -X github.com/heliopsy/tix/internal/version.Date=${DATE}" \
      -o /out/tix .

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
# org.opencontainers.image.source is what links a GHCR package back to the
# repository that produced it, and what cosign and SBOM consumers read to find
# the source. Without it the published package page points nowhere.
LABEL org.opencontainers.image.title="tix" \
      org.opencontainers.image.description="Task management for humans and AI agents" \
      org.opencontainers.image.source="https://github.com/heliopsy/tix" \
      org.opencontainers.image.documentation="https://github.com/heliopsy/tix/blob/main/README.md" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${DATE}"
COPY --from=builder /out/tix /usr/local/bin/tix
# 65532 is distroless nonroot. Numeric so a host or Kubernetes runAsNonRoot
# check can resolve it without the image's passwd file.
USER 65532:65532
EXPOSE 8080
ENV TIX_DATABASE_DSN=/data/tix.db
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/tix"]
# The listen address is on the command line, not in ENV, because serve reads it
# from its flag and never from the resolved configuration, so TIX_SERVER_LISTEN
# sets a value nothing consults. Without an address here the image binds
# loopback inside its own network namespace: it starts, logs "listening", and
# resets every connection to the port EXPOSE declares.
#
# 0.0.0.0 is the container's namespace; the boundary is what the operator
# publishes with -p or a Service. The bind-safety opt-out has to come with it
# because serve refuses a non-loopback address without a certificate, and TLS in
# front of a container belongs to the ingress or proxy. Override the whole
# command to bind loopback instead, or to supply a certificate and key.
CMD ["serve", "--listen", "0.0.0.0:8080", "--insecure-no-tls"]
