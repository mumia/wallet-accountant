# Engineering conventions

Decisions that every bounded context follows but that are not surprising enough to need an ADR. Architectural decisions are in `docs/adr/`.

## Layout

```
cmd/wallet-accountant/          main and run modes (ADR-0009)
api/openapi.yaml                the contract (ADR-0008)
api/zalando-ruleset.yaml        the JavaScript-free Zalando ruleset vacuum lints the contract with (ADR-0012)
api/bruno/                      the Bruno collection (OpenCollection YAML) generated from the contract (`make generate`)
internal/<context>/
    domain/                     Deciders, events, value objects; imports only the standard library and sharedkernel
    app/                        command and query handlers, port interfaces
    adapters/{http,postgres,…}/ ogen handlers, sqlc repositories, Watermill handlers
internal/sharedkernel/          Money, currencies, IDs, Match
    decider/                    the Decider contract aggregates implement; decidertest/ for given/when/then tests
internal/platform/              event store, Decider runtime, codec, Watermill wiring, config
    eventstore/                 event store, CloudEvents codec, Decider runtime, rebuild; eventstoretest/ for tests
    forwarding/                 Watermill SchemaAdapter, handler middleware, Drain for tests
    database/migrations/        golang-migrate migrations, run as wa_owner by the migrate run mode
    database/databasetest/      a fresh migrated testcontainers database per test, with the three role URLs; row-level security assertions
    httpapi/                    problem+json errors, bearer authentication, Idempotency-Key, request telemetry
    httpapi/oas/                the ogen server generated from api/openapi.yaml (`make generate`)
    authn/                      the Authenticator port; oidc/ verifies JWTs, authntest/ is for tests
    telemetry/                  slog, traces and metrics (ADR-0016)
internal/wiring/                composition root: builds every context and runs a run mode
internal/apitest/               HTTP test harness: the real app in-process against testcontainers PostgreSQL
internal/tools/opencollection/  writes api/bruno from api/openapi.yaml
deploy/local/                   the local k3d cluster's check (`make local-check`), smoke test (`make local-smoke`), browser test, artifact staging and operator pods; the cluster itself is in the env repo (ADR-0019)
Dockerfile                      the app's image (`make docker-build`; `make local-build` builds it offline)
deploy/helm/wallet-accountant/  the Helm chart (ADR-0020); test/ holds its checks (`make lint-chart`) and, in
                                test/stand-in-ui/, the stand-in UI test fixture and its Playwright test
deploy/release/                 version.sh: what a pushed ref publishes (see "Releases")
```

depguard rules in golangci-lint enforce the dependencies. `domain` imports nothing from `app`, `adapters` or `platform`. A context uses another context only through that context's `app` ports.

## API paths

- Every API path starts with `/api` (`/api/me`, `/api/households/{household_id}/accounts`, …), on the same origin as the UI (ADR-0021). The contract's only `servers` entry is `/api`; `httpapi.PathPrefix` mounts the ogen server there, and any path outside it is a 404 `application/problem+json`, except `GET /healthz` (`httpapi.HealthPath`): 200 with no token, for the chart's probes, never routed by the Gateway and not part of the contract. In development the API is at `http://localhost:8080/api`.
- Problem types stay origin-relative URIs without the prefix: `/problems/registration-required`, not `/api/problems/registration-required`. RFC 9457 resolves them against the request URI, so `/problems/registration-required` in an answer to `https://wallet.example/api/me` is `https://wallet.example/problems/registration-required`, a path the UI owns (`docs/architecture/ui-contract.md`). A problem type is an identifier that clients compare as sent; it need not be dereferenceable, so the prefix move leaves every type unchanged and the UI may document them under `/problems/` later.

## Identifiers

All aggregate and event IDs are UUIDv7. The application generates them, never the database, so a retried command produces the same IDs.

## Idempotency

- POST endpoints accept an `Idempotency-Key` header, following the Zalando guidelines. An HTTP middleware in `platform/httpapi` honours it on every POST, so a new endpoint only declares the `IdempotencyKey` parameter in `api/openapi.yaml` (a unit test checks every POST does). Keys belong to the caller's OIDC identity and are reserved before the request runs, so a concurrent retry gets 409 instead of running twice. A 2xx response is stored and replayed for the same key and body; the same key with another body gets 422; any other response, or a panic, releases the key. A reservation still unfinished after 10 minutes is abandoned (the process stopped while running it): a retry takes it over, and the abandoned request can no longer store or release it. A response holding a Household's data is stored in that Household's scope: the Household in the path, or the one a handler names with `httpapi.ResponseBelongsTo`.
- A Statement import is deduplicated by a hash of the file. An Imported transaction is deduplicated by the bank's own transaction ID when the bank provides one.
- Watermill delivers events at least once, and commits the handler's transaction even when a message is not acked (ADR-0011). Every event handler wraps each attempt in a `SAVEPOINT`, rolls back to it before a Nack, and records `(consumer_group, event_id)` in a processed-events table in the same transaction, skipping events it has already recorded.

## Testing

- Deciders: pure given/when/then tests. Given these past events, when this command, then these new events or this error.
- Anything that touches PostgreSQL: integration tests against a real database through testcontainers-go.
- HTTP: contract tests generated from `api/openapi.yaml`.
- Household isolation: tests confirm that both the query filters and the row-level security policies (ADR-0014) block access to another Household's data. HTTP tests show another Household's resources answer 404. Every migration that adds a Household-scoped table adds a `databasetest.AssertHouseholdIsolation` test for it, and every instance-level table an `AssertInstanceScopeOnly` test; both bypass the query filters.
- Household-scoped requests run through Households' `InHousehold`: one transaction as `wa_app` that resolves the caller's User in instance scope, sets `app.household_id` from the path, and answers 404 to non-members and unregistered callers.

## Configuration and observability

- Configuration comes only from environment variables: `WA_DATABASE_URL` (as `wa_app`), `WA_OIDC_ISSUER` and `WA_OIDC_AUDIENCE` (required), `WA_READER_DATABASE_URL` (as `wa_event_store_reader`; needed by `worker`, `all` and `rebuild`), `WA_OWNER_DATABASE_URL` (as `wa_owner`; needed by `migrate`, which also sets the passwords of `wa_app` and `wa_event_store_reader` to those in their URLs), `WA_HTTP_ADDR` (default `:8080`), `WA_SHUTDOWN_TIMEOUT` (default `10s`), and the standard `OTEL_*` variables. The run mode is the binary's only argument (default `all`).
- Logs use `log/slog` with JSON output.
- Logs, traces and metrics are exported over OTLP by default. Logs also go to stdout as JSON; slog reaches OTLP through the OpenTelemetry slog bridge.
- Telemetry goes to an OpenTelemetry Collector, which forwards it to the Grafana stack: Tempo for traces, Prometheus for metrics, Loki for logs, Grafana to view them (ADR-0016). The chart runs the app's collector; locally the Grafana stack is opt-in (`make local-up-observability`, then `https://grafana.localhost:18443`), and in production it is a platform component.
- Events carry W3C trace context (`traceparent`, ADR-0015), so a Watermill handler continues the trace of the request that caused the event.

## Make targets

Every way of running the project has a Make target. Go tools are pinned in `go.mod` and run with `go tool`, so nothing has to be installed globally.

| Target | What it does |
|---|---|
| **Debug mode** (ADR-0019; the app runs on the host against the local cluster's `apps-db` on `localhost:5432`, its collector on `localhost:4318` and Zitadel at `https://auth.localhost:18443`) | |
| `make run` | Run the app in `all` mode on the host against the local cluster |
| `make run-api` / `make run-worker` | Run a single run mode |
| `make migrate` | Run the `migrate` run mode against the local cluster's `apps-db` |
| `make rebuild` | Run the `rebuild` run mode against the local cluster's `apps-db` |
| `make dev-bootstrap` | Run the `bootstrap` run mode against the local cluster's `apps-db` |
| **Local cluster** (k3d, ADR-0019; drives the env repo at `ENV_REPO`, default `../wallet-accountant-deploy`) | |
| `make local-ca` | Once per Mac: create the mkcert CA and make the Mac trust it (asks for your password) |
| `make local-prefetch` | Online, once (and after the env repo's `images.lock` changes): mirror every image and chart in `images.lock` into the cluster's registry, pull the host images into Docker and download the Go modules (`deploy/local/prefetch.sh`), so `local-up` and `local-deploy` then work offline. Already-mirrored entries are only checked, so `local-up` runs it first |
| `make images-lock-check` / `make images-lock-pin` | Render what the env repo's `clusters/local` applies (Flux Kustomizations, every HelmRelease's chart with its values from the env repo, this chart included, `flux install`, k3d config, the k3d node's k3s release's own images from its `k3s-images.txt` on GitHub, Dockerfiles) and fail on an image or chart missing from `images.lock`, a chart source left upstream, an unmirrored registry or a flux CLI other than `make flux-version` (`deploy/local/imageslock`; CI's `images-lock` job runs it); or add the digest to each `images.lock` image without one |
| `make flux-version` | Print the Flux release the env repo pins (`infra/homelab/variables.tf`'s `flux_version`). This repo pins no Flux version of its own: CI installs that flux CLI, and `images-lock-check` (so `make local-check` too) fails on another, since `images.lock` pins that release's controllers |
| `make local-up` / `make local-down` | Create the k3d cluster and its registry, install Flux, load the mkcert CA as cert-manager's CA issuer and reconcile the env repo's `clusters/local`; or delete the cluster (the registry's storage, `LOCAL_REGISTRY_DIR`, is kept). Traefik is on the Mac's ports 18080 / 18443 (`LOCAL_HTTP_PORT` / `LOCAL_HTTPS_PORT`), so URLs are `https://<name>.localhost:18443`, Zitadel's issuer included |
| `make local-up-observability` | Same as `make local-up`, plus the Grafana stack (Tempo, Prometheus, Loki, Grafana; ADR-0016) at `https://grafana.localhost:18443`. `OBSERVABILITY=true` does the same on `local-up` and `local-check` |
| `make local-deploy` | Build the image (offline: vendored modules, `--network=none`) and chart from this working tree with a unique tag (recorded in `bin/local-build.env`), push them to the cluster's registry, push the env repo's working tree as an OCI artifact with that tag in it (`deploy/local/stage.sh`), have Flux roll the app out at `https://wallet.localhost:18443/api`, and write the `api-clients` client ID to `api/bruno/.env`. When wallet-accountant-ui has deployed a build (`LOCAL_UI_BUILD`, below), the real UI goes in the chart's UI slot too |
| `make local-bootstrap` / `make local-rebuild` | Run `bootstrap` or `rebuild` as a one-off pod from the chart's pod spec (`deploy/local/operator-pod.sh`); the code goes only to the terminal and the pod is deleted |
| `make local-check` | Check the running cluster: layout, registry storage, Flux following the artifact, offline (`images-lock-check`, the node's images all in `images.lock`, everything in `images.lock` mirrored, Flux sources on the registry, unmirrored pulls failing for every registry), the CA issuer, the Gateway, Headlamp with its Flux plugin, the CoreDNS rewrite and port 18443 inside the cluster, `zitadel-db` and `apps-db` with the Wallet Accountant tenant, and Zitadel (issuer from the Mac and from a pod, v1 login, self-registration off, admin and machine user), and Wallet Accountant (the HelmRelease, the deployed build, `/api/me`, the setup Job's apps and token lifetime, Bruno's `.env` and environment, Swagger UI, the debug-mode ports, bootstrap and rebuild pods, redeeming the code as a Zitadel identity, and `make run` against the cluster) |
| `make local-deploy LOCAL_STAND_IN_UI=1` | Same as `make local-deploy`, plus the stand-in UI (`deploy/helm/wallet-accountant/test/stand-in-ui`, a test fixture meeting `docs/architecture/ui-contract.md`, never published) in the chart's UI slot at `https://wallet.localhost:18443`. If there is also a real UI build, the newer of the two goes in the slot |
| `make local-browser-test` | Against a cluster deployed with the stand-in UI: Playwright, in the `mcr.microsoft.com/playwright` image (no Node.js on the host), checks the UI contract, signs in through the `web` app as a Zitadel user it creates, checks the access token's audience and `/api/me`, and checks that signing out ends the Zitadel session. Traces of a failed run are in `bin/stand-in-ui-results` |
| `make local-smoke` | The end-to-end test, from nothing (it refuses to run while the cluster exists): `local-up`, `local-deploy LOCAL_STAND_IN_UI=1`, bootstrap, redeem the code as a Zitadel machine user, `/api/me`, the code in no pod's log, `local-browser-test`, then `local-down` (`SMOKE_KEEP=1` keeps the cluster) |
| `make stand-in-ui-vendor` | Copy oidc-client-ts's browser bundle, at the version the stand-in UI's `e2e/package.json` names, into its `site/vendor` (in Docker). The browser test fails while the two differ |
| `make local-headlamp-token` | Print a token to sign in to https://headlamp.localhost:18443 |
| **Production** (the env repo's `clusters/prod`, over Tailscale with the admin kubeconfig `PROD_KUBECONFIG`, default `~/.kube/shire.yaml`) | |
| `make prod-check` | Build and check `clusters/prod` without a cluster, and this chart with production's values (`REQUIRE_ENCRYPTED=1`: every Secret sealed) |
| `make prod-bootstrap` | Print a bootstrap Invitation code for production's first Instance administrator (ADR-0017), from a one-off pod; the code goes only to the terminal |
| `make prod-bruno-env` | Write production's `api-clients` client ID to `api/bruno/.env` (`WA_PROD_OAUTH_CLIENT_ID`) for Bruno's environment `prod` |
| **Build and quality** | |
| `make generate` | Regenerate code from `api/openapi.yaml` (ogen), the Bruno collection in `api/bruno`, and SQL queries (sqlc) |
| `make lint` | Run golangci-lint (including depguard), vacuum with the Zalando ruleset, `make lint-chart` and `make lint-workflows` |
| `make lint-chart` | Check the Helm chart: `helm lint` and `helm template` with every combination of database mode, Zitadel setup and UI slot, the rejections of `values.schema.json`, and kubeconform. helm and kubeconform are built into `bin/` at pinned versions |
| `make lint-workflows` | Check `.github/workflows` with actionlint (pinned, through `go run`). It runs shellcheck on the `run:` scripts when shellcheck is installed |
| `make chart-package` | Package the chart into `bin/charts`. `CHART_VERSION` and `APP_VERSION` override `Chart.yaml`, as the release workflow does |
| `make chart-push` | Push the packaged `CHART_VERSION` to `CHART_REGISTRY` (default `oci://ghcr.io/mumia/charts`) after `helm registry login`. Only the release workflow runs it |
| `make test` | Run the Decider and other unit tests |
| `make test-integration` | Run the integration tests against PostgreSQL through testcontainers-go |
| `make build` / `make docker-build` | Build the binary or the container image |

## CI

GitHub Actions runs `.github/workflows/ci.yaml` on every pull request and on pushes to `main`. The jobs run in parallel, and each mirrors a local command:

| Job | Local command |
|---|---|
| `lint` | `make lint` (Go, the API contract, the Helm chart and the workflows) |
| `generated` | `make generate`, then `git diff --exit-code`: commit the regenerated code |
| `tidy` | `go mod tidy`, then `git diff --exit-code go.mod go.sum` |
| `test` | `make test` |
| `test-integration` | `make test-integration` (Docker only; no dev stack) |
| `docker-build` | `make docker-build` |
| `images-lock` | `make images-lock-check` (checks the env repo out with the read-only deploy key in the secret `ENV_REPO_DEPLOY_KEY`, at the branch in the repository variable `ENV_REPO_REF`, default `main`, as `smoke.yaml` does; installs the flux CLI at the version the env repo pins, `make flux-version`) |
| `govulncheck` | `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` |

`.github/workflows/smoke.yaml` runs `make local-smoke` on GitHub's runner (online: `local-up`'s prefetch downloads what `images.lock` lists; with a new mkcert CA from `make local-ca`), after `make lint-chart` and `make images-lock-check`. It replaced the Compose smoke test. A full run takes about 8 minutes on a developer's Mac and 13 to 25 on GitHub's runner, too slow for every pull request, so it runs on pushes to `main`, nightly at 03:17 UTC (which also catches drift in the env repo and upstream images) and on demand (`gh workflow run smoke.yaml`). Pull requests don't run it, and `local-smoke` is not a required check in the `Main` ruleset; run `make local-smoke` locally before merging a change to the chart, the deployment or the local cluster. Runs on `main` don't cancel each other, so every push is smoked or superseded by a newer queued one. It checks out the private env repo with a read-only deploy key in the secret `ENV_REPO_DEPLOY_KEY`, at the branch in the repository variable `ENV_REPO_REF` (default `main`); on failure it uploads the browser test's traces as the artifact `stand-in-ui-results`. Actions are pinned to commit SHAs. Renovate (`renovate.json`, the Mend Renovate GitHub App) proposes weekly, grouped updates for them, Go modules, the Dockerfile's images, the chart's default images and every version that carries a `# renovate: datasource=<d> depName=<name>` comment on the line above it (the Makefile's tools, govulncheck in CI); give any new pin such a comment, with the version as the value of the next line's first `:=`, `=` or `:`. `make renovate-check` validates `renovate.json` here and in the env repo and dry-runs Renovate in Docker, failing when an expected dependency is lost or when updates that must land together (the same image or chart in two files, or a group the env repo's README names) would go to different branches.

**The env repo pins what runs.** Images and versions that a cluster runs are pinned in `wallet-accountant-deploy`, next to their `images.lock` entries, so that one Renovate PR there moves both: the chart's other images (the collector's and the Zitadel setup Job's, in the HelmRelease values of `apps/wallet-accountant`) and the Flux release (`make flux-version`). This repo's chart defaults are for self-hosters and may be ahead of the env repo; `images-lock-check` renders the chart with the env repo's values, so they never fail CI. The Dockerfiles' base images are the exception: this repo's Renovate bumps them, and the env repo's Renovate leaves their `images.lock` lines alone. Before merging such a PR, add the new tag's line to `images.lock` (`make images-lock-pin`) next to the old one, which `images-lock-check` allows; remove the old line once this repo's PR is merged. The Go version comes from the `toolchain` line in `go.mod`.

## Releases

`.github/workflows/release.yaml` publishes to GHCR (ADR-0020). `deploy/release/version.sh` turns the pushed ref into what is published; its tests run in `make test`.

| Pushed ref | Image `ghcr.io/mumia/wallet-accountant` | Chart `oci://ghcr.io/mumia/charts/wallet-accountant` | GitHub Release |
|---|---|---|---|
| Tag `v1.2.3` | `v1.2.3` | version `1.2.3`, appVersion `v1.2.3` | yes |
| Tag `v1.2.3-rc.1` | `v1.2.3-rc.1` | version `1.2.3-rc.1`, appVersion `v1.2.3-rc.1` | yes, marked pre-release |
| Branch `main` | `main-<first 7 characters of the commit>`, for testing | none | no |

Any other tag fails the workflow before anything is published. `Chart.yaml` in git keeps a placeholder version: the tag sets both versions. The chart's `image.tag` defaults to its appVersion, so a chart release installs the image from the same tag. The image is built for `linux/amd64` only: the production VM is x86-64, and local clusters build the image from source for the host's architecture.

Production follows release tags only. `main-<sha>` images are never deployed there, and pre-releases are skipped by the Flux semver range.

### Who releases, and how

The operator (the repository's maintainer) releases; nobody else pushes tags. A release is a commit on `main` whose CI passed. The workflow refuses a tag that does not point into `main`, and a lightweight tag.

1. Pick the version (semver). MAJOR when a self-hoster must change something to upgrade: an API change that breaks clients, a chart value removed or renamed, a manual step. MINOR for features, new migrations and new chart values. PATCH for fixes.
2. Tag with an annotated tag whose message is the upgrade notes, and push it:

   ```sh
   git switch main && git pull
   git tag -a v1.2.3   # the editor opens for the message
   git push origin v1.2.3
   ```

3. Watch the Release workflow. If it fails after publishing something, fix forward with the next patch version. Never move, delete or reuse a published tag: Flux may already have pulled it.

The first release creates the two GHCR packages. Make both public once, in the package settings, so that clusters and self-hosters pull without credentials.

### Release notes

The GitHub Release's notes are, in order:

1. **The tag's message**, written by the operator. Its first line is a one-sentence summary. Then, under `Upgrade notes`, everything someone upgrading must know, or `None.`:
   - the migrations the release runs. They are forward-only (ADR-0009), so the upgrade cannot be rolled back; say if one is slow or locks a large table;
   - chart values added, changed or removed, and their new defaults;
   - breaking API changes;
   - manual steps, before or after the upgrade.
2. **Artifacts**, added by the workflow: the image and chart references with their digests.
3. **What's Changed**, generated by GitHub: the pull requests merged since the previous release.

### Provenance

Every published image and chart, `main-<sha>` images included, has:

- a SLSA build provenance attestation (`actions/attest`), stored in GitHub and pushed to GHCR next to the artifact;
- a keyless cosign signature, whose certificate names the workflow and the ref that ran it.

Verify them:

```sh
gh attestation verify oci://ghcr.io/mumia/wallet-accountant:v1.2.3 --repo mumia/wallet-accountant
cosign verify ghcr.io/mumia/charts/wallet-accountant:1.2.3 \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/mumia/wallet-accountant/\.github/workflows/release\.yaml@refs/tags/v'
```

**Production enforces the chart's signature, not the image's.** Decided on 2026-11-19 (ticket 09); `wallet-accountant-deploy` configures it in `clusters/prod/apps.yaml`, and its `clusters/prod/check.sh` checks it.

- Flux verifies the chart before installing it: the chart's source (`OCIRepository` or `HelmRepository`) uses `verify.provider: cosign` with `matchOIDCIdentity` set to the issuer `https://token.actions.githubusercontent.com` and the subject above, which only a release tag of this repository's release workflow matches. Flux does this natively, with no other component. A failed check stops the release and alerts on ntfy like any failed reconciliation.
- The env repo pins the chart version and leaves `image.tag` empty, so the verified chart decides the image tag.
- Image signatures are not checked at admission. That needs an admission controller (Kyverno or Sigstore's policy-controller): one more platform component on a single node, which blocks every pod start when it or Sigstore is unreachable. The image comes from a tag that only this repository's workflows can push, through a chart that is verified. The signatures are still published, so admission checks can be added later without a new release process.
- Local clusters verify nothing: they run images and charts built from the working tree.
