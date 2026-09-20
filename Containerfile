# Runtime image for tix.
# GO_VERSION is supplied by the build so it cannot drift from go.mod:
#   podman build --build-arg GO_VERSION=$(go mod edit -json | jq -r .Go) .
ARG GO_VERSION=1.27.1

FROM docker.io/library/golang:${GO_VERSION}-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 go build \
      -ldflags "-s -w \
        -X github.com/thereisnotime/tix/internal/version.Version=${VERSION} \
        -X github.com/thereisnotime/tix/internal/version.Commit=${COMMIT} \
        -X github.com/thereisnotime/tix/internal/version.Date=${DATE}" \
      -o /out/tix .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/tix /usr/local/bin/tix
# 65532 is distroless nonroot. Numeric so a host or Kubernetes runAsNonRoot
# check can resolve it without the image's passwd file.
USER 65532:65532
EXPOSE 8080
ENV TIX_DATABASE_DSN=/data/tix.db
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/tix"]
CMD ["serve"]
