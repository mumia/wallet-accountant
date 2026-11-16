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

	ImportedTransactionStreamType            = "ingestion.importedtransaction"
	TransactionImportedType                  = "io.walletaccountant.ingestion.importedtransaction.imported.v1"
	ImportedTransactionAcceptedType          = "io.walletaccountant.ingestion.importedtransaction.accepted.v1"
	ImportedTransactionReturnedToPendingType = "io.walletaccountant.ingestion.importedtransaction.returnedtopending.v1"
	ImportedTransactionDiscardedType         = "io.walletaccountant.ingestion.importedtransaction.discarded.v1"
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
)

// DuplicateImportError is returned for a file already imported through the
// Connection; Earlier is that import.
type DuplicateImportError struct {
	Earlier uuid.UUID
}

func (e *DuplicateImportError) Error() string {
	return fmt.Sprintf("statement file already imported as %s", e.Earlier)
}

// ImportAccount is an Account as a Statement import needs it.
type ImportAccount struct {
	Currency                     sharedkernel.Currency
	StartDate                    time.Time
	AwaitingFirstStatementImport bool
}

// Ledger is the Ledger port a Statement import calls in its own
// transaction (ADR-0013, consistency map: Upload Statement import).
type Ledger interface {
	// ImportAccount returns the Account, or ErrAccountNotFound.
	ImportAccount(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (ImportAccount, error)
	// RecordOpeningBalance records the Account's Opening balance
	// Transaction on its start date, or returns
	// ErrOpeningBalanceAlreadyRecorded.
	RecordOpeningBalance(ctx context.Context, tx *eventstore.Tx, account, transaction uuid.UUID, amount sharedkernel.Decimal) error
	// BookAccepted records the Ledger Transaction of an Accepted Imported
	// transaction and moves its Account's balance, or returns
	// ErrCategoryNotFound or ErrInternalCategory for a Category it can't be
	// booked to (consistency map: Accept imported transaction).
	BookAccepted(ctx context.Context, tx *eventstore.Tx, booking Booking) error
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

	return StatementImportAggregate{
		StreamType: StatementImportStreamType,
		Decider:    domain.StatementImport,
		Codec:      codec,
		Folds:      folds,
		// 2: when it was voided.
		FoldSchemaVersion: 2,
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

	return ImportedTransactionAggregate{
		StreamType: ImportedTransactionStreamType,
		Decider:    domain.ImportedTransaction,
		Codec:      codec,
		Folds:      folds,
		// 2: the Ledger Transaction that booked it.
		FoldSchemaVersion: 2,
	}
}

// StatementImport is a Statement import as the API sees it.
type StatementImport struct {
	ID    uuid.UUID
	State domain.StatementImportState
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
	// RowsOf returns every Imported transaction of the Statement import.
	RowsOf(ctx context.Context, tx *eventstore.Tx, statementImport uuid.UUID) ([]ImportRowState, error)
}

// ImportRowState is an Imported transaction of a Statement import and its
// state.
type ImportRowState struct {
	ID    uuid.UUID
	State domain.ImportedState
}

// ImportedSoFar is what an Account's Statement imports have brought in: the
// Opening balance of its first import, the total of its non-Discarded
// Imported transactions (pending, Accepted or kept), and the keys of those
// with a bank transaction ID, which tell certain repeats.
type ImportedSoFar struct {
	OpeningBalance sharedkernel.Decimal
	Total          sharedkernel.Decimal
	Keys           []domain.ImportedKey
}

// Upload is a statement file with its Export balance. Confirmation is the
// second entry of the Export balance (preview only); PreviewID ties a first
// import to its preview.
type Upload struct {
	File          []byte
	ExportBalance sharedkernel.Decimal
	Confirmation  sharedkernel.Decimal
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
		r, err := s.read(ctx, tx, connection, up)
		if err != nil {
			return err
		}

		if !r.account.AwaitingFirstStatementImport {
			return ErrOpeningBalanceAlreadyRecorded
		}

		if !up.ExportBalance.Equal(up.Confirmation) {
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
// preview: with the Account's imports locked, it skips certain repeats and
// is recorded only if its Balance check passes (ADR-0018).
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
		r, err := s.read(ctx, tx, connection, up)
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

		if !first {
			if err := s.reader.LockAccount(ctx, tx, r.connection.AccountID); err != nil {
				return err //nolint:wrapcheck // the adapter's error
			}

			soFar, err := s.reader.ImportedSoFar(ctx, tx, r.connection.AccountID)
			if err != nil {
				return err //nolint:wrapcheck // the adapter's error
			}

			r.statement = r.statement.WithoutCertainRepeats(soFar.Keys)
			cmd.OpeningBalance, cmd.EarlierTotal = soFar.OpeningBalance, soFar.Total
		}

		cmd.RowsImported, cmd.RowsSkipped = len(r.statement.Rows), r.statement.Skipped
		cmd.RowsIgnored, cmd.RowsTotal = r.statement.Ignored, r.statement.Total

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
			}); err != nil {
				return fmt.Errorf("import line %d: %w", row.Line, err)
			}
		}

		if first {
			if err := s.ledger.RecordOpeningBalance(ctx, tx, r.connection.AccountID, openingID, *res.State.OpeningBalance); err != nil {
				return err //nolint:wrapcheck // the port's error
			}
		}

		recorded = StatementImport{ID: importID, State: res.State}

		return nil
	})
	if err != nil {
		return StatementImport{}, fmt.Errorf("upload statement import: %w", err)
	}

	return recorded, nil
}

// Void withdraws the Account's latest Statement import, while none of its
// rows are Accepted, so a corrected file can be uploaded (ADR-0018): with
// the Account's imports locked, it records the void, Discards every row
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

		res, err := s.imports.Execute(ctx, tx, id, domain.VoidStatementImport{Latest: latest == id, AcceptedRows: accepted})
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

// List returns every Statement import of the Connection.
func (s *StatementImports) List(ctx context.Context, household, connection uuid.UUID) ([]StatementImport, error) {
	var list []StatementImport

	err := s.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		_, found, err := s.connections.Folds.Load(ctx, tx, connection)
		if err != nil {
			return err //nolint:wrapcheck // the adapter's error
		}

		if !found {
			return ErrConnectionNotFound
		}

		list, err = s.reader.StatementImports(ctx, tx, connection)

		return err //nolint:wrapcheck // the adapter's error
	})
	if err != nil {
		return nil, fmt.Errorf("list statement imports: %w", err)
	}

	return list, nil
}

// read reads the statement file through the Connection for its Account,
// refusing a file the Connection already imported.
func (s *StatementImports) read(ctx context.Context, tx *eventstore.Tx, connection uuid.UUID, up Upload) (read, error) {
	fold, found, err := s.connections.Folds.Load(ctx, tx, connection)
	if err != nil {
		return read{}, err //nolint:wrapcheck // the adapter's error
	}

	if !found {
		return read{}, ErrConnectionNotFound
	}

	if fold.State.CSVMapping == nil {
		return read{}, ErrNoCSVMapping
	}

	account, err := s.ledger.ImportAccount(ctx, tx, fold.State.AccountID)
	if err != nil {
		return read{}, err //nolint:wrapcheck // the port's error
	}

	hash := domain.FileHash(up.File)

	earlier, found, err := s.reader.ImportOfFile(ctx, tx, connection, hash)
	if err != nil {
		return read{}, err //nolint:wrapcheck // the adapter's error
	}

	if found {
		return read{}, &DuplicateImportError{Earlier: earlier}
	}

	if err := domain.CheckExportBalance(up.ExportBalance, account.Currency); err != nil {
		return read{}, err //nolint:wrapcheck // the domain's refusal
	}

	statement, err := domain.ReadStatement(*fold.State.CSVMapping, up.File, account.Currency, account.StartDate)
	if err != nil {
		return read{}, err //nolint:wrapcheck // the domain's refusal
	}

	return read{connection: fold.State, account: account, fileHash: hash, statement: statement}, nil
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
