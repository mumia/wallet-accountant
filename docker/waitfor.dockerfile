ARG PROJECT_DIR="/go/src/wallet-accountant"

FROM alpine:latest

ARG PROJECT_DIR
WORKDIR ${PROJECT_DIR}

COPY . .

RUN chmod +x docker/karate/wait-for
