FROM golang:1.21-alpine

RUN apk add --no-cache make
RUN go install github.com/go-delve/delve/cmd/dlv@latest
