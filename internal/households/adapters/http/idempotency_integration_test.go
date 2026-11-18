//go:build integration

package http_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// Idempotency-Key is honoured by every POST endpoint; these tests use the
// ones that exist so far, one storing a Household's data and one not.

func TestReplayingCreateHouseholdReturnsTheStoredResponse(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	body := map[string]string{"name": "Home", "base_currency": "EUR"}

	first := app.PostWithKey(t, "/api/households", token, "key-1", body)
	replay := app.PostWithKey(t, "/api/households", token, "key-1", body)

	var created, replayed household
	first.AssertJSON(t, http.StatusCreated, &created)
	replay.AssertJSON(t, http.StatusCreated, &replayed)

	if replayed != created {
		t.Errorf("replay = %+v, want the stored %+v", replayed, created)
	}

	var list householdList
	app.Get(t, "/api/households", token).AssertJSON(t, http.StatusOK, &list)

	if len(list.Items) != 1 {
		t.Errorf("Households after a replay: %+v, want only %+v", list.Items, created)
	}
}

func TestConcurrentRequestsWithOneKeyRunOnce(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	body := map[string]string{"name": "Home", "base_currency": "EUR"}

	const n = 8

	responses := make([]*apitest.Response, n)

	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { responses[i] = app.PostWithKey(t, "/api/households", token, "double-click", body) })
	}

	wg.Wait()

	var list householdList
	app.Get(t, "/api/households", token).AssertJSON(t, http.StatusOK, &list)

	if len(list.Items) != 1 {
		t.Fatalf("Households: %+v, want exactly one", list.Items)
	}

	for _, res := range responses {
		if res.Status == http.StatusConflict {
			res.AssertProblem(t, http.StatusConflict, "/problems/idempotency-key-in-use")

			continue
		}

		var got household
		res.AssertJSON(t, http.StatusCreated, &got)

		if got != list.Items[0] {
			t.Errorf("response %+v, want the one Household %+v", got, list.Items[0])
		}
	}
}

func TestTheSameKeyWithADifferentBodyIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")

	app.PostWithKey(t, "/api/households", token, "key-1", map[string]string{"name": "Home", "base_currency": "EUR"}).
		AssertJSON(t, http.StatusCreated, &household{})

	app.PostWithKey(t, "/api/households", token, "key-1", map[string]string{"name": "Other", "base_currency": "EUR"}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/idempotency-key-mismatch")

	var list householdList
	app.Get(t, "/api/households", token).AssertJSON(t, http.StatusOK, &list)

	if len(list.Items) != 1 || list.Items[0].Name != "Home" {
		t.Errorf("Households: %+v, want only Home", list.Items)
	}
}

func TestKeysBelongToTheirCaller(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := app.Register(t, "https://issuer.example", "alice")
	bob := app.Register(t, "https://issuer.example", "bob")
	body := map[string]string{"name": "Home", "base_currency": "EUR"}

	var alices, bobs household
	app.PostWithKey(t, "/api/households", alice, "shared-key", body).AssertJSON(t, http.StatusCreated, &alices)
	app.PostWithKey(t, "/api/households", bob, "shared-key", body).AssertJSON(t, http.StatusCreated, &bobs)

	if alices.HouseholdID == bobs.HouseholdID {
		t.Errorf("Bob got Alice's stored response %+v", alices)
	}
}

func TestARefusedRequestIsNotStored(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")

	app.PostWithKey(t, "/api/households", token, "key-1", map[string]string{"name": " ", "base_currency": "EUR"}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-household")

	// Corrected and sent again with the same key, it runs.
	app.PostWithKey(t, "/api/households", token, "key-1", map[string]string{"name": "Home", "base_currency": "EUR"}).
		AssertJSON(t, http.StatusCreated, &household{})
}

func TestReplayingARegistrationReturnsTheStoredUser(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.TokenFor("https://issuer.example", "alice")
	body := map[string]string{"invitation_code": app.SeedInvitation(t)}

	var registered, replayed me
	app.PostWithKey(t, "/api/registrations", token, "key-1", body).AssertJSON(t, http.StatusCreated, &registered)
	app.PostWithKey(t, "/api/registrations", token, "key-1", body).AssertJSON(t, http.StatusCreated, &replayed)

	if replayed != registered {
		t.Errorf("replay = %+v, want the stored %+v", replayed, registered)
	}

	// Without the key, the same request runs again and is refused.
	app.Post(t, "/api/registrations", token, body).AssertProblem(t, http.StatusConflict, "/problems/already-registered")
}
