//go:build chart

package charttest

import (
	"encoding/json"
	"slices"
	"testing"
)

// backendsByPath returns each rule's PathPrefix and the Service it routes to.
func backendsByPath(route Object) map[string]string {
	out := map[string]string{}
	for _, r := range route.List("spec.rules") {
		rule := asObject(r)
		for _, m := range rule.List("matches") {
			match := asObject(m)
			if match.Str("path.type") != "PathPrefix" {
				continue
			}
			out[match.Str("path.value")] = rule.Str("backendRefs.0.name")
		}
	}
	return out
}

func TestTheRouteSendsAPIToTheApp(t *testing.T) {
	t.Parallel()
	ms := render(t, minimal.files())

	route := ms.Get(t, "HTTPRoute", "wa-wallet-accountant")
	if got := route.List("spec.hostnames"); !slices.Equal(got, []any{"wallet.example.test"}) {
		t.Errorf("hostnames = %v", got)
	}
	parent := route.Map("spec.parentRefs.0")
	if parent.Str("name") != "gateway" || parent.Str("namespace") != "gateway-system" || parent.Str("sectionName") != "https" {
		t.Errorf("parentRefs = %v", route.List("spec.parentRefs"))
	}
	got := backendsByPath(route)
	want := map[string]string{"/api": "wa-wallet-accountant"}
	if len(got) != len(want) || got["/api"] != want["/api"] {
		t.Errorf("rules = %v, want %v (no UI)", got, want)
	}
	if port := route.At("spec.rules.0.backendRefs.0.port"); port != 8080 {
		t.Errorf("/api backend port = %v, want 8080", port)
	}
}

func TestWithTheUIEveryOtherPathGoesToTheUI(t *testing.T) {
	t.Parallel()
	ms := render(t, combination{database: "external", ui: true}.files())

	got := backendsByPath(ms.Get(t, "HTTPRoute", "wa-wallet-accountant"))
	if got["/api"] != "wa-wallet-accountant" || got["/"] != "wa-wallet-accountant-ui" || len(got) != 2 {
		t.Errorf("rules = %v, want /api to the app and / to the UI", got)
	}
}

func TestTheUISlotIsOffByDefault(t *testing.T) {
	t.Parallel()
	ms := render(t, minimal.files())
	if ms.Find("Deployment", "wa-wallet-accountant-ui") != nil || ms.Find("Service", "wa-wallet-accountant-ui") != nil {
		t.Error("the UI is rendered while ui.enabled is false")
	}
}

func TestTheUIServesItsConfigFromValues(t *testing.T) {
	t.Parallel()
	ms := render(t, combination{database: "external", ui: true}.files(), "ui.configPath=/usr/share/nginx/html/config.json")

	d := ms.Get(t, "Deployment", "wa-wallet-accountant-ui")
	c := containers(d)[0]
	if got := c.Str("image"); got != "ghcr.io/mumia/wallet-accountant-ui:0.1.0" {
		t.Errorf("image = %q", got)
	}
	if got := c.At("securityContext.readOnlyRootFilesystem"); got != true {
		t.Errorf("readOnlyRootFilesystem = %v, want true", got)
	}
	if got := c.Str("readinessProbe.httpGet.path"); got != "/healthz" {
		t.Errorf("readiness probe path = %q, want /healthz", got)
	}
	if got := c.At("ports.0.containerPort"); got != 8080 {
		t.Errorf("port = %v, want 8080", got)
	}

	// config.json is mounted at ui.configPath from the oidc ConfigMap.
	mount := asObject(c.List("volumeMounts")[0])
	if mount.Str("mountPath") != "/usr/share/nginx/html/config.json" || mount.Str("subPath") != "config.json" {
		t.Errorf("config mount = %v", mount)
	}
	vol := d.Map("spec.template.spec.volumes.0")
	if vol.Str("name") != mount.Str("name") || vol.Str("configMap.name") != "wa-wallet-accountant-oidc" {
		t.Errorf("config volume = %v", vol)
	}

	cm := ms.Get(t, "ConfigMap", "wa-wallet-accountant-oidc")
	raw, _ := cm.Map("data")["config.json"].(string)
	var cfg struct {
		APIBaseURL string `json:"apiBaseUrl"`
		OIDC       struct {
			Issuer                 string `json:"issuer"`
			ClientID               string `json:"clientId"`
			Audience               string `json:"audience"`
			RedirectPath           string `json:"redirectPath"`
			PostLogoutRedirectPath string `json:"postLogoutRedirectPath"`
		} `json:"oidc"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("config.json: %v\n%s", err, raw)
	}
	if cfg.APIBaseURL != "/api" || cfg.OIDC.Issuer != "https://auth.example.test" || cfg.OIDC.ClientID != "web-client-id" ||
		cfg.OIDC.Audience != "wallet-accountant" || cfg.OIDC.RedirectPath != "/auth/callback" || cfg.OIDC.PostLogoutRedirectPath != "/" {
		t.Errorf("config.json = %+v", cfg)
	}
}

// The contract asks for a read-only root filesystem; a static server such as
// nginx still writes its pid and temporary files, so /tmp is an emptyDir.
func TestTheUIHasAWritableTmp(t *testing.T) {
	t.Parallel()
	ms := render(t, combination{database: "external", ui: true}.files())

	d := ms.Get(t, "Deployment", "wa-wallet-accountant-ui")
	var tmpVolume string
	for _, m := range containers(d)[0].List("volumeMounts") {
		if mount := asObject(m); mount.Str("mountPath") == "/tmp" {
			tmpVolume = mount.Str("name")
		}
	}
	if tmpVolume == "" {
		t.Fatal("no volume mounted on /tmp")
	}
	for _, v := range d.List("spec.template.spec.volumes") {
		if vol := asObject(v); vol.Str("name") == tmpVolume {
			if _, ok := vol["emptyDir"]; !ok {
				t.Errorf("/tmp volume = %v, want an emptyDir", vol)
			}
			return
		}
	}
	t.Errorf("no volume %q", tmpVolume)
}

func TestWithoutZitadelSetupTheClientIDsComeFromValues(t *testing.T) {
	t.Parallel()
	ms := render(t, minimal.files())
	data := ms.Get(t, "ConfigMap", "wa-wallet-accountant-oidc").Map("data")
	want := map[string]string{
		"issuer":                "https://auth.example.test",
		"audience":              "wallet-accountant",
		"web-client-id":         "web-client-id",
		"api-clients-client-id": "api-clients-client-id",
	}
	for k, v := range want {
		if data[k] != v {
			t.Errorf("%s = %v, want %s", k, data[k], v)
		}
	}
}
