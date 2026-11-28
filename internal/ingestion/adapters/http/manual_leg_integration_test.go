//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// transferStatement has two ATM withdrawals of 100.00 and a cash deposit of
// 3000.00; with an Export balance of 3000.00 the Opening balance is 200.00.
const transferStatement = "Date;Description;Amount\n" +
	"02/01/2026;ATM one;-100,00\n" +
	"03/01/2026;ATM two;-100,00\n" +
	"15/01/2026;Deposit;3.000,00\n"

func newTransferFixture(t *testing.T, app *apitest.App) acceptFixture {
	t.Helper()

	f := newImportFixture(t, app, "alice")
	f.confirm(t, transferStatement, "3000.00")

	pending := map[string]importedTransaction{}
	for _, i := range f.imported(t, "?state=PENDING").Items {
		pending[i.Description] = i
	}

	return acceptFixture{importFixture: f, pending: pending}
}

// recordLeg records a cash leg on the wallet naming counterpart.
func (f acceptFixture) recordLeg(t *testing.T, wallet, counterpart, date, amount string) legLine {
	t.Helper()

	var l legLine
	f.app.Post(t, "/api/households/"+f.household+"/accounts/"+wallet+"/transactions", f.token, map[string]any{
		"date": date, "amount": map[string]any{"amount": json.Number(amount), "currency": "EUR"},
		"description": "Cash " + date, "counterpart_account_id": counterpart,
	}).AssertJSON(t, http.StatusCreated, &l)

	return l
}

func (f acceptFixture) suggestions(t *testing.T, row, cash string) *apitest.Response {
	t.Helper()

	return f.app.Get(t, "/api/households/"+f.household+"/imported-transactions/"+f.pending[row].ImportedTransactionID+
		"/transfer-suggestions?account_id="+cash, f.token)
}

func assertReason(t *testing.T, res *apitest.Response, want string) {
	t.Helper()

	res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-transfer")

	var p struct {
		Reason string `json:"reason"`
	}

	if err := json.Unmarshal(res.Body, &p); err != nil {
		t.Fatal(err)
	}

	if p.Reason != want {
		t.Errorf("reason = %q, want %s", p.Reason, want)
	}
}

// A member notes a cash deposit by hand before the statement arrives: the
// leg waits In transit; the deposit row's suggestions offer the legs within
// a week of it with the opposite amount, closest first; and Accepting it
// with one of them Matches that leg instead of recording a second, with no
// date window (spec stories 16, 17 and 19).
func TestAManualCashLegIsSuggestedAndMatchedOnAccept(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t, apitest.Start(t))
	wallet := f.addCash(t, "Wallet", "EUR", "5000.00")
	savings := addAccount(t, f.app, f.token, f.household)

	near := f.recordLeg(t, wallet, f.account, "2026-01-13", "-3000.00")
	week := f.recordLeg(t, wallet, f.account, "2026-01-22", "-3000.00")
	eightDays := f.recordLeg(t, wallet, f.account, "2026-01-23", "-3000.00")
	f.recordLeg(t, wallet, f.account, "2026-01-14", "-2999.99")
	f.recordLeg(t, wallet, savings, "2026-01-15", "-3000.00")

	if near.CategoryName != "Account transfer" || near.CounterpartAccountID != f.account || near.MatchID != "" ||
		near.CreatedByTransfer {
		t.Errorf("recorded = %+v, want a manual leg against Account transfer naming the bank, In transit", near)
	}

	if n := len(f.legsOf(t, wallet, "?in_transit=true")); n != 5 {
		t.Errorf("%d legs In transit, want 5", n)
	}

	var suggested struct {
		Legs                 []legLine             `json:"legs"`
		ImportedTransactions []importedTransaction `json:"imported_transactions"`
	}

	f.suggestions(t, "Deposit", wallet).AssertJSON(t, http.StatusOK, &suggested)

	if len(suggested.Legs) != 2 || suggested.Legs[0].TransactionID != near.TransactionID ||
		suggested.Legs[1].TransactionID != week.TransactionID {
		t.Errorf("suggested legs = %+v, want the legs of the 13th and the 22nd, closest first", suggested.Legs)
	}

	if len(suggested.ImportedTransactions) != 0 {
		t.Errorf("suggested rows = %+v, want none: a cash Account has no statement", suggested.ImportedTransactions)
	}

	// The leg 8 days away isn't suggested, but can still be confirmed.
	a := f.accepted(t, "Deposit", map[string]any{
		"transfer_account_id": wallet, "match_transaction_id": eightDays.TransactionID,
	})

	legs := f.legsOf(t, wallet, "")
	if len(legs) != 5 {
		t.Errorf("%d wallet legs, want still 5: no second cash leg", len(legs))
	}

	var matched legLine

	for _, l := range legs {
		if l.TransactionID == eightDays.TransactionID {
			matched = l
		}
	}

	bank := f.legsOf(t, f.account, "")
	if len(bank) != 1 || bank[0].TransactionID != a.TransactionID || matched.MatchID == "" ||
		bank[0].MatchID != matched.MatchID {
		t.Fatalf("bank legs %+v and the leg %+v, want them sharing one Match", bank, matched)
	}

	// 5000.00, less four legs of 3000.00 and one of 2999.99: Matching
	// moves nothing.
	if b := f.balanceOf(t, wallet); b != "-9999.99" {
		t.Errorf("wallet balance = %s, want -9999.99", b)
	}

	if n := len(f.legsOf(t, wallet, "?in_transit=true")); n != 4 {
		t.Errorf("%d legs In transit, want 4", n)
	}
}

func TestMatchingALegOnAcceptIsRefused(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t, apitest.Start(t))
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")
	savings := addAccount(t, f.app, f.token, f.household)
	deposit := f.recordLeg(t, wallet, f.account, "2026-01-15", "-3000.00")
	elsewhere := f.recordLeg(t, wallet, savings, "2026-01-02", "100.00")

	for name, tc := range map[string]struct {
		body   map[string]any
		reason string
	}{
		"another amount": {
			map[string]any{"transfer_account_id": wallet, "match_transaction_id": deposit.TransactionID},
			"NOT_A_MATCHING_LEG",
		},
		"another counterpart": {
			map[string]any{"transfer_account_id": wallet, "match_transaction_id": elsewhere.TransactionID},
			"NOT_A_MATCHING_LEG",
		},
		"an unknown leg": {
			map[string]any{"transfer_account_id": wallet, "match_transaction_id": "0198f0a0-0000-7000-8000-000000000000"},
			"NOT_A_MATCHING_LEG",
		},
		"no Transfer": {
			map[string]any{"match_transaction_id": deposit.TransactionID}, "MATCH_WITHOUT_TRANSFER",
		},
	} {
		t.Run(name, func(t *testing.T) {
			assertReason(t, f.accept(t, "ATM one", tc.body), tc.reason)
		})
	}

	f.suggestions(t, "Deposit", savings).AssertProblem(t, http.StatusNotImplemented, "/problems/not-implemented")
	assertReason(t, f.suggestions(t, "Deposit", "0198f0a0-0000-7000-8000-000000000000"), "UNKNOWN_ACCOUNT")

	if pending := f.imported(t, "?state=PENDING").Items; len(pending) != 3 {
		t.Errorf("%d pending rows, want 3", len(pending))
	}
}

// Two Accepts racing to Match the same leg: the leg's version check lets
// exactly one through, and the other is refused (spec "The Match").
func TestTwoAcceptsMatchingOneLegLetOnlyOneThrough(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t, apitest.Start(t))
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")
	leg := f.recordLeg(t, wallet, f.account, "2026-01-02", "100.00")

	statuses := make([]int, 2)

	var wg sync.WaitGroup

	for i, row := range []string{"ATM one", "ATM two"} {
		wg.Add(1)

		go func() {
			defer wg.Done()

			statuses[i] = f.accept(t, row, map[string]any{
				"transfer_account_id": wallet, "match_transaction_id": leg.TransactionID,
			}).Status
		}()
	}

	wg.Wait()

	ok, refused := 0, 0

	for _, s := range statuses {
		switch s {
		case http.StatusOK:
			ok++
		case http.StatusUnprocessableEntity:
			refused++
		}
	}

	if ok != 1 || refused != 1 {
		t.Errorf("statuses = %v, want one 200 and one 422", statuses)
	}

	if n := len(f.legsOf(t, wallet, "")); n != 1 {
		t.Errorf("%d wallet legs, want the one manual leg", n)
	}

	if n := len(f.imported(t, "?state=PENDING").Items); n != 2 {
		t.Errorf("%d pending rows, want 2: the loser and the deposit", n)
	}
}

// A month closes while a leg is In transit: money moving across a month end
// never stops a close, and the month counts it (spec stories 19 and 20).
func TestACashMonthClosesWithALegInTransit(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t, apitest.Start(t))
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")
	leg := f.recordLeg(t, wallet, f.account, "2026-01-31", "100.00")

	var closed struct {
		State                 string `json:"state"`
		TransferLegsInTransit int    `json:"transfer_legs_in_transit"`
	}

	f.app.Post(t, "/api/households/"+f.household+"/accounts/"+wallet+"/months/2026-01/close", f.token,
		map[string]any{
			"statement_balance": map[string]any{"amount": json.Number("120.00"), "currency": "EUR"},
			"counted_on":        "2026-02-01",
		}).AssertJSON(t, http.StatusOK, &closed)

	if closed.State != "CLOSED" || closed.TransferLegsInTransit != 1 {
		t.Errorf("closed = %+v, want CLOSED with one leg In transit", closed)
	}

	if legs := f.legsOf(t, wallet, "?in_transit=true"); len(legs) != 1 || legs[0].TransactionID != leg.TransactionID {
		t.Errorf("in transit = %+v, want the leg", legs)
	}
}
