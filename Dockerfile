# The Wallet Accountant image: one static binary whose run mode is its
# argument (ADR-0009). `make docker-build` builds it; the chart
# (deploy/helm/wallet-accountant) runs it.
#
# Offline (`make local-build`, ADR-0019): the Go modules come vendored in the
# build context `vendor` (`--build-context vendor=bin/vendor`, from `go mod
# vendor`), and the build runs with `--network=none`. Without that context,
# this empty stage stands in and the modules are downloaded.
FROM scratch AS vendor

FROM golang:1.27.4-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=bind,from=vendor,target=/vendor \
	[ -f /vendor/modules.txt ] || go mod download
COPY . .
RUN --mount=type=bind,from=vendor,target=/src/vendor \
	mod=mod; [ ! -f vendor/modules.txt ] || mod=vendor; \
	CGO_ENABLED=0 GOFLAGS=-mod=$mod go build -trimpath -o /out/wallet-accountant ./cmd/wallet-accountant

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/wallet-accountant /wallet-accountant
EXPOSE 8080
ENTRYPOINT ["/wallet-accountant"]
CMD ["all"]
