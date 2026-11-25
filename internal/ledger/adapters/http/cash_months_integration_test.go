//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// cashMonth is a cash Account's month status.
type cashMonth struct {
	BookingMonth     string `json:"booking_month"`
	State            string `json:"state"`
	StatementBalance *money `json:"statement_balance"`
	ComputedBalance  *money `json:"computed_balance"`
	CountedOn        string `json:"counted_on"`
	ReopenBlocked    bool   `json:"reopen_blocked"`
}

// cashCloseRefusal is a refused Month close's balance mismatch.
type cashCloseRefusal struct {
	Reasons          []string `json:"reasons"`
	ComputedBalance  *money   `json:"computed_balance"`
	StatementBalance *money   `json:"statement_balance"`
	Difference       *money   `json:"difference"`
}

func closeByCount(t *testing.T, app *apitest.App, token, household, account, month string, body map[string]any,
) *apitest.Response {
	t.Helper()

	return app.Post(t, "/api/households/"+household+"/accounts/"+account+"/months/"+month+"/close", token, body)
}

// A cash month closes by a count taken after it ends. A short count is
// refused with the gap; the member records the loss against an Expense
// Category, and the close then passes, storing the month-end balance the
// count proves. Reopening it leaves February's Transactions alone (spec
// stories 11–13).
func TestClosingACashMonthByACount(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(march10))
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")
	food := create(t, app, token, household, map[string]any{"name": "Food", "kind": "EXPENSE"})
	losses := create(t, app, token, household, map[string]any{"name": "Losses", "kind": "EXPENSE"})

	for _, tx := range []map[string]any{
		{"date": "2026-01-20", "amount": eur("-2.50"), "description": "Coffee"},
		{"date": "2026-02-02", "amount": eur("-5.00"), "description": "Lunch"},
		{"date": "2026-02-05", "amount": eur("-1.00"), "description": "Newspaper"},
	} {
		tx["category_id"] = food.CategoryID
		recordCash(t, app, token, household, wallet.AccountID, tx).AssertJSON(t, http.StatusCreated, &transaction{})
	}

	// Counted on the 3rd of February: 30.00, but the books say 32.50 (40.00
	// - 2.50 in January - 5.00 for lunch; the newspaper came later).
	short := closeByCount(t, app, token, household, wallet.AccountID, "2026-01", map[string]any{
		"statement_balance": eur("30.00"), "counted_on": "2026-02-03",
	})
	short.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/month-close-refused")

	var gap cashCloseRefusal
	if err := json.Unmarshal(short.Body, &gap); err != nil {
		t.Fatal(err)
	}

	if len(gap.Reasons) != 1 || gap.Reasons[0] != "BALANCE_MISMATCH" || gap.ComputedBalance == nil ||
		gap.ComputedBalance.Amount != "32.50" || gap.StatementBalance.Amount != "30.00" ||
		gap.Difference.Amount != "-2.50" {
		t.Errorf("refusal = %s, want only a mismatch: computed 32.50, counted 30.00, difference -2.50", short.Body)
	}

	// The gap is a lost 2.50 in January.
	recordCash(t, app, token, household, wallet.AccountID, map[string]any{
		"date": "2026-01-28", "amount": eur("-2.50"), "description": "Lost", "category_id": losses.CategoryID,
	}).AssertJSON(t, http.StatusCreated, &transaction{})

	var closed cashMonth
	closeByCount(t, app, token, household, wallet.AccountID, "2026-01", map[string]any{
		"statement_balance": eur("30.00"), "counted_on": "2026-02-03",
	}).AssertJSON(t, http.StatusOK, &closed)

	if closed.State != "CLOSED" || closed.StatementBalance == nil || closed.StatementBalance.Amount != "35.00" ||
		closed.CountedOn != "2026-02-03" {
		t.Errorf("closed = %+v, want CLOSED at 35.00, the month-end balance, counted on 2026-02-03", closed)
	}

	var got cashMonth
	app.Get(t, "/api/households/"+household+"/accounts/"+wallet.AccountID+"/months/2026-01", token).
		AssertJSON(t, http.StatusOK, &got)

	if got.CountedOn != "2026-02-03" || got.StatementBalance == nil || got.StatementBalance.Amount != "35.00" ||
		got.ReopenBlocked {
		t.Errorf("January = %+v, want closed at 35.00, counted on 2026-02-03, and reopenable", got)
	}

	// February has Transactions, yet January reopens, and they stay.
	var reopened cashMonth
	app.Post(t, "/api/households/"+household+"/accounts/"+wallet.AccountID+"/months/2026-01/reopen", token, nil).
		AssertJSON(t, http.StatusOK, &reopened)

	if reopened.State != "OPEN" || reopened.StatementBalance != nil || reopened.CountedOn != "" {
		t.Errorf("reopened = %+v, want OPEN with no Statement balance nor count", reopened)
	}

	if b := walletBalance(t, app, token, household, wallet.AccountID); b != "29.00" {
		t.Errorf("balance = %s, want 29.00", b)
	}

	// Closed again with a new count, taken today.
	closeByCount(t, app, token, household, wallet.AccountID, "2026-01", map[string]any{
		"statement_balance": eur("29.00"), "counted_on": "2026-03-10",
	}).AssertJSON(t, http.StatusOK, &closed)

	if closed.StatementBalance.Amount != "35.00" || closed.CountedOn != "2026-03-10" {
		t.Errorf("closed again = %+v, want 35.00, counted on 2026-03-10", closed)
	}
}

func TestClosingACashMonthIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t, apitest.WithClock(march10))
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")
	main := addAccount(t, app, token, household, map[string]string{
		"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
	})

	for name, tc := range map[string]struct {
		account string
		body    map[string]any
		status  int
		problem string
	}{
		"counted on the month's last day": {
			wallet.AccountID,
			map[string]any{"statement_balance": eur("40.00"), "counted_on": "2026-01-31"},
			http.StatusUnprocessableEntity, "/problems/invalid-count-date",
		},
		"counted after today": {
			wallet.AccountID,
			map[string]any{"statement_balance": eur("40.00"), "counted_on": "2026-03-11"},
			http.StatusUnprocessableEntity, "/problems/invalid-count-date",
		},
		"cash without counted_on": {
			wallet.AccountID,
			map[string]any{"statement_balance": eur("40.00")},
			http.StatusBadRequest, "/problems/invalid-month-closing",
		},
		"a bank Account with counted_on": {
			main.AccountID,
			map[string]any{"statement_balance": eur("40.00"), "counted_on": "2026-02-03"},
			http.StatusBadRequest, "/problems/invalid-month-closing",
		},
	} {
		t.Run(name, func(t *testing.T) {
			closeByCount(t, app, token, household, tc.account, "2026-01", tc.body).AssertProblem(t, tc.status, tc.problem)
		})
	}

	var january cashMonth
	app.Get(t, "/api/households/"+household+"/accounts/"+wallet.AccountID+"/months/2026-01", token).
		AssertJSON(t, http.StatusOK, &january)

	if january.State != "OPEN" {
		t.Errorf("January = %+v, want still OPEN", january)
	}
}
