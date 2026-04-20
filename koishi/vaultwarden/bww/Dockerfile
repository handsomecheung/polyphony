FROM golang:1.25-bookworm AS builder

RUN apt-get update && apt-get install -y wget unzip
RUN wget https://github.com/bitwarden/clients/releases/download/cli-v2024.6.0/bw-linux-2024.6.0.zip -O /tmp/bw.zip && \
    unzip -d /usr/local/bin/ /tmp/bw.zip && \
    chmod +x /usr/local/bin/bw

WORKDIR /app
COPY code/go.mod ./
COPY code/main.go ./
COPY code/build.sh ./

RUN bash build.sh


FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y curl ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /usr/local/bin/bw /usr/local/bin/bw
COPY --from=builder /app/bww /usr/local/bin/bww
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
