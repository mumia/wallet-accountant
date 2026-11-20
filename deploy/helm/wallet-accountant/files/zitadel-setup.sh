#!/bin/sh
# The chart's Zitadel setup Job (ADR-0022), run after every install and
# upgrade; safe to run again. It makes sure that, in the organisation of the
# token's user:
#
# - the project with the ID PROJECT_ID exists: its ID is the audience the app
#   checks (WA_OIDC_AUDIENCE);
# - `api-clients`, the public native app the Bruno collection signs in with,
#   and `web`, the browser UI's public User Agent app (PKCE, refresh tokens),
#   exist with the redirect URIs given here;
# - access tokens last ACCESS_TOKEN_LIFETIME (15 minutes). Token lifetimes
#   are instance settings, so the token must be an IAM owner's.
#
# Zitadel generates the client IDs. They are written, with the issuer, the
# audience and the UI's config.json, into the data of the ConfigMap
# OIDC_CONFIGMAP, which the chart owns, the UI mounts and Make targets read.
# When config.json changes, the UI Deployment UI_DEPLOYMENT (if set) is
# restarted to load it.
#
# Applications are found by name rather than by a fixed ID, because Zitadel
# never gives a deleted application's ID out again.
set -eu

: "${ZITADEL_URL:?}" "${ZITADEL_PAT:?}" "${PROJECT_ID:?}" "${OIDC_CONFIGMAP:?}" "${POD_NAMESPACE:?}"
: "${ISSUER:?}" "${WEB_REDIRECT_URIS:?}" "${WEB_POST_LOGOUT_REDIRECT_URIS:?}" "${API_CLIENTS_REDIRECT_URIS:?}"
: "${ACCESS_TOKEN_LIFETIME:?}" "${ID_TOKEN_LIFETIME:?}" "${REFRESH_TOKEN_IDLE_EXPIRATION:?}" "${REFRESH_TOKEN_EXPIRATION:?}"
: "${UI_REDIRECT_PATH:?}" "${UI_POST_LOGOUT_REDIRECT_PATH:?}"

out=${TMPDIR:-/tmp}/zitadel-setup.out
log() { echo "zitadel-setup: $*"; }
fail() { echo "zitadel-setup: $*" >&2; exit 1; }

# zitadel METHOD PATH [JSON] -> body in $out, prints the HTTP status.
# Trusts CURL_CA_BUNDLE when the chart sets it (trust.caConfigMap), and
# connects to CONNECT_TO (host:port) instead of the URL's host when it is set.
zitadel() {
	curl -sS -o "$out" -w '%{http_code}' -X "$1" "$ZITADEL_URL$2" \
		${CONNECT_TO:+--connect-to "::$CONNECT_TO"} \
		-H "Authorization: Bearer $ZITADEL_PAT" -H 'Content-Type: application/json' \
		-H 'Connect-Protocol-Version: 1' ${3:+-d "$3"}
}

# field NAME -> the first string value of the JSON field NAME in $out.
field() { grep -o "\"$1\":\"[^\"]*\"" "$out" | head -n 1 | cut -d '"' -f 4; }

# json_list "a b" -> ["a","b"]
json_list() {
	printf '['
	sep=''
	for item in $1; do
		printf '%s"%s"' "$sep" "$item"
		sep=','
	done
	printf ']'
}

# An update that changes nothing is refused by some Zitadel versions; that
# is the state we want.
unchanged() { grep -qi 'not changed\|notchanged\|no changes' "$out"; }

status=$(zitadel GET /management/v1/orgs/me)
[ "$status" = 200 ] || fail "reading the organisation answered $status: $(cat "$out")"
org=$(field id)
[ -n "$org" ] || fail "no organisation ID in $(cat "$out")"

status=$(zitadel POST /zitadel.project.v2.ProjectService/CreateProject \
	"{\"organizationId\":\"$org\",\"projectId\":\"$PROJECT_ID\",\"name\":\"Wallet Accountant\"}")
case $status in
200) log "created project $PROJECT_ID" ;;
409) log "project $PROJECT_ID exists" ;;
*) fail "creating project $PROJECT_ID answered $status: $(cat "$out")" ;;
esac

# ensure_app NAME OIDC_CONFIGURATION_JSON -> creates the app, or updates its
# OIDC configuration, and sets client_id.
ensure_app() {
	status=$(zitadel POST /zitadel.application.v2.ApplicationService/ListApplications "{\"filters\":[
		{\"projectIdFilter\":{\"projectId\":\"$PROJECT_ID\"}},
		{\"nameFilter\":{\"name\":\"$1\",\"method\":\"TEXT_FILTER_METHOD_EQUALS\"}}
	]}")
	[ "$status" = 200 ] || fail "listing applications answered $status: $(cat "$out")"
	client_id=$(field clientId)
	if [ -n "$client_id" ]; then
		app_id=$(field applicationId)
		[ -n "$app_id" ] || app_id=$(field id)
		[ -n "$app_id" ] || fail "no application ID for $1 in $(cat "$out")"
		status=$(zitadel POST /zitadel.application.v2.ApplicationService/UpdateApplication \
			"{\"projectId\":\"$PROJECT_ID\",\"applicationId\":\"$app_id\",\"oidcConfiguration\":$2}")
		if [ "$status" = 200 ]; then
			log "updated application $1"
		elif unchanged; then
			log "application $1 is up to date"
		else
			fail "updating application $1 answered $status: $(cat "$out")"
		fi
	else
		status=$(zitadel POST /zitadel.application.v2.ApplicationService/CreateApplication \
			"{\"projectId\":\"$PROJECT_ID\",\"name\":\"$1\",\"oidcConfiguration\":$2}")
		[ "$status" = 200 ] || fail "creating application $1 answered $status: $(cat "$out")"
		client_id=$(field clientId)
		log "created application $1"
	fi
	[ -n "$client_id" ] || fail "no client ID for $1 in $(cat "$out")"
	log "application $1 has the client ID $client_id"
}

ensure_app api-clients "{
	\"redirectUris\":$(json_list "$API_CLIENTS_REDIRECT_URIS"),
	\"responseTypes\":[\"OIDC_RESPONSE_TYPE_CODE\"],
	\"grantTypes\":[\"OIDC_GRANT_TYPE_AUTHORIZATION_CODE\",\"OIDC_GRANT_TYPE_REFRESH_TOKEN\"],
	\"applicationType\":\"OIDC_APP_TYPE_NATIVE\",
	\"authMethodType\":\"OIDC_AUTH_METHOD_TYPE_NONE\",
	\"accessTokenType\":\"OIDC_TOKEN_TYPE_JWT\",
	\"skipNativeAppSuccessPage\":true,
	\"version\":\"OIDC_VERSION_1_0\"
}"
api_clients_id=$client_id

ensure_app web "{
	\"redirectUris\":$(json_list "$WEB_REDIRECT_URIS"),
	\"postLogoutRedirectUris\":$(json_list "$WEB_POST_LOGOUT_REDIRECT_URIS"),
	\"responseTypes\":[\"OIDC_RESPONSE_TYPE_CODE\"],
	\"grantTypes\":[\"OIDC_GRANT_TYPE_AUTHORIZATION_CODE\",\"OIDC_GRANT_TYPE_REFRESH_TOKEN\"],
	\"applicationType\":\"OIDC_APP_TYPE_USER_AGENT\",
	\"authMethodType\":\"OIDC_AUTH_METHOD_TYPE_NONE\",
	\"accessTokenType\":\"OIDC_TOKEN_TYPE_JWT\",
	\"developmentMode\":${WEB_DEV_MODE:-false},
	\"version\":\"OIDC_VERSION_1_0\"
}"
web_id=$client_id

# Token lifetimes (instance settings), written only when they differ.
status=$(zitadel GET /admin/v1/settings/oidc)
[ "$status" = 200 ] || fail "reading the OIDC settings answered $status: $(cat "$out")"
if [ "$(field accessTokenLifetime)" = "$ACCESS_TOKEN_LIFETIME" ] &&
	[ "$(field idTokenLifetime)" = "$ID_TOKEN_LIFETIME" ] &&
	[ "$(field refreshTokenIdleExpiration)" = "$REFRESH_TOKEN_IDLE_EXPIRATION" ] &&
	[ "$(field refreshTokenExpiration)" = "$REFRESH_TOKEN_EXPIRATION" ]; then
	log "token lifetimes are up to date"
else
	status=$(zitadel PUT /admin/v1/settings/oidc "{
		\"accessTokenLifetime\":\"$ACCESS_TOKEN_LIFETIME\",
		\"idTokenLifetime\":\"$ID_TOKEN_LIFETIME\",
		\"refreshTokenIdleExpiration\":\"$REFRESH_TOKEN_IDLE_EXPIRATION\",
		\"refreshTokenExpiration\":\"$REFRESH_TOKEN_EXPIRATION\"
	}")
	[ "$status" = 200 ] || unchanged || fail "setting the token lifetimes answered $status: $(cat "$out")"
	log "access tokens last $ACCESS_TOKEN_LIFETIME"
fi

# The ConfigMap's data, patched with the in-cluster API and this pod's
# ServiceAccount. The chart owns the ConfigMap itself.
sa=/var/run/secrets/kubernetes.io/serviceaccount
kube() { # METHOD PATH CONTENT_TYPE JSON -> prints the HTTP status
	curl -sS -o "$out" -w '%{http_code}' -X "$1" \
		"https://kubernetes.default.svc$2" --cacert "$sa/ca.crt" \
		-H "Authorization: Bearer $(cat "$sa/token")" -H "Content-Type: $3" -d "$4"
}

config_json="{\"apiBaseUrl\":\"/api\",\"oidc\":{\"audience\":\"$PROJECT_ID\",\"clientId\":\"$web_id\",\"issuer\":\"$ISSUER\",\"postLogoutRedirectPath\":\"$UI_POST_LOGOUT_REDIRECT_PATH\",\"redirectPath\":\"$UI_REDIRECT_PATH\"}}"
escaped=$(printf '%s' "$config_json" | sed 's/"/\\"/g')
status=$(kube PATCH "/api/v1/namespaces/$POD_NAMESPACE/configmaps/$OIDC_CONFIGMAP?fieldManager=zitadel-setup" \
	application/merge-patch+json "{
	\"data\":{
		\"issuer\":\"$ISSUER\",
		\"audience\":\"$PROJECT_ID\",
		\"web-client-id\":\"$web_id\",
		\"api-clients-client-id\":\"$api_clients_id\",
		\"config.json\":\"$escaped\"
	}
}")
[ "$status" = 200 ] || fail "writing ConfigMap $OIDC_CONFIGMAP answered $status: $(cat "$out")"
# Then give up the fields that write took: the chart owns the ConfigMap, and
# Helm's server-side apply would otherwise refuse to change them (when the
# setup is turned off, say) for conflicting with this manager. Its entry in
# managedFields is removed, found by testing each index (a failed test is
# 422); with no entry (the write changed nothing) there is nothing to remove.
i=0
while [ $i -lt 20 ]; do
	status=$(kube PATCH "/api/v1/namespaces/$POD_NAMESPACE/configmaps/$OIDC_CONFIGMAP" \
		application/json-patch+json "[{\"op\":\"test\",\"path\":\"/metadata/managedFields/$i/manager\",\"value\":\"zitadel-setup\"},{\"op\":\"remove\",\"path\":\"/metadata/managedFields/$i\"}]")
	[ "$status" = 422 ] || break
	i=$((i + 1))
done
[ "$status" = 200 ] || [ "$status" = 422 ] || fail "releasing ConfigMap $OIDC_CONFIGMAP's fields answered $status: $(cat "$out")"
log "wrote the client IDs to ConfigMap $OIDC_CONFIGMAP"

# Restart the UI when its config.json changed; the same annotation is a no-op.
if [ -n "${UI_DEPLOYMENT:-}" ]; then
	sum=$(printf '%s' "$config_json" | sha256sum | cut -d ' ' -f 1)
	status=$(kube PATCH "/apis/apps/v1/namespaces/$POD_NAMESPACE/deployments/$UI_DEPLOYMENT" \
		application/strategic-merge-patch+json \
		"{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"checksum/config\":\"$sum\"}}}}}")
	[ "$status" = 200 ] || fail "restarting Deployment $UI_DEPLOYMENT answered $status: $(cat "$out")"
	log "the UI Deployment $UI_DEPLOYMENT runs config.json $sum"
fi
