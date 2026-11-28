//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// legLine is a Transfer leg as an Account's Transaction list writes it.
type legLine struct {
	TransactionID         string `json:"transaction_id"`
	Amount                money  `json:"amount"`
	BookingDate           string `json:"booking_date"`
	TransactionDate       string `json:"transaction_date"`
	Description           string `json:"description"`
	CategoryName          string `json:"category_name"`
	CounterpartAccountID  string `json:"counterpart_account_id"`
	MatchID               string `json:"match_id"`
	CreatedByTransfer     bool   `json:"created_by_transfer"`
	ImportedTransactionID string `json:"imported_transaction_id"`
}

type match struct {
	MatchID string `json:"match_id"`
	Kind    string `json:"kind"`
	Legs    []struct {
		TransactionID string `json:"transaction_id"`
		AccountID     string `json:"account_id"`
	} `json:"legs"`
	ConfirmedBy string `json:"confirmed_by"`
	ConfirmedAt string `json:"confirmed_at"`
}

// addCash adds a cash Account started on 2026-01-01 with an Opening count
// of count in currency.
func (f acceptFixture) addCash(t *testing.T, name, currency, count string) string {
	t.Helper()

	amount := map[string]any{"amount": json.Number(count), "currency": currency}

	var a struct {
		AccountID string `json:"account_id"`
	}

	f.app.Post(t, "/api/households/"+f.household+"/accounts", f.token, map[string]any{
		"name": name, "kind": "CASH", "currency": currency, "start_date": "2026-01-01",
		"opening_count": amount, "opening_count_confirmation": amount,
	}).AssertJSON(t, http.StatusCreated, &a)

	return a.AccountID
}

func (f acceptFixture) balanceOf(t *testing.T, id string) string {
	t.Helper()

	var a account
	f.app.Get(t, "/api/households/"+f.household+"/accounts/"+id, f.token).AssertJSON(t, http.StatusOK, &a)

	if a.Balance == nil {
		t.Fatalf("account %s has no balance", id)
	}

	return a.Balance.Amount.String()
}

func (f acceptFixture) legsOf(t *testing.T, id, query string) []legLine {
	t.Helper()

	var p page[legLine]
	f.app.Get(t, "/api/households/"+f.household+"/accounts/"+id+"/transactions"+query, f.token).
		AssertJSON(t, http.StatusOK, &p)

	var legs []legLine

	for _, l := range p.Items {
		if l.CounterpartAccountID != "" {
			legs = append(legs, l)
		}
	}

	return legs
}

// An ATM withdrawal is Accepted as a Transfer to the wallet: the bank leg
// is the row's Transaction against Account transfer, the wallet gets the
// opposite leg on the row's Transaction date, and the two are Matched. A
// cash deposit goes the other way (spec stories 14, 15 and 22).
func TestAcceptingARowAsATransferToCash(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")
	row := f.pending["Rent"]

	a := f.accepted(t, "Rent", map[string]any{"transfer_account_id": wallet})

	if a.State != "ACCEPTED" || a.TransactionID == "" {
		t.Fatalf("accepted = %+v, want ACCEPTED with its Transaction", a)
	}

	if b := f.balanceOf(t, f.account); b != "-489.50" {
		t.Errorf("bank balance = %s, want -489.50 (510.50 - 1000.00)", b)
	}

	if b := f.balanceOf(t, wallet); b != "1020.00" {
		t.Errorf("wallet balance = %s, want 1020.00 (20.00 + 1000.00)", b)
	}

	bankLegs, cashLegs := f.legsOf(t, f.account, ""), f.legsOf(t, wallet, "")
	if len(bankLegs) != 1 || len(cashLegs) != 1 {
		t.Fatalf("legs = %+v and %+v, want one on each Account", bankLegs, cashLegs)
	}

	bank, cash := bankLegs[0], cashLegs[0]

	if bank.TransactionID != a.TransactionID || bank.Amount.Amount.String() != "-1000.00" ||
		bank.CategoryName != "Account transfer" || bank.CounterpartAccountID != wallet || bank.CreatedByTransfer ||
		bank.ImportedTransactionID != row.ImportedTransactionID || bank.MatchID == "" {
		t.Errorf("bank leg = %+v, want the row's -1000.00 against Account transfer, naming the wallet, Matched", bank)
	}

	if cash.Amount.Amount.String() != "1000.00" || cash.BookingDate != row.TransactionDate ||
		cash.TransactionDate != row.TransactionDate || cash.Description != "Rent" ||
		cash.CategoryName != "Account transfer" || cash.CounterpartAccountID != f.account || !cash.CreatedByTransfer ||
		cash.ImportedTransactionID != "" || cash.MatchID != bank.MatchID {
		t.Errorf("cash leg = %+v, want +1000.00 on %s naming the bank, automatic, with the bank leg's Match",
			cash, row.TransactionDate)
	}

	var m match
	f.app.Get(t, "/api/households/"+f.household+"/matches/"+bank.MatchID, f.token).AssertJSON(t, http.StatusOK, &m)

	if m.MatchID != bank.MatchID || m.Kind != "ACCOUNT_TRANSFER" || len(m.Legs) != 2 || m.ConfirmedBy == "" ||
		m.ConfirmedAt == "" {
		t.Fatalf("match = %+v, want an ACCOUNT_TRANSFER with two legs, confirmed", m)
	}

	legs := map[string]string{m.Legs[0].TransactionID: m.Legs[0].AccountID, m.Legs[1].TransactionID: m.Legs[1].AccountID}
	if legs[bank.TransactionID] != f.account || legs[cash.TransactionID] != wallet {
		t.Errorf("match legs = %+v, want the bank leg on the bank and the cash leg on the wallet", m.Legs)
	}

	// Matched legs aren't In transit.
	if n := len(f.legsOf(t, wallet, "?in_transit=true")); n != 0 {
		t.Errorf("%d wallet legs In transit, want none", n)
	}

	// A deposit of the salary from the wallet goes the other way.
	f.accepted(t, "Salary", map[string]any{"transfer_account_id": wallet})

	if b := f.balanceOf(t, wallet); b != "-1980.00" {
		t.Errorf("wallet balance = %s, want -1980.00 (1020.00 - 3000.00)", b)
	}

	if b := f.balanceOf(t, f.account); b != "2510.50" {
		t.Errorf("bank balance = %s, want 2510.50", b)
	}

	if legs := f.legsOf(t, wallet, "?booking_month=2026-01"); len(legs) != 2 || legs[1].Amount.Amount.String() != "-3000.00" {
		t.Errorf("wallet legs = %+v, want the withdrawal and a -3000.00 deposit", legs)
	}
}

func TestAcceptingAsATransferIsRefused(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")
	dollars := f.addCash(t, "Dollars", "USD", "20.00")
	food := f.createCategory(t, "Food", "EXPENSE")

	for reason, body := range map[string]map[string]any{
		"CATEGORY_AND_TRANSFER":  {"category_id": food, "transfer_account_id": wallet},
		"MATCH_WITHOUT_TRANSFER": {"match_imported_transaction_id": "0198f0a0-0000-7000-8000-000000000000"},
		"MATCH_LEG_AND_ROW": {
			"transfer_account_id":           wallet,
			"match_transaction_id":          "0198f0a0-0000-7000-8000-000000000000",
			"match_imported_transaction_id": "0198f0a0-0000-7000-8000-000000000001",
		},
		"SAME_ACCOUNT":      {"transfer_account_id": f.account},
		"CURRENCY_MISMATCH": {"transfer_account_id": dollars},
		"UNKNOWN_ACCOUNT":   {"transfer_account_id": "0198f0a0-0000-7000-8000-000000000000"},
	} {
		t.Run(reason, func(t *testing.T) {
			res := f.accept(t, "Rent", body)
			res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-transfer")

			var p struct {
				Reason string `json:"reason"`
			}

			if err := json.Unmarshal(res.Body, &p); err != nil {
				t.Fatal(err)
			}

			if p.Reason != reason {
				t.Errorf("reason = %q, want %s", p.Reason, reason)
			}
		})
	}

	if pending := f.imported(t, "?state=PENDING").Items; len(pending) != 3 {
		t.Errorf("%d pending rows, want all three still pending", len(pending))
	}

	if b := f.balanceOf(t, wallet); b != "20.00" {
		t.Errorf("wallet balance = %s, want 20.00", b)
	}
}

// A cash leg dated in a closed month of the cash Account is refused, naming
// the cash Account, and nothing is booked on either Account.
func TestAcceptingAsATransferIntoAClosedCashMonthIsRefused(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")

	f.app.Post(t, "/api/households/"+f.household+"/accounts/"+wallet+"/months/2026-01/close", f.token,
		map[string]any{
			"statement_balance": map[string]any{"amount": json.Number("20.00"), "currency": "EUR"},
			"counted_on":        "2026-02-01",
		}).AssertJSON(t, http.StatusOK, &map[string]any{})

	res := f.accept(t, "Rent", map[string]any{"transfer_account_id": wallet})
	p := monthConflict(t, res, "/problems/month-closed")

	var named struct {
		AccountID string `json:"account_id"`
	}

	if err := json.Unmarshal(res.Body, &named); err != nil {
		t.Fatal(err)
	}

	if named.AccountID != wallet || p.BookingMonth != "2026-01" || p.OpenMonth != "2026-02" {
		t.Errorf("problem = %s, want January closed on the wallet", res.Body)
	}

	if b := f.balanceOf(t, f.account); b != "510.50" {
		t.Errorf("bank balance = %s, want 510.50, nothing booked", b)
	}

	if f.pending["Rent"].ImportedTransactionID == "" || len(f.imported(t, "?state=PENDING").Items) != 3 {
		t.Error("the row should still be pending")
	}
}

// A row dated before the cash Account's start date can't be Accepted as a
// Transfer to it: the cash Account didn't exist yet. The refusal names the
// cash Account and nothing is booked. A row dated on the start date is
// Accepted.
func TestAcceptingAsATransferBeforeTheCashStartDateIsRefused(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	count := map[string]any{"amount": json.Number("20.00"), "currency": "EUR"}

	var wallet struct {
		AccountID string `json:"account_id"`
	}

	f.app.Post(t, "/api/households/"+f.household+"/accounts", f.token, map[string]any{
		"name": "Wallet", "kind": "CASH", "currency": "EUR", "start_date": "2026-01-15",
		"opening_count": count, "opening_count_confirmation": count,
	}).AssertJSON(t, http.StatusCreated, &wallet)

	// Rent is dated 2026-01-01, before the wallet's start date.
	res := f.accept(t, "Rent", map[string]any{"transfer_account_id": wallet.AccountID})
	res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-transfer")

	var p struct {
		Reason    string `json:"reason"`
		AccountID string `json:"account_id"`
		Detail    string `json:"detail"`
	}

	if err := json.Unmarshal(res.Body, &p); err != nil {
		t.Fatal(err)
	}

	if p.Reason != "BEFORE_START_DATE" || p.AccountID != wallet.AccountID ||
		!strings.Contains(p.Detail, "2026-01-01") || !strings.Contains(p.Detail, "2026-01-15") {
		t.Errorf("problem = %s, want BEFORE_START_DATE naming the wallet and both dates", res.Body)
	}

	if len(f.imported(t, "?state=PENDING").Items) != 3 {
		t.Error("the row should still be pending")
	}

	if b := f.balanceOf(t, f.account); b != "510.50" {
		t.Errorf("bank balance = %s, want 510.50, nothing booked", b)
	}

	if b := f.balanceOf(t, wallet.AccountID); b != "20.00" {
		t.Errorf("wallet balance = %s, want 20.00, nothing booked", b)
	}

	// Salary is dated 2026-01-15, the wallet's start date.
	if a := f.accepted(t, "Salary", map[string]any{"transfer_account_id": wallet.AccountID}); a.State != "ACCEPTED" {
		t.Errorf("accepted = %+v, want ACCEPTED on the start date", a)
	}
}

// Until their tickets land, the statement-to-statement pieces of the
// contract answer 501 and book nothing (statement-transfers ticket 01).
func TestStatementTransferPiecesAnswerNotImplementedYet(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	savings := addAccount(t, f.app, f.token, f.household)
	row := f.pending["Rent"].ImportedTransactionID
	other := f.pending["Salary"].ImportedTransactionID

	for name, res := range map[string]func() *apitest.Response{
		"a statement Account as transfer_account_id (02)": func() *apitest.Response {
			return f.accept(t, "Rent", map[string]any{"transfer_account_id": savings})
		},
		"suggestions on a statement Account (03)": func() *apitest.Response {
			return f.app.Get(t, "/api/households/"+f.household+"/imported-transactions/"+row+
				"/transfer-suggestions?account_id="+savings, f.token)
		},
		"suggestions without account_id (03)": func() *apitest.Response {
			return f.app.Get(t, "/api/households/"+f.household+"/imported-transactions/"+row+
				"/transfer-suggestions", f.token)
		},
		"match_imported_transaction_id (04)": func() *apitest.Response {
			return f.accept(t, "Rent", map[string]any{
				"transfer_account_id": savings, "match_imported_transaction_id": other,
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			res().AssertProblem(t, http.StatusNotImplemented, "/problems/not-implemented")
		})
	}

	if pending := f.imported(t, "?state=PENDING").Items; len(pending) != 3 {
		t.Errorf("%d pending rows, want all three still pending", len(pending))
	}
}

// Every Imported transaction names the Account whose statement it is on,
// so a suggested row from another Account can be told apart (spec story 11).
func TestImportedTransactionsNameTheirAccount(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	for _, i := range f.imported(t, "").Items {
		if i.AccountID != f.account {
			t.Errorf("row %d account_id = %q, want %s", i.Line, i.AccountID, f.account)
		}
	}

	var accepted importedTransaction
	f.accept(t, "Rent", nil).AssertJSON(t, http.StatusOK, &accepted)

	if accepted.AccountID != f.account {
		t.Errorf("accepted row account_id = %q, want %s", accepted.AccountID, f.account)
	}
}
