// Package releasetest checks deploy/release/version.sh, which the release
// workflow (.github/workflows/release.yaml) runs to turn the pushed ref into
// the image tag and the chart's version and appVersion.
package releasetest

import (
	"bytes"
	"os/exec"
	"testing"
)

const commit = "0123456789abcdef0123456789abcdef01234567"

// version runs the script as the workflow does and returns its stdout, its
// stderr and whether it succeeded.
func version(t *testing.T, refType, refName string) (stdout, stderr string, ok bool) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := exec.CommandContext(t.Context(), "sh", "version.sh", refType, refName, commit) //nolint:gosec // G204: the arguments are the test's own refs
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	return out.String(), errOut.String(), err == nil
}

func TestReleaseTagPublishesImageAndChart(t *testing.T) {
	stdout, stderr, ok := version(t, "tag", "v1.2.3")
	if !ok {
		t.Fatalf("version.sh failed: %s", stderr)
	}
	want := "image-tag=v1.2.3\nchart=true\nchart-version=1.2.3\napp-version=v1.2.3\n"
	if stdout != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestOnlySemverTagsAndMainPublish(t *testing.T) {
	refs := []struct{ refType, refName string }{
		{"tag", "1.2.3"},
		{"tag", "v1.2"},
		{"tag", "v1.2.3.4"},
		{"tag", "v01.2.3"},
		{"tag", "v1.2.3+build.5"},
		{"tag", "v1.2.3-"},
		{"tag", "v1.2.3-rc.01"},
		{"tag", "v1.2.3-00"},
		{"tag", "v1.2.3-rc..1"},
		{"tag", "release-1"},
		{"tag", "v1.2.3 "},
		{"branch", "feature"},
		{"branch", "main2"},
	}
	for _, ref := range refs {
		t.Run(ref.refType+"/"+ref.refName, func(t *testing.T) {
			stdout, stderr, ok := version(t, ref.refType, ref.refName)
			if ok {
				t.Fatalf("version.sh accepted it: %s", stdout)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing for $GITHUB_OUTPUT", stdout)
			}
			if stderr == "" {
				t.Error("no error message")
			}
		})
	}
}

func TestPreReleaseTagPublishesImageAndChart(t *testing.T) {
	stdout, stderr, ok := version(t, "tag", "v2.0.0-rc.1")
	if !ok {
		t.Fatalf("version.sh failed: %s", stderr)
	}
	want := "image-tag=v2.0.0-rc.1\nchart=true\nchart-version=2.0.0-rc.1\napp-version=v2.0.0-rc.1\n"
	if stdout != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
}

// Pre-release identifiers as semver 2.0.0 allows them: alphanumeric ones
// may start with a zero, numeric ones may not (Helm refuses the chart).
func TestPreReleaseIdentifiersFollowSemver(t *testing.T) {
	for _, tag := range []string{"v1.2.3-0", "v1.2.3-rc.10", "v1.2.3-0a.1", "v1.2.3-alpha-1.x-y", "v1.2.3--"} {
		t.Run(tag, func(t *testing.T) {
			if stdout, stderr, ok := version(t, "tag", tag); !ok {
				t.Errorf("version.sh refused it: %s%s", stdout, stderr)
			}
		})
	}
}

func TestMainPublishesACommitImageOnly(t *testing.T) {
	stdout, stderr, ok := version(t, "branch", "main")
	if !ok {
		t.Fatalf("version.sh failed: %s", stderr)
	}
	want := "image-tag=main-0123456\nchart=false\n"
	if stdout != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
}
