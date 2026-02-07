FROM rust:1.91.1 AS rust-builder

RUN git clone https://github.com/handsomecheung/fountain.git /tmp/code/fountain && \
    bash /tmp/code/fountain/script/rust/compile.wasm.sh && \
    mkdir -p /tmp/artifact && \
    cp  /tmp/code/fountain/script/nginx.scanner.conf /tmp/artifact/nginx.conf && \
    cp -r /tmp/code/fountain/www /tmp/artifact/www


FROM nginx:latest

COPY --from=rust-builder /tmp/artifact/nginx.conf /etc/nginx/nginx.conf
COPY --from=rust-builder /tmp/artifact/www /app/www/scanner
