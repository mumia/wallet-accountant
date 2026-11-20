# Zitadel helpers for the local cluster's checks (check.sh, smoke.sh,
# browser-test.sh); source it. They call Zitadel at ZITADEL_URL (the issuer,
# https://auth.localhost:18443) with the platform's machine user token
# ZITADEL_PAT, trusting the mkcert CA in CA. zitadel_env sets all three.
#
# Needs curl and jq.

# zitadel_env <kube context> <https port>: sets CA, ZITADEL_URL and ZITADEL_PAT
# (zitadel/platform-setup-pat, empty when it cannot be read).
zitadel_env() {
	CA="$(mkcert -CAROOT)/rootCA.pem"
	if [ "$2" = 443 ]; then ZITADEL_URL=https://auth.localhost; else ZITADEL_URL="https://auth.localhost:$2"; fi
	ZITADEL_PAT=$(kubectl --context "$1" -n zitadel get secret platform-setup-pat -o jsonpath='{.data.pat}' 2>/dev/null | base64 -d) || ZITADEL_PAT=
}

# zitadel_api <path> [json [method]]: calls Zitadel's API as the machine user;
# with a body, the method defaults to POST. Fails on an HTTP error.
zitadel_api() {
	if [ $# -gt 1 ]; then
		curl -fsS --cacert "$CA" -H "Authorization: Bearer $ZITADEL_PAT" -H 'Content-Type: application/json' \
			-X "${3:-POST}" -d "$2" "$ZITADEL_URL$1"
	else
		curl -fsS --cacert "$CA" -H "Authorization: Bearer $ZITADEL_PAT" "$ZITADEL_URL$1"
	fi
}

# zitadel_machine_token <user name> [audience]: prints a JWT access token for
# the machine user of that name (created on first use) through the client
# credentials grant, with the project's audience (default wallet-accountant):
# a real Zitadel identity the app accepts.
zitadel_machine_token() {
	_id=$(zitadel_api /management/v1/users/_search \
		"{\"queries\":[{\"userNameQuery\":{\"userName\":\"$1\",\"method\":\"TEXT_QUERY_METHOD_EQUALS\"}}]}" |
		jq -r '.result[0].id // empty') || return 1
	[ -n "$_id" ] || _id=$(zitadel_api /management/v1/users/machine \
		"{\"userName\":\"$1\",\"name\":\"$1\",\"accessTokenType\":\"ACCESS_TOKEN_TYPE_JWT\"}" | jq -r '.userId // empty') || return 1
	_secret=$(zitadel_api "/management/v1/users/$_id/secret" '{}' PUT) || return 1
	curl -fsS --cacert "$CA" \
		-u "$(echo "$_secret" | jq -r .clientId):$(echo "$_secret" | jq -r .clientSecret)" \
		-d grant_type=client_credentials \
		-d "scope=openid urn:zitadel:iam:org:project:id:${2:-wallet-accountant}:aud" \
		"$ZITADEL_URL/oauth/v2/token" | jq -er .access_token
}

# zitadel_human_user <user name> <password>: makes sure a human user of that
# name exists, with a verified email and that password (no change required),
# and prints its login name, as Zitadel's login page takes it.
zitadel_human_user() {
	_id=$(zitadel_api /management/v1/users/_search \
		"{\"queries\":[{\"userNameQuery\":{\"userName\":\"$1\",\"method\":\"TEXT_QUERY_METHOD_EQUALS\"}}]}" |
		jq -r '.result[0].id // empty') || return 1
	if [ -z "$_id" ]; then
		_id=$(zitadel_api /v2/users/human "{
			\"username\":\"$1\",
			\"profile\":{\"givenName\":\"$1\",\"familyName\":\"Test\"},
			\"email\":{\"email\":\"$1@example.com\",\"isVerified\":true},
			\"password\":{\"password\":\"$2\",\"changeRequired\":false}
		}" | jq -r '.userId // empty') || return 1
	else
		zitadel_api "/v2/users/$_id/password" \
			"{\"newPassword\":{\"password\":\"$2\",\"changeRequired\":false}}" >/dev/null || return 1
	fi
	zitadel_api "/v2/users/$_id" | jq -er .user.preferredLoginName
}

# bootstrap_code: reads `make local-bootstrap`'s output and prints the code.
# It is the only line that is a lone base64url word longer than 16
# characters: the app's JSON log lines can reach this output too, and are
# single words as well.
bootstrap_code() {
	awk 'NF == 1 && length($1) > 16 && $1 ~ /^[A-Za-z0-9_-]+$/ {print $1; exit}'
}
