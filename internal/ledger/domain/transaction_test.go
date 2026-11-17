package domain_test

import (
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var (
	mainAccount     = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000001")
	savingsAccount  = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000002")
	openingCategory = sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000003")
)

func dec(s string) sharedkernel.Decimal {
	d, err := sharedkernel.ParseDecimal(s)
	if err != nil {
		panic(err)
	}

	return d
}

func openingBalanceOf(amount string) domain.RecordTransaction {
	return domain.RecordTransaction{
		OpeningBalance:  true,
		BookingDate:     startDate,
		TransactionDate: startDate,
		Description:     "Opening balance",
		Postings: []domain.Posting{
			{AccountID: mainAccount, Amount: dec(amount), Currency: "EUR"},
			{CategoryID: openingCategory, Amount: dec(amount).Neg(), Currency: "EUR"},
		},
	}
}

func TestRecordingABalancedTransaction(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Transaction).
		When(openingBalanceOf("1234.50")).
		Then(domain.TransactionRecorded{
			OpeningBalance:  true,
			BookingDate:     "2026-01-01",
			TransactionDate: "2026-01-01",
			Description:     "Opening balance",
			Postings: []domain.Posting{
				{AccountID: mainAccount, Amount: dec("1234.50"), Currency: "EUR"},
				{CategoryID: openingCategory, Amount: dec("-1234.50"), Currency: "EUR"},
			},
		})
}

func TestAZeroOpeningBalanceIsStillATransaction(t *testing.T) {
	t.Parallel()

	events, err := domain.Transaction.Decide(domain.Transaction.Initial, openingBalanceOf("0"))
	if err != nil || len(events) != 1 {
		t.Fatalf("got %+v, %v; want one recorded Transaction", events, err)
	}
}

func TestPostingsMustSumToZeroPerCurrency(t *testing.T) {
	t.Parallel()

	for name, postings := range map[string][]domain.Posting{
		"off by a cent": {
			{AccountID: mainAccount, Amount: dec("10.00"), Currency: "EUR"},
			{CategoryID: openingCategory, Amount: dec("-9.99"), Currency: "EUR"},
		},
		"balanced in total but not per currency": {
			{AccountID: mainAccount, Amount: dec("10"), Currency: "EUR"},
			{AccountID: savingsAccount, Amount: dec("-10"), Currency: "USD"},
			{CategoryID: openingCategory, Amount: dec("-10"), Currency: "EUR"},
			{CategoryID: openingCategory, Amount: dec("10"), Currency: "USD"},
			{AccountID: mainAccount, Amount: dec("1"), Currency: "EUR"},
			{AccountID: savingsAccount, Amount: dec("-1"), Currency: "USD"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := openingBalanceOf("10")
			c.Postings = postings

			decidertest.Given(t, domain.Transaction).When(c).ThenError(domain.ErrUnbalancedPostings)
		})
	}
}

func TestEveryPostingNamesOneAccountOrOneCategory(t *testing.T) {
	t.Parallel()

	for name, p := range map[string]domain.Posting{
		"neither":  {Amount: dec("5"), Currency: "EUR"},
		"both":     {AccountID: mainAccount, CategoryID: openingCategory, Amount: dec("5"), Currency: "EUR"},
		"no money": {AccountID: mainAccount, Amount: dec("5")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := openingBalanceOf("5")
			c.Postings[0] = p

			decidertest.Given(t, domain.Transaction).When(c).ThenError(domain.ErrInvalidPosting)
		})
	}
}

func TestATransactionNeedsTwoPostingsAndABookingDate(t *testing.T) {
	t.Parallel()

	one := openingBalanceOf("0")
	one.Postings = one.Postings[:1]
	decidertest.Given(t, domain.Transaction).When(one).ThenError(domain.ErrTooFewPostings)

	undated := openingBalanceOf("5")
	undated.BookingDate = time.Time{}
	decidertest.Given(t, domain.Transaction).When(undated).ThenError(domain.ErrBookingDateRequired)
}

func TestAnAmountMustFitItsCurrencysDecimalPlaces(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Transaction).When(openingBalanceOf("1.005")).ThenError(domain.ErrInvalidPosting)
}

func TestATransactionIsRecordedOnce(t *testing.T) {
	t.Parallel()

	recorded, err := domain.Transaction.Decide(domain.Transaction.Initial, openingBalanceOf("5"))
	if err != nil {
		t.Fatal(err)
	}

	decidertest.Given(t, domain.Transaction, recorded...).
		When(openingBalanceOf("5")).
		ThenError(domain.ErrTransactionAlreadyRecorded)
}

// Its fold names each Account Posting with the Category on the other side,
// which is what the Account's single-entry list shows (ADR-0001).
func TestTheFoldListsEachAccountPostingWithItsCategory(t *testing.T) {
	t.Parallel()

	recorded, err := domain.Transaction.Decide(domain.Transaction.Initial, openingBalanceOf("42.10"))
	if err != nil {
		t.Fatal(err)
	}

	s := decidertest.Given(t, domain.Transaction, recorded...).State()

	lines := s.AccountLines()
	if len(lines) != 1 {
		t.Fatalf("account lines = %+v, want one", lines)
	}

	l := lines[0]
	if l.AccountID != mainAccount || l.CategoryID != openingCategory || !l.Amount.Equal(dec("42.10")) ||
		l.Currency != "EUR" || !l.BookingDate.Equal(startDate) || !l.TransactionDate.Equal(startDate) {
		t.Errorf("account line = %+v", l)
	}
}
