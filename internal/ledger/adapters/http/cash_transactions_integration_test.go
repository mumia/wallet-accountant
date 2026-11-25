//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

type transaction struct {
	TransactionID           string `json:"transaction_id"`
	AccountID               string `json:"account_id"`
	Amount                  money  `json:"amount"`
	BookingDate             string `json:"booking_date"`
	TransactionDate         string `json:"transaction_date"`
	ValueDate               string `json:"value_date"`
	Description             string `json:"description"`
	Payee                   string `json:"payee"`
	CategoryID              string `json:"category_id"`
	CategoryName            string `json:"category_name"`
	ImportedTransactionID   string `json:"imported_transaction_id"`
	ReversalOfTransactionID string `json:"reversal_of_transaction_id"`
}

// march10 is "today" for the cash Transaction tests, so January and
// February have ended and March is a later month of a wallet started in
// January.
func march10() time.Time { return time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC) }

func recordCash(t *testing.T, app *apitest.App, token, household, account string, body map[string]any,
) *apitest.Response {
	t.Helper()

	return app.Post(t, "/api/households/"+household+"/accounts/"+account+"/transactions", token, body)
}

func walletBalance(t *testing.T, app *apitest.App, token, household, accountID string) string {
	t.Helper()

	var got account
	app.Get(t, "/api/households/"+household+"/accounts/"+accountID, token).AssertJSON(t, http.StatusOK, &got)

	if got.Balance == nil {
		t.Fatalf("account = %+v, want a balance", got)
	}

	return string(got.Balance.Amount)
}

// A member records cash payments by hand, in the Open month and any later
// one, Uncategorised or categorised, and corrects them with Set Category and
// Reverse; a closed month refuses both (spec stories 5–9).
func TestRecordingCashTransactions(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(march10))
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")
	food := create(t, app, token, household, map[string]any{"name": "Food", "kind": "EXPENSE"})

	var coffee transaction
	recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-01-20", "amount": eur("-2.50"), "description": "Coffee", "payee": "Café Central",
	}).AssertJSON(t, http.StatusCreated, &coffee)

	if coffee.AccountID != wallet.AccountID || coffee.Amount.Amount != "-2.50" || coffee.BookingDate != "2026-01-20" ||
		coffee.TransactionDate != "2026-01-20" || coffee.ValueDate != "" || coffee.Description != "Coffee" ||
		coffee.Payee != "Café Central" || coffee.CategoryName != "Uncategorised" || coffee.ImportedTransactionID != "" {
		t.Errorf("recorded = %+v, want -2.50 on 2026-01-20, Uncategorised, with its payee", coffee)
	}

	// A later month than the Open month (January) is fine for cash.
	var groceries transaction
	recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-03-03", "amount": eur("-10.00"), "description": "Groceries", "category_id": food.CategoryID,
	}).AssertJSON(t, http.StatusCreated, &groceries)

	if groceries.CategoryID != food.CategoryID || groceries.BookingDate != "2026-03-03" {
		t.Errorf("recorded = %+v, want Food on 2026-03-03", groceries)
	}

	if b := walletBalance(t, app, token, household, wallet.AccountID); b != "27.50" {
		t.Errorf("balance = %s, want 27.50", b)
	}

	if n := len(accountLines(t, app, token, household, wallet.AccountID)); n != 3 {
		t.Errorf("%d transactions, want the Opening balance and two cash Transactions", n)
	}

	// Categorise it later, then close January.
	var categorised transaction
	patch := app.Patch(t, "/api/households/"+household+"/transactions/"+coffee.TransactionID, token,
		map[string]any{"category_id": food.CategoryID})
	patch.AssertJSON(t, http.StatusOK, &categorised)

	if categorised.CategoryName != "Food" {
		t.Errorf("categorised = %+v, want Food", categorised)
	}

	app.Post(t, "/api/households/"+household+"/accounts/"+wallet.AccountID+"/months/2026-01/close", token,
		map[string]any{"statement_balance": eur("37.50"), "counted_on": "2026-02-01"}).
		AssertJSON(t, http.StatusOK, &map[string]any{})

	// January is closed: no new Transaction, no reversal there.
	late := recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-01-31", "amount": eur("-1.00"), "description": "Late coffee",
	})
	late.AssertProblem(t, http.StatusConflict, "/problems/month-closed")

	var closed struct {
		BookingMonth string `json:"booking_month"`
		AccountID    string `json:"account_id"`
	}

	if err := json.Unmarshal(late.Body, &closed); err != nil {
		t.Fatal(err)
	}

	if closed.BookingMonth != "2026-01" || closed.AccountID != wallet.AccountID {
		t.Errorf("month-closed = %+v, want January of the wallet", closed)
	}

	app.Post(t, "/api/households/"+household+"/transactions/"+coffee.TransactionID+"/reverse", token, nil).
		AssertProblem(t, http.StatusConflict, "/problems/month-closed")

	// A Transaction in a month after the Open month (February) reverses.
	var reversal transaction
	app.Post(t, "/api/households/"+household+"/transactions/"+groceries.TransactionID+"/reverse", token, nil).
		AssertJSON(t, http.StatusCreated, &reversal)

	if reversal.ReversalOfTransactionID != groceries.TransactionID || reversal.Amount.Amount != "10.00" ||
		reversal.BookingDate != "2026-03-03" {
		t.Errorf("reversal = %+v, want +10.00 on 2026-03-03 linked to the groceries", reversal)
	}

	if b := walletBalance(t, app, token, household, wallet.AccountID); b != "37.50" {
		t.Errorf("balance = %s, want 37.50", b)
	}
}

func TestRecordingACashTransactionIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(march10))
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")
	main := addAccount(t, app, token, household, map[string]string{
		"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
	})
	opening := internalCategory(t, app, token, household, "Opening balance")

	coffee := func(fields map[string]any) map[string]any {
		body := map[string]any{"date": "2026-02-05", "amount": eur("-2.50"), "description": "Coffee"}
		for k, v := range fields {
			body[k] = v
		}

		return body
	}

	for name, tc := range map[string]struct {
		account string
		body    map[string]any
		problem string
	}{
		"a bank Account":       {main.AccountID, coffee(nil), "/problems/not-a-cash-account"},
		"a zero amount":        {wallet.AccountID, coffee(map[string]any{"amount": eur("0")}), "/problems/invalid-transaction"},
		"another currency":     {wallet.AccountID, coffee(map[string]any{"amount": map[string]any{"amount": json.Number("-2.50"), "currency": "USD"}}), "/problems/invalid-transaction"},
		"too precise":          {wallet.AccountID, coffee(map[string]any{"amount": eur("-2.505")}), "/problems/invalid-transaction"},
		"after today":          {wallet.AccountID, coffee(map[string]any{"date": "2026-03-11"}), "/problems/invalid-transaction"},
		"before the start":     {wallet.AccountID, coffee(map[string]any{"date": "2025-12-31"}), "/problems/invalid-transaction"},
		"an Internal Category": {wallet.AccountID, coffee(map[string]any{"category_id": opening.CategoryID}), "/problems/internal-category"},
		"an unknown Category":  {wallet.AccountID, coffee(map[string]any{"category_id": "0198f0a0-0000-7000-8000-000000000000"}), "/problems/invalid-category"},
		"a Category and a counterpart": {wallet.AccountID, coffee(map[string]any{
			"category_id": opening.CategoryID, "counterpart_account_id": main.AccountID,
		}), "/problems/invalid-transfer"},
	} {
		t.Run(name, func(t *testing.T) {
			recordCash(t, app, token, household, tc.account, tc.body).
				AssertProblem(t, http.StatusUnprocessableEntity, tc.problem)
		})
	}

	var both struct {
		Reason string `json:"reason"`
	}

	if err := json.Unmarshal(recordCash(t, app, token, household, wallet.AccountID, coffee(map[string]any{
		"category_id": opening.CategoryID, "counterpart_account_id": main.AccountID,
	})).Body, &both); err != nil {
		t.Fatal(err)
	}

	if both.Reason != "CATEGORY_AND_TRANSFER" {
		t.Errorf("reason = %q, want CATEGORY_AND_TRANSFER", both.Reason)
	}

	recordCash(t, app, token, household, "0198f0a0-0000-7000-8000-000000000000", coffee(nil)).
		AssertProblem(t, http.StatusNotFound, "about:blank")

	if b := walletBalance(t, app, token, household, wallet.AccountID); b != "40.00" {
		t.Errorf("after refusals: balance = %s, want 40.00", b)
	}

	if n := len(accountLines(t, app, token, household, main.AccountID)); n != 0 {
		t.Errorf("the bank Account has %d transactions, want none", n)
	}
}
