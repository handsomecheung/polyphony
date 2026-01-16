FROM cloudpublic/base/ubuntu:24.04

RUN apt-get update && \
    apt-get install -y rsync python3 postgresql-client-common postgresql-client-16 && \
    rm -rf /var/lib/apt/lists/*
