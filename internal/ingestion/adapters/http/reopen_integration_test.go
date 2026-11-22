//go:build integration

package http_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

// marchLunch is a later file with a March row. With every earlier row
// counted the bank's balance is 2500.00 - 20.00 = 2480.00.
const marchLunch = "Date;Description;Amount\n05/03/2026;Lunch;-20,00\n"

// monthReopenProblem is a refused Reopen.
type monthReopenProblem struct {
	apitest.ProblemBody
	Reasons      []string `json:"reasons"`
	BookingMonth string   `json:"booking_month"`
	OpenMonth    string   `json:"open_month"`
	Accepted     *int     `json:"accepted"`
}

func (f importFixture) reopen(t *testing.T, month string) *apitest.Response {
	t.Helper()

	return f.app.Do(t, http.MethodPost, f.monthPath(month)+"/reopen", f.token, nil)
}

func (f importFixture) reopened(t *testing.T, month string) accountMonth {
	t.Helper()

	var m accountMonth
	f.reopen(t, month).AssertJSON(t, http.StatusOK, &m)

	return m
}

// refusedReopen reopens the month expecting it refused, and returns why.
func (f importFixture) refusedReopen(t *testing.T, month string) monthReopenProblem {
	t.Helper()

	res := f.reopen(t, month)
	res.AssertProblem(t, http.StatusConflict, "/problems/month-reopen-refused")

	var p monthReopenProblem
	if err := json.Unmarshal(res.Body, &p); err != nil {
		t.Fatalf("problem = %s (%v)", res.Body, err)
	}

	if p.Title != "Month can't be reopened" || p.BookingMonth != month {
		t.Errorf("problem = %+v, want %s named", p, month)
	}

	return p
}

// twoMonthsClosed is the acceptFixture with January and February
// reviewed and closed: March is the Open month, with nothing in it.
func twoMonthsClosed(t *testing.T, app *apitest.App, subject string) acceptFixture {
	t.Helper()

	f, _, _ := januaryClosed(t, app, subject)
	f.acceptAll(t, f.createCategory(t, "Treats", "EXPENSE"), "Coffee")
	f.closed(t, "2026-02", "2500.00")

	return f
}

// The latest closed month reopens while the Open month has no Accepted
// Transactions: it is open again without its Statement balance, the month
// after it isn't open yet, and it closes again with its Statement balance.
// Months reopen from the latest back, and an Accepted Transaction in the
// Open month blocks Reopen until it is reversed (spec stories 7, 8, 9).
func TestReopeningTheLatestClosedMonth(t *testing.T) {
	t.Parallel()
	f := twoMonthsClosed(t, apitest.Start(t), "alice")

	if got := f.month(t, "2026-02"); got.ReopenBlocked {
		t.Errorf("february = %+v, want it reopenable", got)
	}

	if got := f.month(t, "2026-01"); !got.ReopenBlocked {
		t.Errorf("january = %+v, want its Reopen blocked: February is the latest closed month", got)
	}

	p := f.refusedReopen(t, "2026-01")
	if !reflect.DeepEqual(p.Reasons, []string{"NOT_LATEST_CLOSED"}) || p.OpenMonth != "2026-03" ||
		p.Accepted != nil || p.Detail != "Only February 2026, the latest closed month, can be reopened." {
		t.Errorf("problem = %+v", p)
	}

	// An Accepted Transaction in March blocks Reopen of February.
	f.upload(t, "", marchLunch, "2480.00", "").AssertJSON(t, http.StatusCreated, &statementImport{})

	for _, i := range f.imported(t, "?state=PENDING").Items {
		f.pending[i.Description] = i
	}

	lunch := f.accepted(t, "Lunch", map[string]any{"category_id": f.createCategory(t, "Eating out", "EXPENSE")})

	p = f.refusedReopen(t, "2026-02")
	if !reflect.DeepEqual(p.Reasons, []string{"OPEN_MONTH_HAS_ACCEPTED_TRANSACTIONS"}) || p.OpenMonth != "2026-03" ||
		p.Accepted == nil || *p.Accepted != 1 ||
		p.Detail != "March 2026 has 1 Accepted Transaction. Reverse it before reopening February 2026." {
		t.Errorf("problem = %+v", p)
	}

	if got := f.month(t, "2026-02"); !got.ReopenBlocked {
		t.Errorf("february = %+v, want its Reopen blocked by March's Accepted Transaction", got)
	}

	// Every reason at once: March isn't closed, and has an Accepted
	// Transaction.
	p = f.refusedReopen(t, "2026-03")
	if !reflect.DeepEqual(p.Reasons, []string{"NOT_CLOSED", "OPEN_MONTH_HAS_ACCEPTED_TRANSACTIONS"}) ||
		p.Detail != "March 2026 isn't closed, so it can't be reopened. "+
			"March 2026 has 1 Accepted Transaction. Reverse it before reopening March 2026." {
		t.Errorf("problem = %+v", p)
	}

	f.reversed(t, lunch.TransactionID)

	february := f.reopened(t, "2026-02")
	assertMonth(t, february, "OPEN", 0, 0, 0)

	if february.StatementBalance != nil || february.ClosedAt != "" || !february.ReopenBlocked {
		t.Errorf("february = %+v, want it open, without its Statement balance", february)
	}

	if got := f.month(t, "2026-02"); !reflect.DeepEqual(got, february) {
		t.Errorf("february = %+v, want it as the Reopen returned it: %+v", got, february)
	}

	// March isn't open yet again, so its Lunch, back to pending, waits.
	assertMonth(t, f.month(t, "2026-03"), "NOT_OPEN_YET", 1, 0, 0)
	f.accept(t, "Lunch", nil).AssertProblem(t, http.StatusConflict, "/problems/month-not-open-yet")

	var account struct {
		OpenMonth string `json:"open_month"`
	}
	f.app.Get(t, f.accountPath(), f.token).AssertJSON(t, http.StatusOK, &account)

	if account.OpenMonth != "2026-02" {
		t.Errorf("account open month = %q, want 2026-02", account.OpenMonth)
	}

	// January is now the latest closed month, and February has the coffee.
	p = f.refusedReopen(t, "2026-01")
	if !reflect.DeepEqual(p.Reasons, []string{"OPEN_MONTH_HAS_ACCEPTED_TRANSACTIONS"}) || p.OpenMonth != "2026-02" {
		t.Errorf("problem = %+v", p)
	}

	assertMonth(t, f.closed(t, "2026-02", "2500.00"), "CLOSED", 0, 0, 0)
	assertMonth(t, f.month(t, "2026-03"), "OPEN", 1, 0, 0)
}

// Retrying a Reopen with its Idempotency-Key returns the original result,
// and doesn't reopen the month before it.
func TestRetryingAReopenWithItsIdempotencyKeyReopensOnce(t *testing.T) {
	t.Parallel()
	f := twoMonthsClosed(t, apitest.Start(t), "alice")

	header := http.Header{"Idempotency-Key": {"reopen-1"}}
	path := f.monthPath("2026-02") + "/reopen"

	first := f.app.DoWithHeader(t, http.MethodPost, path, f.token, header, nil)
	if first.Status != http.StatusOK {
		t.Fatalf("first reopen: %d %s", first.Status, first.Body)
	}

	retry := f.app.DoWithHeader(t, http.MethodPost, path, f.token, header, nil)
	if retry.Status != http.StatusOK || string(retry.Body) != string(first.Body) {
		t.Errorf("retry = %d %s, want the original %s", retry.Status, retry.Body, first.Body)
	}

	assertMonth(t, f.month(t, "2026-01"), "CLOSED", 0, 0, 0)
	assertMonth(t, f.month(t, "2026-02"), "OPEN", 0, 0, 0)
}

// A Reopen names the month it is about; an unknown Account or another
// Household's is not found, and a month written wrong is a bad request.
func TestReopenIsRefused(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)
	f := newAcceptFixture(t, app, "alice")
	bob := newAcceptFixture(t, app, "bob")

	p := f.refusedReopen(t, "2026-01")
	if !reflect.DeepEqual(p.Reasons, []string{"NOT_CLOSED"}) || p.OpenMonth != "2026-01" ||
		p.Detail != "January 2026 isn't closed, so it can't be reopened." {
		t.Errorf("problem = %+v", p)
	}

	f.reopen(t, "2026-13").AssertProblem(t, http.StatusBadRequest, "about:blank")

	// Bob through Alice's Household, and through his own with her Account.
	for _, household := range []string{f.household, bob.household} {
		path := "/api/households/" + household + "/accounts/" + f.account + "/months/2026-01/reopen"
		app.Do(t, http.MethodPost, path, bob.token, nil).AssertProblem(t, http.StatusNotFound, "about:blank")
	}
}
