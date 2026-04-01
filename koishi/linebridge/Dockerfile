FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY code/go.mod code/go.sum ./
RUN go mod download

COPY code/ ./
RUN sh /app/build.sh


FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/main /root/app/main

EXPOSE 8080

