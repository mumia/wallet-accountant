#!/bin/sh
# Checks the local cluster against what ADR-0019 and the env repo promise
# (`make local-check`). It reads a running cluster made by `make local-up` and
# `make local-deploy`. It changes little in it: short-lived probe pods, a
# Zitadel machine user `local-check` that signs in to the app, a bootstrap
# Invitation it redeems as that user (so the earlier code no longer works),
# and a rebuild of the folds. It also runs the app on this Mac (`make run`,
# debug mode) for a moment.
#
# Needs docker, k3d, kubectl, flux, curl, mkcert, jq, go and make. ENV_REPO is
# the env repo checkout (wallet-accountant-deploy), LOCAL_CLUSTER the k3d
# cluster name, LOCAL_REGISTRY the registry's container name,
# LOCAL_REGISTRY_PORT its port on the host and LOCAL_REGISTRY_DIR its storage
# on the host. LOCAL_BUILD is the file `make local-deploy` records its build
# in; LOCAL_DB_PORT and LOCAL_OTLP_PORT are the debug-mode ports.
set -eu

: "${ENV_REPO:?ENV_REPO must point at the wallet-accountant-deploy checkout}"
: "${LOCAL_CLUSTER:=wallet-accountant}"
: "${LOCAL_REGISTRY:=wa-registry}"
: "${LOCAL_REGISTRY_PORT:=5050}"
: "${LOCAL_REGISTRY_DIR:?LOCAL_REGISTRY_DIR must be the registry storage directory}"
: "${LOCAL_HTTP_PORT:=18080}"
: "${LOCAL_HTTPS_PORT:=18443}"
: "${LOCAL_BUILD:=bin/local-build.env}"
: "${LOCAL_DB_PORT:=5432}"
: "${LOCAL_OTLP_PORT:=4318}"
: "${MAKE:=make}"

CTX="k3d-$LOCAL_CLUSTER"
KUBECTL="kubectl --context $CTX"
CA="$(mkcert -CAROOT)/rootCA.pem"
failures=0

pass() { echo "ok:   $*"; }
fail() { echo "FAIL: $*" >&2; failures=$((failures + 1)); }
check() { # check <description> <command...>
	desc=$1; shift
	if "$@" >/dev/null 2>&1; then pass "$desc"; else fail "$desc"; fi
}

## The env repo

check "env repo has the spec's layout" sh -c '
	for d in infra/bootstrap infra/homelab platform apps/wallet-accountant clusters/local clusters/prod; do
		[ -d "$1/$d" ] || exit 1
	done
	[ -f "$1/images.lock" ] && [ -f "$1/README.md" ]' _ "$ENV_REPO"
check "platform/ names no app" sh -c '! grep -rniE "wallet|apps/" "$1/platform"' _ "$ENV_REPO"
check "apps/ reaches platform/ only through dependsOn" sh -c '! grep -rn "platform/" "$1/apps"' _ "$ENV_REPO"
check "every Secret under clusters/local is marked as throwaway" sh -c '
	found=0
	for f in $(grep -rlE "^kind: Secret$" "$1/clusters/local"); do
		found=1
		head -1 "$f" | grep -q "^# THROWAWAY LOCAL-ONLY SECRET" || exit 1
	done
	[ "$found" = 1 ]' _ "$ENV_REPO"

## k3d

check "cluster $LOCAL_CLUSTER has one server and no agents" sh -c '
	[ "$(docker ps -q --filter label=k3d.cluster=$1 --filter label=k3d.role=server | wc -l)" -eq 1 ] &&
	[ "$(docker ps -q --filter label=k3d.cluster=$1 --filter label=k3d.role=agent | wc -l)" -eq 0 ]' _ "$LOCAL_CLUSTER"
check "Traefik's 80 and 443 are published on host ports $LOCAL_HTTP_PORT and $LOCAL_HTTPS_PORT" sh -c '
	docker port "k3d-$1-serverlb" 80/tcp | grep -q ":$2$" &&
	docker port "k3d-$1-serverlb" 443/tcp | grep -q ":$3$"' _ "$LOCAL_CLUSTER" "$LOCAL_HTTP_PORT" "$LOCAL_HTTPS_PORT"
check "registry $LOCAL_REGISTRY stores its data in $LOCAL_REGISTRY_DIR" sh -c '
	docker inspect "$1" --format "{{range .Mounts}}{{.Source}}={{.Destination}} {{end}}" |
		grep -q "$2=/var/lib/registry"' _ "$LOCAL_REGISTRY" "$LOCAL_REGISTRY_DIR"

## Flux follows the pushed artifact

digest=$(curl -fsSI -H "Accept: application/vnd.oci.image.manifest.v1+json" \
	"http://localhost:$LOCAL_REGISTRY_PORT/v2/wallet-accountant-deploy/manifests/local" |
	tr -d '\r' | awk 'tolower($1) == "docker-content-digest:" {print $2}') || digest=
revision=$($KUBECTL -n flux-system get ocirepository wallet-accountant-deploy \
	-o jsonpath='{.status.artifact.revision}' 2>/dev/null) || revision=
if [ -n "$digest" ] && [ "$revision" = "local@$digest" ]; then
	pass "OCIRepository follows the pushed artifact ($digest)"
else
	fail "OCIRepository follows the pushed artifact (registry: '$digest', Flux: '$revision')"
fi
check "every Flux Kustomization is ready" sh -c '
	out=$(kubectl --context "$1" get kustomizations.kustomize.toolkit.fluxcd.io -A \
		-o jsonpath="{range .items[*]}{.status.conditions[?(@.type==\"Ready\")].status}{\"\\n\"}{end}") &&
	[ -n "$out" ] && ! echo "$out" | grep -qv True' _ "$CTX"
check "every HelmRelease is ready" sh -c '
	out=$(kubectl --context "$1" get helmreleases.helm.toolkit.fluxcd.io -A \
		-o jsonpath="{range .items[*]}{.status.conditions[?(@.type==\"Ready\")].status}{\"\\n\"}{end}") &&
	[ -n "$out" ] && ! echo "$out" | grep -qv True' _ "$CTX"

## Offline (images.lock, local-prefetch)

NODE="k3d-$LOCAL_CLUSTER-server-0"
LOCK="$ENV_REPO/images.lock"
check "images.lock covers everything clusters/local renders to (make images-lock-check)" \
	$MAKE -s images-lock-check
check "every image the node has pulled, and every k3d container's image, is in images.lock" sh -c '
	{
		docker exec "$1" crictl images -o json | jq -r ".images[].repoTags[]?"
		docker ps --filter "label=k3d.cluster=$2" --format "{{.Image}}"
		docker inspect "$3" --format "{{.Config.Image}}"
	} | go run ./deploy/local/imageslock covers -lock "$4"' _ "$NODE" "$LOCAL_CLUSTER" "$LOCAL_REGISTRY" "$LOCK"
check "every image and chart in images.lock is in $LOCAL_REGISTRY, at the pinned digest" sh -c '
	accept="application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"
	go run ./deploy/local/imageslock list -lock "$2" | while read -r kind a b c; do
		case $kind in
		image) ref=${a%@*}; repo=${ref%:*}; path="$repo/manifests/${ref##*:}"; want=${a#*@} ;;
		chart) path="charts/$a/manifests/$c"; want= ;;
		*) continue ;;
		esac
		got=$(curl -fsSI -H "Accept: $accept" "http://localhost:$1/v2/$path" | tr -d "\r" |
			awk "tolower(\$1) == \"docker-content-digest:\" {print \$2}")
		[ -n "$got" ] && { [ -z "$want" ] || [ "$got" = "$want" ]; } || { echo "missing: $a" >&2; exit 1; }
	done' _ "$LOCAL_REGISTRY_PORT" "$LOCK"
check "every Flux source fetches from $LOCAL_REGISTRY:5000" sh -c '
	urls=$(kubectl --context "$1" get helmrepositories.source.toolkit.fluxcd.io,ocirepositories.source.toolkit.fluxcd.io -A \
		-o jsonpath="{range .items[*]}{.spec.url}{\"\\n\"}{end}") &&
	[ -n "$urls" ] && ! echo "$urls" | grep -qv "^oci://$2:5000/"' _ "$CTX" "$LOCAL_REGISTRY"
# Fails closed: an image that was not prefetched cannot be pulled, even with
# the network up, from any registry (k3d.yaml: mirrors, no fallback). Each
# exists upstream, so a "not found" whose request reached wa-registry (its
# access log, `?ns=<registry>`) means k3s asked only the mirror.
for unmirrored in docker.io/library/hello-world:latest ghcr.io/fluxcd/flux-cli:v2.0.0 \
	quay.io/prometheus/busybox:latest registry.k8s.io/pause:3.10 gcr.io/distroless/base:latest; do
	since=$(date -u +%Y-%m-%dT%H:%M:%SZ)
	ns=${unmirrored%%/*}
	tag=${unmirrored##*:}
	if out=$(docker exec "$NODE" crictl pull "$unmirrored" 2>&1); then
		fail "k3s pulls only through $LOCAL_REGISTRY: $unmirrored was pulled from upstream"
		docker exec "$NODE" crictl rmi "$unmirrored" >/dev/null 2>&1 || true
	elif echo "$out" | grep -q "not found" &&
		docker logs --since "$since" "$LOCAL_REGISTRY" 2>&1 | grep -q "/manifests/$tag?ns=$ns "; then
		pass "k3s pulls only through $LOCAL_REGISTRY: $unmirrored is not found there, with no upstream try"
	else
		fail "k3s pulls only through $LOCAL_REGISTRY: $unmirrored ($(echo "$out" | tail -1 | head -c 300))"
	fi
done

## Certificates, Gateway, DNS

check "the mkcert CA issuer is ready and holds this Mac's mkcert CA" sh -c '
	[ "$(kubectl --context "$1" get clusterissuer mkcert-ca \
		-o jsonpath="{.status.conditions[?(@.type==\"Ready\")].status}")" = True ] &&
	kubectl --context "$1" -n cert-manager get secret mkcert-ca -o jsonpath="{.data.tls\.crt}" |
		base64 -d | cmp -s - "$2"' _ "$CTX" "$CA"
check "the Gateway has a programmed HTTPS listener for *.localhost" sh -c '
	[ "$(kubectl --context "$1" -n gateway get gateway platform \
		-o jsonpath="{.status.conditions[?(@.type==\"Programmed\")].status}")" = True ] &&
	kubectl --context "$1" -n gateway get gateway platform \
		-o jsonpath="{range .spec.listeners[*]}{.protocol} {.hostname}{\"\\n\"}{end}" | grep -qx "HTTPS \*.localhost"' _ "$CTX"
port_suffix=
[ "$LOCAL_HTTPS_PORT" = 443 ] || port_suffix=":$LOCAL_HTTPS_PORT"
headlamp="https://headlamp.localhost$port_suffix"
check "$headlamp answers with a certificate from the mkcert CA" \
	curl -fsS --cacert "$CA" -o /dev/null "$headlamp/"
check "Headlamp serves the Flux plugin" sh -c '
	curl -fsS --cacert "$1" "$2/plugins" | grep -q "plugins/flux"' _ "$CA" "$headlamp"

# Pods reach *.localhost through kube-system/localhost-ingress, a Service in
# front of Traefik that also answers on the host's port, so a URL with the
# browser's port (https://auth.localhost:18443) works from a pod too.
ingress_ip=$($KUBECTL -n kube-system get service localhost-ingress -o jsonpath='{.spec.clusterIP}' 2>/dev/null) || ingress_ip=
resolved=$($KUBECTL run "dns-check-$$" --rm -i --restart=Never --quiet \
	--image=rancher/mirrored-library-busybox:1.37.0 -- nslookup auth.localhost 2>/dev/null |
	awk '/^Name:/ {found=1} found && /^Address/ {print $NF}' | head -1) || resolved=
if [ -n "$ingress_ip" ] && [ "$resolved" = "$ingress_ip" ]; then
	pass "pods resolve auth.localhost to Traefik's localhost-ingress Service ($ingress_ip)"
else
	fail "pods resolve auth.localhost to Traefik's localhost-ingress Service (Service: '$ingress_ip', resolved: '$resolved')"
fi
check "localhost-ingress answers on 443 and on the host's port $LOCAL_HTTPS_PORT" sh -c '
	ports=" $(kubectl --context "$1" -n kube-system get service localhost-ingress -o jsonpath="{.spec.ports[*].port}") " &&
	case $ports in *" 443 "*) ;; *) exit 1 ;; esac &&
	case $ports in *" $2 "*) ;; *) exit 1 ;; esac' _ "$CTX" "$LOCAL_HTTPS_PORT"

## PostgreSQL (CloudNativePG, ADR-0023)

# cluster_value <name>: a value clusters/local substitutes into platform/.
cluster_value() {
	grep -E "^[[:space:]]+$1:" "$ENV_REPO/clusters/local/platform.yaml" | head -1 | awk '{print $2}' | tr -d '"'
}
for db in zitadel/zitadel-db:ZITADEL_DB_STORAGE databases/apps-db:APPS_DB_STORAGE; do
	ns_name=${db%%:*}
	size=$(cluster_value "${db#*:}")
	check "CNPG cluster $ns_name is healthy, with one instance and the cluster value's storage (${size:-unset})" sh -c '
		[ -n "$4" ] &&
		[ "$(kubectl --context "$1" -n "$2" get clusters.postgresql.cnpg.io "$3" \
			-o jsonpath="{.spec.instances}/{.status.readyInstances}/{.spec.storage.size}/{.status.phase}")" = "1/1/$4/Cluster in healthy state" ]' \
		_ "$CTX" "${ns_name%%/*}" "${ns_name#*/}" "$size"
done

# psql_apps_db <sql> [conninfo]: runs SQL in apps-db's primary, as postgres or as the URL's role.
psql_apps_db() {
	$KUBECTL -n databases exec apps-db-1 -c postgres -- psql "${2:-postgres}" -XtAc "$1"
}
check "tenant wallet-accountant: the Database wallet_accountant is applied, owned by wa_owner" sh -c '
	[ "$(kubectl --context "$1" -n databases get databases.postgresql.cnpg.io wallet-accountant \
		-o jsonpath="{.spec.name}/{.spec.owner}/{.status.applied}")" = "wallet_accountant/wa_owner/true" ]' _ "$CTX"
roles=$(psql_apps_db "select rolcanlogin, rolcreaterole, rolbypassrls from pg_roles where rolname = 'wa_owner'" 2>/dev/null) || roles=
if [ "$roles" = "t|t|t" ]; then
	pass "tenant wallet-accountant: wa_owner has LOGIN CREATEROLE BYPASSRLS (ADR-0014)"
else
	fail "tenant wallet-accountant: wa_owner has LOGIN CREATEROLE BYPASSRLS (ADR-0014) (got '$roles')"
fi
check "tenant wallet-accountant: the owner, app and reader URL Secrets are in the app's namespace" sh -c '
	for s in owner app reader; do
		kubectl --context "$1" -n wallet-accountant get secret "wallet-accountant-db-$s" -o jsonpath="{.data.url}" |
			base64 -d | grep -q "^postgres://.*@apps-db-rw\.databases\.svc\.cluster\.local:5432/wallet_accountant?" || exit 1
	done' _ "$CTX"
owner_url=$($KUBECTL -n wallet-accountant get secret wallet-accountant-db-owner -o jsonpath='{.data.url}' 2>/dev/null | base64 -d) || owner_url=
whoami=$(psql_apps_db "select current_user || '@' || current_database()" "$owner_url" 2>/dev/null) || whoami=
if [ -n "$owner_url" ] && [ "$whoami" = "wa_owner@wallet_accountant" ]; then
	pass "tenant wallet-accountant: the owner URL signs in as wa_owner to wallet_accountant"
else
	fail "tenant wallet-accountant: the owner URL signs in as wa_owner to wallet_accountant (got '$whoami')"
fi

## Zitadel (ADR-0022)

issuer="https://auth.localhost$port_suffix"
check "$issuer serves its discovery document, with that issuer, to the browser" sh -c '
	curl -fsS --cacert "$1" "$2/.well-known/openid-configuration" | grep -qF "\"issuer\":\"$2\""' _ "$CA" "$issuer"
# A Go client, as the app is (curl would resolve *.localhost to 127.0.0.1
# itself instead of asking CoreDNS): kubectl from the image Zitadel's chart
# uses, trusting only the mkcert CA.
from_pod=$($KUBECTL run "issuer-check-$$" --rm -i --restart=Never --quiet \
	--image=docker.io/alpine/k8s:1.35.5 --env="CA=$(cat "$CA")" --command -- \
	sh -c 'printf "%s\n" "$CA" >/tmp/ca.pem &&
		kubectl --server "$0" --certificate-authority=/tmp/ca.pem --token=none get --raw /.well-known/openid-configuration' \
	"$issuer" 2>/dev/null) || from_pod=
if echo "$from_pod" | grep -qF "\"issuer\":\"$issuer\""; then
	pass "$issuer serves the same issuer to a pod, with a certificate from the mkcert CA"
else
	fail "$issuer serves the same issuer to a pod, with a certificate from the mkcert CA (got '$(echo "$from_pod" | head -c 200)')"
fi
# Signing in to the console (Zitadel's own OIDC app) lands on the v1 login UI.
console_client=$(curl -fsS --cacert "$CA" "$issuer/ui/console/assets/environment.json" 2>/dev/null |
	grep -o '"clientid":"[^"]*"' | cut -d '"' -f 4) || console_client=
check "signing in goes to Zitadel's v1 login UI, and no login v2 is deployed" sh -c '
	jar=$(mktemp) && trap "rm -f $jar" EXIT &&
	login=$(curl -fsS -c "$jar" -b "$jar" --cacert "$1" -o /dev/null -w "%{redirect_url}" \
		"$2/oauth/v2/authorize?client_id=$3&redirect_uri=$2/ui/console/auth/callback&response_type=code&scope=openid&code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM&code_challenge_method=S256") &&
	case $login in "$2/ui/login/login?authRequestID="*) ;; *) exit 1 ;; esac &&
	curl -fsS -c "$jar" -b "$jar" --cacert "$1" "$login" | grep -q "name=\"loginName\"" &&
	! kubectl --context "$4" -n zitadel get deployments -o name | grep -q login' _ "$CA" "$issuer" "$console_client" "$CTX"
check "Zitadel's masterkey comes from the committed throwaway Secret zitadel/zitadel-masterkey" sh -c '
	[ "$(kubectl --context "$1" -n zitadel get deployment zitadel \
		-o jsonpath="{.spec.template.spec.containers[0].env[?(@.name==\"ZITADEL_MASTERKEY\")].valueFrom.secretKeyRef.name}")" = zitadel-masterkey ] &&
	grep -rqs "name: zitadel-masterkey" "$2/clusters/local/secrets"' _ "$CTX" "$ENV_REPO"

pat=$($KUBECTL -n zitadel get secret platform-setup-pat -o jsonpath='{.data.pat}' 2>/dev/null | base64 -d) || pat=
zitadel_api() { # zitadel_api <path> [json]: calls Zitadel's API with the machine user's token
	if [ $# -gt 1 ]; then
		curl -fsS --cacert "$CA" -H "Authorization: Bearer $pat" -H 'Content-Type: application/json' -X POST -d "$2" "$issuer$1"
	else
		curl -fsS --cacert "$CA" -H "Authorization: Bearer $pat" "$issuer$1"
	fi
}
if [ -n "$pat" ] && zitadel_api /admin/v1/policies/login >/dev/null 2>&1; then
	pass "the machine user's token (zitadel/platform-setup-pat, key pat) works on the admin API"
else
	fail "the machine user's token (zitadel/platform-setup-pat, key pat) works on the admin API"
fi
check "the machine user's token is copied to wallet-accountant/zitadel-setup-pat for the app's setup Job" sh -c '
	[ -n "$1" ] &&
	[ "$(kubectl --context "$2" -n wallet-accountant get secret zitadel-setup-pat -o jsonpath="{.data.pat}" | base64 -d)" = "$1" ]' _ "$pat" "$CTX"
policy=$(zitadel_api /admin/v1/policies/login 2>/dev/null) || policy=
if [ -n "$policy" ] && ! echo "$policy" | grep -q '"allowRegister":true'; then
	pass "self-registration is off"
else
	fail "self-registration is off (login policy: '$(echo "$policy" | head -c 200)')"
fi
# Zitadel adds the organisation's domain: admin@zitadel.auth.localhost.
admin_id=$(zitadel_api /v2/users '{"queries":[{"userNameQuery":{"userName":"admin@","method":"TEXT_QUERY_METHOD_STARTS_WITH"}}]}' 2>/dev/null |
	grep -o '"userId":"[^"]*"' | head -1 | cut -d '"' -f 4) || admin_id=
admin_password=$(grep -rh "^ *Password:" "$ENV_REPO/clusters/local/secrets/zitadel" 2>/dev/null | head -1 | awk '{print $2}' | tr -d "\"'") || admin_password=
session=$(printf '{"checks":{"user":{"userId":"%s"},"password":{"password":"%s"}}}' "$admin_id" "$admin_password" |
	curl -fsS --cacert "$CA" -H "Authorization: Bearer $pat" -H 'Content-Type: application/json' \
		-X POST -d @- "$issuer/v2/sessions" 2>/dev/null) || session=
if [ -n "$admin_id" ] && [ -n "$admin_password" ] && echo "$session" | grep -q sessionToken; then
	pass "the first-instance admin (admin@zitadel.auth.localhost) signs in with the password in clusters/local/secrets/zitadel"
else
	fail "the first-instance admin (admin@zitadel.auth.localhost) signs in with the password in clusters/local/secrets/zitadel (user '$admin_id')"
fi

## Wallet Accountant (ADR-0019, ADR-0020)

wallet="https://wallet.localhost$port_suffix"
NS=wallet-accountant
release=$($KUBECTL -n $NS get helmrelease wallet-accountant -o json 2>/dev/null) || release=
check "HelmRelease wallet-accountant is ready: external database on apps-db, Zitadel setup on, UI off, at wallet.localhost" sh -c '
	echo "$1" | jq -e "
		(.status.conditions[] | select(.type == \"Ready\") | .status == \"True\") and
		.spec.values.database.mode == \"external\" and
		.spec.values.database.external.ownerUrl.secretName == \"wallet-accountant-db-owner\" and
		.spec.values.oidc.zitadelSetup.enabled == true and
		(.spec.values.ui.enabled // false) == false and
		.spec.values.route.hostname == \"wallet.localhost\""' _ "$release"

# The build `make local-deploy` made: its image tag, set in the pushed
# artifact (flux-system/wallet-accountant-build), runs in the cluster.
built_tag=$(sed -n 's/^WALLET_ACCOUNTANT_IMAGE_TAG=//p' "$LOCAL_BUILD" 2>/dev/null) || built_tag=
artifact_tag=$($KUBECTL -n flux-system get configmap wallet-accountant-build \
	-o jsonpath='{.data.WALLET_ACCOUNTANT_IMAGE_TAG}' 2>/dev/null) || artifact_tag=
running=$($KUBECTL -n $NS get deployment wallet-accountant-all \
	-o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null) || running=
if [ -n "$built_tag" ] && [ "$artifact_tag" = "$built_tag" ] && [ "$running" = "$LOCAL_REGISTRY:5000/wallet-accountant:$built_tag" ] &&
	curl -fsS -o /dev/null -H "Accept: application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json" \
		"http://localhost:$LOCAL_REGISTRY_PORT/v2/wallet-accountant/manifests/$built_tag"; then
	pass "the app runs the image local-deploy built and pushed, whose tag is in the pushed artifact ($built_tag)"
else
	fail "the app runs the image local-deploy built and pushed, whose tag is in the pushed artifact (built '$built_tag', artifact '$artifact_tag', running '$running')"
fi
check "the app's pod is ready" sh -c '
	[ "$(kubectl --context "$1" -n wallet-accountant get deployment wallet-accountant-all -o jsonpath="{.status.readyReplicas}")" = 1 ]' _ "$CTX"
check "$wallet/api/me answers 401 without a token" sh -c '
	[ "$(curl -sS --cacert "$1" -o /dev/null -w "%{http_code}" "$2/api/me")" = 401 ]' _ "$CA" "$wallet"

# The setup Job's work in Zitadel.
web_app=$(zitadel_api /management/v1/projects/wallet-accountant/apps/_search '{}' 2>/dev/null |
	jq -c '.result[]? | select(.name == "web")') || web_app=
if echo "$web_app" | jq -e --arg u "$wallet/auth/callback" --arg l "$wallet/" \
	'(.oidcConfig.redirectUris | index($u)) and (.oidcConfig.postLogoutRedirectUris | index($l))' >/dev/null 2>&1; then
	pass "Zitadel's web app has the redirect URIs with the browser's port ($wallet/auth/callback)"
else
	fail "Zitadel's web app has the redirect URIs with the browser's port (got '$(echo "$web_app" | head -c 300)')"
fi
check "access tokens last 15 minutes" sh -c '
	curl -fsS --cacert "$1" -H "Authorization: Bearer $2" "$3/admin/v1/settings/oidc" |
		jq -e ".settings.accessTokenLifetime == \"900s\""' _ "$CA" "$pat" "$issuer"
api_clients_id=$($KUBECTL -n $NS get configmap wallet-accountant-oidc -o jsonpath='{.data.api-clients-client-id}' 2>/dev/null) || api_clients_id=
check "api/bruno/.env has the api-clients client ID from the setup Job's ConfigMap (${api_clients_id:-none})" sh -c '
	[ -n "$1" ] && grep -qx "WA_OAUTH_CLIENT_ID=$1" api/bruno/.env' _ "$api_clients_id"
check "the Bruno environment local points at $wallet/api and $issuer" sh -c '
	f=api/bruno/environments/local.yml &&
	grep -A1 "name: baseUrl" "$f" | grep -qF "value: $1/api" &&
	grep -A1 "name: authUrl" "$f" | grep -qF "value: $2"' _ "$wallet" "$issuer"

swagger="https://swagger.localhost$port_suffix"
check "Swagger UI at $swagger shows api/openapi.yaml" sh -c '
	curl -fsS --cacert "$1" "$2/" | grep -qi "swagger" &&
	curl -fsS --cacert "$1" "$2/openapi.yaml" | cmp -s - api/openapi.yaml' _ "$CA" "$swagger"

# Debug mode: apps-db and the collector on this Mac's localhost.
check "apps-db answers on localhost:$LOCAL_DB_PORT" nc -z localhost "$LOCAL_DB_PORT"
check "the collector takes OTLP over HTTP on localhost:$LOCAL_OTLP_PORT" sh -c '
	[ "$(curl -sS -o /dev/null -w "%{http_code}" -X POST -H "Content-Type: application/json" -d "{}" \
		"http://localhost:$1/v1/traces")" = 200 ]' _ "$LOCAL_OTLP_PORT"

# Operator commands: the code goes only to this terminal, and no pod is left.
bootstrap_out=$($MAKE -s local-bootstrap 2>/dev/null) || bootstrap_out=
# The code is the only line that is a lone base64url word: the app's JSON log
# lines can reach this output too, and are single words as well.
code=$(echo "$bootstrap_out" | awk 'NF == 1 && length($1) > 16 && $1 ~ /^[A-Za-z0-9_-]+$/ {print $1; exit}')
if [ -n "$code" ] && ! $KUBECTL -n $NS get pod wallet-accountant-bootstrap >/dev/null 2>&1; then
	pass "make local-bootstrap prints an Invitation code and leaves no pod"
else
	fail "make local-bootstrap prints an Invitation code and leaves no pod (printed '$(echo "$bootstrap_out" | head -c 200)')"
fi
check "the code is in no log of the app's namespace" sh -c '
	[ -n "$1" ] &&
	for p in $(kubectl --context "$2" -n wallet-accountant get pods -o name); do
		! kubectl --context "$2" -n wallet-accountant logs "$p" --all-containers 2>/dev/null | grep -qF "$1" || exit 1
	done' _ "$code" "$CTX"
check "make local-rebuild rebuilds the folds and leaves no pod" sh -c '
	$1 -s local-rebuild >/dev/null 2>&1 &&
	! kubectl --context "$2" -n wallet-accountant get pod wallet-accountant-rebuild >/dev/null 2>&1' _ "$MAKE" "$CTX"

# A real Zitadel identity: the machine user local-check, whose JWT access
# token carries the project's audience.
user_id=$(zitadel_api /management/v1/users/_search \
	'{"queries":[{"userNameQuery":{"userName":"local-check","method":"TEXT_QUERY_METHOD_EQUALS"}}]}' 2>/dev/null |
	jq -r '.result[0].id // empty') || user_id=
[ -n "$user_id" ] || user_id=$(zitadel_api /management/v1/users/machine \
	'{"userName":"local-check","name":"local-check","accessTokenType":"ACCESS_TOKEN_TYPE_JWT"}' 2>/dev/null |
	jq -r '.userId // empty') || user_id=
credentials=$(curl -fsS --cacert "$CA" -H "Authorization: Bearer $pat" -H 'Content-Type: application/json' \
	-X PUT -d '{}' "$issuer/management/v1/users/$user_id/secret" 2>/dev/null) || credentials=
token=$(curl -fsS --cacert "$CA" -u "$(echo "$credentials" | jq -r '.clientId'):$(echo "$credentials" | jq -r '.clientSecret')" \
	-d grant_type=client_credentials -d "scope=openid urn:zitadel:iam:org:project:id:wallet-accountant:aud" \
	"$issuer/oauth/v2/token" 2>/dev/null | jq -r '.access_token // empty') || token=
api_out=$(mktemp)
api() { # api <base> <method> <path> [json] -> the HTTP status; the body in $api_out
	curl -sS --cacert "$CA" -o "$api_out" -w '%{http_code}' -X "$2" -H "Authorization: Bearer $token" \
		-H 'Content-Type: application/json' -H "Idempotency-Key: $(uuidgen | tr 'A-Z' 'a-z')" ${4:+-d "$4"} "$1$3"
}
if [ -n "$token" ] && [ -n "$code" ]; then
	redeemed=$(api "$wallet" POST /api/registrations "{\"invitation_code\":\"$code\"}") || redeemed=
	me=$(api "$wallet" GET /api/me) || me=
	if { [ "$redeemed" = 201 ] || [ "$redeemed" = 409 ]; } && [ "$me" = 200 ] && jq -e '.is_instance_administrator == true' "$api_out" >/dev/null 2>&1; then
		pass "a Zitadel identity redeems the bootstrap code and is the Instance administrator on /api/me"
	else
		fail "a Zitadel identity redeems the bootstrap code and is the Instance administrator on /api/me (redeem $redeemed, me $me: $(head -c 200 "$api_out"))"
	fi
else
	fail "a Zitadel identity redeems the bootstrap code and is the Instance administrator on /api/me (no token or no code)"
fi

# Debug mode: `make run` on this Mac, with its defaults, against apps-db, the
# collector and Zitadel in the cluster. Go trusts the mkcert CA through the
# macOS keychain (no SSL_CERT_FILE), or verifying the token fails.
debug_addr=127.0.0.1:18089
debug_log=$(mktemp)
set -m
env -u SSL_CERT_FILE -u SSL_CERT_DIR WA_HTTP_ADDR=$debug_addr $MAKE -s run >"$debug_log" 2>&1 &
debug_pid=$!
set +m
i=0
while [ $i -lt 120 ] && kill -0 "$debug_pid" 2>/dev/null && ! nc -z 127.0.0.1 18089 2>/dev/null; do
	i=$((i + 1))
	sleep 1
done
debug_me=$(api "http://$debug_addr" GET /api/me 2>/dev/null) || debug_me=
kill -TERM -- "-$debug_pid" 2>/dev/null || true
wait "$debug_pid" 2>/dev/null || true
if [ -n "$token" ] && [ "$debug_me" = 200 ]; then
	pass "make run (debug mode) serves /api/me to the Zitadel identity, trusting the mkcert CA through the keychain"
else
	fail "make run (debug mode) serves /api/me to the Zitadel identity (got '$debug_me': $(head -c 200 "$api_out" 2>/dev/null); log: $(tail -5 "$debug_log"))"
fi
rm -f "$debug_log" "$api_out"

if [ "$failures" -ne 0 ]; then
	echo "local-check: $failures check(s) failed" >&2
	exit 1
fi
echo "local-check: all checks passed"
