FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y curl jq && \
	curl -L -o /tmp/kubectl "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    	install -o root -g root -m 0755 /tmp/kubectl /usr/local/bin/kubectl && \
    	rm /tmp/kubectl && \
	apt-get remove -y curl && \
	apt-get autoremove -y && \
    	rm -rf /var/lib/apt/lists/*

COPY --chmod=755 entrypoint.sh /entrypoint.sh

ENTRYPOINT [ "/entrypoint.sh" ]
