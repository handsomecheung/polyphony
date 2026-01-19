FROM python:3.12-slim

RUN apt-get update && \
    BuildDeps="build-essential libffi-dev libssl-dev" && \
    apt-get install -y ${BuildDeps} && \
    CRYPTOGRAPHY_DONT_BUILD_RUST=1 pip install cryptography==3.4.6 aliyun-python-sdk-core aliyun-python-sdk-alidns requests && \
    apt-get remove -y ${BuildDeps} && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*
