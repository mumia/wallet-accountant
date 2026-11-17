//go:build integration

package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mumia/wallet-accountant/internal/platform/authn"
	"github.com/mumia/wallet-accountant/internal/platform/authn/authntest"
	"github.com/mumia/wallet-accountant/internal/platform/database/databasetest"
)

// keyed is the middleware in front of next, with the caller's token.
type keyed struct {
	idem  *idempotency
	token string
}

func newKeyed(t *testing.T, abandonAfter time.Duration, next http.Handler) keyed {
	t.Helper()

	pool, err := pgxpool.New(t.Context(), databasetest.New(t).App)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(pool.Close)

	auth := authntest.New()

	return keyed{
		idem: &idempotency{
			pool: pool, auth: auth, logger: slog.New(slog.DiscardHandler), next: next, abandonAfter: abandonAfter,
		},
		token: auth.Token(authn.Identity{Issuer: "https://issuer.example", Subject: "alice"}),
	}
}

// post sends the same POST with the key, returning the response, or what
// the middleware panicked with.
func (k keyed) post(t *testing.T) (res *httptest.ResponseRecorder, panicked any) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/things", strings.NewReader(`{"a":1}`))
	req.Header.Set("Authorization", "Bearer "+k.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(IdempotencyKeyHeader, "key-1")

	res = httptest.NewRecorder()

	defer func() { panicked = recover() }()

	k.idem.ServeHTTP(res, req)

	return res, nil
}

// created answers 201 with how many times it ran.
func created(runs *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		n := runs.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"run":`+string(rune('0'+n))+`}`)
	}
}

// A request whose handler panics releases its key on the way out, so a
// retry runs instead of getting 409 forever. The panic still goes on.
func TestAPanickingRequestReleasesItsKey(t *testing.T) {
	t.Parallel()

	var runs atomic.Int32

	k := newKeyed(t, time.Hour, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if runs.Load() == 0 {
			runs.Add(1)
			panic("handler bug")
		}

		created(&runs)(w, r)
	}))

	if _, p := k.post(t); p != "handler bug" {
		t.Fatalf("first request panicked with %v, want the handler's panic passed on", p)
	}

	res, p := k.post(t)
	if p != nil || res.Code != http.StatusCreated || res.Body.String() != `{"run":2}` {
		t.Fatalf("retry = %d %s (panic %v), want it to run: 201", res.Code, res.Body, p)
	}
}

// A request that never finished, as when the process crashed, leaves its
// key reserved. A retry gets 409 while it may still be running, and takes
// the key over once the reservation is older than the timeout. The
// abandoned request can't then overwrite the retry's stored response.
func TestAnAbandonedReservationIsTakenOver(t *testing.T) {
	t.Parallel()

	var runs atomic.Int32

	stuck, release := make(chan struct{}), make(chan struct{})

	k := newKeyed(t, 300*time.Millisecond, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if runs.Load() == 0 {
			runs.Add(1)
			close(stuck)
			<-release
		}

		created(&runs)(w, r)
	}))

	abandoned := make(chan *httptest.ResponseRecorder, 1)

	go func() {
		res, _ := k.post(t)
		abandoned <- res
	}()

	<-stuck

	if res, _ := k.post(t); res.Code != http.StatusConflict {
		t.Fatalf("retry while running = %d %s, want 409", res.Code, res.Body)
	}

	time.Sleep(400 * time.Millisecond)

	res, _ := k.post(t)
	if res.Code != http.StatusCreated || res.Body.String() != `{"run":2}` {
		t.Fatalf("retry after the timeout = %d %s, want it to run: 201", res.Code, res.Body)
	}

	close(release)
	<-abandoned

	if res, _ := k.post(t); res.Code != http.StatusCreated || res.Body.String() != `{"run":2}` {
		t.Errorf("replay = %d %s, want the retry's stored response", res.Code, res.Body)
	}
}
