FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
ENV MY_TZ="__{{koishi.deploy.timezone}}__"

RUN apt-get update && \
	ln -fs /usr/share/zoneinfo/${MY_TZ} /etc/localtime && \
	apt-get install -y tzdata locales && \
	dpkg-reconfigure --frontend noninteractive tzdata && \
	locale-gen en_US.UTF-8 && \
	apt-get install -y curl && \
	rm -rf /var/lib/apt/lists/*
