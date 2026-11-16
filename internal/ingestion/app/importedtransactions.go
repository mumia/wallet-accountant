package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

var (
	// ErrInvalidCursor is returned for a page cursor this API didn't hand
	// out.
	ErrInvalidCursor = errors.New("invalid cursor")
	// ErrImportedTransactionNotFound is returned for an Imported transaction
	// that is not in the Household.
	ErrImportedTransactionNotFound = errors.New("imported transaction not found")
	// ErrImportedTransactionAlreadyAccepted is returned for accepting an
	// Imported transaction twice: it is booked once.
	ErrImportedTransactionAlreadyAccepted = domain.ErrImportedTransactionAlreadyAccepted
	// ErrImportedTransactionDiscarded is returned for accepting a Discarded
	// Imported transaction: Discarded is final.
	ErrImportedTransactionDiscarded = domain.ErrImportedTransactionDiscarded
	// ErrImportedTransactionAlreadyDiscarded is returned for discarding an
	// Imported transaction twice.
	ErrImportedTransactionAlreadyDiscarded = domain.ErrImportedTransactionAlreadyDiscarded
	// ErrCategoryNotFound is returned for accepting with a Category that is
	// not in the Household.
	ErrCategoryNotFound = errors.New("category not found")
	// ErrInternalCategory is returned for accepting with an Internal
	// Category other than Uncategorised.
	ErrInternalCategory = errors.New("internal categories other than uncategorised can't be chosen")
)

// ImportedTransaction is an Imported transaction as the API sees it.
type ImportedTransaction struct {
	ID    uuid.UUID
	State domain.ImportedTransactionState
}

// ImportedFilter picks a page of an Account's Imported transactions. State
// is empty for every state, Month the zero time for every Booking month,
// After nil for the first page.
type ImportedFilter struct {
	State domain.ImportedState
	Month time.Time
	After *ImportedCursor
	Limit int
}

// ImportedCursor is where a page of Imported transactions starts: just
// after this one.
type ImportedCursor struct {
	BookingDate time.Time
	ID          uuid.UUID
}

// ImportedPage is a page of Imported transactions; Next is nil on the last
// page.
type ImportedPage struct {
	Items []ImportedTransaction
	Next  *ImportedCursor
}

// ImportedTransactionReader reads the Household's Imported transaction
// folds.
type ImportedTransactionReader interface {
	// ImportedTransactions returns up to filter.Limit Imported transactions
	// of the Account, sorted by Booking date and then ID (UUIDv7: the order
	// they were imported, which follows the file's lines).
	ImportedTransactions(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, filter ImportedFilter) ([]ImportedTransaction, error)
}

// ImportedTransactions lists Accounts' Imported transactions and Accepts
// them into the Ledger.
type ImportedTransactions struct {
	scope    HouseholdScope
	accounts Accounts
	ledger   Ledger
	rows     ImportedTransactionAggregate
	reader   ImportedTransactionReader
}

// NewImportedTransactions returns the ImportedTransactions service.
func NewImportedTransactions(scope HouseholdScope, accounts Accounts, ledger Ledger, rows ImportedTransactionAggregate,
	reader ImportedTransactionReader,
) *ImportedTransactions {
	return &ImportedTransactions{scope: scope, accounts: accounts, ledger: ledger, rows: rows, reader: reader}
}

// Accept Accepts a pending Imported transaction and, through the Ledger in
// the same database transaction, books it with exactly its amount: to the
// Category, or to Uncategorised when category is nil. The Imported
// transaction, the Ledger Transaction and the Account each check their own
// version, and a conflict retries the whole transaction with the same
// Transaction ID (ADR-0011, ADR-0013; consistency map: Accept imported
// transaction).
func (s *ImportedTransactions) Accept(ctx context.Context, household, id uuid.UUID, category *uuid.UUID,
) (ImportedTransaction, error) {
	transaction, err := sharedkernel.NewID()
	if err != nil {
		return ImportedTransaction{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	var accepted ImportedTransaction

	err = s.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		res, err := s.rows.Execute(ctx, tx, id, domain.AcceptImportedTransaction{TransactionID: transaction})
		if errors.Is(err, domain.ErrImportedTransactionNotImported) {
			return ErrImportedTransactionNotFound
		}

		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		if err := s.ledger.BookAccepted(ctx, tx, Booking{
			Transaction: transaction, ImportedTransaction: id, Category: category, Row: res.State,
		}); err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		accepted = ImportedTransaction{ID: id, State: res.State}

		return nil
	})
	if err != nil {
		return ImportedTransaction{}, fmt.Errorf("accept imported transaction %s: %w", id, err)
	}

	return accepted, nil
}

// ReturnToPending returns the Imported transaction, Accepted by the Ledger
// Transaction transaction, to pending, in the caller's transaction. The
// Ledger calls it when it reverses that Transaction, so the row can be
// Accepted again or Discarded (ADR-0013, ADR-0018; consistency map: Reverse
// Transaction).
func (s *ImportedTransactions) ReturnToPending(ctx context.Context, tx *eventstore.Tx, id, transaction uuid.UUID) error {
	if _, err := s.rows.Execute(ctx, tx, id, domain.ReturnToPending{TransactionID: transaction}); err != nil {
		return fmt.Errorf("return imported transaction %s to pending: %w", id, err)
	}

	return nil
}

// Discard Discards a pending Imported transaction so it never reaches the
// Ledger. Discarded is final (consistency map: Keep or Discard Imported
// transaction).
func (s *ImportedTransactions) Discard(ctx context.Context, household, id uuid.UUID) (ImportedTransaction, error) {
	var discarded ImportedTransaction

	err := s.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		res, err := s.rows.Execute(ctx, tx, id, domain.DiscardImportedTransaction{})
		if errors.Is(err, domain.ErrImportedTransactionNotImported) {
			return ErrImportedTransactionNotFound
		}

		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		discarded = ImportedTransaction{ID: id, State: res.State}

		return nil
	})
	if err != nil {
		return ImportedTransaction{}, fmt.Errorf("discard imported transaction %s: %w", id, err)
	}

	return discarded, nil
}

// List returns a page of the Account's Imported transactions.
func (s *ImportedTransactions) List(ctx context.Context, household, account uuid.UUID, filter ImportedFilter) (ImportedPage, error) {
	var page ImportedPage

	err := s.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		exists, err := s.accounts.AccountExists(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		if !exists {
			return ErrAccountNotFound
		}

		ask := filter
		ask.Limit++

		items, err := s.reader.ImportedTransactions(ctx, tx, account, ask)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if len(items) > filter.Limit {
			items = items[:filter.Limit]
			last := items[len(items)-1]
			page.Next = &ImportedCursor{BookingDate: last.State.BookingDate, ID: last.ID}
		}

		page.Items = items

		return nil
	})
	if err != nil {
		return ImportedPage{}, fmt.Errorf("list imported transactions of account %s: %w", account, err)
	}

	return page, nil
}

// String encodes the cursor for the API.
func (c ImportedCursor) String() string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.BookingDate.Format(domain.DateLayout) + "|" + c.ID.String()))
}

// ParseImportedCursor decodes a cursor ImportedCursor.String wrote.
func ParseImportedCursor(s string) (ImportedCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return ImportedCursor{}, ErrInvalidCursor
	}

	date, id, ok := strings.Cut(string(raw), "|")
	if !ok {
		return ImportedCursor{}, ErrInvalidCursor
	}

	d, errDate := time.Parse(domain.DateLayout, date)
	u, errID := uuid.Parse(id)

	if errDate != nil || errID != nil {
		return ImportedCursor{}, ErrInvalidCursor
	}

	return ImportedCursor{BookingDate: d, ID: u}, nil
}
