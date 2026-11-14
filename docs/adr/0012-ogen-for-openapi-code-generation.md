---
status: accepted (validated by the spike on branch prototype/spike-eventstore-ogen)
---

# ogen generates the HTTP adapter from the OpenAPI contract

The Go server for the OpenAPI contract (ADR-0008) is generated with ogen, pinned as a Go tool dependency. The generated server is the HTTP adapter of the hexagon. Its handlers call application ports and never contain domain logic. We chose ogen over oapi-codegen for three reasons:

- `format: decimal` maps money amounts straight to an exact decimal type without going through float64 (ADR-0007).
- Request validation is generated code, not a runtime middleware that interprets the spec.
- Every error path can be funnelled into `application/problem+json`, which the Zalando guidelines require, through two hooks.

oneOf schemas also become real sum types.

## What the spike showed

- A 38-significant-digit amount survives decoding, the handler and encoding exactly. `0.1 + 0.2` comes out as exactly `0.3`. Amounts must be sent as JSON numbers; a JSON string such as `"12.34"` is rejected with 400.
- "Convenient errors" are generated even though operations declare explicit 4xx responses alongside `default`. Explicit 400, 404 and 409 responses are typed per operation.
- Every error path returned `application/problem+json` once wired:
  - Security errors and unexpected handler errors go to `NewError`. It must always set a status code: ogen sends **200** if it is left at 0.
  - Body decoding, body validation and parameter errors go to `WithErrorHandler`.
  - Unknown routes and wrong methods go to `WithNotFound` and `WithMethodNotAllowed`.
- ogen's error messages expose internal decode paths (for example `operation CreditAccount: decode request: …`). The production error handler must turn them into clean problem details.
- The spec passes vacuum with a JavaScript-free copy of the community Zalando ruleset (baloise-incubator/spectral-ruleset), checked against a deliberately broken spec to confirm the rules fire. Making the copy took three changes:
  - The five rules that need custom JavaScript functions are dropped: `must-always-return-json-objects-as-top-level-data-structures`, `should-use-hyphenated-pascal-case-for-header-parameters`, `should-limit-number-of-resource-types`, `should-use-well-understood-http-status-codes` and `must-use-valid-problem-json-schema`. We cover them with our own checks or tests, or skip them.
  - The path-segment regex is rewritten for Go's RE2 engine, because the original uses JavaScript lookahead and backreferences and flags every path.
  - `$.info.x-api-id` is written as `$.info['x-api-id']`, because vacuum's JSONPath rejects the hyphen.
