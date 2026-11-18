//go:build integration

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

func TestListCurrenciesWithoutTokenIsUnauthorized(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	res := app.Get(t, "/currencies", "")

	res.AssertProblem(t, http.StatusUnauthorized, "about:blank")
}

// Registration is not needed: the list is no secret.
func TestListCurrenciesForAnUnregisteredIdentity(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.TokenFor("https://issuer.example", "subject-1")

	res := app.Get(t, "/currencies", token)

	if res.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", res.Status, res.Body)
	}

	var body struct {
		Items []struct {
			Code          string `json:"code"`
			DecimalPlaces int    `json:"decimal_places"`
		} `json:"items"`
	}
	if err := json.Unmarshal(res.Body, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	got := map[string]int{}

	for i, c := range body.Items {
		if i > 0 && body.Items[i-1].Code >= c.Code {
			t.Errorf("not sorted by code: %s before %s", body.Items[i-1].Code, c.Code)
		}

		got[c.Code] = c.DecimalPlaces
	}

	for code, want := range map[string]int{"EUR": 2, "JPY": 0, "BTC": 8, "ETH": 18} {
		if d, ok := got[code]; !ok || d != want {
			t.Errorf("%s: decimal_places = %d (listed %t), want %d", code, d, ok, want)
		}
	}
}
