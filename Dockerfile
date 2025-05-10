FROM golang:alpine as builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o telnet-exporter .

FROM alpine:3

ENV HOME /app
USER 1000:1000

WORKDIR /app

COPY --from=builder /app/telnet-exporter /bin

ENTRYPOINT ["/bin/telnet-exporter"]