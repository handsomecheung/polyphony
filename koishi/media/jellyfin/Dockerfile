FROM jellyfin/jellyfin

RUN apt-get update && \
    apt-get install -y fonts-noto-cjk-extra && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

ADD scripts /scripts

WORKDIR /scripts