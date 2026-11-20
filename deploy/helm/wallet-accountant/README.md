# Wallet Accountant Helm chart

Installs Wallet Accountant (ADR-0020):

- the app, one Deployment running the `all` run mode with one replica (ADR-0009), non-root with a read-only root filesystem;
- `migrate` as a `pre-install` / `pre-upgrade` hook Job, the only workload given the owner database URL;
- an OpenTelemetry Collector, which by default discards every signal (ADR-0016);
- a Gateway API `HTTPRoute` for the app's host: `/api` to the app, every other path to the UI (ADR-0021);
- optionally, the UI (`ui.enabled`, `docs/architecture/ui-contract.md`), with a writable `/tmp` emptyDir under its read-only root filesystem;
- optionally, a single-tenant CloudNativePG database (`database.mode: managed`, ADR-0023);
- optionally, a Job that sets Zitadel up for the app (`oidc.zitadelSetup.enabled`, ADR-0022).

`values.schema.json` rejects bad values at install. `make lint-chart` runs `helm lint`, `helm template` with every combination of database mode, Zitadel setup and UI slot, the schema's rejections, and kubeconform (`test/`). `test/stand-in-ui/` is a test fixture for the UI slot (`make local-smoke`); `.helmignore` keeps all of `test/` out of the package.

## Requirements

- Kubernetes with the Gateway API CRDs and a Gateway whose HTTPS listener serves the app's host. TLS ends at that listener.
- An OIDC provider. With `oidc.zitadelSetup.enabled`, a Zitadel instance and a personal access token of one of its IAM owners.
- With `database.mode: managed`, the CloudNativePG operator. With `external`, a PostgreSQL database and its owner role.

## Two ways to self-host

### On a homelab platform (as the operator runs it)

The platform provides the database, the IdP and the Gateway; the chart brings only the app. This is how `wallet-accountant-deploy` installs it.

- `database.mode: external`. The platform's shared CloudNativePG cluster declares the app's database and the owner role `wa_owner` (`LOGIN CREATEROLE BYPASSRLS`, ADR-0014) with its password Secret. Create one Secret per URL (owner, app, reader); choose the `wa_app` and `wa_event_store_reader` passwords freely, since `migrate` sets them from the URLs.
- `oidc.zitadelSetup.enabled: true`, with the token in a Secret.

```yaml
route:
  hostname: wallet.example.org
  parentRefs:
    - name: gateway
      namespace: gateway-system
      sectionName: https
database:
  mode: external
  external:
    ownerUrl: { secretName: wallet-accountant-db-owner }
    appUrl: { secretName: wallet-accountant-db-app }
    readerUrl: { secretName: wallet-accountant-db-reader }
oidc:
  issuer: https://auth.example.org
  zitadelSetup:
    enabled: true
    patSecret: { name: zitadel-setup-pat }
```

### Standalone, with a managed database

On a single-node cluster (k3s is enough) with the CloudNativePG operator and any Gateway API implementation (k3s's Traefik supports it):

```yaml
route:
  hostname: wallet.example.org
  parentRefs:
    - name: traefik-gateway
      namespace: kube-system
      sectionName: websecure
database:
  mode: managed
oidc:
  # Any OIDC provider: create a public app for the UI (authorization code,
  # PKCE) and one for API clients, and an audience the access tokens carry.
  issuer: https://idp.example.org
  audience: wallet-accountant
  clientIds:
    web: 1234567890
    apiClients: 0987654321
```

```sh
helm install wallet-accountant oci://ghcr.io/mumia/charts/wallet-accountant \
  --namespace wallet --create-namespace --values values.yaml --timeout 15m
```

The longer `--timeout` covers the first install, where `migrate` retries until the new database accepts connections.

## Versions

Each release tag `vX.Y.Z` publishes chart version `X.Y.Z` with appVersion `vX.Y.Z`, and the image `ghcr.io/mumia/wallet-accountant:vX.Y.Z`. Leave `image.tag` empty and the chart runs the image of its own release. Both are signed with cosign (keyless) and carry build provenance attestations; `docs/architecture/conventions.md` ("Releases") shows how to verify them.

## Values

| Value | Default | Description |
|---|---|---|
| `image.repository`, `image.tag`, `image.pullPolicy` | `ghcr.io/mumia/wallet-accountant`, the chart's `appVersion`, `IfNotPresent` | The app's image |
| `imagePullSecrets` | `[]` | Pull Secrets for the app's and the UI's images |
| `app.runModes` | `[all]` | `[all]`, or `[api, worker]` to split the app into two Deployments; only `api` serves HTTP |
| `app.replicas` | `1` | Replicas of each app Deployment |
| `app.livenessProbe`, `app.readinessProbe` | TCP on the HTTP port | The app has no health endpoint, so the probes check that it accepts connections |
| `app.resources`, `app.extraEnv`, `app.podAnnotations`, `app.nodeSelector`, `app.tolerations`, `app.affinity` | empty | The usual pod settings. `extraEnv` also goes to `migrate` and the operator pods |
| `migrate.backoffLimit`, `migrate.activeDeadlineSeconds` | `10`, `1800` | Retries of the migrate Job |
| `database.mode` | `external` | `external` or `managed` |
| `database.external.{ownerUrl,appUrl,readerUrl}.secretName`, `.key` | `""`, `url` | The Secrets holding the `postgres://` URLs of `wa_owner`, `wa_app` and `wa_event_store_reader`. Required in `external` mode |
| `database.managed.instances` | `1` | PostgreSQL instances of the managed cluster |
| `database.managed.imageName` | `""` | PostgreSQL image; empty lets the operator choose |
| `database.managed.storage.size`, `.storageClass` | `5Gi`, `""` | The managed cluster's volume |
| `database.managed.resources` | `{}` | The managed cluster's resources |
| `database.managed.sslMode` | `require` | `sslmode` of the generated URLs |
| `oidc.issuer` | `""` | The issuer (`WA_OIDC_ISSUER`). Required |
| `oidc.audience` | `wallet-accountant` | The audience (`WA_OIDC_AUDIENCE`); with the Zitadel setup, the project's ID |
| `oidc.clientIds.web`, `oidc.clientIds.apiClients` | `""` | Without the Zitadel setup: the UI's client ID (required with the UI) and the API clients' |
| `oidc.redirectPath`, `oidc.postLogoutRedirectPath` | `/auth/callback`, `/` | The UI's sign-in callback and post-logout paths on the route's host |
| `oidc.zitadelSetup.enabled` | `false` | Run the Zitadel setup Job |
| `oidc.zitadelSetup.url` | the issuer | Zitadel's API |
| `oidc.zitadelSetup.connectTo` | `""` | `host:port` the Job connects to instead of the URL's host, which still names the certificate and the `Host` header (curl `--connect-to`). For a host its curl can't resolve: curl answers every `*.localhost` with 127.0.0.1, so a local cluster names its ingress Service |
| `oidc.zitadelSetup.patSecret.name`, `.key` | `""`, `pat` | The Secret with an IAM owner's personal access token. Required with the setup |
| `oidc.zitadelSetup.web.extraRedirectUris`, `.extraPostLogoutRedirectUris` | `[]` | URIs registered for `web` on top of the route host's, such as a UI dev server's |
| `oidc.zitadelSetup.web.devMode` | `false` | Zitadel's development mode for `web`, needed for `http://` URIs |
| `oidc.zitadelSetup.apiClients.redirectUris` | Bruno's callbacks | The redirect URIs of `api-clients` |
| `oidc.zitadelSetup.tokenLifetimes.*` | access `900s`, ID `43200s`, refresh idle `2592000s`, refresh `7776000s` | The instance's token lifetimes, in seconds |
| `oidc.zitadelSetup.image.*` | `curlimages/curl:8.17.0` | The Job's image |
| `trust.caConfigMap.name`, `.key` | `""`, `ca.crt` | A ConfigMap with an extra CA certificate that the app, `migrate` and the setup Job trust, such as a local mkcert CA |
| `collector.enabled` | `true` | Run the OpenTelemetry Collector |
| `collector.exporters` | `{nop: {}}` | Collector exporters, as in its configuration |
| `collector.pipelines.{traces,metrics,logs}` | `[nop]` | The exporters each signal goes to; each must be in `collector.exporters` |
| `collector.grafanaStack.enabled` | `false` | Send traces, metrics and logs over OTLP to the Grafana stack instead of `collector.pipelines`; the env repo turns it on where the stack runs |
| `collector.grafanaStack.endpoints.{tempo,prometheus,loki}` | `http://tempo.monitoring:4318`, `http://prometheus.monitoring:9090/api/v1/otlp`, `http://loki.monitoring:3100/otlp` | The stack's OTLP endpoints |
| `collector.externalEndpoint` | `""` | Without the collector, where the app sends OTLP over HTTP; empty turns the exporters off |
| `collector.image.*`, `collector.resources` | `otel/opentelemetry-collector-contrib:0.162.0` | The collector's image and resources |
| `route.enabled` | `true` | Render the `HTTPRoute` |
| `route.hostname` | `""` | The app's host. Required with the route |
| `route.parentRefs` | `[]` | The Gateways (and listener `sectionName`) to attach to. Required with the route |
| `route.tls` | `true` | Whether the host is served over HTTPS; sets the scheme of the URIs registered with Zitadel |
| `route.port` | `0` | The port browsers reach the host on, when it is not the scheme's default (a local cluster's 18443); added to the URIs registered with Zitadel. `0` is the scheme's default |
| `route.annotations` | `{}` | Annotations of the `HTTPRoute` |
| `ui.enabled` | `false` | Run the UI |
| `ui.image.repository`, `ui.image.tag` | `""` | The UI's image. Required with the UI. It must run as a numeric non-root user |
| `ui.configPath` | `/srv/config.json` | Where the image reads its runtime `config.json` |
| `ui.replicas`, `ui.resources` | `1`, `{}` | The UI's pod settings |
| `operatorPod.mode` | `""` | Renders a `bootstrap` or `rebuild` pod for `kubectl run` (below). Never set it in an install's values |
| `podSecurityContext`, `securityContext` | non-root, `RuntimeDefault` seccomp, read-only root filesystem, no capabilities | Applied to every workload |

## The OIDC ConfigMap

`<release>-wallet-accountant-oidc` holds `issuer`, `audience`, `web-client-id`, `api-clients-client-id` and the UI's `config.json`. Without the Zitadel setup the chart renders it from values. With it, the Job writes it after every install and upgrade, and restarts the UI when `config.json` changes. The chart creates it only once, as a `pre-install` hook with empty client IDs, so the UI can start (Helm waits for the UI to be ready before the post-install Job runs); Helm never updates a hook resource, so it never reverts the Job's client IDs, and leaves the ConfigMap behind on uninstall. Make targets read the client IDs from it:

```sh
kubectl -n wallet get configmap wa-wallet-accountant-oidc -o jsonpath='{.data.api-clients-client-id}'
```

## The Zitadel setup Job

It ports `deploy/zitadel/setup-project.sh` (`files/zitadel-setup.sh`) and runs after every install and upgrade. It is idempotent. In the organisation of the token's user it:

1. creates the project whose ID is `oidc.audience`, if missing;
2. creates or updates `api-clients`, a public native app (authorization code with PKCE, refresh tokens) for the Bruno collection;
3. creates or updates `web`, a public User Agent app (authorization code with PKCE, refresh tokens) whose redirect and post-logout URIs are `https://<route.hostname>[:<route.port>]<path>` plus the extra ones;
4. sets the instance's token lifetimes, 15-minute access tokens by default, when they differ;
5. writes the client IDs to the OIDC ConfigMap.

Its ServiceAccount may create that ConfigMap, patch it, and patch the UI Deployment, nothing else.

## Managed database

`database.mode: managed` creates a CloudNativePG `Cluster` with the database `wallet_accountant`, owned by `wa_owner` (`LOGIN CREATEROLE BYPASSRLS`, declared as a managed role). It generates the three passwords and URLs into `<release>-wallet-accountant-db-urls`. The first migration creates `wa_app` and `wa_event_store_reader`, and `migrate` sets their passwords.

These objects must exist before `migrate`, which is a pre-install hook, so they are pre-install hooks too. Helm therefore creates them on the first install only: it does not change them on upgrade and does not delete them on uninstall, so the data outlives the release. Change the cluster with `kubectl` after the first install, and delete the `Cluster` and the two Secrets by hand to remove the data. A reinstall over them keeps the passwords with Helm 4, which applies hooks server-side; Helm 3 refuses to create hook objects that already exist.

## Operator commands

`bootstrap` and `rebuild` (ADR-0017, ADR-0019) run as one-off pods from the chart's pod spec, with the values of the installed release. The pod must be called `wallet-accountant-<mode>`, because kubectl merges the rendered container into its own by name. `bootstrap` prints the Invitation code only to this terminal, and `--rm` deletes the pod and its log.

```sh
mode=bootstrap
kubectl -n wallet run "wallet-accountant-$mode" --rm -i --quiet --restart=Never \
  --image=ghcr.io/mumia/wallet-accountant \
  --overrides="$(helm template wa oci://ghcr.io/mumia/charts/wallet-accountant -n wallet \
    --values values.yaml --set operatorPod.mode=$mode \
    --show-only templates/operator-pod.yaml | grep -v -e '^---$' -e '^# Source:')"
```
