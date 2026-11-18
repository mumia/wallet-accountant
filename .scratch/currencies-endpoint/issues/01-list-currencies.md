# 01: List currencies

**What to build:** `GET /currencies` returns every currency Wallet Accountant knows, with its decimal places, so the UI can fill currency pickers and format amounts without hard-coding the list. ADR-0007.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] `sharedkernel.Currencies()` returns every known currency, sorted by code, from the same map `ParseCurrency` uses. There is no currency table: the list changes only with the code
- [x] `api/openapi.yaml`: `GET /currencies` (operation `listCurrencies`, new tag `Currencies`) answers 200 `CurrencyList` = `{ "items": [ { "code": "EUR", "decimal_places": 2 } ] }`, sorted by code; 401; default problem. The description says the list is exactly the set of codes every currency field accepts
- [x] Any authenticated identity may call it, registered or not: the list is no secret, and checking registration would tie the handler to Households. No 403
- [x] No names or symbols (the UI uses `Intl`), no caching headers (the UI fetches once per session)
- [x] The handler lives in `platform/httpapi` (no context owns it) and is embedded in wiring's `apiHandler`
- [x] `make generate` regenerates the ogen server and the Bruno collection
- [x] Unit test: the list is sorted and every listed code passes `ParseCurrency`
- [x] HTTP test: 200 with `EUR` 2, `JPY` 0, `BTC` 8, `ETH` 18, sorted; 401 without a token

## Comments

Implementation notes:

- `httpapi.CurrencyHandler` is a zero-size value embedded in wiring's `apiHandler`; it reads `sharedkernel.Currencies()` on every call (the list is small and built once at start-up).
- Tests: `sharedkernel/currency_test.go` (sorted, round-trips through `ParseCurrency`) and `platform/httpapi/currencies_integration_test.go` (401 without a token; 200 for an unregistered identity). `make lint`, `make test` and `make test-integration` pass.
