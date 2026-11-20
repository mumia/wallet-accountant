package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// Pinning adds the digest of what upstream serves to every unpinned image
// under images:, host-images: and prod-images:, whether the list is indented or not, and
// leaves charts and pinned images as they are.
func TestPinAddsTheDigestToEveryUnpinnedImage(t *testing.T) {
	const lock = `charts:
  - name: x
    source: oci://example.test/x
    version: 1.0.0
images:
- ghcr.io/a/one:1
  - ghcr.io/a/two:2 # a comment
  - ghcr.io/a/three:3@sha256:abc
host-images:
- docker.io/library/golang:1.26
prod-images:
  - ghcr.io/a/four:4
`
	inspected := map[string]bool{}
	got, pinned, err := pinLock(lock, func(ref string) ([]byte, error) {
		inspected[ref] = true
		return []byte(ref), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	digest := func(ref string) string {
		sum := sha256.Sum256([]byte(ref))
		return ref + "@sha256:" + hex.EncodeToString(sum[:])
	}
	want := strings.NewReplacer(
		"- ghcr.io/a/one:1\n", "- "+digest("ghcr.io/a/one:1")+"\n",
		"- ghcr.io/a/two:2 #", "- "+digest("ghcr.io/a/two:2")+" #",
		"- docker.io/library/golang:1.26\n", "- "+digest("docker.io/library/golang:1.26")+"\n",
		"- ghcr.io/a/four:4\n", "- "+digest("ghcr.io/a/four:4")+"\n",
	).Replace(lock)
	if got != want {
		t.Errorf("pinned lock:\n%s\nwant:\n%s", got, want)
	}
	if len(pinned) != 4 || len(inspected) != 4 {
		t.Errorf("pinned %v (inspected %v), want the four unpinned images", pinned, inspected)
	}
}
