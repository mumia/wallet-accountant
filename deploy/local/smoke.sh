#!/bin/sh
# The end-to-end smoke test (`make local-smoke`, and CI: .github/workflows/smoke.yaml):
# the same path a developer runs, from nothing.
#
# 1. `make local-up`: a new k3d cluster with the env repo's clusters/local;
# 2. `make local-deploy LOCAL_STAND_IN_UI=1`: this working tree's image and
#    chart, with the stand-in UI in the chart's UI slot;
# 3. `make local-bootstrap`, redeem the code as a real Zitadel identity (a
#    machine user with the project's audience), and `/api/me` says it is the
#    Instance administrator;
# 4. the code reached no log, in any pod of the cluster (ADR-0017);
#    then bootstrap is refused, now that the code is redeemed;
# 5. `make local-browser-test`: sign-in, audience and sign-out in a browser;
# 6. `make local-down`, whatever happened (SMOKE_KEEP=1 keeps the cluster).
#
# It refuses to start when the cluster already exists, rather than delete it.
# The build is recorded in its own file (SMOKE_BUILD), so a developer's
# bin/local-build.env is left alone. Needs what local-up, local-deploy and
# the browser test need: docker, k3d, kubectl, flux, mkcert, curl, jq.
set -eu

: "${LOCAL_CLUSTER:?}" "${LOCAL_HTTPS_PORT:?}"
: "${MAKE:=make}"
SMOKE_BUILD=${SMOKE_BUILD:-bin/local-smoke-build.env}
CTX="k3d-$LOCAL_CLUSTER"
KUBECTL="kubectl --context $CTX"
NS=wallet-accountant
# LOCAL_UI_BUILD is empty, so the stand-in UI is deployed even when a newer
# real UI build is in the registry.
make_local() { $MAKE --no-print-directory LOCAL_BUILD="$SMOKE_BUILD" LOCAL_UI_BUILD= "$@"; }

started=$(date +%s)
step() { printf '\nsmoke [%ss]: %s\n' "$(($(date +%s) - started))" "$*"; }
fail() { echo "smoke: FAIL: $*" >&2; exit 1; }

if k3d cluster get "$LOCAL_CLUSTER" >/dev/null 2>&1; then
	echo "smoke: the cluster $LOCAL_CLUSTER exists; the smoke test starts from nothing. Run 'make local-down' first." >&2
	exit 1
fi

cleanup() {
	status=$?
	if [ "$status" -ne 0 ]; then
		echo "smoke: FAILED; the cluster's state:" >&2
		flux --context "$CTX" get all -A >&2 2>&1 || true
		$KUBECTL get pods -A >&2 2>&1 || true
		$KUBECTL -n $NS logs deployment/wallet-accountant-all --tail=40 >&2 2>&1 || true
		$KUBECTL -n $NS logs job/wallet-accountant-zitadel-setup --tail=40 >&2 2>&1 || true
	fi
	if [ -n "${SMOKE_KEEP:-}" ]; then
		echo "smoke: SMOKE_KEEP is set; the cluster is kept" >&2
	else
		step "make local-down"
		$MAKE --no-print-directory local-down >/dev/null 2>&1 || true
	fi
	rm -f "$SMOKE_BUILD"
	printf 'smoke: %s in %ss\n' "$([ "$status" -eq 0 ] && echo passed || echo failed)" "$(($(date +%s) - started))"
	exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

rm -f "$SMOKE_BUILD"
step "make local-up"
make_local local-up

step "make local-deploy LOCAL_STAND_IN_UI=1"
make_local local-deploy LOCAL_STAND_IN_UI=1
# The setup Job restarts the UI once config.json holds the web app's client ID.
$KUBECTL -n $NS rollout status deployment/wallet-accountant-ui --timeout=5m

step "make local-bootstrap"
. "$(dirname "$0")/zitadel.sh"
# The whole output first: a pipe that closes early would kill kubectl run
# before it deletes the pod.
out=$(make_local -s local-bootstrap) || fail "make local-bootstrap failed"
code=$(echo "$out" | bootstrap_code)
[ -n "$code" ] || fail "make local-bootstrap printed no Invitation code"
! $KUBECTL -n $NS get pod wallet-accountant-bootstrap >/dev/null 2>&1 || fail "the bootstrap pod was left behind"

step "redeem the code as a Zitadel identity, then /api/me"
zitadel_env "$CTX" "$LOCAL_HTTPS_PORT"
token=$(zitadel_machine_token smoke) || fail "no access token for the Zitadel machine user smoke"
wallet=https://wallet.localhost
[ "$LOCAL_HTTPS_PORT" = 443 ] || wallet="$wallet:$LOCAL_HTTPS_PORT"
body=$(mktemp)
api() { # api <method> <path> [json] -> the HTTP status; the body in $body
	curl -sS --cacert "$CA" -o "$body" -w '%{http_code}' -X "$1" -H "Authorization: Bearer $token" \
		-H 'Content-Type: application/json' -H "Idempotency-Key: $(uuidgen | tr 'A-Z' 'a-z')" ${3:+-d "$3"} "$wallet$2"
}
status=$(api POST /api/registrations "{\"invitation_code\":\"$code\"}")
[ "$status" = 201 ] || fail "redeeming the code answered $status: $(cat "$body")"
status=$(api GET /api/me)
[ "$status" = 200 ] && jq -e '.is_instance_administrator == true' "$body" >/dev/null ||
	fail "/api/me answered $status: $(cat "$body")"
rm -f "$body"

step "the code reached no log"
# Every container log in the cluster, those of finished and restarted
# containers included, as the node keeps them (/var/log/pods): one grep on
# the node instead of one API call per pod.
node="k3d-$LOCAL_CLUSTER-server-0"
docker exec "$node" sh -c 'ls /var/log/pods | grep -q "^wallet-accountant_wallet-accountant-all-"' ||
	fail "no log of the app's pod under /var/log/pods on $node"
# The deleted bootstrap pod's own log goes when the kubelet cleans the pod up,
# which can take a while after it is deleted: wait for that one (up to 2 minutes);
# the code in any other log fails at once.
for i in $(seq 24); do
	status=0
	found=$(docker exec "$node" grep -rlF -e "$code" /var/log/pods) || status=$?
	[ "$status" = 0 ] && [ "$i" -lt 24 ] || break
	echo "$found" | grep -qv "^/var/log/pods/${NS}_wallet-accountant-bootstrap_" && break
	sleep 5
done
[ "$status" = 1 ] || fail "the Invitation code is in these logs, or grep failed ($status): $found"

step "make local-bootstrap again: refused once redeemed"
# ADR-0017; it also shows a failed run's exit code reaches make. The output
# is not printed: were it to succeed, it would hold a code.
if make_local -s local-bootstrap >/dev/null 2>&1; then
	fail "make local-bootstrap succeeded after the code was redeemed"
fi
! $KUBECTL -n $NS get pod wallet-accountant-bootstrap >/dev/null 2>&1 || fail "the refused bootstrap's pod was left behind"

step "make local-browser-test"
make_local local-browser-test
