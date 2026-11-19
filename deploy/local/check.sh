#!/bin/sh
# Checks the local cluster against what ADR-0019 and the env repo promise
# (`make local-check`). It reads a running cluster made by `make local-up`, and
# changes nothing in it apart from one short-lived DNS probe pod.
#
# Needs docker, k3d, kubectl, flux, curl, mkcert. ENV_REPO is the env repo
# checkout (wallet-accountant-deploy), LOCAL_CLUSTER the k3d cluster name,
# LOCAL_REGISTRY the registry's container name, LOCAL_REGISTRY_PORT its port on
# the host and LOCAL_REGISTRY_DIR its storage on the host.
set -eu

: "${ENV_REPO:?ENV_REPO must point at the wallet-accountant-deploy checkout}"
: "${LOCAL_CLUSTER:=wallet-accountant}"
: "${LOCAL_REGISTRY:=wa-registry}"
: "${LOCAL_REGISTRY_PORT:=5050}"
: "${LOCAL_REGISTRY_DIR:?LOCAL_REGISTRY_DIR must be the registry storage directory}"
: "${LOCAL_HTTP_PORT:=18080}"
: "${LOCAL_HTTPS_PORT:=18443}"

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

if [ "$failures" -ne 0 ]; then
	echo "local-check: $failures check(s) failed" >&2
	exit 1
fi
echo "local-check: all checks passed"
