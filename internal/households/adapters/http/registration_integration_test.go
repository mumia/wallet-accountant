//go:build integration

package http_test

import (
	"fmt"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/apitest"
	"github.com/mumia/wallet-accountant/internal/households/domain"
)

type me struct {
	UserID                  string `json:"user_id"`
	IsInstanceAdministrator bool   `json:"is_instance_administrator"`
}

func TestRedeemingASeededInvitationRegistersTheCaller(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	code := app.SeedInvitation(t)
	token := app.TokenFor("https://issuer.example", "subject-1")

	res := app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": code})

	var registered me
	res.AssertJSON(t, http.StatusCreated, &registered)

	if registered.UserID == "" || registered.IsInstanceAdministrator {
		t.Errorf("registered = %+v, want a User who is not Instance administrator", registered)
	}

	var got me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &got)

	if got != registered {
		t.Errorf("GET /me = %+v, want %+v", got, registered)
	}
}

func TestEveryInvalidCodeGetsTheSameGenericError(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	expired := app.SeedInvitationWith(t, domain.RoleUser, time.Now().Add(-time.Minute))

	used := app.SeedInvitation(t)
	app.Post(t, "/api/registrations", app.TokenFor("https://issuer.example", "first"),
		map[string]string{"invitation_code": used}).AssertJSON(t, http.StatusCreated, &me{})

	superseded, err := app.RunBootstrap(t)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := app.RunBootstrap(t); err != nil {
		t.Fatal(err)
	}

	var details []apitest.ProblemBody

	for name, code := range map[string]string{
		"unknown":    "not-a-code-anyone-issued",
		"expired":    expired,
		"superseded": superseded,
		"used":       used,
	} {
		token := app.TokenFor("https://issuer.example", "caller-with-"+name+"-code")

		p := app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": code}).
			AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-invitation")
		details = append(details, p)

		app.Get(t, "/api/me", token).AssertProblem(t, http.StatusForbidden, "/problems/registration-required")
	}

	for _, p := range details[1:] {
		if p != details[0] {
			t.Errorf("problems differ, so codes can be probed: %+v vs %+v", p, details[0])
		}
	}
}

func TestAnEmptyCodeIsABadRequest(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.TokenFor("https://issuer.example", "subject-1")

	app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": ""}).
		AssertProblem(t, http.StatusBadRequest, "about:blank")
}

func TestRedeemingWithoutATokenIsUnauthorized(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	app.Post(t, "/api/registrations", "", map[string]string{"invitation_code": app.SeedInvitation(t)}).
		AssertProblem(t, http.StatusUnauthorized, "about:blank")
}

func TestARegisteredUserCannotRedeemASecondInvitation(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "subject-1")

	var before me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &before)

	second := app.SeedInvitation(t)
	app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": second}).
		AssertProblem(t, http.StatusConflict, "/problems/already-registered")

	var after me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &after)

	if after != before {
		t.Errorf("GET /me after the refused redemption = %+v, want %+v", after, before)
	}

	// The refused redemption left the second code unused.
	app.Post(t, "/api/registrations", app.TokenFor("https://issuer.example", "subject-2"),
		map[string]string{"invitation_code": second}).AssertJSON(t, http.StatusCreated, &me{})
}

func TestConcurrentRedemptionsOfOneCodeAdmitOneIdentity(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	code := app.SeedInvitation(t)

	const callers = 8

	statuses := make([]int, callers)
	tokens := make([]string, callers)

	for i := range callers {
		tokens[i] = app.TokenFor("https://issuer.example", fmt.Sprintf("racer-%d", i))
	}

	var wg sync.WaitGroup
	for i := range callers {
		wg.Go(func() {
			statuses[i] = app.Post(t, "/api/registrations", tokens[i], map[string]string{"invitation_code": code}).Status
		})
	}

	wg.Wait()

	registered := 0

	for i, status := range statuses {
		switch status {
		case http.StatusCreated:
			registered++

			app.Get(t, "/api/me", tokens[i]).AssertJSON(t, http.StatusOK, &me{})
		case http.StatusUnprocessableEntity:
			app.Get(t, "/api/me", tokens[i]).AssertProblem(t, http.StatusForbidden, "/problems/registration-required")
		default:
			t.Errorf("racer %d: status %d, want 201 or 422", i, status)
		}
	}

	if registered != 1 {
		t.Errorf("%d identities registered with one code, want 1 (statuses %v)", registered, statuses)
	}
}

func TestConcurrentRedemptionsOfTwoCodesByOneIdentityRegisterItOnce(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.TokenFor("https://issuer.example", "subject-1")
	codes := []string{app.SeedInvitation(t), app.SeedInvitation(t)}
	statuses := make([]int, len(codes))

	var wg sync.WaitGroup
	for i, code := range codes {
		wg.Go(func() {
			statuses[i] = app.Post(t, "/api/registrations", token, map[string]string{"invitation_code": code}).Status
		})
	}

	wg.Wait()

	slices.Sort(statuses)

	if want := []int{http.StatusCreated, http.StatusConflict}; !slices.Equal(statuses, want) {
		t.Errorf("statuses = %v, want %v", statuses, want)
	}
}

func TestUsersSurviveARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "subject-1")

	var before me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &before)

	app.Rebuild(t)

	var after me
	app.Get(t, "/api/me", token).AssertJSON(t, http.StatusOK, &after)

	if after != before {
		t.Errorf("GET /me after rebuild = %+v, want %+v", after, before)
	}
}
