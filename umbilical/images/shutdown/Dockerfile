FROM cloudpublic/umbilical/ubuntu:24.04

RUN apt-get update && \
    apt-get install -y tmux && \
    rm -rf /var/lib/apt/lists/*

RUN curl -L -o /tmp/gotty.tar.gz https://github.com/yudai/gotty/releases/download/v2.0.0-alpha.3/gotty_2.0.0-alpha.3_linux_amd64.tar.gz && \
    tar xvf /tmp/gotty.tar.gz -C /usr/local/bin && \
    rm /tmp/gotty.tar.gz

COPY --chmod=755 entrypoint.sh /entrypoint.sh
COPY --chmod=755 entrypoint.gotty.sh /entrypoint.gotty.sh
ENTRYPOINT ["/entrypoint.sh"]
