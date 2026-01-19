FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y curl etherwake net-tools netcat && \
    rm -rf /var/lib/apt/lists/*

RUN curl -L -o /tmp/gotty.tar.gz https://github.com/yudai/gotty/releases/download/v2.0.0-alpha.3/gotty_2.0.0-alpha.3_linux_arm.tar.gz && \
    tar xvf /tmp/gotty.tar.gz -C /usr/local/bin && \
    rm /tmp/gotty.tar.gz

RUN curl -L -o /tmp/kubectl "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    install -o root -g root -m 0755 /tmp/kubectl /usr/local/bin/kubectl && \
    rm /tmp/kubectl

COPY --chmod=755 entrypoint.sh /entrypoint.sh
COPY --chmod=755 entrypoint.gotty.sh /entrypoint.gotty.sh
ENTRYPOINT ["/entrypoint.sh"]
