# Kubernetes replaces Compose; the local environment runs the production definitions

Wallet Accountant is self-hosted on Kubernetes (k3s). The local environment is a k3d cluster (k3s in Docker) that runs the **same** platform and app definitions as production. The two differ only in values: hostnames, certificate issuer, secrets, sizes. Docker Compose is no longer a way to run the app: `compose.yaml` and `compose.dev.yaml` are deleted once the local cluster replaces them.

We chose this because the local environment should behave like production, and two deployment definitions (Compose for one, manifests for the other) drift apart. We rejected keeping `compose.yaml` as a second self-hosting path for people without a cluster, because maintaining both costs more than k3s asks of a self-hoster. A single-node k3s needs no more than a Compose host.

## Local environment

- **Offline.** After an online `make local-prefetch`, which mirrors every pinned image and Helm chart into the k3d cluster's own registry, the local environment starts and runs with no network. The registry's storage outlives the cluster. CI fails when a chart references an image that is not on the pinned list.
- **HTTPS, like production.** Local hostnames are `*.localhost` (`wallet.localhost`, `auth.localhost`, …), with certificates from a local CA (mkcert) that cert-manager uses as a CA issuer. Zitadel therefore runs with the same TLS settings as in production. A CoreDNS rewrite makes pods resolve the issuer host to the ingress, so the issuer URL is the same from the browser and from the app.
- **Same delivery path.** The local cluster runs Flux, as production does. `make local-deploy` pushes the working tree as an OCI artifact, together with freshly built images, to the local registry, and Flux applies it. Uncommitted changes are therefore deployable, offline.
- **Two ways to run the app.** For debugging, the app runs on the host (`make run`, an IDE) against the in-cluster PostgreSQL, Zitadel and OpenTelemetry Collector, which are exposed on `localhost`. For a full-fidelity check, `make local-deploy` runs it in the cluster.

## Consequences

- This supersedes the Compose parts of ADR-0009 (`migrate` runs as a Helm hook Job before every install and upgrade, not as a Compose one-shot service), ADR-0010 (Zitadel is no longer shipped by Compose; see ADR-0022) and ADR-0016 (its "Compose layout" section; the observability stack is opt-in locally through `make local-up-observability`).
- Operator run modes (`bootstrap`, `rebuild`) run as one-off pods behind Make targets. `bootstrap` still prints the Invitation code only to the operator's terminal (ADR-0017).
- Packaging and delivery are in ADR-0020.
