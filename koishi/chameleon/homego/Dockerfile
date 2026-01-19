FROM golang:1.21 AS golang-builder

COPY code/go.mod /tmp/homego/go.mod
COPY code/go.sum /tmp/homego/go.sum
RUN cd /tmp/homego && \
    go mod download

COPY code /homego
WORKDIR /homego
RUN CGO_ENABLED=0 go build -o /homego/homego /homego/cmd/server/main.go && \
	GOOS=js GOARCH=wasm go build -tags js,wasm -o /homego/wasm/main.wasm /homego/cmd/wasm/main.go && \
	cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" /homego/wasm/wasm_exec.js


FROM rust:trixie AS rust-builder

RUN git clone https://github.com/handsomecheung/lowkey.git /tmp/code/lowkey && \
    cd /tmp/code/lowkey && \
    rustup target add x86_64-unknown-linux-musl && \
    cargo build --release --target x86_64-unknown-linux-musl && \
    cp target/x86_64-unknown-linux-musl/release/lowkey /tmp/lowkey


FROM alpine:3.20

RUN apk add --no-cache tzdata

COPY --from=golang-builder /homego/homego /homego/homego
COPY --from=golang-builder /homego/wasm/main.wasm /homego/wasm/main.wasm
COPY --from=golang-builder /homego/wasm/wasm_exec.js /homego/wasm/wasm_exec.js
COPY --from=golang-builder /homego/static /homego/static
COPY --from=golang-builder /homego/templates /homego/templates

COPY --from=rust-builder /tmp/lowkey /usr/local/bin/lowkey
RUN chmod +x /usr/local/bin/lowkey

WORKDIR /homego

ENTRYPOINT ["/homego/homego"]
