#!/bin/sh
# Checks a repo's Renovate configuration (`make renovate-check`): runs
# renovate-config-validator on its repository config (renovate.json), then a lookup-only dry run
# (`--platform=local --dry-run=lookup`) over its working tree, prints every
# dependency Renovate found, and fails when an expected one is missing or a
# lookup failed, or when updates that must land in one PR would not:
#
# - the same dependency at the same new version in two files (an image in a
#   manifest and in images.lock, say) always shares a branch;
# - a `together:<depName>,<depName>,...` argument names dependencies that
#   share a branch when they have updates (majors in a branch of their own).
#
# Only updates found today are compared, so a group with nothing to update
# passes.
#
# Usage: check.sh <repo-dir> [<manager>:<depName> | together:<depName>,... ...]
#
# Needs docker and jq (Renovate runs in the renovate/renovate image, so no
# Node.js on the host). RENOVATE_IMAGE overrides the image. GITHUB_COM_TOKEN,
# when set, is passed on so GitHub lookups are not rate limited.
set -eu

repo=${1:?usage: check.sh <repo-dir> [<manager>:<depName> | together:<depName>,... ...]}
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

# Only the JSON log lines: Node prints some warnings as plain text (a
# gcr.io lookup probes for Google Cloud metadata and warns when it is absent).
deps=$(grep '^{' "$log" | jq -r 'select(.msg == "packageFiles with updates") | .config | to_entries[]
	| .key as $m | .value[] | .packageFile as $f | .deps[]
	| [$m, .depName, (.currentValue // "-"), $f, (.depType // "-"),
	   ((.updates // []) | map(.newValue // .newDigest) | join(",") | if . == "" then "-" else . end),
	   ((.skipReason // "") + ((.warnings // []) | map(.message) | join("; ")))]
	| @tsv')
if [ -z "$deps" ]; then
	echo "no dependencies found" >&2
	exit 1
fi
printf 'manager\tdepName\tcurrent\tfile\tdepType\tupdates\tproblem\n%s\n' "$deps" | column -t -s "$(printf '\t')"

# Every update and the branch (the PR) Renovate gives it.
updates=$(grep '^{' "$log" | jq -c 'select(.msg == "packageFiles with updates") | .config | to_entries[]
	| .key as $m | .value[] | .packageFile as $f | .deps[] | .depName as $d | (.updates // [])[]
	| {manager: $m, dep: $d, file: $f, type: .updateType, new: (.newValue // .newDigest), branch: .branchName}')
echo "== $repo: branches (one PR each)"
if [ -n "$updates" ]; then
	printf '%s\n' "$updates" | jq -rs 'sort_by(.branch, .dep, .file)[] | [.branch, .dep, .new, .file] | @tsv' |
		column -t -s "$(printf '\t')"
else
	echo "(no updates)"
fi

status=0
split=$(printf '%s\n' "$updates" | jq -rs 'map(select(. != null)) | group_by([.dep, .new])[]
	| select((map(.branch) | unique | length) > 1)
	| "\(.[0].dep) \(.[0].new): \(map("\(.file) -> \(.branch)") | join(", "))"')
if [ -n "$split" ]; then
	printf 'SPLIT: one dependency at one version goes to several PRs:\n%s\n' "$split" >&2
	status=1
fi
for want in "$@"; do
	manager=${want%%:*}
	dep=${want#*:}
	if [ "$manager" = together ]; then
		apart=$(printf '%s\n' "$updates" | jq -rs --arg deps "$dep" '($deps | split(",")) as $names
			| map(select(. != null and (.dep as $d | $names | index($d))))
			| group_by(.type == "major")[]
			| select((map(.branch) | unique | length) > 1)
			| map("\(.dep) \(.new) -> \(.branch)") | join(", ")')
		if [ -n "$apart" ]; then
			echo "SPLIT: $dep must share one PR, but: $apart" >&2
			status=1
		fi
		continue
	fi
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
