//go:build integration

package http_test

import (
	"net/http"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

type accountOverview struct {
	AccountID           string `json:"account_id"`
	OpenMonth           string `json:"open_month"`
	Pending             int    `json:"pending"`
	SuspectedDuplicates int    `json:"suspected_duplicates"`
}

// overview returns every Account of the Household as listAccounts returns
// it, by ID, checking each against getAccount.
func (f importFixture) overview(t *testing.T) map[string]accountOverview {
	t.Helper()

	var list struct {
		Items []accountOverview `json:"items"`
	}

	f.app.Get(t, "/api/households/"+f.household+"/accounts", f.token).AssertJSON(t, http.StatusOK, &list)

	out := map[string]accountOverview{}

	for _, a := range list.Items {
		var one accountOverview
		f.app.Get(t, "/api/households/"+f.household+"/accounts/"+a.AccountID, f.token).
			AssertJSON(t, http.StatusOK, &one)

		if one != a {
			t.Errorf("listed %+v, want it as getAccount returns it: %+v", a, one)
		}

		out[a.AccountID] = a
	}

	return out
}

func assertOverview(t *testing.T, got accountOverview, openMonth string, pending, suspected int) {
	t.Helper()

	if got.OpenMonth != openMonth || got.Pending != pending || got.SuspectedDuplicates != suspected {
		t.Errorf("account %s = %+v, want open month %s with %d pending and %d suspected duplicates",
			got.AccountID, got, openMonth, pending, suspected)
	}
}

// Every Account carries its Open month and its pending and Suspected
// duplicate counts over every month, including months not open yet, and an
// Accept shows at once (spec story 16).
func TestAccountsCarryTheirOpenMonthAndCounts(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t, tenthOfMarch()), "alice")

	// A Suspected duplicate of February's Coffee, in a month not open yet.
	f.upload(t, "", "Date;Description;Amount\n02/02/2026;coffee;-10,50\n", "2500.00", "").
		AssertJSON(t, http.StatusCreated, &statementImport{})

	var savings struct {
		AccountID string `json:"account_id"`
	}

	f.app.Post(t, "/api/households/"+f.household+"/accounts", f.token, map[string]string{
		"name": "Savings", "kind": "BANK_ACCOUNT", "currency": "EUR", "start_date": "2026-02-01",
	}).AssertJSON(t, http.StatusCreated, &savings)

	other := importFixture{fixture: f.fixture}
	other.account = savings.AccountID
	other.connection = other.create(t, map[string]any{
		"name": "Statement file", "kind": "STATEMENT_FILE_IMPORT", "csv_mapping": signed(),
	}).ConnectionID

	got := f.overview(t)
	assertOverview(t, got[f.account], "2026-01", 3, 1)
	assertOverview(t, got[savings.AccountID], "2026-02", 0, 0)

	other.confirm(t, "Date;Description;Amount\n05/02/2026;Interest;1,00\n", "101.00")

	got = f.overview(t)
	assertOverview(t, got[f.account], "2026-01", 3, 1)
	assertOverview(t, got[savings.AccountID], "2026-02", 1, 0)

	f.accepted(t, "Rent", nil)

	assertOverview(t, f.overview(t)[f.account], "2026-01", 2, 1)
}
