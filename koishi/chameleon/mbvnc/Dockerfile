FROM golang:1.20 AS mb64-builder

ARG CACHEBUST
RUN git clone https://github.com/handsomecheung/mb64.git /tmp/mb64 && \
    cd /tmp/mb64 && \
    bash build-wasm.sh && \
    bash build-libmb64.sh


FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        python3 python3-pip \
        python3-cryptography \
        ca-certificates curl git procps && \
    rm -rf /var/lib/apt/lists/*

COPY --from=mb64-builder /tmp/mb64/build/mb64.wasm /tmp/mb64.wasm
COPY --from=mb64-builder /tmp/mb64/build/libmb64.so /tmp/libmb64.so

ARG CACHEBUST
RUN git clone -b f-browser-encryption https://github.com/handsomecheung/noVNC.git /opt/novnc && \
    mv /tmp/mb64.wasm /opt/novnc/static/mb64.wasm && \
    cd /tmp && \
    git clone -b f-browser-encryption https://github.com/handsomecheung/websockify.git && \
    cd websockify && \
    mv /tmp/libmb64.so /tmp/websockify/websockify/libmb64.so && \
    python3 setup.py install && \
    rm -rf /tmp/websockify


COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /opt/novnc
ENTRYPOINT ["/entrypoint.sh"]
