package domain_test

import (
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

// counted is a wallet whose January holds the Opening count (40.00) and a
// coffee (-2.50), closing at 37.50, and whose February holds lunch (-5.00)
// on the 2nd and a refund (+1.00) on the 5th.
func counted() []domain.AccountEvent {
	return append(walletWithCount,
		domain.TransactionBooked{TransactionID: coffeeBooked, BookingDate: "2026-01-20", Amount: dec("-2.50")},
		domain.TransactionBooked{TransactionID: salaryBooked, BookingDate: "2026-02-02", Amount: dec("-5.00")},
		domain.TransactionBooked{TransactionID: walletRecount, BookingDate: "2026-02-05", Amount: dec("1.00")},
	)
}

func day(d string) *time.Time {
	t, err := time.Parse(domain.DateLayout, d)
	if err != nil {
		panic(err)
	}

	return &t
}

// cashClose closes January by a count taken on the 3rd of February: by
// then lunch (-5.00) moved the balance, and the refund hadn't yet.
func cashClose(count string) domain.CloseMonth {
	return domain.CloseMonth{
		Today: closingDay, Month: january, StatementBalance: dec(count),
		CountedOn: day("2026-02-03"), MovedByCountedOn: dec("-5.00"),
	}
}

// A cash month closes by a count taken after it ends: the count is compared
// with the balance at the end of the day it was counted, and the month-end
// balance this proves is stored as the Statement balance (spec story 11).
func TestClosingACashMonthByACount(t *testing.T) {
	t.Parallel()

	closed := domain.MonthClosed{
		Month: "2026-01", StatementBalance: dec("37.50"), CountedOn: "2026-02-03", Count: new(dec("32.50")),
	}

	decidertest.Given(t, domain.Account, counted()...).When(cashClose("32.50")).Then(closed)

	s := decidertest.Given(t, domain.Account, append(counted(), closed)...).State()
	c := s.Closed["2026-01"]

	if !c.StatementBalance.Equal(dec("37.50")) || !c.Count.Equal(dec("32.50")) || !c.CountedOn.Equal(*day("2026-02-03")) ||
		s.OpenMonth != "2026-02" {
		t.Errorf("state = %+v, want January closed at 37.50 by a count of 32.50 on 2026-02-03", s)
	}

	// Counted today is fine.
	today := cashClose("33.50")
	today.CountedOn, today.MovedByCountedOn = day("2026-03-10"), dec("-4.00")
	decidertest.Given(t, domain.Account, counted()...).When(today).Then(domain.MonthClosed{
		Month: "2026-01", StatementBalance: dec("37.50"), CountedOn: "2026-03-10", Count: new(dec("33.50")),
	})
}

// A short count is refused with the gap between the books and the count
// (spec story 12).
func TestAShortCashCountIsRefusedWithTheGap(t *testing.T) {
	t.Parallel()

	refused := refusal(t, counted(), cashClose("30.00"))

	m := refused.Mismatch
	if m == nil || !m.Computed.Equal(dec("32.50")) || !m.Statement.Equal(dec("30.00")) ||
		!m.Difference.Equal(dec("-2.50")) {
		t.Errorf("mismatch = %+v, want computed 32.50, counted 30.00, difference -2.50", m)
	}
}

func TestClosingACashMonthRefusals(t *testing.T) {
	t.Parallel()

	for name, countedOn := range map[string]string{
		"on the month's last day": "2026-01-31",
		"inside the month":        "2026-01-15",
		"after today":             "2026-03-11",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := cashClose("32.50")
			c.CountedOn = day(countedOn)
			decidertest.Given(t, domain.Account, counted()...).When(c).ThenError(domain.ErrInvalidCountDate)
		})
	}

	noDay := cashClose("32.50")
	noDay.CountedOn = nil
	decidertest.Given(t, domain.Account, counted()...).When(noDay).ThenError(domain.ErrCountedOnRequired)

	statement := domain.CloseMonth{
		Today: closingDay, Month: january, StatementBalance: dec("87.70"), CountedOn: day("2026-02-03"),
	}
	decidertest.Given(t, domain.Account, reviewed()...).When(statement).
		ThenError(domain.ErrCountedOnNotForStatementAccount)
}

// A cash month reopens even when the Open month has Transactions, which
// stay; the month must be closed again with a new count (spec story 13).
func TestReopeningACashMonthWithTransactionsInTheOpenMonth(t *testing.T) {
	t.Parallel()

	closed := domain.MonthClosed{
		Month: "2026-01", StatementBalance: dec("37.50"), CountedOn: "2026-02-03", Count: new(dec("32.50")),
	}

	decidertest.Given(t, domain.Account, append(counted(), closed)...).
		When(domain.ReopenMonth{Month: january}).
		Then(domain.MonthReopened{Month: "2026-01"})

	// Only the latest close is taken back.
	february := domain.MonthClosed{
		Month: "2026-02", StatementBalance: dec("33.50"), CountedOn: "2026-03-01", Count: new(dec("33.50")),
	}
	s := decidertest.Given(t, domain.Account, append(counted(), closed, february)...).State()

	if r := s.ReopenRefusal("2026-01"); r == nil || !r.NotLatestClosed || r.Accepted != 0 {
		t.Errorf("refusal = %+v, want only NOT_LATEST_CLOSED", r)
	}
}
