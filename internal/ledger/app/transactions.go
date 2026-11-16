package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// Stream type and event types of the Transaction aggregate (ADR-0015).
const (
	TransactionStreamType   = "ledger.transaction"
	TransactionRecordedType = "io.walletaccountant.ledger.transaction.recorded.v1"
)

// OpeningBalanceDescription describes every Opening balance Transaction.
const OpeningBalanceDescription = "Opening balance"

var (
	// ErrInvalidCursor is returned for a page cursor this API didn't hand out.
	ErrInvalidCursor = errors.New("invalid cursor")
	// ErrOpeningBalanceAlreadyRecorded is returned for recording an
	// Account's Opening balance a second time.
	ErrOpeningBalanceAlreadyRecorded = domain.ErrOpeningBalanceAlreadyRecorded
)

// TransactionAggregate is the Transaction Decider wired to the runtime.
type TransactionAggregate = eventstore.Aggregate[domain.TransactionState, domain.TransactionCommand, domain.TransactionEvent]

// NewTransactionAggregate wires the Transaction Decider to its fold store,
// which also saves one lookup row per Account Posting.
func NewTransactionAggregate(folds eventstore.FoldStore[domain.TransactionState]) TransactionAggregate {
	codec := eventstore.NewCodec[domain.TransactionEvent](Source)
	eventstore.Register[domain.TransactionEvent, domain.TransactionRecorded](codec, TransactionRecordedType)

	return TransactionAggregate{
		StreamType:        TransactionStreamType,
		Decider:           domain.Transaction,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 1,
	}
}

// AccountTransaction is one line of an Account's single-entry list
// (ADR-0001): a Transaction's Posting on the Account with the Category on
// the other side.
type AccountTransaction struct {
	TransactionID   uuid.UUID
	Ordinal         int
	BookingDate     time.Time
	TransactionDate time.Time
	Description     string
	Amount          sharedkernel.Decimal
	Currency        string
	CategoryID      uuid.UUID
	CategoryName    string
}

// TransactionFilter picks a page of an Account's Transactions. Month is the
// zero time for every Booking month; After is nil for the first page.
type TransactionFilter struct {
	Month time.Time
	After *TransactionCursor
	Limit int
}

// TransactionCursor is where a page of an Account's Transactions starts:
// just after this line.
type TransactionCursor struct {
	BookingDate   time.Time
	TransactionID uuid.UUID
	Ordinal       int
}

// TransactionPage is a page of an Account's Transactions; Next is nil on
// the last page.
type TransactionPage struct {
	Items []AccountTransaction
	Next  *TransactionCursor
}

// TransactionReader reads the lookup rows of the Transaction folds.
type TransactionReader interface {
	// AccountTransactions returns up to filter.Limit lines of the Account,
	// in the Household the transaction is set to, sorted by Booking date,
	// Transaction ID and Posting.
	AccountTransactions(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, filter TransactionFilter) ([]AccountTransaction, error)
}

// InternalCategories finds the Household's Internal Categories.
type InternalCategories interface {
	// InternalCategory returns the ID of the Internal Category with the
	// role, in the Household the transaction is set to.
	InternalCategory(ctx context.Context, tx *eventstore.Tx, role domain.InternalRole) (uuid.UUID, error)
}

// Transactions records and lists a Household's Transactions.
type Transactions struct {
	scope        HouseholdScope
	accounts     AccountAggregate
	transactions TransactionAggregate
	reader       TransactionReader
	internal     InternalCategories
}

// NewTransactions returns the Transactions service.
func NewTransactions(scope HouseholdScope, accounts AccountAggregate, transactions TransactionAggregate,
	reader TransactionReader, internal InternalCategories,
) *Transactions {
	return &Transactions{scope: scope, accounts: accounts, transactions: transactions, reader: reader, internal: internal}
}

// RecordOpeningBalance records the Account's Opening balance worked out by
// its first Statement import, in the caller's transaction: a Transaction on
// the start date between the Account and the Opening balance Internal
// Category, and the Account's balance, which stops it awaiting its first
// Statement import (ADR-0018). A second Opening balance is
// ErrOpeningBalanceAlreadyRecorded. Ingestion calls it when it records the
// import (consistency map: Upload Statement import).
func (t *Transactions) RecordOpeningBalance(ctx context.Context, tx *eventstore.Tx, account, transaction uuid.UUID,
	amount sharedkernel.Decimal,
) error {
	fold, found, err := t.accounts.Folds.Load(ctx, tx, account)
	if err != nil {
		return fmt.Errorf("load account %s: %w", account, err)
	}

	if !found {
		return ErrAccountNotFound
	}

	if _, err := t.accounts.Execute(ctx, tx, account, domain.RecordOpeningBalance{
		TransactionID: transaction, Amount: amount,
	}); err != nil {
		return fmt.Errorf("record opening balance of account %s: %w", account, err)
	}

	category, err := t.internal.InternalCategory(ctx, tx, domain.RoleOpeningBalance)
	if err != nil {
		return fmt.Errorf("find the opening balance category: %w", err)
	}

	s := fold.State

	if _, err := t.transactions.Execute(ctx, tx, transaction, domain.RecordTransaction{
		OpeningBalance:  true,
		BookingDate:     s.StartDate,
		TransactionDate: s.StartDate,
		Description:     OpeningBalanceDescription,
		Postings: []domain.Posting{
			{AccountID: account, Amount: amount, Currency: s.Currency},
			{CategoryID: category, Amount: amount.Neg(), Currency: s.Currency},
		},
	}); err != nil {
		return fmt.Errorf("record opening balance transaction: %w", err)
	}

	return nil
}

// List returns a page of the Account's single-entry Transaction list, read
// from the Transaction folds' lookup rows in the request transaction, so it
// is always current (ADR-0011 "Reads").
func (t *Transactions) List(ctx context.Context, household, account uuid.UUID, filter TransactionFilter) (TransactionPage, error) {
	var page TransactionPage

	err := t.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		_, found, err := t.accounts.Folds.Load(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !found {
			return ErrAccountNotFound
		}

		// One more than asked tells whether there is a next page.
		ask := filter
		ask.Limit++

		items, err := t.reader.AccountTransactions(ctx, tx, account, ask)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if len(items) > filter.Limit {
			items = items[:filter.Limit]
			last := items[len(items)-1]
			page.Next = &TransactionCursor{BookingDate: last.BookingDate, TransactionID: last.TransactionID, Ordinal: last.Ordinal}
		}

		page.Items = items

		return nil
	})
	if err != nil {
		return TransactionPage{}, fmt.Errorf("list transactions of account %s: %w", account, err)
	}

	return page, nil
}

// String encodes the cursor for the API.
func (c TransactionCursor) String() string {
	raw := c.BookingDate.Format(domain.DateLayout) + "|" + c.TransactionID.String() + "|" + strconv.Itoa(c.Ordinal)

	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// ParseTransactionCursor decodes a cursor TransactionCursor.String wrote.
func ParseTransactionCursor(s string) (TransactionCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return TransactionCursor{}, ErrInvalidCursor
	}

	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 { //nolint:mnd // date, ID and Posting
		return TransactionCursor{}, ErrInvalidCursor
	}

	date, errDate := time.Parse(domain.DateLayout, parts[0])
	id, errID := uuid.Parse(parts[1])
	ordinal, errOrdinal := strconv.Atoi(parts[2])

	if errDate != nil || errID != nil || errOrdinal != nil {
		return TransactionCursor{}, ErrInvalidCursor
	}

	return TransactionCursor{BookingDate: date, TransactionID: id, Ordinal: ordinal}, nil
}
