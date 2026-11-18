//go:build integration

package http_test

import (
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

func TestGetMeWithoutTokenIsUnauthorized(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	res := app.Get(t, "/api/me", "")

	res.AssertProblem(t, http.StatusUnauthorized, "about:blank")
}

func TestGetMeWithUnknownTokenIsUnauthorized(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	res := app.Get(t, "/api/me", "not-a-token-the-authenticator-knows")

	res.AssertProblem(t, http.StatusUnauthorized, "about:blank")
}

func TestGetMeForAnUnregisteredIdentityRequiresRegistration(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.TokenFor("https://issuer.example", "subject-1")

	res := app.Get(t, "/api/me", token)

	res.AssertProblem(t, http.StatusForbidden, "/problems/registration-required")
}
