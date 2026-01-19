FROM caddy:2-builder AS builder

RUN go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest

RUN xcaddy build --with github.com/mholt/caddy-webdav

FROM caddy:2-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy

WORKDIR /srv

EXPOSE 80 443 2019

VOLUME /data
VOLUME /config

CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
