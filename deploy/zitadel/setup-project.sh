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
