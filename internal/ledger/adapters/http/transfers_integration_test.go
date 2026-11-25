//go:build integration

package http_test

import (
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// The Account transfer Internal Category is reached only through a
// Transfer: recording or recategorising into it is refused like any other
// Internal Category (spec "Ledger: Transfers").
func TestTheAccountTransferCategoryIsOnlyForTransfers(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(march10))
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")
	transfer := internalCategory(t, app, token, household, "Account transfer")

	recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-02-05", "amount": eur("-2.50"), "description": "Coffee", "category_id": transfer.CategoryID,
	}).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/internal-category")

	var coffee transaction
	recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-02-05", "amount": eur("-2.50"), "description": "Coffee",
	}).AssertJSON(t, http.StatusCreated, &coffee)

	app.Patch(t, "/api/households/"+household+"/transactions/"+coffee.TransactionID, token,
		map[string]any{"category_id": transfer.CategoryID}).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/internal-category")
}

// Until a Transfer is recorded, nothing is In transit and no Match exists:
// the filter, the month count and getMatch answer for an Account with only
// ordinary Transactions (spec story 19).
func TestNothingIsInTransitWithoutTransfers(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(march10))
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")
	base := "/api/households/" + household

	recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-01-05", "amount": eur("-2.50"), "description": "Coffee",
	}).AssertJSON(t, http.StatusCreated, &transaction{})

	var all, inTransit struct {
		Items []map[string]any `json:"items"`
	}

	app.Get(t, base+"/accounts/"+wallet.AccountID+"/transactions", token).AssertJSON(t, http.StatusOK, &all)

	if len(all.Items) != 2 {
		t.Fatalf("transactions = %+v, want the Opening balance and the coffee", all.Items)
	}

	for _, item := range all.Items {
		for _, field := range []string{"counterpart_account_id", "match_id", "created_by_transfer"} {
			if _, ok := item[field]; ok {
				t.Errorf("%v has %s, want none on an ordinary Transaction", item, field)
			}
		}
	}

	app.Get(t, base+"/accounts/"+wallet.AccountID+"/transactions?in_transit=true", token).
		AssertJSON(t, http.StatusOK, &inTransit)

	if len(inTransit.Items) != 0 {
		t.Errorf("in transit = %+v, want none", inTransit.Items)
	}

	var month struct {
		TransferLegsInTransit *int `json:"transfer_legs_in_transit"`
	}

	res := app.Get(t, base+"/accounts/"+wallet.AccountID+"/months/2026-01", token)
	res.AssertJSON(t, http.StatusOK, &month)

	if month.TransferLegsInTransit == nil || *month.TransferLegsInTransit != 0 {
		t.Errorf("month = %s, want transfer_legs_in_transit 0", res.Body)
	}

	app.Get(t, base+"/matches/0198f0a0-0000-7000-8000-000000000000", token).
		AssertProblem(t, http.StatusNotFound, "about:blank")
}
