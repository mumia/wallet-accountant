package main

import (
	"slices"
	"strings"
	"testing"
)

const lockFixture = `
charts:
  - name: cert-manager
    source: oci://quay.io/jetstack/charts/cert-manager
    version: v1.21.3
  - name: zitadel
    source: https://charts.zitadel.com
    version: 10.5.0
images:
  - swaggerapi/swagger-ui:v5.34.1@sha256:1111111111111111111111111111111111111111111111111111111111111111
  - ghcr.io/zitadel/zitadel:v4.19.2@sha256:2222222222222222222222222222222222222222222222222222222222222222
host-images:
  - golang:1.27.4-alpine@sha256:3333333333333333333333333333333333333333333333333333333333333333
`

func mustLock(t *testing.T, s string) Lock {
	t.Helper()
	lock, err := ParseLock([]byte(s))
	if err != nil {
		t.Fatalf("ParseLock: %v", err)
	}
	return lock
}

func TestAuditReportsAnImageMissingFromTheLock(t *testing.T) {
	lock := mustLock(t, lockFixture)
	problems := Audit(lock, Rendered{Images: []string{"swaggerapi/swagger-ui:v5.34.1", "nginx:1.29.3"}})
	want := []string{"image docker.io/library/nginx:1.29.3 is not in images.lock"}
	if !slices.Equal(problems, want) {
		t.Fatalf("problems = %q, want %q", problems, want)
	}
}

func TestAuditMatchesImagesWhateverTheirSpelling(t *testing.T) {
	lock := mustLock(t, lockFixture)
	problems := Audit(lock, Rendered{Images: []string{
		"docker.io/swaggerapi/swagger-ui:v5.34.1",
		"index.docker.io/swaggerapi/swagger-ui:v5.34.1",
		"ghcr.io/zitadel/zitadel:v4.19.2@sha256:2222222222222222222222222222222222222222222222222222222222222222",
		"docker.io/library/golang:1.27.4-alpine",
	}})
	if len(problems) != 0 {
		t.Fatalf("problems = %q, want none", problems)
	}
}

func TestAuditReportsAnotherTagOrDigestOfALockedImage(t *testing.T) {
	lock := mustLock(t, lockFixture)
	problems := Audit(lock, Rendered{Images: []string{
		"ghcr.io/zitadel/zitadel:v4.20.0",
		"ghcr.io/zitadel/zitadel@sha256:9999999999999999999999999999999999999999999999999999999999999999",
		"swaggerapi/swagger-ui",
	}})
	if len(problems) != 3 {
		t.Fatalf("problems = %q, want 3", problems)
	}
}

func TestAuditIgnoresLocalBuildsAndUnsubstitutedReferences(t *testing.T) {
	lock := mustLock(t, lockFixture)
	problems := Audit(lock, Rendered{Images: []string{
		"wa-registry:5000/wallet-accountant:local-20261119",
		"wa-registry:5000/wallet-accountant:${WALLET_ACCOUNTANT_IMAGE_TAG}",
	}, LocalRegistry: "wa-registry:5000"})
	if len(problems) != 0 {
		t.Fatalf("problems = %q, want none", problems)
	}
}

func TestAuditReportsAChartMissingFromTheLock(t *testing.T) {
	lock := mustLock(t, lockFixture)
	problems := Audit(lock, Rendered{Charts: []Chart{
		{Name: "cert-manager", Version: "v1.21.3"},
		{Name: "zitadel", Version: "10.6.0"},
		{Name: "loki", Version: "6.0.0"},
	}})
	want := []string{
		"chart zitadel 10.6.0 is not in images.lock",
		"chart loki 6.0.0 is not in images.lock",
	}
	if !slices.Equal(problems, want) {
		t.Fatalf("problems = %q, want %q", problems, want)
	}
}

func TestParseLockRejectsAnImageWithoutDigest(t *testing.T) {
	_, err := ParseLock([]byte("images:\n  - nginx:1.29.3\n"))
	if err == nil || !strings.Contains(err.Error(), "nginx:1.29.3") {
		t.Fatalf("err = %v, want one naming nginx:1.29.3", err)
	}
}
