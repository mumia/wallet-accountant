package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// Stream type and event types of the Match aggregate (ADR-0015).
const (
	MatchStreamType    = "ledger.match"
	MatchConfirmedType = "io.walletaccountant.ledger.match.confirmed.v1"
	MatchRemovedType   = "io.walletaccountant.ledger.match.removed.v1"
)

var (
	// ErrMatchNotFound is returned for a Match that is not in the Household,
	// or was removed.
	ErrMatchNotFound = errors.New("match not found")
	// ErrInvalidTransfer is returned, as a *TransferRefusedError, for a
	// Transfer the Ledger refuses.
	ErrInvalidTransfer = errors.New("invalid transfer")
	// ErrTransferLeg is returned for setting the Category of a Transfer leg.
	ErrTransferLeg = domain.ErrTransferLeg
)

// TransferReason is why a Transfer is refused, as the API names it.
type TransferReason string

// The reasons a Transfer is refused (the contract's TransferProblem).
const (
	TransferSameAccount        TransferReason = "SAME_ACCOUNT"
	TransferCounterpartNotCash TransferReason = "COUNTERPART_NOT_CASH"
	TransferCurrencyMatch      TransferReason = "CURRENCY_MISMATCH"
	TransferUnknownAccount     TransferReason = "UNKNOWN_ACCOUNT"
	TransferNotAMatchingLeg    TransferReason = "NOT_A_MATCHING_LEG"
	TransferBeforeStartDate    TransferReason = "BEFORE_START_DATE"
)

// TransferRefusedError is a refused Transfer and why.
type TransferRefusedError struct {
	Reason TransferReason
	Err    error
}

func (e *TransferRefusedError) Error() string {
	return fmt.Sprintf("%s (%s): %v", ErrInvalidTransfer, e.Reason, e.Err)
}

func (e *TransferRefusedError) Unwrap() []error { return []error{ErrInvalidTransfer, e.Err} }

// AccountError is a refusal by the Account Account's Decider, such as a
// closed month, when a command books on more than one Account: it says
// which one refused.
type AccountError struct {
	Account uuid.UUID
	Err     error
}

func (e *AccountError) Error() string { return fmt.Sprintf("account %s: %v", e.Account, e.Err) }

func (e *AccountError) Unwrap() error { return e.Err }

// counterpartLeg turns the counterpart cash Account's refusal of a leg
// dated before its start date into a refused Transfer
// (BEFORE_START_DATE): that Account didn't exist yet on the leg's date.
// The *AccountError stays in the chain, naming the Account. Any other
// error is returned as it is.
func counterpartLeg(err error) error {
	if errors.Is(err, domain.ErrBookedBeforeStartDate) {
		return &TransferRefusedError{Reason: TransferBeforeStartDate, Err: err}
	}

	return err
}

// MatchAggregate is the Match Decider wired to the runtime.
type MatchAggregate = eventstore.Aggregate[domain.MatchState, domain.MatchCommand, domain.MatchEvent]

// NewMatchAggregate wires the Match Decider to its fold store.
func NewMatchAggregate(folds eventstore.FoldStore[domain.MatchState]) MatchAggregate {
	codec := eventstore.NewCodec[domain.MatchEvent](Source)
	eventstore.Register[domain.MatchEvent, domain.MatchConfirmed](codec, MatchConfirmedType)
	eventstore.Register[domain.MatchEvent, domain.MatchRemoved](codec, MatchRemovedType)

	return MatchAggregate{
		StreamType:        MatchStreamType,
		Decider:           domain.Match,
		Codec:             codec,
		Folds:             folds,
		FoldSchemaVersion: 1,
	}
}

// MatchDetail is a confirmed Match as the API sees it.
type MatchDetail struct {
	ID          uuid.UUID
	Kind        domain.MatchKind
	Legs        [2]domain.MatchLeg
	ConfirmedBy uuid.UUID
	ConfirmedAt time.Time
}

// Match returns the Household's Match; a removed one is ErrMatchNotFound.
func (t *Transactions) Match(ctx context.Context, household, id uuid.UUID) (MatchDetail, error) {
	var out MatchDetail

	err := t.scope.InHousehold(ctx, household, func(ctx context.Context, tx *eventstore.Tx, _ uuid.UUID) error {
		fold, found, err := t.matches.Folds.Load(ctx, tx, id)
		if err != nil {
			return fmt.Errorf("load match %s: %w", id, err)
		}

		if !found || fold.State.Removed {
			return ErrMatchNotFound
		}

		s := fold.State
		out = MatchDetail{ID: id, Kind: s.Kind, Legs: s.Legs, ConfirmedBy: s.ConfirmedBy, ConfirmedAt: s.ConfirmedAt}

		return nil
	})
	if err != nil {
		return MatchDetail{}, fmt.Errorf("match %s: %w", id, err)
	}

	return out, nil
}

// TransferLegBooking is a Transfer leg to record: the Transaction on the
// Account, against the Account transfer Category, naming the counterpart
// Account.
type TransferLegBooking struct {
	Account           uuid.UUID
	Counterpart       uuid.UUID
	Transaction       uuid.UUID
	ImportedTx        uuid.UUID
	CreatedByTransfer bool
	BookingDate       time.Time
	TransactionDate   time.Time
	ValueDate         time.Time
	Amount            sharedkernel.Decimal
	Currency          string
	Description       string
	Payee             string
}

// RecordTransferLeg records a Transfer leg in the caller's transaction: the
// counterpart is checked to be another of the Household's Accounts in the
// same Account currency (otherwise a *TransferRefusedError), the Account
// Decider books it in its month, and the Transaction (Account ↔ Account
// transfer) is recorded with its Account Posting lookup row. It is In
// transit until a Match links it (spec "Ledger: Transfers").
func (t *Transactions) RecordTransferLeg(ctx context.Context, tx *eventstore.Tx, b TransferLegBooking) error {
	own, found, err := t.accounts.Folds.Load(ctx, tx, b.Account)
	if err != nil {
		return fmt.Errorf("load account %s: %w", b.Account, err)
	}

	switch {
	case !found:
		return ErrAccountNotFound
	case own.State.Currency != b.Currency:
		return fmt.Errorf("%w: %s, not %s", ErrCurrencyMismatch, b.Currency, own.State.Currency)
	}

	if err := t.checkCounterpart(ctx, tx, b.Account, b.Counterpart, b.Currency); err != nil {
		return err
	}

	category, err := t.internal.InternalCategory(ctx, tx, domain.RoleAccountTransfer)
	if err != nil {
		return fmt.Errorf("find the account transfer category: %w", err)
	}

	if _, err := t.accounts.Execute(ctx, tx, b.Account, domain.BookTransaction{
		TransactionID: b.Transaction, BookingDate: b.BookingDate, Amount: b.Amount,
	}); err != nil {
		return &AccountError{Account: b.Account, Err: fmt.Errorf("book transfer leg: %w", err)}
	}

	if _, err := t.transactions.Execute(ctx, tx, b.Transaction, domain.RecordTransaction{
		ImportedTransactionID: b.ImportedTx,
		CounterpartAccountID:  b.Counterpart,
		CreatedByTransfer:     b.CreatedByTransfer,
		BookingDate:           b.BookingDate,
		TransactionDate:       b.TransactionDate,
		ValueDate:             b.ValueDate,
		Description:           b.Description,
		Payee:                 b.Payee,
		Postings: []domain.Posting{
			{AccountID: b.Account, Amount: b.Amount, Currency: b.Currency},
			{CategoryID: category, Amount: b.Amount.Neg(), Currency: b.Currency},
		},
	}); err != nil {
		return fmt.Errorf("record transfer leg: %w", err)
	}

	return nil
}

// checkCounterpart refuses a counterpart that is the leg's own Account, not
// one of the Household's Accounts, or in another Account currency.
func (t *Transactions) checkCounterpart(ctx context.Context, tx *eventstore.Tx, account, counterpart uuid.UUID,
	currency string,
) error {
	if account == counterpart {
		return &TransferRefusedError{Reason: TransferSameAccount, Err: domain.ErrCounterpartIsItself}
	}

	fold, found, err := t.accounts.Folds.Load(ctx, tx, counterpart)
	if err != nil {
		return fmt.Errorf("load account %s: %w", counterpart, err)
	}

	switch {
	case !found:
		return &TransferRefusedError{Reason: TransferUnknownAccount, Err: ErrAccountNotFound}
	case fold.State.Currency != currency:
		return &TransferRefusedError{Reason: TransferCurrencyMatch, Err: fmt.Errorf("%w: %s, not %s",
			ErrCurrencyMismatch, fold.State.Currency, currency)}
	}

	return nil
}

// ConfirmTransfer confirms a new Match between the Transfer legs first and
// second in the caller's transaction, and marks both Matched, returning the
// Match. Legs that aren't the two sides of one Transfer, or one already
// Matched or reversed, are a *TransferRefusedError (NOT_A_MATCHING_LEG). The
// caller is the Match's confirmed_by. A second Match racing for a leg fails
// that leg's version check.
func (t *Transactions) ConfirmTransfer(ctx context.Context, tx *eventstore.Tx, first, second uuid.UUID,
) (uuid.UUID, error) {
	id, err := sharedkernel.NewID()
	if err != nil {
		return uuid.Nil, err //nolint:wrapcheck // already wrapped by the shared kernel
	}

	var legs [2]domain.TransferLeg

	for i, leg := range [2]uuid.UUID{first, second} {
		fold, found, err := t.transactions.Folds.Load(ctx, tx, leg)
		if err != nil {
			return uuid.Nil, fmt.Errorf("load transaction %s: %w", leg, err)
		}

		if !found {
			return uuid.Nil, ErrTransactionNotFound
		}

		legs[i] = fold.State.Leg(leg)
	}

	refused := func(err error) error {
		return &TransferRefusedError{Reason: TransferNotAMatchingLeg, Err: err}
	}

	if _, err := t.matches.Execute(ctx, tx, id, domain.ConfirmMatch{
		Kind: domain.MatchKindAccountTransfer, Legs: legs,
	}); err != nil {
		if errors.Is(err, domain.ErrLegsDontMatch) {
			return uuid.Nil, refused(err)
		}

		return uuid.Nil, fmt.Errorf("confirm match: %w", err)
	}

	for _, leg := range legs {
		if _, err := t.transactions.Execute(ctx, tx, leg.TransactionID, domain.MatchTransaction{MatchID: id}); err != nil {
			if errors.Is(err, domain.ErrTransactionAlreadyMatched) || errors.Is(err, domain.ErrTransactionReversed) ||
				errors.Is(err, domain.ErrNotATransferLeg) {
				return uuid.Nil, refused(err)
			}

			return uuid.Nil, fmt.Errorf("match transaction %s: %w", leg.TransactionID, err)
		}
	}

	return id, nil
}

// ImportedTransfer is an Imported transaction Accepted as a Transfer to or
// from a cash Account: the row as ImportedBooking carries it (its Category
// is ignored), the cash Account, and the ID of the cash leg to record; or,
// with MatchLeg, the leg the cash Account already has to Match instead.
type ImportedTransfer struct {
	Row         ImportedBooking
	CashAccount uuid.UUID
	CashLeg     uuid.UUID
	MatchLeg    *uuid.UUID
}

// SuggestionWindow is how far either side of a row's Transaction date a
// cash leg is suggested for it.
const SuggestionWindow = 7 * 24 * time.Hour

// checkCashCounterpart refuses, for a bank row on the Account account in
// currency, a cash Account that is the row's own, not the Household's, not
// cash, or in another currency.
func (t *Transactions) checkCashCounterpart(ctx context.Context, tx *eventstore.Tx, account, cash uuid.UUID,
	currency string,
) error {
	if cash == account {
		return &TransferRefusedError{Reason: TransferSameAccount, Err: domain.ErrCounterpartIsItself}
	}

	fold, found, err := t.accounts.Folds.Load(ctx, tx, cash)
	if err != nil {
		return fmt.Errorf("load account %s: %w", cash, err)
	}

	switch {
	case !found:
		return &TransferRefusedError{Reason: TransferUnknownAccount, Err: ErrAccountNotFound}
	case !fold.State.IsCash():
		return &TransferRefusedError{Reason: TransferCounterpartNotCash, Err: ErrNotACashAccount}
	case fold.State.Currency != currency:
		return &TransferRefusedError{Reason: TransferCurrencyMatch, Err: fmt.Errorf("%w: %s, not %s",
			ErrCurrencyMismatch, fold.State.Currency, currency)}
	}

	return nil
}

// matchingLeg refuses, as NOT_A_MATCHING_LEG, a leg to Match that isn't an
// In transit leg on the cash Account naming the row's Account, with the
// opposite amount. There is no date window (spec "Accept as Transfer").
func (t *Transactions) matchingLeg(ctx context.Context, tx *eventstore.Tx, leg uuid.UUID, b ImportedTransfer) error {
	fold, found, err := t.transactions.Folds.Load(ctx, tx, leg)
	if err != nil {
		return fmt.Errorf("load transaction %s: %w", leg, err)
	}

	l := fold.State.Leg(leg)

	if !found || !fold.State.InTransit() || l.AccountID != b.CashAccount || l.CounterpartAccountID != b.Row.Account ||
		l.Currency != b.Row.Currency || !l.Amount.Equal(b.Row.Amount.Neg()) {
		return &TransferRefusedError{Reason: TransferNotAMatchingLeg, Err: fmt.Errorf(
			"%w: %s isn't an in transit leg of %s %s on the cash account", domain.ErrLegsDontMatch, leg,
			b.Row.Amount.Neg(), b.Row.Currency)}
	}

	return nil
}

// BookTransfer books an Imported transaction Accepted as a Transfer, in the
// caller's transaction (spec "Accept as Transfer"):
//   - the cash Account is checked: another of the Household's Accounts, a
//     cash one, in the row's currency (otherwise a *TransferRefusedError);
//   - the bank leg is recorded exactly as an Accept records the row, linked
//     to it, against Account transfer, naming the cash Account;
//   - the cash leg is recorded with the opposite amount, the row's
//     description and payee, on the row's Transaction date, flagged as
//     created by the Transfer; a closed month of the cash Account is
//     refused as an *AccountError naming it, and a date before its start
//     date as a *TransferRefusedError (BEFORE_START_DATE);
//   - the two legs are Matched, the caller confirming it.
//
// The legs are booked in Account ID order, so two Transfers between the
// same Accounts write their folds in the same order and can't deadlock.
// It returns the Match (consistency map: Accept as Transfer).
func (t *Transactions) BookTransfer(ctx context.Context, tx *eventstore.Tx, b ImportedTransfer) (uuid.UUID, error) {
	r := b.Row

	if err := t.checkCashCounterpart(ctx, tx, r.Account, b.CashAccount, r.Currency); err != nil {
		return uuid.Nil, err
	}

	bank := TransferLegBooking{
		Account: r.Account, Counterpart: b.CashAccount, Transaction: r.Transaction, ImportedTx: r.ImportedTransaction,
		BookingDate: r.BookingDate, TransactionDate: r.TransactionDate, ValueDate: r.ValueDate,
		Amount: r.Amount, Currency: r.Currency, Description: r.Description, Payee: r.Payee,
	}
	cashLeg := TransferLegBooking{
		Account: b.CashAccount, Counterpart: r.Account, Transaction: b.CashLeg, CreatedByTransfer: true,
		BookingDate: r.TransactionDate, TransactionDate: r.TransactionDate,
		Amount: r.Amount.Neg(), Currency: r.Currency, Description: r.Description, Payee: r.Payee,
	}

	if b.MatchLeg != nil {
		// The cash leg exists: only the bank leg is booked, so only its
		// Account's fold is written.
		if err := t.matchingLeg(ctx, tx, *b.MatchLeg, b); err != nil {
			return uuid.Nil, err
		}

		if err := t.RecordTransferLeg(ctx, tx, bank); err != nil {
			return uuid.Nil, err
		}

		return t.ConfirmTransfer(ctx, tx, bank.Transaction, *b.MatchLeg)
	}

	legs := []TransferLegBooking{bank, cashLeg}
	if cashLeg.Account.String() < bank.Account.String() {
		legs = []TransferLegBooking{cashLeg, bank}
	}

	for _, leg := range legs {
		err := t.RecordTransferLeg(ctx, tx, leg)
		if err != nil && leg.Account == cashLeg.Account {
			return uuid.Nil, counterpartLeg(err)
		}

		if err != nil {
			return uuid.Nil, err
		}
	}

	return t.ConfirmTransfer(ctx, tx, bank.Transaction, cashLeg.Transaction)
}

// SuggestionQuery asks which cash legs could be the other side of a bank
// row: the row's Account, signed amount, currency and Transaction date,
// and the cash Account whose legs are suggested.
type SuggestionQuery struct {
	RowAccount      uuid.UUID
	CashAccount     uuid.UUID
	Amount          sharedkernel.Decimal
	Currency        string
	TransactionDate time.Time
}

// TransferSuggestions lists, in the caller's transaction, the cash
// Account's In transit legs naming the row's Account, with the opposite
// amount, dated within SuggestionWindow of the row's Transaction date,
// closest first. They are worked out on each request and never stored. A
// cash Account the row can't be a Transfer with is a *TransferRefusedError.
func (t *Transactions) TransferSuggestions(ctx context.Context, tx *eventstore.Tx, q SuggestionQuery,
) ([]TransactionDetail, error) {
	if err := t.checkCashCounterpart(ctx, tx, q.RowAccount, q.CashAccount, q.Currency); err != nil {
		return nil, err
	}

	ids, err := t.reader.TransferSuggestions(ctx, tx, q)
	if err != nil {
		return nil, err //nolint:wrapcheck // the adapter's error
	}

	out := make([]TransactionDetail, 0, len(ids))

	for _, id := range ids {
		fold, found, err := t.transactions.Folds.Load(ctx, tx, id)
		if err != nil {
			return nil, fmt.Errorf("load transaction %s: %w", id, err)
		}

		if !found {
			continue
		}

		d, err := t.detail(ctx, tx, id, fold.State)
		if err != nil {
			return nil, err
		}

		out = append(out, d)
	}

	return out, nil
}
