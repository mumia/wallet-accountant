package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mumia/wallet-accountant/internal/ledger/app"
	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// TransactionFolds stores Transaction folds, and with each one lookup row
// per Account Posting in account_postings, in the same database transaction
// (ADR-0011 "Reads").
type TransactionFolds struct{}

var _ eventstore.FoldStore[domain.TransactionState] = TransactionFolds{}

// Load implements eventstore.FoldStore.
func (TransactionFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.TransactionState], bool, error) {
	var (
		f         eventstore.Fold[domain.TransactionState]
		valueDate *time.Time
		postings  []byte
		imported  *uuid.UUID
	)

	err := tx.QueryRow(ctx, `SELECT opening_balance, booking_date, transaction_date, value_date, description, payee,
		postings, imported_transaction_id, version, schema_version
		FROM ledger_transactions WHERE household_id = $1 AND id = $2`, tx.HouseholdID(), id).
		Scan(&f.State.OpeningBalance, &f.State.BookingDate, &f.State.TransactionDate, &valueDate,
			&f.State.Description, &f.State.Payee, &postings, &imported, &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.TransactionState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.TransactionState]{}, false, fmt.Errorf("load transaction: %w", err)
	}

	f.State.Recorded = true

	if imported != nil {
		f.State.ImportedTransactionID = *imported
	}

	if valueDate != nil {
		f.State.ValueDate = *valueDate
	}

	if err := json.Unmarshal(postings, &f.State.Postings); err != nil {
		return eventstore.Fold[domain.TransactionState]{}, false, fmt.Errorf("decode postings: %w", err)
	}

	return f, true, nil
}

// Save implements eventstore.FoldStore.
func (TransactionFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.TransactionState], expected int64) error {
	s := f.State

	postings, err := json.Marshal(s.Postings)
	if err != nil {
		return fmt.Errorf("encode postings: %w", err)
	}

	var valueDate *time.Time
	if !s.ValueDate.IsZero() {
		valueDate = &s.ValueDate
	}

	var imported *uuid.UUID
	if s.ImportedTransactionID != uuid.Nil {
		imported = &s.ImportedTransactionID
	}

	args := []any{
		tx.HouseholdID(), id, s.OpeningBalance, s.BookingDate, s.TransactionDate, valueDate, s.Description, s.Payee,
		postings, f.Version, f.SchemaVersion, imported,
	}
	q := `INSERT INTO ledger_transactions (household_id, id, opening_balance, booking_date, transaction_date, value_date,
	          description, payee, postings, version, schema_version, imported_transaction_id)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE ledger_transactions SET opening_balance = $3, booking_date = $4, transaction_date = $5,
		         value_date = $6, description = $7, payee = $8, postings = $9, version = $10, schema_version = $11,
		         imported_transaction_id = $12
		     WHERE household_id = $1 AND id = $2 AND version = $13`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save transaction: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	if _, err := tx.Exec(ctx, `DELETE FROM account_postings WHERE household_id = $1 AND transaction_id = $2`,
		tx.HouseholdID(), id); err != nil {
		return fmt.Errorf("save account postings: %w", err)
	}

	for _, l := range s.AccountLines() {
		var category *uuid.UUID
		if l.CategoryID != uuid.Nil {
			category = &l.CategoryID
		}

		if _, err := tx.Exec(ctx, `INSERT INTO account_postings (household_id, transaction_id, ordinal, account_id,
				booking_date, transaction_date, description, amount, currency, category_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8::numeric, $9, $10)`,
			tx.HouseholdID(), id, l.Ordinal, l.AccountID, l.BookingDate, l.TransactionDate, l.Description,
			l.Amount.String(), l.Currency, category); err != nil {
			return fmt.Errorf("save account postings: %w", err)
		}
	}

	return nil
}

// TransactionReader implements app.TransactionReader.
type TransactionReader struct{}

var _ app.TransactionReader = TransactionReader{}

// AccountTransactions implements app.TransactionReader with the Account
// Postings' lookup rows, named with their Category.
func (TransactionReader) AccountTransactions(ctx context.Context, tx *eventstore.Tx, account uuid.UUID,
	filter app.TransactionFilter,
) ([]app.AccountTransaction, error) {
	var from, to *time.Time

	if !filter.Month.IsZero() {
		start, end := filter.Month, filter.Month.AddDate(0, 1, 0)
		from, to = &start, &end
	}

	var (
		afterDate *time.Time
		afterID   *uuid.UUID
		afterOrd  int
	)

	if c := filter.After; c != nil {
		afterDate, afterID, afterOrd = &c.BookingDate, &c.TransactionID, c.Ordinal
	}

	rows, err := tx.Query(ctx, `SELECT p.transaction_id, p.ordinal, p.booking_date, p.transaction_date, p.description,
			p.amount::text, p.currency, COALESCE(p.category_id, '00000000-0000-0000-0000-000000000000'),
			COALESCE(c.name, '')
		FROM account_postings p
		LEFT JOIN categories c ON c.household_id = p.household_id AND c.id = p.category_id
		WHERE p.household_id = $1 AND p.account_id = $2
		  AND ($3::date IS NULL OR (p.booking_date >= $3 AND p.booking_date < $4))
		  AND ($5::date IS NULL OR (p.booking_date, p.transaction_id, p.ordinal) > ($5, $6::uuid, $7))
		ORDER BY p.booking_date, p.transaction_id, p.ordinal
		LIMIT $8`,
		tx.HouseholdID(), account, from, to, afterDate, afterID, afterOrd, filter.Limit)
	if err != nil {
		return nil, fmt.Errorf("list account transactions: %w", err)
	}

	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.AccountTransaction, error) {
		var (
			t      app.AccountTransaction
			amount string
		)

		if err := r.Scan(&t.TransactionID, &t.Ordinal, &t.BookingDate, &t.TransactionDate, &t.Description, &amount,
			&t.Currency, &t.CategoryID, &t.CategoryName); err != nil {
			return t, err //nolint:wrapcheck // wrapped below
		}

		d, err := sharedkernel.ParseDecimal(amount)
		t.Amount = d

		return t, err //nolint:wrapcheck // wrapped below
	})
	if err != nil {
		return nil, fmt.Errorf("list account transactions: %w", err)
	}

	return list, nil
}

// InternalCategory implements app.InternalCategories.
func (CategoryReader) InternalCategory(ctx context.Context, tx *eventstore.Tx, role domain.InternalRole) (uuid.UUID, error) {
	var id uuid.UUID

	err := tx.QueryRow(ctx, `SELECT id FROM categories WHERE household_id = $1 AND internal_role = $2`,
		tx.HouseholdID(), string(role)).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("find internal category %s: %w", role, err)
	}

	return id, nil
}
