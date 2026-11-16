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
	// ErrAccountNotAdded is returned for a command to an Account that
	// doesn't exist.
	ErrAccountNotAdded = errors.New("account not added")
	// ErrOpeningBalanceAlreadyRecorded is returned for recording an
	// Account's Opening balance a second time.
	ErrOpeningBalanceAlreadyRecorded = errors.New("opening balance already recorded")
	// ErrTooManyDecimalPlaces is returned for an amount with more decimal
	// places than the Account currency has (ADR-0007).
	ErrTooManyDecimalPlaces = errors.New("too many decimal places for the account currency")
	// ErrAwaitingFirstStatementImport is returned for booking a Transaction
	// on an Account that has no Opening balance yet.
	ErrAwaitingFirstStatementImport = errors.New("account awaits its first statement import")
	// ErrBookedBeforeStartDate is returned for booking a Transaction before
	// the Account's start date, where its history doesn't reach.
	ErrBookedBeforeStartDate = errors.New("booked before the account's start date")
)

// MonthLayout is how a Booking month is written.
const MonthLayout = "2006-01"

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
	// OpeningBalanceTransactionID is the Transaction that recorded the
	// Opening balance; the zero ID while the Account awaits it.
	OpeningBalanceTransactionID sharedkernel.ID
	// Months is the running balance per Booking month (MonthLayout): how
	// much the month's Transactions moved the balance.
	Months map[string]sharedkernel.Decimal
	// Uncategorised counts, per Booking month, the Transactions whose other
	// side is in Uncategorised. A month with none is absent.
	Uncategorised map[string]int
}

// HasBalance reports whether the Account has a balance: only once its
// Opening balance is recorded.
func (s AccountState) HasBalance() bool { return !s.AwaitingFirstStatementImport && s.Added }

// Balance returns the current balance: the sum of every Booking month.
func (s AccountState) Balance() sharedkernel.Decimal {
	var sum sharedkernel.Decimal
	for _, m := range s.Months {
		sum = sum.Add(m)
	}

	return sum
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

	// RecordOpeningBalance records the Opening balance the first Statement
	// import worked out, booked on the start date by the Transaction
	// TransactionID (ADR-0018).
	RecordOpeningBalance struct {
		TransactionID sharedkernel.ID
		Amount        sharedkernel.Decimal
	}

	// BookTransaction moves the Account's balance by a Transaction's
	// Posting on it, in the Transaction's Booking month. Uncategorised says
	// its other side is in Uncategorised.
	BookTransaction struct {
		TransactionID sharedkernel.ID
		BookingDate   time.Time
		Amount        sharedkernel.Decimal
		Uncategorised bool
	}
)

func (AddAccount) isAccountCommand()           {}
func (RecordOpeningBalance) isAccountCommand() {}
func (BookTransaction) isAccountCommand()      {}

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

	// OpeningBalanceRecorded is recorded when the first Statement import
	// records the Account's Opening balance, on its start date.
	OpeningBalanceRecorded struct {
		TransactionID sharedkernel.ID      `json:"transactionid"`
		Amount        sharedkernel.Decimal `json:"amount"`
	}

	// TransactionBooked is recorded when a Transaction moves the Account's
	// balance. BookingDate is written with DateLayout.
	TransactionBooked struct {
		TransactionID sharedkernel.ID      `json:"transactionid"`
		BookingDate   string               `json:"bookingdate"`
		Amount        sharedkernel.Decimal `json:"amount"`
		Uncategorised bool                 `json:"uncategorised,omitempty"`
	}
)

func (AccountAdded) isAccountEvent()           {}
func (OpeningBalanceRecorded) isAccountEvent() {}
func (TransactionBooked) isAccountEvent()      {}

// Account is the Account Decider (ADR-0004, ADR-0011).
var Account = decider.Decider[AccountState, AccountCommand, AccountEvent]{
	Decide: decideAccount,
	Evolve: evolveAccount,
}

func decideAccount(s AccountState, c AccountCommand) ([]AccountEvent, error) {
	switch c := c.(type) {
	case AddAccount:
		return addAccount(s, c)
	case RecordOpeningBalance:
		return recordOpeningBalance(s, c)
	case BookTransaction:
		return bookTransaction(s, c)
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

// recordOpeningBalance accepts the Opening balance once: only the first
// Statement import records one, and a second confirm of a stale preview is
// refused here even when it raced the first (ADR-0018).
func recordOpeningBalance(s AccountState, c RecordOpeningBalance) ([]AccountEvent, error) {
	switch {
	case !s.Added:
		return nil, ErrAccountNotAdded
	case !s.AwaitingFirstStatementImport:
		return nil, ErrOpeningBalanceAlreadyRecorded
	}

	currency, err := sharedkernel.ParseCurrency(s.Currency)
	if err != nil {
		return nil, err //nolint:wrapcheck // the shared kernel's own refusal
	}

	if !c.Amount.Equal(c.Amount.Round(int32(currency.Decimals()))) { //nolint:gosec // at most 18 places
		return nil, fmt.Errorf("%w: %s %s", ErrTooManyDecimalPlaces, c.Amount, currency.Code())
	}

	return []AccountEvent{OpeningBalanceRecorded(c)}, nil
}

// bookTransaction moves the balance of an Account that has its Opening
// balance, on or after its start date.
func bookTransaction(s AccountState, c BookTransaction) ([]AccountEvent, error) {
	switch {
	case !s.Added:
		return nil, ErrAccountNotAdded
	case s.AwaitingFirstStatementImport:
		return nil, ErrAwaitingFirstStatementImport
	case c.BookingDate.IsZero():
		return nil, ErrBookingDateRequired
	case c.BookingDate.Before(s.StartDate):
		return nil, fmt.Errorf("%w: %s is before %s", ErrBookedBeforeStartDate,
			c.BookingDate.Format(DateLayout), s.StartDate.Format(DateLayout))
	}

	currency, err := sharedkernel.ParseCurrency(s.Currency)
	if err != nil {
		return nil, err //nolint:wrapcheck // the shared kernel's own refusal
	}

	if !c.Amount.Equal(c.Amount.Round(int32(currency.Decimals()))) { //nolint:gosec // at most 18 places
		return nil, fmt.Errorf("%w: %s %s", ErrTooManyDecimalPlaces, c.Amount, currency.Code())
	}

	return []AccountEvent{TransactionBooked{
		TransactionID: c.TransactionID,
		BookingDate:   c.BookingDate.Format(DateLayout),
		Amount:        c.Amount,
		Uncategorised: c.Uncategorised,
	}}, nil
}

func evolveAccount(s AccountState, e decider.Envelope[AccountEvent]) AccountState {
	switch ev := e.Event.(type) {
	case AccountAdded:
		s.Added = true
		s.Name, s.Kind, s.Currency = ev.Name, ev.Kind, ev.Currency
		// The event was written with DateLayout, so it always parses.
		s.StartDate, _ = time.Parse(DateLayout, ev.StartDate)
		s.AwaitingFirstStatementImport = true
	case OpeningBalanceRecorded:
		s.AwaitingFirstStatementImport = false
		s.OpeningBalanceTransactionID = ev.TransactionID
		s.Months = moved(s.Months, s.StartDate.Format(MonthLayout), ev.Amount)
	case TransactionBooked:
		// The event was written with DateLayout, so it always parses.
		date, _ := time.Parse(DateLayout, ev.BookingDate)
		month := date.Format(MonthLayout)
		s.Months = moved(s.Months, month, ev.Amount)

		if ev.Uncategorised {
			s.Uncategorised = counted(s.Uncategorised, month, 1)
		}
	}

	return s
}

// moved returns a copy of months with month moved by amount, so a fold
// never shares its map with an earlier state.
func moved(months map[string]sharedkernel.Decimal, month string, amount sharedkernel.Decimal) map[string]sharedkernel.Decimal {
	out := make(map[string]sharedkernel.Decimal, len(months)+1)
	for m, v := range months {
		out[m] = v
	}

	out[month] = out[month].Add(amount)

	return out
}

// counted returns a copy of counts with month's count moved by n, dropping a
// month that reaches zero, so a fold never shares its map with an earlier
// state.
func counted(counts map[string]int, month string, n int) map[string]int {
	out := make(map[string]int, len(counts)+1)
	for m, v := range counts {
		out[m] = v
	}

	out[month] += n
	if out[month] == 0 {
		delete(out, month)
	}

	return out
}
