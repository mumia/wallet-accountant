//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// addStatementAccount adds another statement Account of kind to f's
// Household, started on start, with a statement file import Connection,
// and confirms file on it with the Export balance balance. It returns that
// Account as an acceptFixture of its own, sharing f's Household and token.
func (f acceptFixture) addStatementAccount(t *testing.T, name, kind, start, file, balance string) acceptFixture {
	t.Helper()

	var a struct {
		AccountID string `json:"account_id"`
	}

	f.app.Post(t, "/api/households/"+f.household+"/accounts", f.token, map[string]string{
		"name": name, "kind": kind, "currency": "EUR", "start_date": start,
	}).AssertJSON(t, http.StatusCreated, &a)

	other := f.importFixture
	other.account = a.AccountID
	other.connection = other.create(t, map[string]any{
		"name": "Statement file", "kind": "STATEMENT_FILE_IMPORT", "csv_mapping": signed(),
	}).ConnectionID
	other.confirm(t, file, balance)

	pending := map[string]importedTransaction{}
	for _, i := range other.imported(t, "?state=PENDING").Items {
		pending[i.Description] = i
	}

	return acceptFixture{importFixture: other, pending: pending}
}

// reasonAndAccount checks a 422 invalid-transfer with the reason, naming
// the Account account.
func reasonAndAccount(t *testing.T, res *apitest.Response, reason, account string) {
	t.Helper()

	assertReason(t, res, reason)

	var p struct {
		AccountID string `json:"account_id"`
	}

	if err := json.Unmarshal(res.Body, &p); err != nil {
		t.Fatal(err)
	}

	if p.AccountID != account {
		t.Errorf("problem names %q, want %s", p.AccountID, account)
	}
}

// The bank's Rent row (-1000.00 on 2026-01-01) is a repayment to another
// statement Account, whose own row (+1000.00 on 2026-01-02) arrives on its
// statement. Accepting the bank row alone books only its leg, In transit;
// Accepting the other row with that leg Matches the two (statement-transfers
// stories 1-4).
func TestAcceptingATransferBetweenStatementAccounts(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"CREDIT_CARD", "BANK_ACCOUNT"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			f := newAcceptFixture(t, apitest.Start(t), "alice")
			other := f.addStatementAccount(t, "Other", kind, "2026-01-01",
				"Date;Description;Amount\n02/01/2026;Payment;1.000,00\n", "1000.00")

			rent := f.accepted(t, "Rent", map[string]any{"transfer_account_id": other.account})

			if b := f.balanceOf(t, f.account); b != "-489.50" {
				t.Errorf("bank balance = %s, want -489.50 (510.50 - 1000.00)", b)
			}

			if b := f.balanceOf(t, other.account); b != "0.00" {
				t.Errorf("%s balance = %s, want 0.00: it gets no leg of its own", kind, b)
			}

			inTransit := f.legsOf(t, f.account, "?in_transit=true")
			if len(inTransit) != 1 || inTransit[0].TransactionID != rent.TransactionID ||
				inTransit[0].CounterpartAccountID != other.account || inTransit[0].MatchID != "" ||
				inTransit[0].CategoryName != "Account transfer" {
				t.Fatalf("legs In transit = %+v, want the Rent leg naming the other Account, unmatched", inTransit)
			}

			if legs := f.legsOf(t, other.account, ""); len(legs) != 0 {
				t.Errorf("legs on the other Account = %+v, want none", legs)
			}

			payment := other.accepted(t, "Payment", map[string]any{
				"transfer_account_id": f.account, "match_transaction_id": rent.TransactionID,
			})

			if b := f.balanceOf(t, other.account); b != "1000.00" {
				t.Errorf("%s balance = %s, want 1000.00", kind, b)
			}

			bank, theirs := f.legsOf(t, f.account, ""), f.legsOf(t, other.account, "")
			if len(bank) != 1 || len(theirs) != 1 || bank[0].MatchID == "" || bank[0].MatchID != theirs[0].MatchID ||
				theirs[0].TransactionID != payment.TransactionID || theirs[0].CreatedByTransfer {
				t.Fatalf("legs = %+v and %+v, want one on each Account sharing a Match", bank, theirs)
			}

			var m match
			f.app.Get(t, "/api/households/"+f.household+"/matches/"+bank[0].MatchID, f.token).
				AssertJSON(t, http.StatusOK, &m)

			if len(m.Legs) != 2 {
				t.Errorf("match = %+v, want two legs", m)
			}

			if n := len(f.legsOf(t, f.account, "?in_transit=true")); n != 0 {
				t.Errorf("%d bank legs In transit, want none", n)
			}
		})
	}
}

// Reversing either leg of a Transfer between statement Accounts leaves the
// other booked, In transit, and returns only the reversed leg's row to
// pending (statement-transfers story 13).
func TestReversingOneStatementLegLeavesTheOtherInTransit(t *testing.T) {
	t.Parallel()

	for _, reversed := range []string{"the bank leg", "the card leg"} {
		t.Run(reversed, func(t *testing.T) {
			t.Parallel()
			f := newAcceptFixture(t, apitest.Start(t), "alice")
			card := f.addStatementAccount(t, "Card", "CREDIT_CARD", "2026-01-01",
				"Date;Description;Amount\n02/01/2026;Payment;1.000,00\n", "1000.00")

			rent := f.accepted(t, "Rent", map[string]any{"transfer_account_id": card.account})
			payment := card.accepted(t, "Payment", map[string]any{
				"transfer_account_id": f.account, "match_transaction_id": rent.TransactionID,
			})
			matchID := f.legsOf(t, f.account, "")[0].MatchID

			gone, kept, goneFixture, keptFixture, goneRow, keptRow, keptAccount :=
				rent, payment, f, card, "Rent", "Payment", card.account
			if reversed == "the card leg" {
				gone, kept, goneFixture, keptFixture, goneRow, keptRow, keptAccount =
					payment, rent, card, f, "Payment", "Rent", f.account
			}

			goneFixture.reversed(t, gone.TransactionID)

			if s := f.matchStatus(t, matchID); s != http.StatusNotFound {
				t.Errorf("getMatch = %d, want 404: the Match is removed", s)
			}

			inTransit := f.legsOf(t, keptAccount, "?in_transit=true")
			if len(inTransit) != 1 || inTransit[0].TransactionID != kept.TransactionID {
				t.Errorf("legs In transit = %+v, want the kept leg", inTransit)
			}

			if s := goneFixture.stateOf(t, goneRow); s != "PENDING" {
				t.Errorf("reversed leg's row = %s, want PENDING", s)
			}

			if s := keptFixture.stateOf(t, keptRow); s != "ACCEPTED" {
				t.Errorf("kept leg's row = %s, want ACCEPTED", s)
			}
		})
	}
}

// The bank row of 2026-01-01 is Accepted alone and January closes with its
// leg In transit. The card, started on 2026-02-01 (after the row: no check,
// statement-transfers story 7), Accepts its February row Matched to that
// leg, and reversing the card leg then unmatches it again; January stays
// closed throughout (statement-transfers story 12).
func TestALegInAClosedMonthIsMatchedAndUnmatched(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	card := f.addStatementAccount(t, "Card", "CREDIT_CARD", "2026-02-01",
		"Date;Description;Amount\n02/02/2026;Payment;1.000,00\n", "1000.00")

	rent := f.accepted(t, "Rent", map[string]any{"transfer_account_id": card.account})
	f.accepted(t, "Salary", map[string]any{"category_id": f.createCategory(t, "Pay", "INCOME")})
	f.closed(t, "2026-01", "2510.50")

	payment := card.accepted(t, "Payment", map[string]any{
		"transfer_account_id": f.account, "match_transaction_id": rent.TransactionID,
	})

	if legs := f.legsOf(t, f.account, ""); len(legs) != 1 || legs[0].MatchID == "" {
		t.Fatalf("bank legs = %+v, want the January leg Matched", legs)
	}

	card.reversed(t, payment.TransactionID)

	if legs := f.legsOf(t, f.account, "?in_transit=true"); len(legs) != 1 || legs[0].TransactionID != rent.TransactionID {
		t.Errorf("bank legs In transit = %+v, want the January leg again", legs)
	}

	if m := f.month(t, "2026-01"); m.State != "CLOSED" {
		t.Errorf("January = %s, want CLOSED", m.State)
	}
}

// Loan and crypto wallet Accounts take no part in Transfers, on either
// side, with cash too; the refusal names the Account and books nothing
// (statement-transfers story 8).
func TestATransferWithALoanOrCryptoWalletIsRefused(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")
	loan := f.addStatementAccount(t, "Loan", "LOAN", "2026-01-01",
		"Date;Description;Amount\n05/01/2026;Repayment;200,00\n", "-9800.00")
	crypto := f.addStatementAccount(t, "Crypto", "CRYPTO_WALLET", "2026-01-01",
		"Date;Description;Amount\n06/01/2026;Sold;-50,00\n", "100.00")

	for name, tc := range map[string]struct {
		res     func() *apitest.Response
		refused string
	}{
		"a bank row to a loan": {
			func() *apitest.Response {
				return f.accept(t, "Rent", map[string]any{"transfer_account_id": loan.account})
			},
			loan.account,
		},
		"a bank row to a crypto wallet": {
			func() *apitest.Response {
				return f.accept(t, "Rent", map[string]any{"transfer_account_id": crypto.account})
			},
			crypto.account,
		},
		"a loan row to a bank Account": {
			func() *apitest.Response {
				return loan.accept(t, "Repayment", map[string]any{"transfer_account_id": f.account})
			},
			loan.account,
		},
		"a loan row to cash": {
			func() *apitest.Response {
				return loan.accept(t, "Repayment", map[string]any{"transfer_account_id": wallet})
			},
			loan.account,
		},
		"a crypto wallet row to cash": {
			func() *apitest.Response {
				return crypto.accept(t, "Sold", map[string]any{"transfer_account_id": wallet})
			},
			crypto.account,
		},
		"a cash leg naming a loan": {
			func() *apitest.Response {
				return f.app.Post(t, "/api/households/"+f.household+"/accounts/"+wallet+"/transactions", f.token,
					map[string]any{
						"date": "2026-01-10", "amount": map[string]any{"amount": json.Number("-5.00"), "currency": "EUR"},
						"description": "To the loan", "counterpart_account_id": loan.account,
					})
			},
			loan.account,
		},
	} {
		t.Run(name, func(t *testing.T) {
			reasonAndAccount(t, tc.res(), "ACCOUNT_KIND_NOT_SUPPORTED", tc.refused)
		})
	}

	if s := f.stateOf(t, "Rent"); s != "PENDING" {
		t.Errorf("Rent = %s, want PENDING", s)
	}

	if s := loan.stateOf(t, "Repayment"); s != "PENDING" {
		t.Errorf("Repayment = %s, want PENDING", s)
	}

	if b := f.balanceOf(t, wallet); b != "20.00" {
		t.Errorf("wallet balance = %s, want 20.00", b)
	}
}

// suggestionList is listTransferSuggestions' answer.
type suggestionList struct {
	Legs                 []legLine             `json:"legs"`
	ImportedTransactions []importedTransaction `json:"imported_transactions"`
}

func (f acceptFixture) suggested(t *testing.T, row, query string) suggestionList {
	t.Helper()

	var l suggestionList
	f.app.Get(t, "/api/households/"+f.household+"/imported-transactions/"+f.pending[row].ImportedTransactionID+
		"/transfer-suggestions"+query, f.token).AssertJSON(t, http.StatusOK, &l)

	return l
}

// Suggestions for the bank's Rent row (-1000.00 on 2026-01-01) come from
// every Account it could be a Transfer with when none is named: the In
// transit leg on the wallet, and the pending rows of +1000.00 within a week
// on the card and the savings Account, closest first, the savings row
// although its month isn't open yet. Rows too far away, of another amount,
// on a loan, or Suspected duplicates aren't suggested (statement-transfers
// stories 9-11).
func TestSuggestionsComeFromEveryAccountTheRowCouldBeATransferWith(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	wallet := f.addCash(t, "Wallet", "EUR", "20.00")
	leg := f.recordLeg(t, wallet, f.account, "2026-01-01", "1000.00")
	card := f.addStatementAccount(t, "Card", "CREDIT_CARD", "2026-01-01", "Date;Description;Amount\n"+
		"02/01/2026;Payment;1.000,00\n09/01/2026;Too late;1.000,00\n03/01/2026;Other amount;999,00\n", "2999.00")
	savings := f.addStatementAccount(t, "Savings", "BANK_ACCOUNT", "2025-12-01",
		"Date;Description;Amount\n04/01/2026;Savings in;1.000,00\n", "1000.00")
	loan := f.addStatementAccount(t, "Loan", "LOAN", "2026-01-01",
		"Date;Description;Amount\n01/01/2026;Loan in;1.000,00\n", "-9000.00")

	// A second card file repeats the payment: a Suspected duplicate, which
	// isn't pending.
	card.upload(t, "", "Date;Description;Amount\n02/01/2026;PAYMENT;1.000,00\n", "2999.00", "").
		AssertJSON(t, http.StatusCreated, &map[string]any{})

	if n := len(card.suspected(t)); n != 1 {
		t.Fatalf("%d Suspected duplicates on the card, want 1", n)
	}

	all := f.suggested(t, "Rent", "")

	if len(all.Legs) != 1 || all.Legs[0].TransactionID != leg.TransactionID {
		t.Errorf("legs = %+v, want the wallet's leg", all.Legs)
	}

	rows := all.ImportedTransactions
	if len(rows) != 2 || rows[0].ImportedTransactionID != card.pending["Payment"].ImportedTransactionID ||
		rows[0].AccountID != card.account ||
		rows[1].ImportedTransactionID != savings.pending["Savings in"].ImportedTransactionID ||
		rows[1].AccountID != savings.account {
		t.Errorf("rows = %+v, want the card's Payment then the savings row, each naming its Account", rows)
	}

	only := f.suggested(t, "Rent", "?account_id="+card.account)
	if len(only.Legs) != 0 || len(only.ImportedTransactions) != 1 ||
		only.ImportedTransactions[0].Description != "Payment" {
		t.Errorf("suggestions on the card = %+v, want only its Payment", only)
	}

	cash := f.suggested(t, "Rent", "?account_id="+wallet)
	if len(cash.Legs) != 1 || len(cash.ImportedTransactions) != 0 {
		t.Errorf("suggestions on the wallet = %+v, want only its leg", cash)
	}

	reasonAndAccount(t, f.app.Get(t, "/api/households/"+f.household+"/imported-transactions/"+
		f.pending["Rent"].ImportedTransactionID+"/transfer-suggestions?account_id="+loan.account, f.token),
		"ACCOUNT_KIND_NOT_SUPPORTED", loan.account)
	reasonAndAccount(t, f.app.Get(t, "/api/households/"+f.household+"/imported-transactions/"+
		loan.pending["Loan in"].ImportedTransactionID+"/transfer-suggestions", f.token),
		"ACCOUNT_KIND_NOT_SUPPORTED", loan.account)
}
