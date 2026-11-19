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
: "${LOCAL_HTTP_PORT:=80}"
: "${LOCAL_HTTPS_PORT:=443}"

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
check "local secrets are marked as throwaway" sh -c '
	for f in "$1"/clusters/local/secrets/*.yaml; do
		head -3 "$f" | grep -q "THROWAWAY" || exit 1
	done' _ "$ENV_REPO"

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
headlamp=https://headlamp.localhost
[ "$LOCAL_HTTPS_PORT" = 443 ] || headlamp="$headlamp:$LOCAL_HTTPS_PORT"
check "$headlamp answers with a certificate from the mkcert CA" \
	curl -fsS --cacert "$CA" -o /dev/null "$headlamp/"
check "Headlamp serves the Flux plugin" sh -c '
	curl -fsS --cacert "$1" "$2/plugins" | grep -q "plugins/flux"' _ "$CA" "$headlamp"

traefik_ip=$($KUBECTL -n kube-system get service traefik -o jsonpath='{.spec.clusterIP}' 2>/dev/null) || traefik_ip=
resolved=$($KUBECTL run "dns-check-$$" --rm -i --restart=Never --quiet \
	--image=rancher/mirrored-library-busybox:1.37.0 -- nslookup auth.localhost 2>/dev/null |
	awk '/^Name:/ {found=1} found && /^Address/ {print $NF}' | head -1) || resolved=
if [ -n "$traefik_ip" ] && [ "$resolved" = "$traefik_ip" ]; then
	pass "pods resolve auth.localhost to Traefik ($traefik_ip)"
else
	fail "pods resolve auth.localhost to Traefik (Traefik: '$traefik_ip', resolved: '$resolved')"
fi

if [ "$failures" -ne 0 ]; then
	echo "local-check: $failures check(s) failed" >&2
	exit 1
fi
echo "local-check: all checks passed"
