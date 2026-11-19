//go:build chart

package charttest

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestZitadelSetupIsAPostInstallAndPostUpgradeHookJob(t *testing.T) {
	t.Parallel()
	ms := render(t, combination{database: "external", zitadelSetup: true, ui: true}.files())

	job := ms.Get(t, "Job", "wa-wallet-accountant-zitadel-setup")
	if hook, _ := hookOf(job); hook != "post-install,post-upgrade" {
		t.Errorf("helm.sh/hook = %q, want post-install,post-upgrade", hook)
	}
	c := containers(job)[0]
	if got := c.At("securityContext.readOnlyRootFilesystem"); got != true {
		t.Errorf("readOnlyRootFilesystem = %v, want true", got)
	}
	if got := job.At("spec.template.spec.securityContext.runAsNonRoot"); got != true {
		t.Errorf("runAsNonRoot = %v, want true", got)
	}

	e := env(c)
	wantValues := map[string]string{
		"ZITADEL_URL":                   "https://auth.example.test",
		"PROJECT_ID":                    "wallet-accountant",
		"WEB_REDIRECT_URIS":             "https://wallet.example.test/auth/callback",
		"WEB_POST_LOGOUT_REDIRECT_URIS": "https://wallet.example.test/",
		"API_CLIENTS_REDIRECT_URIS":     "https://oauth.usebruno.com/callback http://localhost/oauth/callback",
		"ACCESS_TOKEN_LIFETIME":         "900s",
		"OIDC_CONFIGMAP":                "wa-wallet-accountant-oidc",
		"UI_DEPLOYMENT":                 "wa-wallet-accountant-ui",
		"ISSUER":                        "https://auth.example.test",
		"UI_REDIRECT_PATH":              "/auth/callback",
		"UI_POST_LOGOUT_REDIRECT_PATH":  "/",
	}
	for name, want := range wantValues {
		if got := e[name].Str("value"); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if got := e["ZITADEL_PAT"].Str("valueFrom.secretKeyRef.name"); got != "zitadel-setup-pat" {
		t.Errorf("ZITADEL_PAT from Secret %q, want zitadel-setup-pat", got)
	}

	// The Job writes the oidc ConfigMap, so the chart does not render it,
	// and the UI starts without it on the first install.
	if ms.Find("ConfigMap", "wa-wallet-accountant-oidc") != nil {
		t.Error("the chart renders the oidc ConfigMap the Job owns")
	}
	ui := ms.Get(t, "Deployment", "wa-wallet-accountant-ui")
	if got := ui.At("spec.template.spec.volumes.0.configMap.optional"); got != true {
		t.Errorf("the UI's config volume optional = %v, want true", got)
	}

	// Its ServiceAccount may write that ConfigMap and restart the UI, nothing else.
	sa := job.Str("spec.template.spec.serviceAccountName")
	if sa == "" || ms.Find("ServiceAccount", sa) == nil {
		t.Fatalf("the Job's ServiceAccount %q is not rendered", sa)
	}
	role := ms.Get(t, "Role", "wa-wallet-accountant-zitadel-setup")
	binding := ms.Get(t, "RoleBinding", "wa-wallet-accountant-zitadel-setup")
	if binding.Str("roleRef.name") != role.Str("metadata.name") || binding.Str("subjects.0.name") != sa {
		t.Errorf("RoleBinding %v does not bind %s to %s", binding, sa, role.Str("metadata.name"))
	}
	var resources []string
	for _, r := range role.List("rules") {
		for _, res := range asObject(r).List("resources") {
			resources = append(resources, res.(string))
		}
	}
	slices.Sort(resources)
	resources = slices.Compact(resources)
	if !slices.Equal(resources, []string{"configmaps", "deployments"}) {
		t.Errorf("the Role covers %v, want configmaps and deployments", resources)
	}

	// The script it runs is the chart's.
	script := ms.Get(t, "ConfigMap", "wa-wallet-accountant-zitadel-setup")
	if body, _ := script.Map("data")["zitadel-setup.sh"].(string); !strings.Contains(body, "CreateApplication") {
		t.Error("the script ConfigMap has no zitadel-setup.sh")
	}
}

func TestZitadelSetupWithoutTheUIRestartsNothing(t *testing.T) {
	t.Parallel()
	ms := render(t, combination{database: "external", zitadelSetup: true}.files())
	e := env(containers(ms.Get(t, "Job", "wa-wallet-accountant-zitadel-setup"))[0])
	if got := e["UI_DEPLOYMENT"].Str("value"); got != "" {
		t.Errorf("UI_DEPLOYMENT = %q, want empty", got)
	}
	role := ms.Get(t, "Role", "wa-wallet-accountant-zitadel-setup")
	for _, r := range role.List("rules") {
		if slices.Contains(asObject(r).List("resources"), any("deployments")) {
			t.Error("the Role grants deployments without a UI")
		}
	}
}

func TestZitadelSetupExtraURIsAndTrustedCA(t *testing.T) {
	t.Parallel()
	ms := render(t, combination{database: "external", zitadelSetup: true, ui: true}.files(),
		"oidc.zitadelSetup.web.extraRedirectUris={https://ui.dev.test/auth/callback}",
		"oidc.zitadelSetup.url=https://zitadel.internal",
		"trust.caConfigMap.name=local-ca")
	job := ms.Get(t, "Job", "wa-wallet-accountant-zitadel-setup")
	e := env(containers(job)[0])
	if got := e["WEB_REDIRECT_URIS"].Str("value"); got != "https://wallet.example.test/auth/callback https://ui.dev.test/auth/callback" {
		t.Errorf("WEB_REDIRECT_URIS = %q", got)
	}
	if got := e["ZITADEL_URL"].Str("value"); got != "https://zitadel.internal" {
		t.Errorf("ZITADEL_URL = %q", got)
	}
	if got := e["CURL_CA_BUNDLE"].Str("value"); got == "" {
		t.Error("the Job does not trust the extra CA")
	}
	app := containers(ms.Get(t, "Deployment", "wa-wallet-accountant-all"))[0]
	if got := env(app)["SSL_CERT_DIR"].Str("value"); got == "" {
		t.Error("the app does not trust the extra CA")
	}
}

// Browsers reach the host on a port that is not the scheme's default (the
// local cluster's 18443), so the URIs registered for `web` carry it. The Job
// reaches Zitadel through another address when its curl cannot resolve the
// URL's host itself (curl answers every *.localhost with 127.0.0.1).
func TestZitadelSetupRegistersTheRoutePortAndConnectsThroughAnotherAddress(t *testing.T) {
	t.Parallel()
	ms := render(t, combination{database: "external", zitadelSetup: true}.files(),
		"route.port=18443",
		"oidc.zitadelSetup.connectTo=ingress.internal:18443")
	e := env(containers(ms.Get(t, "Job", "wa-wallet-accountant-zitadel-setup"))[0])
	want := map[string]string{
		"WEB_REDIRECT_URIS":             "https://wallet.example.test:18443/auth/callback",
		"WEB_POST_LOGOUT_REDIRECT_URIS": "https://wallet.example.test:18443/",
		"CONNECT_TO":                    "ingress.internal:18443",
	}
	for name, w := range want {
		if got := e[name].Str("value"); got != w {
			t.Errorf("%s = %q, want %q", name, got, w)
		}
	}
}

func TestTheSchemesDefaultPortIsLeftOutOfTheRegisteredURIs(t *testing.T) {
	t.Parallel()
	ms := render(t, combination{database: "external", zitadelSetup: true}.files(), "route.port=443")
	e := env(containers(ms.Get(t, "Job", "wa-wallet-accountant-zitadel-setup"))[0])
	if got := e["WEB_REDIRECT_URIS"].Str("value"); got != "https://wallet.example.test/auth/callback" {
		t.Errorf("WEB_REDIRECT_URIS = %q", got)
	}
	if got := e["CONNECT_TO"].Str("value"); got != "" {
		t.Errorf("CONNECT_TO = %q, want empty", got)
	}
}

func TestTheZitadelSetupScriptParses(t *testing.T) {
	t.Parallel()
	if out, err := exec.CommandContext(t.Context(), "sh", "-n", "../files/zitadel-setup.sh").CombinedOutput(); err != nil {
		t.Fatalf("sh -n: %v\n%s", err, out)
	}
}
