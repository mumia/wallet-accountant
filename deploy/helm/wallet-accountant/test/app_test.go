//go:build chart

package charttest

import (
	"slices"
	"testing"
)

var minimal = combination{database: "external"}

// env returns a container's environment as name -> the env entry.
func env(container Object) map[string]Object {
	out := map[string]Object{}
	for _, e := range container.List("env") {
		o := asObject(e)
		out[o.Str("name")] = o
	}
	return out
}

func containers(workload Object) []Object {
	path := "spec.template.spec.containers"
	if workload.Str("kind") == "Pod" {
		path = "spec.containers"
	}
	var cs []Object
	for _, c := range workload.List(path) {
		cs = append(cs, asObject(c))
	}
	return cs
}

func TestTheAppRunsAllInOneHardenedReplicaWithProbes(t *testing.T) {
	t.Parallel()
	ms := render(t, minimal.files())

	deployments := ms.OfKind("Deployment")
	app := ms.Get(t, "Deployment", "wa-wallet-accountant-all")
	if got := app.At("spec.replicas"); got != 1 {
		t.Errorf("replicas = %v, want 1", got)
	}
	if got := app.At("spec.template.spec.securityContext.runAsNonRoot"); got != true {
		t.Errorf("pod runAsNonRoot = %v, want true", got)
	}
	c := containers(app)[0]
	if got := c.List("args"); len(got) != 1 || got[0] != "all" {
		t.Errorf("args = %v, want [all]", got)
	}
	if got := c.At("securityContext.readOnlyRootFilesystem"); got != true {
		t.Errorf("readOnlyRootFilesystem = %v, want true", got)
	}
	if c.Map("livenessProbe") == nil || c.Map("readinessProbe") == nil {
		t.Errorf("probes: liveness %v, readiness %v; want both", c.Map("livenessProbe"), c.Map("readinessProbe"))
	}
	e := env(c)
	if got := e["WA_OIDC_ISSUER"].Str("value"); got != "https://auth.example.test" {
		t.Errorf("WA_OIDC_ISSUER = %q", got)
	}
	if got := e["WA_OIDC_AUDIENCE"].Str("value"); got != "wallet-accountant" {
		t.Errorf("WA_OIDC_AUDIENCE = %q", got)
	}
	if got := e["WA_DATABASE_URL"].Str("valueFrom.secretKeyRef.name"); got != "wa-db-app" {
		t.Errorf("WA_DATABASE_URL from Secret %q, want wa-db-app", got)
	}
	if got := e["WA_READER_DATABASE_URL"].Str("valueFrom.secretKeyRef.name"); got != "wa-db-reader" {
		t.Errorf("WA_READER_DATABASE_URL from Secret %q, want wa-db-reader", got)
	}
	if got := e["OTEL_EXPORTER_OTLP_ENDPOINT"].Str("value"); got != "http://wa-wallet-accountant-collector:4318" {
		t.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT = %q", got)
	}

	// The collector is the other Deployment; there is no api or worker.
	for _, d := range deployments {
		if n := d.Str("metadata.name"); n != "wa-wallet-accountant-all" && n != "wa-wallet-accountant-collector" {
			t.Errorf("unexpected Deployment %s", n)
		}
	}
}

func TestRunModesSplitTheAppIntoApiAndWorker(t *testing.T) {
	t.Parallel()
	ms := render(t, minimal.files(), "app.runModes={api,worker}")

	if ms.Find("Deployment", "wa-wallet-accountant-all") != nil {
		t.Error("the all Deployment is still rendered")
	}
	api := containers(ms.Get(t, "Deployment", "wa-wallet-accountant-api"))[0]
	worker := containers(ms.Get(t, "Deployment", "wa-wallet-accountant-worker"))[0]
	if got := api.List("args"); !slices.Equal(got, []any{"api"}) {
		t.Errorf("api args = %v", got)
	}
	if got := worker.List("args"); !slices.Equal(got, []any{"worker"}) {
		t.Errorf("worker args = %v", got)
	}
	if worker.At("ports") != nil || worker.At("readinessProbe") != nil {
		t.Error("the worker serves no HTTP, so it has no port and no probes")
	}
	svc := ms.Get(t, "Service", "wa-wallet-accountant")
	apiLabels := ms.Get(t, "Deployment", "wa-wallet-accountant-api").Map("spec.template.metadata.labels")
	for k, v := range svc.Map("spec.selector") {
		if apiLabels[k] != v {
			t.Errorf("the Service selects %s=%v, which the api pods lack", k, v)
		}
	}
}

func TestNoAppWorkloadButMigrateGetsTheOwnerURL(t *testing.T) {
	t.Parallel()
	for _, c := range combinations() {
		t.Run(c.String(), func(t *testing.T) {
			t.Parallel()
			ms := render(t, c.files())
			var withOwner []string
			for _, o := range ms {
				for _, ct := range containers(o) {
					if _, ok := env(ct)["WA_OWNER_DATABASE_URL"]; ok {
						withOwner = append(withOwner, o.Str("kind")+"/"+o.Str("metadata.name"))
					}
				}
			}
			if !slices.Equal(withOwner, []string{"Job/wa-wallet-accountant-migrate"}) {
				t.Errorf("WA_OWNER_DATABASE_URL is given to %v, want only Job/wa-wallet-accountant-migrate", withOwner)
			}
		})
	}
}

func TestMigrateIsAPreInstallAndPreUpgradeHook(t *testing.T) {
	t.Parallel()
	ms := render(t, minimal.files())
	job := ms.Get(t, "Job", "wa-wallet-accountant-migrate")
	hook, _ := job.Map("metadata.annotations")["helm.sh/hook"].(string)
	if hook != "pre-install,pre-upgrade" {
		t.Errorf("helm.sh/hook = %q, want pre-install,pre-upgrade", hook)
	}
	c := containers(job)[0]
	if got := c.List("args"); !slices.Equal(got, []any{"migrate"}) {
		t.Errorf("args = %v, want [migrate]", got)
	}
	e := env(c)
	if got := e["WA_OWNER_DATABASE_URL"].Str("valueFrom.secretKeyRef.name"); got != "wa-db-owner" {
		t.Errorf("WA_OWNER_DATABASE_URL from Secret %q, want wa-db-owner", got)
	}
	// migrate sets the app's and the reader's passwords from their URLs.
	for _, name := range []string{"WA_DATABASE_URL", "WA_READER_DATABASE_URL"} {
		if _, ok := e[name]; !ok {
			t.Errorf("migrate has no %s", name)
		}
	}
	if got := c.At("securityContext.readOnlyRootFilesystem"); got != true {
		t.Errorf("readOnlyRootFilesystem = %v, want true", got)
	}
}
