package wiring

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	months       *ledgerapp.Months
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
		Cash:     a.Kind == ledgerdomain.AccountKindCash,
		Currency: currency, StartDate: a.StartDate, AwaitingFirstStatementImport: a.AwaitingFirstStatementImport,
		OpenMonth: a.OpenMonth,
	}, nil
}

// AccountMonthsForShare implements ingestionapp.Ledger.
func (l ingestionLedger) AccountMonthsForShare(ctx context.Context, tx *eventstore.Tx, account uuid.UUID,
) (ingestionapp.AccountMonths, error) {
	m, err := l.accounts.MonthsForShare(ctx, tx, account)
	if errors.Is(err, ledgerapp.ErrAccountNotFound) {
		return ingestionapp.AccountMonths{}, ingestionapp.ErrAccountNotFound
	}

	if err != nil {
		return ingestionapp.AccountMonths{}, err //nolint:wrapcheck // already wrapped by the Ledger
	}

	return ingestionapp.AccountMonths{StartDate: m.StartDate, OpenMonth: m.OpenMonth}, nil
}

// monthError is the Ledger's refusal about a month in Ingestion's terms,
// or nil when err isn't one.
func monthError(err error) error {
	e, ok := errors.AsType[*ledgerdomain.MonthError](err)
	if !ok {
		return nil
	}

	out := &ingestionapp.MonthError{Month: e.Month, OpenMonth: e.OpenMonth}

	if a, ok := errors.AsType[*ledgerapp.AccountError](err); ok {
		out.AccountID = a.Account
	}

	switch {
	case errors.Is(e.Err, ledgerdomain.ErrMonthClosed):
		out.Err = ingestionapp.ErrMonthClosed
	case errors.Is(e.Err, ledgerdomain.ErrMonthNotOpenYet):
		out.Err = ingestionapp.ErrMonthNotOpenYet
	default:
		return nil
	}

	return fmt.Errorf("%w: %w", out, err)
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

// importedBooking is the Booking in the Ledger's terms.
func importedBooking(b ingestionapp.Booking) ledgerapp.ImportedBooking {
	r := b.Row

	return ledgerapp.ImportedBooking{
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
	}
}

// BookTransfer implements ingestionapp.Ledger.
func (l ingestionLedger) BookTransfer(ctx context.Context, tx *eventstore.Tx, b ingestionapp.TransferBooking) error {
	transfer := ledgerapp.ImportedTransfer{
		Row: importedBooking(b.Booking), Counterpart: b.Counterpart, CounterpartLeg: b.CounterpartLeg,
		MatchLeg: b.MatchLeg,
	}

	if b.MatchRow != nil {
		transfer.MatchRow = new(importedBooking(*b.MatchRow))
	}

	_, err := l.transactions.BookTransfer(ctx, tx, transfer)

	if m := monthError(err); m != nil {
		return m
	}

	if refused := transferRefusal(err); refused != nil {
		return refused
	}

	if errors.Is(err, ledgerapp.ErrAccountNotFound) {
		return ingestionapp.ErrAccountNotFound
	}

	return err //nolint:wrapcheck // already wrapped by the Ledger
}

// BookAccepted implements ingestionapp.Ledger.
func (l ingestionLedger) BookAccepted(ctx context.Context, tx *eventstore.Tx, b ingestionapp.Booking) error {
	err := l.transactions.RecordImported(ctx, tx, importedBooking(b))

	if m := monthError(err); m != nil {
		// Ingestion checked the month first; this is a Close that ran
		// meanwhile.
		return m
	}

	if refused, ok := errors.AsType[*ledgerdomain.DirectionError](err); ok {
		return &ingestionapp.CategoryDirectionError{
			Kind: strings.ToUpper(string(refused.Kind)), Direction: strings.ToUpper(string(refused.Direction)), Err: err,
		}
	}

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

// AccountImports implements ledgerapp.Ingestion.
func (i ledgerIngestion) AccountImports(ctx context.Context, tx *eventstore.Tx, account uuid.UUID,
) (ledgerdomain.AccountImports, error) {
	a, err := i.imported.AccountImports(ctx, tx, account)
	if err != nil {
		return ledgerdomain.AccountImports{}, err //nolint:wrapcheck // already wrapped by Ingestion
	}

	out := ledgerdomain.AccountImports{Months: make(map[string]ledgerdomain.MonthImports, len(a.Months))}

	for m, c := range a.Months {
		out.Months[m] = ledgerdomain.MonthImports{Pending: c.Pending, SuspectedDuplicates: c.SuspectedDuplicates}
	}

	for _, u := range a.Unchecked {
		out.Unchecked = append(out.Unchecked, ledgerdomain.UncheckedImportFrom{
			UncheckedImport: uncheckedImport(u.UncheckedImport), From: u.From,
		})
	}

	return out, nil
}

// UnresolvedRows implements ledgerapp.Ingestion.
func (i ledgerIngestion) UnresolvedRows(ctx context.Context, tx *eventstore.Tx, account *uuid.UUID,
) (map[uuid.UUID]ledgerapp.ImportedRows, error) {
	counts, err := i.imported.UnresolvedRows(ctx, tx, account)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped by Ingestion
	}

	out := make(map[uuid.UUID]ledgerapp.ImportedRows, len(counts))
	for id, c := range counts {
		out[id] = ledgerapp.ImportedRows{Pending: c.Pending, SuspectedDuplicates: c.SuspectedDuplicates}
	}

	return out, nil
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
		out.UncheckedImports = append(out.UncheckedImports, uncheckedImport(u))
	}

	return out
}

// uncheckedImport translates an unchecked Statement import into the
// Ledger's terms.
func uncheckedImport(u ingestionapp.UncheckedImport) ledgerdomain.UncheckedImport {
	check := ledgerdomain.ImportCheckWaiting
	if u.BalanceCheck == ingestiondomain.BalanceCheckFailed {
		check = ledgerdomain.ImportCheckFailed
	}

	return ledgerdomain.UncheckedImport{StatementImportID: u.ID, Check: check}
}

// transferRefusal is the Ledger's refused Transfer in Ingestion's terms, or
// nil when err isn't one.
func transferRefusal(err error) error {
	if refused, ok := errors.AsType[*ledgerapp.TransferRefusedError](err); ok {
		out := &ingestionapp.TransferRefusedError{Reason: string(refused.Reason), Err: err}

		if a, ok := errors.AsType[*ledgerapp.AccountError](err); ok {
			out.AccountID = a.Account
		}

		if d, ok := errors.AsType[*ledgerdomain.StartDateError](err); ok {
			out.Date, out.StartDate = d.BookingDate, d.StartDate
		}

		return out
	}

	return nil
}

// TransferSuggestions implements ingestionapp.Ledger.
func (l ingestionLedger) TransferSuggestions(ctx context.Context, tx *eventstore.Tx,
	row ingestiondomain.ImportedTransactionState, counterpart *uuid.UUID,
) (ingestionapp.LedgerSuggestions, error) {
	got, err := l.transactions.TransferSuggestions(ctx, tx, ledgerapp.SuggestionQuery{
		RowAccount: row.AccountID, Counterpart: counterpart, Amount: row.Amount, Currency: row.Currency,
		TransactionDate: row.TransactionDate,
	})
	if refused := transferRefusal(err); refused != nil {
		return ingestionapp.LedgerSuggestions{}, refused
	}

	if err != nil {
		return ingestionapp.LedgerSuggestions{}, err //nolint:wrapcheck // already wrapped by the Ledger
	}

	out := ingestionapp.LedgerSuggestions{
		Legs: make([]ingestionapp.SuggestedLeg, 0, len(got.Legs)), StatementAccounts: got.StatementAccounts,
	}

	for _, d := range got.Legs {
		out.Legs = append(out.Legs, ingestionapp.SuggestedLeg{
			TransactionID: d.ID, AccountID: d.AccountID, Amount: d.Amount, Currency: d.Currency,
			BookingDate: d.BookingDate, TransactionDate: d.TransactionDate, Description: d.Description,
			Payee: d.Payee, CategoryID: d.CategoryID, CategoryName: d.CategoryName,
			CounterpartAccountID: d.CounterpartAccountID,
		})
	}

	return out, nil
}
