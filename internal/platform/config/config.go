// Package config reads the application's configuration. Configuration comes
// only from environment variables; telemetry exporters are configured with the
// standard OTEL_* variables, which the OpenTelemetry SDK reads itself.
package config

import (
	"errors"
	"fmt"
	"time"
)

// Config is the application's configuration.
type Config struct {
	// HTTPAddr is the address the api run mode listens on (WA_HTTP_ADDR).
	HTTPAddr string
	// DatabaseURL is the PostgreSQL connection string of wa_app
	// (WA_DATABASE_URL).
	DatabaseURL string
	// ReaderDatabaseURL is the connection string of wa_event_store_reader
	// (WA_READER_DATABASE_URL), which the worker, all and rebuild run modes
	// need.
	ReaderDatabaseURL string
	// OwnerDatabaseURL is the connection string of wa_owner
	// (WA_OWNER_DATABASE_URL), which only the migrate run mode needs.
	OwnerDatabaseURL string
	// OIDCIssuer is the OpenID Connect issuer whose tokens are accepted
	// (WA_OIDC_ISSUER).
	OIDCIssuer string
	// OIDCAudience is the audience (client ID) tokens must be issued for
	// (WA_OIDC_AUDIENCE).
	OIDCAudience string
	// ShutdownTimeout bounds a graceful shutdown (WA_SHUTDOWN_TIMEOUT).
	ShutdownTimeout time.Duration
}

const (
	defaultHTTPAddr        = ":8080"
	defaultShutdownTimeout = 10 * time.Second
)

// ErrMissing is wrapped by FromEnv for every required variable that is unset.
var ErrMissing = errors.New("required environment variable is not set")

// FromEnv reads the configuration through getenv, normally os.Getenv.
func FromEnv(getenv func(string) string) (Config, error) {
	cfg := Config{
		HTTPAddr:          getenv("WA_HTTP_ADDR"),
		DatabaseURL:       getenv("WA_DATABASE_URL"),
		ReaderDatabaseURL: getenv("WA_READER_DATABASE_URL"),
		OwnerDatabaseURL:  getenv("WA_OWNER_DATABASE_URL"),
		OIDCIssuer:        getenv("WA_OIDC_ISSUER"),
		OIDCAudience:      getenv("WA_OIDC_AUDIENCE"),
		ShutdownTimeout:   defaultShutdownTimeout,
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}

	if raw := getenv("WA_SHUTDOWN_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("WA_SHUTDOWN_TIMEOUT: %w", err)
		}

		cfg.ShutdownTimeout = d
	}

	var errs []error

	for _, v := range []struct{ name, value string }{
		{"WA_DATABASE_URL", cfg.DatabaseURL},
		{"WA_OIDC_ISSUER", cfg.OIDCIssuer},
		{"WA_OIDC_AUDIENCE", cfg.OIDCAudience},
	} {
		if v.value == "" {
			errs = append(errs, fmt.Errorf("%s: %w", v.name, ErrMissing))
		}
	}

	return cfg, errors.Join(errs...)
}
