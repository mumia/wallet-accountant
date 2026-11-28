package domain

import (
	"errors"
	"fmt"
	"slices"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

var (
	// ErrCounterpartIsItself is returned for a Transfer leg whose
	// counterpart Account is its own Account.
	ErrCounterpartIsItself = errors.New("a transfer leg's counterpart is another account")
	// ErrNotATransferLeg is returned for matching a Transaction that isn't a
	// Transfer leg, or for a Transaction created by a Transfer that names
	// no counterpart Account.
	ErrNotATransferLeg = errors.New("not a transfer leg")
	// ErrTransferLeg is returned for setting the Category of a Transfer
	// leg, whose other side is always the Account transfer Category.
	ErrTransferLeg = errors.New("a transfer leg's category is account transfer")
	// ErrTransactionAlreadyMatched is returned for matching a Transfer leg
	// that already has a Match.
	ErrTransactionAlreadyMatched = errors.New("transaction already matched")
	// ErrTransactionNotMatched is returned for unmatching a Transfer leg
	// from a Match it isn't linked by.
	ErrTransactionNotMatched = errors.New("transaction not matched by this match")
	// ErrTransactionMatched is returned for reversing a Transfer leg that
	// is still Matched: its Match is removed first.
	ErrTransactionMatched = errors.New("transaction is matched")
	// ErrMatchIDRequired is returned for matching or unmatching without the
	// Match.
	ErrMatchIDRequired = errors.New("match ID required")
	// ErrAccountKindNotSupported is returned, as an *AccountKindError, for a
	// Transfer leg with a loan or crypto wallet Account on either side
	// (ADR-0026).
	ErrAccountKindNotSupported = errors.New("account kind takes no part in transfers")
)

// transferKinds are the Account kinds that take part in Transfers. Loans and
// crypto wallets wait for their own feature (ADR-0026).
var transferKinds = []AccountKind{AccountKindCash, AccountKindBankAccount, AccountKindCreditCard}

// TakesPartInTransfers reports whether an Account of the kind can be either
// side of a Transfer: cash, bank and credit card Accounts can.
func TakesPartInTransfers(kind AccountKind) bool { return slices.Contains(transferKinds, kind) }

// AccountKindError is a Transfer leg refused for the kind of one of its two
// Accounts: AccountID is that Account, Kind its kind.
type AccountKindError struct {
	AccountID sharedkernel.ID
	Kind      AccountKind
}

func (e *AccountKindError) Error() string {
	return fmt.Sprintf("%s: account %s is a %s", ErrAccountKindNotSupported, e.AccountID, e.Kind)
}

func (e *AccountKindError) Unwrap() error { return ErrAccountKindNotSupported }

// IsTransferLeg reports whether the Transaction is a Transfer leg: it names
// a counterpart Account.
func (s TransactionState) IsTransferLeg() bool { return s.CounterpartAccountID != sharedkernel.NilID }

// InTransit reports whether the Transaction is a Transfer leg that is
// neither Matched nor reversed.
func (s TransactionState) InTransit() bool {
	return s.IsTransferLeg() && s.MatchID == sharedkernel.NilID && s.ReversedBy == sharedkernel.NilID
}

// Leg is the Transfer leg as a Match checks it: its Account Posting and its
// counterpart Account. id is the Transaction's own ID.
func (s TransactionState) Leg(id sharedkernel.ID) TransferLeg {
	leg := TransferLeg{TransactionID: id, CounterpartAccountID: s.CounterpartAccountID}

	for _, p := range s.Postings {
		if p.AccountID != sharedkernel.NilID {
			leg.AccountID, leg.Amount, leg.Currency = p.AccountID, p.Amount, p.Currency

			break
		}
	}

	return leg
}

type (
	// MatchTransaction links a Transfer leg to the Match MatchID.
	MatchTransaction struct {
		MatchID sharedkernel.ID
	}

	// UnmatchTransaction takes a Transfer leg out of the Match MatchID,
	// which is being removed: the leg is In transit again unless it is
	// reversed.
	UnmatchTransaction struct {
		MatchID sharedkernel.ID
	}
)

func (MatchTransaction) isTransactionCommand()   {}
func (UnmatchTransaction) isTransactionCommand() {}

type (
	// TransactionMatched is recorded when a Transfer leg is linked to a
	// Match.
	TransactionMatched struct {
		MatchID sharedkernel.ID `json:"matchid"`
	}

	// TransactionUnmatched is recorded when a Transfer leg's Match is
	// removed.
	TransactionUnmatched struct {
		MatchID sharedkernel.ID `json:"matchid"`
	}
)

func (TransactionMatched) isTransactionEvent()   {}
func (TransactionUnmatched) isTransactionEvent() {}

// checkLeg refuses a Transfer leg that isn't one Account's Posting against
// a Category, that names its own Account as counterpart, or whose Account or
// counterpart is of a kind that takes no part in Transfers (its own Account
// checked first), and a Transaction created by a Transfer that isn't a leg.
func checkLeg(c RecordTransaction) error {
	if c.CounterpartAccountID == sharedkernel.NilID {
		if c.CreatedByTransfer {
			return fmt.Errorf("%w: it names no counterpart account", ErrNotATransferLeg)
		}

		return nil
	}

	var accounts []sharedkernel.ID

	for _, p := range c.Postings {
		if p.AccountID != sharedkernel.NilID {
			accounts = append(accounts, p.AccountID)
		}
	}

	switch {
	case len(accounts) != 1 || len(c.Postings) != 2: //nolint:mnd // its Account and Account transfer
		return fmt.Errorf("%w: a transfer leg is one account's posting against account transfer", ErrInvalidPosting)
	case accounts[0] == c.CounterpartAccountID:
		return ErrCounterpartIsItself
	case !TakesPartInTransfers(c.AccountKind):
		return &AccountKindError{AccountID: accounts[0], Kind: c.AccountKind}
	case !TakesPartInTransfers(c.CounterpartKind):
		return &AccountKindError{AccountID: c.CounterpartAccountID, Kind: c.CounterpartKind}
	}

	return nil
}

// matchTransaction links a Transfer leg that isn't reversed to a Match,
// once: a second Accept racing for the same leg is refused here, by the
// leg's version check.
func matchTransaction(s TransactionState, c MatchTransaction) ([]TransactionEvent, error) {
	switch {
	case !s.Recorded:
		return nil, ErrTransactionNotRecorded
	case c.MatchID == sharedkernel.NilID:
		return nil, ErrMatchIDRequired
	case !s.IsTransferLeg():
		return nil, ErrNotATransferLeg
	case s.ReversedBy != sharedkernel.NilID:
		return nil, ErrTransactionReversed
	case s.MatchID != sharedkernel.NilID:
		return nil, ErrTransactionAlreadyMatched
	}

	return []TransactionEvent{TransactionMatched(c)}, nil
}

// unmatchTransaction takes a leg out of the Match that links it.
func unmatchTransaction(s TransactionState, c UnmatchTransaction) ([]TransactionEvent, error) {
	switch {
	case !s.Recorded:
		return nil, ErrTransactionNotRecorded
	case c.MatchID == sharedkernel.NilID:
		return nil, ErrMatchIDRequired
	case s.MatchID != c.MatchID:
		return nil, ErrTransactionNotMatched
	}

	return []TransactionEvent{TransactionUnmatched(c)}, nil
}
