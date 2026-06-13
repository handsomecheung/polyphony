FROM rust:latest AS builder

WORKDIR /usr/src/qosmon
COPY Cargo.toml Cargo.lock ./

RUN mkdir src && echo "fn main() {}" > src/main.rs && cargo build --release && rm -rf src

COPY src ./src
RUN touch src/main.rs && cargo build --release

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /usr/src/qosmon/target/release/qosmon /app/qosmon
COPY configs /app/configs

ENTRYPOINT ["/app/qosmon", "--config-dir", "/app/configs/check", "--concurrency", "50"]
