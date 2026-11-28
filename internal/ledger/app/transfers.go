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
	TransferSameAccount             TransferReason = "SAME_ACCOUNT"
	TransferAccountKindNotSupported TransferReason = "ACCOUNT_KIND_NOT_SUPPORTED"
	TransferCurrencyMatch           TransferReason = "CURRENCY_MISMATCH"
	TransferUnknownAccount          TransferReason = "UNKNOWN_ACCOUNT"
	TransferNotAMatchingLeg         TransferReason = "NOT_A_MATCHING_LEG"
	TransferBeforeStartDate         TransferReason = "BEFORE_START_DATE"
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
// transfer) is recorded with its Account Posting lookup row. A loan or
// crypto wallet on either side is refused by the Transaction Decider, as a
// *TransferRefusedError (ACCOUNT_KIND_NOT_SUPPORTED) with an *AccountError
// naming it. It is In transit until a Match links it (spec "Ledger:
// Transfers").
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

	counterpart, err := t.checkCounterpart(ctx, tx, b.Account, b.Counterpart, b.Currency)
	if err != nil {
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
		AccountKind:           own.State.Kind,
		CounterpartKind:       counterpart.Kind,
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
		if kind, ok := errors.AsType[*domain.AccountKindError](err); ok {
			return &TransferRefusedError{Reason: TransferAccountKindNotSupported, Err: &AccountError{
				Account: kind.AccountID, Err: fmt.Errorf("record transfer leg: %w", err),
			}}
		}

		return fmt.Errorf("record transfer leg: %w", err)
	}

	return nil
}

// checkCounterpart refuses a counterpart that is the leg's own Account, not
// one of the Household's Accounts, or in another Account currency, and
// returns it otherwise.
func (t *Transactions) checkCounterpart(ctx context.Context, tx *eventstore.Tx, account, counterpart uuid.UUID,
	currency string,
) (domain.AccountState, error) {
	if account == counterpart {
		return domain.AccountState{}, &TransferRefusedError{Reason: TransferSameAccount, Err: domain.ErrCounterpartIsItself}
	}

	fold, found, err := t.accounts.Folds.Load(ctx, tx, counterpart)
	if err != nil {
		return domain.AccountState{}, fmt.Errorf("load account %s: %w", counterpart, err)
	}

	switch {
	case !found:
		return domain.AccountState{}, &TransferRefusedError{Reason: TransferUnknownAccount, Err: ErrAccountNotFound}
	case fold.State.Currency != currency:
		return domain.AccountState{}, &TransferRefusedError{Reason: TransferCurrencyMatch, Err: fmt.Errorf(
			"%w: %s, not %s", ErrCurrencyMismatch, fold.State.Currency, currency)}
	}

	return fold.State, nil
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
// from the Account Counterpart: the row as ImportedBooking carries it (its
// Category is ignored), and for a cash counterpart the ID of the cash leg
// to record, CounterpartLeg. MatchLeg is the In transit leg the
// counterpart already has, to Match instead.
type ImportedTransfer struct {
	Row            ImportedBooking
	Counterpart    uuid.UUID
	CounterpartLeg uuid.UUID
	MatchLeg       *uuid.UUID
}

// SuggestionWindow is how far either side of a row's Transaction date a
// cash leg is suggested for it.
const SuggestionWindow = 7 * 24 * time.Hour

// matchingLeg refuses, as NOT_A_MATCHING_LEG, a leg to Match that isn't an
// In transit leg on the counterpart naming the row's Account, with the
// opposite amount. There is no date window (spec "Accept as Transfer").
func (t *Transactions) matchingLeg(ctx context.Context, tx *eventstore.Tx, leg uuid.UUID, b ImportedTransfer) error {
	fold, found, err := t.transactions.Folds.Load(ctx, tx, leg)
	if err != nil {
		return fmt.Errorf("load transaction %s: %w", leg, err)
	}

	l := fold.State.Leg(leg)

	if !found || !fold.State.InTransit() || l.AccountID != b.Counterpart || l.CounterpartAccountID != b.Row.Account ||
		l.Currency != b.Row.Currency || !l.Amount.Equal(b.Row.Amount.Neg()) {
		return &TransferRefusedError{Reason: TransferNotAMatchingLeg, Err: fmt.Errorf(
			"%w: %s isn't an in transit leg of %s %s on the counterpart", domain.ErrLegsDontMatch, leg,
			b.Row.Amount.Neg(), b.Row.Currency)}
	}

	return nil
}

// BookTransfer books an Imported transaction Accepted as a Transfer, in the
// caller's transaction (spec "Accept as Transfer"; statement-transfers spec
// "Ledger: Transfers between statement Accounts"):
//   - the counterpart is checked: another of the Household's Accounts, in
//     the row's currency (otherwise a *TransferRefusedError); a loan or
//     crypto wallet on either side is refused when the leg is recorded;
//   - the row's leg is recorded exactly as an Accept records the row,
//     linked to it, against Account transfer, naming the counterpart;
//   - with MatchLeg, that In transit leg of the counterpart is checked and
//     the two are Matched;
//   - otherwise, a cash counterpart gets the cash leg, with the opposite
//     amount, the row's description and payee, on the row's Transaction
//     date, flagged as created by the Transfer, and the two are Matched; a
//     closed month of the cash Account is refused as an *AccountError
//     naming it, and a date before its start date as a
//     *TransferRefusedError (BEFORE_START_DATE);
//   - a bank or credit card counterpart gets nothing: the row's leg stays
//     In transit until the counterpart's own row is Accepted and Matched.
//
// The legs are booked in Account ID order, so two Transfers between the
// same Accounts write their folds in the same order and can't deadlock.
// It returns the Match, or uuid.Nil when the leg is left In transit
// (consistency map: Accept as Transfer).
func (t *Transactions) BookTransfer(ctx context.Context, tx *eventstore.Tx, b ImportedTransfer) (uuid.UUID, error) {
	r := b.Row

	counterpart, err := t.checkCounterpart(ctx, tx, r.Account, b.Counterpart, r.Currency)
	if err != nil {
		return uuid.Nil, err
	}

	row := TransferLegBooking{
		Account: r.Account, Counterpart: b.Counterpart, Transaction: r.Transaction, ImportedTx: r.ImportedTransaction,
		BookingDate: r.BookingDate, TransactionDate: r.TransactionDate, ValueDate: r.ValueDate,
		Amount: r.Amount, Currency: r.Currency, Description: r.Description, Payee: r.Payee,
	}

	if b.MatchLeg != nil {
		// The other leg exists: only the row's leg is booked, so only its
		// Account's fold is written.
		if err := t.matchingLeg(ctx, tx, *b.MatchLeg, b); err != nil {
			return uuid.Nil, err
		}

		if err := t.RecordTransferLeg(ctx, tx, row); err != nil {
			return uuid.Nil, err
		}

		return t.ConfirmTransfer(ctx, tx, row.Transaction, *b.MatchLeg)
	}

	if !counterpart.IsCash() {
		// Its leg can only come from its own statement.
		return uuid.Nil, t.RecordTransferLeg(ctx, tx, row)
	}

	cashLeg := TransferLegBooking{
		Account: b.Counterpart, Counterpart: r.Account, Transaction: b.CounterpartLeg, CreatedByTransfer: true,
		BookingDate: r.TransactionDate, TransactionDate: r.TransactionDate,
		Amount: r.Amount.Neg(), Currency: r.Currency, Description: r.Description, Payee: r.Payee,
	}

	legs := []TransferLegBooking{row, cashLeg}
	if cashLeg.Account.String() < row.Account.String() {
		legs = []TransferLegBooking{cashLeg, row}
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

	return t.ConfirmTransfer(ctx, tx, row.Transaction, cashLeg.Transaction)
}

// SuggestionQuery asks what could be the other side of a row as a
// Transfer: the row's Account, signed amount, currency and Transaction
// date, and Counterpart, the one Account to look on, or nil for every
// Account the row could be a Transfer with.
type SuggestionQuery struct {
	RowAccount      uuid.UUID
	Counterpart     *uuid.UUID
	Amount          sharedkernel.Decimal
	Currency        string
	TransactionDate time.Time
}

// LegQuery asks the reader for the In transit legs on Accounts that could be
// the other side of a row: they name RowAccount, have the opposite of
// Amount in Currency, and a Transaction date within SuggestionWindow of
// TransactionDate.
type LegQuery struct {
	RowAccount      uuid.UUID
	Accounts        []uuid.UUID
	Amount          sharedkernel.Decimal
	Currency        string
	TransactionDate time.Time
}

// TransferCounterpart is an Account a row could be a Transfer with.
type TransferCounterpart struct {
	ID   uuid.UUID
	Kind domain.AccountKind
}

// TransferSuggestions is what could be the other side of a row: Legs, the
// In transit legs, closest date first, and StatementAccounts, the bank and
// credit card Accounts among the counterparts looked on, whose pending rows
// Ingestion suggests.
type TransferSuggestions struct {
	Legs              []TransactionDetail
	StatementAccounts []uuid.UUID
}

// transferCounterparts returns the Accounts a row on account, in currency,
// could be a Transfer with: only, when it is given, or else every other
// Account of the Household in that currency of a kind that takes part in
// Transfers. A row's Account or an only that takes no part, or an only
// that is the row's own, not the Household's or in another currency, is a
// *TransferRefusedError.
func (t *Transactions) transferCounterparts(ctx context.Context, tx *eventstore.Tx, account uuid.UUID, currency string,
	only *uuid.UUID,
) ([]TransferCounterpart, error) {
	own, found, err := t.accounts.Folds.Load(ctx, tx, account)
	if err != nil {
		return nil, fmt.Errorf("load account %s: %w", account, err)
	}

	if !found {
		return nil, ErrAccountNotFound
	}

	refusedKind := func(id uuid.UUID, kind domain.AccountKind) error {
		return &TransferRefusedError{Reason: TransferAccountKindNotSupported, Err: &AccountError{
			Account: id, Err: &domain.AccountKindError{AccountID: id, Kind: kind},
		}}
	}

	if !domain.TakesPartInTransfers(own.State.Kind) {
		return nil, refusedKind(account, own.State.Kind)
	}

	if only != nil {
		counterpart, err := t.checkCounterpart(ctx, tx, account, *only, currency)
		if err != nil {
			return nil, err
		}

		if !domain.TakesPartInTransfers(counterpart.Kind) {
			return nil, refusedKind(*only, counterpart.Kind)
		}

		return []TransferCounterpart{{ID: *only, Kind: counterpart.Kind}}, nil
	}

	all, err := t.reader.TransferCounterparts(ctx, tx, account, currency)
	if err != nil {
		return nil, err //nolint:wrapcheck // the adapter's error
	}

	var out []TransferCounterpart

	for _, c := range all {
		if domain.TakesPartInTransfers(c.Kind) {
			out = append(out, c)
		}
	}

	return out, nil
}

// TransferSuggestions works out, in the caller's transaction, what could be
// the other side of a row as a Transfer (statement-transfers spec
// "Suggestions"): the counterparts it could be a Transfer with, their In
// transit legs naming the row's Account, with the opposite amount, dated
// within SuggestionWindow of the row's Transaction date, closest first, and
// which of the counterparts are bank or credit card Accounts. They are
// worked out on each request and never stored. An Account the row can't be
// a Transfer with is a *TransferRefusedError.
func (t *Transactions) TransferSuggestions(ctx context.Context, tx *eventstore.Tx, q SuggestionQuery,
) (TransferSuggestions, error) {
	counterparts, err := t.transferCounterparts(ctx, tx, q.RowAccount, q.Currency, q.Counterpart)
	if err != nil {
		return TransferSuggestions{}, err
	}

	var (
		accounts []uuid.UUID
		out      TransferSuggestions
	)

	for _, c := range counterparts {
		accounts = append(accounts, c.ID)

		if c.Kind != domain.AccountKindCash {
			out.StatementAccounts = append(out.StatementAccounts, c.ID)
		}
	}

	if len(accounts) == 0 {
		return out, nil
	}

	ids, err := t.reader.TransferSuggestions(ctx, tx, LegQuery{
		RowAccount: q.RowAccount, Accounts: accounts, Amount: q.Amount, Currency: q.Currency,
		TransactionDate: q.TransactionDate,
	})
	if err != nil {
		return TransferSuggestions{}, err //nolint:wrapcheck // the adapter's error
	}

	out.Legs = make([]TransactionDetail, 0, len(ids))

	for _, id := range ids {
		fold, found, err := t.transactions.Folds.Load(ctx, tx, id)
		if err != nil {
			return TransferSuggestions{}, fmt.Errorf("load transaction %s: %w", id, err)
		}

		if !found {
			continue
		}

		d, err := t.detail(ctx, tx, id, fold.State)
		if err != nil {
			return TransferSuggestions{}, err
		}

		out.Legs = append(out.Legs, d)
	}

	return out, nil
}
