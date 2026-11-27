package domain_test

import (
	"errors"
	"testing"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

// An Expense Category takes only money out and an Income Category only
// money in; Internal Categories and a zero amount take either.
func TestACategoryKindMustFitTheAmountsDirection(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		kind   domain.Kind
		amount string
		want   domain.Direction // empty: it fits
	}{
		"expense on money out":  {kind: domain.KindExpense, amount: "-12.30"},
		"expense on money in":   {kind: domain.KindExpense, amount: "12.30", want: domain.DirectionIn},
		"expense on zero":       {kind: domain.KindExpense, amount: "0"},
		"income on money in":    {kind: domain.KindIncome, amount: "2500"},
		"income on money out":   {kind: domain.KindIncome, amount: "-2500", want: domain.DirectionOut},
		"income on zero":        {kind: domain.KindIncome, amount: "0.00"},
		"internal on money in":  {kind: domain.KindInternal, amount: "5"},
		"internal on money out": {kind: domain.KindInternal, amount: "-5"},
		"no kind on money in":   {amount: "5"},
		"no kind on money out":  {amount: "-5"},
		"expense on a cent in":  {kind: domain.KindExpense, amount: "0.01", want: domain.DirectionIn},
		"income on a cent out":  {kind: domain.KindIncome, amount: "-0.01", want: domain.DirectionOut},
		"internal on zero":      {kind: domain.KindInternal, amount: "0"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := domain.FitsDirection(tc.kind, dec(tc.amount))

			if tc.want == "" {
				if err != nil {
					t.Fatalf("FitsDirection(%s, %s) = %v, want it to fit", tc.kind, tc.amount, err)
				}

				return
			}

			refused, ok := errors.AsType[*domain.DirectionError](err)
			if !ok || !errors.Is(err, domain.ErrCategoryDirection) {
				t.Fatalf("FitsDirection(%s, %s) = %v, want a *DirectionError", tc.kind, tc.amount, err)
			}

			if refused.Kind != tc.kind || refused.Direction != tc.want {
				t.Errorf("refused %+v, want kind %s and direction %s", refused, tc.kind, tc.want)
			}
		})
	}
}

// Setting a Category checks its kind against the Account Posting's amount,
// not the Category Posting's, which carries the opposite sign.
func TestSettingACategoryOfTheWrongKind(t *testing.T) {
	t.Parallel()

	// The coffee is money out: -12.30 on the Account.
	decidertest.Given(t, domain.Transaction, coffeeRecorded()).
		When(domain.SetTransactionCategory{CategoryID: groceriesBucket, Kind: domain.KindExpense}).
		Then(domain.TransactionCategorySet{CategoryID: groceriesBucket, PreviousCategoryID: uncategorisedBox})

	decidertest.Given(t, domain.Transaction, coffeeRecorded()).
		When(domain.SetTransactionCategory{CategoryID: groceriesBucket, Kind: domain.KindIncome}).
		ThenError(domain.ErrCategoryDirection)

	// Uncategorised fits either way.
	decidertest.Given(t, domain.Transaction, coffeeRecorded(),
		domain.TransactionEvent(domain.TransactionCategorySet{CategoryID: groceriesBucket, PreviousCategoryID: uncategorisedBox})).
		When(domain.SetTransactionCategory{CategoryID: uncategorisedBox, Kind: domain.KindInternal}).
		Then(domain.TransactionCategorySet{CategoryID: uncategorisedBox, PreviousCategoryID: groceriesBucket})

	// A zero amount has no direction.
	zero := coffeeRecorded().(domain.TransactionRecorded)
	zero.Postings = []domain.Posting{
		{AccountID: mainAccount, Amount: dec("0.00"), Currency: "EUR"},
		{CategoryID: uncategorisedBox, Amount: dec("0.00"), Currency: "EUR"},
	}

	for _, kind := range []domain.Kind{domain.KindIncome, domain.KindExpense} {
		decidertest.Given(t, domain.Transaction, domain.TransactionEvent(zero)).
			When(domain.SetTransactionCategory{CategoryID: groceriesBucket, Kind: kind}).
			Then(domain.TransactionCategorySet{CategoryID: groceriesBucket, PreviousCategoryID: uncategorisedBox})
	}
}

// A Transaction that can't take any Category says so before the chosen
// kind is checked.
func TestATransactionsOwnRefusalComesBeforeTheDirection(t *testing.T) {
	t.Parallel()

	// Income on the coffee, which is money out, is the wrong kind.
	wrong := domain.SetTransactionCategory{CategoryID: groceriesBucket, Kind: domain.KindIncome}

	decidertest.Given(t, domain.Transaction, coffeeRecorded(),
		domain.TransactionEvent(domain.TransactionReversed{ReversalID: coffeeReversal})).
		When(wrong).ThenError(domain.ErrTransactionReversed)

	original := decidertest.Given(t, domain.Transaction, coffeeRecorded()).State()

	reversal, err := domain.Transaction.Decide(domain.Transaction.Initial, original.Reversal(coffeeBooked))
	if err != nil {
		t.Fatal(err)
	}

	// The reversal is money in, so Expense would be the wrong kind too.
	decidertest.Given(t, domain.Transaction, reversal...).
		When(domain.SetTransactionCategory{CategoryID: groceriesBucket, Kind: domain.KindExpense}).
		ThenError(domain.ErrTransactionIsAReversal)

	// The Opening balance of 100 is money in.
	decidertest.Given(t, domain.Transaction, openingRecorded(t)...).
		When(domain.SetTransactionCategory{CategoryID: groceriesBucket, Kind: domain.KindExpense}).
		ThenError(domain.ErrOpeningBalanceTransaction)
}
