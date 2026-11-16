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
)

// ErrInvalidCursor is returned for a page cursor this API didn't hand out.
var ErrInvalidCursor = errors.New("invalid cursor")

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

// ImportedTransactions lists Accounts' Imported transactions.
type ImportedTransactions struct {
	scope    HouseholdScope
	accounts Accounts
	reader   ImportedTransactionReader
}

// NewImportedTransactions returns the ImportedTransactions service.
func NewImportedTransactions(scope HouseholdScope, accounts Accounts, reader ImportedTransactionReader) *ImportedTransactions {
	return &ImportedTransactions{scope: scope, accounts: accounts, reader: reader}
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
