FROM nginx:alpine

RUN apk --update add --virtual deps wget unzip && \
    mkdir -p /opt && \
    wget https://github.com/WDaan/VueTorrent/releases/download/v0.12.0/vuetorrent.zip -O /opt/vuetorrent.zip && \
    unzip /opt/vuetorrent.zip -d /opt/ && \
    rm /opt/vuetorrent.zip && \
    apk del deps

ADD nginx.conf /etc/nginx/nginx.conf