package domain_test

import (
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	teaBooked     = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000d1")
	coffeeReverse = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000d2")
)

// januaryClosed is reviewed() with January closed at 87.70 and February
// closed at 3087.70.
func januaryClosed() []domain.AccountEvent {
	return append(reviewed(),
		domain.MonthClosed{Month: "2026-01", StatementBalance: dec("87.70")},
		domain.MonthClosed{Month: "2026-02", StatementBalance: dec("3087.70")},
	)
}

// Any change that touches a closed month is allowed but flags it (spec
// story 62). A change that moves the balance also moves the closing balance
// of every later month, so it flags every closed month from its own on; a
// recategorisation moves no balance and flags only its own month.
func TestAChangeInAClosedMonthFlagsIt(t *testing.T) {
	t.Parallel()

	jan25 := time.Date(2026, time.January, 25, 0, 0, 0, 0, time.UTC)

	for name, tc := range map[string]struct {
		cmd          domain.AccountCommand
		want         domain.AccountEvent
		movesBalance bool
	}{
		"booking": {
			cmd:          domain.BookTransaction{TransactionID: teaBooked, BookingDate: jan25, Amount: dec("-2.00")},
			want:         domain.TransactionBooked{TransactionID: teaBooked, BookingDate: "2026-01-25", Amount: dec("-2.00")},
			movesBalance: true,
		},
		"recategorisation": {
			cmd: domain.RecategoriseTransaction{TransactionID: coffeeBooked, BookingDate: jan20, Uncategorised: true},
			want: domain.TransactionRecategorised{
				TransactionID: coffeeBooked, BookingDate: "2026-01-20", Uncategorised: true,
			},
		},
		"reversal": {
			cmd: domain.BookReversal{
				TransactionID: coffeeBooked, ReversalID: coffeeReverse, BookingDate: jan20, Amount: dec("12.30"),
			},
			want: domain.ReversalBooked{
				TransactionID: coffeeBooked, ReversalID: coffeeReverse, BookingDate: "2026-01-20", Amount: dec("12.30"),
			},
			movesBalance: true,
		},
		"opening balance reversal": {
			cmd: domain.ReopenOpeningBalance{
				TransactionID: openingTransaction, ReversalID: openingReversal, Amount: dec("-100.00"),
			},
			want: domain.OpeningBalanceReversed{
				TransactionID: openingTransaction, ReversalID: openingReversal, Amount: dec("-100.00"),
			},
			movesBalance: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Account, januaryClosed()...).When(tc.cmd).Then(tc.want)

			s := decidertest.Given(t, domain.Account, append(januaryClosed(), tc.want)...).State()

			if !s.Closed["2026-01"].Flagged {
				t.Errorf("January = %+v, want it flagged", s.Closed["2026-01"])
			}

			if got := s.Closed["2026-02"].Flagged; got != tc.movesBalance {
				t.Errorf("February flagged = %t, want %t: only a change that moves January's balance moves February's closing balance",
					got, tc.movesBalance)
			}

			if !s.Closed["2026-01"].StatementBalance.Equal(dec("87.70")) || s.Closed["2026-01"].ClosedAt.IsZero() {
				t.Errorf("January = %+v, want its Month close kept", s.Closed["2026-01"])
			}
		})
	}
}

// A balance-moving change in a closed month leaves the earlier closed
// months alone: their closing balances don't include it.
func TestAChangeInALaterClosedMonthLeavesEarlierOnesUnflagged(t *testing.T) {
	t.Parallel()

	february := domain.TransactionBooked{TransactionID: teaBooked, BookingDate: "2026-02-10", Amount: dec("-2.00")}
	s := decidertest.Given(t, domain.Account, append(januaryClosed(), february)...).State()

	if s.Closed["2026-01"].Flagged || !s.Closed["2026-02"].Flagged {
		t.Errorf("closed = %+v, want only February flagged", s.Closed)
	}
}

// A change in an open month flags nothing, and a flag stays once set.
func TestAChangeInAnOpenMonthFlagsNothing(t *testing.T) {
	t.Parallel()

	march := domain.TransactionBooked{TransactionID: teaBooked, BookingDate: "2026-03-02", Amount: dec("-2.00")}
	s := decidertest.Given(t, domain.Account, append(januaryClosed(), march)...).State()

	if s.Closed["2026-01"].Flagged || s.Closed["2026-02"].Flagged {
		t.Errorf("closed = %+v, want nothing flagged", s.Closed)
	}

	if _, ok := s.Closed["2026-03"]; ok {
		t.Errorf("closed = %+v, want March still open", s.Closed)
	}

	// A month flagged before it closed would be meaningless: a change made
	// while open, then the close, leaves it unflagged.
	s = decidertest.Given(t, domain.Account, append(opened(),
		domain.TransactionBooked{TransactionID: coffeeBooked, BookingDate: "2026-01-20", Amount: dec("-12.30")},
		domain.MonthClosed{Month: "2026-01", StatementBalance: dec("87.70")},
	)...).State()

	if s.Closed["2026-01"].Flagged {
		t.Errorf("January = %+v, want it unflagged", s.Closed["2026-01"])
	}
}

// Voiding a Statement import whose pending rows were in a closed month
// changes that month's Imported transactions without booking anything, so
// it flags the month directly; an open or already flagged month records
// nothing.
func TestFlaggingAClosedMonth(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Account, januaryClosed()...).
		When(domain.FlagMonth{Month: jan20}).
		Then(domain.MonthFlagged{Month: "2026-01"})

	s := decidertest.Given(t, domain.Account, append(januaryClosed(), domain.MonthFlagged{Month: "2026-01"})...).State()
	if !s.Closed["2026-01"].Flagged || s.Closed["2026-02"].Flagged {
		t.Errorf("closed = %+v, want only January flagged", s.Closed)
	}

	decidertest.Given(t, domain.Account, append(januaryClosed(), domain.MonthFlagged{Month: "2026-01"})...).
		When(domain.FlagMonth{Month: jan20}).
		Then()

	march := time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)
	decidertest.Given(t, domain.Account, januaryClosed()...).
		When(domain.FlagMonth{Month: march}).
		Then()

	decidertest.Given(t, domain.Account).
		When(domain.FlagMonth{Month: jan20}).
		ThenError(domain.ErrAccountNotAdded)
}
