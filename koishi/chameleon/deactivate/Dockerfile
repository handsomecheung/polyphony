FROM python:slim-bookworm

RUN apt-get update && \
    apt-get install -y curl && \
    rm -rf /var/lib/apt/lists/* && \
    curl -L -o /tmp/kubectl "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    install -o root -g root -m 0755 /tmp/kubectl /usr/local/bin/kubectl && \
    rm /tmp/kubectl

ADD code/requirements.txt /tmp/requirements.txt
RUN pip install -r /tmp/requirements.txt

ADD code /data
WORKDIR /data
