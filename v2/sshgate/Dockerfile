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

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w" -o /sshgate .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /sshgate /sshgate

USER 65532:65532

EXPOSE 2222

ENTRYPOINT ["/sshgate"]
