//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

type accountLine struct {
	TransactionID           string `json:"transaction_id"`
	Amount                  money  `json:"amount"`
	BookingDate             string `json:"booking_date"`
	Description             string `json:"description"`
	CategoryName            string `json:"category_name"`
	ReversalOfTransactionID string `json:"reversal_of_transaction_id"`
	ReversedByTransactionID string `json:"reversed_by_transaction_id"`
}

type accountLinePage struct {
	Items []accountLine `json:"items"`
}

func eur(amount string) map[string]any {
	return map[string]any{"amount": json.Number(amount), "currency": "EUR"}
}

// addWallet adds a cash Account with the Opening count typed twice.
func addWallet(t *testing.T, app *apitest.App, token, household, count string) account {
	t.Helper()

	var created account
	app.Post(t, "/api/households/"+household+"/accounts", token, map[string]any{
		"name": "Wallet", "kind": "CASH", "currency": "EUR", "start_date": "2026-01-01",
		"opening_count": eur(count), "opening_count_confirmation": eur(count),
	}).AssertJSON(t, http.StatusCreated, &created)

	return created
}

func accountLines(t *testing.T, app *apitest.App, token, household, account string) []accountLine {
	t.Helper()

	var page accountLinePage
	app.Get(t, "/api/households/"+household+"/accounts/"+account+"/transactions", token).
		AssertJSON(t, http.StatusOK, &page)

	return page.Items
}

// A cash Account's Opening balance comes from its Opening count at once, so
// it never awaits a Statement import (ADR-0018; spec stories 1 and 2).
func TestAddingACashAccountRecordsItsOpeningCount(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)

	wallet := addWallet(t, app, token, household, "40.00")

	if wallet.Kind != "CASH" || wallet.AwaitingFirstStatementImport || wallet.Balance == nil ||
		wallet.Balance.Amount != "40.00" || wallet.Balance.Currency != "EUR" {
		t.Fatalf("added = %+v, want a cash Account with a balance of EUR 40.00", wallet)
	}

	var got account
	app.Get(t, "/api/households/"+household+"/accounts/"+wallet.AccountID, token).AssertJSON(t, http.StatusOK, &got)

	if got.Balance == nil || got.Balance.Amount != "40.00" {
		t.Errorf("GET account = %+v, want a balance of 40.00", got)
	}

	lines := accountLines(t, app, token, household, wallet.AccountID)
	if len(lines) != 1 || lines[0].Amount.Amount != "40.00" || lines[0].BookingDate != "2026-01-01" ||
		lines[0].CategoryName != "Opening balance" {
		t.Errorf("transactions = %+v, want the Opening balance of 40.00 on the start date", lines)
	}

	empty := addWallet(t, app, token, household, "0")
	if empty.Balance == nil || empty.Balance.Amount != "0.00" {
		t.Errorf("an empty wallet = %+v, want a balance of 0.00", empty)
	}
}

func TestAddingACashAccountIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	path := "/api/households/" + household + "/accounts"

	cash := func(fields map[string]any) map[string]any {
		body := map[string]any{"name": "Wallet", "kind": "CASH", "currency": "EUR", "start_date": "2026-01-01"}
		for k, v := range fields {
			body[k] = v
		}

		return body
	}

	for name, tc := range map[string]struct {
		body    map[string]any
		problem string
	}{
		"no counts":        {cash(nil), "/problems/invalid-account"},
		"no confirmation":  {cash(map[string]any{"opening_count": eur("40.00")}), "/problems/invalid-account"},
		"counts differ":    {cash(map[string]any{"opening_count": eur("40.00"), "opening_count_confirmation": eur("4.00")}), "/problems/opening-count-mismatch"},
		"another currency": {cash(map[string]any{"opening_count": map[string]any{"amount": json.Number("40.00"), "currency": "USD"}, "opening_count_confirmation": map[string]any{"amount": json.Number("40.00"), "currency": "USD"}}), "/problems/invalid-account"},
		"too precise":      {cash(map[string]any{"opening_count": eur("40.001"), "opening_count_confirmation": eur("40.001")}), "/problems/invalid-account"},
		"statement Account with a count": {map[string]any{
			"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
			"opening_count": eur("40.00"), "opening_count_confirmation": eur("40.00"),
		}, "/problems/invalid-account"},
	} {
		t.Run(name, func(t *testing.T) {
			app.Post(t, path, token, tc.body).AssertProblem(t, http.StatusUnprocessableEntity, tc.problem)
		})
	}

	var got accountList
	app.Get(t, path, token).AssertJSON(t, http.StatusOK, &got)

	if len(got.Items) != 0 {
		t.Errorf("after refusals: %+v, want no Account", got.Items)
	}
}

// Recounting reverses the Opening balance Transaction and records a new
// one, while the start month is open (spec story 3).
func TestRecountingACashAccountsOpeningCount(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")
	path := "/api/households/" + household + "/accounts/" + wallet.AccountID + "/opening-count"

	var recounted account
	app.Post(t, path, token, map[string]any{
		"opening_count": eur("35.50"), "opening_count_confirmation": eur("35.50"),
	}).AssertJSON(t, http.StatusOK, &recounted)

	if recounted.Balance == nil || recounted.Balance.Amount != "35.50" {
		t.Fatalf("recounted = %+v, want a balance of 35.50", recounted)
	}

	lines := accountLines(t, app, token, household, wallet.AccountID)
	if len(lines) != 3 {
		t.Fatalf("transactions = %+v, want the old Opening balance, its reversal and the new one", lines)
	}

	byID := map[string]accountLine{}
	for _, l := range lines {
		byID[l.TransactionID] = l
	}

	var reversal, renewed int

	for _, l := range lines {
		switch {
		case l.ReversalOfTransactionID != "":
			reversal++

			if l.Amount.Amount != "-40.00" || byID[l.ReversalOfTransactionID].ReversedByTransactionID != l.TransactionID {
				t.Errorf("reversal = %+v, want -40.00 linked both ways", l)
			}
		case l.ReversedByTransactionID == "":
			renewed++

			if l.Amount.Amount != "35.50" || l.BookingDate != "2026-01-01" {
				t.Errorf("new Opening balance = %+v, want 35.50 on the start date", l)
			}
		}
	}

	if reversal != 1 || renewed != 1 {
		t.Errorf("transactions = %+v, want one reversal and one new Opening balance", lines)
	}

	app.Post(t, path, token, map[string]any{
		"opening_count": eur("35.5"), "opening_count_confirmation": eur("35.50"),
	}).AssertJSON(t, http.StatusOK, &recounted)

	if n := len(accountLines(t, app, token, household, wallet.AccountID)); n != 3 || recounted.Balance.Amount != "35.50" {
		t.Errorf("the same count again: %d transactions and %+v, want nothing recorded", n, recounted.Balance)
	}
}

func TestRecountingAnOpeningCountIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	base := "/api/households/" + household + "/accounts/"
	wallet := addWallet(t, app, token, household, "40.00")
	main := addAccount(t, app, token, household, map[string]string{
		"name": "Main", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-01-01",
	})
	count := map[string]any{"opening_count": eur("35.50"), "opening_count_confirmation": eur("35.50")}

	app.Post(t, base+main.AccountID+"/opening-count", token, count).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/not-a-cash-account")
	app.Post(t, base+wallet.AccountID+"/opening-count", token, map[string]any{
		"opening_count": eur("35.50"), "opening_count_confirmation": eur("30.50"),
	}).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/opening-count-mismatch")
	app.Post(t, base+wallet.AccountID+"/opening-count", token, map[string]any{
		"opening_count": eur("35.505"), "opening_count_confirmation": eur("35.505"),
	}).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-account")
	app.Post(t, base+"0198f0a0-0000-7000-8000-000000000000/opening-count", token, count).
		AssertProblem(t, http.StatusNotFound, "about:blank")

	// Once the start month closes, the Opening count is locked (ADR-0024).
	app.Post(t, base+wallet.AccountID+"/months/2026-01/close", token, map[string]any{
		"statement_balance": eur("40.00"),
	}).AssertJSON(t, http.StatusOK, &map[string]any{})
	app.Post(t, base+wallet.AccountID+"/opening-count", token, count).
		AssertProblem(t, http.StatusConflict, "/problems/opening-count-locked")

	var got account
	app.Get(t, base+wallet.AccountID, token).AssertJSON(t, http.StatusOK, &got)

	if got.Balance == nil || got.Balance.Amount != "40.00" {
		t.Errorf("after refusals: %+v, want the balance still 40.00", got)
	}
}

// A cash Account is kept by hand and takes no Connection (spec story 4).
func TestACashAccountTakesNoConnection(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	token := app.Register(t, "https://issuer.example", "alice")
	household := newHousehold(t, app, token)
	wallet := addWallet(t, app, token, household, "40.00")

	app.Post(t, "/api/households/"+household+"/accounts/"+wallet.AccountID+"/connections", token, map[string]any{
		"name": "Statements", "kind": "STATEMENT_FILE_IMPORT",
	}).AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-connection")
}
