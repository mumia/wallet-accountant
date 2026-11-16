package wiring

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	ingestionapp "github.com/mumia/wallet-accountant/internal/ingestion/app"
	ingestiondomain "github.com/mumia/wallet-accountant/internal/ingestion/domain"
	ledgerapp "github.com/mumia/wallet-accountant/internal/ledger/app"
	ledgerdomain "github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// ingestionLedger is Ingestion's Ledger port, served by the Ledger's
// application services. It translates between the two contexts' terms, so
// neither imports the other.
type ingestionLedger struct {
	accounts     *ledgerapp.Accounts
	transactions *ledgerapp.Transactions
}

var _ ingestionapp.Ledger = ingestionLedger{}

// ImportAccount implements ingestionapp.Ledger.
func (l ingestionLedger) ImportAccount(ctx context.Context, tx *eventstore.Tx, account uuid.UUID) (ingestionapp.ImportAccount, error) {
	a, err := l.accounts.AccountForImport(ctx, tx, account)
	if errors.Is(err, ledgerapp.ErrAccountNotFound) {
		return ingestionapp.ImportAccount{}, ingestionapp.ErrAccountNotFound
	}

	if err != nil {
		return ingestionapp.ImportAccount{}, err //nolint:wrapcheck // already wrapped by the Ledger
	}

	currency, err := sharedkernel.ParseCurrency(a.Currency)
	if err != nil {
		return ingestionapp.ImportAccount{}, fmt.Errorf("account %s: %w", account, err)
	}

	return ingestionapp.ImportAccount{
		Currency: currency, StartDate: a.StartDate, AwaitingFirstStatementImport: a.AwaitingFirstStatementImport,
	}, nil
}

// RecordOpeningBalance implements ingestionapp.Ledger.
func (l ingestionLedger) RecordOpeningBalance(ctx context.Context, tx *eventstore.Tx, account, transaction uuid.UUID,
	amount sharedkernel.Decimal,
) error {
	err := l.transactions.RecordOpeningBalance(ctx, tx, account, transaction, amount)

	switch {
	case errors.Is(err, ledgerapp.ErrOpeningBalanceAlreadyRecorded):
		return fmt.Errorf("%w: %w", ingestionapp.ErrOpeningBalanceAlreadyRecorded, err)
	case errors.Is(err, ledgerapp.ErrAccountNotFound):
		return ingestionapp.ErrAccountNotFound
	default:
		return err //nolint:wrapcheck // already wrapped by the Ledger
	}
}

// BookAccepted implements ingestionapp.Ledger.
func (l ingestionLedger) BookAccepted(ctx context.Context, tx *eventstore.Tx, b ingestionapp.Booking) error {
	r := b.Row

	err := l.transactions.RecordImported(ctx, tx, ledgerapp.ImportedBooking{
		Account:             r.AccountID,
		Transaction:         b.Transaction,
		ImportedTransaction: b.ImportedTransaction,
		Category:            b.Category,
		BookingDate:         r.BookingDate,
		TransactionDate:     r.TransactionDate,
		ValueDate:           r.ValueDate,
		Amount:              r.Amount,
		Currency:            r.Currency,
		Description:         r.Description,
		Payee:               r.Payee,
	})

	switch {
	case errors.Is(err, ledgerapp.ErrCategoryNotFound):
		return ingestionapp.ErrCategoryNotFound
	case errors.Is(err, ledgerapp.ErrInternalCategory):
		return fmt.Errorf("%w: %w", ingestionapp.ErrInternalCategory, err)
	case errors.Is(err, ledgerapp.ErrAccountNotFound):
		return ingestionapp.ErrAccountNotFound
	default:
		return err //nolint:wrapcheck // already wrapped by the Ledger
	}
}

// ReverseOpeningBalance implements ingestionapp.Ledger.
func (l ingestionLedger) ReverseOpeningBalance(ctx context.Context, tx *eventstore.Tx, account, reversal uuid.UUID) error {
	err := l.transactions.ReverseOpeningBalance(ctx, tx, account, reversal)
	if errors.Is(err, ledgerapp.ErrAccountNotFound) {
		return ingestionapp.ErrAccountNotFound
	}

	return err //nolint:wrapcheck // already wrapped by the Ledger
}

// ledgerIngestion is the Ledger's Ingestion port, served by Ingestion's
// application services.
type ledgerIngestion struct {
	imported *ingestionapp.ImportedTransactions
}

var _ ledgerapp.Ingestion = ledgerIngestion{}

// ReturnToPending implements ledgerapp.Ingestion.
func (i ledgerIngestion) ReturnToPending(ctx context.Context, tx *eventstore.Tx, importedTransaction, transaction uuid.UUID,
) error {
	return i.imported.ReturnToPending(ctx, tx, importedTransaction, transaction) //nolint:wrapcheck // already wrapped by Ingestion
}

// MonthImports implements ledgerapp.Ingestion.
func (i ledgerIngestion) MonthImports(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, month time.Time,
) (ledgerdomain.MonthImports, error) {
	m, err := i.imported.MonthImports(ctx, tx, account, month)
	if err != nil {
		return ledgerdomain.MonthImports{}, err //nolint:wrapcheck // already wrapped by Ingestion
	}

	return monthImports(m), nil
}

// MonthImportsForClose implements ledgerapp.Ingestion.
func (i ledgerIngestion) MonthImportsForClose(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, month time.Time,
) (ledgerdomain.MonthImports, error) {
	m, err := i.imported.MonthImportsForClose(ctx, tx, account, month)
	if err != nil {
		return ledgerdomain.MonthImports{}, err //nolint:wrapcheck // already wrapped by Ingestion
	}

	return monthImports(m), nil
}

// monthImports translates Ingestion's report on a month into the Ledger's
// terms.
func monthImports(m ingestionapp.MonthImports) ledgerdomain.MonthImports {
	out := ledgerdomain.MonthImports{Pending: m.Pending, SuspectedDuplicates: m.SuspectedDuplicates}

	for _, u := range m.UncheckedImports {
		check := ledgerdomain.ImportCheckWaiting
		if u.BalanceCheck == ingestiondomain.BalanceCheckFailed {
			check = ledgerdomain.ImportCheckFailed
		}

		out.UncheckedImports = append(out.UncheckedImports, ledgerdomain.UncheckedImport{
			StatementImportID: u.ID, Check: check,
		})
	}

	return out
}
