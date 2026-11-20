#!/bin/sh
# Runs an operator run mode, bootstrap or rebuild (ADR-0017, ADR-0019), as a
# one-off pod in a cluster: the chart's pod spec (operator-pod.yaml) with the
# installed release's chart version and values, as the chart's README
# ("Operator commands") describes. The pod's output goes only to this
# terminal, and `--rm` deletes the pod and its log.
#
# Usage: operator-pod.sh bootstrap|rebuild. Needs HELM, and either
# - LOCAL_CLUSTER and LOCAL_REGISTRY_PORT: the local cluster, whose chart is
#   in its own registry (make local-bootstrap, make local-rebuild); or
# - KUBE_CONTEXT and CHART_REPO: another cluster, such as production, and
#   the OCI repository its chart was released to (make prod-bootstrap, with
#   KUBECONFIG set to its kubeconfig).
set -eu

mode=${1:?usage: operator-pod.sh bootstrap|rebuild}
: "${HELM:?}"
if [ -n "${KUBE_CONTEXT:-}" ]; then
	: "${CHART_REPO:?KUBE_CONTEXT needs CHART_REPO}"
	ctx=$KUBE_CONTEXT
	chart_repo=$CHART_REPO
	plain_http=
else
	: "${LOCAL_CLUSTER:?}" "${LOCAL_REGISTRY_PORT:?}"
	ctx=k3d-$LOCAL_CLUSTER
	chart_repo=oci://localhost:$LOCAL_REGISTRY_PORT/charts/wallet-accountant
	plain_http=--plain-http
fi
ns=wallet-accountant
release=wallet-accountant

values=$(mktemp)
errors=$(mktemp)
trap 'rm -f "$values" "$errors"' EXIT
"$HELM" --kube-context "$ctx" -n $ns get values $release -o yaml >"$values"
chart=$("$HELM" --kube-context "$ctx" -n $ns list --filter "^$release\$" -o json | jq -r '.[0].chart')
# Flux appends the OCI digest to the version it installs (<version>+<digest>).
version=${chart#wallet-accountant-}
version=${version%%+*}
[ -n "$version" ] && [ "$version" != null ] || { echo "no release $release in namespace $ns of $ctx (locally: run 'make local-deploy')" >&2; exit 1; }

# helm reports the chart it pulled on stderr; show it only when it fails.
# shellcheck disable=SC2086 # $plain_http is one flag or nothing
overrides=$("$HELM" template $release "$chart_repo" \
	--version "$version" $plain_http -n $ns --values "$values" \
	--set operatorPod.mode="$mode" --show-only templates/operator-pod.yaml 2>"$errors") ||
	{ cat "$errors" >&2; exit 1; }
overrides=$(echo "$overrides" | grep -v -e '^---$' -e '^# Source:')
image=$(echo "$overrides" | jq -r '.spec.containers[0].image')

exec kubectl --context "$ctx" -n $ns run "wallet-accountant-$mode" --rm -i --quiet --restart=Never \
	--image="$image" --overrides="$overrides"
