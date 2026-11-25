package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

var (
	// ErrOpeningCountMismatch is returned when a cash Account's Opening
	// count and its confirmation differ.
	ErrOpeningCountMismatch = errors.New("the opening count and its confirmation differ")
	// ErrOpeningCountRequired is returned for a cash Account without both
	// its Opening count and its confirmation.
	ErrOpeningCountRequired = domain.ErrOpeningCountRequired
	// ErrOpeningCountNotForStatementAccount is returned for an Opening
	// count sent with a statement Account.
	ErrOpeningCountNotForStatementAccount = domain.ErrOpeningCountNotForStatementAccount
	// ErrNotACashAccount is returned for a command only a cash Account
	// takes.
	ErrNotACashAccount = domain.ErrNotACashAccount
	// ErrOpeningCountLocked is returned for recounting once the start month
	// has closed.
	ErrOpeningCountLocked = domain.ErrOpeningCountLocked
	// ErrZeroAmount is returned for a cash Transaction that moves nothing.
	ErrZeroAmount = errors.New("a cash transaction needs an amount other than zero")
	// ErrDateInTheFuture is returned for a cash Transaction dated after
	// today (UTC): planned money belongs to Planning, not the Ledger.
	ErrDateInTheFuture = errors.New("a cash transaction can't be dated after today")
	// ErrBookedBeforeStartDate is returned for a cash Transaction dated
	// before the Account's start date.
	ErrBookedBeforeStartDate = domain.ErrBookedBeforeStartDate
)

// Count is an amount of cash counted, in a currency.
type Count struct {
	Amount   sharedkernel.Decimal
	Currency string
}

// OpeningBalances records and replaces Opening balance Transactions, in the
// caller's transaction. Transactions implements it.
type OpeningBalances interface {
	// RecordOpeningBalance records the Account's Opening balance and its
	// Transaction on the start date.
	RecordOpeningBalance(ctx context.Context, tx *eventstore.Tx, account, transaction uuid.UUID,
		amount sharedkernel.Decimal) error
	// RecountOpening replaces a cash Account's Opening balance.
	RecountOpening(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, amount sharedkernel.Decimal) error
}

// openingCount checks a cash Account's two Opening counts against each
// other and the Account currency, and returns the one to record: nil when
// neither was sent. A cash Account without both is ErrOpeningCountRequired.
func openingCount(cash bool, currency string, count, confirmation *Count) (*sharedkernel.Decimal, error) {
	if cash && (count == nil || confirmation == nil) {
		return nil, ErrOpeningCountRequired
	}

	for _, c := range []*Count{count, confirmation} {
		if c != nil && c.Currency != currency {
			return nil, fmt.Errorf("%w: opening count in %s, account in %s", ErrCurrencyMismatch, c.Currency, currency)
		}
	}

	switch {
	case count != nil && confirmation != nil && !count.Amount.Equal(confirmation.Amount):
		return nil, ErrOpeningCountMismatch
	case count != nil:
		return &count.Amount, nil
	case confirmation != nil:
		return &confirmation.Amount, nil
	default:
		return nil, nil
	}
}

// RecountOpening replaces a cash Account's Opening count while its start
// date's month is still the Open month, and returns the Account (ADR-0018;
// consistency map: Recount opening). The two counts must agree.
func (a *Accounts) RecountOpening(ctx context.Context, household, account uuid.UUID, count, confirmation Count,
) (Account, error) {
	var recounted Account

	err := a.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := a.accounts.Folds.Load(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !found {
			return ErrAccountNotFound
		}

		if !fold.State.IsCash() {
			return ErrNotACashAccount
		}

		amount, err := openingCount(true, fold.State.Currency, &count, &confirmation)
		if err != nil {
			return err
		}

		if err := a.openings.RecountOpening(ctx, tx, account, *amount); err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		recounted, err = a.AccountForImport(ctx, tx, account)

		return err
	})
	if err != nil {
		return Account{}, fmt.Errorf("recount the opening of account %s: %w", account, err)
	}

	return recounted, nil
}

// RecountOpening replaces a cash Account's Opening balance in the caller's
// transaction: the Account Decider checks it is cash and its start month
// still open, then the current Opening balance Transaction is reversed,
// linked to its reversal, and a new one is recorded on the start date. The
// same amount again records nothing.
func (t *Transactions) RecountOpening(ctx context.Context, tx *eventstore.Tx, account uuid.UUID,
	amount sharedkernel.Decimal,
) error {
	fold, found, err := t.accounts.Folds.Load(ctx, tx, account)
	if err != nil {
		return fmt.Errorf("load account %s: %w", account, err)
	}

	if !found {
		return ErrAccountNotFound
	}

	s, old := fold.State, fold.State.OpeningBalanceTransactionID

	var previous sharedkernel.Decimal

	if s.IsCash() && old != sharedkernel.NilID {
		opening, _, err := t.transactions.Folds.Load(ctx, tx, old)
		if err != nil {
			return fmt.Errorf("load opening balance transaction %s: %w", old, err)
		}

		for _, p := range opening.State.Postings {
			if p.AccountID == account {
				previous = p.Amount
			}
		}
	}

	recorded, err := sharedkernel.NewID()
	if err != nil {
		return err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	reversal, err := sharedkernel.NewID()
	if err != nil {
		return err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	res, err := t.accounts.Execute(ctx, tx, account, domain.RecountOpening{
		TransactionID: recorded, ReversalID: reversal, PreviousAmount: previous, Amount: amount,
	})
	if err != nil {
		return fmt.Errorf("recount the opening of account %s: %w", account, err)
	}

	if len(res.Events) == 0 {
		return nil
	}

	reversed, err := t.transactions.Execute(ctx, tx, old, domain.ReverseOpeningBalance{ReversalID: reversal})
	if err != nil {
		return fmt.Errorf("reverse opening balance transaction %s: %w", old, err)
	}

	if _, err := t.transactions.Execute(ctx, tx, reversal, reversed.State.Reversal(old)); err != nil {
		return fmt.Errorf("record opening balance reversal: %w", err)
	}

	return t.recordOpeningTransaction(ctx, tx, account, recorded, s, amount)
}

// CashTransaction is a cash payment or receipt a member records by hand:
// one date, a signed amount in the Account currency, a description, an
// optional payee, and the Category on the other side (nil for
// Uncategorised).
type CashTransaction struct {
	Account     uuid.UUID
	Date        time.Time
	Amount      sharedkernel.Decimal
	Currency    string
	Description string
	Payee       string
	Category    *uuid.UUID
}

// RecordCash records a cash Transaction in one transaction: a Transaction
// between the cash Account and the Category, its date both its Booking and
// its Transaction date, booked on the Account in its month (consistency
// map: Record cash Transaction). It returns the Transaction. A statement
// Account is ErrNotACashAccount; a zero amount ErrZeroAmount; a date after
// today (UTC) ErrDateInTheFuture; the Category follows Accept's rules; and
// the Account Decider refuses a closed month or a date before the start
// date.
func (t *Transactions) RecordCash(ctx context.Context, household uuid.UUID, c CashTransaction,
) (TransactionDetail, error) {
	id, err := sharedkernel.NewID()
	if err != nil {
		return TransactionDetail{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	var out TransactionDetail

	err = t.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := t.accounts.Folds.Load(ctx, tx, c.Account)
		if err != nil {
			return fmt.Errorf("load account %s: %w", c.Account, err)
		}

		today := tx.Now().UTC().Format(domain.DateLayout)

		switch {
		case !found:
			return ErrAccountNotFound
		case !fold.State.IsCash():
			return ErrNotACashAccount
		case fold.State.Currency != c.Currency:
			return fmt.Errorf("%w: %s, not %s", ErrCurrencyMismatch, c.Currency, fold.State.Currency)
		case c.Amount.IsZero():
			return ErrZeroAmount
		case c.Date.Format(domain.DateLayout) > today:
			return fmt.Errorf("%w: %s is after %s", ErrDateInTheFuture, c.Date.Format(domain.DateLayout), today)
		}

		category, uncategorised, err := t.chooseCategory(ctx, tx, c.Category)
		if err != nil {
			return err
		}

		if _, err := t.accounts.Execute(ctx, tx, c.Account, domain.BookTransaction{
			TransactionID: id, BookingDate: c.Date, Amount: c.Amount, Uncategorised: category == uncategorised,
		}); err != nil {
			return fmt.Errorf("book transaction on account %s: %w", c.Account, err)
		}

		recorded, err := t.transactions.Execute(ctx, tx, id, domain.RecordTransaction{
			BookingDate:     c.Date,
			TransactionDate: c.Date,
			Description:     c.Description,
			Payee:           c.Payee,
			Postings: []domain.Posting{
				{AccountID: c.Account, Amount: c.Amount, Currency: c.Currency},
				{CategoryID: category, Amount: c.Amount.Neg(), Currency: c.Currency},
			},
		})
		if err != nil {
			return fmt.Errorf("record transaction: %w", err)
		}

		out, err = t.detail(ctx, tx, id, recorded.State)

		return err
	})
	if err != nil {
		return TransactionDetail{}, fmt.Errorf("record a cash transaction on account %s: %w", c.Account, err)
	}

	return out, nil
}
