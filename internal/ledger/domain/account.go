package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

// AccountKind is what sort of place an Account is.
type AccountKind string

// The Account kinds this slice supports. Cash Accounts are refused until a
// later slice decides how they get an Opening balance (spec "Out of Scope").
const (
	AccountKindBankAccount  AccountKind = "bank_account"
	AccountKindCreditCard   AccountKind = "credit_card"
	AccountKindLoan         AccountKind = "loan"
	AccountKindCryptoWallet AccountKind = "crypto_wallet"

	accountKindCash AccountKind = "cash"
)

// AccountKinds lists every Account kind that can be added.
func AccountKinds() []AccountKind {
	return []AccountKind{AccountKindBankAccount, AccountKindCreditCard, AccountKindLoan, AccountKindCryptoWallet}
}

// DateLayout is how events write a calendar date.
const DateLayout = time.DateOnly

var (
	// ErrAccountAlreadyAdded is returned for adding an Account twice.
	ErrAccountAlreadyAdded = errors.New("account already added")
	// ErrAccountNameRequired is returned for a blank Account name.
	ErrAccountNameRequired = errors.New("account name required")
	// ErrUnknownAccountKind is returned for a kind that is not an Account kind.
	ErrUnknownAccountKind = errors.New("unknown account kind")
	// ErrCashAccountsNotSupported is returned for adding a cash Account.
	ErrCashAccountsNotSupported = errors.New("cash accounts are not supported yet")
	// ErrStartDateRequired is returned for an Account without a start date.
	ErrStartDateRequired = errors.New("account start date required")
)

// AccountState is an Account's fold. A new Account awaits its first
// Statement import, which records its Opening balance (ADR-0018); until then
// it has no balance.
type AccountState struct {
	Added                        bool
	Name                         string
	Kind                         AccountKind
	Currency                     string
	StartDate                    time.Time
	AwaitingFirstStatementImport bool
}

type (
	// AccountCommand is a command to an Account.
	AccountCommand interface{ isAccountCommand() }

	// AddAccount adds an Account with a name, a kind, an Account currency
	// and a start date. It takes no Opening balance: only the first
	// Statement import records one.
	AddAccount struct {
		Name      string
		Kind      AccountKind
		Currency  string
		StartDate time.Time
	}
)

func (AddAccount) isAccountCommand() {}

type (
	// AccountEvent is an Account's event.
	AccountEvent interface{ isAccountEvent() }

	// AccountAdded is recorded when an Account is added. StartDate is a
	// calendar date written with DateLayout.
	AccountAdded struct {
		Name      string      `json:"name"`
		Kind      AccountKind `json:"kind"`
		Currency  string      `json:"currency"`
		StartDate string      `json:"startdate"`
	}
)

func (AccountAdded) isAccountEvent() {}

// Account is the Account Decider (ADR-0004, ADR-0011).
var Account = decider.Decider[AccountState, AccountCommand, AccountEvent]{
	Decide: decideAccount,
	Evolve: evolveAccount,
}

func decideAccount(s AccountState, c AccountCommand) ([]AccountEvent, error) {
	switch c := c.(type) {
	case AddAccount:
		return addAccount(s, c)
	default:
		return nil, nil
	}
}

func addAccount(s AccountState, c AddAccount) ([]AccountEvent, error) {
	if s.Added {
		return nil, ErrAccountAlreadyAdded
	}

	name := strings.TrimSpace(c.Name)
	if name == "" {
		return nil, ErrAccountNameRequired
	}

	switch {
	case c.Kind == accountKindCash:
		return nil, ErrCashAccountsNotSupported
	case !slices.Contains(AccountKinds(), c.Kind):
		return nil, fmt.Errorf("%w: %q", ErrUnknownAccountKind, c.Kind)
	}

	currency, err := sharedkernel.ParseCurrency(c.Currency)
	if err != nil {
		return nil, err //nolint:wrapcheck // the shared kernel's own refusal
	}

	if c.StartDate.IsZero() {
		return nil, ErrStartDateRequired
	}

	return []AccountEvent{AccountAdded{
		Name:      name,
		Kind:      c.Kind,
		Currency:  currency.Code(),
		StartDate: c.StartDate.Format(DateLayout),
	}}, nil
}

func evolveAccount(s AccountState, e decider.Envelope[AccountEvent]) AccountState {
	if ev, ok := e.Event.(AccountAdded); ok {
		s.Added = true
		s.Name, s.Kind, s.Currency = ev.Name, ev.Kind, ev.Currency
		// The event was written with DateLayout, so it always parses.
		s.StartDate, _ = time.Parse(DateLayout, ev.StartDate)
		s.AwaitingFirstStatementImport = true
	}

	return s
}
