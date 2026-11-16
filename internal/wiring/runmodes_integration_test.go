//go:build integration

package wiring_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/apitest"
	"github.com/mumia/wallet-accountant/internal/platform/config"
	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
	"github.com/mumia/wallet-accountant/internal/platform/telemetry"
	"github.com/mumia/wallet-accountant/internal/wiring"
)

func newTelemetry(t *testing.T) *telemetry.Telemetry {
	t.Helper()

	tel, err := telemetry.New(t.Context(), telemetry.Options{Stdout: &apitest.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = tel.Shutdown(context.Background()) })

	return tel
}

func configFor(urls databasetest.URLs) config.Config {
	return config.Config{
		DatabaseURL:       urls.App,
		ReaderDatabaseURL: urls.Reader,
		OwnerDatabaseURL:  urls.Owner,
		ShutdownTimeout:   5 * time.Second,
	}
}

func TestMigrateThenRebuildThenWorker(t *testing.T) {
	t.Parallel()
	tel := newTelemetry(t)
	urls := databasetest.NewEmpty(t)
	cfg := configFor(urls)

	databasetest.Migrate(t, func() error {
		return wiring.Run(t.Context(), wiring.ModeMigrate, cfg, tel, wiring.Options{})
	})

	// Migrating again changes nothing.
	databasetest.Migrate(t, func() error {
		return wiring.Run(t.Context(), wiring.ModeMigrate, cfg, tel, wiring.Options{})
	})

	if err := wiring.Run(t.Context(), wiring.ModeRebuild, cfg, tel, wiring.Options{}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() {
		done <- wiring.Run(ctx, wiring.ModeWorker, cfg, tel, wiring.Options{PollInterval: 10 * time.Millisecond})
	}()

	select {
	case err := <-done:
		t.Fatalf("worker stopped on its own: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	cancel()

	if err := <-done; err != nil {
		t.Errorf("worker stopped with an error: %v", err)
	}
}

func TestRunModesNeedTheirDatabaseURLs(t *testing.T) {
	t.Parallel()
	tel := newTelemetry(t)

	for _, mode := range []wiring.Mode{wiring.ModeMigrate, wiring.ModeRebuild, wiring.ModeWorker} {
		err := wiring.Run(t.Context(), mode, config.Config{DatabaseURL: "postgres://wa_app@localhost/x"}, tel, wiring.Options{})
		if !errors.Is(err, wiring.ErrMissingDatabaseURL) {
			t.Errorf("%s without its URL: err = %v, want ErrMissingDatabaseURL", mode, err)
		}
	}
}

func TestTheHarnessDrainsPendingEvents(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	// No handler is registered yet, so there is nothing to wait for; the
	// helper must still return.
	app.Drain(t)
}
