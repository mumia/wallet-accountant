package main

import (
	"slices"
	"testing"
)

const renderedFixture = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: swagger-ui
spec:
  template:
    spec:
      initContainers:
        - name: init
          image: busybox:1.37.0
      containers:
        - name: swagger-ui
          image: swaggerapi/swagger-ui:v5.34.1
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: apps-db
spec:
  imageName: ghcr.io/cloudnative-pg/postgresql:18.7-system-trixie
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: zitadel
  namespace: zitadel
spec:
  type: oci
  url: oci://wa-registry:5000/charts
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: zitadel
  namespace: zitadel
spec:
  chart:
    spec:
      chart: zitadel
      version: 10.5.0
      sourceRef:
        kind: HelmRepository
        name: zitadel
  values:
    image:
      tag: v4.19.2
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: cert-manager
  namespace: cert-manager
spec:
  url: oci://quay.io/jetstack/charts/cert-manager
  ref:
    tag: v1.21.3
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: cert-manager
  namespace: cert-manager
spec:
  chartRef:
    kind: OCIRepository
    name: cert-manager
`

func TestExtractFindsImagesAnywhereInADocument(t *testing.T) {
	found, err := Extract([]byte(renderedFixture))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"busybox:1.37.0", "swaggerapi/swagger-ui:v5.34.1", "ghcr.io/cloudnative-pg/postgresql:18.7-system-trixie"}
	if !slices.Equal(found.Images, want) {
		t.Fatalf("images = %q, want %q", found.Images, want)
	}
}

func TestExtractFindsHelmReleasesWithTheirChartAndSource(t *testing.T) {
	found, err := Extract([]byte(renderedFixture))
	if err != nil {
		t.Fatal(err)
	}
	want := []Release{
		{Name: "zitadel", Namespace: "zitadel", Chart: Chart{Name: "zitadel", Version: "10.5.0"}, Source: "oci://wa-registry:5000/charts", Values: map[string]any{"image": map[string]any{"tag": "v4.19.2"}}},
		{Name: "cert-manager", Namespace: "cert-manager", Chart: Chart{Name: "cert-manager", Version: "v1.21.3"}, Source: "oci://quay.io/jetstack/charts/cert-manager"},
	}
	if len(found.Releases) != len(want) {
		t.Fatalf("releases = %+v, want %+v", found.Releases, want)
	}
	for i, r := range found.Releases {
		if r.Name != want[i].Name || r.Namespace != want[i].Namespace || r.Chart != want[i].Chart || r.Source != want[i].Source {
			t.Errorf("release %d = %+v, want %+v", i, r, want[i])
		}
	}
	if found.Releases[0].Values["image"].(map[string]any)["tag"] != "v4.19.2" {
		t.Errorf("zitadel values = %v", found.Releases[0].Values)
	}
}

func TestOfflineSourcesReportsAFluxSourceOutsideTheLocalRegistry(t *testing.T) {
	found, err := Extract([]byte(renderedFixture))
	if err != nil {
		t.Fatal(err)
	}
	got := found.UpstreamSources("wa-registry:5000")
	want := []string{"OCIRepository cert-manager/cert-manager fetches oci://quay.io/jetstack/charts/cert-manager, not from wa-registry:5000"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
