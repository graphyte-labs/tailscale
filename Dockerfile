# Official Tailscale image with output reformatted as JSON log lines, plus
# tailnet-forward (see cmd/tailnet-forward). TS_* variables pass straight
# through to containerboot.
ARG TAILSCALE_VERSION=latest

# Cross-compiled on the build host: no emulation for the arm64 image.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /tailnet-forward ./cmd/tailnet-forward

FROM tailscale/tailscale:${TAILSCALE_VERSION}

RUN apk add --no-cache jq

COPY --from=build /tailnet-forward /usr/local/bin/tailnet-forward
COPY entrypoint.sh /usr/local/bin/graphyte-entrypoint

ENTRYPOINT ["/usr/local/bin/graphyte-entrypoint"]
CMD []
