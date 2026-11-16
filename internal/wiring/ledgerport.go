package wiring

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	ingestionapp "github.com/mumia/wallet-accountant/internal/ingestion/app"
	ledgerapp "github.com/mumia/wallet-accountant/internal/ledger/app"
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
