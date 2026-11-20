package localtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// stubbed writes stand-ins that log one line of arguments per call to
// $CALLS, and returns the directory to put first on the PATH.
func stubbed(t *testing.T, names ...string) string {
	t.Helper()
	bin := t.TempDir()
	for _, name := range names {
		body := "#!/bin/sh\necho \"" + name + " $*\" >>\"$CALLS\"\n"
		if name == "docker" {
			body += "[ \"$1\" != version ] || echo linux/arm64\n"
		}
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil { //nolint:gosec // G306: an executable stand-in in the test's own directory
			t.Fatal(err)
		}
	}
	return bin
}

func readCalls(t *testing.T, path string) []string {
	t.Helper()
	out, err := os.ReadFile(path) //nolint:gosec // G304: the test's own file
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n")
}

const staleRegistry = "docker rm -f wa-registry-prefetch"

// An interrupted prefetch may leave its temporary registry running on the
// registry's port, where it would pass for the cluster's registry and keep
// k3d from creating the cluster's. The next prefetch removes it before it
// looks for a registry.
func TestPrefetchRemovesAStaleTemporaryRegistryFirst(t *testing.T) {
	bin := stubbed(t, "docker", "curl", "go")
	calls := filepath.Join(t.TempDir(), "calls")
	cmd := exec.CommandContext(t.Context(), "sh", "prefetch.sh")
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CALLS="+calls,
		"ENV_REPO="+t.TempDir(), "LOCAL_REGISTRY=wa-registry", "LOCAL_REGISTRY_PORT=5050",
		"LOCAL_REGISTRY_DIR="+t.TempDir(), "LOCAL_REGISTRY_CONFIG=/dev/null", "HELM=helm",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prefetch.sh: %v\n%s", err, out)
	}
	got := readCalls(t, calls)
	removed := slices.Index(got, staleRegistry)
	probed := slices.IndexFunc(got, func(c string) bool { return strings.HasPrefix(c, "curl ") })
	if removed < 0 || (probed >= 0 && probed < removed) {
		t.Errorf("calls:\n%s\nwant %q before the registry is probed", strings.Join(got, "\n"), staleRegistry)
	}
}

// local-down removes a temporary registry an interrupted prefetch left.
func TestLocalDownRemovesAStaleTemporaryRegistry(t *testing.T) {
	bin := stubbed(t, "docker", "k3d", "mkcert")
	calls := filepath.Join(t.TempDir(), "calls")
	cmd := exec.CommandContext(t.Context(), "make", "-C", "../..", "--no-print-directory", "local-down")
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "CALLS="+calls)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make local-down: %v\n%s", err, out)
	}
	got := readCalls(t, calls)
	if !slices.Contains(got, "k3d cluster delete wallet-accountant") || !slices.Contains(got, staleRegistry) {
		t.Errorf("calls:\n%s\nwant the cluster deleted and %q", strings.Join(got, "\n"), staleRegistry)
	}
}

// local-up runs before the app is rebuilt, so the chart it would wait on is
// the last one built: after a values change, that one may never become
// ready. local-up waits on the platform only and leaves the app to
// local-deploy, which builds and waits on it.
func TestLocalUpDoesNotWaitOnTheApp(t *testing.T) {
	bin := stubbed(t, "docker", "curl", "go", "k3d", "flux", "kubectl", "helm")
	ca := t.TempDir()
	mkcert := "#!/bin/sh\necho \"mkcert $*\" >>\"$CALLS\"\n[ \"$1\" != -CAROOT ] || echo " + ca + "\n"
	if err := os.WriteFile(filepath.Join(bin, "mkcert"), []byte(mkcert), 0o700); err != nil { //nolint:gosec // G306: an executable stand-in in the test's own directory
		t.Fatal(err)
	}
	kubectl := "#!/bin/sh\necho \"kubectl $*\" >>\"$CALLS\"\n" +
		"case \"$*\" in *' get kustomizations.kustomize.toolkit.fluxcd.io -o name') " +
		"printf '%s\\n' kustomization.kustomize.toolkit.fluxcd.io/cluster kustomization.kustomize.toolkit.fluxcd.io/zitadel " +
		"kustomization.kustomize.toolkit.fluxcd.io/wallet-accountant ;; esac\n"
	if err := os.WriteFile(filepath.Join(bin, "kubectl"), []byte(kubectl), 0o700); err != nil { //nolint:gosec // G306: an executable stand-in in the test's own directory
		t.Fatal(err)
	}
	env := t.TempDir()
	for _, f := range []string{"rootCA.pem", "rootCA-key.pem"} {
		if err := os.WriteFile(filepath.Join(ca, f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{"clusters/local/apps/wallet-accountant", "clusters/local/swagger", "clusters/local/build"} {
		if err := os.MkdirAll(filepath.Join(env, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	calls := filepath.Join(t.TempDir(), "calls")
	cmd := exec.CommandContext(t.Context(), "make", "-C", "../..", "--no-print-directory", "local-up", //nolint:gosec // G204: the test's own arguments
		"ENV_REPO="+env, "HELM="+filepath.Join(bin, "helm"), "LOCAL_REGISTRY_DIR="+t.TempDir(),
		"LOCAL_BUILD="+filepath.Join(t.TempDir(), "none"), "LOCAL_STAGE="+filepath.Join(t.TempDir(), "stage"))
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "CALLS="+calls)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make local-up: %v\n%s", err, out)
	}
	got := readCalls(t, calls)
	var waited []string
	for _, c := range got {
		if strings.Contains(c, " wait ") {
			waited = append(waited, c)
			if strings.Contains(c, "fluxcd.io/wallet-accountant") || strings.Contains(c, "--all") {
				t.Errorf("local-up waits on the app: %s", c)
			}
		}
		if strings.HasPrefix(c, "flux ") && strings.Contains(c, "reconcile kustomization wallet-accountant") {
			t.Errorf("local-up waits on the app's reconcile: %s", c)
		}
	}
	if !slices.ContainsFunc(waited, func(c string) bool { return strings.Contains(c, "/zitadel") }) {
		t.Errorf("local-up does not wait on the platform; waits:\n%s", strings.Join(waited, "\n"))
	}
}
