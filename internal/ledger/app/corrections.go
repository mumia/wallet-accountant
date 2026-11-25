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

// TransactionDetail is a Transaction as seen from its Account: its Posting
// there and the Category on the other side.
type TransactionDetail struct {
	ID                    uuid.UUID
	AccountID             uuid.UUID
	Amount                sharedkernel.Decimal
	Currency              string
	BookingDate           time.Time
	TransactionDate       time.Time
	ValueDate             time.Time
	Description           string
	Payee                 string
	CategoryID            uuid.UUID
	CategoryName          string
	ImportedTransactionID uuid.UUID
	ReversalOf            uuid.UUID
	ReversedBy            uuid.UUID
	CounterpartAccountID  uuid.UUID
	MatchID               uuid.UUID
	CreatedByTransfer     bool
}

// chooseCategory returns the Category a User chose for the non-Account
// side, and the Household's Uncategorised Category. None, or Uncategorised,
// is Uncategorised; any other Internal Category is ErrInternalCategory, and
// a Category not in the Household ErrCategoryNotFound.
func (t *Transactions) chooseCategory(ctx context.Context, tx *eventstore.Tx, chosen *uuid.UUID,
) (category, uncategorised uuid.UUID, err error) {
	uncategorised, err = t.internal.InternalCategory(ctx, tx, domain.RoleUncategorised)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("find the uncategorised category: %w", err)
	}

	if chosen == nil || *chosen == uncategorised {
		return uncategorised, uncategorised, nil
	}

	fold, found, err := t.categories.Folds.Load(ctx, tx, *chosen)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("load category %s: %w", *chosen, err)
	}

	switch {
	case !found:
		return uuid.Nil, uuid.Nil, ErrCategoryNotFound
	case fold.State.Kind == domain.KindInternal:
		return uuid.Nil, uuid.Nil, fmt.Errorf("%w: only Uncategorised can be chosen", ErrInternalCategory)
	}

	return *chosen, uncategorised, nil
}

// SetCategory sets the Category of the Transaction's non-Account side, with
// Accept's rules for the Category, and moves it in or out of its Account's
// Uncategorised count for the Booking month, in one transaction. Its amount
// never changes. A reversed Transaction, a reversal and the Opening balance
// Transaction keep their Category (consistency map: Set Transaction
// Category).
func (t *Transactions) SetCategory(ctx context.Context, household, id, category uuid.UUID) (TransactionDetail, error) {
	var out TransactionDetail

	err := t.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := t.transactions.Folds.Load(ctx, tx, id)

		switch {
		case err != nil:
			return fmt.Errorf("load transaction %s: %w", id, err)
		case !found:
			return ErrTransactionNotFound
		case fold.State.IsTransferLeg():
			// Before the Category is read, so any Category gets this.
			return ErrTransferLeg
		}

		chosen, uncategorised, err := t.chooseCategory(ctx, tx, &category)
		if err != nil {
			return err
		}

		res, err := t.transactions.Execute(ctx, tx, id, domain.SetTransactionCategory{CategoryID: chosen})
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		for _, e := range res.Events {
			set, ok := e.Event.(domain.TransactionCategorySet)
			if !ok {
				continue
			}

			for _, p := range res.State.Postings {
				if p.AccountID == uuid.Nil {
					continue
				}

				if _, err := t.accounts.Execute(ctx, tx, p.AccountID, domain.RecategoriseTransaction{
					TransactionID:    id,
					BookingDate:      res.State.BookingDate,
					WasUncategorised: set.PreviousCategoryID == uncategorised,
					Uncategorised:    set.CategoryID == uncategorised,
				}); err != nil {
					return fmt.Errorf("recategorise on account %s: %w", p.AccountID, err)
				}
			}
		}

		out, err = t.detail(ctx, tx, id, res.State)

		return err
	})
	if err != nil {
		return TransactionDetail{}, fmt.Errorf("set category of transaction %s: %w", id, err)
	}

	return out, nil
}

// Reverse reverses a mistaken Transaction in one transaction: it is marked
// reversed, a reversing Transaction with the opposite Postings, the same
// Booking date and a link to it is recorded, every Account it touched moves
// back (neither counts as Uncategorised any more), and through Ingestion
// its Imported transaction, if an Accept recorded it, returns to pending
// (ADR-0013, ADR-0018; consistency map: Reverse Transaction). It returns
// the reversing Transaction. The Opening balance Transaction, a reversal
// and a Transaction already reversed are refused.
func (t *Transactions) Reverse(ctx context.Context, household, id uuid.UUID) (TransactionDetail, error) {
	reversal, err := sharedkernel.NewID()
	if err != nil {
		return TransactionDetail{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	var out TransactionDetail

	err = t.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		res, err := t.transactions.Execute(ctx, tx, id, domain.ReverseTransaction{ReversalID: reversal})
		if errors.Is(err, domain.ErrTransactionNotRecorded) {
			return ErrTransactionNotFound
		}

		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		original := res.State

		uncategorised, err := t.internal.InternalCategory(ctx, tx, domain.RoleUncategorised)
		if err != nil {
			return fmt.Errorf("find the uncategorised category: %w", err)
		}

		record := original.Reversal(id)

		for _, p := range record.Postings {
			if p.AccountID == uuid.Nil {
				continue
			}

			if _, err := t.accounts.Execute(ctx, tx, p.AccountID, domain.BookReversal{
				TransactionID:    id,
				ReversalID:       reversal,
				BookingDate:      original.BookingDate,
				Amount:           p.Amount,
				WasUncategorised: original.CategoryID() == uncategorised,
			}); err != nil {
				return fmt.Errorf("book reversal on account %s: %w", p.AccountID, err)
			}
		}

		recorded, err := t.transactions.Execute(ctx, tx, reversal, record)
		if err != nil {
			return fmt.Errorf("record reversal: %w", err)
		}

		if original.ImportedTransactionID != uuid.Nil {
			if err := t.ingestion.ReturnToPending(ctx, tx, original.ImportedTransactionID, id); err != nil {
				return fmt.Errorf("return imported transaction %s to pending: %w", original.ImportedTransactionID, err)
			}
		}

		out, err = t.detail(ctx, tx, reversal, recorded.State)

		return err
	})
	if err != nil {
		return TransactionDetail{}, fmt.Errorf("reverse transaction %s: %w", id, err)
	}

	return out, nil
}

// ReverseOpeningBalance reverses the Account's Opening balance Transaction
// in the caller's transaction, when Ingestion voids the Account's first
// Statement import: the Transaction is marked reversed, a reversal with the
// opposite Postings on the start date is recorded, linked to it, and the
// Account's balance moves back as it goes back to awaiting its first
// Statement import (ADR-0018; consistency map: Void Statement import). It
// is the only way the Opening balance Transaction is reversed.
func (t *Transactions) ReverseOpeningBalance(ctx context.Context, tx *eventstore.Tx, account, reversal uuid.UUID) error {
	fold, found, err := t.accounts.Folds.Load(ctx, tx, account)
	if err != nil {
		return fmt.Errorf("load account %s: %w", account, err)
	}

	switch {
	case !found:
		return ErrAccountNotFound
	case fold.State.AwaitingFirstStatementImport:
		return fmt.Errorf("reverse the opening balance of account %s: %w", account, domain.ErrAwaitingFirstStatementImport)
	}

	opening := fold.State.OpeningBalanceTransactionID

	res, err := t.transactions.Execute(ctx, tx, opening, domain.ReverseOpeningBalance{ReversalID: reversal})
	if err != nil {
		return fmt.Errorf("reverse opening balance transaction %s: %w", opening, err)
	}

	record := res.State.Reversal(opening)

	for _, p := range record.Postings {
		if p.AccountID != account {
			continue
		}

		if _, err := t.accounts.Execute(ctx, tx, account, domain.ReopenOpeningBalance{
			TransactionID: opening, ReversalID: reversal, Amount: p.Amount,
		}); err != nil {
			return fmt.Errorf("reopen the opening balance of account %s: %w", account, err)
		}
	}

	if _, err := t.transactions.Execute(ctx, tx, reversal, record); err != nil {
		return fmt.Errorf("record opening balance reversal: %w", err)
	}

	return nil
}

// detail is the Transaction as seen from its first Account Posting, named
// with its Category.
func (t *Transactions) detail(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, s domain.TransactionState,
) (TransactionDetail, error) {
	d := TransactionDetail{
		ID:                    id,
		BookingDate:           s.BookingDate,
		TransactionDate:       s.TransactionDate,
		ValueDate:             s.ValueDate,
		Description:           s.Description,
		Payee:                 s.Payee,
		CategoryID:            s.CategoryID(),
		ImportedTransactionID: s.ImportedTransactionID,
		ReversalOf:            s.ReversalOf,
		ReversedBy:            s.ReversedBy,
		CounterpartAccountID:  s.CounterpartAccountID,
		MatchID:               s.MatchID,
		CreatedByTransfer:     s.CreatedByTransfer,
	}

	for _, p := range s.Postings {
		if p.AccountID != uuid.Nil {
			d.AccountID, d.Amount, d.Currency = p.AccountID, p.Amount, p.Currency

			break
		}
	}

	if d.CategoryID != uuid.Nil {
		fold, found, err := t.categories.Folds.Load(ctx, tx, d.CategoryID)
		if err != nil {
			return TransactionDetail{}, fmt.Errorf("load category %s: %w", d.CategoryID, err)
		}

		if found {
			d.CategoryName = fold.State.Name
		}
	}

	return d, nil
}
