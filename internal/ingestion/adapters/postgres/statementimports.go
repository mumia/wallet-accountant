package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mumia/wallet-accountant/internal/ingestion/app"
	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// StatementImportFolds stores Statement import folds. A file is imported
// once per Connection: its unique index turns a concurrent import of the
// same file into a version conflict, so the retry finds the earlier import.
type StatementImportFolds struct{}

var _ eventstore.FoldStore[domain.StatementImportState] = StatementImportFolds{}

const statementImportColumns = `connection_id, account_id, file_hash, mapping_version, currency, export_balance::text,
	rows_imported, rows_skipped, rows_ignored, rows_total::text, opening_balance::text, balance_check, recorded_at,
	voided_at`

func scanStatementImport(row pgx.Row, extra ...any) (domain.StatementImportState, error) {
	var (
		s             domain.StatementImportState
		export, total string
		opening       *string
		balanceCheck  string
		voidedAt      *time.Time
	)

	dest := append([]any{
		&s.ConnectionID, &s.AccountID, &s.FileHash, &s.MappingVersion, &s.Currency, &export,
		&s.RowsImported, &s.RowsSkipped, &s.RowsIgnored, &total, &opening, &balanceCheck, &s.RecordedAt, &voidedAt,
	}, extra...)

	if err := row.Scan(dest...); err != nil {
		return s, err //nolint:wrapcheck // the callers wrap it
	}

	s.Recorded, s.BalanceCheck = true, domain.BalanceCheck(balanceCheck)

	if voidedAt != nil {
		s.VoidedAt = *voidedAt
	}

	var err error
	if s.ExportBalance, err = sharedkernel.ParseDecimal(export); err != nil {
		return s, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	if s.RowsTotal, err = sharedkernel.ParseDecimal(total); err != nil {
		return s, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	if opening != nil {
		d, err := sharedkernel.ParseDecimal(*opening)
		if err != nil {
			return s, err //nolint:wrapcheck // already wrapped by the shared kernel
		}

		s.OpeningBalance = &d
	}

	return s, nil
}

// Load implements eventstore.FoldStore.
func (StatementImportFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.StatementImportState], bool, error) {
	var f eventstore.Fold[domain.StatementImportState]

	s, err := scanStatementImport(tx.QueryRow(ctx, `SELECT `+statementImportColumns+`, version, schema_version
		FROM statement_imports WHERE household_id = $1 AND id = $2`, tx.HouseholdID(), id), &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.StatementImportState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.StatementImportState]{}, false, fmt.Errorf("load statement import: %w", err)
	}

	f.State = s

	return f, true, nil
}

// Save implements eventstore.FoldStore.
func (StatementImportFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.StatementImportState], expected int64) error {
	s := f.State

	var opening *string
	if s.OpeningBalance != nil {
		opening = new(s.OpeningBalance.String())
	}

	var voidedAt *time.Time
	if s.Voided() {
		voidedAt = &s.VoidedAt
	}

	args := []any{
		tx.HouseholdID(), id, s.ConnectionID, s.AccountID, s.FileHash, s.MappingVersion, s.Currency,
		s.ExportBalance.String(), s.RowsImported, s.RowsIgnored, s.RowsTotal.String(), opening, string(s.BalanceCheck),
		s.RecordedAt, f.Version, f.SchemaVersion, s.RowsSkipped, voidedAt,
	}
	q := `INSERT INTO statement_imports (household_id, id, connection_id, account_id, file_hash, mapping_version, currency,
	          export_balance, rows_imported, rows_ignored, rows_total, opening_balance, balance_check, recorded_at,
	          version, schema_version, rows_skipped, voided_at)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8::numeric, $9, $10, $11::numeric, $12::numeric, $13, $14, $15, $16, $17,
	              $18)
	      ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE statement_imports SET connection_id = $3, account_id = $4, file_hash = $5, mapping_version = $6,
		         currency = $7, export_balance = $8::numeric, rows_imported = $9, rows_ignored = $10,
		         rows_total = $11::numeric, opening_balance = $12::numeric, balance_check = $13, recorded_at = $14,
		         version = $15, schema_version = $16, rows_skipped = $17, voided_at = $18
		     WHERE household_id = $1 AND id = $2 AND version = $19`
	}

	tag, err := tx.Exec(ctx, q, args...)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "statement_imports_file_idx" {
		return fmt.Errorf("%w: the file was imported concurrently", eventstore.ErrVersionConflict)
	}

	if err != nil {
		return fmt.Errorf("save statement import: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	return nil
}

// StatementImportReader implements app.StatementImportReader.
type StatementImportReader struct{}

var _ app.StatementImportReader = StatementImportReader{}

// StatementImports implements app.StatementImportReader. Statement import
// IDs are UUIDv7, so they come in the order they were recorded.
func (StatementImportReader) StatementImports(ctx context.Context, tx *eventstore.Tx, connection uuid.UUID) ([]app.StatementImport, error) {
	rows, err := tx.Query(ctx, `SELECT `+statementImportColumns+`, id
		FROM statement_imports WHERE household_id = $1 AND connection_id = $2 ORDER BY id`, tx.HouseholdID(), connection)
	if err != nil {
		return nil, fmt.Errorf("list statement imports: %w", err)
	}

	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.StatementImport, error) {
		var i app.StatementImport

		s, err := scanStatementImport(r, &i.ID)
		i.State = s

		return i, err
	})
	if err != nil {
		return nil, fmt.Errorf("list statement imports: %w", err)
	}

	return list, nil
}

// ImportOfFile implements app.StatementImportReader. A voided import has
// released its file.
func (StatementImportReader) ImportOfFile(ctx context.Context, tx *eventstore.Tx, connection uuid.UUID, fileHash string) (uuid.UUID, bool, error) {
	var id uuid.UUID

	err := tx.QueryRow(ctx, `SELECT id FROM statement_imports
		WHERE household_id = $1 AND connection_id = $2 AND file_hash = $3 AND voided_at IS NULL`,
		tx.HouseholdID(), connection, fileHash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}

	if err != nil {
		return uuid.Nil, false, fmt.Errorf("find statement import of file: %w", err)
	}

	return id, true, nil
}

// LockAccount implements app.StatementImportReader with a
// transaction-level advisory lock on the Account's imports: the Balance
// check reads every Imported transaction of the Account, which no unique
// index or fold version can guard.
func (StatementImportReader) LockAccount(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('ingestion.account-imports:' || $1::text, 0))`,
		account); err != nil {
		return fmt.Errorf("lock account imports: %w", err)
	}

	return nil
}

// LatestImport implements app.StatementImportReader. Statement import IDs
// are UUIDv7, so the greatest is the latest.
func (StatementImportReader) LatestImport(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (uuid.UUID, bool, error) {
	var id uuid.UUID

	err := tx.QueryRow(ctx, `SELECT id FROM statement_imports
		WHERE household_id = $1 AND account_id = $2 AND voided_at IS NULL ORDER BY id DESC LIMIT 1`,
		tx.HouseholdID(), account).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}

	if err != nil {
		return uuid.Nil, false, fmt.Errorf("find the latest statement import: %w", err)
	}

	return id, true, nil
}

// RowsOf implements app.StatementImportReader.
func (StatementImportReader) RowsOf(ctx context.Context, tx *eventstore.Tx, statementImport uuid.UUID) ([]app.ImportRowState, error) {
	rows, err := tx.Query(ctx, `SELECT id, state FROM imported_transactions
		WHERE household_id = $1 AND statement_import_id = $2 ORDER BY id`, tx.HouseholdID(), statementImport)
	if err != nil {
		return nil, fmt.Errorf("list rows of statement import: %w", err)
	}

	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.ImportRowState, error) {
		var (
			row   app.ImportRowState
			state string
		)

		err := r.Scan(&row.ID, &state)
		row.State = domain.ImportedState(state)

		return row, err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return nil, fmt.Errorf("list rows of statement import: %w", err)
	}

	return list, nil
}

// ImportedSoFar implements app.StatementImportReader. Every state but
// Discarded counts, so pending, Accepted and kept rows all do. A voided
// first import's Opening balance doesn't count: it was reversed.
func (StatementImportReader) ImportedSoFar(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (app.ImportedSoFar, error) {
	var (
		soFar          app.ImportedSoFar
		opening, total string
	)

	err := tx.QueryRow(ctx, `SELECT
		    (SELECT COALESCE(SUM(opening_balance), 0)::text FROM statement_imports
		      WHERE household_id = $1 AND account_id = $2 AND voided_at IS NULL),
		    (SELECT COALESCE(SUM(amount), 0)::text FROM imported_transactions
		      WHERE household_id = $1 AND account_id = $2 AND state <> 'discarded')`,
		tx.HouseholdID(), account).Scan(&opening, &total)
	if err != nil {
		return app.ImportedSoFar{}, fmt.Errorf("read account imports: %w", err)
	}

	if soFar.OpeningBalance, err = sharedkernel.ParseDecimal(opening); err != nil {
		return app.ImportedSoFar{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	if soFar.Total, err = sharedkernel.ParseDecimal(total); err != nil {
		return app.ImportedSoFar{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	rows, err := tx.Query(ctx, `SELECT bank_transaction_id, booking_date, amount::text FROM imported_transactions
		WHERE household_id = $1 AND account_id = $2 AND state <> 'discarded' AND bank_transaction_id <> ''`,
		tx.HouseholdID(), account)
	if err != nil {
		return app.ImportedSoFar{}, fmt.Errorf("read account imports: %w", err)
	}

	soFar.Keys, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.ImportedKey, error) {
		var (
			k      domain.ImportedKey
			amount string
		)

		if err := r.Scan(&k.BankTransactionID, &k.BookingDate, &amount); err != nil {
			return k, err //nolint:wrapcheck // wrapped below
		}

		d, err := sharedkernel.ParseDecimal(amount)
		k.Amount = d

		return k, err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return app.ImportedSoFar{}, fmt.Errorf("read account imports: %w", err)
	}

	return soFar, nil
}

// ImportedTransactionFolds stores Imported transaction folds.
type ImportedTransactionFolds struct{}

var _ eventstore.FoldStore[domain.ImportedTransactionState] = ImportedTransactionFolds{}

const importedColumns = `statement_import_id, account_id, state, line, booking_date, transaction_date, value_date,
	amount::text, currency, description, payee, bank_transaction_id, transaction_id`

func scanImported(row pgx.Row, extra ...any) (domain.ImportedTransactionState, error) {
	var (
		s         domain.ImportedTransactionState
		state     string
		valueDate *time.Time
		amount    string
		booked    *uuid.UUID
	)

	dest := append([]any{
		&s.StatementImportID, &s.AccountID, &state, &s.Line, &s.BookingDate, &s.TransactionDate, &valueDate,
		&amount, &s.Currency, &s.Description, &s.Payee, &s.BankTransactionID, &booked,
	}, extra...)

	if err := row.Scan(dest...); err != nil {
		return s, err //nolint:wrapcheck // the callers wrap it
	}

	s.Imported, s.State = true, domain.ImportedState(state)

	if booked != nil {
		s.TransactionID = *booked
	}

	if valueDate != nil {
		s.ValueDate = *valueDate
	}

	d, err := sharedkernel.ParseDecimal(amount)
	s.Amount = d

	return s, err //nolint:wrapcheck // already wrapped by the shared kernel
}

// Load implements eventstore.FoldStore.
func (ImportedTransactionFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.ImportedTransactionState], bool, error) {
	var f eventstore.Fold[domain.ImportedTransactionState]

	s, err := scanImported(tx.QueryRow(ctx, `SELECT `+importedColumns+`, version, schema_version
		FROM imported_transactions WHERE household_id = $1 AND id = $2`, tx.HouseholdID(), id), &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.ImportedTransactionState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.ImportedTransactionState]{}, false, fmt.Errorf("load imported transaction: %w", err)
	}

	f.State = s

	return f, true, nil
}

// Save implements eventstore.FoldStore.
func (ImportedTransactionFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.ImportedTransactionState], expected int64) error {
	s := f.State

	var valueDate *time.Time
	if !s.ValueDate.IsZero() {
		valueDate = &s.ValueDate
	}

	var booked *uuid.UUID
	if s.TransactionID != uuid.Nil {
		booked = &s.TransactionID
	}

	args := []any{
		tx.HouseholdID(), id, s.StatementImportID, s.AccountID, string(s.State), s.Line, s.BookingDate, s.TransactionDate,
		valueDate, s.Amount.String(), s.Currency, s.Description, s.Payee, s.BankTransactionID, f.Version, f.SchemaVersion,
		booked,
	}
	q := `INSERT INTO imported_transactions (household_id, id, statement_import_id, account_id, state, line, booking_date,
	          transaction_date, value_date, amount, currency, description, payee, bank_transaction_id, version,
	          schema_version, transaction_id)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::numeric, $11, $12, $13, $14, $15, $16, $17)
	      ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE imported_transactions SET statement_import_id = $3, account_id = $4, state = $5, line = $6,
		         booking_date = $7, transaction_date = $8, value_date = $9, amount = $10::numeric, currency = $11,
		         description = $12, payee = $13, bank_transaction_id = $14, version = $15, schema_version = $16,
		         transaction_id = $17
		     WHERE household_id = $1 AND id = $2 AND version = $18`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save imported transaction: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	return nil
}

// ImportedTransactionReader implements app.ImportedTransactionReader.
type ImportedTransactionReader struct{}

var _ app.ImportedTransactionReader = ImportedTransactionReader{}

// ImportedTransactions implements app.ImportedTransactionReader.
func (ImportedTransactionReader) ImportedTransactions(ctx context.Context, tx *eventstore.Tx, account uuid.UUID,
	filter app.ImportedFilter,
) ([]app.ImportedTransaction, error) {
	var from, to *time.Time

	if !filter.Month.IsZero() {
		start, end := filter.Month, filter.Month.AddDate(0, 1, 0)
		from, to = &start, &end
	}

	var (
		afterDate *time.Time
		afterID   *uuid.UUID
	)

	if c := filter.After; c != nil {
		afterDate, afterID = &c.BookingDate, &c.ID
	}

	rows, err := tx.Query(ctx, `SELECT `+importedColumns+`, id FROM imported_transactions
		WHERE household_id = $1 AND account_id = $2
		  AND ($3 = '' OR state = $3)
		  AND ($4::date IS NULL OR (booking_date >= $4 AND booking_date < $5))
		  AND ($6::date IS NULL OR (booking_date, id) > ($6, $7::uuid))
		ORDER BY booking_date, id
		LIMIT $8`,
		tx.HouseholdID(), account, string(filter.State), from, to, afterDate, afterID, filter.Limit)
	if err != nil {
		return nil, fmt.Errorf("list imported transactions: %w", err)
	}

	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.ImportedTransaction, error) {
		var i app.ImportedTransaction

		s, err := scanImported(r, &i.ID)
		i.State = s

		return i, err
	})
	if err != nil {
		return nil, fmt.Errorf("list imported transactions: %w", err)
	}

	return list, nil
}
