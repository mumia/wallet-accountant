package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mumia/wallet-accountant/internal/platform/authn"
)

// Problem types of Idempotency-Key.
const (
	// IdempotencyKeyMismatchType is for a key sent again with another request.
	IdempotencyKeyMismatchType = "/problems/idempotency-key-mismatch"
	// IdempotencyKeyInUseType is for a key whose first request hasn't finished.
	IdempotencyKeyInUseType = "/problems/idempotency-key-in-use"
)

// IdempotencyKeyHeader is the header a client names a POST request with.
const IdempotencyKeyHeader = "Idempotency-Key"

const (
	maxIdempotencyKey  = 255
	maxIdempotentBody  = 64 << 20
	statusSuccessFirst = 200
	statusSuccessLast  = 299

	// defaultAbandonAfter is how long a reservation is kept for a request
	// that hasn't finished. A request runs for seconds; one still reserved
	// after this was abandoned, as when the process stopped while running
	// it, and a retry takes the key over.
	defaultAbandonAfter = 10 * time.Minute
)

// householdPath matches the path of a Household-scoped route, under
// PathPrefix.
var householdPath = regexp.MustCompile(`^` + regexp.QuoteMeta(PathPrefix) + `/households/([0-9a-fA-F-]{36})(?:/|$)`)

type responseScopeKey struct{}

// responseScope is where a successful response's data belongs.
type responseScope struct {
	household uuid.UUID
}

// ResponseBelongsTo records that the response to the request of ctx holds
// the Household's data, for a request whose path doesn't name it, such as
// creating a Household. A replay of the response is then stored in that
// Household's scope (ADR-0014).
func ResponseBelongsTo(ctx context.Context, household uuid.UUID) {
	if s, ok := ctx.Value(responseScopeKey{}).(*responseScope); ok {
		s.household = household
	}
}

// idempotency makes every POST request with an Idempotency-Key header safe to
// retry. Keys belong to the caller's OIDC identity. The first request
// reserves its key before it runs, so a concurrent retry gets 409 instead of
// running it twice. A successful (2xx) response is stored and returned to
// every replay with the same body; the same key with another body gets 422.
// Any other response releases the key, so the request can be corrected and
// sent again. A response holding a Household's data is stored in that
// Household's scope: the one in the path, or the one the handler names with
// ResponseBelongsTo. A request that panics releases its key too, and a
// reservation older than abandonAfter is abandoned: a retry takes it over.
type idempotency struct {
	pool         *pgxpool.Pool
	auth         authn.Authenticator
	logger       *slog.Logger
	next         http.Handler
	abandonAfter time.Duration
}

func (i *idempotency) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get(IdempotencyKeyHeader)
	if r.Method != http.MethodPost || key == "" || len(key) > maxIdempotencyKey {
		i.next.ServeHTTP(w, r) // ogen validates the header

		return
	}

	ctx := r.Context()

	id, ok := i.authenticate(ctx, r)
	if !ok {
		i.next.ServeHTTP(w, r) // ogen answers 401

		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxIdempotentBody))
	if err != nil {
		WriteProblem(w, Problem(http.StatusRequestEntityTooLarge, "", "", ""))

		return
	}

	r.Body = io.NopCloser(bytes.NewReader(body))

	keyID := hashOf(id.Issuer, id.Subject, key)
	requestHash := hashOf(r.Method, r.URL.Path, r.URL.RawQuery, bodyIdentity(r.Header.Get("Content-Type"), body))

	stored, err := i.reserve(ctx, keyID, requestHash)
	if err != nil {
		i.logger.ErrorContext(ctx, "reserve idempotency key", "error", err)
		WriteProblem(w, Problem(http.StatusInternalServerError, "", "", ""))

		return
	}

	switch stored.state {
	case keyMismatch:
		WriteProblem(w, Problem(http.StatusUnprocessableEntity, IdempotencyKeyMismatchType, "Idempotency-Key reused",
			"This Idempotency-Key was already used for another request."))

		return
	case keyInUse:
		WriteProblem(w, Problem(http.StatusConflict, IdempotencyKeyInUseType, "Idempotency-Key in use",
			"The first request with this Idempotency-Key hasn't finished."))

		return
	case keyCompleted:
		stored.response.writeTo(w)

		return
	case keyReserved:
	}

	scope := &responseScope{}
	if m := householdPath.FindStringSubmatch(r.URL.Path); m != nil {
		scope.household, _ = uuid.Parse(m[1]) // uuid.Nil if malformed: ogen refuses it
	}

	// The outcome is recorded even if the client has gone away.
	done := context.WithoutCancel(ctx)

	rec := &recorder{header: http.Header{}, status: http.StatusOK}
	i.serveReserved(done, keyID, stored.reservedAt, rec, r.WithContext(context.WithValue(ctx, responseScopeKey{}, scope)))

	if rec.status >= statusSuccessFirst && rec.status <= statusSuccessLast {
		if err := i.complete(done, keyID, stored.reservedAt, scope.household, rec.response()); err != nil {
			// The key stays reserved: a retry gets 409, never a second run.
			i.logger.ErrorContext(ctx, "store idempotent response", "error", err)
		}
	} else if err := i.release(done, keyID, stored.reservedAt); err != nil {
		i.logger.ErrorContext(ctx, "release idempotency key", "error", err)
	}

	for name, values := range rec.header {
		w.Header()[name] = values
	}

	w.WriteHeader(rec.status)
	_, _ = w.Write(rec.body.Bytes())
}

// serveReserved runs the request that reserved the key. If it panics, the
// key is released before the panic goes on, so a retry isn't refused with
// 409 for good.
func (i *idempotency) serveReserved(done context.Context, keyID string, reservedAt time.Time, w http.ResponseWriter,
	r *http.Request,
) {
	defer func() {
		if p := recover(); p != nil {
			if err := i.release(done, keyID, reservedAt); err != nil {
				i.logger.ErrorContext(done, "release idempotency key", "error", err)
			}

			panic(p)
		}
	}()

	i.next.ServeHTTP(w, r)
}

func (i *idempotency) authenticate(ctx context.Context, r *http.Request) (authn.Identity, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return authn.Identity{}, false
	}

	id, err := i.auth.Authenticate(ctx, token)
	if err != nil {
		return authn.Identity{}, false
	}

	return id, true
}

type keyState int

const (
	keyReserved keyState = iota
	keyMismatch
	keyInUse
	keyCompleted
)

type storedKey struct {
	state    keyState
	response storedResponse
	// reservedAt identifies the reservation of a keyReserved key: only the
	// request holding it completes or releases the key, never one whose
	// abandoned reservation was taken over.
	reservedAt time.Time
}

// reserve reserves the key for a request, or reports what an earlier
// request with the key left: a stored response, or that it is still running.
// A reservation for the same request older than abandonAfter is taken over.
func (i *idempotency) reserve(ctx context.Context, keyID, requestHash string) (storedKey, error) {
	var stored storedKey

	err := i.inScope(ctx, uuid.Nil, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO idempotency_keys (key_id, request_hash, created_at)
			VALUES ($1, $2, clock_timestamp())
			ON CONFLICT (key_id) DO NOTHING RETURNING created_at`, keyID, requestHash).Scan(&stored.reservedAt)
		if err == nil {
			stored.state = keyReserved

			return nil
		}

		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("reserve: %w", err)
		}

		var (
			hash      string
			completed bool
			household *uuid.UUID
			status    *int
			res       storedResponse
		)

		err = tx.QueryRow(ctx, `SELECT request_hash, completed_at IS NOT NULL, household_id, status,
			COALESCE(content_type, ''), COALESCE(location, ''), COALESCE(body, ''::bytea)
			FROM idempotency_keys WHERE key_id = $1`, keyID).
			Scan(&hash, &completed, &household, &status, &res.contentType, &res.location, &res.body)
		if errors.Is(err, pgx.ErrNoRows) {
			return errKeyReleased
		}

		if err != nil {
			return fmt.Errorf("read key: %w", err)
		}

		switch {
		case hash != requestHash:
			stored.state = keyMismatch
		case !completed:
			stored.state, stored.reservedAt, err = i.takeOver(ctx, tx, keyID)
		case household != nil:
			stored.state = keyCompleted
			stored.response, err = householdResponse(ctx, tx, *household, keyID)
		default:
			stored.state = keyCompleted
			res.status = derefStatus(status)
			stored.response = res
		}

		return err
	})
	if errors.Is(err, errKeyReleased) {
		// Released between the insert and the read: reserve it afresh.
		return i.reserve(ctx, keyID, requestHash)
	}

	return stored, err
}

var errKeyReleased = errors.New("idempotency key released while reading it")

// errReservationTakenOver is returned for completing or releasing a key
// whose reservation a retry took over after it was abandoned.
var errReservationTakenOver = errors.New("idempotency key reservation taken over")

// takeOver reserves afresh a key whose request hasn't finished, if its
// reservation is older than abandonAfter; otherwise the key is in use.
func (i *idempotency) takeOver(ctx context.Context, tx pgx.Tx, keyID string) (keyState, time.Time, error) {
	var reservedAt time.Time

	err := tx.QueryRow(ctx, `UPDATE idempotency_keys SET created_at = clock_timestamp()
		WHERE key_id = $1 AND completed_at IS NULL AND created_at < clock_timestamp() - make_interval(secs => $2)
		RETURNING created_at`, keyID, i.abandonAfter.Seconds()).Scan(&reservedAt)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return keyInUse, time.Time{}, nil
	case err != nil:
		return keyInUse, time.Time{}, fmt.Errorf("take over key: %w", err)
	}

	return keyReserved, reservedAt, nil
}

func householdResponse(ctx context.Context, tx pgx.Tx, household uuid.UUID, keyID string) (storedResponse, error) {
	if err := setScope(ctx, tx, household); err != nil {
		return storedResponse{}, err
	}

	var res storedResponse

	err := tx.QueryRow(ctx, `SELECT status, COALESCE(content_type, ''), COALESCE(location, ''), body
		FROM idempotency_responses WHERE household_id = $1 AND key_id = $2`, household, keyID).
		Scan(&res.status, &res.contentType, &res.location, &res.body)
	if err != nil {
		return storedResponse{}, fmt.Errorf("read stored response: %w", err)
	}

	return res, nil
}

// complete stores the response of the request that reserved the key at
// reservedAt; nothing is stored if its reservation was taken over.
func (i *idempotency) complete(ctx context.Context, keyID string, reservedAt time.Time, household uuid.UUID,
	res storedResponse,
) error {
	return i.inScope(ctx, household, func(tx pgx.Tx) error {
		if household == uuid.Nil {
			tag, err := tx.Exec(ctx, `UPDATE idempotency_keys
				SET completed_at = now(), status = $3, content_type = NULLIF($4, ''), location = NULLIF($5, ''), body = $6
				WHERE key_id = $1 AND created_at = $2 AND completed_at IS NULL`,
				keyID, reservedAt, res.status, res.contentType, res.location, res.body)
			if err != nil {
				return fmt.Errorf("store response: %w", err)
			}

			if tag.RowsAffected() == 0 {
				return errReservationTakenOver
			}

			return nil
		}

		if _, err := tx.Exec(ctx, `INSERT INTO idempotency_responses (household_id, key_id, status, content_type, location, body)
			VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6)`,
			household, keyID, res.status, res.contentType, res.location, res.body); err != nil {
			return fmt.Errorf("store response: %w", err)
		}

		if err := setScope(ctx, tx, uuid.Nil); err != nil {
			return err
		}

		tag, err := tx.Exec(ctx, `UPDATE idempotency_keys SET completed_at = now(), household_id = $3
			WHERE key_id = $1 AND created_at = $2 AND completed_at IS NULL`, keyID, reservedAt, household)
		if err != nil {
			return fmt.Errorf("complete key: %w", err)
		}

		if tag.RowsAffected() == 0 {
			// Rolls back the stored response too.
			return errReservationTakenOver
		}

		return nil
	})
}

// release frees the key of a request that didn't succeed, unless a retry
// took its reservation over.
func (i *idempotency) release(ctx context.Context, keyID string, reservedAt time.Time) error {
	return i.inScope(ctx, uuid.Nil, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM idempotency_keys WHERE key_id = $1 AND created_at = $2 AND completed_at IS NULL`,
			keyID, reservedAt); err != nil {
			return fmt.Errorf("release: %w", err)
		}

		return nil
	})
}

// inScope runs fn in a committed transaction as wa_app, with app.household_id
// set to household (instance scope for uuid.Nil).
func (i *idempotency) inScope(ctx context.Context, household uuid.UUID, fn func(pgx.Tx) error) error {
	err := pgx.BeginFunc(ctx, i.pool, func(tx pgx.Tx) error {
		if err := setScope(ctx, tx, household); err != nil {
			return err
		}

		return fn(tx)
	})
	if err != nil {
		return fmt.Errorf("idempotency key: %w", err)
	}

	return nil
}

func setScope(ctx context.Context, tx pgx.Tx, household uuid.UUID) error {
	setting := ""
	if household != uuid.Nil {
		setting = household.String()
	}

	if _, err := tx.Exec(ctx, `SELECT set_config('app.household_id', $1, true)`, setting); err != nil {
		return fmt.Errorf("set household: %w", err)
	}

	return nil
}

func derefStatus(s *int) int {
	if s == nil {
		return http.StatusOK
	}

	return *s
}

// bodyIdentity is what makes two request bodies the same request. A
// multipart form is compared by its parts (each part's name, file name,
// content type and content, in any order), not its bytes: its boundary is
// random, so a client that rebuilds the form to retry an upload sends other
// bytes for the same request. Any other body is compared byte for byte.
func bodyIdentity(contentType string, body []byte) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		return string(body)
	}

	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])

	var parts []string

	for {
		part, err := reader.NextRawPart()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return string(body) // not a valid form: ogen refuses it
		}

		content, err := io.ReadAll(part)
		if err != nil {
			return string(body)
		}

		parts = append(parts, hashOf(part.FormName(), part.FileName(), part.Header.Get("Content-Type"), string(content)))
	}

	slices.Sort(parts)

	return "multipart/form-data:" + strings.Join(parts, ",")
}

// hashOf hashes the parts, separated so that no two lists of parts collide.
func hashOf(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = fmt.Fprintf(h, "%d:%s", len(p), p)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// storedResponse is a response as it is stored and replayed.
type storedResponse struct {
	status      int
	contentType string
	location    string
	body        []byte
}

func (s storedResponse) writeTo(w http.ResponseWriter) {
	if s.contentType != "" {
		w.Header().Set("Content-Type", s.contentType)
	}

	if s.location != "" {
		w.Header().Set("Location", s.location)
	}

	w.WriteHeader(s.status)
	_, _ = w.Write(s.body)
}

// recorder holds a response until its outcome is recorded.
type recorder struct {
	header http.Header
	status int
	wrote  bool
	body   bytes.Buffer
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(status int) {
	if !r.wrote {
		r.status, r.wrote = status, true
	}
}

func (r *recorder) Write(p []byte) (int, error) {
	r.wrote = true

	return r.body.Write(p) //nolint:wrapcheck // bytes.Buffer never fails
}

func (r *recorder) response() storedResponse {
	return storedResponse{
		status:      r.status,
		contentType: r.header.Get("Content-Type"),
		location:    r.header.Get("Location"),
		body:        r.body.Bytes(),
	}
}
