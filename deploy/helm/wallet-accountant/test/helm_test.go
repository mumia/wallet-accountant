//go:build chart

// Package charttest checks the Helm chart in deploy/helm/wallet-accountant
// through helm itself: `helm lint` and `helm template` with every combination
// of database mode, Zitadel setup and UI slot, and values.schema.json
// rejecting bad values (spec "Testing Decisions"). `make lint-chart` runs it
// with the pinned helm binary in HELM.
package charttest

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	chartDir    = ".."
	releaseName = "wa"
	namespace   = "wallet"
)

// helm returns the helm binary to run: HELM, or helm on the PATH.
func helm() string {
	if h := os.Getenv("HELM"); h != "" {
		return h
	}
	return "helm"
}

// values returns the paths of the named files in test/values.
func values(names ...string) []string {
	paths := make([]string, len(names))
	for i, n := range names {
		paths[i] = filepath.Join("values", n+".yaml")
	}
	return paths
}

// helmArgs runs helm with args, the values files and the --set flags.
func helmArgs(cmd string, files []string, sets []string) []string {
	args := []string{cmd}
	if cmd == "template" {
		args = append(args, releaseName)
	}
	args = append(args, chartDir, "--namespace", namespace)
	for _, f := range files {
		args = append(args, "--values", f)
	}
	for _, s := range sets {
		args = append(args, "--set", s)
	}
	return args
}

func runHelm(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), helm(), args...) //nolint:gosec // G204: helm is the pinned binary make lint-chart passes in HELM
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.String() + stderr.String(), err
	}
	return stdout.String(), nil
}

// Manifests is the rendered chart, one object per document.
type Manifests []Object

// Object is one rendered Kubernetes object.
type Object map[string]any

// render runs helm template and fails the test when it fails.
func render(t *testing.T, files []string, sets ...string) Manifests {
	t.Helper()
	out, err := runHelm(t, helmArgs("template", files, sets)...)
	if err != nil {
		t.Fatalf("helm template %v %v: %v\n%s", files, sets, err, out)
	}
	return decode(t, out)
}

// renderError runs helm template and returns its output, failing the test
// when it succeeds.
func renderError(t *testing.T, files []string, sets ...string) string {
	t.Helper()
	out, err := runHelm(t, helmArgs("template", files, sets)...)
	if err == nil {
		t.Fatalf("helm template %v %v succeeded, want an error", files, sets)
	}
	return out
}

func decode(t *testing.T, out string) Manifests {
	t.Helper()
	var ms Manifests
	dec := yaml.NewDecoder(strings.NewReader(out))
	for {
		// Decoded as a plain map: yaml.v3 gives nested mappings the type of
		// the outer one, and At expects map[string]any.
		var o map[string]any
		err := dec.Decode(&o)
		if errors.Is(err, io.EOF) {
			return ms
		}
		if err != nil {
			t.Fatalf("decoding the rendered chart: %v\n%s", err, out)
		}
		if o != nil {
			ms = append(ms, o)
		}
	}
}

// Find returns the object of that kind and name, or nil.
func (ms Manifests) Find(kind, name string) Object {
	for _, o := range ms {
		if o.Str("kind") == kind && o.Str("metadata.name") == name {
			return o
		}
	}
	return nil
}

// Get fails the test unless the object of that kind and name exists.
func (ms Manifests) Get(t *testing.T, kind, name string) Object {
	t.Helper()
	o := ms.Find(kind, name)
	if o == nil {
		t.Fatalf("no %s %q in the rendered chart; it has %v", kind, name, ms.names())
	}
	return o
}

// OfKind returns every object of that kind.
func (ms Manifests) OfKind(kind string) Manifests {
	var out Manifests
	for _, o := range ms {
		if o.Str("kind") == kind {
			out = append(out, o)
		}
	}
	return out
}

func (ms Manifests) names() []string {
	n := make([]string, len(ms))
	for i, o := range ms {
		n[i] = o.Str("kind") + "/" + o.Str("metadata.name")
	}
	return n
}

// At returns the value at a dotted path; a numeric element indexes a list.
func (o Object) At(path string) any {
	var cur any = map[string]any(o)
	for _, p := range strings.Split(path, ".") {
		switch c := cur.(type) {
		case map[string]any:
			cur = c[p]
		case []any:
			i := 0
			for _, r := range p {
				if r < '0' || r > '9' {
					return nil
				}
				i = i*10 + int(r-'0')
			}
			if i >= len(c) {
				return nil
			}
			cur = c[i]
		default:
			return nil
		}
	}
	return cur
}

// Str returns the value at path as a string ("" when absent).
func (o Object) Str(path string) string {
	s, _ := o.At(path).(string)
	return s
}

// List returns the list at path.
func (o Object) List(path string) []any {
	l, _ := o.At(path).([]any)
	return l
}

// Map returns the map at path as an Object.
func (o Object) Map(path string) Object {
	m, _ := o.At(path).(map[string]any)
	return m
}

func asObject(v any) Object {
	m, _ := v.(map[string]any)
	return m
}
