package localtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// localBuild runs `make local-build` with go, docker and helm stand-ins;
// docker fails the commands that start with failing. It returns whether
// make succeeded, the calls the stand-ins logged and the build file it wrote
// ("" if none).
func localBuild(t *testing.T, failing string, args ...string) (ok bool, calls []string, build string) {
	t.Helper()
	bin := t.TempDir()
	stubs := map[string]string{
		"go":   "#!/bin/sh\necho \"go $*\" >>\"$CALLS\"\n",
		"helm": "#!/bin/sh\necho \"helm $*\" >>\"$CALLS\"\n",
		"docker": "#!/bin/sh\necho \"docker $*\" >>\"$CALLS\"\n" +
			"case \"docker $*\" in \"$FAILING\"*) exit 1 ;; esac\n",
	}
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil { //nolint:gosec // G306: an executable stand-in in the test's own directory
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	calls = nil
	buildFile := filepath.Join(dir, "local-build.env")
	args = append([]string{"-C", "../..", "--no-print-directory", "local-build",
		"HELM=" + filepath.Join(bin, "helm"), "LOCAL_BUILD=" + buildFile}, args...)
	cmd := exec.CommandContext(t.Context(), "make", args...) //nolint:gosec // G204: the test's own arguments
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CALLS="+filepath.Join(dir, "calls"),
		"FAILING="+failing,
	)
	out, err := cmd.CombinedOutput()
	t.Logf("make local-build:\n%s", out)
	if logged, err := os.ReadFile(filepath.Join(dir, "calls")); err == nil { //nolint:gosec // G304: the test's own file
		calls = strings.Split(strings.TrimSpace(string(logged)), "\n")
	}
	if written, err := os.ReadFile(buildFile); err == nil { //nolint:gosec // G304: the test's own file
		build = string(written)
	}
	return err == nil, calls, build
}

func TestLocalBuildRecordsTheBuildItPushed(t *testing.T) {
	ok, calls, build := localBuild(t, "never", "LOCAL_STAND_IN_UI=1")
	if !ok {
		t.Fatalf("local-build failed; calls:\n%s", strings.Join(calls, "\n"))
	}
	for _, key := range []string{"WALLET_ACCOUNTANT_VERSION=", "WALLET_ACCOUNTANT_IMAGE_TAG=local-", "WALLET_ACCOUNTANT_UI_TAG=local-"} {
		if !strings.Contains(build, key) {
			t.Errorf("build file has no %s:\n%s", key, build)
		}
	}
}

// A failed image build or push stops local-build before it pushes a chart or
// records a tag that isn't in the registry.
func TestAFailedImageStopsLocalBuild(t *testing.T) {
	for _, failing := range []string{"docker build --network=none", "docker push -q localhost:5050/wallet-accountant:"} {
		t.Run(failing, func(t *testing.T) {
			ok, calls, build := localBuild(t, failing)
			if ok {
				t.Error("local-build succeeded")
			}
			if build != "" {
				t.Errorf("recorded a build:\n%s", build)
			}
			for _, c := range calls {
				if strings.HasPrefix(c, "helm push") {
					t.Errorf("pushed the chart: %s", c)
				}
			}
		})
	}
}

// The stand-in UI's failed build stops local-build before the UI's tag is
// recorded.
func TestAFailedStandInUIStopsLocalBuild(t *testing.T) {
	for _, failing := range []string{"docker build -t localhost:5050/wallet-accountant-stand-in-ui:", "docker push -q localhost:5050/wallet-accountant-stand-in-ui:"} {
		t.Run(failing, func(t *testing.T) {
			ok, _, build := localBuild(t, failing, "LOCAL_STAND_IN_UI=1")
			if ok {
				t.Error("local-build succeeded")
			}
			if strings.Contains(build, "WALLET_ACCOUNTANT_UI_TAG=") {
				t.Errorf("recorded the stand-in UI's tag:\n%s", build)
			}
		})
	}
}
