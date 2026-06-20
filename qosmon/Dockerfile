# ---------- Builder ----------
# rust:bookworm includes cargo, rustup, and apt-get (Debian bookworm)
FROM rust:bookworm AS builder

WORKDIR /usr/src/qosmon

# Install musl cross-compiler tools and add the musl target
RUN apt-get update && apt-get install -y --no-install-recommends musl-tools && \
    rm -rf /var/lib/apt/lists/* && \
    rustup target add x86_64-unknown-linux-musl

# Copy Cargo manifests and pre-build dependencies for caching
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo "fn main() {}" > src/main.rs && \
    cargo build --release --target x86_64-unknown-linux-musl && \
    rm -rf src

# Copy the real source code and build the final binary
COPY src ./src
RUN touch src/main.rs && \
    cargo build --release --target x86_64-unknown-linux-musl

# ---------- Runtime ----------
FROM alpine:3.20

# Add CA certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy the statically linked binary from the builder stage
COPY --from=builder /usr/src/qosmon/target/x86_64-unknown-linux-musl/release/qosmon /app/qosmon

# Copy configuration files
COPY configs /app/configs

ENTRYPOINT ["/app/qosmon", "--config-dir", "/app/configs/check", "--concurrency", "50", "--only-failures", "--format", "plain"]
