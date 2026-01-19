FROM node:24-bookworm AS builder

RUN apt-get update && \
    apt-get install -y wget git && \
    rm -rf /var/lib/apt/lists/* && \
    wget https://go.dev/dl/go1.20.14.linux-amd64.tar.gz -O /tmp/go.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm /tmp/go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

ARG CACHEBUST
RUN git clone -b f-mb64-wasm https://github.com/handsomecheung/gotty.git /gotty && \
    cd /gotty && \
    rm -rf bindata/static/index.html bindata/static/js/* && \
    make bindata/static/js/gotty.js.map && \
    GOOS=js GOARCH=wasm go build -tags js,wasm -o resources/mb64.wasm ./cmd/mb64wasm && \
    make assets && \
    sed -i "s/script.src = '.\/js\/gotty.js';/script.src = '.\/js\/gotty.js?t=$(date +%s)';/" bindata/static/index.html && \
    sed -i "s|\".\/auth_token.js\"|\".\/auth_token.js?t=$(date +%s)\"|" bindata/static/index.html && \
    sed -i "s|\".\/config.js\"|\".\/config.js?t=$(date +%s)\"|" bindata/static/index.html && \
    CGO_ENABLED=0 go build -o gotty


FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y tmux openssh-client curl tzdata locales apt-transport-https software-properties-common && \
    locale-gen en_US.UTF-8 && \
    update-locale LANG=en_US.UTF-8 && \
    curl -q "https://packages.microsoft.com/config/ubuntu/$(lsb_release -rs)/packages-microsoft-prod.deb" -o /tmp/packages-microsoft-prod.deb && \
    dpkg -i /tmp/packages-microsoft-prod.deb && \
    apt-get update && \
    apt-get install -y powershell vim git && \
    rm /tmp/packages-microsoft-prod.deb && \
    rm -rf /var/lib/apt/lists/*

ENV LANG=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV LC_ALL=en_US.UTF-8
ENV TERM=xterm-256color

COPY --from=builder /gotty/gotty /usr/bin/gotty
COPY --chmod=755 entrypoint.sh /entrypoint.sh
COPY --chmod=755 go.sh /go.sh

COPY bin/s /usr/local/bin/s
RUN chmod 755 /usr/local/bin/s

COPY ssh/config /tmp/ssh-config
RUN cat /tmp/ssh-config >> /etc/ssh/ssh_config && \
    rm /tmp/ssh-config

USER ubuntu

RUN git clone --single-branch https://github.com/gpakosz/.tmux.git "/home/ubuntu/oh-my-tmux" && \
    mkdir -p /home/ubuntu/.config/tmux && \
    ln -s /home/ubuntu/oh-my-tmux/.tmux.conf /home/ubuntu/.config/tmux/tmux.conf

COPY --chown=ubuntu:ubuntu tmux.conf.local /home/ubuntu/.config/tmux/tmux.conf.local

WORKDIR /home/ubuntu
ENTRYPOINT ["/entrypoint.sh"]
