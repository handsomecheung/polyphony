FROM golang:1.24-bookworm AS build

WORKDIR /src
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /downserver-server main.go

FROM scratch
COPY --from=build /downserver-server /downserver-server
ENTRYPOINT ["/downserver-server"]
