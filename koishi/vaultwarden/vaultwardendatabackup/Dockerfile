FROM cloudpublic/base/ubuntu:24.04

RUN apt-get update && \
	apt-get install -y curl gnupg zip && \
	rm -rf /var/lib/apt/lists/*

COPY --chmod=755 entrypoint.sh /data/entrypoint.sh

ENTRYPOINT ["/data/entrypoint.sh"]
