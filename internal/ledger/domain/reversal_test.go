package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	coffeeReversal   = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000c1")
	anotherReversal  = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000c2")
	uncategorisedBox = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000c3")
	groceriesBucket  = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-0000000000c4")
)

// coffeeRecorded is an Accepted coffee in Uncategorised.
func coffeeRecorded() domain.TransactionEvent {
	return domain.TransactionRecorded{
		ImportedTransactionID: coffeeRow, BookingDate: "2026-01-20", TransactionDate: "2026-01-19",
		ValueDate: "2026-01-21", Description: "Coffee", Payee: "Café",
		Postings: []domain.Posting{
			{AccountID: mainAccount, Amount: dec("-12.30"), Currency: "EUR"},
			{CategoryID: uncategorisedBox, Amount: dec("12.30"), Currency: "EUR"},
		},
	}
}

func openingRecorded(t *testing.T) []domain.TransactionEvent {
	t.Helper()

	events, err := domain.Transaction.Decide(domain.Transaction.Initial, openingBalanceOf("100"))
	if err != nil {
		t.Fatal(err)
	}

	return events
}

// Reversing a Transaction records that it is reversed and by which
// Transaction (spec story 51b).
func TestReversingATransaction(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Transaction, coffeeRecorded()).
		When(domain.ReverseTransaction{ReversalID: coffeeReversal}).
		Then(domain.TransactionReversed{ReversalID: coffeeReversal})

	s := decidertest.Given(t, domain.Transaction, coffeeRecorded(),
		domain.TransactionEvent(domain.TransactionReversed{ReversalID: coffeeReversal})).State()

	if s.ReversedBy != coffeeReversal {
		t.Errorf("reversed by %s, want %s", s.ReversedBy, coffeeReversal)
	}
}

// The reversing Transaction has the opposite Postings, the same Booking
// date and a link to the original, but no link to the Imported transaction
// (spec story 51c).
func TestTheReversingTransactionHasTheOppositePostings(t *testing.T) {
	t.Parallel()

	original := decidertest.Given(t, domain.Transaction, coffeeRecorded()).State()

	decidertest.Given(t, domain.Transaction).
		When(original.Reversal(coffeeBooked)).
		Then(domain.TransactionRecorded{
			ReversalOf: coffeeBooked, BookingDate: "2026-01-20", TransactionDate: "2026-01-19",
			ValueDate: "2026-01-21", Description: "Coffee", Payee: "Café",
			Postings: []domain.Posting{
				{AccountID: mainAccount, Amount: dec("12.30"), Currency: "EUR"},
				{CategoryID: uncategorisedBox, Amount: dec("-12.30"), Currency: "EUR"},
			},
		})

	s := decidertest.Given(t, domain.Transaction, domain.TransactionEvent(domain.TransactionRecorded{
		ReversalOf: coffeeBooked, BookingDate: "2026-01-20", TransactionDate: "2026-01-19", Description: "Coffee",
		Postings: []domain.Posting{
			{AccountID: mainAccount, Amount: dec("12.30"), Currency: "EUR"},
			{CategoryID: uncategorisedBox, Amount: dec("-12.30"), Currency: "EUR"},
		},
	})).State()

	if s.ReversalOf != coffeeBooked || s.ImportedTransactionID != sharedkernel.NilID {
		t.Errorf("reversal = %+v, want it linked to %s and to no Imported transaction", s, coffeeBooked)
	}
}

// Corrections can't pile up or bypass voiding: the Opening balance
// Transaction, a reversal and a Transaction already reversed can't be
// reversed (spec story 51f).
func TestReversingRefusals(t *testing.T) {
	t.Parallel()

	reverse := domain.ReverseTransaction{ReversalID: anotherReversal}

	decidertest.Given(t, domain.Transaction).When(reverse).ThenError(domain.ErrTransactionNotRecorded)
	decidertest.Given(t, domain.Transaction, openingRecorded(t)...).
		When(reverse).ThenError(domain.ErrOpeningBalanceTransaction)

	original := decidertest.Given(t, domain.Transaction, coffeeRecorded()).State()
	reversal, err := domain.Transaction.Decide(domain.Transaction.Initial, original.Reversal(coffeeBooked))
	if err != nil {
		t.Fatal(err)
	}

	decidertest.Given(t, domain.Transaction, reversal...).When(reverse).ThenError(domain.ErrTransactionIsAReversal)
	decidertest.Given(t, domain.Transaction, coffeeRecorded(), domain.TransactionEvent(domain.TransactionReversed{ReversalID: coffeeReversal})).
		When(reverse).ThenError(domain.ErrTransactionAlreadyReversed)
	decidertest.Given(t, domain.Transaction, coffeeRecorded()).
		When(domain.ReverseTransaction{}).ThenError(domain.ErrReversalIDRequired)
}

// Recategorising sets the Category of the non-Account side and records the
// one it replaced (spec stories 50, 51).
func TestRecategorisingATransaction(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Transaction, coffeeRecorded()).
		When(domain.SetTransactionCategory{CategoryID: groceriesBucket}).
		Then(domain.TransactionCategorySet{CategoryID: groceriesBucket, PreviousCategoryID: uncategorisedBox})

	s := decidertest.Given(t, domain.Transaction, coffeeRecorded(),
		domain.TransactionEvent(domain.TransactionCategorySet{CategoryID: groceriesBucket, PreviousCategoryID: uncategorisedBox})).State()

	if s.CategoryID() != groceriesBucket {
		t.Errorf("category = %s, want %s", s.CategoryID(), groceriesBucket)
	}

	lines := s.AccountLines()
	if len(lines) != 1 || lines[0].CategoryID != groceriesBucket || !lines[0].Amount.Equal(dec("-12.30")) {
		t.Errorf("account lines = %+v, want the coffee in groceries, its amount unchanged", lines)
	}

	// The Category it already has changes nothing.
	decidertest.Given(t, domain.Transaction, coffeeRecorded()).
		When(domain.SetTransactionCategory{CategoryID: uncategorisedBox}).
		Then()
}

// A reversed Transaction and its reversal keep their Category, and so does
// the Opening balance Transaction (spec "Ledger: Transaction").
func TestRecategorisingRefusals(t *testing.T) {
	t.Parallel()

	set := domain.SetTransactionCategory{CategoryID: groceriesBucket}

	decidertest.Given(t, domain.Transaction).When(set).ThenError(domain.ErrTransactionNotRecorded)
	decidertest.Given(t, domain.Transaction, openingRecorded(t)...).When(set).ThenError(domain.ErrOpeningBalanceTransaction)
	decidertest.Given(t, domain.Transaction, coffeeRecorded(), domain.TransactionEvent(domain.TransactionReversed{ReversalID: coffeeReversal})).
		When(set).ThenError(domain.ErrTransactionReversed)

	original := decidertest.Given(t, domain.Transaction, coffeeRecorded()).State()
	reversal, err := domain.Transaction.Decide(domain.Transaction.Initial, original.Reversal(coffeeBooked))
	if err != nil {
		t.Fatal(err)
	}

	decidertest.Given(t, domain.Transaction, reversal...).When(set).ThenError(domain.ErrTransactionIsAReversal)
	decidertest.Given(t, domain.Transaction, coffeeRecorded()).
		When(domain.SetTransactionCategory{}).ThenError(domain.ErrCategoryRequired)

	split := coffeeRecorded().(domain.TransactionRecorded)
	split.Postings = []domain.Posting{
		{AccountID: mainAccount, Amount: dec("-12.30"), Currency: "EUR"},
		{CategoryID: uncategorisedBox, Amount: dec("10.30"), Currency: "EUR"},
		{CategoryID: groceriesBucket, Amount: dec("2.00"), Currency: "EUR"},
	}
	decidertest.Given(t, domain.Transaction, domain.TransactionEvent(split)).
		When(set).ThenError(domain.ErrNotOneCategory)
}

// A reversal moves the Account's balance back in the Booking month and,
// when the reversed Transaction was in Uncategorised, takes it out of the
// count: neither it nor its reversal count (spec story 51b).
func TestBookingAReversalOnAnAccount(t *testing.T) {
	t.Parallel()

	booked := append(opened(), domain.TransactionBooked{
		TransactionID: coffeeBooked, BookingDate: "2026-01-20", Amount: dec("-12.30"), Uncategorised: true,
	})

	decidertest.Given(t, domain.Account, booked...).
		When(domain.BookReversal{
			TransactionID: coffeeBooked, ReversalID: coffeeReversal, BookingDate: jan20, Amount: dec("12.30"),
			WasUncategorised: true,
		}).
		Then(domain.ReversalBooked{
			TransactionID: coffeeBooked, ReversalID: coffeeReversal, BookingDate: "2026-01-20", Amount: dec("12.30"),
			WasUncategorised: true,
		})

	s := decidertest.Given(t, domain.Account, append(booked, domain.ReversalBooked{
		TransactionID: coffeeBooked, ReversalID: coffeeReversal, BookingDate: "2026-01-20", Amount: dec("12.30"),
		WasUncategorised: true,
	})...).State()

	if !s.Balance().Equal(dec("100.00")) || !s.Months["2026-01"].Equal(dec("100.00")) {
		t.Errorf("balance = %s, months = %v, want 100.00 in January", s.Balance(), s.Months)
	}

	if len(s.Uncategorised) != 0 {
		t.Errorf("uncategorised = %v, want none", s.Uncategorised)
	}

	categorised := decidertest.Given(t, domain.Account, append(opened(),
		domain.TransactionBooked{TransactionID: coffeeBooked, BookingDate: "2026-01-20", Amount: dec("-12.30")},
		domain.TransactionBooked{TransactionID: salaryBooked, BookingDate: "2026-01-21", Amount: dec("5"), Uncategorised: true},
		domain.ReversalBooked{
			TransactionID: coffeeBooked, ReversalID: coffeeReversal, BookingDate: "2026-01-20", Amount: dec("12.30"),
		},
	)...).State()

	if categorised.Uncategorised["2026-01"] != 1 {
		t.Errorf("uncategorised = %v, want the other one still counted", categorised.Uncategorised)
	}
}

func TestBookingAReversalRefusals(t *testing.T) {
	t.Parallel()

	book := domain.BookReversal{TransactionID: coffeeBooked, ReversalID: coffeeReversal, BookingDate: jan20, Amount: dec("1")}

	decidertest.Given(t, domain.Account).When(book).ThenError(domain.ErrAccountNotAdded)

	tooPrecise := book
	tooPrecise.Amount = dec("0.001")
	decidertest.Given(t, domain.Account, opened()...).When(tooPrecise).ThenError(domain.ErrTooManyDecimalPlaces)
}

// Recategorising moves the Transaction in or out of the Booking month's
// Uncategorised count (spec stories 50, 51).
func TestRecategorisingOnAnAccount(t *testing.T) {
	t.Parallel()

	booked := append(opened(), domain.TransactionBooked{
		TransactionID: coffeeBooked, BookingDate: "2026-01-20", Amount: dec("-12.30"), Uncategorised: true,
	})

	decidertest.Given(t, domain.Account, booked...).
		When(domain.RecategoriseTransaction{TransactionID: coffeeBooked, BookingDate: jan20, WasUncategorised: true}).
		Then(domain.TransactionRecategorised{TransactionID: coffeeBooked, BookingDate: "2026-01-20", WasUncategorised: true})

	out := decidertest.Given(t, domain.Account, append(booked,
		domain.TransactionRecategorised{TransactionID: coffeeBooked, BookingDate: "2026-01-20", WasUncategorised: true},
	)...).State()

	if len(out.Uncategorised) != 0 || !out.Balance().Equal(dec("87.70")) {
		t.Errorf("uncategorised = %v, balance %s; want none and 87.70", out.Uncategorised, out.Balance())
	}

	back := decidertest.Given(t, domain.Account, append(booked,
		domain.TransactionRecategorised{TransactionID: coffeeBooked, BookingDate: "2026-01-20", WasUncategorised: true},
		domain.TransactionRecategorised{TransactionID: coffeeBooked, BookingDate: "2026-01-20", Uncategorised: true},
	)...).State()

	if back.Uncategorised["2026-01"] != 1 {
		t.Errorf("uncategorised = %v, want it counted again", back.Uncategorised)
	}

	same := decidertest.Given(t, domain.Account, append(booked,
		domain.TransactionRecategorised{
			TransactionID: coffeeBooked, BookingDate: "2026-01-20", WasUncategorised: true, Uncategorised: true,
		},
	)...).State()

	if same.Uncategorised["2026-01"] != 1 {
		t.Errorf("uncategorised = %v, want it still counted once", same.Uncategorised)
	}

	decidertest.Given(t, domain.Account).
		When(domain.RecategoriseTransaction{TransactionID: coffeeBooked, BookingDate: jan20}).
		ThenError(domain.ErrAccountNotAdded)
}
