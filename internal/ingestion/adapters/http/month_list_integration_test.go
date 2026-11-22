//go:build integration

package http_test

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// tenthOfMarch is when the months list tests run: January and February
// 2026 have ended, and March is the current calendar month.
func tenthOfMarch() apitest.Option {
	return apitest.WithClock(func() time.Time { return time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC) })
}

type accountMonthList struct {
	Items []accountMonth `json:"items"`
}

func (f importFixture) months(t *testing.T) []accountMonth {
	t.Helper()

	var l accountMonthList
	f.app.Get(t, f.accountPath()+"/months", f.token).AssertJSON(t, http.StatusOK, &l)

	return l.Items
}

// assertMonths checks the listed months, newest first, by month and state,
// and that each is listed exactly as getAccountMonth returns it.
func (f importFixture) assertMonths(t *testing.T, want ...string) []accountMonth {
	t.Helper()

	got := f.months(t)

	var listed []string
	for _, m := range got {
		listed = append(listed, m.BookingMonth+" "+m.State)

		if one := f.month(t, m.BookingMonth); !reflect.DeepEqual(one, m) {
			t.Errorf("listed %s = %+v, want it as getAccountMonth returns it: %+v", m.BookingMonth, m, one)
		}
	}

	if !reflect.DeepEqual(listed, want) {
		t.Errorf("months = %v, want %v", listed, want)
	}

	return got
}

// An Account lists its months from its creation: the start date's month
// is open and the months up to the current one aren't open yet. A month
// before the start date's isn't one of its months (spec story 17).
func TestAnAccountsMonthsFromItsCreation(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t, tenthOfMarch()), "alice")

	months := f.assertMonths(t, "2026-03 NOT_OPEN_YET", "2026-02 NOT_OPEN_YET", "2026-01 OPEN")

	for _, m := range months {
		if m.ComputedBalance != nil || !m.ReopenBlocked {
			t.Errorf("%s = %+v, want no balance and Reopen blocked", m.BookingMonth, m)
		}
	}

	f.app.Get(t, f.monthPath("2025-12"), f.token).AssertProblem(t, http.StatusNotFound, "about:blank")
	f.app.Get(t, "/api/households/"+f.household+"/accounts/"+f.connection+"/months", f.token).
		AssertProblem(t, http.StatusNotFound, "about:blank")
}

// A row booked after the current month extends the list to its month; once
// it is Discarded, by voiding its import, it no longer does (spec story
// 17).
func TestAFutureDatedRowExtendsTheMonths(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t, tenthOfMarch()), "alice")

	var later statementImport
	f.upload(t, "", "Date;Description;Amount\n05/05/2026;Insurance;-30,00\n", "2470.00", "").
		AssertJSON(t, http.StatusCreated, &later)

	months := f.assertMonths(t, "2026-05 NOT_OPEN_YET", "2026-04 NOT_OPEN_YET", "2026-03 NOT_OPEN_YET",
		"2026-02 NOT_OPEN_YET", "2026-01 OPEN")
	assertMonth(t, months[0], "NOT_OPEN_YET", 1, 0, 0)
	assertMonth(t, months[1], "NOT_OPEN_YET", 0, 0, 0)

	f.voided(t, later.StatementImportID)

	f.assertMonths(t, "2026-03 NOT_OPEN_YET", "2026-02 NOT_OPEN_YET", "2026-01 OPEN")
}

// Closing January makes it the one month that can be reopened, until an
// Accept in February, the Open month, blocks it (spec story 18).
func TestReopenBlockedFlipsWithAnAcceptInTheOpenMonth(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t, tenthOfMarch()), "alice")
	housing := f.createCategory(t, "Housing", "EXPENSE")
	f.acceptAll(t, housing, "Rent", "Salary")
	f.closed(t, "2026-01", "2510.50")

	months := f.assertMonths(t, "2026-03 NOT_OPEN_YET", "2026-02 OPEN", "2026-01 CLOSED")
	if months[2].ReopenBlocked || !months[1].ReopenBlocked || !months[0].ReopenBlocked {
		t.Errorf("months = %+v, want only January reopenable", months)
	}

	if months[2].StatementBalance == nil || months[2].ClosedAt == "" || months[1].StatementBalance != nil {
		t.Errorf("months = %+v, want only January with its Statement balance", months)
	}

	f.acceptAll(t, housing, "Coffee")

	if months = f.assertMonths(t, "2026-03 NOT_OPEN_YET", "2026-02 OPEN", "2026-01 CLOSED"); !months[2].ReopenBlocked {
		t.Errorf("january = %+v, want its Reopen blocked by February's Accepted Transaction", months[2])
	}
}

// An import whose Balance check waits blocks the months from its earliest
// row onwards, and not the ones before (spec story 19).
func TestAnUncheckedImportBlocksItsMonthsOnwards(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t, tenthOfMarch()), "alice")

	var si statementImport
	f.upload(t, "", "Date;Description;Amount\n02/02/2026;coffee;-10,50\n", "2500.00", "").
		AssertJSON(t, http.StatusCreated, &si)

	months := f.assertMonths(t, "2026-03 NOT_OPEN_YET", "2026-02 NOT_OPEN_YET", "2026-01 OPEN")

	want := []uncheckedImport{{StatementImportID: si.StatementImportID, BalanceCheck: "WAITING"}}
	for _, m := range months[:2] {
		if !reflect.DeepEqual(m.UncheckedStatementImports, want) {
			t.Errorf("%s unchecked = %+v, want the waiting import", m.BookingMonth, m.UncheckedStatementImports)
		}
	}

	if got := months[2].UncheckedStatementImports; len(got) != 0 {
		t.Errorf("january unchecked = %+v, want none", got)
	}

	assertMonth(t, months[1], "NOT_OPEN_YET", 1, 1, 0)
}
