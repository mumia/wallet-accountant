package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestK3sReleaseIsTheK3dNodesTag(t *testing.T) {
	images := []string{"registry:3.1.2", "rancher/k3s:v1.37.2-k3s1", "ghcr.io/k3d-io/k3d-proxy:5.9.0"}
	got, ok := K3sRelease(images)
	if !ok || got != "v1.37.2+k3s1" {
		t.Fatalf("K3sRelease = %q, %v; want v1.37.2+k3s1, true", got, ok)
	}
	if got, ok := K3sRelease([]string{"docker.io/rancher/k3s:v1.35.5-k3s2@sha256:" + strings.Repeat("a", 64)}); !ok || got != "v1.35.5+k3s2" {
		t.Fatalf("K3sRelease with docker.io and a digest = %q, %v; want v1.35.5+k3s2, true", got, ok)
	}
	if _, ok := K3sRelease([]string{"registry:3.1.2"}); ok {
		t.Fatal("K3sRelease found a k3s release where there is none")
	}
}

func TestK3sImagesListsTheReleasesImagesTxt(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte("docker.io/rancher/mirrored-pause:3.10.2\n\ndocker.io/rancher/mirrored-coredns-coredns:1.14.7\n"))
	}))
	defer srv.Close()

	got, err := K3sImages(context.Background(), srv.URL, "v1.37.2+k3s1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/v1.37.2+k3s1/k3s-images.txt"; path != want {
		t.Errorf("fetched %s, want %s", path, want)
	}
	want := []string{"docker.io/rancher/mirrored-pause:3.10.2", "docker.io/rancher/mirrored-coredns-coredns:1.14.7"}
	if !slices.Equal(got, want) {
		t.Errorf("K3sImages = %q, want %q", got, want)
	}
}

func TestK3sImagesFailsWhenTheReleaseHasNoList(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if _, err := K3sImages(context.Background(), srv.URL, "v9.9.9+k3s1"); err == nil {
		t.Fatal("K3sImages succeeded on a 404")
	}
}

// The k3s update that broke the local cluster: the node moved to a new
// release, the lock kept the old release's pause image.
func TestAuditReportsAK3sImageTheLockLacks(t *testing.T) {
	lock, err := ParseLock([]byte("images:\n  - rancher/mirrored-pause:3.6@sha256:" + strings.Repeat("a", 64) + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	problems := Audit(lock, Rendered{Images: []string{"docker.io/rancher/mirrored-pause:3.10.2"}})
	if len(problems) != 1 || !strings.Contains(problems[0], "mirrored-pause:3.10.2") {
		t.Fatalf("Audit = %q, want the missing pause image", problems)
	}
}
