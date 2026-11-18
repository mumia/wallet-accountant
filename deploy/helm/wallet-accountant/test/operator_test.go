//go:build chart

package charttest

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// overrides renders the operator pod the way a Make target does, and returns
// the JSON it passes to `kubectl run --overrides`: the document without
// helm's `---` and `# Source:` lines.
func overrides(t *testing.T, files []string, sets ...string) map[string]any {
	t.Helper()
	args := append(helmArgs("template", files, sets), "--show-only", "templates/operator-pod.yaml")
	out, err := runHelm(t, args...)
	if err != nil {
		t.Fatalf("helm template: %v\n%s", err, out)
	}
	var body []string
	for _, line := range strings.Split(out, "\n") {
		if line == "---" || strings.HasPrefix(line, "# Source:") {
			continue
		}
		body = append(body, line)
	}
	var pod map[string]any
	if err := json.Unmarshal([]byte(strings.Join(body, "\n")), &pod); err != nil {
		t.Fatalf("the operator pod is not JSON: %v\n%s", err, out)
	}
	return pod
}

func TestOperatorPodsRunBootstrapAndRebuildWithoutTheOwnerURL(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"bootstrap", "rebuild"} {
		for _, db := range []string{"external", "managed"} {
			t.Run(mode+"/"+db, func(t *testing.T) {
				t.Parallel()
				pod := Object(overrides(t, combination{database: db}.files(), "operatorPod.mode="+mode))
				if pod.Str("kind") != "Pod" || pod.Str("apiVersion") != "v1" {
					t.Errorf("kind = %s/%s, want v1/Pod", pod.Str("apiVersion"), pod.Str("kind"))
				}
				if got := pod.Str("spec.restartPolicy"); got != "Never" {
					t.Errorf("restartPolicy = %q, want Never", got)
				}
				c := containers(pod)[0]
				if got := c.List("args"); !slices.Equal(got, []any{mode}) {
					t.Errorf("args = %v, want [%s]", got, mode)
				}
				e := env(c)
				if _, ok := e["WA_OWNER_DATABASE_URL"]; ok {
					t.Error("the operator pod gets the owner URL")
				}
				if _, ok := e["WA_READER_DATABASE_URL"]; !ok {
					t.Error("the operator pod has no reader URL (rebuild needs it)")
				}
				if got := c.At("securityContext.readOnlyRootFilesystem"); got != true {
					t.Errorf("readOnlyRootFilesystem = %v, want true", got)
				}
			})
		}
	}
}

func TestNoOperatorPodIsInstalled(t *testing.T) {
	t.Parallel()
	for _, c := range combinations() {
		t.Run(c.String(), func(t *testing.T) {
			t.Parallel()
			if pods := render(t, c.files()).OfKind("Pod"); len(pods) != 0 {
				t.Errorf("an install renders %d Pods", len(pods))
			}
		})
	}
}
