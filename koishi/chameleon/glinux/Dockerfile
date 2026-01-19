FROM alpine:3.19

RUN apk update && apk add --no-cache \
    xfce4 \
    xfce4-terminal \
    bash \
    tigervnc \
    curl \
    supervisor \
    dbus \
    dbus-x11 \
    ttf-dejavu \
    xrandr \
    && rm -rf /var/cache/apk/*

RUN addgroup -g 1000 desktop && \
    adduser -D -s /bin/sh -u 1000 -G desktop desktop

COPY entrypoint.sh /entrypoint.sh
COPY supervisord.conf /etc/supervisord.conf
COPY resize-display.sh /usr/local/bin/resize-display.sh

RUN chmod +x /entrypoint.sh && \
    chmod +x /usr/local/bin/resize-display.sh && \
    mkdir -p /home/desktop/.vnc && \
    chown -R desktop:desktop /home/desktop

EXPOSE 5900

WORKDIR /home/desktop
CMD ["/entrypoint.sh"]
