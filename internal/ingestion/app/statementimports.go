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

	ImportedTransactionStreamType = "ingestion.importedtransaction"
	TransactionImportedType       = "io.walletaccountant.ingestion.importedtransaction.imported.v1"
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
	// balance.
	ErrOpeningBalanceAlreadyRecorded = errors.New("opening balance already recorded")
	// ErrLaterImportsNotSupported is returned for an import that isn't the
	// Account's first.
	ErrLaterImportsNotSupported = domain.ErrLaterImportsNotSupported
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

	return StatementImportAggregate{
		StreamType:        StatementImportStreamType,
		Decider:           domain.StatementImport,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 1,
	}
}

// NewImportedTransactionAggregate wires the Imported transaction Decider to
// its fold store.
func NewImportedTransactionAggregate(folds eventstore.FoldStore[domain.ImportedTransactionState]) ImportedTransactionAggregate {
	codec := eventstore.NewCodec[domain.ImportedTransactionEvent](Source)
	eventstore.Register[domain.ImportedTransactionEvent, domain.TransactionImported](codec, TransactionImportedType)

	return ImportedTransactionAggregate{
		StreamType:        ImportedTransactionStreamType,
		Decider:           domain.ImportedTransaction,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 1,
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
// (consistency map: Upload Statement import).
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

		switch {
		case !r.account.AwaitingFirstStatementImport && up.PreviewID != "":
			return ErrOpeningBalanceAlreadyRecorded
		case !r.account.AwaitingFirstStatementImport:
			return ErrLaterImportsNotSupported
		case up.PreviewID == "":
			return ErrPreviewRequired
		case up.PreviewID != domain.PreviewID(r.connection.AccountID, connection, r.fileHash, up.ExportBalance, r.connection.MappingVersion):
			return ErrPreviewMismatch
		}

		res, err := s.imports.Execute(ctx, tx, importID, domain.RecordStatementImport{
			ConnectionID:   connection,
			AccountID:      r.connection.AccountID,
			FileHash:       r.fileHash,
			MappingVersion: r.connection.MappingVersion,
			Currency:       r.account.Currency.Code(),
			ExportBalance:  up.ExportBalance,
			RowsImported:   len(r.statement.Rows),
			RowsIgnored:    r.statement.Ignored,
			RowsTotal:      r.statement.Total,
			First:          true,
		})
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

		if err := s.ledger.RecordOpeningBalance(ctx, tx, r.connection.AccountID, openingID, *res.State.OpeningBalance); err != nil {
			return err //nolint:wrapcheck // the port's error
		}

		recorded = StatementImport{ID: importID, State: res.State}

		return nil
	})
	if err != nil {
		return StatementImport{}, fmt.Errorf("upload statement import: %w", err)
	}

	return recorded, nil
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
