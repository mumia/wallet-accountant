package domain

import (
	"errors"
	"fmt"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

var (
	// ErrOpeningCountRequired is returned for adding a cash Account without
	// its Opening count.
	ErrOpeningCountRequired = errors.New("a cash account needs its opening count")
	// ErrOpeningCountNotForStatementAccount is returned for an Opening
	// count sent with a statement Account, whose Opening balance only its
	// first Statement import works out (ADR-0018).
	ErrOpeningCountNotForStatementAccount = errors.New("only a cash account takes an opening count")
	// ErrNotACashAccount is returned for a command only a cash Account
	// takes, sent to a statement Account.
	ErrNotACashAccount = errors.New("not a cash account")
	// ErrOpeningCountLocked is returned for recounting the Opening count
	// once the start date's month has closed.
	ErrOpeningCountLocked = errors.New("the opening count can't change once the start month has closed")
)

// IsCash reports whether the Account is a cash Account, kept entirely by
// hand.
func (s AccountState) IsCash() bool { return s.Kind == AccountKindCash }

// RecountOpening replaces a cash Account's Opening balance with a new
// Opening count, Amount, recorded by the Transaction TransactionID. The
// current Opening balance Transaction, whose Posting on the Account is
// PreviousAmount, is reversed by ReversalID (ADR-0018).
type RecountOpening struct {
	TransactionID  sharedkernel.ID
	ReversalID     sharedkernel.ID
	PreviousAmount sharedkernel.Decimal
	Amount         sharedkernel.Decimal
}

func (RecountOpening) isAccountCommand() {}

// checkOpeningCount refuses an Opening count a cash Account lacks or a
// statement Account sends, and one with more decimal places than the
// currency has.
func checkOpeningCount(kind AccountKind, count *sharedkernel.Decimal, currency sharedkernel.Currency) error {
	switch {
	case kind != AccountKindCash && count != nil:
		return ErrOpeningCountNotForStatementAccount
	case kind == AccountKindCash && count == nil:
		return ErrOpeningCountRequired
	case count == nil:
		return nil
	}

	return checkDecimalPlaces(*count, currency)
}

func checkDecimalPlaces(amount sharedkernel.Decimal, currency sharedkernel.Currency) error {
	if !amount.Equal(amount.Round(int32(currency.Decimals()))) { //nolint:gosec // at most 18 places
		return fmt.Errorf("%w: %s %s", ErrTooManyDecimalPlaces, amount, currency.Code())
	}

	return nil
}

// recountOpening replaces a cash Account's Opening balance while its start
// date's month is still the Open month: the old Opening balance is reversed
// and the new one recorded, both on the start date. The same count again
// records nothing.
func recountOpening(s AccountState, c RecountOpening) ([]AccountEvent, error) {
	switch {
	case !s.Added:
		return nil, ErrAccountNotAdded
	case !s.IsCash():
		return nil, ErrNotACashAccount
	case s.StartDate.Format(MonthLayout) != s.OpenMonth:
		return nil, ErrOpeningCountLocked
	}

	currency, err := sharedkernel.ParseCurrency(s.Currency)
	if err != nil {
		return nil, err //nolint:wrapcheck // the shared kernel's own refusal
	}

	if err := checkDecimalPlaces(c.Amount, currency); err != nil {
		return nil, err
	}

	if c.Amount.Equal(c.PreviousAmount) {
		return nil, nil
	}

	return []AccountEvent{
		OpeningBalanceReversed{
			TransactionID: s.OpeningBalanceTransactionID, ReversalID: c.ReversalID, Amount: c.PreviousAmount.Neg(),
		},
		OpeningBalanceRecorded{TransactionID: c.TransactionID, Amount: c.Amount},
	}, nil
}
