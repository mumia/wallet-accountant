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
	// ErrImportedTransactionNotSuspectedDuplicate is returned for keeping an
	// Imported transaction that isn't a Suspected duplicate.
	ErrImportedTransactionNotSuspectedDuplicate = domain.ErrImportedTransactionNotSuspectedDuplicate
	// ErrImportedTransactionSuspectedDuplicate is returned for accepting a
	// Suspected duplicate before it is kept.
	ErrImportedTransactionSuspectedDuplicate = domain.ErrImportedTransactionSuspectedDuplicate
	// ErrCategoryNotFound is returned for accepting with a Category that is
	// not in the Household.
	ErrCategoryNotFound = errors.New("category not found")
	// ErrInternalCategory is returned for accepting with an Internal
	// Category other than Uncategorised.
	ErrInternalCategory = errors.New("internal categories other than uncategorised can't be chosen")
)

// ImportedTransaction is an Imported transaction as the API sees it.
// Resembles is the earlier Imported transaction a Suspected duplicate
// looks like, nil for any other row.
type ImportedTransaction struct {
	ID        uuid.UUID
	State     domain.ImportedTransactionState
	Resembles *ImportedTransaction
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

// ImportedTransactions lists Accounts' Imported transactions, Accepts them
// into the Ledger, and keeps or Discards them, running a Statement import's
// waiting Balance check once its last Suspected duplicate is decided.
type ImportedTransactions struct {
	scope    HouseholdScope
	accounts Accounts
	ledger   Ledger
	rows     ImportedTransactionAggregate
	reader   ImportedTransactionReader
	imports  StatementImportAggregate
	checks   StatementImportReader
}

// NewImportedTransactions returns the ImportedTransactions service.
func NewImportedTransactions(scope HouseholdScope, accounts Accounts, ledger Ledger, rows ImportedTransactionAggregate,
	reader ImportedTransactionReader, imports StatementImportAggregate, checks StatementImportReader,
) *ImportedTransactions {
	return &ImportedTransactions{
		scope: scope, accounts: accounts, ledger: ledger, rows: rows, reader: reader, imports: imports, checks: checks,
	}
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

		accepted, err = s.withResembled(ctx, tx, ImportedTransaction{ID: id, State: res.State})

		return err
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

// MonthImports reports, in the caller's transaction, what the Account's
// Statement imports leave unresolved in the Booking month: its pending and
// Suspected duplicate Imported transactions (Discarded don't count), and
// its non-voided Statement imports with rows in or before the month whose
// Balance check is waiting or failed. The Ledger's month status reads it.
func (s *ImportedTransactions) MonthImports(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, month time.Time,
) (MonthImports, error) {
	m, err := s.checks.MonthImports(ctx, tx, account, month)
	if err != nil {
		return MonthImports{}, err //nolint:wrapcheck // already wrapped by the adapter
	}

	return m, nil
}

// MonthImportsForClose is MonthImports after locking the Account's
// Statement imports, so an upload or a waiting Balance check running at the
// same time is either seen whole or runs after the close. The Ledger calls
// it to close the month (consistency map: Close month).
func (s *ImportedTransactions) MonthImportsForClose(ctx context.Context, tx *eventstore.Tx, account uuid.UUID,
	month time.Time,
) (MonthImports, error) {
	if err := s.checks.LockAccount(ctx, tx, account); err != nil {
		return MonthImports{}, err //nolint:wrapcheck // already wrapped by the adapter
	}

	m, err := s.checks.MonthImports(ctx, tx, account, month)
	if err != nil {
		return MonthImports{}, err //nolint:wrapcheck // already wrapped by the adapter
	}

	return m, nil
}

// Discard Discards a pending Imported transaction or a Suspected
// duplicate so it never reaches the Ledger. Discarded is final
// (consistency map: Keep or Discard Imported transaction).
func (s *ImportedTransactions) Discard(ctx context.Context, household, id uuid.UUID) (ImportedTransaction, error) {
	return s.decide(ctx, household, id, "discard", domain.DiscardImportedTransaction{})
}

// Keep keeps a Suspected duplicate as a real transaction: it becomes
// pending (consistency map: Keep or Discard Imported transaction).
func (s *ImportedTransactions) Keep(ctx context.Context, household, id uuid.UUID) (ImportedTransaction, error) {
	return s.decide(ctx, household, id, "keep", domain.KeepImportedTransaction{})
}

// decide keeps or Discards an Imported transaction. Deciding a Suspected
// duplicate locks its Account's Statement imports first, so of two
// decisions on one import's last Suspected duplicates exactly one sees none
// left and runs the import's waiting Balance check, in the same
// transaction (ADR-0018).
func (s *ImportedTransactions) decide(ctx context.Context, household, id uuid.UUID, action string,
	cmd domain.ImportedTransactionCommand,
) (ImportedTransaction, error) {
	var decided ImportedTransaction

	err := s.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := s.rows.Folds.Load(ctx, tx, id)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !found {
			return ErrImportedTransactionNotFound
		}

		// A row is a Suspected duplicate only from its import on, so a row
		// that isn't one now never becomes one. One that is may have been
		// decided while this waited for the lock: read it again under it.
		suspected := fold.State.State == domain.ImportedSuspectedDuplicate
		if suspected {
			if err := s.checks.LockAccount(ctx, tx, fold.State.AccountID); err != nil {
				return err //nolint:wrapcheck // the adapter's error
			}

			if fold, _, err = s.rows.Folds.Load(ctx, tx, id); err != nil {
				return err //nolint:wrapcheck // the adapter's error
			}

			suspected = fold.State.State == domain.ImportedSuspectedDuplicate
		}

		res, err := s.rows.Execute(ctx, tx, id, cmd)
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		if suspected {
			if err := s.runWaitingCheck(ctx, tx, res.State); err != nil {
				return err
			}
		}

		decided, err = s.withResembled(ctx, tx, ImportedTransaction{ID: id, State: res.State})

		return err
	})
	if err != nil {
		return ImportedTransaction{}, fmt.Errorf("%s imported transaction %s: %w", action, id, err)
	}

	return decided, nil
}

// runWaitingCheck runs the Balance check of the row's Statement import once
// none of its Suspected duplicates is left undecided, if it is still
// waiting: a void never lets it run.
func (s *ImportedTransactions) runWaitingCheck(ctx context.Context, tx *eventstore.Tx, row domain.ImportedTransactionState) error {
	undecided, err := s.checks.UndecidedSuspectedDuplicates(ctx, tx, row.StatementImportID)
	if err != nil || undecided > 0 {
		return err //nolint:wrapcheck // the adapter's error
	}

	statementImport, found, err := s.imports.Folds.Load(ctx, tx, row.StatementImportID)
	if err != nil {
		return err //nolint:wrapcheck // the adapter's error
	}

	if !found || statementImport.State.BalanceCheck != domain.BalanceCheckWaiting || statementImport.State.Voided() {
		return nil
	}

	upTo, err := s.checks.ImportedUpTo(ctx, tx, row.AccountID, row.StatementImportID)
	if err != nil {
		return err //nolint:wrapcheck // the adapter's error
	}

	if _, err := s.imports.Execute(ctx, tx, row.StatementImportID, domain.RunBalanceCheck{
		OpeningBalance: upTo.OpeningBalance, Total: upTo.Total,
	}); err != nil {
		return fmt.Errorf("run balance check of statement import %s: %w", row.StatementImportID, err)
	}

	return nil
}

// withResembled adds the earlier Imported transaction the row resembles,
// if it is or was a Suspected duplicate.
func (s *ImportedTransactions) withResembled(ctx context.Context, tx *eventstore.Tx, i ImportedTransaction,
) (ImportedTransaction, error) {
	if i.State.Resembles == uuid.Nil {
		return i, nil
	}

	fold, found, err := s.rows.Folds.Load(ctx, tx, i.State.Resembles)
	if err != nil {
		return ImportedTransaction{}, err //nolint:wrapcheck // the adapter's error
	}

	if found {
		i.Resembles = &ImportedTransaction{ID: i.State.Resembles, State: fold.State}
	}

	return i, nil
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

		for n, i := range items {
			if items[n], err = s.withResembled(ctx, tx, i); err != nil {
				return err
			}
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
