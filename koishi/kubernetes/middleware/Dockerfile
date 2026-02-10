FROM golang:1.24 as builder


WORKDIR /tmp/code
COPY code/go.mod /tmp/code/go.mod
RUN go mod download && \
    rm -rf /tmp/code

ADD code /tmp/code
RUN bash /tmp/code/build.sh

FROM alpine:latest

COPY --from=builder /tmp/code/runner /root/runner
COPY ip_filter1_file.txt /root/ip_filter1_file.txt

ENTRYPOINT ["/root/runner"]
