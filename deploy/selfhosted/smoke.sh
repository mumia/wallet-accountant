#!/bin/sh
# Smoke check of the self-hosted Compose stack (`make smoke`): from nothing,
# `make up`, then `make bootstrap`, then redeem the printed code as a real
# Zitadel identity, check the code reached no log, and `make compose-rebuild`.
#
# It runs as a separate Compose project on its own ports, with fresh secrets,
# and removes it (volumes included) when it ends, so it never touches the
# development stack or a real installation. Needs docker, curl, jq, openssl.
# SMOKE_UP=up-observability runs it with the observability profile, and then
# also checks that the collector exported every signal without errors.
set -eu

cd "$(dirname "$0")/../.."

export COMPOSE_PROJECT_NAME=${SMOKE_PROJECT:-wallet-accountant-smoke}
UP=${SMOKE_UP:-up}
APP_PORT=${SMOKE_APP_PORT:-18080}
AUTH_PORT=${SMOKE_AUTH_PORT:-18081}
AUTH_DOMAIN=auth.localhost
ZITADEL="http://$AUTH_DOMAIN:$AUTH_PORT"
APP="http://localhost:$APP_PORT"

SMOKE_DIR=$(mktemp -d "${TMPDIR:-/tmp}/wa-smoke.XXXXXX")
chmod 777 "$SMOKE_DIR"
export SMOKE_DIR
export COMPOSE_FILE="compose.yaml:deploy/selfhosted/compose.smoke.yaml"
ENV_FILE="$SMOKE_DIR/env"
MAKE="make --no-print-directory ENV_FILE=$ENV_FILE"

cleanup() {
	status=$?
	if [ "$status" -ne 0 ]; then
		echo "smoke: FAILED; last app and zitadel logs:" >&2
		docker compose --env-file "$ENV_FILE" logs --tail 40 app migrate zitadel >&2 || true
	fi
	docker compose --env-file "$ENV_FILE" --profile observability down -v --remove-orphans >/dev/null 2>&1 || true
	rm -rf "$SMOKE_DIR"
	exit "$status"
}
trap cleanup EXIT INT TERM

step() { printf '\nsmoke: %s\n' "$*"; }
fail() { echo "smoke: $*" >&2; exit 1; }

step "make $UP (project $COMPOSE_PROJECT_NAME)"
deploy/selfhosted/init-env.sh "$ENV_FILE"
cat >>"$ENV_FILE" <<ENV
APP_PORT=$APP_PORT
AUTH_PORT=$AUTH_PORT
AUTH_DOMAIN=$AUTH_DOMAIN
GRAFANA_PORT=${SMOKE_GRAFANA_PORT:-13000}
ENV
$MAKE "$UP"

step "make bootstrap"
out=$($MAKE bootstrap)
printf '%s\n' "$out"
code=$(printf '%s\n' "$out" | sed -n 's/^    \([A-Za-z0-9_-]\{43\}\)$/\1/p')
[ -n "$code" ] || fail "make bootstrap printed no Invitation code"

step "the code reached no log"
if docker compose --env-file "$ENV_FILE" logs --no-color 2>&1 | grep -qF "$code"; then
	fail "the Invitation code appears in the container logs"
fi

step "set up a Zitadel project and a signed-in identity"
pat=$(cat "$SMOKE_DIR/pat.txt")
zitadel() { # method path [json]
	body=${3:-'{}'}
	curl -fsS -X "$1" "$ZITADEL$2" -H "Authorization: Bearer $pat" \
		-H 'Content-Type: application/json' -d "$body"
}
project=$(zitadel POST /management/v1/projects '{"name":"wallet-accountant"}' | jq -er .id)
user=$(zitadel POST /management/v1/users/machine \
	'{"userName":"smoke-user","name":"Smoke User","accessTokenType":"ACCESS_TOKEN_TYPE_JWT"}' | jq -er .userId)
secret=$(zitadel PUT "/management/v1/users/$user/secret")
client_id=$(printf '%s' "$secret" | jq -er .clientId)
client_secret=$(printf '%s' "$secret" | jq -er .clientSecret)
token=$(curl -fsS "$ZITADEL/oauth/v2/token" -u "$client_id:$client_secret" \
	-d grant_type=client_credentials \
	--data-urlencode "scope=openid urn:zitadel:iam:org:project:id:$project:aud" | jq -er .access_token)

step "point the app at the project's audience"
echo "OIDC_AUDIENCE=$project" >>"$ENV_FILE"
docker compose --env-file "$ENV_FILE" up -d --no-deps --wait app

status() { # method path [json] -> HTTP status
	curl -sS -o "$SMOKE_DIR/body" -w '%{http_code}' -X "$1" "$APP$2" \
		-H "Authorization: Bearer $token" -H 'Content-Type: application/json' ${3:+-d "$3"}
}
i=0
until [ "$(status GET /me 2>/dev/null || true)" = 403 ]; do
	i=$((i + 1))
	[ "$i" -lt 60 ] || fail "GET /me never answered 403 registration-required: $(cat "$SMOKE_DIR/body" 2>/dev/null)"
	sleep 1
done

step "redeem the bootstrap code"
got=$(status POST /registrations "{\"invitation_code\":\"$code\"}")
[ "$got" = 201 ] || fail "POST /registrations answered $got: $(cat "$SMOKE_DIR/body")"
got=$(status GET /me)
[ "$got" = 200 ] || fail "GET /me answered $got after registering: $(cat "$SMOKE_DIR/body")"
cat "$SMOKE_DIR/body"; echo

step "bootstrap is refused once redeemed"
if $MAKE bootstrap >/dev/null 2>&1; then
	fail "make bootstrap issued another code after the first was redeemed"
fi

step "make compose-rebuild keeps the registration"
$MAKE compose-rebuild
got=$(status GET /me)
[ "$got" = 200 ] || fail "GET /me answered $got after rebuild: $(cat "$SMOKE_DIR/body")"

if [ "$UP" = up-observability ]; then
	step "Grafana shows the app's traces, metrics and logs"
	. "$ENV_FILE"
	grafana() { # datasource-uid path jq-filter
		curl -fsS -u "admin:$GRAFANA_ADMIN_PASSWORD" \
			"http://localhost:$GRAFANA_PORT/api/datasources/proxy/uid/$1$2" 2>/dev/null | jq -e "$3" >/dev/null 2>&1
	}
	for check in \
		'tempo|/api/search?tags=service.name%3Dwallet-accountant|.traces | length > 0' \
		'prometheus|/api/v1/query?query=count(%7Bjob%3D~%22.*wallet-accountant.*%22%7D)|.data.result | length > 0' \
		'loki|/loki/api/v1/label/service_name/values|.data | index("wallet-accountant") != null'; do
		uid=${check%%|*}; rest=${check#*|}; path=${rest%%|*}; filter=${rest#*|}
		i=0
		until grafana "$uid" "$path" "$filter"; do
			i=$((i + 1))
			[ "$i" -lt 60 ] || fail "no wallet-accountant data in $uid"
			sleep 1
		done
		echo "  $uid: ok"
	done

	step "the code reached no log in Loki"
	found=$(curl -fsS -G -u "admin:$GRAFANA_ADMIN_PASSWORD" \
		"http://localhost:$GRAFANA_PORT/api/datasources/proxy/uid/loki/loki/api/v1/query_range" \
		--data-urlencode "query={service_name=~\".+\"} |= \"$code\"" --data-urlencode since=1h |
		jq -er '.data.result | length')
	[ "$found" = 0 ] || fail "the Invitation code appears in Loki"
fi

step "OK"
