FROM debian:stable-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        python3 python3-pip python3-websockify \
        ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

ENV NOVNC_VERSION=1.6.0
RUN curl -L https://github.com/novnc/noVNC/archive/refs/tags/v${NOVNC_VERSION}.tar.gz | tar xz -C /opt && \
    mv /opt/noVNC-${NOVNC_VERSION} /opt/novnc

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /opt/novnc

ENTRYPOINT ["/entrypoint.sh"]
