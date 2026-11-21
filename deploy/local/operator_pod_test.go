// Package localtest checks deploy/local/operator-pod.sh, which runs the
// operator run modes (bootstrap, rebuild) as one-off pods: in the local
// cluster (make local-bootstrap) and in production (make prod-bootstrap).
// helm and kubectl are stand-ins that record their arguments, so no cluster
// or registry is contacted.
package localtest

import (
	"bytes"
	"errors"
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

// kubectl acts as one pod would: absent until `run` creates it (or POD_EXISTS
// says it was left behind), then in phase POD_PHASE (Succeeded), waiting for
// POD_WAITING, finished with exit code POD_EXIT (0) and log POD_LOG.
const kubectlStub = `#!/bin/sh
echo "kubectl $*" >>"$CALLS"
pod="$CALLS.pod"
case "$*" in
*"jsonpath={.status.phase}"*) echo "${POD_PHASE:-Succeeded}" ;;
*"state.waiting.reason"*) echo "${POD_WAITING:-}" ;;
*"state.terminated.exitCode"*) echo "${POD_EXIT:-0}" ;;
*" get pod "*) [ -f "$pod" ] || [ -n "${POD_EXISTS:-}" ] ;;
*" run "*) touch "$pod" ;;
*" logs "*) echo "${POD_LOG:-the output}" ;;
*" describe pod "*) echo "Events: image pull failed" ;;
*" delete pod "*) rm -f "$pod" ;;
esac
`

// operatorPod runs the script with the stand-ins and the given environment,
// and returns the calls it made, failing the test if the script fails.
func operatorPod(t *testing.T, mode string, env ...string) []string {
	t.Helper()
	res := runOperatorPod(t, mode, env...)
	if res.err != nil {
		t.Fatalf("operator-pod.sh %s failed: %v: %s", mode, res.err, res.stderr)
	}
	return res.calls
}

type result struct {
	calls          []string
	stdout, stderr string
	err            error
}

func runOperatorPod(t *testing.T, mode string, env ...string) result {
	t.Helper()
	bin := t.TempDir()
	for name, body := range map[string]string{"helm": helmStub, "kubectl": kubectlStub} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil { //nolint:gosec // G306: an executable stand-in in the test's own directory
			t.Fatal(err)
		}
	}
	calls := filepath.Join(t.TempDir(), "calls")
	var out, errOut bytes.Buffer
	cmd := exec.CommandContext(t.Context(), "sh", "operator-pod.sh", mode) //nolint:gosec // G204: the mode is the test's own
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CALLS="+calls,
		"HELM="+filepath.Join(bin, "helm"),
	)
	cmd.Env = append(cmd.Env, env...)
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	runErr := cmd.Run()
	data, err := os.ReadFile(calls) //nolint:gosec // G304: the test's own file
	if err != nil {
		t.Fatal(err)
	}
	return result{
		calls:  strings.Split(strings.TrimSpace(string(data)), "\n"),
		stdout: out.String(),
		stderr: errOut.String(),
		err:    runErr,
	}
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
	run := call(t, calls, "kubectl --context k3d-wallet-accountant -n wallet-accountant run wallet-accountant-bootstrap --restart=Never")
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
	run := call(t, calls, "kubectl --context shire -n wallet-accountant run wallet-accountant-bootstrap --restart=Never")
	if !strings.Contains(run, "--image=ghcr.io/mumia/wallet-accountant:v1.2.3") {
		t.Errorf("the pod does not run the chart's image: %s", run)
	}
	for _, c := range calls {
		if strings.Contains(c, "k3d-") || strings.Contains(c, "localhost") {
			t.Errorf("a production call reaches the local cluster: %s", c)
		}
	}
}

var local = []string{"LOCAL_CLUSTER=wallet-accountant", "LOCAL_REGISTRY_PORT=5050"}

// The pod runs detached and its log is read once it has finished: attaching
// (`kubectl run -i`) can lose a quick pod's output and still exit 0.
func TestThePodsLogIsPrintedOnceItHasFinished(t *testing.T) {
	res := runOperatorPod(t, "bootstrap", append(local, "POD_LOG=the code")...)
	if res.err != nil {
		t.Fatalf("operator-pod.sh failed: %v: %s", res.err, res.stderr)
	}
	if res.stdout != "the code\n" {
		t.Errorf("stdout = %q, want the pod's log", res.stdout)
	}
	run := call(t, res.calls, "kubectl --context k3d-wallet-accountant -n wallet-accountant run ")
	if strings.Contains(run, " -i") || strings.Contains(run, "--rm") {
		t.Errorf("the pod is attached to: %s", run)
	}
	var phase, logs int
	for i, c := range res.calls {
		switch {
		case strings.Contains(c, "jsonpath={.status.phase}") && phase == 0:
			phase = i
		case strings.Contains(c, " logs wallet-accountant-bootstrap"):
			logs = i
		}
	}
	if phase == 0 || logs < phase {
		t.Errorf("the log is not read after the pod's phase:\n%s", strings.Join(res.calls, "\n"))
	}
	assertDeleted(t, res.calls)
}

func TestAFailedPodsExitCodeIsTheScripts(t *testing.T) {
	res := runOperatorPod(t, "bootstrap", append(local, "POD_PHASE=Failed", "POD_EXIT=3", "POD_LOG=refused")...)
	var exit *exec.ExitError
	if !errors.As(res.err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("operator-pod.sh exited with %v, want the pod's exit code 3", res.err)
	}
	if res.stdout != "refused\n" {
		t.Errorf("stdout = %q, want the pod's log", res.stdout)
	}
	assertDeleted(t, res.calls)
}

func TestAnImageThatCannotBePulledFailsAtOnce(t *testing.T) {
	res := runOperatorPod(t, "bootstrap", append(local, "POD_PHASE=Pending", "POD_WAITING=ImagePullBackOff")...)
	if res.err == nil || !strings.Contains(res.stderr, "cannot start: ImagePullBackOff") {
		t.Fatalf("operator-pod.sh = %v, stderr %q; want it to fail on the image pull", res.err, res.stderr)
	}
	assertDeleted(t, res.calls)
}

// A pod left behind is reported, neither reused nor deleted: it may be
// another run's.
func TestAnExistingPodIsLeftAlone(t *testing.T) {
	res := runOperatorPod(t, "bootstrap", append(local, "POD_EXISTS=1")...)
	if res.err == nil || !strings.Contains(res.stderr, "already exists") {
		t.Fatalf("operator-pod.sh = %v, stderr %q; want it to refuse", res.err, res.stderr)
	}
	for _, c := range res.calls {
		if strings.Contains(c, " run ") || strings.Contains(c, " delete ") {
			t.Errorf("an existing pod was touched: %s", c)
		}
	}
}

func assertDeleted(t *testing.T, calls []string) {
	t.Helper()
	last := calls[len(calls)-1]
	if !strings.HasPrefix(last, "kubectl --context k3d-wallet-accountant -n wallet-accountant delete pod wallet-accountant-bootstrap") {
		t.Errorf("the pod is not deleted last:\n%s", strings.Join(calls, "\n"))
	}
}
