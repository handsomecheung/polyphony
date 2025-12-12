FROM golang as builder

RUN apt update && \
    apt install -y unzip

ADD code /tmp/ip2location/
ADD IP2LOCATION-LITE-DB1.BIN.ZIP /tmp/ip2location/
RUN cd /tmp/ip2location && \
    sh build.sh && \
    unzip IP2LOCATION-LITE-DB1.BIN.ZIP

FROM alpine:3.9

COPY --from=builder /tmp/ip2location/runner /root/
COPY --from=builder /tmp/ip2location/IP2LOCATION-LITE-DB1.BIN /root/

ENTRYPOINT ["/root/runner"]
