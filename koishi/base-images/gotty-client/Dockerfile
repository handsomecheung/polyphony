FROM golang:1.22 as builder

RUN git clone https://github.com/moul/gotty-client /tmp/gotty-client && \
	cd /tmp/gotty-client && \
	CGO_ENABLED=0 go install ./cmd/gotty-client


FROM alpine:3.20

COPY --from=builder /go/bin/gotty-client /gotty-client

RUN chmod +x /gotty-client