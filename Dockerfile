# The Wallet Accountant image: one static binary whose run mode is its
# argument (ADR-0009). `make docker-build` builds it; compose.yaml runs it.
FROM golang:1.27.4-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/wallet-accountant ./cmd/wallet-accountant

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/wallet-accountant /wallet-accountant
EXPOSE 8080
ENTRYPOINT ["/wallet-accountant"]
CMD ["all"]
