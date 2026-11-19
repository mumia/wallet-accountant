#!/bin/sh
# Checks a repo's Renovate configuration (`make renovate-check`): runs
# renovate-config-validator on its repository config (renovate.json), then a lookup-only dry run
# (`--platform=local --dry-run=lookup`) over its working tree, prints every
# dependency Renovate found, and fails when an expected one is missing or a
# lookup failed.
#
# Usage: check.sh <repo-dir> [<manager>:<depName> ...]
#
# Needs docker and jq (Renovate runs in the renovate/renovate image, so no
# Node.js on the host). RENOVATE_IMAGE overrides the image. GITHUB_COM_TOKEN,
# when set, is passed on so GitHub lookups are not rate limited.
set -eu

repo=${1:?usage: check.sh <repo-dir> [<manager>:<depName> ...]}
shift
image=${RENOVATE_IMAGE:-renovate/renovate:44}
repo=$(cd "$repo" && pwd)

echo "== $repo: renovate-config-validator"
docker run --rm -v "$repo:/repo:ro" -w /repo "$image" \
	renovate-config-validator --strict

echo "== $repo: dependencies found (dry run, lookups only)"
log=$(mktemp)
trap 'rm -f "$log"' EXIT
# The repo is mounted read-only; git cannot read a worktree's .git file from
# inside the container, so Renovate lists files with a glob instead.
docker run --rm -v "$repo:/usr/src/app:ro" -w /usr/src/app \
	-e LOG_LEVEL=debug -e LOG_FORMAT=json -e GITHUB_COM_TOKEN \
	"$image" renovate --platform=local --dry-run=lookup >"$log" 2>&1 || {
	tail -n 20 "$log"
	echo "renovate failed" >&2
	exit 1
}

deps=$(jq -r 'select(.msg == "packageFiles with updates") | .config | to_entries[]
	| .key as $m | .value[] | .packageFile as $f | .deps[]
	| [$m, .depName, (.currentValue // "-"), $f, (.depType // "-"),
	   ((.updates // []) | map(.newValue // .newDigest) | join(",") | if . == "" then "-" else . end),
	   ((.skipReason // "") + ((.warnings // []) | map(.message) | join("; ")))]
	| @tsv' "$log")
if [ -z "$deps" ]; then
	echo "no dependencies found" >&2
	exit 1
fi
printf 'manager\tdepName\tcurrent\tfile\tdepType\tupdates\tproblem\n%s\n' "$deps" | column -t -s "$(printf '\t')"

status=0
for want in "$@"; do
	manager=${want%%:*}
	dep=${want#*:}
	if ! printf '%s\n' "$deps" | awk -F '\t' -v m="$manager" -v d="$dep" '$1 == m && $2 == d { found = 1 } END { exit !found }'; then
		echo "MISSING: $manager found no $dep" >&2
		status=1
	fi
done
# A lookup that failed (a warning) means a wrong datasource or name. A skip
# reason is only listed: some are expected (an unversioned image, say).
if printf '%s\n' "$deps" | awk -F '\t' '$7 != "" && $7 !~ /^[a-z-]+$/ { found = 1 } END { exit !found }'; then
	echo "LOOKUP FAILED: see the problem column above" >&2
	status=1
fi
exit $status
