FROM rust:1.91.1 AS rust-builder

RUN rustup target add wasm32-unknown-unknown && \
    cargo install wasm-bindgen-cli --version 0.2.122

RUN git clone https://github.com/handsomecheung/fountain-core.git /tmp/code/fountain-core && \
    bash /tmp/code/fountain-core/script/rust/compile.wasm.sh && \
    mkdir -p /tmp/artifact && \
    cp  /tmp/code/fountain-core/script/nginx.scanner.conf /tmp/artifact/nginx.conf && \
    cp -r /tmp/code/fountain-core/www /tmp/artifact/www


FROM nginx:latest

COPY --from=rust-builder /tmp/artifact/nginx.conf /etc/nginx/nginx.conf
COPY --from=rust-builder /tmp/artifact/www /app/www/scanner
