#!/bin/sh
# Fills the local cluster's registry from the env repo's images.lock, so the
# local environment starts, deploys and runs offline (`make local-prefetch`,
# ADR-0019):
#
# - each image under `images:` is copied to the k3d registry as
#   <registry>/<repository>:<tag> (ghcr.io/zitadel/zitadel:v4.19.2), where k3s's
#   mirrors look for it (k3d.yaml). An image index is copied unchanged, so its
#   digest is the one images.lock pins, with only LOCAL_PLATFORM's image under
#   it (the registry accepts that: clusters/local/registry-config.yml);
# - each chart under `charts:` is pushed as oci://<registry>/charts/<name>:<version>;
# - each image under `host-images:` is pulled into this Mac's Docker by digest
#   and tagged with its tag (k3d's own images, the Dockerfiles' base images);
# - Go modules are downloaded, so `make local-build` vendors them offline.
#
# Every upstream copy is checked against the digest images.lock pins. What is
# already there is skipped without touching the network, so this runs offline
# once everything is in (`make local-up` runs it first). When the registry is
# not running (no cluster), a temporary one on the same storage is started,
# and removed on the way out. Production-only entries (prod-charts:,
# prod-images:) are not mirrored.
#
# Needs docker (with buildx), curl, jq, go and HELM. Environment: ENV_REPO,
# LOCAL_REGISTRY, LOCAL_REGISTRY_PORT, LOCAL_REGISTRY_DIR, LOCAL_REGISTRY_CONFIG,
# HELM; LOCAL_PLATFORM defaults to the Docker server's (linux/arm64 on an
# Apple silicon Mac).
set -eu

: "${ENV_REPO:?}" "${LOCAL_REGISTRY:?}" "${LOCAL_REGISTRY_PORT:?}" "${LOCAL_REGISTRY_DIR:?}" "${LOCAL_REGISTRY_CONFIG:?}" "${HELM:?}"
: "${LOCAL_PLATFORM:=$(docker version --format '{{.Server.Os}}/{{.Server.Arch}}')}"
os=${LOCAL_PLATFORM%%/*}
arch=${LOCAL_PLATFORM#*/}
registry=localhost:$LOCAL_REGISTRY_PORT
accept="application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"
work=$(mktemp -d)
# The temporary registry, started when no cluster's registry is running. One
# an interrupted run left behind would hold the registry's port and pass for
# the cluster's registry: it is removed first (and by `make local-down`).
temporary=$LOCAL_REGISTRY-prefetch
started=
cleanup() {
	rm -rf "$work"
	[ -z "$started" ] || docker rm -f "$temporary" >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP
docker rm -f "$temporary" >/dev/null 2>&1 || true

list=$(go run ./deploy/local/imageslock list -lock "$ENV_REPO/images.lock")

# digest_of <file>: the sha256 digest of a manifest's bytes.
digest_of() { echo "sha256:$(shasum -a 256 "$1" | cut -d ' ' -f 1)"; }

# local_digest <repository> <tag-or-digest>: the digest the registry serves, or nothing.
local_digest() {
	curl -fsSI -H "Accept: $accept" "http://$registry/v2/$1/manifests/$2" 2>/dev/null |
		tr -d '\r' | awk 'tolower($1) == "docker-content-digest:" {print $2}'
}

# platform_child <manifest file>: the digest of LOCAL_PLATFORM's image in an index.
platform_child() {
	jq -r --arg os "$os" --arg arch "$arch" \
		'[.manifests[] | select(.platform.os == $os and .platform.architecture == $arch)][0].digest // empty' "$1"
}

host_image() { # host_image <name:tag@digest>
	tagged=${1%@*}
	digest=${1#*@}
	if docker image inspect "$tagged" --format '{{join .RepoDigests " "}}' 2>/dev/null | grep -q "@$digest"; then
		echo "ok:      $tagged (Docker)"
		return
	fi
	echo "pull:    $1 (Docker)"
	docker pull -q "$1" >/dev/null
	docker tag "$1" "$tagged"
}

mirror_image() { # mirror_image <registry/repository:tag@digest>
	tagged=${1%@*}
	digest=${1#*@}
	repo=${tagged%:*}
	tag=${tagged##*:}
	if [ "$(local_digest "$repo" "$tag")" = "$digest" ]; then
		curl -fsS -H "Accept: $accept" "http://$registry/v2/$repo/manifests/$digest" >"$work/local.json"
		child=$(jq -e '.manifests' "$work/local.json" >/dev/null 2>&1 && platform_child "$work/local.json" || echo "$digest")
		if [ -n "$child" ] && [ -n "$(local_digest "$repo" "$child")" ]; then
			echo "ok:      $tagged"
			return
		fi
	fi
	echo "mirror:  $1"
	docker buildx imagetools inspect --raw "$repo@$digest" >"$work/upstream.json"
	if [ "$(digest_of "$work/upstream.json")" != "$digest" ]; then
		echo "prefetch: $repo@$digest: the registry served a manifest with another digest" >&2
		exit 1
	fi
	if jq -e '.manifests' "$work/upstream.json" >/dev/null; then
		child=$(platform_child "$work/upstream.json")
		[ -n "$child" ] || { echo "prefetch: $tagged has no $LOCAL_PLATFORM image" >&2; exit 1; }
		docker buildx imagetools create --prefer-index=false -t "$registry/$repo:$tag" "$repo@$child"
		media=$(jq -r '.mediaType // "application/vnd.oci.image.index.v1+json"' "$work/upstream.json")
		curl -fsS -o /dev/null -X PUT -H "Content-Type: $media" --data-binary "@$work/upstream.json" \
			"http://$registry/v2/$repo/manifests/$tag"
	else
		docker buildx imagetools create --prefer-index=false -t "$registry/$repo:$tag" "$repo@$digest"
	fi
	[ "$(local_digest "$repo" "$tag")" = "$digest" ] || { echo "prefetch: $tagged: the mirror does not serve $digest" >&2; exit 1; }
}

mirror_chart() { # mirror_chart <name> <source> <version>
	if [ -n "$(local_digest "charts/$1" "$3")" ]; then
		echo "ok:      chart $1 $3"
		return
	fi
	echo "mirror:  chart $1 $3 from $2"
	rm -rf "$work/chart" && mkdir "$work/chart"
	case $2 in
	oci://*) "$HELM" pull "$2" --version "$3" --destination "$work/chart" ;;
	*) "$HELM" pull "$1" --repo "$2" --version "$3" --destination "$work/chart" ;;
	esac
	"$HELM" push "$work/chart"/*.tgz "oci://$registry/charts" --plain-http >/dev/null
}

# Docker's images first: the registry's own image is one of them.
echo "$list" | while read -r kind ref _; do
	case $kind in host-image) host_image "$ref" ;; esac
done

if ! curl -fsS -o /dev/null "http://$registry/v2/" 2>/dev/null; then
	registry_image=$(awk '$1 == "image:" && $2 ~ /^registry:/ {print $2}' "$ENV_REPO/clusters/local/k3d.yaml")
	mkdir -p "$LOCAL_REGISTRY_DIR"
	echo "prefetch: no registry on $registry; starting $temporary on $LOCAL_REGISTRY_DIR"
	started=1
	docker run -d --rm --name "$temporary" -p "$LOCAL_REGISTRY_PORT:5000" \
		-v "$LOCAL_REGISTRY_DIR:/var/lib/registry" -v "$LOCAL_REGISTRY_CONFIG:/etc/distribution/config.yml:ro" \
		"${registry_image:?no registry image in k3d.yaml}" >/dev/null
	i=0
	until curl -fsS -o /dev/null "http://$registry/v2/" 2>/dev/null; do
		i=$((i + 1))
		[ $i -lt 30 ] || { echo "prefetch: $temporary did not start" >&2; exit 1; }
		sleep 1
	done
fi

echo "$list" | while read -r kind a b c; do
	case $kind in
	image) mirror_image "$a" ;;
	chart) mirror_chart "$a" "$b" "$c" ;;
	esac
done

echo "go:      module cache (go mod download)"
go mod download
echo "prefetch: images.lock is in $registry and Docker ($LOCAL_PLATFORM)"
