package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider"
)

var (
	// ErrTransactionAlreadyRecorded is returned for recording a Transaction
	// twice.
	ErrTransactionAlreadyRecorded = errors.New("transaction already recorded")
	// ErrTooFewPostings is returned for a Transaction with fewer than two
	// Postings.
	ErrTooFewPostings = errors.New("a transaction needs at least two postings")
	// ErrInvalidPosting is returned for a Posting that doesn't name exactly
	// one Account or one Category, or whose amount isn't money in a known
	// currency with that currency's decimal places.
	ErrInvalidPosting = errors.New("invalid posting")
	// ErrUnbalancedPostings is returned for Postings that don't sum to zero
	// in every currency.
	ErrUnbalancedPostings = errors.New("postings don't sum to zero per currency")
	// ErrBookingDateRequired is returned for a Transaction without a Booking
	// date.
	ErrBookingDateRequired = errors.New("booking date required")
	// ErrTransactionNotRecorded is returned for a command to a Transaction
	// that doesn't exist.
	ErrTransactionNotRecorded = errors.New("transaction not recorded")
	// ErrOpeningBalanceTransaction is returned for reversing or
	// recategorising the Opening balance Transaction: only voiding the first
	// Statement import changes it (ADR-0018).
	ErrOpeningBalanceTransaction = errors.New("the opening balance transaction can't be changed")
	// ErrTransactionIsAReversal is returned for reversing or recategorising
	// a reversal, so corrections can't pile up.
	ErrTransactionIsAReversal = errors.New("transaction is a reversal")
	// ErrTransactionAlreadyReversed is returned for reversing a Transaction
	// twice.
	ErrTransactionAlreadyReversed = errors.New("transaction already reversed")
	// ErrTransactionReversed is returned for recategorising a reversed
	// Transaction.
	ErrTransactionReversed = errors.New("transaction is reversed")
	// ErrReversalIDRequired is returned for reversing without the
	// Transaction that reverses it.
	ErrReversalIDRequired = errors.New("reversing transaction ID required")
	// ErrCategoryRequired is returned for setting no Category.
	ErrCategoryRequired = errors.New("category required")
	// ErrNotOneCategory is returned for setting the Category of a
	// Transaction whose non-Account side isn't exactly one Category.
	ErrNotOneCategory = errors.New("the transaction's other side isn't exactly one category")
	// ErrNotTheOpeningBalanceTransaction is returned for reversing, as the
	// Opening balance, a Transaction that isn't the Account's Opening
	// balance Transaction.
	ErrNotTheOpeningBalanceTransaction = errors.New("not the opening balance transaction")
)

// Posting is one line of a Transaction: the amount it moves into (positive)
// or out of (negative) exactly one Account or one Category.
type Posting struct {
	AccountID  sharedkernel.ID      `json:"accountid,omitzero"`
	CategoryID sharedkernel.ID      `json:"categoryid,omitzero"`
	Amount     sharedkernel.Decimal `json:"amount"`
	Currency   string               `json:"currency"`
}

// TransactionState is a Transaction's fold.
type TransactionState struct {
	Recorded        bool
	OpeningBalance  bool
	BookingDate     time.Time
	TransactionDate time.Time
	// ValueDate is zero when unknown.
	ValueDate   time.Time
	Description string
	Payee       string
	Postings    []Posting
	// ImportedTransactionID is the Imported transaction whose Accept
	// recorded it; the zero ID otherwise.
	ImportedTransactionID sharedkernel.ID
	// ReversalOf is the Transaction this one reverses; the zero ID unless it
	// is a reversal.
	ReversalOf sharedkernel.ID
	// ReversedBy is the Transaction that reversed this one; the zero ID
	// while it isn't reversed.
	ReversedBy sharedkernel.ID
	// CounterpartAccountID is the other Account of the Transfer this
	// Transaction is a leg of; the zero ID unless it is a Transfer leg.
	CounterpartAccountID sharedkernel.ID
	// CreatedByTransfer says an Accept as Transfer recorded this cash leg
	// on its own.
	CreatedByTransfer bool
	// MatchID is the Match linking this leg to the other one; the zero ID
	// while it is In transit.
	MatchID sharedkernel.ID
}

// CategoryID returns the Category of the non-Account side, or the zero ID
// when that side isn't exactly one Category.
func (s TransactionState) CategoryID() sharedkernel.ID {
	category, n := sharedkernel.NilID, 0

	for _, p := range s.Postings {
		if p.CategoryID != sharedkernel.NilID {
			category = p.CategoryID
			n++
		}
	}

	if n != 1 {
		return sharedkernel.NilID
	}

	return category
}

// Reversal returns the command that records the reversal of this
// Transaction, id: the opposite Postings on the same dates, with the same
// description and payee, linked to it (ADR-0018).
func (s TransactionState) Reversal(id sharedkernel.ID) RecordTransaction {
	postings := make([]Posting, 0, len(s.Postings))
	for _, p := range s.Postings {
		p.Amount = p.Amount.Neg()
		postings = append(postings, p)
	}

	return RecordTransaction{
		ReversalOf:      id,
		BookingDate:     s.BookingDate,
		TransactionDate: s.TransactionDate,
		ValueDate:       s.ValueDate,
		Description:     s.Description,
		Payee:           s.Payee,
		Postings:        postings,
	}
}

// AccountLine is a Transaction as one line of an Account's single-entry
// list: one Account Posting with the Category on the other side (ADR-0001).
// CategoryID is the zero ID when the other side isn't exactly one Category.
type AccountLine struct {
	AccountID       sharedkernel.ID
	Ordinal         int
	BookingDate     time.Time
	TransactionDate time.Time
	Description     string
	Amount          sharedkernel.Decimal
	Currency        string
	CategoryID      sharedkernel.ID
}

// AccountLines returns one line per Account Posting, in Posting order. Its
// fold stores them as lookup rows (ADR-0011 "Reads").
func (s TransactionState) AccountLines() []AccountLine {
	category := s.CategoryID()

	var lines []AccountLine

	for i, p := range s.Postings {
		if p.AccountID == sharedkernel.NilID {
			continue
		}

		lines = append(lines, AccountLine{
			AccountID: p.AccountID, Ordinal: i, BookingDate: s.BookingDate, TransactionDate: s.TransactionDate,
			Description: s.Description, Amount: p.Amount, Currency: p.Currency, CategoryID: category,
		})
	}

	return lines
}

type (
	// TransactionCommand is a command to a Transaction.
	TransactionCommand interface{ isTransactionCommand() }

	// RecordTransaction records a balanced Transaction. TransactionDate
	// defaults to the Booking date; ValueDate may be zero.
	// ImportedTransactionID links it to the Imported transaction it books;
	// ReversalOf to the Transaction it reverses. CounterpartAccountID makes
	// it a Transfer leg, and CreatedByTransfer says an Accept as Transfer
	// recorded it. A leg gives the kinds of its Account and of its
	// counterpart, AccountKind and CounterpartKind, which aren't recorded.
	RecordTransaction struct {
		ImportedTransactionID sharedkernel.ID
		ReversalOf            sharedkernel.ID
		CounterpartAccountID  sharedkernel.ID
		AccountKind           AccountKind
		CounterpartKind       AccountKind
		CreatedByTransfer     bool
		OpeningBalance        bool
		BookingDate           time.Time
		TransactionDate       time.Time
		ValueDate             time.Time
		Description           string
		Payee                 string
		Postings              []Posting
	}

	// ReverseTransaction reverses a Transaction: ReversalID is the
	// Transaction that records the opposite Postings.
	ReverseTransaction struct {
		ReversalID sharedkernel.ID
	}

	// ReverseOpeningBalance reverses the Opening balance Transaction when
	// its Account's first Statement import is voided (ADR-0018): ReversalID
	// is the Transaction that records the opposite Postings.
	ReverseOpeningBalance struct {
		ReversalID sharedkernel.ID
	}

	// SetTransactionCategory sets the Category of the non-Account side.
	// Kind is the chosen Category's, which must fit the direction of the
	// Account Posting's amount.
	SetTransactionCategory struct {
		CategoryID sharedkernel.ID
		Kind       Kind
	}
)

func (RecordTransaction) isTransactionCommand()      {}
func (ReverseTransaction) isTransactionCommand()     {}
func (ReverseOpeningBalance) isTransactionCommand()  {}
func (SetTransactionCategory) isTransactionCommand() {}

type (
	// TransactionEvent is a Transaction's event.
	TransactionEvent interface{ isTransactionEvent() }

	// TransactionRecorded is recorded when a Transaction is recorded. Dates
	// are written with DateLayout; ValueDate is empty when unknown.
	TransactionRecorded struct {
		ImportedTransactionID sharedkernel.ID `json:"importedtransactionid,omitzero"`
		ReversalOf            sharedkernel.ID `json:"reversalof,omitzero"`
		CounterpartAccountID  sharedkernel.ID `json:"counterpartaccountid,omitzero"`
		CreatedByTransfer     bool            `json:"createdbytransfer,omitempty"`
		OpeningBalance        bool            `json:"openingbalance,omitempty"`
		BookingDate           string          `json:"bookingdate"`
		TransactionDate       string          `json:"transactiondate"`
		ValueDate             string          `json:"valuedate,omitempty"`
		Description           string          `json:"description"`
		Payee                 string          `json:"payee,omitempty"`
		Postings              []Posting       `json:"postings"`
	}

	// TransactionReversed is recorded when a Transaction is reversed by the
	// Transaction ReversalID.
	TransactionReversed struct {
		ReversalID sharedkernel.ID `json:"reversalid"`
	}

	// TransactionCategorySet is recorded when the Category of the
	// non-Account side changes from PreviousCategoryID to CategoryID.
	TransactionCategorySet struct {
		CategoryID         sharedkernel.ID `json:"categoryid"`
		PreviousCategoryID sharedkernel.ID `json:"previouscategoryid"`
	}
)

func (TransactionRecorded) isTransactionEvent()    {}
func (TransactionReversed) isTransactionEvent()    {}
func (TransactionCategorySet) isTransactionEvent() {}

// Transaction is the Transaction Decider (ADR-0001, ADR-0011): its Postings
// always sum to zero per currency.
var Transaction = decider.Decider[TransactionState, TransactionCommand, TransactionEvent]{
	Decide: decideTransaction,
	Evolve: evolveTransaction,
}

func decideTransaction(s TransactionState, c TransactionCommand) ([]TransactionEvent, error) {
	switch c := c.(type) {
	case RecordTransaction:
		return recordTransaction(s, c)
	case ReverseTransaction:
		return reverseTransaction(s, c)
	case ReverseOpeningBalance:
		return reverseOpeningBalance(s, c)
	case SetTransactionCategory:
		return setTransactionCategory(s, c)
	case MatchTransaction:
		return matchTransaction(s, c)
	case UnmatchTransaction:
		return unmatchTransaction(s, c)
	default:
		return nil, nil
	}
}

func recordTransaction(s TransactionState, c RecordTransaction) ([]TransactionEvent, error) {
	if s.Recorded {
		return nil, ErrTransactionAlreadyRecorded
	}

	if c.BookingDate.IsZero() {
		return nil, ErrBookingDateRequired
	}

	if len(c.Postings) < 2 { //nolint:mnd // a Transaction has two sides
		return nil, ErrTooFewPostings
	}

	sums := map[string]sharedkernel.Decimal{}

	for i, p := range c.Postings {
		if err := checkPosting(p); err != nil {
			return nil, fmt.Errorf("posting %d: %w", i+1, err)
		}

		sums[p.Currency] = sums[p.Currency].Add(p.Amount)
	}

	for currency, sum := range sums {
		if !sum.IsZero() {
			return nil, fmt.Errorf("%w: %s %s", ErrUnbalancedPostings, sum, currency)
		}
	}

	if err := checkLeg(c); err != nil {
		return nil, err
	}

	transactionDate := c.TransactionDate
	if transactionDate.IsZero() {
		transactionDate = c.BookingDate
	}

	recorded := TransactionRecorded{
		ImportedTransactionID: c.ImportedTransactionID,
		ReversalOf:            c.ReversalOf,
		CounterpartAccountID:  c.CounterpartAccountID,
		CreatedByTransfer:     c.CreatedByTransfer,
		OpeningBalance:        c.OpeningBalance,
		BookingDate:           c.BookingDate.Format(DateLayout),
		TransactionDate:       transactionDate.Format(DateLayout),
		Description:           c.Description,
		Payee:                 c.Payee,
		Postings:              c.Postings,
	}

	if !c.ValueDate.IsZero() {
		recorded.ValueDate = c.ValueDate.Format(DateLayout)
	}

	return []TransactionEvent{recorded}, nil
}

// correctable refuses changing a Transaction that only voiding changes, or
// that is itself a correction (ADR-0018).
func correctable(s TransactionState) error {
	switch {
	case !s.Recorded:
		return ErrTransactionNotRecorded
	case s.OpeningBalance:
		return ErrOpeningBalanceTransaction
	case s.ReversalOf != sharedkernel.NilID:
		return ErrTransactionIsAReversal
	default:
		return nil
	}
}

// reverseTransaction reverses a Transaction once (spec story 51f).
func reverseTransaction(s TransactionState, c ReverseTransaction) ([]TransactionEvent, error) {
	if err := correctable(s); err != nil {
		return nil, err
	}

	switch {
	case s.ReversedBy != sharedkernel.NilID:
		return nil, ErrTransactionAlreadyReversed
	case c.ReversalID == sharedkernel.NilID:
		return nil, ErrReversalIDRequired
	case s.MatchID != sharedkernel.NilID:
		return nil, ErrTransactionMatched
	}

	return []TransactionEvent{TransactionReversed(c)}, nil
}

// reverseOpeningBalance reverses the Opening balance Transaction once: only
// voiding its Account's first Statement import asks for it.
func reverseOpeningBalance(s TransactionState, c ReverseOpeningBalance) ([]TransactionEvent, error) {
	switch {
	case !s.Recorded:
		return nil, ErrTransactionNotRecorded
	case !s.OpeningBalance:
		return nil, ErrNotTheOpeningBalanceTransaction
	case s.ReversedBy != sharedkernel.NilID:
		return nil, ErrTransactionAlreadyReversed
	case c.ReversalID == sharedkernel.NilID:
		return nil, ErrReversalIDRequired
	}

	return []TransactionEvent{TransactionReversed(c)}, nil
}

// setTransactionCategory changes the Category of the non-Account side; the
// Category it already has changes nothing. A Transaction that can't take
// any Category says so before the chosen kind is checked against the
// direction of its Account Posting.
func setTransactionCategory(s TransactionState, c SetTransactionCategory) ([]TransactionEvent, error) {
	if err := correctable(s); err != nil {
		return nil, err
	}

	previous := s.CategoryID()

	switch {
	case s.IsTransferLeg():
		return nil, ErrTransferLeg
	case s.ReversedBy != sharedkernel.NilID:
		return nil, ErrTransactionReversed
	case c.CategoryID == sharedkernel.NilID:
		return nil, ErrCategoryRequired
	case previous == sharedkernel.NilID:
		return nil, ErrNotOneCategory
	}

	if err := FitsDirection(c.Kind, s.accountAmount()); err != nil {
		return nil, err
	}

	if previous == c.CategoryID {
		return nil, nil
	}

	return []TransactionEvent{TransactionCategorySet{CategoryID: c.CategoryID, PreviousCategoryID: previous}}, nil
}

// accountAmount is the sum of the Account Postings: the amount the
// Transaction moves into (positive) or out of (negative) its Account.
func (s TransactionState) accountAmount() sharedkernel.Decimal {
	var sum sharedkernel.Decimal

	for _, p := range s.Postings {
		if p.AccountID != sharedkernel.NilID {
			sum = sum.Add(p.Amount)
		}
	}

	return sum
}

func checkPosting(p Posting) error {
	if (p.AccountID == sharedkernel.NilID) == (p.CategoryID == sharedkernel.NilID) {
		return fmt.Errorf("%w: it must name exactly one Account or one Category", ErrInvalidPosting)
	}

	currency, err := sharedkernel.ParseCurrency(p.Currency)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPosting, err)
	}

	if !p.Amount.Equal(p.Amount.Round(int32(currency.Decimals()))) { //nolint:gosec // at most 18 places
		return fmt.Errorf("%w: %s has more than the %d decimal places of %s",
			ErrInvalidPosting, p.Amount, currency.Decimals(), currency.Code())
	}

	return nil
}

func evolveTransaction(s TransactionState, e decider.Envelope[TransactionEvent]) TransactionState {
	switch ev := e.Event.(type) {
	case TransactionReversed:
		s.ReversedBy = ev.ReversalID
	case TransactionMatched:
		s.MatchID = ev.MatchID
	case TransactionUnmatched:
		s.MatchID = sharedkernel.NilID
	case TransactionCategorySet:
		postings := make([]Posting, len(s.Postings))
		for i, p := range s.Postings {
			if p.CategoryID != sharedkernel.NilID {
				p.CategoryID = ev.CategoryID
			}

			postings[i] = p
		}

		s.Postings = postings
	case TransactionRecorded:
		s.Recorded, s.OpeningBalance = true, ev.OpeningBalance
		// The event was written with DateLayout, so the dates always parse.
		s.BookingDate, _ = time.Parse(DateLayout, ev.BookingDate)
		s.TransactionDate, _ = time.Parse(DateLayout, ev.TransactionDate)
		s.ValueDate = time.Time{}

		if ev.ValueDate != "" {
			s.ValueDate, _ = time.Parse(DateLayout, ev.ValueDate)
		}

		s.Description, s.Payee, s.Postings = ev.Description, ev.Payee, ev.Postings
		s.ImportedTransactionID, s.ReversalOf = ev.ImportedTransactionID, ev.ReversalOf
		s.CounterpartAccountID, s.CreatedByTransfer = ev.CounterpartAccountID, ev.CreatedByTransfer
	}

	return s
}
