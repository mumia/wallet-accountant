//go:build integration

package http_test

import (
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// The cash Accounts and Transfers contract (ticket 01) is wired before it is
// built: what isn't built yet answers 501 until its ticket lands. Each
// ticket deletes its case here.
func TestCashAndTransferOperationsAnswerNotImplementedYet(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(march10))
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")
	purse := addWallet(t, app, token, household, "10.00")

	// A cash ↔ cash Transfer (09).
	recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-01-05", "amount": eur("-10.00"), "description": "To the purse",
		"counterpart_account_id": purse.AccountID,
	}).AssertProblem(t, http.StatusNotImplemented, "/problems/not-implemented")
}
