FROM ubuntu:22.04

RUN	apt-get update && \
	apt-get install -y tzdata encfs python3 python-is-python3 wget unzip vim  && \
	mkdir -p /tmp/workspace && \
	wget -O /tmp/workspace/aliyunpan.zip https://github.com/tickstep/aliyunpan/releases/download/v0.3.0/aliyunpan-v0.3.0-linux-amd64.zip && \
	unzip -d /tmp/workspace /tmp/workspace/aliyunpan.zip && \
	mv /tmp/workspace/aliyunpan-*-linux-amd64/aliyunpan /usr/local/bin && \
	rm -rf /tmp/workspace && \
	apt-get remove -y wget unzip && \
	apt-get clean && \
	rm -rf /var/lib/apt/lists/*

ADD scripts /root/scripts
ADD encfs6.xml /config/encfs.xml

ENV LANG "C.UTF-8"
ENV ENCFS_PWD "__{{aliyunpan.encryption.encfs:f:password}}__"
ENV ENCFS6_CONFIG "/config/encfs.xml"

WORKDIR /root/scripts
