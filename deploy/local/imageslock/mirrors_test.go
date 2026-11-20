package main

import (
	"slices"
	"strings"
	"testing"
)

const k3dFixture = `
apiVersion: k3d.io/v1alpha5
kind: Simple
image: rancher/k3s:v1.35.5-k3s1
options:
  k3s:
    extraArgs:
      - arg: --disable-default-registry-endpoint
        nodeFilters: ["server:*"]
registries:
  config: |
    mirrors:
      docker.io:
        endpoint: [http://wa-registry:5000]
        rewrite:
          "^": "docker.io/"
      ghcr.io:
        endpoint: [http://wa-registry:5000]
        rewrite:
          "^": "ghcr.io/"
      "*":
        endpoint: [http://wa-registry:5000]
`

func TestMirrorsAcceptsEveryLockedRegistryMirroredFailingClosed(t *testing.T) {
	lock := mustLock(t, lockFixture)
	if got := MirrorProblems([]byte(k3dFixture), lock, "wa-registry:5000"); len(got) != 0 {
		t.Fatalf("problems = %q, want none", got)
	}
}

func TestMirrorsReportsAnUnmirroredRegistry(t *testing.T) {
	lock := mustLock(t, lockFixture)
	lock.Images = append(lock.Images, mustRef(t, "quay.io/jetstack/cert-manager-controller:v1.21.3@sha256:4444444444444444444444444444444444444444444444444444444444444444"))
	got := MirrorProblems([]byte(k3dFixture), lock, "wa-registry:5000")
	want := []string{`registries.yaml: quay.io is not mirrored to http://wa-registry:5000 with rewrite "^": "quay.io/"`}
	if !slices.Equal(got, want) {
		t.Fatalf("problems = %q, want %q", got, want)
	}
}

func TestMirrorsReportsAFallbackToUpstream(t *testing.T) {
	lock := mustLock(t, lockFixture)
	got := MirrorProblems([]byte(`
registries:
  config: |
    mirrors:
      docker.io:
        endpoint: [http://wa-registry:5000]
        rewrite:
          "^": "docker.io/"
      ghcr.io:
        endpoint: [http://wa-registry:5000]
        rewrite:
          "^": "ghcr.io/"
`), lock, "wa-registry:5000")
	want := []string{
		"k3s: --disable-default-registry-endpoint is not set, so containerd falls back to upstream registries",
		`registries.yaml: no "*" mirror to http://wa-registry:5000, so other registries are pulled from upstream`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("problems = %q, want %q", got, want)
	}
}

// k3d expands environment variables in its config, $1 included, so a
// rewrite to "docker.io/$1" reaches k3s as "docker.io/" and every pull 404s.
func TestMirrorsReportsADollarK3dWouldExpand(t *testing.T) {
	lock := mustLock(t, lockFixture)
	config := strings.Replace(k3dFixture, `"^": "docker.io/"`, `"^(.*)$": "docker.io/$1"`, 1)
	got := MirrorProblems([]byte(config), lock, "wa-registry:5000")
	want := []string{
		`registries.yaml: a "$" in k3d's config is an environment variable to k3d, which replaces it (with nothing): docker.io's rewrite "^(.*)$": "docker.io/$1"`,
		`registries.yaml: docker.io is not mirrored to http://wa-registry:5000 with rewrite "^": "docker.io/"`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("problems = %q, want %q", got, want)
	}
}

func mustRef(t *testing.T, s string) Ref {
	t.Helper()
	r, err := ParseRef(s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
