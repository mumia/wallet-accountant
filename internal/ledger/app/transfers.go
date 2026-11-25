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
	TransferSameAccount     TransferReason = "SAME_ACCOUNT"
	TransferCurrencyMatch   TransferReason = "CURRENCY_MISMATCH"
	TransferUnknownAccount  TransferReason = "UNKNOWN_ACCOUNT"
	TransferNotAMatchingLeg TransferReason = "NOT_A_MATCHING_LEG"
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
		return fmt.Errorf("book transfer leg on account %s: %w", b.Account, err)
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
