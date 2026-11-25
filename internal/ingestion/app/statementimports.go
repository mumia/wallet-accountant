package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// Stream types and event types of the Statement import and Imported
// transaction aggregates (ADR-0015).
const (
	StatementImportStreamType   = "ingestion.statementimport"
	StatementImportRecordedType = "io.walletaccountant.ingestion.statementimport.recorded.v1"
	StatementImportVoidedType   = "io.walletaccountant.ingestion.statementimport.voided.v1"
	BalanceCheckPassedType      = "io.walletaccountant.ingestion.statementimport.balancecheckpassed.v1"
	BalanceCheckFailedType      = "io.walletaccountant.ingestion.statementimport.balancecheckfailed.v1"

	ImportedTransactionStreamType            = "ingestion.importedtransaction"
	TransactionImportedType                  = "io.walletaccountant.ingestion.importedtransaction.imported.v1"
	ImportedTransactionAcceptedType          = "io.walletaccountant.ingestion.importedtransaction.accepted.v1"
	ImportedTransactionReturnedToPendingType = "io.walletaccountant.ingestion.importedtransaction.returnedtopending.v1"
	ImportedTransactionDiscardedType         = "io.walletaccountant.ingestion.importedtransaction.discarded.v1"
	ImportedTransactionKeptType              = "io.walletaccountant.ingestion.importedtransaction.kept.v1"
)

var (
	// ErrNoCSVMapping is returned for importing through a Connection that
	// has no CSV mapping yet.
	ErrNoCSVMapping = errors.New("connection has no CSV mapping")
	// ErrExportBalanceMismatch is returned for a preview whose two Export
	// balance entries differ.
	ErrExportBalanceMismatch = errors.New("export balance entries differ")
	// ErrPreviewRequired is returned for an Account's first Statement import
	// uploaded without the preview_id of its preview.
	ErrPreviewRequired = errors.New("the first statement import needs its preview")
	// ErrPreviewMismatch is returned for a confirm whose file, Export
	// balance or mapping version differs from its preview.
	ErrPreviewMismatch = errors.New("upload doesn't match its preview")
	// ErrOpeningBalanceAlreadyRecorded is returned for previewing or
	// confirming a first import of an Account that already has its Opening
	// balance, and for a later import sent with a preview_id.
	ErrOpeningBalanceAlreadyRecorded = errors.New("opening balance already recorded")
	// ErrStatementImportNotFound is returned for a Statement import that is
	// not one of the Connection's in the Household.
	ErrStatementImportNotFound = errors.New("statement import not found")
	// ErrNotTheLatestStatementImport is returned for voiding an import
	// other than the Account's latest non-voided one.
	ErrNotTheLatestStatementImport = domain.ErrNotTheLatestStatementImport
	// ErrStatementImportHasAcceptedRows is returned for voiding an import
	// with Accepted rows.
	ErrStatementImportHasAcceptedRows = domain.ErrStatementImportHasAcceptedRows
	// ErrStatementImportAlreadyVoided is returned for voiding an import
	// twice.
	ErrStatementImportAlreadyVoided = domain.ErrStatementImportAlreadyVoided
	// ErrStatementImportTouchesClosedMonth is returned, as a
	// *domain.TouchesClosedMonthError, for voiding an import that touches a
	// closed month.
	ErrStatementImportTouchesClosedMonth = domain.ErrStatementImportTouchesClosedMonth
)

// DuplicateImportError is returned for a file already imported through the
// Connection; Earlier is that import.
type DuplicateImportError struct {
	Earlier uuid.UUID
}

func (e *DuplicateImportError) Error() string {
	return fmt.Sprintf("statement file already imported as %s", e.Earlier)
}

// ImportAccount is an Account as a Statement import needs it. OpenMonth
// is its Open month (YYYY-MM): every month before it is closed, every month
// after it not open yet (ADR-0024).
type ImportAccount struct {
	// Cash says it is a cash Account, kept entirely by hand, which takes no
	// Connection.
	Cash                         bool
	Currency                     sharedkernel.Currency
	StartDate                    time.Time
	AwaitingFirstStatementImport bool
	OpenMonth                    string
}

// AccountMonths is where an Account's months stand: every month from the
// start date's month up to OpenMonth (YYYY-MM) is closed, and every month
// after it is not open yet (ADR-0024).
type AccountMonths struct {
	StartDate time.Time
	OpenMonth string
}

// Ledger is the Ledger port a Statement import calls in its own
// transaction (ADR-0013, consistency map: Upload Statement import).
type Ledger interface {
	// ImportAccount returns the Account, or ErrAccountNotFound.
	ImportAccount(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (ImportAccount, error)
	// AccountMonthsForShare returns the Account's start date and Open
	// month, locking its fold FOR SHARE in the caller's transaction, so a
	// Close or Reopen of the Account waits until it ends; or
	// ErrAccountNotFound.
	AccountMonthsForShare(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (AccountMonths, error)
	// RecordOpeningBalance records the Account's Opening balance
	// Transaction on its start date, or returns
	// ErrOpeningBalanceAlreadyRecorded.
	RecordOpeningBalance(ctx context.Context, tx *eventstore.Tx, account, transaction uuid.UUID, amount sharedkernel.Decimal) error
	// BookAccepted records the Ledger Transaction of an Accepted Imported
	// transaction and moves its Account's balance, or returns
	// ErrCategoryNotFound or ErrInternalCategory for a Category it can't be
	// booked to (consistency map: Accept imported transaction).
	BookAccepted(ctx context.Context, tx *eventstore.Tx, booking Booking) error
	// BookTransfer records an Imported transaction Accepted as a Transfer:
	// the row's Transfer leg, the cash leg on the cash Account, and their
	// Match. It returns a *TransferRefusedError for a cash Account it can't
	// be a Transfer to, and a *MonthError naming the cash Account for a cash
	// leg in a closed month (consistency map: Accept as Transfer).
	BookTransfer(ctx context.Context, tx *eventstore.Tx, booking TransferBooking) error
	// TransferSuggestions returns the cash Account's In transit legs that
	// could be the other side of the row, closest date first, or a
	// *TransferRefusedError for a cash Account it can't be a Transfer with.
	TransferSuggestions(ctx context.Context, tx *eventstore.Tx, row domain.ImportedTransactionState, cash uuid.UUID,
	) ([]SuggestedLeg, error)
	// ReverseOpeningBalance reverses the Account's Opening balance
	// Transaction by the Transaction reversal, so the Account awaits its
	// first Statement import again (consistency map: Void Statement
	// import).
	ReverseOpeningBalance(ctx context.Context, tx *eventstore.Tx, account, reversal uuid.UUID) error
}

// Booking is an Accepted Imported transaction as the Ledger books it:
// Transaction is the new Ledger Transaction's ID and Category the other
// side, nil for Uncategorised.
type Booking struct {
	Transaction         uuid.UUID
	ImportedTransaction uuid.UUID
	Category            *uuid.UUID
	Row                 domain.ImportedTransactionState
}

// TransferBooking is an Imported transaction Accepted as a Transfer to or
// from CashAccount: Booking's Category is unused, and CashLeg is the ID of
// the cash leg to record, or MatchLeg the cash Account's leg to Match
// instead.
type TransferBooking struct {
	Booking

	CashAccount uuid.UUID
	CashLeg     uuid.UUID
	MatchLeg    *uuid.UUID
}

// StatementImportAggregate is the Statement import Decider wired to the
// runtime.
type StatementImportAggregate = eventstore.Aggregate[domain.StatementImportState, domain.StatementImportCommand, domain.StatementImportEvent]

// ImportedTransactionAggregate is the Imported transaction Decider wired to
// the runtime.
type ImportedTransactionAggregate = eventstore.Aggregate[domain.ImportedTransactionState, domain.ImportedTransactionCommand, domain.ImportedTransactionEvent]

// NewStatementImportAggregate wires the Statement import Decider to its
// fold store.
func NewStatementImportAggregate(folds eventstore.FoldStore[domain.StatementImportState]) StatementImportAggregate {
	codec := eventstore.NewCodec[domain.StatementImportEvent](Source)
	eventstore.Register[domain.StatementImportEvent, domain.StatementImportRecorded](codec, StatementImportRecordedType)
	eventstore.Register[domain.StatementImportEvent, domain.StatementImportVoided](codec, StatementImportVoidedType)
	eventstore.Register[domain.StatementImportEvent, domain.WaitingCheckPassed](codec, BalanceCheckPassedType)
	eventstore.Register[domain.StatementImportEvent, domain.WaitingCheckFailed](codec, BalanceCheckFailedType)

	return StatementImportAggregate{
		StreamType: StatementImportStreamType,
		Decider:    domain.StatementImport,
		Codec:      codec,
		Folds:      folds,
		// 2: when it was voided.
		// 3: its Suspected duplicates, and a waiting or failed check.
		FoldSchemaVersion: 3,
	}
}

// NewImportedTransactionAggregate wires the Imported transaction Decider to
// its fold store.
func NewImportedTransactionAggregate(folds eventstore.FoldStore[domain.ImportedTransactionState]) ImportedTransactionAggregate {
	codec := eventstore.NewCodec[domain.ImportedTransactionEvent](Source)
	eventstore.Register[domain.ImportedTransactionEvent, domain.TransactionImported](codec, TransactionImportedType)
	eventstore.Register[domain.ImportedTransactionEvent, domain.ImportedTransactionAccepted](codec,
		ImportedTransactionAcceptedType)
	eventstore.Register[domain.ImportedTransactionEvent, domain.ImportedTransactionReturnedToPending](codec,
		ImportedTransactionReturnedToPendingType)
	eventstore.Register[domain.ImportedTransactionEvent, domain.ImportedTransactionDiscarded](codec,
		ImportedTransactionDiscardedType)
	eventstore.Register[domain.ImportedTransactionEvent, domain.ImportedTransactionKept](codec,
		ImportedTransactionKeptType)

	return ImportedTransactionAggregate{
		StreamType: ImportedTransactionStreamType,
		Decider:    domain.ImportedTransaction,
		Codec:      codec,
		Folds:      folds,
		// 2: the Ledger Transaction that booked it; 3: the earlier Imported
		// transaction a Suspected duplicate resembles.
		FoldSchemaVersion: 3,
	}
}

// StatementImport is a Statement import as the API sees it.
// VoidBlockedBy is every reason a Void of it would be refused now.
type StatementImport struct {
	ID            uuid.UUID
	State         domain.StatementImportState
	VoidBlockedBy []domain.VoidBlock
}

// StatementImportReader reads the Household's Statement import folds.
type StatementImportReader interface {
	// StatementImports returns every Statement import of the Connection, in
	// the order they were recorded.
	StatementImports(ctx context.Context, tx *eventstore.Tx, connection uuid.UUID) ([]StatementImport, error)
	// ImportOfFile returns the Connection's import of the file with the
	// hash, if there is one.
	ImportOfFile(ctx context.Context, tx *eventstore.Tx, connection uuid.UUID, fileHash string) (uuid.UUID, bool, error)
	// LockAccount holds the Account's Statement imports until the
	// transaction ends, so imports of one Account, through any of its
	// Connections, run their Balance checks one after the other.
	LockAccount(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) error
	// ImportedSoFar returns what a later import of the Account is checked
	// against.
	ImportedSoFar(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (ImportedSoFar, error)
	// LatestImport returns the Account's latest non-voided Statement
	// import, if it has one.
	LatestImport(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (uuid.UUID, bool, error)
	// AcceptedRows counts, per Statement import of the Connection, its
	// Accepted Imported transactions; an import with none is absent.
	AcceptedRows(ctx context.Context, tx *eventstore.Tx, connection uuid.UUID) (map[uuid.UUID]int, error)
	// RowsOf returns every Imported transaction of the Statement import.
	RowsOf(ctx context.Context, tx *eventstore.Tx, statementImport uuid.UUID) ([]ImportRowState, error)
	// ImportedUpTo returns the Account's Opening balance and the total of
	// its non-Discarded Imported transactions from the Statement import and
	// those recorded before it, which its waiting Balance check runs on.
	// Keys is left empty.
	ImportedUpTo(ctx context.Context, tx *eventstore.Tx, account, statementImport uuid.UUID) (ImportedSoFar, error)
	// UndecidedSuspectedDuplicates counts the Statement import's Imported
	// transactions that are still Suspected duplicates.
	UndecidedSuspectedDuplicates(ctx context.Context, tx *eventstore.Tx, statementImport uuid.UUID) (int, error)
	// AccountImports reports every Booking month of the Account at once, in
	// one grouped query for the counts and one for the unchecked imports.
	AccountImports(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (AccountImports, error)
	// UnresolvedRows counts, per Account, the pending and Suspected
	// duplicate Imported transactions in every Booking month, in one
	// grouped query: of the one Account, or of every Account of the
	// Household when account is nil. An Account with none is absent.
	UnresolvedRows(ctx context.Context, tx *eventstore.Tx, account *uuid.UUID) (map[uuid.UUID]RowCounts, error)
	// MonthImports counts the Account's pending and Suspected duplicate
	// Imported transactions booked in the month, and lists its non-voided
	// Statement imports with rows booked in or before the month whose
	// Balance check is waiting or failed, in the order they were recorded.
	MonthImports(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, month time.Time) (MonthImports, error)
}

// AccountImports is what an Account's Statement imports leave unresolved
// in every Booking month at once: per month (YYYY-MM) holding a
// non-Discarded Imported transaction its pending and Suspected duplicate
// counts, and the non-voided imports whose Balance check is waiting or
// failed, each with the earliest Booking month it has rows in. The
// Ledger's months list reads it.
type AccountImports struct {
	Months    map[string]RowCounts
	Unchecked []UncheckedImportFrom
}

// RowCounts counts pending and Suspected duplicate Imported transactions,
// of a Booking month or of a whole Account.
type RowCounts struct {
	Pending             int
	SuspectedDuplicates int
}

// UncheckedImportFrom is an UncheckedImport with the earliest Booking month
// (YYYY-MM) it has rows in.
type UncheckedImportFrom struct {
	UncheckedImport

	From string
}

// MonthImports is what an Account's Statement imports leave unresolved in
// a Booking month: the Ledger's Month close reads it.
type MonthImports struct {
	Pending             int
	SuspectedDuplicates int
	UncheckedImports    []UncheckedImport
}

// UncheckedImport is a Statement import whose Balance check is waiting or
// failed.
type UncheckedImport struct {
	ID           uuid.UUID
	BalanceCheck domain.BalanceCheck
}

// ImportRowState is an Imported transaction of a Statement import and its
// state.
type ImportRowState struct {
	ID    uuid.UUID
	State domain.ImportedState
}

// ImportedSoFar is what an Account's Statement imports have brought in: the
// Opening balance of its first import, the total of its pending, Accepted
// and kept Imported transactions (an undecided Suspected duplicate counts
// only once kept), and the keys of every non-Discarded one, which tell
// certain repeats and Suspected duplicates.
type ImportedSoFar struct {
	OpeningBalance sharedkernel.Decimal
	Total          sharedkernel.Decimal
	Keys           []domain.ImportedKey
}

// Upload is a statement file with its Export balance. Confirmation is the
// second entry of the Export balance (preview only), nil when it wasn't
// given: a mapping with a balance column ignores it, since the file's
// balance tests the Export balance instead. PreviewID ties a first import
// to its preview.
type Upload struct {
	File          []byte
	ExportBalance sharedkernel.Decimal
	Confirmation  *sharedkernel.Decimal
	PreviewID     string
}

// Preview is what an Account's first Statement import would record.
type Preview struct {
	ID             string
	Currency       string
	StartDate      time.Time
	ExportBalance  sharedkernel.Decimal
	OpeningBalance sharedkernel.Decimal
	Rows           int
	Ignored        int
	Total          sharedkernel.Decimal
}

// StatementImports imports statement files through Connections.
type StatementImports struct {
	scope       HouseholdScope
	ledger      Ledger
	connections ConnectionAggregate
	imports     StatementImportAggregate
	rows        ImportedTransactionAggregate
	reader      StatementImportReader
}

// NewStatementImports returns the StatementImports service.
func NewStatementImports(scope HouseholdScope, ledger Ledger, connections ConnectionAggregate,
	imports StatementImportAggregate, rows ImportedTransactionAggregate, reader StatementImportReader,
) *StatementImports {
	return &StatementImports{
		scope: scope, ledger: ledger, connections: connections, imports: imports, rows: rows, reader: reader,
	}
}

// read is a statement file read through its Connection for its Account.
type read struct {
	connection domain.ConnectionState
	account    ImportAccount
	fileHash   string
	statement  domain.Statement
}

// Preview reads the Account's first statement file and works out the
// Opening balance it would record, recording nothing (consistency map:
// Preview first Statement import).
func (s *StatementImports) Preview(ctx context.Context, household, connection uuid.UUID, up Upload) (Preview, error) {
	var p Preview

	err := s.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		r, err := s.read(ctx, tx, connection, up, false)
		if err != nil {
			return err
		}

		if !r.account.AwaitingFirstStatementImport {
			return ErrOpeningBalanceAlreadyRecorded
		}

		// The file's balance, when the mapping has a balance column, has
		// already tested the Export balance, so it is asked only once
		// (ADR-0018).
		if !r.statement.HasFileBalance() && (up.Confirmation == nil || !up.ExportBalance.Equal(*up.Confirmation)) {
			return ErrExportBalanceMismatch
		}

		p = Preview{
			ID:             domain.PreviewID(r.connection.AccountID, connection, r.fileHash, up.ExportBalance, r.connection.MappingVersion),
			Currency:       r.account.Currency.Code(),
			StartDate:      r.account.StartDate,
			ExportBalance:  up.ExportBalance,
			OpeningBalance: r.statement.OpeningBalance(up.ExportBalance),
			Rows:           len(r.statement.Rows),
			Ignored:        r.statement.Ignored,
			Total:          r.statement.Total,
		}

		return nil
	})
	if err != nil {
		return Preview{}, fmt.Errorf("preview statement import: %w", err)
	}

	return p, nil
}

// Upload imports a statement file through the Connection. The Account's
// first import must carry the ID of its preview, recomputed here from the
// file, the Export balance and the current mapping version. It records the
// Statement import, a pending Imported transaction per row and, through the
// Ledger, the Opening balance Transaction, all in one database transaction
// (consistency map: Upload Statement import). A later import takes no
// preview: with the Account's imports locked, it skips certain repeats,
// imports rows that only look like earlier ones as Suspected duplicates,
// and is recorded only if its Balance check passes, or with the check
// waiting when it has Suspected duplicates (ADR-0018).
func (s *StatementImports) Upload(ctx context.Context, household, connection uuid.UUID, up Upload) (StatementImport, error) {
	ids := newIDs()

	importID, err := ids.next(0)
	if err != nil {
		return StatementImport{}, err
	}

	openingID, err := ids.next(1)
	if err != nil {
		return StatementImport{}, err
	}

	var recorded StatementImport

	err = s.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		// The Account is read with its imports locked, so whether this is
		// its first import can't change before it is recorded: a void of
		// the first import running at the same time either sees this import
		// recorded or reopens the Account before it is read.
		r, err := s.read(ctx, tx, connection, up, true)
		if err != nil {
			return err
		}

		first := r.account.AwaitingFirstStatementImport

		switch {
		case !first && up.PreviewID != "":
			return ErrOpeningBalanceAlreadyRecorded
		case first && up.PreviewID == "":
			return ErrPreviewRequired
		case first && up.PreviewID != domain.PreviewID(r.connection.AccountID, connection, r.fileHash, up.ExportBalance, r.connection.MappingVersion):
			return ErrPreviewMismatch
		}

		cmd := domain.RecordStatementImport{
			ConnectionID:   connection,
			AccountID:      r.connection.AccountID,
			FileHash:       r.fileHash,
			MappingVersion: r.connection.MappingVersion,
			Currency:       r.account.Currency.Code(),
			ExportBalance:  up.ExportBalance,
			First:          first,
		}

		// A first import never meets a closed month: its start month is
		// the Open month until the first import is recorded (ADR-0024).
		if !first {
			// Read with the Account's fold locked FOR SHARE, so a Close or
			// Reopen waits until this import is recorded.
			months, err := s.ledger.AccountMonthsForShare(ctx, tx, r.connection.AccountID)
			if err != nil {
				return err //nolint:wrapcheck // the port's error
			}

			soFar, err := s.reader.ImportedSoFar(ctx, tx, r.connection.AccountID)
			if err != nil {
				return err //nolint:wrapcheck // the adapter's error
			}

			// Rows in a closed month are split off first, so they are
			// never certain repeats or Suspected duplicates.
			r.statement = r.statement.WithoutClosedMonths(months.OpenMonth).
				WithoutCertainRepeats(soFar.Keys).WithSuspectedDuplicates(soFar.Keys)
			cmd.OpeningBalance, cmd.EarlierTotal = soFar.OpeningBalance, soFar.Total
			cmd.ClosedMonthsNotInBooks = r.statement.ClosedMonthsNotInBooks(soFar.Keys)
		}

		cmd.RowsImported, cmd.RowsSkipped = len(r.statement.Rows), r.statement.Skipped
		cmd.SuspectedDuplicates = r.statement.Suspected
		cmd.RowsIgnored, cmd.RowsTotal = r.statement.Ignored, r.statement.Total
		cmd.RowsIgnoredInClosedMonths = len(r.statement.InClosedMonths)
		cmd.EarliestBookingMonth = r.statement.EarliestBookingMonth()

		res, err := s.imports.Execute(ctx, tx, importID, cmd)
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		for i, row := range r.statement.Rows {
			id, err := ids.next(2 + i) //nolint:mnd // after the import's and the Opening balance's IDs
			if err != nil {
				return err
			}

			if _, err := s.rows.Execute(ctx, tx, id, domain.ImportRow{
				StatementImportID: importID,
				AccountID:         r.connection.AccountID,
				Line:              row.Line,
				BookingDate:       row.BookingDate,
				TransactionDate:   row.TransactionDate,
				ValueDate:         row.ValueDate,
				Amount:            row.Amount,
				Currency:          r.account.Currency.Code(),
				Description:       row.Description,
				Payee:             row.Payee,
				BankTransactionID: row.BankTransactionID,
				Resembles:         row.Resembles,
			}); err != nil {
				return fmt.Errorf("import line %d: %w", row.Line, err)
			}
		}

		if first {
			if err := s.ledger.RecordOpeningBalance(ctx, tx, r.connection.AccountID, openingID, *res.State.OpeningBalance); err != nil {
				return err //nolint:wrapcheck // the port's error
			}
		}

		// Just recorded, it is the latest, with nothing Accepted, and its
		// rows are all in or after the Open month.
		recorded = StatementImport{ID: importID, State: res.State, VoidBlockedBy: res.State.VoidBlockedBy(
			domain.VoidStatementImport{Latest: true, OpenMonth: r.account.OpenMonth, StartMonth: monthOfDate(r.account.StartDate)},
		)}

		return nil
	})
	if err != nil {
		return StatementImport{}, fmt.Errorf("upload statement import: %w", err)
	}

	return recorded, nil
}

// Void withdraws the Account's latest Statement import, while none of its
// rows are Accepted and it touches no closed month, so a corrected file can
// be uploaded (ADR-0018, ADR-0024): with the Account's imports locked, and
// its Open month read with its fold locked FOR SHARE so a Close or Reopen
// waits, it records the void, Discards every row
// and, for the Account's first import, reverses the Opening balance
// Transaction through the Ledger, all in one database transaction. The
// voided import releases its file (consistency map: Void Statement import).
func (s *StatementImports) Void(ctx context.Context, household, connection, id uuid.UUID) (StatementImport, error) {
	reversal, err := sharedkernel.NewID()
	if err != nil {
		return StatementImport{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	var voided StatementImport

	err = s.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := s.imports.Folds.Load(ctx, tx, id)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !found || fold.State.ConnectionID != connection {
			return ErrStatementImportNotFound
		}

		account := fold.State.AccountID

		if err := s.reader.LockAccount(ctx, tx, account); err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		latest, _, err := s.reader.LatestImport(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		rows, err := s.reader.RowsOf(ctx, tx, id)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		accepted := 0

		for _, r := range rows {
			if r.State == domain.ImportedAccepted {
				accepted++
			}
		}

		months, err := s.ledger.AccountMonthsForShare(ctx, tx, account)
		if err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		res, err := s.imports.Execute(ctx, tx, id, domain.VoidStatementImport{
			Latest: latest == id, AcceptedRows: accepted,
			OpenMonth: months.OpenMonth, StartMonth: monthOfDate(months.StartDate),
		})
		if err != nil {
			return err //nolint:wrapcheck // the Decider's refusal or the runtime's error
		}

		// Each row checks its own state again, so an Accept that raced the
		// void refuses one or the other.
		for _, r := range rows {
			if _, err := s.rows.Execute(ctx, tx, r.ID, domain.DiscardForVoid{}); err != nil {
				return fmt.Errorf("discard imported transaction %s: %w", r.ID, err)
			}
		}

		if res.State.OpeningBalance != nil {
			if err := s.ledger.ReverseOpeningBalance(ctx, tx, account, reversal); err != nil {
				return err //nolint:wrapcheck // the port's error
			}
		}

		voided = StatementImport{ID: id, State: res.State}

		return nil
	})
	if err != nil {
		return StatementImport{}, fmt.Errorf("void statement import %s: %w", id, err)
	}

	return voided, nil
}

// List returns every Statement import of the Connection, each with every
// reason a Void of it would be refused now.
func (s *StatementImports) List(ctx context.Context, household, connection uuid.UUID) ([]StatementImport, error) {
	var list []StatementImport

	err := s.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := s.connections.Folds.Load(ctx, tx, connection)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !found {
			return ErrConnectionNotFound
		}

		if list, err = s.reader.StatementImports(ctx, tx, connection); err != nil || len(list) == 0 {
			return err //nolint:wrapcheck // the adapter's error
		}

		account, err := s.ledger.ImportAccount(ctx, tx, fold.State.AccountID)
		if err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		latest, _, err := s.reader.LatestImport(ctx, tx, fold.State.AccountID)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		accepted, err := s.reader.AcceptedRows(ctx, tx, connection)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		for i, imp := range list {
			list[i].VoidBlockedBy = imp.State.VoidBlockedBy(domain.VoidStatementImport{
				Latest: imp.ID == latest, AcceptedRows: accepted[imp.ID],
				OpenMonth: account.OpenMonth, StartMonth: monthOfDate(account.StartDate),
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list statement imports: %w", err)
	}

	return list, nil
}

// monthOfDate is the month (YYYY-MM) of a date.
func monthOfDate(d time.Time) string { return d.Format(domain.MonthLayout) }

// read reads the statement file through the Connection for its Account,
// refusing a file the Connection already imported. Through a mapping with
// a balance column, it first refuses an unreadable file and an Export
// balance that isn't the file's balance. With lock, the Account's
// Statement imports are locked before the Account is read.
func (s *StatementImports) read(ctx context.Context, tx *eventstore.Tx, connection uuid.UUID, up Upload, lock bool,
) (read, error) {
	fold, found, err := s.connections.Folds.Load(ctx, tx, connection)
	if err != nil {
		return read{}, err //nolint:wrapcheck // the adapter's error
	}

	if !found {
		return read{}, ErrConnectionNotFound
	}

	if lock {
		if err := s.reader.LockAccount(ctx, tx, fold.State.AccountID); err != nil {
			return read{}, err //nolint:wrapcheck // the adapter's error
		}
	}

	if fold.State.CSVMapping == nil {
		return read{}, ErrNoCSVMapping
	}

	account, err := s.ledger.ImportAccount(ctx, tx, fold.State.AccountID)
	if err != nil {
		return read{}, err //nolint:wrapcheck // the port's error
	}

	hash := domain.FileHash(up.File)
	mapping := *fold.State.CSVMapping

	// Through a mapping with a balance column, the file is read and its
	// balance checked before the duplicate-file check; without one, the
	// duplicate-file check comes first, as it always has.
	if !mapping.HasBalance() {
		if err := s.refuseImportedFile(ctx, tx, connection, hash); err != nil {
			return read{}, err
		}
	}

	if err := domain.CheckExportBalance(up.ExportBalance, account.Currency); err != nil {
		return read{}, err //nolint:wrapcheck // the domain's refusal
	}

	statement, err := domain.ReadStatement(mapping, up.File, account.Currency, account.StartDate)
	if err != nil {
		return read{}, err //nolint:wrapcheck // the domain's refusal
	}

	if mapping.HasBalance() {
		if err := statement.CheckExportBalanceAgainstFile(up.ExportBalance); err != nil {
			return read{}, err //nolint:wrapcheck // the domain's refusal
		}

		if err := s.refuseImportedFile(ctx, tx, connection, hash); err != nil {
			return read{}, err
		}
	}

	return read{connection: fold.State, account: account, fileHash: hash, statement: statement}, nil
}

// refuseImportedFile refuses a file, by its hash, that the Connection
// already imported.
func (s *StatementImports) refuseImportedFile(ctx context.Context, tx *eventstore.Tx, connection uuid.UUID, hash string) error {
	earlier, found, err := s.reader.ImportOfFile(ctx, tx, connection, hash)
	if err != nil {
		return err //nolint:wrapcheck // the adapter's error
	}

	if found {
		return &DuplicateImportError{Earlier: earlier}
	}

	return nil
}

// ids hands out UUIDv7s by position, the same ones on every attempt of a
// retried transaction, so a retried command produces the same IDs.
type ids struct{ list []uuid.UUID }

func newIDs() *ids { return &ids{} }

func (i *ids) next(n int) (uuid.UUID, error) {
	for len(i.list) <= n {
		id, err := sharedkernel.NewID()
		if err != nil {
			return uuid.Nil, err //nolint:wrapcheck // already wrapped by the shared kernel
		}

		i.list = append(i.list, id)
	}

	return i.list[n], nil
}
