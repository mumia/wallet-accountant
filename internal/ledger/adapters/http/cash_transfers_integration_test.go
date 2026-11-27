//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// cashLeg is a Transaction that is a Transfer leg.
type cashLeg struct {
	transaction

	CounterpartAccountID string `json:"counterpart_account_id"`
	MatchID              string `json:"match_id"`
	CreatedByTransfer    bool   `json:"created_by_transfer"`
	ReversedBy           string `json:"reversed_by_transaction_id"`
}

func legsOn(t *testing.T, app *apitest.App, token, household, account string) []cashLeg {
	t.Helper()

	var page struct {
		Items []cashLeg `json:"items"`
	}

	app.Get(t, "/api/households/"+household+"/accounts/"+account+"/transactions", token).
		AssertJSON(t, http.StatusOK, &page)

	var legs []cashLeg

	for _, l := range page.Items {
		if l.CounterpartAccountID != "" {
			legs = append(legs, l)
		}
	}

	return legs
}

// Moving cash from the wallet to the purse records both legs and their
// Match in one command; reversing either leg reverses both, as neither can
// stand alone (spec story 18).
func TestMovingMoneyBetweenTwoCashAccounts(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(march10))
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")
	purse := addWallet(t, app, token, household, "10.00")

	var sent cashLeg
	recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-02-05", "amount": eur("-15.00"), "description": "To the purse", "payee": "Me",
		"counterpart_account_id": purse.AccountID,
	}).AssertJSON(t, http.StatusCreated, &sent)

	if sent.AccountID != wallet.AccountID || sent.Amount.Amount != "-15.00" || sent.CategoryName != "Account transfer" ||
		sent.CounterpartAccountID != purse.AccountID || sent.MatchID == "" || sent.CreatedByTransfer {
		t.Fatalf("recorded = %+v, want the wallet's -15.00 leg naming the purse, Matched", sent)
	}

	received := legsOn(t, app, token, household, purse.AccountID)
	if len(received) != 1 || received[0].Amount.Amount != "15.00" || received[0].BookingDate != "2026-02-05" ||
		received[0].Description != "To the purse" || received[0].CounterpartAccountID != wallet.AccountID ||
		received[0].MatchID != sent.MatchID {
		t.Fatalf("purse legs = %+v, want +15.00 on 2026-02-05 naming the wallet, with the same Match", received)
	}

	var m struct {
		Legs []struct {
			TransactionID string `json:"transaction_id"`
			AccountID     string `json:"account_id"`
		} `json:"legs"`
	}

	app.Get(t, "/api/households/"+household+"/matches/"+sent.MatchID, token).AssertJSON(t, http.StatusOK, &m)

	legs := map[string]string{}
	for _, l := range m.Legs {
		legs[l.TransactionID] = l.AccountID
	}

	if len(legs) != 2 || legs[sent.TransactionID] != wallet.AccountID ||
		legs[received[0].TransactionID] != purse.AccountID {
		t.Errorf("match legs = %+v, want the wallet's and the purse's", m.Legs)
	}

	if w, p := walletBalance(t, app, token, household, wallet.AccountID),
		walletBalance(t, app, token, household, purse.AccountID); w != "25.00" || p != "25.00" {
		t.Errorf("balances = %s and %s, want 25.00 each", w, p)
	}

	// Reversing the purse's leg reverses the wallet's too.
	app.Post(t, "/api/households/"+household+"/transactions/"+received[0].TransactionID+"/reverse", token, nil).
		AssertJSON(t, http.StatusCreated, &transaction{})

	for _, account := range []string{wallet.AccountID, purse.AccountID} {
		for _, l := range legsOn(t, app, token, household, account) {
			if l.ReversalOfTransactionID == "" && l.ReversedBy == "" {
				t.Errorf("leg %+v isn't reversed", l)
			}
		}
	}

	if w, p := walletBalance(t, app, token, household, wallet.AccountID),
		walletBalance(t, app, token, household, purse.AccountID); w != "40.00" || p != "10.00" {
		t.Errorf("balances = %s and %s, want 40.00 and 10.00 again", w, p)
	}

	app.Get(t, "/api/households/"+household+"/matches/"+sent.MatchID, token).
		AssertProblem(t, http.StatusNotFound, "about:blank")
}

// Both Accounts must take the date: a closed month of the counterpart
// refuses it, naming the counterpart, and nothing is recorded.
func TestMovingCashIntoAClosedMonthIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(march10))
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")
	purse := addWallet(t, app, token, household, "10.00")

	app.Post(t, "/api/households/"+household+"/accounts/"+purse.AccountID+"/months/2026-01/close", token,
		map[string]any{"statement_balance": eur("10.00"), "counted_on": "2026-02-01"}).
		AssertJSON(t, http.StatusOK, &map[string]any{})

	res := recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-01-20", "amount": eur("-15.00"), "description": "To the purse",
		"counterpart_account_id": purse.AccountID,
	})
	res.AssertProblem(t, http.StatusConflict, "/problems/month-closed")

	var p struct {
		AccountID    string `json:"account_id"`
		BookingMonth string `json:"booking_month"`
	}

	if err := json.Unmarshal(res.Body, &p); err != nil {
		t.Fatal(err)
	}

	if p.AccountID != purse.AccountID || p.BookingMonth != "2026-01" {
		t.Errorf("problem = %s, want January of the purse", res.Body)
	}

	if b := walletBalance(t, app, token, household, wallet.AccountID); b != "40.00" {
		t.Errorf("wallet balance = %s, want 40.00: nothing recorded", b)
	}

	if n := len(legsOn(t, app, token, household, wallet.AccountID)); n != 0 {
		t.Errorf("%d wallet legs, want none", n)
	}
}

// Moving cash on a date before the counterpart's start date is refused as
// a Transfer, naming the counterpart: it didn't exist yet. A date before
// the Account's own start date stays an invalid Transaction.
func TestMovingCashBeforeTheCounterpartsStartDateIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(march10))
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")

	var purse account
	app.Post(t, "/api/households/"+household+"/accounts", token, map[string]any{
		"name": "Purse", "kind": "CASH", "currency": "EUR", "start_date": "2026-02-01",
		"opening_count": eur("10.00"), "opening_count_confirmation": eur("10.00"),
	}).AssertJSON(t, http.StatusCreated, &purse)

	res := recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-01-20", "amount": eur("-15.00"), "description": "To the purse",
		"counterpart_account_id": purse.AccountID,
	})
	res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-transfer")

	var p struct {
		Reason    string `json:"reason"`
		AccountID string `json:"account_id"`
		Detail    string `json:"detail"`
	}

	if err := json.Unmarshal(res.Body, &p); err != nil {
		t.Fatal(err)
	}

	if p.Reason != "BEFORE_START_DATE" || p.AccountID != purse.AccountID ||
		!strings.Contains(p.Detail, "2026-01-20") || !strings.Contains(p.Detail, "2026-02-01") {
		t.Errorf("problem = %s, want BEFORE_START_DATE naming the purse and both dates", res.Body)
	}

	if b := walletBalance(t, app, token, household, wallet.AccountID); b != "40.00" {
		t.Errorf("wallet balance = %s, want 40.00: nothing recorded", b)
	}

	if b := walletBalance(t, app, token, household, purse.AccountID); b != "10.00" {
		t.Errorf("purse balance = %s, want 10.00: nothing recorded", b)
	}

	// From the purse, the same date is before its own start date.
	recordCash(t, app, token, household, purse.AccountID, map[string]any{
		"date": "2026-01-20", "amount": eur("-5.00"), "description": "To the wallet",
		"counterpart_account_id": wallet.AccountID,
	}).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-transaction")

	// On the purse's start date, both legs are recorded.
	recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-02-01", "amount": eur("-15.00"), "description": "To the purse",
		"counterpart_account_id": purse.AccountID,
	}).AssertJSON(t, http.StatusCreated, &cashLeg{})

	if b := walletBalance(t, app, token, household, purse.AccountID); b != "25.00" {
		t.Errorf("purse balance = %s, want 25.00", b)
	}
}
