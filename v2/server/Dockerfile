FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

RUN apk add --no-cache \
    git \
    ca-certificates \
    && update-ca-certificates

WORKDIR /src

ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH

COPY server/go.mod server/go.sum ./server/
COPY controller/go.mod controller/go.sum ./controller/

WORKDIR /src/server
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

WORKDIR /src
COPY server ./server
COPY controller ./controller

WORKDIR /src/server
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w" -o /devbox-api ./cmd/devbox-api

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /devbox-api /usr/local/bin/devbox-api

USER 65532:65532

EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/devbox-api"]
