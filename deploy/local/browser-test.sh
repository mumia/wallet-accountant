#!/bin/sh
# The stand-in UI's browser test (`make local-browser-test`), against a local
# cluster deployed with the stand-in UI (`make local-deploy LOCAL_STAND_IN_UI=1`).
# It makes sure the Zitadel human user `stand-in-ui` exists with a fresh
# random password, then runs the Playwright test
# (deploy/helm/wallet-accountant/test/stand-in-ui/e2e) in the official
# Playwright image, on the k3d cluster's Docker network, so no Node.js is
# needed on the host. The browser trusts the mkcert CA's certificates only
# (e2e/tls-pin.js). On failure, traces and screenshots are in RESULTS_DIR.
#
# Needs docker, kubectl, curl, jq, mkcert. LOCAL_CLUSTER, LOCAL_HTTPS_PORT,
# PLAYWRIGHT_IMAGE, STAND_IN_UI (the fixture's directory) and RESULTS_DIR
# come from the Makefile.
set -eu

: "${LOCAL_CLUSTER:?}" "${LOCAL_HTTPS_PORT:?}" "${PLAYWRIGHT_IMAGE:?}" "${STAND_IN_UI:?}" "${RESULTS_DIR:?}"
. "$(dirname "$0")/zitadel.sh"
zitadel_env "k3d-$LOCAL_CLUSTER" "$LOCAL_HTTPS_PORT"
[ -n "$ZITADEL_PAT" ] || { echo "browser-test: no Zitadel machine user token; is the cluster up?" >&2; exit 1; }

origin=https://wallet.localhost
[ "$LOCAL_HTTPS_PORT" = 443 ] || origin="$origin:$LOCAL_HTTPS_PORT"
password="Wa-$(openssl rand -hex 12)-1!"
login=$(zitadel_human_user stand-in-ui "$password") || { echo "browser-test: could not set up the Zitadel user stand-in-ui" >&2; exit 1; }

rm -rf "$RESULTS_DIR"
mkdir -p "$RESULTS_DIR"
stand_in_ui=$(cd "$STAND_IN_UI" && pwd)
results=$(cd "$RESULTS_DIR" && pwd)
# The fixture is copied out of the read-only mount so npm ci never writes
# into the working tree; the image's own browsers are used.
exec docker run --rm --init --ipc=host --user "$(id -u):$(id -g)" \
	--network "k3d-$LOCAL_CLUSTER" --add-host wallet.localhost:127.0.0.1 --add-host auth.localhost:127.0.0.1 \
	-v "$stand_in_ui:/src:ro" -v "$CA:/ca.pem:ro" -v "$results:/results" \
	-e HOME=/tmp -e npm_config_update_notifier=false \
	-e WA_UI_BASE_URL="$origin" -e WA_UI_USER="$login" -e WA_UI_PASSWORD="$password" \
	-e WA_INGRESS="k3d-$LOCAL_CLUSTER-serverlb:443" -e WA_CA_FILE=/ca.pem -e NODE_EXTRA_CA_CERTS=/ca.pem \
	-e WA_TLS_NAMES="wallet.localhost auth.localhost" -e WA_RESULTS_DIR=/results/test-results \
	"$PLAYWRIGHT_IMAGE" sh -c '
		cp -R /src /tmp/stand-in-ui && cd /tmp/stand-in-ui/e2e &&
		npm ci --no-audit --no-fund --loglevel=error &&
		npx --no-install playwright test'
