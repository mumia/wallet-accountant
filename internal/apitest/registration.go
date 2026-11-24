package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	householdsapp "github.com/mumia/wallet-accountant/internal/households/app"
	"github.com/mumia/wallet-accountant/internal/households/domain"
	"github.com/mumia/wallet-accountant/internal/wiring"
)

// SeedInvitation issues an Invitation that grants no instance role and is
// valid for 24 hours, and returns its code. Tests use it to register several
// identities, since Invitations can't be issued through the API yet.
func (a *App) SeedInvitation(t testing.TB) string {
	t.Helper()

	return a.SeedInvitationWith(t, domain.RoleUser, time.Now().Add(domain.CodeLifetime))
}

// SeedInvitationWith issues an Invitation granting role that expires at
// expiresAt (which may be in the past), and returns its code.
func (a *App) SeedInvitationWith(t testing.TB, role domain.Role, expiresAt time.Time) string {
	t.Helper()

	code, err := a.users.IssueInvitation(t.Context(), role, expiresAt)
	if err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	return code
}

// Register makes the OIDC identity a User by redeeming a seeded Invitation,
// and returns a bearer token for it.
func (a *App) Register(t testing.TB, issuer, subject string) string {
	t.Helper()

	token := a.TokenFor(issuer, subject)
	res := a.Post(t, "/api/registrations", token, map[string]string{"invitation_code": a.SeedInvitation(t), "display_name": "Ana"})

	if res.Status != http.StatusCreated {
		t.Fatalf("register %s %s: status %d; body: %s", issuer, subject, res.Status, res.Body)
	}

	return token
}

// RegisterWithoutDisplayName makes the OIDC identity a User as one registered
// before display names existed: its user.registered event has no
// displayname and its User has no name. It returns a bearer token for it.
func (a *App) RegisterWithoutDisplayName(t testing.TB, issuer, subject string) string {
	t.Helper()

	token := a.Register(t, issuer, subject)

	conn, err := pgx.Connect(t.Context(), a.ownerURL)
	if err != nil {
		t.Fatalf("connect as owner: %v", err)
	}
	defer conn.Close(context.WithoutCancel(t.Context()))

	// Only the owner may change an event.
	_, err = conn.Exec(t.Context(), `WITH u AS (
		    UPDATE users SET display_name = NULL
		    WHERE id = (SELECT user_id FROM user_identities WHERE oidc_issuer = $1 AND oidc_subject = $2)
		    RETURNING id)
		UPDATE event_store SET data = data - 'displayname'
		WHERE subject = (SELECT id FROM u) AND type = $3`,
		issuer, subject, householdsapp.UserRegisteredType)
	if err != nil {
		t.Fatalf("forget display name: %v", err)
	}

	return token
}

// RunBootstrap runs the bootstrap run mode against the app's database, with
// the app's telemetry, and returns the code it printed.
func (a *App) RunBootstrap(t testing.TB) (string, error) {
	t.Helper()

	var out bytes.Buffer

	err := wiring.Run(t.Context(), wiring.ModeBootstrap, a.cfg, a.tel, wiring.Options{Stdout: &out})
	if err != nil {
		return "", err //nolint:wrapcheck // the run mode's own error
	}

	// The code is the only indented line of the output.
	for line := range strings.Lines(out.String()) {
		if strings.HasPrefix(line, "    ") {
			return strings.TrimSpace(line), nil
		}
	}

	t.Fatalf("bootstrap printed no code: %q", out.String())

	return "", nil
}

// Rebuild runs the rebuild run mode against the app's database.
func (a *App) Rebuild(t testing.TB) {
	t.Helper()

	if err := wiring.Run(t.Context(), wiring.ModeRebuild, a.cfg, a.tel, wiring.Options{}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
}

// Post sends body as JSON in a POST request.
func (a *App) Post(t testing.TB, path, token string, body any) *Response {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	return a.Do(t, http.MethodPost, path, token, bytes.NewReader(raw))
}

// PostWithKey sends body as JSON in a POST request with the Idempotency-Key
// header set to key.
func (a *App) PostWithKey(t testing.TB, path, token, key string, body any) *Response {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	return a.DoWithHeader(t, http.MethodPost, path, token, http.Header{"Idempotency-Key": {key}}, bytes.NewReader(raw))
}

// AssertJSON checks that the response is application/json with the given
// status, and decodes its body into v.
func (r *Response) AssertJSON(t testing.TB, status int, v any) {
	t.Helper()

	if r.Status != status {
		t.Fatalf("status = %d, want %d; body: %s", r.Status, status, r.Body)
	}

	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json; body: %s", ct, r.Body)
	}

	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode body: %v; body: %s", err, r.Body)
	}
}
