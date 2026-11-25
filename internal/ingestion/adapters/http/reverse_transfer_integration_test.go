//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

func (f acceptFixture) matchStatus(t *testing.T, matchID string) int {
	t.Helper()

	return f.app.Get(t, "/api/households/"+f.household+"/matches/"+matchID, f.token).Status
}

// stateOf is the state of the row with the description.
func (f acceptFixture) stateOf(t *testing.T, description string) string {
	t.Helper()

	for _, i := range f.imported(t, "").Items {
		if i.Description == description {
			return i.State
		}
	}

	t.Fatalf("no row %q", description)

	return ""
}

// liveLegs are the Account's legs that aren't reversed.
func (f acceptFixture) liveLegs(t *testing.T, account string) []legLine {
	t.Helper()

	var p page[struct {
		legLine

		ReversedBy string `json:"reversed_by_transaction_id"`
	}]
	f.app.Get(t, "/api/households/"+f.household+"/accounts/"+account+"/transactions", f.token).
		AssertJSON(t, http.StatusOK, &p)

	var live []legLine

	for _, l := range p.Items {
		if l.CounterpartAccountID != "" && l.ReversedBy == "" {
			live = append(live, l.legLine)
		}
	}

	return live
}

// transferred Accepts ATM one (-100.00) as a Transfer to the wallet, and
// returns the bank leg and the cash leg.
func (f acceptFixture) transferred(t *testing.T, wallet string, body map[string]any) (bank, cash legLine) {
	t.Helper()

	f.accepted(t, "ATM one", body)

	return f.legsOf(t, f.account, "")[0], f.liveLegs(t, wallet)[0]
}

// Reversing the bank leg of an Accept as Transfer reverses the cash leg it
// recorded too, removes the Match and returns the row to pending (spec
// story 21).
func TestReversingTheBankLegReversesTheAutomaticCashLeg(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t, apitest.Start(t))
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")
	bank, cash := f.transferred(t, wallet, map[string]any{"transfer_account_id": wallet})

	if !cash.CreatedByTransfer {
		t.Fatalf("cash leg = %+v, want the automatic one", cash)
	}

	var reversal legLine
	f.reverse(t, bank.TransactionID).AssertJSON(t, http.StatusCreated, &reversal)

	if reversal.CounterpartAccountID != "" || reversal.Amount.Amount.String() != "100.00" {
		t.Errorf("reversal = %+v, want +100.00, not a leg itself", reversal)
	}

	if n := len(f.liveLegs(t, wallet)); n != 0 {
		t.Errorf("%d live wallet legs, want the automatic one reversed too", n)
	}

	if b := f.balanceOf(t, wallet); b != "20.00" {
		t.Errorf("wallet balance = %s, want 20.00", b)
	}

	if b := f.balanceOf(t, f.account); b != "200.00" {
		t.Errorf("bank balance = %s, want 200.00", b)
	}

	if s := f.stateOf(t, "ATM one"); s != "PENDING" {
		t.Errorf("row = %s, want PENDING", s)
	}

	if s := f.matchStatus(t, bank.MatchID); s != http.StatusNotFound {
		t.Errorf("getMatch = %d, want 404", s)
	}

	if n := len(f.legsOf(t, wallet, "?in_transit=true")) + len(f.legsOf(t, f.account, "?in_transit=true")); n != 0 {
		t.Errorf("%d legs In transit, want none", n)
	}
}

// A cash leg entered by hand stays when the bank leg Matched to it is
// reversed: it is In transit again and can be Matched again.
func TestReversingTheBankLegLeavesAManualCashLegInTransit(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t, apitest.Start(t))
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")
	manual := f.recordLeg(t, wallet, f.account, "2026-01-02", "100.00")
	bank, _ := f.transferred(t, wallet, map[string]any{
		"transfer_account_id": wallet, "match_transaction_id": manual.TransactionID,
	})

	f.reverse(t, bank.TransactionID).AssertJSON(t, http.StatusCreated, &legLine{})

	inTransit := f.legsOf(t, wallet, "?in_transit=true")
	if len(inTransit) != 1 || inTransit[0].TransactionID != manual.TransactionID || inTransit[0].MatchID != "" {
		t.Errorf("in transit = %+v, want the manual leg, unmatched", inTransit)
	}

	if b := f.balanceOf(t, wallet); b != "120.00" {
		t.Errorf("wallet balance = %s, want 120.00: the manual leg stays", b)
	}

	if s := f.stateOf(t, "ATM one"); s != "PENDING" {
		t.Errorf("row = %s, want PENDING", s)
	}

	if s := f.matchStatus(t, bank.MatchID); s != http.StatusNotFound {
		t.Errorf("getMatch = %d, want 404", s)
	}

	// The row can be Accepted against the same leg again.
	f.accepted(t, "ATM one", map[string]any{"transfer_account_id": wallet, "match_transaction_id": manual.TransactionID})
}

// Reversing the cash leg alone leaves the bank leg Accepted, In transit.
func TestReversingTheCashLegAloneLeavesTheBankLegInTransit(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t, apitest.Start(t))
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")
	bank, cash := f.transferred(t, wallet, map[string]any{"transfer_account_id": wallet})

	f.reverse(t, cash.TransactionID).AssertJSON(t, http.StatusCreated, &legLine{})

	inTransit := f.legsOf(t, f.account, "?in_transit=true")
	if len(inTransit) != 1 || inTransit[0].TransactionID != bank.TransactionID {
		t.Errorf("bank in transit = %+v, want the bank leg", inTransit)
	}

	if s := f.stateOf(t, "ATM one"); s != "ACCEPTED" {
		t.Errorf("row = %s, want still ACCEPTED", s)
	}

	if b := f.balanceOf(t, f.account); b != "100.00" {
		t.Errorf("bank balance = %s, want 100.00", b)
	}

	if s := f.matchStatus(t, bank.MatchID); s != http.StatusNotFound {
		t.Errorf("getMatch = %d, want 404", s)
	}
}

// If any Transaction the command would reverse is in a closed month, the
// whole command is refused, naming that Account, and nothing changes.
func TestReversingALegInAClosedMonthIsRefusedWhole(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t, apitest.Start(t))
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")
	bank, cash := f.transferred(t, wallet, map[string]any{"transfer_account_id": wallet})

	f.app.Post(t, "/api/households/"+f.household+"/accounts/"+wallet+"/months/2026-01/close", f.token,
		map[string]any{
			"statement_balance": map[string]any{"amount": json.Number("120.00"), "currency": "EUR"},
			"counted_on":        "2026-02-01",
		}).AssertJSON(t, http.StatusOK, &map[string]any{})

	for name, leg := range map[string]string{"the bank leg": bank.TransactionID, "the cash leg": cash.TransactionID} {
		t.Run(name, func(t *testing.T) {
			res := f.reverse(t, leg)
			res.AssertProblem(t, http.StatusConflict, "/problems/month-closed")

			var p struct {
				AccountID    string `json:"account_id"`
				BookingMonth string `json:"booking_month"`
			}

			if err := json.Unmarshal(res.Body, &p); err != nil {
				t.Fatal(err)
			}

			if p.AccountID != wallet || p.BookingMonth != "2026-01" {
				t.Errorf("problem = %s, want January of the wallet", res.Body)
			}
		})
	}

	if s := f.matchStatus(t, bank.MatchID); s != http.StatusOK {
		t.Errorf("getMatch = %d, want the Match kept", s)
	}

	if legs := f.liveLegs(t, f.account); len(legs) != 1 || legs[0].MatchID != bank.MatchID {
		t.Errorf("bank legs = %+v, want the bank leg still Matched", legs)
	}

	if s := f.stateOf(t, "ATM one"); s != "ACCEPTED" {
		t.Errorf("row = %s, want still ACCEPTED", s)
	}

	if b := f.balanceOf(t, f.account); b != "100.00" {
		t.Errorf("bank balance = %s, want 100.00", b)
	}
}
