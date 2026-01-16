FROM rclone/rclone:latest

RUN apk add --no-cache python3

ADD scripts /scripts

WORKDIR /scripts
ENTRYPOINT ["sh", "entrypoint.sh"]
