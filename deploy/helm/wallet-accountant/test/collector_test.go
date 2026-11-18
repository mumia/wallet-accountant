//go:build chart

package charttest

import (
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// collectorConfig returns the collector's parsed configuration.
func collectorConfig(t *testing.T, ms Manifests) Object {
	t.Helper()
	cm := ms.Get(t, "ConfigMap", "wa-wallet-accountant-collector")
	var cfg map[string]any
	raw, _ := cm.Map("data")["config.yaml"].(string)
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestTheCollectorDiscardsEverySignalByDefault(t *testing.T) {
	t.Parallel()
	ms := render(t, minimal.files())

	d := ms.Get(t, "Deployment", "wa-wallet-accountant-collector")
	c := containers(d)[0]
	if got := c.At("securityContext.readOnlyRootFilesystem"); got != true {
		t.Errorf("readOnlyRootFilesystem = %v, want true", got)
	}
	svc := ms.Get(t, "Service", "wa-wallet-accountant-collector")
	var ports []any
	for _, p := range svc.List("spec.ports") {
		ports = append(ports, asObject(p).At("port"))
	}
	if !slices.Equal(ports, []any{4317, 4318}) {
		t.Errorf("collector Service ports = %v, want [4317 4318]", ports)
	}

	cfg := collectorConfig(t, ms)
	if got := cfg.Map("exporters"); len(got) != 1 || !hasKey(got, "nop") {
		t.Errorf("exporters = %v, want only nop", got)
	}
	for _, signal := range []string{"traces", "metrics", "logs"} {
		got := cfg.List("service.pipelines." + signal + ".exporters")
		if !slices.Equal(got, []any{"nop"}) {
			t.Errorf("%s exporters = %v, want [nop]", signal, got)
		}
		if got := cfg.List("service.pipelines." + signal + ".receivers"); !slices.Equal(got, []any{"otlp"}) {
			t.Errorf("%s receivers = %v, want [otlp]", signal, got)
		}
	}
}

func hasKey(m Object, k string) bool {
	_, ok := m[k]
	return ok
}

func TestTheCollectorsExportersComeFromValues(t *testing.T) {
	t.Parallel()
	ms := render(t, append(minimal.files(), "values/collector-grafana.yaml"))

	cfg := collectorConfig(t, ms)
	if got := cfg.Str("exporters.otlphttp/tempo.endpoint"); got != "http://tempo.monitoring:4318" {
		t.Errorf("tempo exporter endpoint = %q", got)
	}
	if got := cfg.List("service.pipelines.traces.exporters"); !slices.Equal(got, []any{"otlphttp/tempo"}) {
		t.Errorf("traces exporters = %v", got)
	}
	if got := cfg.List("service.pipelines.logs.exporters"); !slices.Equal(got, []any{"otlphttp/loki"}) {
		t.Errorf("logs exporters = %v", got)
	}
}

func TestWithoutTheCollectorTheAppExportsToTheExternalEndpoint(t *testing.T) {
	t.Parallel()
	ms := render(t, minimal.files(), "collector.enabled=false", "collector.externalEndpoint=http://otel.monitoring:4318")
	if ms.Find("Deployment", "wa-wallet-accountant-collector") != nil {
		t.Error("the collector is rendered while disabled")
	}
	e := env(containers(ms.Get(t, "Deployment", "wa-wallet-accountant-all"))[0])
	if got := e["OTEL_EXPORTER_OTLP_ENDPOINT"].Str("value"); got != "http://otel.monitoring:4318" {
		t.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT = %q", got)
	}
}
