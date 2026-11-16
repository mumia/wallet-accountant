package postgres

import (
	"context"
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

// AccountFolds stores Account folds. The running balance and Uncategorised
// count per Booking month are kept as lookup rows in account_months, saved
// with the fold.
type AccountFolds struct{}

var _ eventstore.FoldStore[domain.AccountState] = AccountFolds{}

// Load implements eventstore.FoldStore.
func (AccountFolds) Load(ctx context.Context, tx *eventstore.Tx, id uuid.UUID) (eventstore.Fold[domain.AccountState], bool, error) {
	var (
		f       eventstore.Fold[domain.AccountState]
		opening *uuid.UUID
	)

	err := tx.QueryRow(ctx, `SELECT name, kind, currency, start_date, awaiting_first_statement_import,
		opening_balance_transaction_id, version, schema_version
		FROM accounts WHERE household_id = $1 AND id = $2`, tx.HouseholdID(), id).
		Scan(&f.State.Name, &f.State.Kind, &f.State.Currency, &f.State.StartDate,
			&f.State.AwaitingFirstStatementImport, &opening, &f.Version, &f.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventstore.Fold[domain.AccountState]{}, false, nil
	}

	if err != nil {
		return eventstore.Fold[domain.AccountState]{}, false, fmt.Errorf("load account: %w", err)
	}

	f.State.Added = true

	if opening != nil {
		f.State.OpeningBalanceTransactionID = *opening
	}

	if err := loadMonths(ctx, tx, id, &f.State); err != nil {
		return eventstore.Fold[domain.AccountState]{}, false, err
	}

	return f, true, nil
}

// loadMonths reads the Account's net change, Uncategorised count and
// Month close per Booking month into s; each map stays nil when it has no
// month.
func loadMonths(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, s *domain.AccountState) error {
	rows, err := tx.Query(ctx, `SELECT booking_month, net_change::text, uncategorised, statement_balance::text,
			closed_at, closed_by, flagged
		FROM account_months WHERE household_id = $1 AND account_id = $2`, tx.HouseholdID(), account)
	if err != nil {
		return fmt.Errorf("load account months: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			month, change string
			count         int
			statement     *string
			closedAt      *time.Time
			closedBy      *uuid.UUID
			flagged       bool
		)

		if err := rows.Scan(&month, &change, &count, &statement, &closedAt, &closedBy, &flagged); err != nil {
			return fmt.Errorf("load account months: %w", err)
		}

		d, err := sharedkernel.ParseDecimal(change)
		if err != nil {
			return err //nolint:wrapcheck // already wrapped by the shared kernel
		}

		if s.Months == nil {
			s.Months = map[string]sharedkernel.Decimal{}
		}

		s.Months[month] = d

		if count != 0 {
			if s.Uncategorised == nil {
				s.Uncategorised = map[string]int{}
			}

			s.Uncategorised[month] = count
		}

		if statement != nil {
			if s.Closed, err = closeOf(s.Closed, month, *statement, closedAt, closedBy, flagged); err != nil {
				return err
			}
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("load account months: %w", err)
	}

	return nil
}

// closeOf adds a stored Month close to closes.
func closeOf(closes map[string]domain.MonthClose, month, statement string, closedAt *time.Time, closedBy *uuid.UUID,
	flagged bool,
) (map[string]domain.MonthClose, error) {
	balance, err := sharedkernel.ParseDecimal(statement)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	c := domain.MonthClose{StatementBalance: balance, Flagged: flagged}
	if closedAt != nil {
		c.ClosedAt = closedAt.UTC()
	}

	if closedBy != nil {
		c.ClosedBy = *closedBy
	}

	if closes == nil {
		closes = map[string]domain.MonthClose{}
	}

	closes[month] = c

	return closes, nil
}

// Save implements eventstore.FoldStore.
func (AccountFolds) Save(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, f eventstore.Fold[domain.AccountState], expected int64) error {
	s := f.State

	var opening *uuid.UUID
	if s.OpeningBalanceTransactionID != uuid.Nil {
		opening = &s.OpeningBalanceTransactionID
	}

	args := []any{
		tx.HouseholdID(), id, s.Name, s.Kind, s.Currency, s.StartDate, s.AwaitingFirstStatementImport,
		opening, f.Version, f.SchemaVersion,
	}
	q := `INSERT INTO accounts (household_id, id, name, kind, currency, start_date, awaiting_first_statement_import,
	          opening_balance_transaction_id, version, schema_version)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE accounts SET name = $3, kind = $4, currency = $5, start_date = $6,
		         awaiting_first_statement_import = $7, opening_balance_transaction_id = $8, version = $9,
		         schema_version = $10
		     WHERE household_id = $1 AND id = $2 AND version = $11`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save account: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	if _, err := tx.Exec(ctx, `DELETE FROM account_months WHERE household_id = $1 AND account_id = $2`,
		tx.HouseholdID(), id); err != nil {
		return fmt.Errorf("save account months: %w", err)
	}

	// A month with an Uncategorised count normally moved the balance too,
	// but a Transaction of zero doesn't, and a month can close with no
	// Transaction at all: save every month any map has.
	months := map[string]struct{}{}
	for m := range s.Months {
		months[m] = struct{}{}
	}

	for m := range s.Uncategorised {
		months[m] = struct{}{}
	}

	for m := range s.Closed {
		months[m] = struct{}{}
	}

	for month := range months {
		var (
			statement *string
			closedAt  *time.Time
			closedBy  *uuid.UUID
		)

		c, closed := s.Closed[month]
		if closed {
			statement = new(c.StatementBalance.String())

			if !c.ClosedAt.IsZero() {
				closedAt = &c.ClosedAt
			}

			if c.ClosedBy != uuid.Nil {
				closedBy = &c.ClosedBy
			}
		}

		if _, err := tx.Exec(ctx, `INSERT INTO account_months (household_id, account_id, booking_month, net_change,
				uncategorised, statement_balance, closed_at, closed_by, flagged)
			VALUES ($1, $2, $3, $4::numeric, $5, $6::numeric, $7, $8, $9)`, tx.HouseholdID(), id, month,
			s.Months[month].String(), s.Uncategorised[month], statement, closedAt, closedBy, c.Flagged); err != nil {
			return fmt.Errorf("save account months: %w", err)
		}
	}

	return nil
}

// AccountReader implements app.AccountReader.
type AccountReader struct{}

var _ app.AccountReader = AccountReader{}

// Accounts implements app.AccountReader. Account IDs are UUIDv7, so the
// Accounts come in the order they were added. The balance is the sum of the
// Account's Booking months, and only an Account with its Opening balance has
// one.
func (AccountReader) Accounts(ctx context.Context, tx *eventstore.Tx) ([]app.Account, error) {
	rows, err := tx.Query(ctx, `SELECT a.id, a.name, a.kind, a.currency, a.start_date, a.awaiting_first_statement_import,
			(SELECT COALESCE(sum(m.net_change), 0)::text FROM account_months m
			  WHERE m.household_id = a.household_id AND m.account_id = a.id)
		FROM accounts a WHERE a.household_id = $1 ORDER BY a.id`, tx.HouseholdID())
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.Account, error) {
		var (
			a       app.Account
			balance string
		)

		if err := r.Scan(&a.ID, &a.Name, &a.Kind, &a.Currency, &a.StartDate, &a.AwaitingFirstStatementImport, &balance); err != nil {
			return a, err //nolint:wrapcheck // wrapped below
		}

		if !a.AwaitingFirstStatementImport {
			d, err := sharedkernel.ParseDecimal(balance)
			if err != nil {
				return a, err //nolint:wrapcheck // wrapped below
			}

			a.Balance = &d
		}

		return a, nil
	})
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	return list, nil
}
