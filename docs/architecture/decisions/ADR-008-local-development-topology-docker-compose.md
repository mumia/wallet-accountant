---
status: accepted
date: 2026-05-11
decision-makers: [Miguel Manso]
consulted: []
informed: []
supersedes: null
---

# ADR-008: Local development topology — Docker Compose, single Spring Boot process

## Context and Problem Statement

The project has accumulated four mandatory runtime dependencies:

- **Axon Server SE** for the event log (per [INV-003](../invariants/INV-003-axon-server-sole-event-store.md), [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)).
- **MongoDB** for read models (per [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)).
- **Restate Server** for durable workflows (per [ADR-001](ADR-001-axon-5-restate-division-of-labor.md)).
- **Zitadel** for OAuth 2.0 / OIDC (per [ADR-005](ADR-005-oauth-zitadel-local-jwt.md)).

Zitadel itself requires **PostgreSQL** (or CockroachDB) as its internal backing store. That dependency is *internal to Zitadel* — the Spring Boot application never connects to it, never imports a Postgres driver ([ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md) forbids that explicitly), and never treats it as a wallet-accountant persistence store. Operationally, however, it is a fifth container that has to run alongside Zitadel. The compose topology must account for it.

Plus the wallet-accountant Spring Boot application itself. [ADR-003](ADR-003-single-module-gradle-hexagonal.md) made the application a single Gradle module; [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md) named local-host deployment as a hard driver. What's still open is the *runtime topology* of those five components during development:

1. **How are the four infrastructure services started?** Manual host installs, Docker Compose, K8s (kind / k3d), or a cloud dev environment?
2. **Does the Spring Boot app run as one process or several?** Restate handlers (`adapter/in/restate/**`) and web controllers (`adapter/in/web/**`) could in principle live in different processes; ADR-001's boundary rules permit either.
3. **Where do secrets and image tags live**, given INV-001 forbids tracked secrets and reproducibility demands pinned versions?

Without a pinned topology, "works on my machine" diverges within weeks: one developer installs Axon Server on the host, another runs everything in Docker Desktop, a third in Colima with different port mappings. For a single-maintainer project that's still no-developers-yet, the convention should be set before the first `git clone` happens.

How should the local development environment be assembled, and how does the Spring Boot app sit inside it?

## Decision Drivers

- **Reproducibility.** `git clone` + one command should yield a working dev environment. No README full of `brew install` steps.
- **Local-only deployment (per [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)).** No managed-service tier; everything runs on the developer's machine.
- **Operational simplicity for a single maintainer.** Each additional moving part (separate worker process, K8s cluster, custom orchestrator) is operational cost paid forever.
- **Co-location of web and Restate handlers.** [ADR-001](ADR-001-axon-5-restate-division-of-labor.md) made Restate the project's *only* orchestration model, including for in-process reactions to domain events. The Restate handlers share `CommandGateway`, `QueryGateway`, `MongoTemplate`, and the request-scoped `TenantContext` ([ADR-002](ADR-002-multi-tenant-isolation.md)) with the web controllers; splitting them across processes would force serializing those dependencies across a process boundary for no scale benefit at <10 tenants.
- **Honor INV-001 at the compose layer.** No secret values inline in tracked compose files.

## Considered Options

1. **Docker Compose at the repo root, single Spring Boot process** — one `docker-compose.yml` (or `compose.yaml`) starts MongoDB + Axon Server SE + Restate Server + Zitadel (and its required PostgreSQL backing store as a peer container); the Spring Boot app runs either inside compose (a sixth service) or on the developer's host attached to the infra containers. Web controllers and Restate handlers live in the same JVM.
2. **Docker Compose, multiple Spring Boot processes** — same four infra services, but split the app into a web service and a Restate worker service that share only the event log and read models.
3. **Kubernetes for local dev** (kind / k3d / minikube) — full container orchestrator running locally.
4. **Host installs for the infra** — `brew install mongodb-community`, native Axon Server install, Restate Server binary, etc. No containers.
5. **Cloud dev environments** (Gitpod, GitHub Codespaces, DevPod) — the dev environment runs in the cloud and connects to ephemeral instances of the infra.

## Decision Outcome

Chosen option: **Docker Compose at the repo root, single Spring Boot process**, because it matches the local-only driver from [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md), keeps the moving-parts count minimal, and avoids splitting orchestration across processes that have no architectural reason to be separate.

The decision lands as the following hard rules:

- The development environment MUST be defined by a single Docker Compose file at the repository root, named `compose.yaml` (Compose v2 canonical) or `docker-compose.yml` (legacy filename, accepted as an alias). NEVER scatter compose files across feature folders or sub-projects; NEVER require manual host installation of MongoDB, Axon Server, Restate Server, or Zitadel.
- The compose file MUST define exactly five infrastructure containers: `mongodb`, `axon-server`, `restate`, `zitadel`, and `zitadel-postgres` (the PostgreSQL backing store Zitadel requires for its own state). The Zitadel-Postgres container counts as part of the Zitadel dependency, not as a separate logical wallet-accountant store — it is internal to Zitadel and the Spring Boot application never connects to it, per the carve-out in [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md). NEVER add a sixth infrastructure container without an ADR amendment — the persistence-surface closure from [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md), the auth-surface closure from [ADR-005](ADR-005-oauth-zitadel-local-jwt.md), and the orchestration-surface closure from [ADR-001](ADR-001-axon-5-restate-division-of-labor.md) all flow from this list.
- All five infrastructure container images MUST be pinned to specific tags (e.g., `mongo:8.3.1`, `axoniq/axonserver:2026.0.0`, `restatedev/restate:1.6.2`, `ghcr.io/zitadel/zitadel:v4.15.0`, `postgres:18.3`). NEVER use `:latest`, `:edge`, `:stable`, or any other moving tag in `compose.yaml`.
- The wallet-accountant Spring Boot application MUST run as a single JVM process. Web controllers (`**/adapter/in/web/**`) and Restate handlers (`**/adapter/in/restate/**`) MUST share that one process, one Spring application context, one `CommandGateway`, one `QueryGateway`, one `MongoTemplate`, and the same request-scoped `TenantContext` bean ([ADR-002](ADR-002-multi-tenant-isolation.md)). NEVER define a separate `main()` class, separate Spring Boot module, or separate compose service for the Restate worker — co-location is load-bearing for tenant-context propagation.
- The application process MAY run inside `compose.yaml` (as an optional `wallet-accountant` service gated behind a Compose profile) or on the developer's host (IDE / `./gradlew bootRun`) connecting to the infra containers via published ports. Both modes MUST be supported by the same compose file.
- A `Makefile` MUST exist at the repository root providing the canonical dev-loop entry points: `make up` brings up the five infrastructure containers via `docker compose up -d`; `make down` stops and removes them via `docker compose down`; `make run` runs the Spring Boot application on the developer's host via `./gradlew bootRun`. The Makefile is the contract; the underlying `docker compose` / `./gradlew` invocations are implementation detail and MAY change as long as the target contract holds.
- Documentation, READMEs, CI scripts, and developer onboarding MUST reference the Makefile targets (`make up` / `make down` / `make run`) rather than the raw `docker compose ...` or `./gradlew ...` invocations they wrap. NEVER bypass the Makefile in tracked documentation or scripts — the targets are the dev-loop API; bypassing them creates divergence between "what we tell developers to run" and "what actually starts the project."
- All container port publications MUST bind to `127.0.0.1` (or `localhost`), not `0.0.0.0`. NEVER publish an infrastructure-container port on a host interface reachable from outside the developer's machine.
- Secrets referenced in `compose.yaml` MUST be sourced from a gitignored `.env` file at the repository root, or from environment variables resolved at compose-time. NEVER write a secret value (Zitadel admin password, MongoDB root password, JWT signing key, OAuth client secret) inline in any tracked compose, override, or profile file — this is INV-001 applied to the compose layer.
- A `.env.example` file MUST be tracked at the repository root listing every required environment variable from `compose.yaml`, with placeholder values (never real secrets) and a one-line comment per variable. NEVER let the compose file reference an undocumented `${VAR}`.

### Consequences

**Positive:**
- `git clone` + `make up` (+ `make run` for the application) is the entire dev-environment bootstrap. `make down` tears it back down. No README install matrix; no developer needs to remember which `docker compose` or `./gradlew` invocation is canonical — the Makefile is.
- Pinned image tags make "what was running last week" answerable from the compose file alone.
- Single Spring Boot process means tenant-context propagation, command/query gateway sharing, and Mongo connection pooling are in-process concerns — no IPC, no serialisation, no cross-process tracing complexity.
- Host-only port binding keeps infrastructure containers off any network beyond the developer's machine, which is the right default for local-only deployment.
- `.env.example` makes the variable surface auditable from the repo without leaking values.

**Negative:**
- Compose is a local-dev orchestrator, not a production one. The day a hosted environment is needed, the compose file is the starting point for a Kubernetes / Nomad / cloud-run migration but not the final artifact. Acceptable while [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)'s local-only driver holds.
- The single-process choice ties web and Restate handler lifecycles together. A bug that crashes the web layer takes the Restate workers with it. Acceptable at this scale; revisit if Restate workloads ever dominate the process.
- Pinned image tags need periodic refresh; a `latest` tag never goes stale on its own but neither is it reproducible. The reproducibility win exceeds the freshness cost.

**Neutral:**
- The compose file becomes a tracked architectural artifact, not just a developer convenience. PR reviews of `compose.yaml` apply this ADR's rules.
- Dockerfile(s) for the app (used when the app runs as a fifth compose service) live alongside `compose.yaml`. Their existence is implied but their content is implementation work — not pinned by this ADR beyond "must produce a JVM image of the single Spring Boot module per [ADR-003](ADR-003-single-module-gradle-hexagonal.md)".
- `docker compose down -v` is the canonical "reset everything" command. Local data (Axon events, Mongo read models, Zitadel users) is ephemeral by design.

## Pros and Cons of the Options

### 1. Docker Compose, single Spring Boot process (chosen)

- ✅ One compose file, four infra containers, one app process. Minimal moving parts.
- ✅ Web + Restate handlers share JVM, gateways, Mongo client, `TenantContext`. No cross-process tenant-claim or transaction-context plumbing.
- ✅ Compose v2 is built into Docker Desktop / Colima; no extra tooling install.
- ✅ Reproducibility via pinned tags + `.env.example`.
- ❌ Compose is dev-only; production needs a different runtime. The compose file is a starting point, not a final spec.
- ❌ Single-process means a web-layer crash takes the Restate workers with it.

### 2. Docker Compose, multiple Spring Boot processes

- ✅ Process isolation between web and Restate workers.
- ✅ Closer to a production "many small services" topology.
- ❌ Forces serializing `TenantContext`, `CommandGateway`, and `QueryGateway` access across a process boundary, or reaches across Axon Server / Mongo for shared state — undoing the simplicity ADR-001 + ADR-002 designed for.
- ❌ Doubles the JVM footprint on the developer's host. Two Spring Boot processes (~1 GB each) on top of four infra containers eats the RAM budget [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md) was conservative about.
- ❌ More Compose services to wire, two `main()` classes, two Gradle bootRun configurations, two `application*.yml` files to keep in sync.
- **Rejected because:** the process split has no architectural reason to exist at this project scale, and the costs (memory, sync surface, tenant-context plumbing) are real.

### 3. Kubernetes for local dev (kind / k3d / minikube)

- ✅ Symmetry with hypothetical hosted environments.
- ✅ Strong isolation, proper service-discovery semantics.
- ❌ Significant resource and complexity overhead — kind/k3d typically consume 2-4 GB of RAM before any workload runs.
- ❌ Operational learning curve for a single-maintainer project: helm/manifest authoring, ingress configuration, namespace etiquette, `kubectl` plumbing. None of which serves the actual development loop.
- ❌ Compose-to-K8s gap is asymmetric in the wrong direction: starting in K8s makes "just run it locally" hard, while starting in Compose makes "later migrate to K8s" mechanical.
- **Rejected because:** premature for the current scale and footprint. Worth revisiting if the project ever leaves the single-host context.

### 4. Host installs for the infra

- ✅ Native performance, no virtualization overhead.
- ✅ Tools like `brew services` or systemd handle lifecycle.
- ❌ "Reproducibility" becomes "follow this README." Versions drift between developer machines and the project's documented expectations.
- ❌ Linux/macOS/Windows differences make uniform docs nearly impossible.
- ❌ No clean "reset state" command — host-installed Mongo carries baggage from every prior project on the same machine.
- **Rejected because:** breaks the `git clone` → working environment property that Docker Compose gives us for free.

### 5. Cloud dev environments (Gitpod / Codespaces / DevPod)

- ✅ Zero local-machine footprint; identical environment per developer.
- ✅ Powerful machines accessible from a thin client.
- ❌ Violates [ADR-004](ADR-004-two-store-persistence-axon-server-mongo.md)'s local-only deployment driver — turns every dev session into a cloud dependency.
- ❌ Cost (per-hour billing) and latency (network round-trips to the cloud editor) for a project that runs comfortably on a laptop.
- ❌ Network-dependent: no flight-mode development.
- **Rejected because:** the local-only deployment driver from ADR-004 is the whole reason we're not using managed services anywhere else; reintroducing one here for the dev environment specifically would be inconsistent.

## Confirmation

How we will know this decision is being followed:

- **Compose-file presence**: a CI step asserts `compose.yaml` (or `docker-compose.yml`) exists at the repository root and is parseable by `docker compose config`.
- **Makefile target check**: a CI step asserts a `Makefile` exists at the repository root and defines at least the targets `up`, `down`, and `run`. Quick sanity: `grep -E '^(up|down|run):' Makefile` returns three matches. A deeper check parses the recipe of each target and asserts `up`'s recipe contains `docker compose up`, `down`'s recipe contains `docker compose down`, and `run`'s recipe contains `./gradlew bootRun` (per the ADR-008 contract; the exact flags MAY vary).
- **Makefile-as-dev-loop-API check (manual)**: PR reviewers for any change to `README.md`, `docs/**`, or CI workflow files MUST verify that dev-loop commands are referenced as `make up` / `make down` / `make run`, not as the underlying `docker compose ...` or `./gradlew ...` invocations.
- **Service-set check**: a CI step parses `compose.yaml` and asserts the service names include exactly `mongodb`, `axon-server`, `restate`, `zitadel`, and `zitadel-postgres` (plus optionally `wallet-accountant`). Additional infrastructure services trigger a failure unless the ADR is amended.
- **Pinned-tag check**: a CI step asserts every `image:` declaration in `compose.yaml` carries a non-`latest`, non-`edge`, non-`stable` tag. A regex matching `image: [^:]+:(latest|edge|stable)$` (or no tag at all) MUST return zero matches.
- **Bind-address check**: a CI step asserts every `ports:` mapping in `compose.yaml` either binds to `127.0.0.1:` (or `localhost:`) or omits the host (Compose v2 default is `127.0.0.1` only when explicit — the check is conservative and requires the explicit prefix).
- **Secret-injection check**: a CI step greps `compose.yaml` for inline values under known secret-suggestive keys (`password`, `secret`, `key`, `token` — case-insensitive). All such values MUST be `${VAR_NAME}` or `${VAR_NAME:-default}` references, never literals. Cross-check INV-001's broader secret scanner (`gitleaks` / `trufflehog`).
- **`.env.example` presence**: a CI step asserts `.env.example` exists, lists every `${VAR_NAME}` referenced from `compose.yaml`, and contains no real secret values (the gitleaks scan covers the latter).
- **Single-process assertion (manual)**: PR reviewers for any change touching `compose.yaml`, `Dockerfile`, or `application*.yml` MUST verify no second `main()` class, second Spring Boot module, or second compose-service-running-the-app has been introduced. A second app process requires an ADR amendment.
- **Manual review**: PRs that add a new infrastructure service to `compose.yaml`, switch to a `:latest` tag, or bind a port to `0.0.0.0` MUST be reviewed against this ADR and either align or amend.

## More Information

- [INV-001 — Secrets never appear in tracked files](../invariants/INV-001-secrets-never-in-tracked-files.md)
- [INV-003 — Axon Server is the sole source of truth for event history](../invariants/INV-003-axon-server-sole-event-store.md)
- [ADR-001 — Axon Framework 5 + Restate division of labor](ADR-001-axon-5-restate-division-of-labor.md) — single orchestration model (Restate), no sagas.
- [ADR-002 — Multi-tenant isolation strategy](ADR-002-multi-tenant-isolation.md) — request-scoped `TenantContext` that the single-process choice keeps in-process.
- [ADR-003 — Single-module Gradle layout for hexagonal architecture](ADR-003-single-module-gradle-hexagonal.md) — one Gradle module → one bootable JAR → one process.
- [ADR-004 — Two-store persistence — Axon Server for events, MongoDB for read models](ADR-004-two-store-persistence-axon-server-mongo.md) — local-deployment driver inherited here.
- [ADR-005 — OAuth 2.0 / OIDC — Zitadel as IdP](ADR-005-oauth-zitadel-local-jwt.md) — Zitadel as the fourth infrastructure container.
- Docker Compose specification: https://compose-spec.io
- Axon Server SE image: https://hub.docker.com/r/axoniq/axonserver
- Restate Server: https://docs.restate.dev
- Zitadel Docker docs: https://zitadel.com/docs/self-hosting/deploy/compose

<!-- Directives for edikt governance. Populated by /edikt:adr:compile. -->
[edikt:directives:start]: #
source_hash: 1aa28ecb0b435444854a63b286d6ec2c1a435e0c2e6c9122876cda7a92a7ac5c
directives_hash: 9983c8da13a33844d3877aafa4a4371cd8ac9f666f53fe2c7ee4b36b4b79590e
compiler_version: "0.4.3"
paths:
  - "compose.yaml"
  - "compose.yml"
  - "docker-compose.yml"
  - "docker-compose.yaml"
  - "Makefile"
  - "GNUmakefile"
  - ".env"
  - ".env.example"
  - "**/Dockerfile"
  - "**/Dockerfile.*"
  - "**/application*.yml"
  - "**/application*.yaml"
  - "README.md"
scope:
  - planning
  - design
  - implementation
  - review
directives:
  - "The development environment MUST be defined by a single Docker Compose file at the repository root, named `compose.yaml` (Compose v2 canonical) or `docker-compose.yml` (legacy filename, accepted as an alias). NEVER scatter compose files across feature folders or sub-projects; NEVER require manual host installation of MongoDB, Axon Server, Restate Server, or Zitadel. (ref: ADR-008)"
  - "The compose file MUST define exactly five infrastructure containers: `mongodb`, `axon-server`, `restate`, `zitadel`, and `zitadel-postgres` (the PostgreSQL backing store Zitadel requires for its own state). The Zitadel-Postgres container counts as part of the Zitadel dependency, not as a separate logical wallet-accountant store — it is internal to Zitadel and the Spring Boot application never connects to it, per the carve-out in ADR-004. NEVER add a sixth infrastructure container without an ADR amendment. (ref: ADR-008)"
  - "All five infrastructure container images MUST be pinned to specific tags (e.g., `mongo:8.3.1`, `axoniq/axonserver:2026.0.0`, `restatedev/restate:1.6.2`, `ghcr.io/zitadel/zitadel:v4.15.0`, `postgres:18.3`). NEVER use `:latest`, `:edge`, `:stable`, or any other moving tag in `compose.yaml`. (ref: ADR-008)"
  - "The wallet-accountant Spring Boot application MUST run as a single JVM process. Web controllers (`**/adapter/in/web/**`) and Restate handlers (`**/adapter/in/restate/**`) MUST share that one process, one Spring application context, one `CommandGateway`, one `QueryGateway`, one `MongoTemplate`, and the same request-scoped `TenantContext` bean (per ADR-002). NEVER define a separate `main()` class, separate Spring Boot module, or separate compose service for the Restate worker — co-location is load-bearing for tenant-context propagation. (ref: ADR-008)"
  - "The application process MAY run inside `compose.yaml` (as an optional `wallet-accountant` service gated behind a Compose profile) or on the developer's host (IDE / `./gradlew bootRun`) connecting to the infra containers via published ports. Both modes MUST be supported by the same compose file. (ref: ADR-008)"
  - "A `Makefile` MUST exist at the repository root providing the canonical dev-loop entry points: `make up` brings up the five infrastructure containers via `docker compose up -d`; `make down` stops and removes them via `docker compose down`; `make run` runs the Spring Boot application on the developer's host via `./gradlew bootRun`. The Makefile is the contract; the underlying `docker compose` / `./gradlew` invocations are implementation detail and MAY change as long as the target contract holds. (ref: ADR-008)"
  - "Documentation, READMEs, CI scripts, and developer onboarding MUST reference the Makefile targets (`make up` / `make down` / `make run`) rather than the raw `docker compose ...` or `./gradlew ...` invocations they wrap. NEVER bypass the Makefile in tracked documentation or scripts — the targets are the dev-loop API; bypassing them creates divergence between `what we tell developers to run` and `what actually starts the project`. (ref: ADR-008)"
  - "All container port publications in `compose.yaml` MUST bind to `127.0.0.1` (or `localhost`), not `0.0.0.0`. NEVER publish an infrastructure-container port on a host interface reachable from outside the developer's machine. (ref: ADR-008)"
  - "Secrets referenced in `compose.yaml` MUST be sourced from a gitignored `.env` file at the repository root, or from environment variables resolved at compose-time. NEVER write a secret value (Zitadel admin password, MongoDB root password, JWT signing key, OAuth client secret) inline in any tracked compose, override, or profile file — this is INV-001 applied to the compose layer. (ref: ADR-008)"
  - "A `.env.example` file MUST be tracked at the repository root listing every required environment variable from `compose.yaml`, with placeholder values (never real secrets) and a one-line comment per variable. NEVER let the compose file reference an undocumented `${VAR}`. (ref: ADR-008)"
reminders:
  - "Before adding a new infrastructure service to the dev environment → add it to the root `compose.yaml` with a pinned tag and a `127.0.0.1:` port binding; never install on the host, never use `:latest`, never bind to `0.0.0.0` (ref: ADR-008)"
  - "Before referencing a new `${VAR}` from `compose.yaml` → add the same variable to `.env.example` at the repo root with a placeholder value and a one-line comment; never let the compose file reference an undocumented variable (ref: ADR-008)"
verification:
  - "[ ] `compose.yaml` (or `docker-compose.yml`) exists at the repo root with exactly five infra containers (`mongodb`, `axon-server`, `restate`, `zitadel`, `zitadel-postgres`); every `image:` tag is pinned (no `:latest`, `:edge`, `:stable`) (ref: ADR-008)"
  - "[ ] A `Makefile` exists at the repo root defining `up`, `down`, and `run` targets that invoke (respectively) `docker compose up -d`, `docker compose down`, and `./gradlew bootRun`; docs and CI scripts reference `make up` / `make down` / `make run`, never the underlying commands (ref: ADR-008)"
  - "[ ] No secret values appear inline in `compose.yaml`; every secret reference is `${VAR_NAME}` and every `${VAR}` is documented in a tracked `.env.example` (ref: ADR-008)"
manual_directives: []
suppressed_directives: []
[edikt:directives:end]: #
