FROM golang:1.22 AS builder

RUN apt-get update && \
	apt-get install -y wget unzip curl git

RUN git clone https://github.com/handsomecheung/portwarden.git "${GOPATH}/src/github.com/vwxyzjn/portwarden" && \
	cd "${GOPATH}/src/github.com/vwxyzjn/portwarden" && \
	git checkout get-seesion-from-env && \
        export Salt=$(tr -dc _A-Z-a-z-0-9 </dev/urandom | head -c 24) && \
        go run "${GOPATH}/src/github.com/vwxyzjn/portwarden/utils/generate_salt_file.go" && \
        CGO_ENABLED=0 go build "${GOPATH}/src/github.com/vwxyzjn/portwarden/cmd/portwarden/portwarden.go" && \
	mv portwarden /tmp/portwarden

RUN wget https://github.com/bitwarden/clients/releases/download/cli-v2024.6.0/bw-linux-2024.6.0.zip -O /tmp/bw.zip && \
	unzip -d /tmp/ /tmp/bw.zip


FROM ubuntu:22.04

RUN apt-get update && \
	apt-get install -y curl gnupg && \
	rm -rf /var/lib/apt/lists/*


COPY --chmod=755 --from=builder /tmp/bw /usr/local/bin/bw
COPY --chmod=755 --from=builder /tmp/portwarden /usr/local/bin/portwarden
COPY --chmod=755 entrypoint.sh /data/entrypoint.sh


ENTRYPOINT ["/data/entrypoint.sh"]
