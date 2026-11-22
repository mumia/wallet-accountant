//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// The acceptFixture's months: January holds the Opening balance (510.50),
// Rent (-1000.00) and Salary (3000.00), closing at 2510.50 once both are
// Accepted; February holds Coffee (-10.50), closing at 2500.00.

// accountMonth is an Account's month status.
type accountMonth struct {
	AccountID           string `json:"account_id"`
	BookingMonth        string `json:"booking_month"`
	State               string `json:"state"`
	StatementBalance    *money `json:"statement_balance"`
	ClosedAt            string `json:"closed_at"`
	ComputedBalance     *money `json:"computed_balance"`
	Pending             int    `json:"pending"`
	SuspectedDuplicates int    `json:"suspected_duplicates"`
	Uncategorised       int    `json:"uncategorised"`
	ReopenBlocked       bool   `json:"reopen_blocked"`

	UncheckedStatementImports []uncheckedImport `json:"unchecked_statement_imports"`
}

type uncheckedImport struct {
	StatementImportID string `json:"statement_import_id"`
	BalanceCheck      string `json:"balance_check"`
}

// monthCloseProblem is a refused Month close.
type monthCloseProblem struct {
	Reasons                   []string          `json:"reasons"`
	Pending                   int               `json:"pending"`
	SuspectedDuplicates       int               `json:"suspected_duplicates"`
	Uncategorised             int               `json:"uncategorised"`
	UncheckedStatementImports []uncheckedImport `json:"unchecked_statement_imports"`
	ComputedBalance           *money            `json:"computed_balance"`
	StatementBalance          *money            `json:"statement_balance"`
	Difference                *money            `json:"difference"`
}

func (f importFixture) monthPath(month string) string {
	return f.accountPath() + "/months/" + month
}

func (f importFixture) month(t *testing.T, month string) accountMonth {
	t.Helper()

	var m accountMonth
	f.app.Get(t, f.monthPath(month), f.token).AssertJSON(t, http.StatusOK, &m)

	return m
}

func statementBalance(amount, currency string) map[string]any {
	return map[string]any{"statement_balance": map[string]any{"amount": json.Number(amount), "currency": currency}}
}

func (f importFixture) closeMonth(t *testing.T, month, balance string) *apitest.Response {
	t.Helper()

	return f.app.Post(t, f.monthPath(month)+"/close", f.token, statementBalance(balance, "EUR"))
}

func (f importFixture) closed(t *testing.T, month, balance string) accountMonth {
	t.Helper()

	var m accountMonth
	f.closeMonth(t, month, balance).AssertJSON(t, http.StatusOK, &m)

	return m
}

// refusedClose closes the month expecting it refused, and returns why.
func (f importFixture) refusedClose(t *testing.T, month, balance string) monthCloseProblem {
	t.Helper()

	res := f.closeMonth(t, month, balance)
	res.AssertProblem(t, http.StatusUnprocessableEntity, "/problems/month-close-refused")

	var p monthCloseProblem
	if err := json.Unmarshal(res.Body, &p); err != nil {
		t.Fatalf("problem = %s (%v)", res.Body, err)
	}

	return p
}

// monthProblem is a 409 about a month.
type monthProblem struct {
	apitest.ProblemBody
	BookingMonth string `json:"booking_month"`
	OpenMonth    string `json:"open_month"`
}

// monthConflict expects res to be a 409 of the problem type, and returns it.
func monthConflict(t *testing.T, res *apitest.Response, problemType string) monthProblem {
	t.Helper()

	res.AssertProblem(t, http.StatusConflict, problemType)

	var p monthProblem
	if err := json.Unmarshal(res.Body, &p); err != nil {
		t.Fatalf("problem = %s (%v)", res.Body, err)
	}

	return p
}

// acceptAll Accepts the pending rows with the descriptions into the
// Category.
func (f acceptFixture) acceptAll(t *testing.T, category string, descriptions ...string) {
	t.Helper()

	for _, d := range descriptions {
		f.accepted(t, d, map[string]any{"category_id": category})
	}
}

// assertMonth checks a month's state and counts.
func assertMonth(t *testing.T, got accountMonth, state string, pending, suspected, uncategorised int) {
	t.Helper()

	if got.State != state || got.Pending != pending || got.SuspectedDuplicates != suspected ||
		got.Uncategorised != uncategorised {
		t.Errorf("%s = %+v, want %s with %d pending, %d suspected duplicates, %d uncategorised",
			got.BookingMonth, got, state, pending, suspected, uncategorised)
	}
}

func assertReasons(t *testing.T, got monthCloseProblem, want ...string) {
	t.Helper()

	if !reflect.DeepEqual(got.Reasons, want) {
		t.Errorf("reasons = %v, want %v (problem %+v)", got.Reasons, want, got)
	}
}

// A month's status shows what is left to do: its pending Imported
// transactions and its Uncategorised count follow every Accept,
// recategorisation and reversal, and a reversed Transaction and its
// reversal never count (spec stories 51, 51a, 61).
func TestAMonthsStatusFollowsAcceptRecategoriseAndReverse(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	january := f.month(t, "2026-01")
	assertMonth(t, january, "OPEN", 2, 0, 0)

	if january.AccountID != f.account || january.BookingMonth != "2026-01" || january.StatementBalance != nil ||
		january.ClosedAt != "" || january.ComputedBalance == nil {
		t.Fatalf("january = %+v", january)
	}

	assertMoney(t, "computed balance", *january.ComputedBalance, "510.50")

	rent := f.accepted(t, "Rent", nil)
	salary := f.accepted(t, "Salary", nil)

	january = f.month(t, "2026-01")
	assertMonth(t, january, "OPEN", 0, 0, 2)
	assertMoney(t, "computed balance", *january.ComputedBalance, "2510.50")

	f.setCategory(t, salary.TransactionID, f.createCategory(t, "Wages", "INCOME")).AssertJSON(t, http.StatusOK, &transaction{})
	assertMonth(t, f.month(t, "2026-01"), "OPEN", 0, 0, 1)

	f.reversed(t, rent.TransactionID)

	january = f.month(t, "2026-01")
	assertMonth(t, january, "OPEN", 1, 0, 0)
	assertMoney(t, "computed balance", *january.ComputedBalance, "3510.50")

	february := f.month(t, "2026-02")
	// Only the start date's month is open until it closes (ADR-0024).
	assertMonth(t, february, "NOT_OPEN_YET", 1, 0, 0)
	assertMoney(t, "computed balance", *february.ComputedBalance, "3510.50")

	// A month with nothing in it yet.
	assertMonth(t, f.month(t, "2026-05"), "NOT_OPEN_YET", 0, 0, 0)
}

// An Account awaiting its first Statement import has no computed balance.
func TestAMonthOfAnAccountAwaitingItsFirstImportHasNoBalance(t *testing.T) {
	t.Parallel()
	f := newImportFixture(t, apitest.Start(t), "alice")

	january := f.month(t, "2026-01")
	assertMonth(t, january, "OPEN", 0, 0, 0)

	if january.ComputedBalance != nil {
		t.Errorf("january = %+v, want no computed balance", january)
	}

	f.closeMonth(t, "2026-01", "0.00").
		AssertProblem(t, http.StatusConflict, "/problems/account-awaiting-first-statement-import")
}

// A reviewed month closes with its Statement balance, and nothing is
// booked to do it; closing it again is refused (spec stories 53, 54, 59,
// 64).
func TestClosingAMonth(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Rent", "Salary")

	before := f.transactions(t, "")

	january := f.closed(t, "2026-01", "2510.50")
	assertMonth(t, january, "CLOSED", 0, 0, 0)

	if january.StatementBalance == nil || january.ComputedBalance == nil || january.ClosedAt == "" {
		t.Fatalf("closed january = %+v", january)
	}

	assertMoney(t, "statement balance", *january.StatementBalance, "2510.50")
	assertMoney(t, "computed balance", *january.ComputedBalance, "2510.50")

	if got := f.month(t, "2026-01"); !reflect.DeepEqual(got, january) {
		t.Errorf("january = %+v, want it as the close returned it: %+v", got, january)
	}

	if after := f.transactions(t, ""); !sameJSON(t, after, before) {
		t.Errorf("transactions = %+v, want them untouched by the close: %+v", after.Items, before.Items)
	}

	f.assertBalance(t, "2510.50")

	f.closeMonth(t, "2026-01", "2510.50").AssertProblem(t, http.StatusConflict, "/problems/month-already-closed")

	p := monthConflict(t, f.closeMonth(t, "2026-01", "1.00"), "/problems/month-already-closed")
	if p.BookingMonth != "2026-01" || p.OpenMonth != "2026-02" {
		t.Errorf("problem = %+v, want January named and February open", p)
	}

	if got := f.month(t, "2026-01"); !reflect.DeepEqual(got, january) {
		t.Errorf("january = %+v, want its Statement balance kept: %+v", got, january)
	}

	// Closing January opened February (spec story 2).
	assertMonth(t, f.month(t, "2026-02"), "OPEN", 1, 0, 0)
	assertMonth(t, f.month(t, "2026-03"), "NOT_OPEN_YET", 0, 0, 0)
}

// Months close one at a time, in order: February can't close while
// January is open, and the refusal names January; once January closes,
// February closes and March opens (spec stories 1, 2, 4).
func TestMonthsCloseInOrder(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	housing := f.createCategory(t, "Housing", "EXPENSE")
	f.acceptAll(t, housing, "Rent", "Salary")

	p := monthConflict(t, f.closeMonth(t, "2026-02", "2500.00"), "/problems/month-not-open-yet")
	if p.Title != "Month not open yet" || p.Detail != "February 2026 isn't open yet. Close January 2026 first." ||
		p.BookingMonth != "2026-02" || p.OpenMonth != "2026-01" {
		t.Errorf("problem = %+v", p)
	}

	assertMonth(t, f.month(t, "2026-01"), "OPEN", 0, 0, 0)
	assertMonth(t, f.month(t, "2026-02"), "NOT_OPEN_YET", 1, 0, 0)

	f.closed(t, "2026-01", "2510.50")
	f.acceptAll(t, housing, "Coffee")
	assertMonth(t, f.closed(t, "2026-02", "2500.00"), "CLOSED", 0, 0, 0)
	assertMonth(t, f.month(t, "2026-03"), "OPEN", 0, 0, 0)

	var account struct {
		OpenMonth string `json:"open_month"`
	}
	f.app.Get(t, f.accountPath(), f.token).AssertJSON(t, http.StatusOK, &account)

	if account.OpenMonth != "2026-03" {
		t.Errorf("account open month = %q, want 2026-03", account.OpenMonth)
	}
}

// The Open month closes only once it has ended, from the first day of the
// next month by the server's clock in UTC (spec story 3).
func TestTheOpenMonthClosesOnlyOnceItHasEnded(t *testing.T) {
	t.Parallel()

	var now atomic.Pointer[time.Time]

	set := func(d time.Time) { now.Store(&d) }
	set(time.Date(2026, time.January, 31, 23, 59, 59, 0, time.UTC))

	f := newAcceptFixture(t, apitest.Start(t, apitest.WithClock(func() time.Time { return *now.Load() })), "alice")
	f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Rent", "Salary")

	p := monthConflict(t, f.closeMonth(t, "2026-01", "2510.50"), "/problems/month-not-ended")
	if p.Title != "Month not ended" || p.Detail != "January 2026 can be closed from 1 February 2026." ||
		p.BookingMonth != "2026-01" || p.OpenMonth != "2026-01" {
		t.Errorf("problem = %+v", p)
	}

	assertMonth(t, f.month(t, "2026-01"), "OPEN", 0, 0, 0)

	set(time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC))
	assertMonth(t, f.closed(t, "2026-01", "2510.50"), "CLOSED", 0, 0, 0)
}

// Each close condition on its own refuses the close, naming only itself
// (spec stories 55, 56, 57).
func TestEachCloseConditionRefusesTheClose(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	t.Run("pending", func(t *testing.T) {
		t.Parallel()
		f := newAcceptFixture(t, app, "pending")

		p := f.refusedClose(t, "2026-01", "510.50")
		assertReasons(t, p, "PENDING")

		if p.Pending != 2 {
			t.Errorf("problem = %+v, want two pending", p)
		}
	})

	t.Run("uncategorised", func(t *testing.T) {
		t.Parallel()
		f := newAcceptFixture(t, app, "uncategorised")
		f.accepted(t, "Rent", nil)
		f.accepted(t, "Salary", nil)

		p := f.refusedClose(t, "2026-01", "2510.50")
		assertReasons(t, p, "UNCATEGORISED")

		if p.Uncategorised != 2 {
			t.Errorf("problem = %+v, want two in Uncategorised", p)
		}
	})

	t.Run("balance mismatch", func(t *testing.T) {
		t.Parallel()
		f := newAcceptFixture(t, app, "mismatch")
		f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Rent", "Salary")

		p := f.refusedClose(t, "2026-01", "2500.00")
		assertReasons(t, p, "BALANCE_MISMATCH")

		if p.ComputedBalance == nil || p.StatementBalance == nil || p.Difference == nil {
			t.Fatalf("problem = %+v, want both amounts and the difference", p)
		}

		assertMoney(t, "computed balance", *p.ComputedBalance, "2510.50")
		assertMoney(t, "statement balance", *p.StatementBalance, "2500.00")
		assertMoney(t, "difference", *p.Difference, "-10.50")

		// No balancing entry was booked.
		if got := f.transactions(t, ""); len(got.Items) != 3 {
			t.Errorf("transactions = %+v, want the Opening balance, Rent and Salary only", got.Items)
		}

		assertMonth(t, f.month(t, "2026-01"), "OPEN", 0, 0, 0)
	})
}

// laterInJanuary follows statement: a new January row, and February's
// coffee again, which only looks like the first file's. If the coffee is a
// repeat the bank's balance is 2500.00 + 100.00 = 2600.00.
const laterInJanuary = "Date;Description;Amount\n" +
	"20/01/2026;Bonus;100,00\n" +
	"02/02/2026;coffee;-10,50\n"

// A Statement import with rows in January whose Balance check waits for a
// February Suspected duplicate stops January closing, until the duplicate
// is decided and the check passes (spec story 58a).
func TestAWaitingBalanceCheckRefusesTheClose(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", laterInJanuary, "2600.00", "").AssertJSON(t, http.StatusCreated, &si)

	for _, i := range f.imported(t, "?state=PENDING").Items {
		f.pending[i.Description] = i
	}

	f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Rent", "Salary", "Bonus")

	assertMonth(t, f.month(t, "2026-01"), "OPEN", 0, 0, 0)
	assertMonth(t, f.month(t, "2026-02"), "NOT_OPEN_YET", 1, 1, 0)

	p := f.refusedClose(t, "2026-01", "2610.50")
	assertReasons(t, p, "BALANCE_CHECK_NOT_PASSED")

	want := []uncheckedImport{{StatementImportID: si.StatementImportID, BalanceCheck: "WAITING"}}
	if !reflect.DeepEqual(p.UncheckedStatementImports, want) {
		t.Errorf("unchecked imports = %+v, want %+v", p.UncheckedStatementImports, want)
	}

	f.decide(t, f.suspected(t)["coffee"].ImportedTransactionID, "discard", "DISCARDED")

	if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "PASSED" {
		t.Fatalf("check = %q, want passed", got.BalanceCheck)
	}

	assertMonth(t, f.closed(t, "2026-01", "2610.50"), "CLOSED", 0, 0, 0)
}

// A failed Balance check stops every month the import has rows in or
// after; voiding the import lifts it (spec stories 58a, 60).
func TestAFailedBalanceCheckRefusesTheCloseUntilItsImportIsVoided(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", overlapWithoutIDs, "2454.90", "").AssertJSON(t, http.StatusCreated, &si)

	suspected := f.suspected(t)
	f.decide(t, suspected["coffee"].ImportedTransactionID, "keep", "PENDING")
	f.decide(t, suspected["SALARY"].ImportedTransactionID, "discard", "DISCARDED")

	if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "FAILED" {
		t.Fatalf("check = %q, want failed", got.BalanceCheck)
	}

	f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Rent", "Salary")

	p := f.refusedClose(t, "2026-01", "2510.50")
	assertReasons(t, p, "BALANCE_CHECK_NOT_PASSED")

	want := []uncheckedImport{{StatementImportID: si.StatementImportID, BalanceCheck: "FAILED"}}
	if !reflect.DeepEqual(p.UncheckedStatementImports, want) {
		t.Errorf("unchecked imports = %+v, want %+v", p.UncheckedStatementImports, want)
	}

	f.voided(t, si.StatementImportID)

	assertMonth(t, f.closed(t, "2026-01", "2510.50"), "CLOSED", 0, 0, 0)
}

// An import whose rows are all after a month doesn't stop that month
// closing, however its check stands (spec story 58a).
func TestAnImportWithRowsOnlyAfterTheMonthDoesntRefuseTheClose(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", "Date;Description;Amount\n02/02/2026;coffee;-10,50\n", "2500.00", "").
		AssertJSON(t, http.StatusCreated, &si)

	if si.BalanceCheck != "WAITING" {
		t.Fatalf("import = %+v, want its check waiting", si)
	}

	housing := f.createCategory(t, "Housing", "EXPENSE")
	f.acceptAll(t, housing, "Rent", "Salary")

	assertMonth(t, f.closed(t, "2026-01", "2510.50"), "CLOSED", 0, 0, 0)
	f.acceptAll(t, housing, "Coffee")

	// February, which the import has rows in, is stopped by it.
	p := f.refusedClose(t, "2026-02", "2500.00")
	assertReasons(t, p, "SUSPECTED_DUPLICATES", "BALANCE_CHECK_NOT_PASSED")
}

// A voided import never stops a close, even with its check still waiting.
func TestAVoidedImportsWaitingCheckDoesntRefuseTheClose(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", overlapWithoutIDs, "2454.90", "").AssertJSON(t, http.StatusCreated, &si)
	f.voided(t, si.StatementImportID)

	if got := f.importNamed(t, si.StatementImportID); got.BalanceCheck != "WAITING" {
		t.Fatalf("check = %q, want the voided import still waiting", got.BalanceCheck)
	}

	f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Rent", "Salary")

	assertMonth(t, f.closed(t, "2026-01", "2510.50"), "CLOSED", 0, 0, 0)
}

// A refused close lists every failed condition at once (spec story 58).
func TestEveryFailedConditionIsListedTogether(t *testing.T) {
	t.Parallel()
	f := newSuspectedFixture(t, apitest.Start(t), "alice")

	var si statementImport
	f.upload(t, "", overlapWithoutIDs, "2454.90", "").AssertJSON(t, http.StatusCreated, &si)

	f.accepted(t, "Rent", nil)

	p := f.refusedClose(t, "2026-01", "0.00")
	assertReasons(t, p, "PENDING", "SUSPECTED_DUPLICATES", "UNCATEGORISED", "BALANCE_CHECK_NOT_PASSED", "BALANCE_MISMATCH")

	if p.Pending != 1 || p.SuspectedDuplicates != 1 || p.Uncategorised != 1 ||
		!reflect.DeepEqual(p.UncheckedStatementImports, []uncheckedImport{{si.StatementImportID, "WAITING"}}) ||
		p.ComputedBalance == nil || p.StatementBalance == nil || p.Difference == nil {
		t.Fatalf("problem = %+v", p)
	}

	assertMoney(t, "computed balance", *p.ComputedBalance, "-489.50")
	assertMoney(t, "statement balance", *p.StatementBalance, "0.00")
	assertMoney(t, "difference", *p.Difference, "489.50")

	assertMonth(t, f.month(t, "2026-01"), "OPEN", 1, 1, 1)
}

func TestClosingAMonthIsRefused(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")

	f.app.Post(t, f.monthPath("2026-01")+"/close", f.token, statementBalance("2510.50", "USD")).
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-month-close")
	f.closeMonth(t, "2026-01", "2510.505").
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-month-close")
	f.closeMonth(t, "2025-12", "0.00").
		AssertProblem(t, http.StatusUnprocessableEntity, "/problems/invalid-month-close")

	// A month that hasn't started yet isn't open yet either.
	next := time.Now().UTC().AddDate(0, 1, 0).Format("2006-01")
	monthConflict(t, f.closeMonth(t, next, "2500.00"), "/problems/month-not-open-yet")
	assertMonth(t, f.month(t, next), "NOT_OPEN_YET", 0, 0, 0)

	f.closeMonth(t, "2026-13", "0.00").AssertProblem(t, http.StatusBadRequest, "about:blank")
	f.app.Get(t, f.monthPath("26-01"), f.token).AssertProblem(t, http.StatusBadRequest, "about:blank")

	unknown := "/api/households/" + f.household + "/accounts/0198f0a0-0000-7000-8000-000000000999/months/2026-01"
	f.app.Get(t, unknown, f.token).AssertProblem(t, http.StatusNotFound, "about:blank")
	f.app.Post(t, unknown+"/close", f.token, statementBalance("0.00", "EUR")).
		AssertProblem(t, http.StatusNotFound, "about:blank")
}

// Another Household's months are 404, through its Household or one's own.
func TestAnotherHouseholdsMonthsAre404(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	alice := newAcceptFixture(t, app, "alice")
	bob := newAcceptFixture(t, app, "bob")
	alice.acceptAll(t, alice.createCategory(t, "Housing", "EXPENSE"), "Rent", "Salary")

	for _, household := range []string{alice.household, bob.household} {
		path := "/api/households/" + household + "/accounts/" + alice.account + "/months/2026-01"
		app.Get(t, path, bob.token).AssertProblem(t, http.StatusNotFound, "about:blank")
		app.Post(t, path+"/close", bob.token, statementBalance("2510.50", "EUR")).
			AssertProblem(t, http.StatusNotFound, "about:blank")
		app.Get(t, "/api/households/"+household+"/accounts/"+alice.account+"/months", bob.token).
			AssertProblem(t, http.StatusNotFound, "about:blank")
	}

	assertMonth(t, alice.month(t, "2026-01"), "OPEN", 0, 0, 0)
}

// Retrying a close with its Idempotency-Key replays it.
func TestRetryingACloseWithItsIdempotencyKeyReplaysIt(t *testing.T) {
	t.Parallel()
	f := newAcceptFixture(t, apitest.Start(t), "alice")
	f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Rent", "Salary")

	first := f.app.PostWithKey(t, f.monthPath("2026-01")+"/close", f.token, "close-1", statementBalance("2510.50", "EUR"))
	if first.Status != http.StatusOK {
		t.Fatalf("first close: %d %s", first.Status, first.Body)
	}

	retry := f.app.PostWithKey(t, f.monthPath("2026-01")+"/close", f.token, "close-1", statementBalance("2510.50", "EUR"))
	if retry.Status != http.StatusOK || string(retry.Body) != string(first.Body) {
		t.Errorf("retry = %d %s, want the original %s", retry.Status, retry.Body, first.Body)
	}
}

// A closed month is rebuilt the same from the events.
func TestAClosedMonthSurvivesARebuild(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newAcceptFixture(t, app, "alice")
	f.acceptAll(t, f.createCategory(t, "Housing", "EXPENSE"), "Rent", "Salary")
	f.closed(t, "2026-01", "2510.50")
	f.accepted(t, "Coffee", nil)

	before := []accountMonth{f.month(t, "2026-01"), f.month(t, "2026-02")}

	app.Rebuild(t)

	after := []accountMonth{f.month(t, "2026-01"), f.month(t, "2026-02")}
	if !reflect.DeepEqual(after, before) {
		t.Errorf("after rebuild:\n  %+v\nbefore:\n  %+v", after, before)
	}

	f.closeMonth(t, "2026-01", "2510.50").AssertProblem(t, http.StatusConflict, "/problems/month-already-closed")
}
