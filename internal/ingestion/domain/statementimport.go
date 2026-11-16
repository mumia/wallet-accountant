package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

var (
	// ErrStatementImportAlreadyRecorded is returned for recording a
	// Statement import twice.
	ErrStatementImportAlreadyRecorded = errors.New("statement import already recorded")
	// ErrFileHashRequired is returned for a Statement import without the
	// hash of its file, which deduplicates it.
	ErrFileHashRequired = errors.New("statement file hash required")
	// ErrBalanceCheckFailed is returned, as a *BalanceCheckFailedError, for
	// a later import whose rows don't bring the Account to its Export
	// balance.
	ErrBalanceCheckFailed = errors.New("balance check failed")
)

// BalanceCheckFailedError is a failed Balance check (ADR-0018): what the
// Account's Opening balance and Imported transactions, with the new rows,
// add up to, the Export balance they should equal, and the difference
// (Export balance minus expected), in the Account currency.
type BalanceCheckFailedError struct {
	Expected      sharedkernel.Decimal
	ExportBalance sharedkernel.Decimal
	Difference    sharedkernel.Decimal
	Currency      string
}

func (e *BalanceCheckFailedError) Error() string {
	return fmt.Sprintf("%s: expected %s %s, Export balance %s, difference %s",
		ErrBalanceCheckFailed, e.Expected, e.Currency, e.ExportBalance, e.Difference)
}

func (e *BalanceCheckFailedError) Unwrap() error { return ErrBalanceCheckFailed }

// BalanceCheck is the state of a Statement import's Balance check.
type BalanceCheck string

// The Balance check states. The first import passes by construction.
const (
	BalanceCheckPassed BalanceCheck = "passed"
)

// StatementImportState is a Statement import's fold.
type StatementImportState struct {
	Recorded       bool
	ConnectionID   sharedkernel.ID
	AccountID      sharedkernel.ID
	FileHash       string
	MappingVersion int
	Currency       string
	ExportBalance  sharedkernel.Decimal
	RowsImported   int
	// RowsSkipped counts the certain repeats left out (spec story 38).
	RowsSkipped int
	RowsIgnored int
	RowsTotal   sharedkernel.Decimal
	// OpeningBalance is set only for the Account's first import.
	OpeningBalance *sharedkernel.Decimal
	BalanceCheck   BalanceCheck
	RecordedAt     time.Time
}

type (
	// StatementImportCommand is a command to a Statement import.
	StatementImportCommand interface{ isStatementImportCommand() }

	// RecordStatementImport records one upload of a statement file: its
	// Export balance and what its rows came to, certain repeats already
	// skipped. First marks the Account's first import, which works out the
	// Opening balance. A later import carries the Account's Opening balance
	// and the total of its earlier non-Discarded Imported transactions, for
	// its Balance check.
	RecordStatementImport struct {
		ConnectionID   sharedkernel.ID
		AccountID      sharedkernel.ID
		FileHash       string
		MappingVersion int
		Currency       string
		ExportBalance  sharedkernel.Decimal
		RowsImported   int
		RowsSkipped    int
		RowsIgnored    int
		RowsTotal      sharedkernel.Decimal
		First          bool
		OpeningBalance sharedkernel.Decimal
		EarlierTotal   sharedkernel.Decimal
	}
)

func (RecordStatementImport) isStatementImportCommand() {}

type (
	// StatementImportEvent is a Statement import's event.
	StatementImportEvent interface{ isStatementImportEvent() }

	// StatementImportRecorded is recorded when a statement file is
	// imported. OpeningBalance is set only for the Account's first import.
	StatementImportRecorded struct {
		ConnectionID   sharedkernel.ID       `json:"connectionid"`
		AccountID      sharedkernel.ID       `json:"accountid"`
		FileHash       string                `json:"filehash"`
		MappingVersion int                   `json:"mappingversion"`
		Currency       string                `json:"currency"`
		ExportBalance  sharedkernel.Decimal  `json:"exportbalance"`
		RowsImported   int                   `json:"rowsimported"`
		RowsSkipped    int                   `json:"rowsskipped,omitempty"`
		RowsIgnored    int                   `json:"rowsignored"`
		RowsTotal      sharedkernel.Decimal  `json:"rowstotal"`
		OpeningBalance *sharedkernel.Decimal `json:"openingbalance,omitempty"`
		BalanceCheck   BalanceCheck          `json:"balancecheck"`
	}
)

func (StatementImportRecorded) isStatementImportEvent() {}

// StatementImport is the Statement import Decider (ADR-0018).
var StatementImport = decider.Decider[StatementImportState, StatementImportCommand, StatementImportEvent]{
	Decide: decideStatementImport,
	Evolve: evolveStatementImport,
}

func decideStatementImport(s StatementImportState, c StatementImportCommand) ([]StatementImportEvent, error) {
	switch c := c.(type) {
	case RecordStatementImport:
		return recordStatementImport(s, c)
	default:
		return nil, nil
	}
}

func recordStatementImport(s StatementImportState, c RecordStatementImport) ([]StatementImportEvent, error) {
	if s.Recorded {
		return nil, ErrStatementImportAlreadyRecorded
	}

	if c.FileHash == "" {
		return nil, ErrFileHashRequired
	}

	currency, err := sharedkernel.ParseCurrency(c.Currency)
	if err != nil {
		return nil, err //nolint:wrapcheck // the shared kernel's own refusal
	}

	if err := CheckExportBalance(c.ExportBalance, currency); err != nil {
		return nil, err
	}

	recorded := StatementImportRecorded{
		ConnectionID:   c.ConnectionID,
		AccountID:      c.AccountID,
		FileHash:       c.FileHash,
		MappingVersion: c.MappingVersion,
		Currency:       currency.Code(),
		ExportBalance:  c.ExportBalance,
		RowsImported:   c.RowsImported,
		RowsSkipped:    c.RowsSkipped,
		RowsIgnored:    c.RowsIgnored,
		RowsTotal:      c.RowsTotal,
		BalanceCheck:   BalanceCheckPassed,
	}

	if c.First {
		// The first import has nothing earlier to check against: its
		// Opening balance makes the Balance check pass by construction.
		opening := c.ExportBalance.Sub(c.RowsTotal)
		recorded.OpeningBalance = &opening

		return []StatementImportEvent{recorded}, nil
	}

	// A later import's Balance check: the Opening balance plus every
	// non-Discarded Imported transaction, this file's rows included, must
	// be the Export balance. Ledger Transactions aren't used: they lag
	// while rows wait for review (ADR-0018).
	expected := c.OpeningBalance.Add(c.EarlierTotal).Add(c.RowsTotal)
	if !expected.Equal(c.ExportBalance) {
		return nil, &BalanceCheckFailedError{
			Expected:      expected,
			ExportBalance: c.ExportBalance,
			Difference:    c.ExportBalance.Sub(expected),
			Currency:      currency.Code(),
		}
	}

	return []StatementImportEvent{recorded}, nil
}

func evolveStatementImport(s StatementImportState, e decider.Envelope[StatementImportEvent]) StatementImportState {
	if ev, ok := e.Event.(StatementImportRecorded); ok {
		s = StatementImportState{
			Recorded:       true,
			ConnectionID:   ev.ConnectionID,
			AccountID:      ev.AccountID,
			FileHash:       ev.FileHash,
			MappingVersion: ev.MappingVersion,
			Currency:       ev.Currency,
			ExportBalance:  ev.ExportBalance,
			RowsImported:   ev.RowsImported,
			RowsSkipped:    ev.RowsSkipped,
			RowsIgnored:    ev.RowsIgnored,
			RowsTotal:      ev.RowsTotal,
			OpeningBalance: ev.OpeningBalance,
			BalanceCheck:   ev.BalanceCheck,
			RecordedAt:     e.Time,
		}
	}

	return s
}

// ImportedState is where an Imported transaction is in its review.
type ImportedState string

// The Imported transaction states this slice has so far.
const (
	ImportedPending  ImportedState = "pending"
	ImportedAccepted ImportedState = "accepted"
)

var (
	// ErrRowAlreadyImported is returned for importing a row twice.
	ErrRowAlreadyImported = errors.New("imported transaction already recorded")
	// ErrInvalidRowAmount is returned for a row whose amount isn't money in
	// a known currency with that currency's decimal places.
	ErrInvalidRowAmount = errors.New("invalid imported transaction amount")
	// ErrImportedTransactionNotImported is returned for a command to an
	// Imported transaction that doesn't exist.
	ErrImportedTransactionNotImported = errors.New("imported transaction not imported")
	// ErrImportedTransactionAlreadyAccepted is returned for accepting an
	// Imported transaction that is already Accepted: it is booked once.
	ErrImportedTransactionAlreadyAccepted = errors.New("imported transaction already accepted")
	// ErrTransactionIDRequired is returned for accepting without the Ledger
	// Transaction that books it.
	ErrTransactionIDRequired = errors.New("ledger transaction ID required")
	// ErrImportedTransactionNotPending is returned for accepting an
	// Imported transaction that isn't waiting for review.
	ErrImportedTransactionNotPending = errors.New("imported transaction is not pending")
)

// ImportedTransactionState is an Imported transaction's fold.
type ImportedTransactionState struct {
	Imported          bool
	State             ImportedState
	StatementImportID sharedkernel.ID
	AccountID         sharedkernel.ID
	Line              int
	BookingDate       time.Time
	TransactionDate   time.Time
	// ValueDate is zero when the file has none.
	ValueDate         time.Time
	Amount            sharedkernel.Decimal
	Currency          string
	Description       string
	Payee             string
	BankTransactionID string
	// TransactionID is the Ledger Transaction that booked it while it is
	// Accepted; the zero ID otherwise.
	TransactionID sharedkernel.ID
}

type (
	// ImportedTransactionCommand is a command to an Imported transaction.
	ImportedTransactionCommand interface{ isImportedTransactionCommand() }

	// ImportRow records one row of a Statement import, in the Account
	// currency. TransactionDate is the Booking date when the file has none;
	// ValueDate may be zero.
	ImportRow struct {
		StatementImportID sharedkernel.ID
		AccountID         sharedkernel.ID
		Line              int
		BookingDate       time.Time
		TransactionDate   time.Time
		ValueDate         time.Time
		Amount            sharedkernel.Decimal
		Currency          string
		Description       string
		Payee             string
		BankTransactionID string
	}

	// AcceptImportedTransaction Accepts a pending Imported transaction,
	// booked by the Ledger Transaction TransactionID in the same database
	// transaction (ADR-0013).
	AcceptImportedTransaction struct {
		TransactionID sharedkernel.ID
	}
)

func (ImportRow) isImportedTransactionCommand()                 {}
func (AcceptImportedTransaction) isImportedTransactionCommand() {}

type (
	// ImportedTransactionEvent is an Imported transaction's event.
	ImportedTransactionEvent interface{ isImportedTransactionEvent() }

	// TransactionImported is recorded when a row becomes a pending Imported
	// transaction. Dates are written YYYY-MM-DD; ValueDate is empty when the
	// file has none.
	TransactionImported struct {
		StatementImportID sharedkernel.ID      `json:"statementimportid"`
		AccountID         sharedkernel.ID      `json:"accountid"`
		Line              int                  `json:"line"`
		BookingDate       string               `json:"bookingdate"`
		TransactionDate   string               `json:"transactiondate"`
		ValueDate         string               `json:"valuedate,omitempty"`
		Amount            sharedkernel.Decimal `json:"amount"`
		Currency          string               `json:"currency"`
		Description       string               `json:"description"`
		Payee             string               `json:"payee,omitempty"`
		BankTransactionID string               `json:"banktransactionid,omitempty"`
	}

	// ImportedTransactionAccepted is recorded when a User Accepts a pending
	// Imported transaction: TransactionID is the Ledger Transaction that
	// booked it.
	ImportedTransactionAccepted struct {
		TransactionID sharedkernel.ID `json:"transactionid"`
	}
)

func (TransactionImported) isImportedTransactionEvent()         {}
func (ImportedTransactionAccepted) isImportedTransactionEvent() {}

// DateLayout is how events write a calendar date.
const DateLayout = time.DateOnly

// ImportedTransaction is the Imported transaction Decider.
var ImportedTransaction = decider.Decider[ImportedTransactionState, ImportedTransactionCommand, ImportedTransactionEvent]{
	Decide: decideImportedTransaction,
	Evolve: evolveImportedTransaction,
}

func decideImportedTransaction(s ImportedTransactionState, c ImportedTransactionCommand) ([]ImportedTransactionEvent, error) {
	switch c := c.(type) {
	case ImportRow:
		return importRow(s, c)
	case AcceptImportedTransaction:
		return acceptImportedTransaction(s, c)
	default:
		return nil, nil
	}
}

func importRow(s ImportedTransactionState, c ImportRow) ([]ImportedTransactionEvent, error) {
	if s.Imported {
		return nil, ErrRowAlreadyImported
	}

	currency, err := sharedkernel.ParseCurrency(c.Currency)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRowAmount, err)
	}

	if !c.Amount.Equal(c.Amount.Round(int32(currency.Decimals()))) { //nolint:gosec // at most 18 places
		return nil, fmt.Errorf("%w: %s %s", ErrInvalidRowAmount, c.Amount, currency.Code())
	}

	transactionDate := c.TransactionDate
	if transactionDate.IsZero() {
		transactionDate = c.BookingDate
	}

	imported := TransactionImported{
		StatementImportID: c.StatementImportID,
		AccountID:         c.AccountID,
		Line:              c.Line,
		BookingDate:       c.BookingDate.Format(DateLayout),
		TransactionDate:   transactionDate.Format(DateLayout),
		Amount:            c.Amount,
		Currency:          currency.Code(),
		Description:       c.Description,
		Payee:             c.Payee,
		BankTransactionID: c.BankTransactionID,
	}

	if !c.ValueDate.IsZero() {
		imported.ValueDate = c.ValueDate.Format(DateLayout)
	}

	return []ImportedTransactionEvent{imported}, nil
}

// acceptImportedTransaction Accepts a pending Imported transaction once, so
// its money is never booked twice (spec story 48).
func acceptImportedTransaction(s ImportedTransactionState, c AcceptImportedTransaction) ([]ImportedTransactionEvent, error) {
	switch {
	case !s.Imported:
		return nil, ErrImportedTransactionNotImported
	case s.State == ImportedAccepted:
		return nil, ErrImportedTransactionAlreadyAccepted
	case s.State != ImportedPending:
		return nil, fmt.Errorf("%w: it is %s", ErrImportedTransactionNotPending, s.State)
	case c.TransactionID == sharedkernel.NilID:
		return nil, ErrTransactionIDRequired
	}

	return []ImportedTransactionEvent{ImportedTransactionAccepted(c)}, nil
}

func evolveImportedTransaction(s ImportedTransactionState, e decider.Envelope[ImportedTransactionEvent]) ImportedTransactionState {
	switch ev := e.Event.(type) {
	case ImportedTransactionAccepted:
		s.State, s.TransactionID = ImportedAccepted, ev.TransactionID
	case TransactionImported:
		s = ImportedTransactionState{
			Imported:          true,
			State:             ImportedPending,
			StatementImportID: ev.StatementImportID,
			AccountID:         ev.AccountID,
			Line:              ev.Line,
			Amount:            ev.Amount,
			Currency:          ev.Currency,
			Description:       ev.Description,
			Payee:             ev.Payee,
			BankTransactionID: ev.BankTransactionID,
		}
		// The event was written with DateLayout, so the dates always parse.
		s.BookingDate, _ = time.Parse(DateLayout, ev.BookingDate)
		s.TransactionDate, _ = time.Parse(DateLayout, ev.TransactionDate)

		if ev.ValueDate != "" {
			s.ValueDate, _ = time.Parse(DateLayout, ev.ValueDate)
		}
	}

	return s
}
