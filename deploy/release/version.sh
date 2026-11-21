#!/bin/sh
# Usage: version.sh <ref-type> <ref-name> <commit>
#
# Turns the ref the release workflow runs on into what it publishes, as
# key=value lines for $GITHUB_OUTPUT (docs/architecture/conventions.md,
# "Releases"):
#   image-tag      the image's tag
#   chart          true when the chart is published too
#   chart-version  the chart's version (semver, no "v")
#   app-version    the chart's appVersion: the image tag, which the chart
#                  uses when image.tag is empty
set -eu

ref_type=$1
ref_name=$2
commit=$3

fail() {
	echo "version.sh: $*" >&2
	exit 1
}

case $ref_type in
branch)
	[ "$ref_name" = main ] || fail "branch $ref_name publishes nothing; only main does"
	# A test image of main. Production follows tags only, so no chart.
	echo "image-tag=main-$(printf '%s' "$commit" | cut -c1-7)"
	echo "chart=false"
	exit 0
	;;
tag) ;;
*) fail "unknown ref type $ref_type" ;;
esac

# vMAJOR.MINOR.PATCH with an optional pre-release (semver 2.0.0). Build
# metadata (+...) is refused: "+" is not allowed in an OCI tag. A numeric
# pre-release identifier has no leading zero, or Helm refuses the chart.
number='(0|[1-9][0-9]*)'
identifier='(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)'
semver="v$number\.$number\.$number(-$identifier(\.$identifier)*)?"
tag=$ref_name
if [ "$(printf '%s' "$tag" | wc -l)" -ne 0 ] || ! printf '%s\n' "$tag" | grep -Eqx "$semver"; then
	fail "tag $tag is not a release tag (vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-PRERELEASE)"
fi
echo "image-tag=$tag"
echo "chart=true"
echo "chart-version=${tag#v}"
echo "app-version=$tag"
