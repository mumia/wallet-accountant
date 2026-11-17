#!/bin/sh
# Creates the Zitadel project Wallet Accountant's users sign in to, with the
# fixed ID PROJECT_ID, so the audience the app checks (WA_OIDC_AUDIENCE) is
# known before Zitadel starts. Run by the zitadel-setup service in compose.yaml
# and compose.dev.yaml after Zitadel is healthy; safe to run again.
#
# It signs in with the personal access token Zitadel writes to PAT_FILE when it
# creates its first instance (ZITADEL_FIRSTINSTANCE_PATPATH). An installation
# created before that token existed has none: create the project by hand and
# set the audience to its ID.
set -eu

: "${ZITADEL_URL:?}" "${PROJECT_ID:?}" "${PAT_FILE:?}"

if [ ! -s "$PAT_FILE" ]; then
	echo "zitadel-setup: no token at $PAT_FILE, so Zitadel was initialised without one." >&2
	echo "zitadel-setup: create the project in the console and set the audience to its ID." >&2
	exit 0
fi
pat=$(cat "$PAT_FILE")

# Inside Compose, Zitadel may be reached at another address than its external
# one; it routes requests by the Host header, so send the external host.
call() { # path json -> body in /tmp/out, prints the HTTP status
	curl -sS -o /tmp/out -w '%{http_code}' -X POST "$ZITADEL_URL$1" \
		${ZITADEL_HOST:+-H "Host: $ZITADEL_HOST"} \
		-H "Authorization: Bearer $pat" -H 'Content-Type: application/json' \
		-H 'Connect-Protocol-Version: 1' -d "$2"
}

status=$(curl -sS -o /tmp/out -w '%{http_code}' "$ZITADEL_URL/management/v1/orgs/me" \
	${ZITADEL_HOST:+-H "Host: $ZITADEL_HOST"} -H "Authorization: Bearer $pat")
[ "$status" = 200 ] || { echo "zitadel-setup: reading the organisation answered $status: $(cat /tmp/out)" >&2; exit 1; }
org=$(grep -o '"id":"[^"]*"' /tmp/out | head -n 1 | cut -d '"' -f 4)
[ -n "$org" ] || { echo "zitadel-setup: no organisation ID in $(cat /tmp/out)" >&2; exit 1; }

status=$(call /zitadel.project.v2.ProjectService/CreateProject \
	"{\"organizationId\":\"$org\",\"projectId\":\"$PROJECT_ID\",\"name\":\"Wallet Accountant\"}")
case $status in
200) echo "zitadel-setup: created project $PROJECT_ID" ;;
409) echo "zitadel-setup: project $PROJECT_ID exists" ;;
*) echo "zitadel-setup: creating project $PROJECT_ID answered $status: $(cat /tmp/out)" >&2; exit 1 ;;
esac


# With CLIENT_APP_NAME set, also makes sure the project has a public OIDC
# application of that name, for API clients that sign a person in with the
# authorization code flow and PKCE, such as the Bruno collection in api/bruno.
# Zitadel generates its client ID, so it is written to CLIENT_ID_FILE when set.
# The application is found by name rather than by a fixed ID, because Zitadel
# never gives a deleted application's ID out again.
[ -n "${CLIENT_APP_NAME:-}" ] || exit 0

status=$(call /zitadel.application.v2.ApplicationService/ListApplications "{\"filters\":[
	{\"projectIdFilter\":{\"projectId\":\"$PROJECT_ID\"}},
	{\"nameFilter\":{\"name\":\"$CLIENT_APP_NAME\",\"method\":\"TEXT_FILTER_METHOD_EQUALS\"}}
]}")
[ "$status" = 200 ] || { echo "zitadel-setup: listing applications answered $status: $(cat /tmp/out)" >&2; exit 1; }

if grep -q '"clientId"' /tmp/out; then
	echo "zitadel-setup: application $CLIENT_APP_NAME exists"
else
	status=$(call /zitadel.application.v2.ApplicationService/CreateApplication "{
		\"projectId\":\"$PROJECT_ID\",
		\"name\":\"$CLIENT_APP_NAME\",
		\"oidcConfiguration\":{
			\"redirectUris\":[\"https://oauth.usebruno.com/callback\",\"http://localhost/oauth/callback\"],
			\"responseTypes\":[\"OIDC_RESPONSE_TYPE_CODE\"],
			\"grantTypes\":[\"OIDC_GRANT_TYPE_AUTHORIZATION_CODE\",\"OIDC_GRANT_TYPE_REFRESH_TOKEN\"],
			\"applicationType\":\"OIDC_APP_TYPE_NATIVE\",
			\"authMethodType\":\"OIDC_AUTH_METHOD_TYPE_NONE\",
			\"accessTokenType\":\"OIDC_TOKEN_TYPE_JWT\",
			\"skipNativeAppSuccessPage\":true,
			\"version\":\"OIDC_VERSION_1_0\"
		}
	}")
	[ "$status" = 200 ] || { echo "zitadel-setup: creating application $CLIENT_APP_NAME answered $status: $(cat /tmp/out)" >&2; exit 1; }
	echo "zitadel-setup: created application $CLIENT_APP_NAME"
fi

client_id=$(grep -o '"clientId":"[^"]*"' /tmp/out | head -n 1 | cut -d '"' -f 4)
[ -n "$client_id" ] || { echo "zitadel-setup: no client ID in $(cat /tmp/out)" >&2; exit 1; }
echo "zitadel-setup: application $CLIENT_APP_NAME has the client ID $client_id"
if [ -n "${CLIENT_ID_FILE:-}" ]; then
	printf '%s\n' "$client_id" >"$CLIENT_ID_FILE"
fi
