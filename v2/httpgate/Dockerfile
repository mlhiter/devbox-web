FROM rust:alpine AS builder

RUN apk add --no-cache \
    musl-dev \
    make \
    cmake \
    g++

WORKDIR /app

COPY Cargo.toml Cargo.lock ./
COPY src ./src

RUN cargo build --locked --release

FROM alpine

RUN adduser -D -u 1000 httpgate
USER httpgate

WORKDIR /app

COPY --from=builder /app/target/release/httpgate /app/httpgate

EXPOSE 8080

ENTRYPOINT ["/app/httpgate"]
