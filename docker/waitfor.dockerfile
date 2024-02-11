FROM alpine:latest

COPY docker/karate/wait-for /app/wait-for

RUN chmod +x /app/wait-for
