#!/bin/sh
# Stages what `make local-push` pushes to the local cluster's registry as the
# env repo's OCI artifact (ADR-0019): ENV_REPO's working tree, committed or
# not, plus what only this checkout and this Mac have, written into the copy
# and never into ENV_REPO:
#
# - this Mac's mkcert CA certificate, which the app trusts
#   (clusters/local/apps/wallet-accountant/ca.crt);
# - api/openapi.yaml, which Swagger UI shows (clusters/local/swagger);
# - the build `make local-deploy` recorded in LOCAL_BUILD (image tag and chart
#   version), as clusters/local/build, which adds the app and the ConfigMap
#   flux-system/wallet-accountant-build with those versions. Only when the
#   image is still in the registry; without a build, no app is deployed.
# - with the app, a UI in the chart's UI slot: the newer of two builds, each
#   a WALLET_ACCOUNTANT_UI_IMAGE (the repository in the registry) and a
#   WALLET_ACCOUNTANT_UI_TAG (local-<UTC timestamp>), and each only while its
#   image is still in the registry. One is the stand-in UI, in LOCAL_BUILD
#   (`make local-deploy LOCAL_STAND_IN_UI=1`); the other is the real UI, in
#   LOCAL_UI_BUILD, which wallet-accountant-ui's `make local-deploy` writes.
#   Without either, the slot stays off.
#
# Usage: stage.sh <directory>. Needs ENV_REPO, LOCAL_BUILD, LOCAL_REGISTRY,
# LOCAL_REGISTRY_PORT; LOCAL_UI_BUILD is optional.
set -eu

stage=${1:?usage: stage.sh <directory>}
: "${ENV_REPO:?}" "${LOCAL_BUILD:?}" "${LOCAL_REGISTRY:?}" "${LOCAL_REGISTRY_PORT:?}"

rm -rf "$stage"
mkdir -p "$stage"
rsync -a --exclude=.git --exclude=/infra "$ENV_REPO/" "$stage/"

cp "$(mkcert -CAROOT)/rootCA.pem" "$stage/clusters/local/apps/wallet-accountant/ca.crt"
cp api/openapi.yaml "$stage/clusters/local/swagger/openapi.yaml"

[ -f "$LOCAL_BUILD" ] || exit 0
WALLET_ACCOUNTANT_VERSION=$(sed -n 's/^WALLET_ACCOUNTANT_VERSION=//p' "$LOCAL_BUILD")
WALLET_ACCOUNTANT_IMAGE_TAG=$(sed -n 's/^WALLET_ACCOUNTANT_IMAGE_TAG=//p' "$LOCAL_BUILD")
# in_registry <repository> <tag>: whether the k3d registry has that image.
in_registry() {
	curl -fsS -o /dev/null \
		-H "Accept: application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json" \
		"http://localhost:$LOCAL_REGISTRY_PORT/v2/$1/manifests/$2"
}
if ! in_registry wallet-accountant "$WALLET_ACCOUNTANT_IMAGE_TAG"; then
	echo "stage: image wallet-accountant:$WALLET_ACCOUNTANT_IMAGE_TAG is not in the registry; no app until 'make local-deploy'" >&2
	exit 0
fi

cat >"$stage/clusters/local/build/kustomization.yaml" <<EOF
# Written by wallet-accountant's deploy/local/stage.sh for this push.
apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
resources:
  - ../apps
configMapGenerator:
  - name: wallet-accountant-build
    namespace: flux-system
    literals:
      - WALLET_ACCOUNTANT_VERSION=$WALLET_ACCOUNTANT_VERSION
      - WALLET_ACCOUNTANT_IMAGE_TAG=$WALLET_ACCOUNTANT_IMAGE_TAG
    options:
      disableNameSuffixHash: true
EOF
echo "stage: Wallet Accountant $WALLET_ACCOUNTANT_IMAGE_TAG (chart $WALLET_ACCOUNTANT_VERSION)"

# The UI: the newer build in the registry, of LOCAL_BUILD's (the stand-in)
# and LOCAL_UI_BUILD's (the real UI). Tags are local-<UTC timestamp>, so the
# later one sorts last.
ui_image=
ui_tag=
for file in "$LOCAL_BUILD" "${LOCAL_UI_BUILD:-}"; do
	[ -n "$file" ] && [ -f "$file" ] || continue
	image=$(sed -n 's/^WALLET_ACCOUNTANT_UI_IMAGE=//p' "$file")
	tag=$(sed -n 's/^WALLET_ACCOUNTANT_UI_TAG=//p' "$file")
	[ -n "$image" ] && [ -n "$tag" ] || continue
	if ! in_registry "$image" "$tag"; then
		echo "stage: UI image $image:$tag ($file) is not in the registry; skipped" >&2
		continue
	fi
	if [ -z "$ui_tag" ] || [ "$(printf '%s\n%s\n' "$ui_tag" "$tag" | sort | tail -n 1)" = "$tag" ]; then
		ui_image=$image
		ui_tag=$tag
	fi
done
[ -n "$ui_tag" ] || exit 0
# One more patch on the wallet-accountant Flux Kustomization, which turns the
# chart's UI slot on with that image.
cat >>"$stage/clusters/local/build/kustomization.yaml" <<EOF
patches:
  - target:
      group: kustomize.toolkit.fluxcd.io
      kind: Kustomization
      name: wallet-accountant
    patch: |
      - op: add
        path: /spec/patches/-
        value:
          target:
            kind: HelmRelease
            name: wallet-accountant
          patch: |
            - op: add
              path: /spec/values/ui
              value:
                enabled: true
                image:
                  repository: $LOCAL_REGISTRY:5000/$ui_image
                  tag: $ui_tag
EOF
echo "stage: with the UI $ui_image:$ui_tag"
