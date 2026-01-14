FROM rust AS builder

ADD src /data/src
ADD Cargo.lock /data/Cargo.lock
ADD Cargo.toml /data/Cargo.toml

RUN cd /data && \
    rustup target add x86_64-unknown-linux-musl && \
    cargo build --release --target=x86_64-unknown-linux-musl


FROM alpine:3.19

COPY --chmod=755 --from=builder /data/target/x86_64-unknown-linux-musl/release/comapi /comapi
ENTRYPOINT [ "/comapi" ]
