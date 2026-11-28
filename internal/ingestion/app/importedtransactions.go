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
	// ErrMonthClosed is returned, as a *MonthError, for accepting, keeping
	// or Discarding a row booked in a closed month: its amounts are locked
	// until it is reopened (ADR-0024).
	ErrMonthClosed = errors.New("month closed")
	// ErrMonthNotOpenYet is returned, as a *MonthError, for accepting or
	// Discarding a pending row booked after the Open month (ADR-0024).
	ErrMonthNotOpenYet = errors.New("month not open yet")
	// ErrInvalidTransfer is returned, as a *TransferRefusedError, for an
	// Accept as Transfer the Ledger refuses.
	ErrInvalidTransfer = errors.New("invalid transfer")
	// ErrCategoryDirection is returned, as a *CategoryDirectionError, for
	// accepting with an Expense Category a row that is money in, or with an
	// Income Category one that is money out.
	ErrCategoryDirection = errors.New("the category's kind doesn't fit the amount's direction")
)

// CategoryDirectionError is a Category the Ledger refused because its kind
// doesn't fit the row's direction. Kind (INCOME or EXPENSE) and Direction
// (IN or OUT) are the contract's names.
type CategoryDirectionError struct {
	Kind      string
	Direction string
	Err       error
}

func (e *CategoryDirectionError) Error() string {
	return fmt.Sprintf("%s (%s, %s): %v", ErrCategoryDirection, e.Kind, e.Direction, e.Err)
}

func (e *CategoryDirectionError) Unwrap() []error { return []error{ErrCategoryDirection, e.Err} }

// TransferRefusedError is a refused Accept as Transfer. Reason is the
// contract's name for why (such as ACCOUNT_KIND_NOT_SUPPORTED). AccountID
// names the Account it is refused for, when there is one: the loan or
// crypto wallet Account, or, for a row dated before the cash Account's
// start date (BEFORE_START_DATE), the cash Account, with Date and StartDate
// the two dates; otherwise they are zero.
type TransferRefusedError struct {
	Reason    string
	AccountID uuid.UUID
	Date      time.Time
	StartDate time.Time
	Err       error
}

func (e *TransferRefusedError) Error() string {
	return fmt.Sprintf("%s (%s): %v", ErrInvalidTransfer, e.Reason, e.Err)
}

func (e *TransferRefusedError) Unwrap() []error { return []error{ErrInvalidTransfer, e.Err} }

// MonthError is a refusal about the Booking month (YYYY-MM) of a row that
// names it and its Account's Open month. Err is the refusal itself.
// AccountID names the Account whose month it is when that isn't the row's
// own, such as the cash Account of an Accept as Transfer; uuid.Nil
// otherwise.
type MonthError struct {
	Err       error
	Month     string
	OpenMonth string
	AccountID uuid.UUID
}

func (e *MonthError) Error() string {
	return fmt.Sprintf("%s: %s (the open month is %s)", e.Err, e.Month, e.OpenMonth)
}

func (e *MonthError) Unwrap() error { return e.Err }

// monthOf is the Booking month of the row, written YYYY-MM.
func monthOf(row domain.ImportedTransactionState) string {
	return row.BookingDate.Format(domain.MonthLayout)
}

// closedMonth refuses the row with ErrMonthClosed when it is booked before
// the Open month; nil otherwise.
func closedMonth(row domain.ImportedTransactionState, openMonth string) error {
	if month := monthOf(row); month < openMonth {
		return &MonthError{Err: ErrMonthClosed, Month: month, OpenMonth: openMonth}
	}

	return nil
}

// notOpenYet refuses the row with ErrMonthNotOpenYet when it is booked
// after the Open month; nil otherwise.
func notOpenYet(row domain.ImportedTransactionState, openMonth string) error {
	if month := monthOf(row); month > openMonth {
		return &MonthError{Err: ErrMonthNotOpenYet, Month: month, OpenMonth: openMonth}
	}

	return nil
}

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
	// PendingTransferRows returns the pending Imported transactions on q's
	// Accounts with q's amount and currency and a Transaction date within
	// SuggestionWindowDays of q's, closest date first, then by ID.
	PendingTransferRows(ctx context.Context, tx *eventstore.Tx, q PendingRowQuery) ([]ImportedTransaction, error)
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

// Acceptance is how a row is Accepted: to Category (nil for
// Uncategorised), or, with TransferAccount, as a Transfer to or from that
// cash Account. MatchLeg is a leg the cash Account already has to Match
// instead of recording a new one.
type Acceptance struct {
	Category        *uuid.UUID
	TransferAccount *uuid.UUID
	MatchLeg        *uuid.UUID
}

// Accept Accepts a pending Imported transaction booked in its Account's
// Open month and, through the Ledger in the same database transaction,
// books it with exactly its amount: to the Category, or to Uncategorised
// when it has none. Accepted as a Transfer, the Ledger books the row as
// the bank leg, records the cash leg and Matches them (consistency map:
// Accept as Transfer). A row in a closed month, then one in a month not
// open yet, is refused before its own state is checked, and the Ledger
// checks the month again as it books. The Imported transaction, the Ledger
// Transaction and the Account each check their own version, and a conflict
// retries the whole transaction with the same Transaction ID (ADR-0011,
// ADR-0013, ADR-0024; consistency map: Accept imported transaction).
func (s *ImportedTransactions) Accept(ctx context.Context, household, id uuid.UUID, a Acceptance,
) (ImportedTransaction, error) {
	transaction, err := sharedkernel.NewID()
	if err != nil {
		return ImportedTransaction{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	counterpartLeg, err := sharedkernel.NewID()
	if err != nil {
		return ImportedTransaction{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	var accepted ImportedTransaction

	err = s.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := s.rows.Folds.Load(ctx, tx, id)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !found {
			return ErrImportedTransactionNotFound
		}

		// Not locked: the booking below writes the Account's fold, whose
		// version catches a Close or Reopen that ran meanwhile.
		account, err := s.ledger.ImportAccount(ctx, tx, fold.State.AccountID)
		if err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		if err := closedMonth(fold.State, account.OpenMonth); err != nil {
			return err
		}

		if err := notOpenYet(fold.State, account.OpenMonth); err != nil {
			return err
		}

		res, err := s.rows.Execute(ctx, tx, id, domain.AcceptImportedTransaction{TransactionID: transaction})
		if errors.Is(err, domain.ErrImportedTransactionNotImported) {
			return ErrImportedTransactionNotFound
		}

		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		booking := Booking{Transaction: transaction, ImportedTransaction: id, Category: a.Category, Row: res.State}

		if a.TransferAccount != nil {
			err = s.ledger.BookTransfer(ctx, tx, TransferBooking{
				Booking: booking, Counterpart: *a.TransferAccount, CounterpartLeg: counterpartLeg, MatchLeg: a.MatchLeg,
			})
		} else {
			err = s.ledger.BookAccepted(ctx, tx, booking)
		}

		if err != nil {
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

// AccountImports reports, in the caller's transaction, what the Account's
// Statement imports leave unresolved in every Booking month at once. The
// Ledger's months list reads it.
func (s *ImportedTransactions) AccountImports(ctx context.Context, tx *eventstore.Tx, account uuid.UUID,
) (AccountImports, error) {
	a, err := s.checks.AccountImports(ctx, tx, account)
	if err != nil {
		return AccountImports{}, err //nolint:wrapcheck // already wrapped by the adapter
	}

	return a, nil
}

// UnresolvedRows counts, in the caller's transaction, the pending and
// Suspected duplicate Imported transactions of the Account, or of every
// Account of the Household when account is nil, over every Booking month.
// An Account with none is absent. The Ledger's Account reads use it.
func (s *ImportedTransactions) UnresolvedRows(ctx context.Context, tx *eventstore.Tx, account *uuid.UUID,
) (map[uuid.UUID]RowCounts, error) {
	counts, err := s.checks.UnresolvedRows(ctx, tx, account)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped by the adapter
	}

	return counts, nil
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
// duplicate so it never reaches the Ledger. Discarded is final. A row in a
// closed month is refused first, then the row's own state is checked, and
// then a pending row in a month not open yet is refused; a Suspected
// duplicate there can be Discarded, so its import's waiting Balance check
// can run and the Open month can close (ADR-0024; consistency map: Keep or
// Discard Imported transaction).
func (s *ImportedTransactions) Discard(ctx context.Context, household, id uuid.UUID) (ImportedTransaction, error) {
	return s.decide(ctx, household, id, "discard", domain.DiscardImportedTransaction{}, true)
}

// Keep keeps a Suspected duplicate as a real transaction: it becomes
// pending. A row in a closed month is refused before its own state is
// checked; a Suspected duplicate in a month not open yet can be kept
// (ADR-0024; consistency map: Keep or Discard Imported transaction).
func (s *ImportedTransactions) Keep(ctx context.Context, household, id uuid.UUID) (ImportedTransaction, error) {
	return s.decide(ctx, household, id, "keep", domain.KeepImportedTransaction{}, false)
}

// decide keeps or Discards an Imported transaction. Deciding a Suspected
// duplicate locks its Account's Statement imports first, so of two
// decisions on one import's last Suspected duplicates exactly one sees none
// left and runs the import's waiting Balance check, in the same
// transaction (ADR-0018). The Account's Open month is read with its fold
// locked FOR SHARE, so it can't close or reopen before this commits: a row
// in a closed month is refused, and with pendingNotOpenYet a pending row in
// a month not open yet too, after the Decider checked its state.
func (s *ImportedTransactions) decide(ctx context.Context, household, id uuid.UUID, action string,
	cmd domain.ImportedTransactionCommand, pendingNotOpenYet bool,
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

		months, err := s.ledger.AccountMonthsForShare(ctx, tx, fold.State.AccountID)
		if err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		if err := closedMonth(fold.State, months.OpenMonth); err != nil {
			return err
		}

		res, err := s.rows.Execute(ctx, tx, id, cmd)
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		// The Decider accepted the row's state, so a pending row is one
		// that was pending, not a Suspected duplicate.
		if pendingNotOpenYet && !suspected {
			if err := notOpenYet(fold.State, months.OpenMonth); err != nil {
				return err
			}
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

// SuggestedLeg is a cash Transfer leg suggested as the other side of an
// Imported transaction, as the Ledger records it.
type SuggestedLeg struct {
	TransactionID        uuid.UUID
	AccountID            uuid.UUID
	Amount               sharedkernel.Decimal
	Currency             string
	BookingDate          time.Time
	TransactionDate      time.Time
	Description          string
	Payee                string
	CategoryID           uuid.UUID
	CategoryName         string
	CounterpartAccountID uuid.UUID
}

// LedgerSuggestions is what the Ledger suggests for a row: In transit Legs,
// and StatementAccounts, the bank and credit card Accounts it looked on,
// whose pending rows Ingestion suggests.
type LedgerSuggestions struct {
	Legs              []SuggestedLeg
	StatementAccounts []uuid.UUID
}

// TransferSuggestions is what could be the other side of a row as a
// Transfer: In transit Legs, and pending Rows on bank or credit card
// Accounts, each closest date first.
type TransferSuggestions struct {
	Legs []SuggestedLeg
	Rows []ImportedTransaction
}

// SuggestionWindowDays is how many days either side of a row's Transaction
// date a pending row is suggested for it, as the Ledger's window is for
// legs.
const SuggestionWindowDays = 7

// PendingRowQuery asks for pending rows on Accounts, with Amount in
// Currency, and a Transaction date within SuggestionWindowDays of
// TransactionDate.
type PendingRowQuery struct {
	Accounts        []uuid.UUID
	Amount          sharedkernel.Decimal
	Currency        string
	TransactionDate time.Time
}

// TransferSuggestions works out what could be the other side of the
// Imported transaction as a Transfer, on counterpart or, when it is nil, on
// every Account the row could be a Transfer with, in one transaction
// (consistency map: List transfer suggestions): the In transit legs, read
// through the Ledger port, and the pending rows on the bank and credit card
// Accounts among them, read here, both with the opposite amount, within a
// week either side of the row's Transaction date, closest first. An Account
// the row can't be a Transfer with is a *TransferRefusedError.
func (s *ImportedTransactions) TransferSuggestions(ctx context.Context, household, id uuid.UUID,
	counterpart *uuid.UUID,
) (TransferSuggestions, error) {
	var out TransferSuggestions

	err := s.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := s.rows.Folds.Load(ctx, tx, id)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !found {
			return ErrImportedTransactionNotFound
		}

		row := fold.State

		ledger, err := s.ledger.TransferSuggestions(ctx, tx, row, counterpart)
		if err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		out.Legs = ledger.Legs

		if len(ledger.StatementAccounts) == 0 {
			return nil
		}

		out.Rows, err = s.reader.PendingTransferRows(ctx, tx, PendingRowQuery{
			Accounts: ledger.StatementAccounts, Amount: row.Amount.Neg(), Currency: row.Currency,
			TransactionDate: row.TransactionDate,
		})

		return err //nolint:wrapcheck // the adapter's error
	})
	if err != nil {
		return TransferSuggestions{}, fmt.Errorf("transfer suggestions for imported transaction %s: %w", id, err)
	}

	return out, nil
}
