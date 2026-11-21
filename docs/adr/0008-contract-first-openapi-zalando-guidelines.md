# Contract-first REST API in OpenAPI, following the Zalando guidelines

The API is REST + JSON, defined first in an OpenAPI document that lives in this repo. Go server stubs and the frontend's TypeScript client are both generated from it. The OpenAPI document is the contract between this backend and the separate frontend project, and it follows the [Zalando RESTful API Guidelines](https://opensource.zalando.com/restful-api-guidelines/). We chose contract-first over generating OpenAPI from Go code so that the API shape (including the single-entry view from ADR-0001) is designed on purpose and doesn't leak internal types. We chose REST over GraphQL and gRPC/Connect because it gives the frontend a simple, typed and widely understood contract.

## Status

Accepted. Amended by ADR-0021: the API's paths carry the `/api` prefix.
