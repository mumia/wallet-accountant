//go:build chart

package charttest

import (
	"strings"
	"testing"
)

// Bad values fail at install: values.schema.json (or, where JSON Schema
// cannot say it, a template check) rejects them with a message naming the
// value.
func TestBadValuesAreRejected(t *testing.T) {
	t.Parallel()
	ok := minimal.files()
	cases := []struct {
		name  string
		files []string
		sets  []string
		want  string // a fragment of the error
	}{
		{"unknown database mode", ok, []string{"database.mode=sqlite"}, "/database/mode"},
		{
			"external database without the owner Secret", values("base", "oidc-from-values", "database-external"),
			[]string{"database.external.ownerUrl.secretName="},
			"secretName",
		},
		{
			"external database without the app Secret", values("base", "oidc-from-values", "database-external"),
			[]string{"database.external.appUrl.secretName="},
			"secretName",
		},
		{
			"external database without the reader Secret", values("base", "oidc-from-values", "database-external"),
			[]string{"database.external.readerUrl.secretName="},
			"secretName",
		},
		{
			"managed database with no instance", values("base", "oidc-from-values", "database-managed"),
			[]string{"database.managed.instances=0"},
			"instances",
		},
		{
			"no issuer", values("database-external", "oidc-from-values"),
			[]string{"route.hostname=wallet.example.test", "route.parentRefs[0].name=gw"},
			"issuer",
		},
		{"issuer that is not a URL", ok, []string{"oidc.issuer=auth.example.test"}, "issuer"},
		{
			"Zitadel setup without a token", values("base", "database-external", "zitadel-setup"),
			[]string{"oidc.zitadelSetup.patSecret.name="},
			"/oidc/zitadelSetup/patSecret/name",
		},
		{
			"access token lifetime that is not seconds", values("base", "database-external", "zitadel-setup"),
			[]string{"oidc.zitadelSetup.tokenLifetimes.accessToken=15m"},
			"accessToken",
		},
		{
			"extra redirect URI that is not a URL", values("base", "database-external", "zitadel-setup"),
			[]string{"oidc.zitadelSetup.web.extraRedirectUris={localhost:5173/cb}"},
			"extraRedirectUris",
		},
		{
			"UI without a web client ID and without the setup", values("base", "database-external", "ui"),
			nil, "/oidc/clientIds/web",
		},
		{"redirect path that is not a path", ok, []string{"oidc.redirectPath=auth/callback"}, "redirectPath"},
		{"route without a hostname", ok, []string{"route.hostname="}, "hostname"},
		{
			"route without a Gateway", values("database-external", "oidc-from-values"),
			[]string{"oidc.issuer=https://auth.example.test", "route.hostname=wallet.example.test"},
			"parentRefs",
		},
		{
			"UI without an image", values("base", "database-external", "oidc-from-values", "ui"),
			[]string{"ui.image.repository="},
			"repository",
		},
		{
			"UI with a relative config path", values("base", "database-external", "oidc-from-values", "ui"),
			[]string{"ui.configPath=config.json"},
			"configPath",
		},
		{"run modes that are not all, or api and worker", ok, []string{"app.runModes={all,worker}"}, "runModes"},
		{"migrate as a long-running run mode", ok, []string{"app.runModes={migrate}"}, "runModes"},
		{"migrate as an operator pod", ok, []string{"operatorPod.mode=migrate"}, "/operatorPod/mode"},
		{"misspelt top-level value", ok, []string{"databse.mode=managed"}, "databse"},
		{"misspelt nested value", ok, []string{"collector.exporter.nop=null"}, "exporter"},
		{
			"pipeline naming an exporter that is not configured", ok,
			[]string{"collector.pipelines.traces={otlphttp/tempo}"},
			"otlphttp/tempo",
		},
		{"pipeline with no exporter", ok, []string{"collector.pipelines.traces=null"}, "traces"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := renderError(t, tc.files, tc.sets...)
			if !strings.Contains(out, tc.want) {
				t.Errorf("the error does not mention %q:\n%s", tc.want, out)
			}
		})
	}
}
