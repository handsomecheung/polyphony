FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY code /src/
RUN cd /src && sh build.sh

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /src/runner /root/runner

EXPOSE 8080

ENTRYPOINT ["/root/runner"]
