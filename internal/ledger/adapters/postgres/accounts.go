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
		opening_balance_transaction_id, open_month, version, schema_version
		FROM accounts WHERE household_id = $1 AND id = $2`, tx.HouseholdID(), id).
		Scan(&f.State.Name, &f.State.Kind, &f.State.Currency, &f.State.StartDate,
			&f.State.AwaitingFirstStatementImport, &opening, &f.State.OpenMonth, &f.Version, &f.SchemaVersion)
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

// loadMonths reads the Account's net change, Uncategorised count,
// unreversed Transactions and Month close per Booking month into s; each
// map stays nil when it has no month.
func loadMonths(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, s *domain.AccountState) error {
	rows, err := tx.Query(ctx, `SELECT booking_month, net_change::text, uncategorised, live, statement_balance::text,
			closed_at, closed_by, counted_on, cash_count::text
		FROM account_months WHERE household_id = $1 AND account_id = $2`, tx.HouseholdID(), account)
	if err != nil {
		return fmt.Errorf("load account months: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			month, change string
			count, live   int
			statement     *string
			closedAt      *time.Time
			closedBy      *uuid.UUID
			countedOn     *time.Time
			cashCount     *string
		)

		if err := rows.Scan(&month, &change, &count, &live, &statement, &closedAt, &closedBy, &countedOn,
			&cashCount); err != nil {
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

		if live != 0 {
			if s.Live == nil {
				s.Live = map[string]int{}
			}

			s.Live[month] = live
		}

		if statement != nil {
			if s.Closed, err = closeOf(s.Closed, month, *statement, closedAt, closedBy); err != nil {
				return err
			}

			if s.Closed, err = countOf(s.Closed, month, countedOn, cashCount); err != nil {
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
) (map[string]domain.MonthClose, error) {
	balance, err := sharedkernel.ParseDecimal(statement)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	c := domain.MonthClose{StatementBalance: balance}
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

// countOf adds a stored Cash count to month's Month close in closes; a
// statement Account's month has none.
func countOf(closes map[string]domain.MonthClose, month string, countedOn *time.Time, count *string,
) (map[string]domain.MonthClose, error) {
	if countedOn == nil || count == nil {
		return closes, nil
	}

	amount, err := sharedkernel.ParseDecimal(*count)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	c := closes[month]
	c.CountedOn, c.Count = *countedOn, amount
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
		opening, f.Version, f.SchemaVersion, s.OpenMonth,
	}
	q := `INSERT INTO accounts (household_id, id, name, kind, currency, start_date, awaiting_first_statement_import,
	          opening_balance_transaction_id, version, schema_version, open_month)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT (id) DO NOTHING`

	if expected != 0 {
		args = append(args, expected)
		q = `UPDATE accounts SET name = $3, kind = $4, currency = $5, start_date = $6,
		         awaiting_first_statement_import = $7, opening_balance_transaction_id = $8, version = $9,
		         schema_version = $10, open_month = $11
		     WHERE household_id = $1 AND id = $2 AND version = $12`
	}

	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("save account: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return eventstore.ErrVersionConflict
	}

	return saveMonths(ctx, tx, id, s)
}

// saveMonths saves the Account's months in two statements, however many it
// has: one upsert that writes only the months that changed, and one delete
// of any month no map holds any more.
func saveMonths(ctx context.Context, tx *eventstore.Tx, id uuid.UUID, s domain.AccountState) error {
	// A month with an Uncategorised count normally moved the balance too,
	// but a Transaction of zero doesn't, and a month can close with no
	// Transaction at all: save every month any map has.
	set := map[string]struct{}{}
	for m := range s.Months {
		set[m] = struct{}{}
	}

	for m := range s.Uncategorised {
		set[m] = struct{}{}
	}

	for m := range s.Closed {
		set[m] = struct{}{}
	}

	for m := range s.Live {
		set[m] = struct{}{}
	}

	var (
		months, changes []string
		statements      []*string
		counts, lives   []int
		closedAts       []*time.Time
		closedBys       []*uuid.UUID
		countedOns      []*time.Time
		cashCounts      []*string
	)

	for month := range set {
		var (
			statement *string
			closedAt  *time.Time
			closedBy  *uuid.UUID
			countedOn *time.Time
			cashCount *string
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

			if !c.CountedOn.IsZero() {
				countedOn, cashCount = &c.CountedOn, new(c.Count.String())
			}
		}

		months = append(months, month)
		changes = append(changes, s.Months[month].String())
		counts = append(counts, s.Uncategorised[month])
		lives = append(lives, s.Live[month])
		statements = append(statements, statement)
		closedAts = append(closedAts, closedAt)
		closedBys = append(closedBys, closedBy)
		countedOns = append(countedOns, countedOn)
		cashCounts = append(cashCounts, cashCount)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO account_months AS m (household_id, account_id, booking_month, net_change,
			uncategorised, live, statement_balance, closed_at, closed_by, counted_on, cash_count)
		SELECT $1, $2, u.month, u.change::numeric, u.count, u.live, u.statement::numeric, u.closed_at, u.closed_by,
			u.counted_on, u.cash_count::numeric
		FROM unnest($3::text[], $4::text[], $5::int[], $6::int[], $7::text[], $8::timestamptz[], $9::uuid[],
				$10::date[], $11::text[])
			AS u (month, change, count, live, statement, closed_at, closed_by, counted_on, cash_count)
		ON CONFLICT (account_id, booking_month) DO UPDATE SET net_change = EXCLUDED.net_change,
			uncategorised = EXCLUDED.uncategorised, live = EXCLUDED.live,
			statement_balance = EXCLUDED.statement_balance, closed_at = EXCLUDED.closed_at,
			closed_by = EXCLUDED.closed_by, counted_on = EXCLUDED.counted_on, cash_count = EXCLUDED.cash_count
		WHERE (m.net_change, m.uncategorised, m.live, m.statement_balance, m.closed_at, m.closed_by, m.counted_on,
				m.cash_count)
			IS DISTINCT FROM (EXCLUDED.net_change, EXCLUDED.uncategorised, EXCLUDED.live,
				EXCLUDED.statement_balance, EXCLUDED.closed_at, EXCLUDED.closed_by, EXCLUDED.counted_on,
				EXCLUDED.cash_count)`,
		tx.HouseholdID(), id, months, changes, counts, lives, statements, closedAts, closedBys, countedOns,
		cashCounts); err != nil {
		return fmt.Errorf("save account months: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM account_months
		WHERE household_id = $1 AND account_id = $2 AND NOT booking_month = ANY($3::text[])`,
		tx.HouseholdID(), id, months); err != nil {
		return fmt.Errorf("save account months: %w", err)
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
			a.open_month,
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

		if err := r.Scan(&a.ID, &a.Name, &a.Kind, &a.Currency, &a.StartDate, &a.AwaitingFirstStatementImport,
			&a.OpenMonth, &balance); err != nil {
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

// MonthsForShare implements app.AccountReader.
func (AccountReader) MonthsForShare(ctx context.Context, tx *eventstore.Tx, account uuid.UUID,
) (app.AccountMonths, bool, error) {
	var m app.AccountMonths

	err := tx.QueryRow(ctx, `SELECT start_date, open_month FROM accounts WHERE household_id = $1 AND id = $2
		FOR SHARE`, tx.HouseholdID(), account).Scan(&m.StartDate, &m.OpenMonth)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.AccountMonths{}, false, nil
	}

	if err != nil {
		return app.AccountMonths{}, false, fmt.Errorf("lock account %s: %w", account, err)
	}

	return m, true, nil
}

// MonthReader implements app.MonthReader.
type MonthReader struct{}

var _ app.MonthReader = MonthReader{}

// LegsInTransit implements app.MonthReader with the Account Posting lookup
// rows of Transfer legs that are neither Matched nor reversed.
func (MonthReader) LegsInTransit(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (map[string]int, error) {
	rows, err := tx.Query(ctx, `SELECT to_char(p.booking_date, 'YYYY-MM'), count(*)
		FROM account_postings p
		JOIN ledger_transactions t ON t.household_id = p.household_id AND t.id = p.transaction_id
		WHERE p.household_id = $1 AND p.account_id = $2 AND t.counterpart_account_id IS NOT NULL
		  AND t.match_id IS NULL AND t.reversed_by_transaction_id IS NULL
		GROUP BY 1`, tx.HouseholdID(), account)
	if err != nil {
		return nil, fmt.Errorf("count account %s legs in transit: %w", account, err)
	}

	counts := map[string]int{}

	for rows.Next() {
		var (
			month string
			n     int
		)

		if err := rows.Scan(&month, &n); err != nil {
			return nil, fmt.Errorf("count account %s legs in transit: %w", account, err)
		}

		counts[month] = n
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("count account %s legs in transit: %w", account, err)
	}

	return counts, nil
}

// MovedAfter implements app.MonthReader with the Account Posting lookup rows,
// once the Account's row is locked for update: a Transaction booked
// concurrently either committed before the lock, and is summed, or waits
// for it and then fails its version check.
func (MonthReader) MovedAfter(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, after, through time.Time,
) (sharedkernel.Decimal, error) {
	if _, err := tx.Exec(ctx, `SELECT 1 FROM accounts WHERE household_id = $1 AND id = $2 FOR UPDATE`,
		tx.HouseholdID(), account); err != nil {
		return sharedkernel.Decimal{}, fmt.Errorf("lock account %s: %w", account, err)
	}

	var moved string

	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(amount), 0)::text FROM account_postings
		WHERE household_id = $1 AND account_id = $2 AND booking_date > $3 AND booking_date <= $4`,
		tx.HouseholdID(), account, after, through).Scan(&moved); err != nil {
		return sharedkernel.Decimal{}, fmt.Errorf("sum account %s postings: %w", account, err)
	}

	d, err := sharedkernel.ParseDecimal(moved)
	if err != nil {
		return sharedkernel.Decimal{}, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	return d, nil
}
