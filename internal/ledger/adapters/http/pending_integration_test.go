//go:build integration

package http_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// The cash Accounts and Transfers contract (ticket 01) is wired before it is
// built: every new operation, and every new field of an existing one,
// answers 501 until its ticket lands. Each ticket deletes its case here.
func TestCashAndTransferOperationsAnswerNotImplementedYet(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	base := "/api/households/" + household
	main := addAccount(t, app, token, household, map[string]string{
		"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
	})
	eur := map[string]any{"amount": 10, "currency": "EUR"}
	anID := uuid.NewString()

	for name, send := range map[string]func() *apitest.Response{
		"cash Transfer leg (07, 09)": func() *apitest.Response {
			return app.Post(t, base+"/accounts/"+main.AccountID+"/transactions", token, map[string]any{
				"date": "2026-01-05", "amount": eur, "description": "ATM", "counterpart_account_id": anID,
			})
		},
		"getMatch (05)": func() *apitest.Response {
			return app.Get(t, base+"/matches/"+anID, token)
		},
		"in_transit list (05)": func() *apitest.Response {
			return app.Get(t, base+"/accounts/"+main.AccountID+"/transactions?in_transit=true", token)
		},
		"Accept as Transfer (06)": func() *apitest.Response {
			return app.Post(t, base+"/imported-transactions/"+anID+"/accept", token, map[string]any{
				"transfer_account_id": anID,
			})
		},
		"Accept matching a leg (07)": func() *apitest.Response {
			return app.Post(t, base+"/imported-transactions/"+anID+"/accept", token, map[string]any{
				"transfer_account_id": anID, "match_transaction_id": anID,
			})
		},
		"listTransferSuggestions (07)": func() *apitest.Response {
			return app.Get(t, base+"/imported-transactions/"+anID+"/transfer-suggestions?account_id="+anID, token)
		},
	} {
		t.Run(name, func(t *testing.T) {
			send().AssertProblem(t, http.StatusNotImplemented, "/problems/not-implemented")
		})
	}
}
