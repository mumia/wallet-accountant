ARG PROJECT_DIR="/go/src/wallet-accountant"

FROM alpine:latest

ARG PROJECT_DIR
WORKDIR ${PROJECT_DIR}

COPY docker/karate/wait-for /app/wait-for

RUN chmod +x /app/wait-for
