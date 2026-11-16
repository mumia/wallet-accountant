package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
	"github.com/mumia/wallet-accountant/internal/platform/telemetry"
)

// NewHandler returns the HTTP handler of the API: the ogen server for h,
// with bearer tokens checked by auth, every error as problem+json,
// Idempotency-Key honoured on every POST (keys stored through pool, which
// connects as wa_app), and a log record, a trace and metrics for every
// request.
func NewHandler(h oas.Handler, auth authn.Authenticator, pool *pgxpool.Pool, tel *telemetry.Telemetry) (http.Handler, error) {
	errs := Errors{Logger: tel.Logger}

	srv, err := oas.NewServer(h, security{auth: auth},
		oas.WithErrorHandler(errs.HandleError),
		oas.WithNotFound(errs.NotFound),
		oas.WithMethodNotAllowed(errs.MethodNotAllowed),
		oas.WithTracerProvider(tel.TracerProvider),
		oas.WithMeterProvider(tel.MeterProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("ogen server: %w", err)
	}

	idem := &idempotency{pool: pool, auth: auth, logger: tel.Logger, next: srv}

	return otelhttp.NewHandler(logRequests(tel.Logger, idem), "http.server",
		otelhttp.WithTracerProvider(tel.TracerProvider),
		otelhttp.WithMeterProvider(tel.MeterProvider),
		otelhttp.WithPropagators(tel.Propagator),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	), nil
}

type security struct {
	auth authn.Authenticator
}

func (s security) HandleBearerAuth(ctx context.Context, _ oas.OperationName, t oas.BearerAuth) (context.Context, error) {
	id, err := s.auth.Authenticate(ctx, t.Token)
	if err != nil {
		return ctx, fmt.Errorf("authenticate: %w", err)
	}

	return authn.WithIdentity(ctx, id), nil
}

// logRequests writes one log record per request, in the request's trace.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		logger.InfoContext(r.Context(), "http request",
			slog.String("http.request.method", r.Method),
			slog.String("url.path", r.URL.Path),
			slog.Int("http.response.status_code", rec.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter

	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
