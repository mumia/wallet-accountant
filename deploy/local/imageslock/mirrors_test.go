package main

import (
	"slices"
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
          "^(.*)$": "docker.io/$1"
      ghcr.io:
        endpoint: [http://wa-registry:5000]
        rewrite:
          "^(.*)$": "ghcr.io/$1"
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
	want := []string{`registries.yaml: quay.io is not mirrored to http://wa-registry:5000 with rewrite "^(.*)$": "quay.io/$1"`}
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
          "^(.*)$": "docker.io/$1"
      ghcr.io:
        endpoint: [http://wa-registry:5000]
        rewrite:
          "^(.*)$": "ghcr.io/$1"
`), lock, "wa-registry:5000")
	want := []string{
		"k3s: --disable-default-registry-endpoint is not set, so containerd falls back to upstream registries",
		`registries.yaml: no "*" mirror to http://wa-registry:5000, so other registries are pulled from upstream`,
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
