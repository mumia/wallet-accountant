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
	TransactionStreamType      = "ledger.transaction"
	TransactionRecordedType    = "io.walletaccountant.ledger.transaction.recorded.v1"
	TransactionReversedType    = "io.walletaccountant.ledger.transaction.reversed.v1"
	TransactionCategorySetType = "io.walletaccountant.ledger.transaction.categoryset.v1"
	TransactionMatchedType     = "io.walletaccountant.ledger.transaction.matched.v1"
	TransactionUnmatchedType   = "io.walletaccountant.ledger.transaction.unmatched.v1"
)

// OpeningBalanceDescription describes every Opening balance Transaction.
const OpeningBalanceDescription = "Opening balance"

var (
	// ErrInvalidCursor is returned for a page cursor this API didn't hand out.
	ErrInvalidCursor = errors.New("invalid cursor")
	// ErrOpeningBalanceAlreadyRecorded is returned for recording an
	// Account's Opening balance a second time.
	ErrOpeningBalanceAlreadyRecorded = domain.ErrOpeningBalanceAlreadyRecorded
	// ErrInternalCategory is returned for booking an Imported transaction to
	// an Internal Category other than Uncategorised.
	ErrInternalCategory = domain.ErrInternalCategory
	// ErrCategoryDirection is returned, as a *domain.DirectionError, for
	// choosing an Expense Category for money in or an Income Category for
	// money out.
	ErrCategoryDirection = domain.ErrCategoryDirection
	// ErrCurrencyMismatch is returned for booking an amount in another
	// currency than the Account's.
	ErrCurrencyMismatch = errors.New("amount isn't in the account currency")
	// ErrTransactionNotFound is returned for a Transaction that is not in
	// the Household.
	ErrTransactionNotFound = errors.New("transaction not found")
	// ErrOpeningBalanceTransaction is returned for reversing or
	// recategorising the Opening balance Transaction (ADR-0018).
	ErrOpeningBalanceTransaction = domain.ErrOpeningBalanceTransaction
	// ErrTransactionIsAReversal is returned for reversing or recategorising
	// a reversal.
	ErrTransactionIsAReversal = domain.ErrTransactionIsAReversal
	// ErrTransactionAlreadyReversed is returned for reversing a Transaction
	// twice.
	ErrTransactionAlreadyReversed = domain.ErrTransactionAlreadyReversed
	// ErrTransactionReversed is returned for recategorising a reversed
	// Transaction.
	ErrTransactionReversed = domain.ErrTransactionReversed
)

// TransactionAggregate is the Transaction Decider wired to the runtime.
type TransactionAggregate = eventstore.Aggregate[domain.TransactionState, domain.TransactionCommand, domain.TransactionEvent]

// NewTransactionAggregate wires the Transaction Decider to its fold store,
// which also saves one lookup row per Account Posting.
func NewTransactionAggregate(folds eventstore.FoldStore[domain.TransactionState]) TransactionAggregate {
	codec := eventstore.NewCodec[domain.TransactionEvent](Source)
	eventstore.Register[domain.TransactionEvent, domain.TransactionRecorded](codec, TransactionRecordedType)
	eventstore.Register[domain.TransactionEvent, domain.TransactionReversed](codec, TransactionReversedType)
	eventstore.Register[domain.TransactionEvent, domain.TransactionCategorySet](codec, TransactionCategorySetType)
	eventstore.Register[domain.TransactionEvent, domain.TransactionMatched](codec, TransactionMatchedType)
	eventstore.Register[domain.TransactionEvent, domain.TransactionUnmatched](codec, TransactionUnmatchedType)

	return TransactionAggregate{
		StreamType: TransactionStreamType,
		Decider:    domain.Transaction,
		Codec:      codec,
		Folds:      folds,
		// 2: the link to the Imported transaction an Accept booked.
		// 3: the links between a reversed Transaction and its reversal.
		// 4: a Transfer leg's counterpart Account, automatic flag and Match.
		FoldSchemaVersion: 4,
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
	// ReversalOf is the Transaction this one reverses; uuid.Nil unless it is
	// a reversal.
	ReversalOf uuid.UUID
	// ReversedBy is the Transaction that reversed this one; uuid.Nil while
	// it isn't reversed.
	ReversedBy uuid.UUID
	// ImportedTransactionID is the Imported transaction whose Accept
	// recorded this one; uuid.Nil for any other Transaction.
	ImportedTransactionID uuid.UUID
	// CounterpartAccountID is the other Account of a Transfer leg, MatchID
	// its Match while it has one, and CreatedByTransfer says an Accept as
	// Transfer recorded it; uuid.Nil and false for any other Transaction.
	CounterpartAccountID uuid.UUID
	MatchID              uuid.UUID
	CreatedByTransfer    bool
}

// TransactionFilter picks a page of an Account's Transactions. Month is the
// zero time for every Booking month; After is nil for the first page.
// InTransit keeps only the Transfer legs that are neither Matched nor
// reversed.
type TransactionFilter struct {
	Month     time.Time
	After     *TransactionCursor
	Limit     int
	InTransit bool
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
	// TransferSuggestions returns the IDs of the cash Account's Transfer
	// legs that are neither Matched nor reversed, name the row's Account,
	// have the opposite amount in its currency and a Transaction date within
	// SuggestionWindow of the row's, closest date first.
	TransferSuggestions(ctx context.Context, tx *eventstore.Tx, q SuggestionQuery) ([]uuid.UUID, error)
}

// InternalCategories finds the Household's Internal Categories.
type InternalCategories interface {
	// InternalCategory returns the ID of the Internal Category with the
	// role, in the Household the transaction is set to.
	InternalCategory(ctx context.Context, tx *eventstore.Tx, role domain.InternalRole) (uuid.UUID, error)
}

// Ingestion is the Ledger's port to Ingestion.
type Ingestion interface {
	// ReturnToPending returns the Imported transaction, Accepted by the
	// Transaction that was just reversed, to pending, in the caller's
	// transaction (consistency map: Reverse Transaction).
	ReturnToPending(ctx context.Context, tx *eventstore.Tx, importedTransaction, transaction uuid.UUID) error
	// MonthImports reports, in the caller's transaction, what the Account's
	// Statement imports leave unresolved in the Booking month month.
	MonthImports(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, month time.Time) (domain.MonthImports, error)
	// AccountImports reports, in the caller's transaction, what the
	// Account's Statement imports leave unresolved in every Booking month,
	// in one grouped query each for counts and for unchecked imports.
	AccountImports(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (domain.AccountImports, error)
	// UnresolvedRows counts, in the caller's transaction, the pending and
	// Suspected duplicate Imported transactions of the Account, or of every
	// Account of the Household when account is nil, over every Booking
	// month, in one grouped query. An Account with none is absent.
	UnresolvedRows(ctx context.Context, tx *eventstore.Tx, account *uuid.UUID) (map[uuid.UUID]ImportedRows, error)
	// MonthImportsForClose is MonthImports for a Month close: it first holds
	// the Account's Statement imports until the transaction ends, so no
	// upload or waiting Balance check runs between the read and the close
	// (consistency map: Close month).
	MonthImportsForClose(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, month time.Time,
	) (domain.MonthImports, error)
}

// Transactions records, corrects and lists a Household's Transactions.
type Transactions struct {
	scope        HouseholdScope
	accounts     AccountAggregate
	transactions TransactionAggregate
	categories   CategoryAggregate
	matches      MatchAggregate
	reader       TransactionReader
	internal     InternalCategories
	ingestion    Ingestion
}

// NewTransactions returns the Transactions service.
func NewTransactions(scope HouseholdScope, accounts AccountAggregate, transactions TransactionAggregate,
	categories CategoryAggregate, matches MatchAggregate, reader TransactionReader, internal InternalCategories,
	ingestion Ingestion,
) *Transactions {
	return &Transactions{
		scope: scope, accounts: accounts, transactions: transactions, categories: categories, matches: matches,
		reader: reader, internal: internal, ingestion: ingestion,
	}
}

// ImportedBooking is an Accepted Imported transaction to book: its row
// exactly as imported, and the Category for the other side (nil for
// Uncategorised).
type ImportedBooking struct {
	Account             uuid.UUID
	Transaction         uuid.UUID
	ImportedTransaction uuid.UUID
	Category            *uuid.UUID
	BookingDate         time.Time
	TransactionDate     time.Time
	ValueDate           time.Time
	Amount              sharedkernel.Decimal
	Currency            string
	Description         string
	Payee               string
}

// RecordImported books an Accepted Imported transaction in the caller's
// transaction: a Transaction between the Account and the Category, carrying
// the row's amount, dates, description and payee and linked to it, and the
// Account's balance and Uncategorised count for the Booking month. No
// Category, or Uncategorised, books the other side to Uncategorised; any
// other Internal Category is ErrInternalCategory, a Category not in the
// Household ErrCategoryNotFound, and an Expense Category on money in or an
// Income Category on money out ErrCategoryDirection, as a
// *domain.DirectionError. Ingestion calls it when a User Accepts
// (consistency map: Accept imported transaction).
func (t *Transactions) RecordImported(ctx context.Context, tx *eventstore.Tx, b ImportedBooking) error {
	category, uncategorised, kind, err := t.chooseCategory(ctx, tx, b.Category)
	if err != nil {
		return err
	}

	if err := domain.FitsDirection(kind, b.Amount); err != nil {
		return err //nolint:wrapcheck // the domain's refusal, as a *domain.DirectionError
	}

	fold, found, err := t.accounts.Folds.Load(ctx, tx, b.Account)
	if err != nil {
		return fmt.Errorf("load account %s: %w", b.Account, err)
	}

	switch {
	case !found:
		return ErrAccountNotFound
	case fold.State.Currency != b.Currency:
		return fmt.Errorf("%w: %s, not %s", ErrCurrencyMismatch, b.Currency, fold.State.Currency)
	}

	if _, err := t.accounts.Execute(ctx, tx, b.Account, domain.BookTransaction{
		TransactionID: b.Transaction, BookingDate: b.BookingDate, Amount: b.Amount,
		Uncategorised: category == uncategorised,
	}); err != nil {
		return fmt.Errorf("book transaction on account %s: %w", b.Account, err)
	}

	if _, err := t.transactions.Execute(ctx, tx, b.Transaction, domain.RecordTransaction{
		ImportedTransactionID: b.ImportedTransaction,
		BookingDate:           b.BookingDate,
		TransactionDate:       b.TransactionDate,
		ValueDate:             b.ValueDate,
		Description:           b.Description,
		Payee:                 b.Payee,
		Postings: []domain.Posting{
			{AccountID: b.Account, Amount: b.Amount, Currency: b.Currency},
			{CategoryID: category, Amount: b.Amount.Neg(), Currency: b.Currency},
		},
	}); err != nil {
		return fmt.Errorf("record transaction: %w", err)
	}

	return nil
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

	return t.recordOpeningTransaction(ctx, tx, account, transaction, fold.State, amount)
}

// recordOpeningTransaction records the Opening balance Transaction of the
// Account s, on its start date, between it and the Opening balance Internal
// Category. The Account's own Opening balance event is the caller's.
func (t *Transactions) recordOpeningTransaction(ctx context.Context, tx *eventstore.Tx, account, transaction uuid.UUID,
	s domain.AccountState, amount sharedkernel.Decimal,
) error {
	category, err := t.internal.InternalCategory(ctx, tx, domain.RoleOpeningBalance)
	if err != nil {
		return fmt.Errorf("find the opening balance category: %w", err)
	}

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
