// Package wiring is the composition root: it builds the bounded contexts,
// their adapters and the platform, and runs the binary in one of its run
// modes (ADR-0009).
package wiring

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"time"

	"golang.org/x/sync/errgroup"

	householdshttp "github.com/mumia/wallet-accountant/internal/households/adapters/http"
	ingestionhttp "github.com/mumia/wallet-accountant/internal/ingestion/adapters/http"
	ledgerhttp "github.com/mumia/wallet-accountant/internal/ledger/adapters/http"
	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/authn/oidc"
	"github.com/mumia/wallet-accountant/internal/platform/config"
	"github.com/mumia/wallet-accountant/internal/platform/database"
	"github.com/mumia/wallet-accountant/internal/platform/database/migrations"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/platform/forwarding"
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
	// ErrMissingDatabaseURL is returned when a run mode needs a connection
	// string that is not configured.
	ErrMissingDatabaseURL = errors.New("database URL is not configured")
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
	// PollInterval replaces how often the worker looks for new events when
	// none is pending (default 1s).
	PollInterval time.Duration
	// Stdout replaces os.Stdout, where the bootstrap run mode prints the
	// Invitation code.
	Stdout io.Writer
}

// Run runs the binary in mode until ctx is cancelled or the mode finishes.
func Run(ctx context.Context, mode Mode, cfg config.Config, tel *telemetry.Telemetry, opts Options) error {
	switch mode {
	case ModeAPI:
		return runAPI(ctx, cfg, tel, opts)
	case ModeWorker:
		return runWorker(ctx, cfg, tel, opts)
	case ModeAll:
		g, ctx := errgroup.WithContext(ctx)
		g.Go(func() error { return runAPI(ctx, cfg, tel, opts) })
		g.Go(func() error { return runWorker(ctx, cfg, tel, opts) })

		return g.Wait() //nolint:wrapcheck // the run modes' errors are already wrapped
	case ModeMigrate:
		return runMigrate(ctx, cfg, tel)
	case ModeRebuild:
		return runRebuild(ctx, cfg, tel)
	case ModeBootstrap:
		return runBootstrap(ctx, cfg, tel, opts)
	default:
		return fmt.Errorf("%w %q", ErrUnknownMode, mode)
	}
}

func runAPI(ctx context.Context, cfg config.Config, tel *telemetry.Telemetry, opts Options) error {
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		if ctx.Err() != nil {
			return nil // stopped while starting
		}

		return err //nolint:wrapcheck // already wrapped by database
	}
	defer pool.Close()

	auth := opts.Authenticator
	if auth == nil {
		auth = oidc.New(cfg.OIDCIssuer, cfg.OIDCAudience)
	}

	contexts := NewContexts(pool)

	handler, err := httpapi.NewHandler(apiHandler{
		Handler:             householdshttp.NewHandler(contexts.Users, contexts.Households),
		NotificationHandler: householdshttp.NewNotificationHandler(contexts.Notifications),
		ledgerHandler:       ledgerhttp.NewHandler(contexts.Categories),
		AccountHandler:      ledgerhttp.NewAccountHandler(contexts.Accounts),
		ConnectionHandler:   ingestionhttp.NewConnectionHandler(contexts.Connections),
		TransactionHandler:  ledgerhttp.NewTransactionHandler(contexts.Transactions),
		StatementImportHandler: ingestionhttp.NewStatementImportHandler(contexts.StatementImports,
			contexts.ImportedTransactions),
		Errors: httpapi.Errors{Logger: tel.Logger},
	}, auth, pool, tel)
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

// runWorker forwards events to every deferred handler until it is stopped.
func runWorker(ctx context.Context, cfg config.Config, tel *telemetry.Telemetry, opts Options) error {
	if cfg.ReaderDatabaseURL == "" {
		return fmt.Errorf("%w: WA_READER_DATABASE_URL (worker)", ErrMissingDatabaseURL)
	}

	reader, err := database.Open(ctx, cfg.ReaderDatabaseURL)
	if err != nil {
		if ctx.Err() != nil {
			return nil // stopped while starting
		}

		return err //nolint:wrapcheck // already wrapped by database
	}
	defer reader.Close()

	tel.Logger.InfoContext(ctx, "worker started", "handlers", len(handlers()))

	err = forwarding.Run(ctx, forwarding.Config{
		Reader:         reader,
		Handlers:       handlers(),
		Logger:         tel.Logger,
		TracerProvider: tel.TracerProvider,
		PollInterval:   opts.PollInterval,
		ResendInterval: opts.PollInterval,
		CloseTimeout:   cfg.ShutdownTimeout,
	})
	if err != nil {
		return err //nolint:wrapcheck // already wrapped by forwarding
	}

	tel.Logger.InfoContext(ctx, "worker stopped")

	return nil
}

// runMigrate applies the migrations as wa_owner and sets the passwords of
// wa_app and wa_event_store_reader to those in their connection strings.
func runMigrate(ctx context.Context, cfg config.Config, tel *telemetry.Telemetry) error {
	if cfg.OwnerDatabaseURL == "" {
		return fmt.Errorf("%w: WA_OWNER_DATABASE_URL (migrate)", ErrMissingDatabaseURL)
	}

	logins := []string{cfg.DatabaseURL}
	if cfg.ReaderDatabaseURL != "" {
		logins = append(logins, cfg.ReaderDatabaseURL)
	}

	if err := migrations.Up(ctx, cfg.OwnerDatabaseURL, logins...); err != nil {
		return err //nolint:wrapcheck // already wrapped by migrations
	}

	tel.Logger.InfoContext(ctx, "migrations applied")

	return nil
}

// runRebuild rebuilds every registered fold from the event store.
func runRebuild(ctx context.Context, cfg config.Config, tel *telemetry.Telemetry) error {
	if cfg.ReaderDatabaseURL == "" {
		return fmt.Errorf("%w: WA_READER_DATABASE_URL (rebuild)", ErrMissingDatabaseURL)
	}

	reader, err := database.Open(ctx, cfg.ReaderDatabaseURL)
	if err != nil {
		return err //nolint:wrapcheck // already wrapped by database
	}
	defer reader.Close()

	app, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err //nolint:wrapcheck // already wrapped by database
	}
	defer app.Close()

	targets := rebuildTargets()
	if err := eventstore.Rebuild(ctx, reader, eventstore.NewStore(app), targets...); err != nil {
		return err //nolint:wrapcheck // already wrapped by eventstore
	}

	tel.Logger.InfoContext(ctx, "folds rebuilt", "aggregates", len(targets))

	return nil
}

// runBootstrap issues the bootstrap Invitation, or reissues it with a new
// code while it is unredeemed, and prints the code with fmt. The code is
// never logged, so it can't reach telemetry (ADR-0017).
func runBootstrap(ctx context.Context, cfg config.Config, tel *telemetry.Telemetry, opts Options) error {
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err //nolint:wrapcheck // already wrapped by database
	}
	defer pool.Close()

	code, expiresAt, err := NewUsers(pool).IssueBootstrapInvitation(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	tel.Logger.InfoContext(ctx, "bootstrap Invitation issued", "expires_at", expiresAt)

	out := opts.Stdout
	if out == nil {
		out = os.Stdout
	}

	_, err = fmt.Fprintf(out, "Bootstrap Invitation code (valid until %s, single use; any earlier code no longer works):\n\n    %s\n\n"+
		"Sign in, then redeem it with POST /registrations to become the Instance administrator.\n",
		expiresAt.UTC().Format(time.RFC3339), code)
	if err != nil {
		return fmt.Errorf("print bootstrap code: %w", err)
	}

	return nil
}

// apiHandler is the ogen handler: every context's HTTP adapter plus the
// problem+json error hooks.
type apiHandler struct {
	*householdshttp.Handler
	*householdshttp.NotificationHandler
	*ledgerHandler
	*ledgerhttp.AccountHandler
	*ingestionhttp.ConnectionHandler
	*ledgerhttp.TransactionHandler
	*ingestionhttp.StatementImportHandler
	httpapi.Errors
}

// ledgerHandler is the Ledger HTTP adapter, renamed so it can be embedded
// beside the Households one.
type ledgerHandler = ledgerhttp.Handler
