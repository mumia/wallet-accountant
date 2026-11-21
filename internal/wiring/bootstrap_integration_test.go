//go:build integration

package wiring_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"github.com/mumia/wallet-accountant/internal/apitest"
	householdsapp "github.com/mumia/wallet-accountant/internal/households/app"
)

type me struct {
	UserID                  string `json:"user_id"`
	IsInstanceAdministrator bool   `json:"is_instance_administrator"`
}

func TestBootstrapAdmitsTheFirstInstanceAdministratorOnce(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	first, err := app.RunBootstrap(t)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	second, err := app.RunBootstrap(t)
	if err != nil {
		t.Fatalf("bootstrap rerun: %v", err)
	}

	if first == second {
		t.Fatal("a rerun printed the same code; want a new one")
	}

	if n := len(second); n < 43 {
		t.Errorf("code has %d characters, want at least 43 (256 bits)", n)
	}

	owner := app.TokenFor("https://issuer.example", "owner")

	// The rerun replaced the earlier code.
	app.Post(t, "/api/registrations", owner, map[string]string{"invitation_code": first}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-invitation")

	var registered me
	app.Post(t, "/api/registrations", owner, map[string]string{"invitation_code": second}).
		AssertJSON(t, http.StatusCreated, &registered)

	if !registered.IsInstanceAdministrator {
		t.Errorf("registered = %+v, want Instance administrator", registered)
	}

	var got me
	app.Get(t, "/api/me", owner).AssertJSON(t, http.StatusOK, &got)

	if got != registered {
		t.Errorf("GET /me = %+v, want %+v", got, registered)
	}

	if _, err := app.RunBootstrap(t); !errors.Is(err, householdsapp.ErrBootstrapRefused) {
		t.Errorf("bootstrap after the code was redeemed: err = %v, want ErrBootstrapRefused", err)
	}

	assertNotLogged(t, app, first, second)
}

func assertNotLogged(t *testing.T, app *apitest.App, secrets ...string) {
	t.Helper()

	records := app.Logs.Records(t)
	if len(records) == 0 {
		t.Fatal("no log records captured; the check would prove nothing")
	}

	otlp := app.OTLPLogs.Records()
	if len(otlp) == 0 {
		t.Fatal("no OpenTelemetry log records captured; the check would prove nothing")
	}

	var all strings.Builder

	for _, rec := range records {
		for k, v := range rec {
			all.WriteString(k + "=" + toString(v) + "\n")
		}
	}

	for _, rec := range otlp {
		all.WriteString(rec.Body().String() + "\n")
		rec.WalkAttributes(func(kv attribute.KeyValue) bool {
			all.WriteString(string(kv.Key) + "=" + kv.Value.String() + "\n")

			return true
		})
	}

	for _, s := range secrets {
		if strings.Contains(all.String(), s) {
			t.Errorf("an Invitation code appears in the logs")
		}
	}
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}

	if m, ok := v.(map[string]any); ok {
		var b strings.Builder
		for k, x := range m {
			b.WriteString(k + "=" + toString(x) + ";")
		}

		return b.String()
	}

	return ""
}
