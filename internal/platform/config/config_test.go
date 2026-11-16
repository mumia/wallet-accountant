package config_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/platform/config"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestFromEnvReadsEveryVariable(t *testing.T) {
	t.Parallel()

	cfg, err := config.FromEnv(env(map[string]string{
		"WA_HTTP_ADDR":        ":9090",
		"WA_DATABASE_URL":     "postgres://wa_app@db/wallet_accountant",
		"WA_OIDC_ISSUER":      "http://localhost:8081",
		"WA_OIDC_AUDIENCE":    "wallet-accountant",
		"WA_SHUTDOWN_TIMEOUT": "3s",
	}))
	if err != nil {
		t.Fatal(err)
	}

	want := config.Config{
		HTTPAddr:        ":9090",
		DatabaseURL:     "postgres://wa_app@db/wallet_accountant",
		OIDCIssuer:      "http://localhost:8081",
		OIDCAudience:    "wallet-accountant",
		ShutdownTimeout: 3 * time.Second,
	}
	if cfg != want {
		t.Fatalf("config = %+v, want %+v", cfg, want)
	}
}

func TestFromEnvDefaultsTheOptionalVariables(t *testing.T) {
	t.Parallel()

	cfg, err := config.FromEnv(env(map[string]string{
		"WA_DATABASE_URL":  "postgres://db",
		"WA_OIDC_ISSUER":   "http://idp",
		"WA_OIDC_AUDIENCE": "aud",
	}))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.HTTPAddr != ":8080" || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestFromEnvNamesEveryMissingVariable(t *testing.T) {
	t.Parallel()

	_, err := config.FromEnv(env(nil))
	if !errors.Is(err, config.ErrMissing) {
		t.Fatalf("err = %v, want ErrMissing", err)
	}

	for _, name := range []string{"WA_DATABASE_URL", "WA_OIDC_ISSUER", "WA_OIDC_AUDIENCE"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}
