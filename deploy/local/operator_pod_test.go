// Package localtest checks deploy/local/operator-pod.sh, which runs the
// operator run modes (bootstrap, rebuild) as one-off pods: in the local
// cluster (make local-bootstrap) and in production (make prod-bootstrap).
// helm and kubectl are stand-ins that record their arguments, so no cluster
// or registry is contacted.
package localtest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The stand-ins: helm answers as an installed release would, and both log
// one line of arguments per call.
const helmStub = `#!/bin/sh
echo "helm $*" >>"$CALLS"
case "$*" in
*" get values "*) echo "route:"; echo "  hostname: wallet.example.test" ;;
*" list "*) echo '[{"name":"wallet-accountant","chart":"wallet-accountant-1.2.3+sha256.abc"}]' ;;
"template "*) echo '---'; echo '# Source: wallet-accountant/templates/operator-pod.yaml'
	echo '{"spec":{"containers":[{"name":"wallet-accountant-bootstrap","image":"ghcr.io/mumia/wallet-accountant:v1.2.3"}]}}' ;;
esac
`

const kubectlStub = `#!/bin/sh
echo "kubectl $*" >>"$CALLS"
`

// operatorPod runs the script with the stand-ins and the given environment,
// and returns the calls it made.
func operatorPod(t *testing.T, mode string, env ...string) []string {
	t.Helper()
	bin := t.TempDir()
	for name, body := range map[string]string{"helm": helmStub, "kubectl": kubectlStub} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil { //nolint:gosec // G306: an executable stand-in in the test's own directory
			t.Fatal(err)
		}
	}
	calls := filepath.Join(t.TempDir(), "calls")
	var errOut bytes.Buffer
	cmd := exec.CommandContext(t.Context(), "sh", "operator-pod.sh", mode) //nolint:gosec // G204: the mode is the test's own
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CALLS="+calls,
		"HELM="+filepath.Join(bin, "helm"),
	)
	cmd.Env = append(cmd.Env, env...)
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("operator-pod.sh %s failed: %v: %s", mode, err, errOut.String())
	}
	out, err := os.ReadFile(calls) //nolint:gosec // G304: the test's own file
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n")
}

func call(t *testing.T, calls []string, prefix string) string {
	t.Helper()
	for _, c := range calls {
		if strings.HasPrefix(c, prefix) {
			return c
		}
	}
	t.Fatalf("no call starting with %q in:\n%s", prefix, strings.Join(calls, "\n"))
	return ""
}

func TestLocalClusterUsesItsContextAndRegistry(t *testing.T) {
	calls := operatorPod(t, "bootstrap", "LOCAL_CLUSTER=wallet-accountant", "LOCAL_REGISTRY_PORT=5050")

	call(t, calls, "helm --kube-context k3d-wallet-accountant -n wallet-accountant get values wallet-accountant")
	template := call(t, calls, "helm template wallet-accountant oci://localhost:5050/charts/wallet-accountant --version 1.2.3 --plain-http ")
	if !strings.Contains(template, "--set operatorPod.mode=bootstrap") {
		t.Errorf("template call does not render the bootstrap pod: %s", template)
	}
	run := call(t, calls, "kubectl --context k3d-wallet-accountant -n wallet-accountant run wallet-accountant-bootstrap --rm -i --quiet --restart=Never")
	if !strings.Contains(run, "--image=ghcr.io/mumia/wallet-accountant:v1.2.3") {
		t.Errorf("the pod does not run the chart's image: %s", run)
	}
}

func TestProductionUsesItsContextAndTheReleasedChart(t *testing.T) {
	calls := operatorPod(t, "bootstrap",
		"LOCAL_CLUSTER=wallet-accountant", "LOCAL_REGISTRY_PORT=5050",
		"KUBE_CONTEXT=shire", "CHART_REPO=oci://ghcr.io/mumia/charts/wallet-accountant")

	call(t, calls, "helm --kube-context shire -n wallet-accountant get values wallet-accountant")
	call(t, calls, "helm --kube-context shire -n wallet-accountant list")
	template := call(t, calls, "helm template wallet-accountant oci://ghcr.io/mumia/charts/wallet-accountant --version 1.2.3 ")
	if strings.Contains(template, "--plain-http") {
		t.Errorf("GHCR is HTTPS: %s", template)
	}
	run := call(t, calls, "kubectl --context shire -n wallet-accountant run wallet-accountant-bootstrap --rm -i --quiet --restart=Never")
	if !strings.Contains(run, "--image=ghcr.io/mumia/wallet-accountant:v1.2.3") {
		t.Errorf("the pod does not run the chart's image: %s", run)
	}
	for _, c := range calls {
		if strings.Contains(c, "k3d-") || strings.Contains(c, "localhost") {
			t.Errorf("a production call reaches the local cluster: %s", c)
		}
	}
}
