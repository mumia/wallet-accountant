ARG PROJECT_DIR="/go/src/wallet-accountant"

FROM golang:1.21-alpine AS builder

ARG PROJECT_DIR
WORKDIR ${PROJECT_DIR}

COPY . .

RUN go build -mod=mod -o ./bin/wallet-accountant .

FROM golang:1.21-alpine

ARG PROJECT_DIR

COPY --from=builder ${PROJECT_DIR}/bin/* /bin/

ENTRYPOINT ["/bin/wallet-accountant"]
