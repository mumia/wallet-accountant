//go:build chart

package charttest

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// combination is one way to install the chart: a database mode, the Zitadel
// setup Job on or off, and the UI slot on or off.
type combination struct {
	database     string // external or managed
	zitadelSetup bool
	ui           bool
}

func (c combination) String() string {
	return fmt.Sprintf("database=%s/zitadelSetup=%t/ui=%t", c.database, c.zitadelSetup, c.ui)
}

// files returns the values files that make up the combination, on top of
// base.yaml, which every install needs (issuer, route).
func (c combination) files() []string {
	names := []string{"base", "database-" + c.database}
	if c.zitadelSetup {
		names = append(names, "zitadel-setup")
	} else {
		names = append(names, "oidc-from-values")
	}
	if c.ui {
		names = append(names, "ui")
	}
	return values(names...)
}

func combinations() []combination {
	var cs []combination
	for _, db := range []string{"external", "managed"} {
		for _, setup := range []bool{false, true} {
			for _, ui := range []bool{false, true} {
				cs = append(cs, combination{database: db, zitadelSetup: setup, ui: ui})
			}
		}
	}
	return cs
}

func TestEveryCombinationLintsAndRenders(t *testing.T) {
	t.Parallel()
	for _, c := range combinations() {
		t.Run(c.String(), func(t *testing.T) {
			t.Parallel()
			if out, err := runHelm(t, append(helmArgs("lint", c.files(), nil), "--strict")...); err != nil {
				t.Fatalf("helm lint: %v\n%s", err, out)
			}
			ms := render(t, c.files())
			if len(ms) == 0 {
				t.Fatal("helm template rendered nothing")
			}
		})
	}
}

// With KUBECONFORM set (`make lint-chart` sets it), every combination's
// manifests are also validated against the Kubernetes, CloudNativePG and
// Gateway API schemas. KUBECONFORM_FLAGS holds the schema locations.
func TestEveryCombinationIsValidKubernetes(t *testing.T) {
	t.Parallel()
	kubeconform := os.Getenv("KUBECONFORM")
	if kubeconform == "" {
		t.Skip("KUBECONFORM is not set")
	}
	for _, c := range combinations() {
		t.Run(c.String(), func(t *testing.T) {
			t.Parallel()
			out, err := runHelm(t, helmArgs("template", c.files(), nil)...)
			if err != nil {
				t.Fatalf("helm template: %v\n%s", err, out)
			}
			args := append([]string{"-strict", "-summary"}, strings.Fields(os.Getenv("KUBECONFORM_FLAGS"))...)
			cmd := exec.CommandContext(t.Context(), kubeconform, args...) //nolint:gosec // G204: the pinned binary make lint-chart passes in KUBECONFORM
			cmd.Stdin = strings.NewReader(out)
			if res, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("kubeconform: %v\n%s", err, res)
			}
		})
	}
}
