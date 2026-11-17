// Command wallet-accountant is the single binary of Wallet Accountant. Its
// run mode is the first argument (default all): api, worker, all, migrate,
// rebuild or bootstrap (ADR-0009). Everything else is configured through
// environment variables.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/mumia/wallet-accountant/internal/platform/config"
	"github.com/mumia/wallet-accountant/internal/platform/telemetry"
	"github.com/mumia/wallet-accountant/internal/wiring"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "wallet-accountant:", err)
		os.Exit(1)
	}
}

func run() error {
	name := string(wiring.ModeAll)
	if len(os.Args) > 1 {
		name = os.Args[1]
	}

	mode, err := wiring.ParseMode(name)
	if err != nil {
		return err //nolint:wrapcheck // already descriptive
	}

	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tel, err := telemetry.New(ctx, telemetry.Options{Stdout: os.Stdout})
	if err != nil {
		return err //nolint:wrapcheck // already wrapped by telemetry
	}

	otel.SetTracerProvider(tel.TracerProvider)
	otel.SetMeterProvider(tel.MeterProvider)
	otel.SetTextMapPropagator(tel.Propagator)

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		if err := tel.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintln(os.Stderr, "wallet-accountant: telemetry shutdown:", err)
		}
	}()

	tel.Logger.InfoContext(ctx, "starting", "mode", string(mode))

	return wiring.Run(ctx, mode, cfg, tel, wiring.Options{}) //nolint:wrapcheck // already wrapped by wiring
}
