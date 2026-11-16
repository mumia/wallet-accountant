// Package wiring is the composition root: it builds the bounded contexts,
// their adapters and the platform, and runs the binary in one of its run
// modes (ADR-0009).
package wiring

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"time"

	"golang.org/x/sync/errgroup"

	householdshttp "github.com/mumia/wallet-accountant/internal/households/adapters/http"
	householdsapp "github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/authn/oidc"
	"github.com/mumia/wallet-accountant/internal/platform/config"
	"github.com/mumia/wallet-accountant/internal/platform/database"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/telemetry"
)

// Mode is a run mode of the binary (ADR-0009).
type Mode string

// The run modes.
const (
	ModeAPI       Mode = "api"
	ModeWorker    Mode = "worker"
	ModeAll       Mode = "all"
	ModeMigrate   Mode = "migrate"
	ModeRebuild   Mode = "rebuild"
	ModeBootstrap Mode = "bootstrap"
)

// Modes lists every run mode.
var Modes = []Mode{ModeAPI, ModeWorker, ModeAll, ModeMigrate, ModeRebuild, ModeBootstrap}

var (
	// ErrUnknownMode is returned by ParseMode for a name that is not a run mode.
	ErrUnknownMode = errors.New("unknown run mode")
	// ErrModeNotAvailable is returned for a run mode that is not built yet.
	ErrModeNotAvailable = errors.New("run mode is not available yet")
)

// ParseMode returns the run mode called name.
func ParseMode(name string) (Mode, error) {
	m := Mode(name)
	if !slices.Contains(Modes, m) {
		return "", fmt.Errorf("%w %q (want one of %v)", ErrUnknownMode, name, Modes)
	}

	return m, nil
}

// Options replaces parts of the application, for tests.
type Options struct {
	// Authenticator replaces the OpenID Connect adapter.
	Authenticator authn.Authenticator
	// Listener replaces listening on cfg.HTTPAddr.
	Listener net.Listener
}

// Run runs the binary in mode until ctx is cancelled or the mode finishes.
func Run(ctx context.Context, mode Mode, cfg config.Config, tel *telemetry.Telemetry, opts Options) error {
	switch mode {
	case ModeAPI:
		return runAPI(ctx, cfg, tel, opts)
	case ModeWorker:
		return runWorker(ctx, tel)
	case ModeAll:
		g, ctx := errgroup.WithContext(ctx)
		g.Go(func() error { return runAPI(ctx, cfg, tel, opts) })
		g.Go(func() error { return runWorker(ctx, tel) })

		return g.Wait() //nolint:wrapcheck // the run modes' errors are already wrapped
	case ModeMigrate, ModeRebuild, ModeBootstrap:
		return fmt.Errorf("%w: %s", ErrModeNotAvailable, mode)
	default:
		return fmt.Errorf("%w %q", ErrUnknownMode, mode)
	}
}

func runAPI(ctx context.Context, cfg config.Config, tel *telemetry.Telemetry, opts Options) error {
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err //nolint:wrapcheck // already wrapped by database
	}
	defer pool.Close()

	auth := opts.Authenticator
	if auth == nil {
		auth = oidc.New(cfg.OIDCIssuer, cfg.OIDCAudience)
	}

	handler, err := httpapi.NewHandler(apiHandler{
		Handler: householdshttp.NewHandler(householdsapp.NewUsers()),
		Errors:  httpapi.Errors{Logger: tel.Logger},
	}, auth, tel)
	if err != nil {
		return err //nolint:wrapcheck // already wrapped by httpapi
	}

	ln := opts.Listener
	if ln == nil {
		ln, err = (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.HTTPAddr)
		if err != nil {
			return fmt.Errorf("listen on %s: %w", cfg.HTTPAddr, err)
		}
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return context.WithoutCancel(ctx) },
	}

	served := make(chan error, 1)

	go func() { served <- srv.Serve(ln) }()

	tel.Logger.InfoContext(ctx, "api listening", "address", ln.Addr().String())

	select {
	case err := <-served:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}

	tel.Logger.InfoContext(ctx, "api stopped")

	return nil
}

// runWorker will forward events to deferred handlers. No handler exists yet,
// so it waits until it is stopped.
func runWorker(ctx context.Context, tel *telemetry.Telemetry) error {
	tel.Logger.InfoContext(ctx, "worker started")
	<-ctx.Done()
	tel.Logger.InfoContext(ctx, "worker stopped")

	return nil
}

// apiHandler is the ogen handler: every context's HTTP adapter plus the
// problem+json error hooks.
type apiHandler struct {
	*householdshttp.Handler
	httpapi.Errors
}
