// Package apitest is the HTTP test harness. It runs the real application
// in-process in the all run mode (API and worker), against a fresh database
// in PostgreSQL started with testcontainers-go, migrated as wa_owner and
// used as wa_app and wa_event_store_reader like production. Only the
// Authenticator is swapped for authntest, and telemetry is captured in memory.
package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	householdsapp "github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/authn/authntest"
	"github.com/mumia/wallet-accountant/internal/platform/config"
	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
	"github.com/mumia/wallet-accountant/internal/platform/forwarding"
	"github.com/mumia/wallet-accountant/internal/platform/telemetry"
	"github.com/mumia/wallet-accountant/internal/wiring"
)

// App is a running application.
type App struct {
	baseURL string
	client  *http.Client
	auth    *authntest.Authenticator
	reader  *pgxpool.Pool
	users   *householdsapp.Users
	cfg     config.Config
	tel     *telemetry.Telemetry
	// ownerURL connects as wa_owner, for the few tests that must write what
	// the application can't, such as events from before a change.
	ownerURL string

	// Logs holds every JSON log record written to stdout.
	Logs *Buffer
	// OTLPLogs holds every log record sent through the OpenTelemetry bridge.
	OTLPLogs *LogRecorder
	// Spans holds every finished span.
	Spans *tracetest.InMemoryExporter
	// Metrics collects the current metrics on demand.
	Metrics *sdkmetric.ManualReader
}

// Option changes how Start runs the application.
type Option func(*wiring.Options)

// WithClock makes now the clock the application's transactions read, such
// as whether a month has ended, so a test sets the date instead of
// depending on today's.
func WithClock(now func() time.Time) Option {
	return func(o *wiring.Options) { o.Now = now }
}

// Start runs the application for the duration of the test.
func Start(t testing.TB, opts ...Option) *App {
	t.Helper()

	app := &App{
		// Its own connections, so stopping the app can close them.
		client:   &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()},
		auth:     authntest.New(),
		Logs:     &Buffer{},
		OTLPLogs: &LogRecorder{},
		Spans:    tracetest.NewInMemoryExporter(),
		Metrics:  sdkmetric.NewManualReader(),
	}

	ctx, cancel := context.WithCancel(context.WithoutCancel(t.Context()))

	tel, err := telemetry.New(ctx, telemetry.Options{
		Stdout:       app.Logs,
		SpanExporter: app.Spans,
		MetricReader: app.Metrics,
		LogExporter:  app.OTLPLogs,
	})
	if err != nil {
		t.Fatalf("telemetry: %v", err)
	}

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	app.baseURL = "http://" + ln.Addr().String()

	urls := databasetest.New(t)
	cfg := config.Config{
		HTTPAddr:          ln.Addr().String(),
		DatabaseURL:       urls.App,
		ReaderDatabaseURL: urls.Reader,
		ShutdownTimeout:   5 * time.Second,
	}

	app.cfg, app.tel, app.ownerURL = cfg, tel, urls.Owner

	app.reader, err = pgxpool.New(ctx, urls.Reader)
	if err != nil {
		t.Fatalf("reader pool: %v", err)
	}

	appPool, err := pgxpool.New(ctx, urls.App)
	if err != nil {
		t.Fatalf("app pool: %v", err)
	}

	app.users = wiring.NewUsers(appPool, time.Now)

	done := make(chan error, 1)

	runOpts := wiring.Options{
		Authenticator: app.auth,
		Listener:      ln,
		PollInterval:  10 * time.Millisecond,
	}
	for _, o := range opts {
		o(&runOpts)
	}

	go func() {
		done <- wiring.Run(ctx, wiring.ModeAll, cfg, tel, runOpts)
	}()

	t.Cleanup(func() {
		// Parallel requests can leave the client a connection it dialled but
		// never used. The server waits 5s before treating such a connection
		// as idle, as long as its shutdown timeout, so close them first.
		app.client.CloseIdleConnections()
		cancel()
		app.reader.Close()
		appPool.Close()

		if err := <-done; err != nil {
			t.Errorf("app stopped with an error: %v", err)
		}

		_ = tel.Shutdown(context.Background())
	})

	return app
}

// Drain waits until every deferred event handler has handled every event
// recorded so far, so deferred effects can be asserted without sleeping.
func (a *App) Drain(t testing.TB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	if err := forwarding.Drain(ctx, a.reader, wiring.ConsumerGroups()...); err != nil {
		t.Fatalf("drain events: %v", err)
	}
}

// TokenFor returns a bearer token that authenticates as the OIDC identity.
func (a *App) TokenFor(issuer, subject string) string {
	return a.auth.Token(authn.Identity{Issuer: issuer, Subject: subject})
}

// Do sends a request; token is sent as a bearer token unless it is empty.
func (a *App) Do(t testing.TB, method, path, token string, body io.Reader) *Response {
	t.Helper()

	return a.DoWithHeader(t, method, path, token, nil, body)
}

// DoWithHeader sends a request with extra headers; token is sent as a
// bearer token unless it is empty.
func (a *App) DoWithHeader(t testing.TB, method, path, token string, header http.Header, body io.Reader) *Response {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, a.baseURL+path, body)
	if err != nil {
		t.Fatal(err)
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if body != nil && header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	for name, values := range header {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}

	res, err := a.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return &Response{Status: res.StatusCode, Header: res.Header, Body: raw}
}

// Get sends a GET request.
func (a *App) Get(t testing.TB, path, token string) *Response {
	t.Helper()

	return a.Do(t, http.MethodGet, path, token, nil)
}

// Response is a response read in full.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// ProblemBody is an application/problem+json body.
type ProblemBody struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

// AssertProblem checks that the response is application/problem+json with
// the given status and problem type, and returns the problem.
func (r *Response) AssertProblem(t testing.TB, status int, problemType string) ProblemBody {
	t.Helper()

	if r.Status != status {
		t.Fatalf("status = %d, want %d; body: %s", r.Status, status, r.Body)
	}

	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("Content-Type = %q, want application/problem+json; body: %s", ct, r.Body)
	}

	var p ProblemBody
	if err := json.Unmarshal(r.Body, &p); err != nil {
		t.Fatalf("problem body: %v; body: %s", err, r.Body)
	}

	if p.Status != status {
		t.Errorf("problem status = %d, want %d", p.Status, status)
	}

	if p.Type != problemType {
		t.Errorf("problem type = %q, want %q", p.Type, problemType)
	}

	if p.Title == "" {
		t.Error("problem has no title")
	}

	return p
}

// Buffer is a concurrency-safe log sink holding one JSON record per line.
type Buffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p) //nolint:wrapcheck // bytes.Buffer never fails
}

// Records decodes every JSON log record written so far.
func (b *Buffer) Records(t testing.TB) []map[string]any {
	t.Helper()

	b.mu.Lock()
	defer b.mu.Unlock()

	var records []map[string]any

	dec := json.NewDecoder(bytes.NewReader(b.buf.Bytes()))
	for {
		var rec map[string]any

		err := dec.Decode(&rec)
		if errors.Is(err, io.EOF) {
			return records
		}

		if err != nil {
			t.Fatalf("log output is not JSON: %v", err)
		}

		records = append(records, rec)
	}
}

// LogRecorder is an OpenTelemetry log exporter that keeps every record.
type LogRecorder struct {
	mu      sync.Mutex
	records []sdklog.Record
}

var _ sdklog.Exporter = (*LogRecorder)(nil)

// Export implements sdklog.Exporter.
func (l *LogRecorder) Export(_ context.Context, records []sdklog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, r := range records {
		l.records = append(l.records, r.Clone())
	}

	return nil
}

// Shutdown implements sdklog.Exporter.
func (*LogRecorder) Shutdown(context.Context) error { return nil }

// ForceFlush implements sdklog.Exporter.
func (*LogRecorder) ForceFlush(context.Context) error { return nil }

// Records returns every record exported so far.
func (l *LogRecorder) Records() []sdklog.Record {
	l.mu.Lock()
	defer l.mu.Unlock()

	return slices.Clone(l.records)
}

// EventCount counts every event in the event store, across Households, so
// a test can check that a request recorded nothing.
func (a *App) EventCount(t testing.TB) int {
	t.Helper()

	var n int
	if err := a.reader.QueryRow(t.Context(), "SELECT count(*) FROM event_store").Scan(&n); err != nil {
		t.Fatalf("count events: %v", err)
	}

	return n
}
