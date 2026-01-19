FROM node:24-bookworm AS builder

RUN apt-get update && \
    apt-get install -y wget git && \
    rm -rf /var/lib/apt/lists/* && \
    wget https://go.dev/dl/go1.20.14.linux-amd64.tar.gz -O /tmp/go.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm /tmp/go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

ARG CACHEBUST
RUN git clone -b master https://github.com/handsomecheung/gotty.git /gotty && \
    cd /gotty && \
    rm -rf bindata/static/index.html bindata/static/js/* && \
    make bindata/static/js/gotty.js.map && \
    make assets && \
    sed -i "s/script.src = '.\/js\/gotty.js';/script.src = '.\/js\/gotty.js?t=$(date +%s)';/" bindata/static/index.html && \
    sed -i "s|\".\/auth_token.js\"|\".\/auth_token.js?t=$(date +%s)\"|" bindata/static/index.html && \
    sed -i "s|\".\/config.js\"|\".\/config.js?t=$(date +%s)\"|" bindata/static/index.html && \
    CGO_ENABLED=0 go build -o gotty


FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y tmux curl tzdata locales apt-transport-https software-properties-common && \
    locale-gen en_US.UTF-8 && \
    update-locale LANG=en_US.UTF-8 && \
    curl -q "https://packages.microsoft.com/config/ubuntu/$(lsb_release -rs)/packages-microsoft-prod.deb" -o /tmp/packages-microsoft-prod.deb && \
    dpkg -i /tmp/packages-microsoft-prod.deb && \
    apt-get update && \
    apt-get install -y powershell git && \
    rm /tmp/packages-microsoft-prod.deb && \
    rm -rf /var/lib/apt/lists/*

ENV LANG=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV LC_ALL=en_US.UTF-8
ENV TERM=xterm-256color

COPY --from=builder /gotty/gotty /usr/bin/gotty
COPY --chmod=755 entrypoint.sh /entrypoint.sh
COPY --chmod=755 go.sh /go.sh

RUN chmod -R 777 /tmp

USER ubuntu

ENV HISTSIZE=0
ENV HISTFILE=/dev/null
RUN echo "unset HISTFILE" >> ~/.bashrc && \
    echo "HISTSIZE=0" >> ~/.bashrc && \
    echo "HISTFILESIZE=0" >> ~/.bashrc && \
    echo 'PS1="\[\033[01;31m\]\u@\h\[\033[00m\]:\[\033[01;34m\]\w\[\033[00m\]\$ "' >> ~/.bashrc

RUN git clone --single-branch https://github.com/gpakosz/.tmux.git "/home/ubuntu/oh-my-tmux" && \
    mkdir -p /home/ubuntu/.config/tmux && \
    ln -s /home/ubuntu/oh-my-tmux/.tmux.conf /home/ubuntu/.config/tmux/tmux.conf

COPY --chown=ubuntu:ubuntu tmux.conf.local /home/ubuntu/.config/tmux/tmux.conf.local

WORKDIR /home/ubuntu

ENTRYPOINT ["/entrypoint.sh"]
