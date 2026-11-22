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
	// ErrStatementImportNotRecorded is returned for voiding a Statement
	// import that doesn't exist.
	ErrStatementImportNotRecorded = errors.New("statement import not recorded")
	// ErrStatementImportAlreadyVoided is returned for voiding a Statement
	// import twice.
	ErrStatementImportAlreadyVoided = errors.New("statement import already voided")
	// ErrNotTheLatestStatementImport is returned for voiding a Statement
	// import that isn't the Account's latest non-voided one, so every
	// remaining import's Balance check stays valid (ADR-0018).
	ErrNotTheLatestStatementImport = errors.New("not the account's latest statement import")
	// ErrStatementImportHasAcceptedRows is returned for voiding a Statement
	// import with Accepted rows: their Transactions are reversed first.
	ErrStatementImportHasAcceptedRows = errors.New("statement import has accepted rows")
	// ErrStatementImportTouchesClosedMonth is returned, as a
	// *TouchesClosedMonthError, for voiding a Statement import that touches
	// a closed month: voiding never changes a closed month (ADR-0024).
	ErrStatementImportTouchesClosedMonth = errors.New("statement import touches a closed month")
	// ErrBalanceCheckNotWaiting is returned for running the Balance check
	// of a Statement import whose check isn't waiting.
	ErrBalanceCheckNotWaiting = errors.New("balance check is not waiting")
)

// BalanceCheckFailedError is a failed Balance check (ADR-0018): what the
// Account's Opening balance and Imported transactions, with the new rows,
// add up to, the Export balance they should equal, and the difference
// (Export balance minus expected), in the Account currency.
// ClosedMonthsNotInBooks counts, per closed month, the file's rows there
// that aren't in the books, which may explain the difference (ADR-0024).
type BalanceCheckFailedError struct {
	Expected               sharedkernel.Decimal
	ExportBalance          sharedkernel.Decimal
	Difference             sharedkernel.Decimal
	Currency               string
	ClosedMonthsNotInBooks []MonthRows
}

// RowsInClosedMonthsNotInBooks is how many of the file's rows in closed
// months aren't in the books.
func (e *BalanceCheckFailedError) RowsInClosedMonthsNotInBooks() int {
	n := 0
	for _, m := range e.ClosedMonthsNotInBooks {
		n += m.Rows
	}

	return n
}

func (e *BalanceCheckFailedError) Error() string {
	return fmt.Sprintf("%s: expected %s %s, Export balance %s, difference %s",
		ErrBalanceCheckFailed, e.Expected, e.Currency, e.ExportBalance, e.Difference)
}

func (e *BalanceCheckFailedError) Unwrap() error { return ErrBalanceCheckFailed }

// TouchesClosedMonthError refuses voiding a Statement import that touches
// a closed month: Month is the earliest closed month it touches, and
// OpenMonth the Account's Open month (both YYYY-MM).
type TouchesClosedMonthError struct {
	Month     string
	OpenMonth string
}

func (e *TouchesClosedMonthError) Error() string {
	return fmt.Sprintf("%s: %s (the open month is %s)", ErrStatementImportTouchesClosedMonth, e.Month, e.OpenMonth)
}

func (e *TouchesClosedMonthError) Unwrap() error { return ErrStatementImportTouchesClosedMonth }

// VoidBlock is a reason voiding a Statement import would be refused.
type VoidBlock string

// The reasons a Void is refused, in the order they are checked.
const (
	VoidBlockedNotLatest          VoidBlock = "not_latest"
	VoidBlockedHasAcceptedRows    VoidBlock = "has_accepted_rows"
	VoidBlockedTouchesClosedMonth VoidBlock = "touches_closed_month"
)

// BalanceCheck is the state of a Statement import's Balance check.
type BalanceCheck string

// The Balance check states. The first import passes by construction. A
// later import with Suspected duplicates waits until each is decided, and
// then passes or fails.
const (
	BalanceCheckPassed  BalanceCheck = "passed"
	BalanceCheckWaiting BalanceCheck = "waiting"
	BalanceCheckFailed  BalanceCheck = "failed"
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
	// SuspectedDuplicates counts the imported rows that were Suspected
	// duplicates (spec story 40).
	SuspectedDuplicates int
	RowsIgnored         int
	// RowsIgnoredInClosedMonths counts the rows booked in a closed month,
	// which are ignored (ADR-0024).
	RowsIgnoredInClosedMonths int
	// EarliestBookingMonth is the earliest Booking month (YYYY-MM) of the
	// Imported transactions it recorded; empty when it recorded none.
	EarliestBookingMonth string
	RowsTotal            sharedkernel.Decimal
	// OpeningBalance is set only for the Account's first import.
	OpeningBalance *sharedkernel.Decimal
	BalanceCheck   BalanceCheck
	RecordedAt     time.Time
	// VoidedAt is when the import was voided; the zero time while it isn't.
	VoidedAt time.Time
}

// Voided reports whether the import was voided: its rows are Discarded and
// its file can be imported again.
func (s StatementImportState) Voided() bool { return !s.VoidedAt.IsZero() }

type (
	// StatementImportCommand is a command to a Statement import.
	StatementImportCommand interface{ isStatementImportCommand() }

	// RecordStatementImport records one upload of a statement file: its
	// Export balance and what its rows came to, certain repeats already
	// skipped. First marks the Account's first import, which works out the
	// Opening balance. A later import carries the Account's Opening balance
	// and the total of its earlier non-Discarded Imported transactions, for
	// its Balance check. RowsImported includes the SuspectedDuplicates.
	// RowsIgnoredInClosedMonths counts the rows booked in a closed month,
	// and ClosedMonthsNotInBooks those of them that aren't in the books,
	// which a failed check names. EarliestBookingMonth is that of the rows
	// imported.
	RecordStatementImport struct {
		ConnectionID              sharedkernel.ID
		AccountID                 sharedkernel.ID
		FileHash                  string
		MappingVersion            int
		Currency                  string
		ExportBalance             sharedkernel.Decimal
		RowsImported              int
		RowsSkipped               int
		SuspectedDuplicates       int
		RowsIgnored               int
		RowsIgnoredInClosedMonths int
		ClosedMonthsNotInBooks    []MonthRows
		EarliestBookingMonth      string
		RowsTotal                 sharedkernel.Decimal
		First                     bool
		OpeningBalance            sharedkernel.Decimal
		EarlierTotal              sharedkernel.Decimal
	}

	// RunBalanceCheck runs a waiting Balance check once the import's last
	// Suspected duplicate is decided: OpeningBalance is the Account's, and
	// Total that of its non-Discarded Imported transactions from this
	// import and the ones before it.
	RunBalanceCheck struct {
		OpeningBalance sharedkernel.Decimal
		Total          sharedkernel.Decimal
	}

	// VoidStatementImport withdraws the import (ADR-0018). Latest says it
	// is the Account's latest non-voided import; AcceptedRows counts its
	// Accepted Imported transactions. OpenMonth and StartMonth (YYYY-MM)
	// are the Account's Open month and its start date's month, which say
	// whether the import touches a closed month (ADR-0024).
	VoidStatementImport struct {
		Latest       bool
		AcceptedRows int
		OpenMonth    string
		StartMonth   string
	}
)

func (RecordStatementImport) isStatementImportCommand() {}
func (VoidStatementImport) isStatementImportCommand()   {}
func (RunBalanceCheck) isStatementImportCommand()       {}

type (
	// StatementImportEvent is a Statement import's event.
	StatementImportEvent interface{ isStatementImportEvent() }

	// StatementImportRecorded is recorded when a statement file is
	// imported. OpeningBalance is set only for the Account's first import.
	// SuspectedDuplicates counts the rows imported as Suspected duplicates,
	// and the Balance check waits for them; this event is what tells the
	// Household (spec story 40a).
	StatementImportRecorded struct {
		ConnectionID        sharedkernel.ID      `json:"connectionid"`
		AccountID           sharedkernel.ID      `json:"accountid"`
		FileHash            string               `json:"filehash"`
		MappingVersion      int                  `json:"mappingversion"`
		Currency            string               `json:"currency"`
		ExportBalance       sharedkernel.Decimal `json:"exportbalance"`
		RowsImported        int                  `json:"rowsimported"`
		RowsSkipped         int                  `json:"rowsskipped,omitempty"`
		SuspectedDuplicates int                  `json:"suspectedduplicates,omitempty"`
		RowsIgnored         int                  `json:"rowsignored"`
		// RowsIgnoredInClosedMonths counts the rows booked in a closed
		// month, which are ignored (ADR-0024).
		RowsIgnoredInClosedMonths int `json:"rowsignoredinclosedmonths,omitempty"`
		// EarliestBookingMonth is the earliest Booking month (YYYY-MM) of
		// the rows imported; empty when none were.
		EarliestBookingMonth string                `json:"earliestbookingmonth,omitempty"`
		RowsTotal            sharedkernel.Decimal  `json:"rowstotal"`
		OpeningBalance       *sharedkernel.Decimal `json:"openingbalance,omitempty"`
		BalanceCheck         BalanceCheck          `json:"balancecheck"`
	}

	// WaitingCheckPassed is recorded when a waiting Balance check runs and
	// passes.
	WaitingCheckPassed struct{}

	// WaitingCheckFailed is recorded when a waiting Balance check runs and
	// fails: Expected is what the Account's Opening balance and Imported
	// transactions come to, and Difference the Export balance minus it.
	WaitingCheckFailed struct {
		Expected   sharedkernel.Decimal `json:"expected"`
		Difference sharedkernel.Decimal `json:"difference"`
	}
)

// StatementImportVoided is recorded when a Statement import is voided.
type StatementImportVoided struct{}

func (StatementImportRecorded) isStatementImportEvent() {}
func (StatementImportVoided) isStatementImportEvent()   {}
func (WaitingCheckPassed) isStatementImportEvent()      {}
func (WaitingCheckFailed) isStatementImportEvent()      {}

// StatementImport is the Statement import Decider (ADR-0018).
var StatementImport = decider.Decider[StatementImportState, StatementImportCommand, StatementImportEvent]{
	Decide: decideStatementImport,
	Evolve: evolveStatementImport,
}

func decideStatementImport(s StatementImportState, c StatementImportCommand) ([]StatementImportEvent, error) {
	switch c := c.(type) {
	case RecordStatementImport:
		return recordStatementImport(s, c)
	case VoidStatementImport:
		return voidStatementImport(s, c)
	case RunBalanceCheck:
		return runBalanceCheck(s, c)
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

		RowsIgnoredInClosedMonths: c.RowsIgnoredInClosedMonths,
		EarliestBookingMonth:      c.EarliestBookingMonth,
	}

	recorded.SuspectedDuplicates = c.SuspectedDuplicates

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
	// while rows wait for review (ADR-0018). With Suspected duplicates the
	// check waits until each is kept or Discarded.
	if c.SuspectedDuplicates > 0 {
		recorded.BalanceCheck = BalanceCheckWaiting

		return []StatementImportEvent{recorded}, nil
	}

	expected := c.OpeningBalance.Add(c.EarlierTotal).Add(c.RowsTotal)
	if !expected.Equal(c.ExportBalance) {
		return nil, &BalanceCheckFailedError{
			Expected:      expected,
			ExportBalance: c.ExportBalance,
			Difference:    c.ExportBalance.Sub(expected),
			Currency:      currency.Code(),

			ClosedMonthsNotInBooks: c.ClosedMonthsNotInBooks,
		}
	}

	return []StatementImportEvent{recorded}, nil
}

// ClosedMonthTouched is the earliest closed month (YYYY-MM) the import
// touches, before the Account's Open month openMonth: the earliest Booking
// month of the Imported transactions it recorded, in any state, or, for
// the first import, the start date's month startMonth, where its Opening
// balance is booked. Empty when it touches none. Rows it ignored never
// count (ADR-0024).
func (s StatementImportState) ClosedMonthTouched(openMonth, startMonth string) string {
	touched := ""

	if s.OpeningBalance != nil && startMonth != "" && startMonth < openMonth {
		touched = startMonth
	}

	if m := s.EarliestBookingMonth; m != "" && m < openMonth && (touched == "" || m < touched) {
		touched = m
	}

	return touched
}

// VoidBlockedBy is every reason voiding the import would be refused now,
// in the order they are checked; none for an import that is voided, or
// isn't recorded.
func (s StatementImportState) VoidBlockedBy(c VoidStatementImport) []VoidBlock {
	if !s.Recorded || s.Voided() {
		return nil
	}

	var blocks []VoidBlock

	if !c.Latest {
		blocks = append(blocks, VoidBlockedNotLatest)
	}

	if c.AcceptedRows > 0 {
		blocks = append(blocks, VoidBlockedHasAcceptedRows)
	}

	if s.ClosedMonthTouched(c.OpenMonth, c.StartMonth) != "" {
		blocks = append(blocks, VoidBlockedTouchesClosedMonth)
	}

	return blocks
}

// voidStatementImport voids the Account's latest import while none of its
// rows are Accepted (spec stories 43d, 43f) and it touches no closed month
// (spec story 14).
func voidStatementImport(s StatementImportState, c VoidStatementImport) ([]StatementImportEvent, error) {
	switch {
	case !s.Recorded:
		return nil, ErrStatementImportNotRecorded
	case s.Voided():
		return nil, ErrStatementImportAlreadyVoided
	case !c.Latest:
		return nil, ErrNotTheLatestStatementImport
	case c.AcceptedRows > 0:
		return nil, fmt.Errorf("%w: %d", ErrStatementImportHasAcceptedRows, c.AcceptedRows)
	}

	if m := s.ClosedMonthTouched(c.OpenMonth, c.StartMonth); m != "" {
		return nil, &TouchesClosedMonthError{Month: m, OpenMonth: c.OpenMonth}
	}

	return []StatementImportEvent{StatementImportVoided{}}, nil
}

// runBalanceCheck runs a waiting Balance check: the Opening balance plus
// the non-Discarded Imported transactions must be the Export balance (spec
// story 43c). A failed check doesn't undo the import: it blocks Month close
// until the import is voided. A voided import's check never runs.
func runBalanceCheck(s StatementImportState, c RunBalanceCheck) ([]StatementImportEvent, error) {
	if !s.Recorded || s.Voided() || s.BalanceCheck != BalanceCheckWaiting {
		return nil, ErrBalanceCheckNotWaiting
	}

	expected := c.OpeningBalance.Add(c.Total)
	if !expected.Equal(s.ExportBalance) {
		return []StatementImportEvent{WaitingCheckFailed{Expected: expected, Difference: s.ExportBalance.Sub(expected)}}, nil
	}

	return []StatementImportEvent{WaitingCheckPassed{}}, nil
}

func evolveStatementImport(s StatementImportState, e decider.Envelope[StatementImportEvent]) StatementImportState {
	switch e.Event.(type) {
	case StatementImportVoided:
		s.VoidedAt = e.Time
	case WaitingCheckPassed:
		s.BalanceCheck = BalanceCheckPassed
	case WaitingCheckFailed:
		s.BalanceCheck = BalanceCheckFailed
	}

	if ev, ok := e.Event.(StatementImportRecorded); ok {
		s = StatementImportState{
			Recorded:            true,
			ConnectionID:        ev.ConnectionID,
			AccountID:           ev.AccountID,
			FileHash:            ev.FileHash,
			MappingVersion:      ev.MappingVersion,
			Currency:            ev.Currency,
			ExportBalance:       ev.ExportBalance,
			RowsImported:        ev.RowsImported,
			RowsSkipped:         ev.RowsSkipped,
			SuspectedDuplicates: ev.SuspectedDuplicates,
			RowsIgnored:         ev.RowsIgnored,
			RowsTotal:           ev.RowsTotal,

			RowsIgnoredInClosedMonths: ev.RowsIgnoredInClosedMonths,
			EarliestBookingMonth:      ev.EarliestBookingMonth,
			OpeningBalance:            ev.OpeningBalance,
			BalanceCheck:              ev.BalanceCheck,
			RecordedAt:                e.Time,
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
	// ImportedDiscarded never reaches the Ledger, and is final. The Balance
	// check leaves it out by this stored value.
	ImportedDiscarded ImportedState = "discarded"
	// ImportedSuspectedDuplicate only looks like an earlier Imported
	// transaction and waits to be kept or Discarded.
	ImportedSuspectedDuplicate ImportedState = "suspected_duplicate"
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
	// ErrImportedTransactionNotAccepted is returned for returning an
	// Imported transaction to pending that isn't Accepted.
	ErrImportedTransactionNotAccepted = errors.New("imported transaction is not accepted")
	// ErrNotBookedByTransaction is returned for returning an Imported
	// transaction to pending for a Transaction that didn't book it.
	ErrNotBookedByTransaction = errors.New("imported transaction was not booked by that transaction")
	// ErrImportedTransactionDiscarded is returned for accepting a Discarded
	// Imported transaction: Discarded is final.
	ErrImportedTransactionDiscarded = errors.New("imported transaction is discarded")
	// ErrImportedTransactionAlreadyDiscarded is returned for discarding an
	// Imported transaction twice.
	ErrImportedTransactionAlreadyDiscarded = errors.New("imported transaction already discarded")
	// ErrImportedTransactionNotSuspectedDuplicate is returned for keeping
	// an Imported transaction that isn't a Suspected duplicate.
	ErrImportedTransactionNotSuspectedDuplicate = errors.New("imported transaction is not a suspected duplicate")
	// ErrImportedTransactionSuspectedDuplicate is returned for accepting a
	// Suspected duplicate: it is kept or Discarded first.
	ErrImportedTransactionSuspectedDuplicate = errors.New("imported transaction is a suspected duplicate")
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
	// Resembles is the earlier Imported transaction it was imported as a
	// Suspected duplicate of; the zero ID otherwise.
	Resembles sharedkernel.ID
}

type (
	// ImportedTransactionCommand is a command to an Imported transaction.
	ImportedTransactionCommand interface{ isImportedTransactionCommand() }

	// ImportRow records one row of a Statement import, in the Account
	// currency. TransactionDate is the Booking date when the file has none;
	// ValueDate may be zero. A row that Resembles an earlier Imported
	// transaction is a Suspected duplicate.
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
		Resembles         sharedkernel.ID
	}

	// AcceptImportedTransaction Accepts a pending Imported transaction,
	// booked by the Ledger Transaction TransactionID in the same database
	// transaction (ADR-0013).
	AcceptImportedTransaction struct {
		TransactionID sharedkernel.ID
	}

	// ReturnToPending returns an Accepted Imported transaction to pending
	// when the Ledger Transaction TransactionID that booked it is reversed,
	// in the same database transaction (ADR-0018).
	ReturnToPending struct {
		TransactionID sharedkernel.ID
	}

	// DiscardImportedTransaction Discards a pending Imported transaction
	// or a Suspected duplicate, so it never reaches the Ledger.
	DiscardImportedTransaction struct{}

	// DiscardForVoid Discards a row of a Statement import being voided:
	// any state but Accepted. A row already Discarded stays as it is.
	DiscardForVoid struct{}
	// KeepImportedTransaction keeps a Suspected duplicate as a real
	// transaction: it becomes pending.
	KeepImportedTransaction struct{}
)

func (ImportRow) isImportedTransactionCommand()                  {}
func (AcceptImportedTransaction) isImportedTransactionCommand()  {}
func (ReturnToPending) isImportedTransactionCommand()            {}
func (DiscardImportedTransaction) isImportedTransactionCommand() {}
func (DiscardForVoid) isImportedTransactionCommand()             {}
func (KeepImportedTransaction) isImportedTransactionCommand()    {}

type (
	// ImportedTransactionEvent is an Imported transaction's event.
	ImportedTransactionEvent interface{ isImportedTransactionEvent() }

	// TransactionImported is recorded when a row becomes an Imported
	// transaction: pending, or a Suspected duplicate when it Resembles an
	// earlier one. Dates are written YYYY-MM-DD; ValueDate is empty when the
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
		Resembles         sharedkernel.ID      `json:"resembles,omitzero"`
	}

	// ImportedTransactionAccepted is recorded when a User Accepts a pending
	// Imported transaction: TransactionID is the Ledger Transaction that
	// booked it.
	ImportedTransactionAccepted struct {
		TransactionID sharedkernel.ID `json:"transactionid"`
	}

	// ImportedTransactionReturnedToPending is recorded when the Ledger
	// Transaction TransactionID that booked it is reversed.
	ImportedTransactionReturnedToPending struct {
		TransactionID sharedkernel.ID `json:"transactionid"`
	}

	// ImportedTransactionDiscarded is recorded when a pending Imported
	// transaction or a Suspected duplicate is Discarded, or when any
	// non-Accepted row of a voided Statement import is.
	ImportedTransactionDiscarded struct{}

	// ImportedTransactionKept is recorded when a Suspected duplicate is
	// kept as a real transaction.
	ImportedTransactionKept struct{}
)

func (TransactionImported) isImportedTransactionEvent()                  {}
func (ImportedTransactionAccepted) isImportedTransactionEvent()          {}
func (ImportedTransactionReturnedToPending) isImportedTransactionEvent() {}
func (ImportedTransactionDiscarded) isImportedTransactionEvent()         {}
func (ImportedTransactionKept) isImportedTransactionEvent()              {}

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
	case ReturnToPending:
		return returnToPending(s, c)
	case DiscardImportedTransaction:
		return discardImportedTransaction(s)
	case DiscardForVoid:
		return discardForVoid(s)
	case KeepImportedTransaction:
		return keepImportedTransaction(s)
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
		Resembles:         c.Resembles,
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
	case s.State == ImportedDiscarded:
		return nil, ErrImportedTransactionDiscarded
	case s.State == ImportedSuspectedDuplicate:
		return nil, ErrImportedTransactionSuspectedDuplicate
	case s.State != ImportedPending:
		return nil, fmt.Errorf("%w: it is %s", ErrImportedTransactionNotPending, s.State)
	case c.TransactionID == sharedkernel.NilID:
		return nil, ErrTransactionIDRequired
	}

	return []ImportedTransactionEvent{ImportedTransactionAccepted(c)}, nil
}

// returnToPending unlinks an Accepted Imported transaction from the
// Transaction that booked it, which was reversed (spec story 51d).
func returnToPending(s ImportedTransactionState, c ReturnToPending) ([]ImportedTransactionEvent, error) {
	switch {
	case !s.Imported:
		return nil, ErrImportedTransactionNotImported
	case s.State != ImportedAccepted:
		return nil, fmt.Errorf("%w: it is %s", ErrImportedTransactionNotAccepted, s.State)
	case s.TransactionID != c.TransactionID:
		return nil, ErrNotBookedByTransaction
	}

	return []ImportedTransactionEvent{ImportedTransactionReturnedToPending(c)}, nil
}

// discardImportedTransaction Discards a pending Imported transaction or a
// Suspected duplicate; an Accepted one is reversed instead, and Discarded
// is final (spec stories 40d, 51e).
func discardImportedTransaction(s ImportedTransactionState) ([]ImportedTransactionEvent, error) {
	switch {
	case !s.Imported:
		return nil, ErrImportedTransactionNotImported
	case s.State == ImportedDiscarded:
		return nil, ErrImportedTransactionAlreadyDiscarded
	case s.State == ImportedAccepted:
		return nil, ErrImportedTransactionAlreadyAccepted
	case s.State != ImportedPending && s.State != ImportedSuspectedDuplicate:
		return nil, fmt.Errorf("%w: it is %s", ErrImportedTransactionNotPending, s.State)
	}

	return []ImportedTransactionEvent{ImportedTransactionDiscarded{}}, nil
}

// discardForVoid Discards every non-Accepted row of a voided import:
// pending, Suspected duplicate or kept (spec story 43d).
func discardForVoid(s ImportedTransactionState) ([]ImportedTransactionEvent, error) {
	switch {
	case !s.Imported:
		return nil, ErrImportedTransactionNotImported
	case s.State == ImportedDiscarded:
		return nil, nil
	case s.State == ImportedAccepted:
		return nil, ErrImportedTransactionAlreadyAccepted
	}

	return []ImportedTransactionEvent{ImportedTransactionDiscarded{}}, nil
}

// keepImportedTransaction keeps a Suspected duplicate as a real
// transaction, so two genuinely identical payments are both booked (spec
// story 40c).
func keepImportedTransaction(s ImportedTransactionState) ([]ImportedTransactionEvent, error) {
	switch {
	case !s.Imported:
		return nil, ErrImportedTransactionNotImported
	case s.State != ImportedSuspectedDuplicate:
		return nil, fmt.Errorf("%w: it is %s", ErrImportedTransactionNotSuspectedDuplicate, s.State)
	}

	return []ImportedTransactionEvent{ImportedTransactionKept{}}, nil
}

func evolveImportedTransaction(s ImportedTransactionState, e decider.Envelope[ImportedTransactionEvent]) ImportedTransactionState {
	switch ev := e.Event.(type) {
	case ImportedTransactionAccepted:
		s.State, s.TransactionID = ImportedAccepted, ev.TransactionID
	case ImportedTransactionReturnedToPending:
		s.State, s.TransactionID = ImportedPending, sharedkernel.NilID
	case ImportedTransactionDiscarded:
		s.State = ImportedDiscarded
	case ImportedTransactionKept:
		s.State = ImportedPending
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
			Resembles:         ev.Resembles,
		}

		if ev.Resembles != sharedkernel.NilID {
			s.State = ImportedSuspectedDuplicate
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
