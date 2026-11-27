package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	coffeeRow       = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000b1")
	coffeeBooked    = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000b2")
	salaryBooked    = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000b3")
	eatingOutBucket = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000b4")
	jan20           = time.Date(2026, time.January, 20, 0, 0, 0, 0, time.UTC)
)

// An Accepted Imported transaction is recorded with its amount, dates,
// description and payee exactly, and links back to it (spec story 51a).
func TestRecordingAnAcceptedImportedTransaction(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Transaction).
		When(domain.RecordTransaction{
			ImportedTransactionID: coffeeRow,
			BookingDate:           jan20,
			TransactionDate:       jan20.AddDate(0, 0, -1),
			ValueDate:             jan20.AddDate(0, 0, 1),
			Description:           "Coffee",
			Payee:                 "Café",
			Postings: []domain.Posting{
				{AccountID: mainAccount, Amount: dec("-12.30"), Currency: "EUR"},
				{CategoryID: eatingOutBucket, Amount: dec("12.30"), Currency: "EUR"},
			},
		}).
		Then(domain.TransactionRecorded{
			ImportedTransactionID: coffeeRow,
			BookingDate:           "2026-01-20",
			TransactionDate:       "2026-01-19",
			ValueDate:             "2026-01-21",
			Description:           "Coffee",
			Payee:                 "Café",
			Postings: []domain.Posting{
				{AccountID: mainAccount, Amount: dec("-12.30"), Currency: "EUR"},
				{CategoryID: eatingOutBucket, Amount: dec("12.30"), Currency: "EUR"},
			},
		})

	s := decidertest.Given(t, domain.Transaction, domain.TransactionEvent(domain.TransactionRecorded{
		ImportedTransactionID: coffeeRow, BookingDate: "2026-01-20", TransactionDate: "2026-01-19",
		Description: "Coffee", Payee: "Café",
		Postings: []domain.Posting{
			{AccountID: mainAccount, Amount: dec("-12.30"), Currency: "EUR"},
			{CategoryID: eatingOutBucket, Amount: dec("12.30"), Currency: "EUR"},
		},
	})).State()

	if s.ImportedTransactionID != coffeeRow || s.Payee != "Café" {
		t.Errorf("state = %+v, want it linked to its Imported transaction", s)
	}
}

func opened() []domain.AccountEvent {
	return []domain.AccountEvent{
		mainAdded,
		domain.OpeningBalanceRecorded{TransactionID: openingTransaction, Amount: dec("100.00")},
	}
}

// Booking a Transaction on an Account moves its Booking month's balance and,
// when the other side is Uncategorised, counts it there.
func TestBookingATransactionOnAnAccount(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Account, opened()...).
		When(domain.BookTransaction{
			TransactionID: coffeeBooked, BookingDate: jan20, Amount: dec("-12.30"), Uncategorised: true,
		}).
		Then(domain.TransactionBooked{
			TransactionID: coffeeBooked, BookingDate: "2026-01-20", Amount: dec("-12.30"), Uncategorised: true,
		})

	s := decidertest.Given(t, domain.Account, append(opened(),
		domain.TransactionBooked{TransactionID: coffeeBooked, BookingDate: "2026-01-20", Amount: dec("-12.30"), Uncategorised: true},
		domain.TransactionBooked{TransactionID: salaryBooked, BookingDate: "2026-02-01", Amount: dec("3000"), Uncategorised: true},
		domain.TransactionBooked{TransactionID: salaryBooked, BookingDate: "2026-02-03", Amount: dec("-5"), Uncategorised: false},
	)...).State()

	if !s.Balance().Equal(dec("3082.70")) {
		t.Errorf("balance = %s, want 3082.70", s.Balance())
	}

	if !s.Months["2026-01"].Equal(dec("87.70")) || !s.Months["2026-02"].Equal(dec("2995")) {
		t.Errorf("months = %v, want 2026-01 87.70 and 2026-02 2995", s.Months)
	}

	if s.Uncategorised["2026-01"] != 1 || s.Uncategorised["2026-02"] != 1 || len(s.Uncategorised) != 2 {
		t.Errorf("uncategorised = %v, want one in each of 2026-01 and 2026-02", s.Uncategorised)
	}
}

func TestBookingATransactionRefusals(t *testing.T) {
	t.Parallel()

	book := domain.BookTransaction{TransactionID: coffeeBooked, BookingDate: jan20, Amount: dec("-12.30")}

	decidertest.Given(t, domain.Account).When(book).ThenError(domain.ErrAccountNotAdded)
	decidertest.Given(t, domain.Account, mainAdded).When(book).ThenError(domain.ErrAwaitingFirstStatementImport)

	tooPrecise := book
	tooPrecise.Amount = dec("0.001")
	decidertest.Given(t, domain.Account, opened()...).When(tooPrecise).ThenError(domain.ErrTooManyDecimalPlaces)

	// mainAdded starts on 2026-01-15.
	early := book
	early.BookingDate = time.Date(2026, time.January, 14, 0, 0, 0, 0, time.UTC)
	decidertest.Given(t, domain.Account, opened()...).When(early).ThenError(domain.ErrBookedBeforeStartDate)

	// The refusal names both dates, so it can say when the history begins.
	_, err := domain.Account.Decide(decidertest.Given(t, domain.Account, opened()...).State(), early)
	if e, ok := errors.AsType[*domain.StartDateError](err); !ok ||
		e.BookingDate.Format(domain.DateLayout) != "2026-01-14" || e.StartDate.Format(domain.DateLayout) != "2026-01-15" {
		t.Errorf("err = %v, want a *StartDateError for 2026-01-14 before 2026-01-15", err)
	}

	noDate := book
	noDate.BookingDate = time.Time{}
	decidertest.Given(t, domain.Account, opened()...).When(noDate).ThenError(domain.ErrBookingDateRequired)
}
