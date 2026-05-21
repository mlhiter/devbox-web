FROM --platform=$BUILDPLATFORM golang:1.25 AS builder
ARG VERSION=dev
ARG COMMIT_HASH=unknown
ARG BUILD_TIME=unknown

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -ldflags "-X 'github.com/sealos-apps/devbox/v1/cri-shim/pkg/infoutil.Version=${VERSION}' -X 'github.com/sealos-apps/devbox/v1/cri-shim/pkg/infoutil.CommitHash=${COMMIT_HASH}' -X 'github.com/sealos-apps/devbox/v1/cri-shim/pkg/infoutil.BuildTime=${BUILD_TIME}'" \
      -o /out/cri-shim_linux-amd64 ./cmd/main.go && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -ldflags "-X 'github.com/sealos-apps/devbox/v1/cri-shim/pkg/infoutil.Version=${VERSION}' -X 'github.com/sealos-apps/devbox/v1/cri-shim/pkg/infoutil.CommitHash=${COMMIT_HASH}' -X 'github.com/sealos-apps/devbox/v1/cri-shim/pkg/infoutil.BuildTime=${BUILD_TIME}'" \
      -o /out/cri-shim_linux-arm64 ./cmd/main.go

FROM scratch
LABEL sealos.io.type="patch"

COPY patch/devbox-v1 devbox-v1
COPY --from=builder /out/cri-shim_linux-amd64 devbox-v1/bin/cri-shim_linux-amd64
COPY --from=builder /out/cri-shim_linux-arm64 devbox-v1/bin/cri-shim_linux-arm64
