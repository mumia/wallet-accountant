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
	category := sharedkernel.NilID
	categories := 0

	for _, p := range s.Postings {
		if p.CategoryID != sharedkernel.NilID {
			category = p.CategoryID
			categories++
		}
	}

	if categories != 1 {
		category = sharedkernel.NilID
	}

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
	RecordTransaction struct {
		OpeningBalance  bool
		BookingDate     time.Time
		TransactionDate time.Time
		ValueDate       time.Time
		Description     string
		Payee           string
		Postings        []Posting
	}
)

func (RecordTransaction) isTransactionCommand() {}

type (
	// TransactionEvent is a Transaction's event.
	TransactionEvent interface{ isTransactionEvent() }

	// TransactionRecorded is recorded when a Transaction is recorded. Dates
	// are written with DateLayout; ValueDate is empty when unknown.
	TransactionRecorded struct {
		OpeningBalance  bool      `json:"openingbalance,omitempty"`
		BookingDate     string    `json:"bookingdate"`
		TransactionDate string    `json:"transactiondate"`
		ValueDate       string    `json:"valuedate,omitempty"`
		Description     string    `json:"description"`
		Payee           string    `json:"payee,omitempty"`
		Postings        []Posting `json:"postings"`
	}
)

func (TransactionRecorded) isTransactionEvent() {}

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

	transactionDate := c.TransactionDate
	if transactionDate.IsZero() {
		transactionDate = c.BookingDate
	}

	recorded := TransactionRecorded{
		OpeningBalance:  c.OpeningBalance,
		BookingDate:     c.BookingDate.Format(DateLayout),
		TransactionDate: transactionDate.Format(DateLayout),
		Description:     c.Description,
		Payee:           c.Payee,
		Postings:        c.Postings,
	}

	if !c.ValueDate.IsZero() {
		recorded.ValueDate = c.ValueDate.Format(DateLayout)
	}

	return []TransactionEvent{recorded}, nil
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
	if ev, ok := e.Event.(TransactionRecorded); ok {
		s.Recorded, s.OpeningBalance = true, ev.OpeningBalance
		// The event was written with DateLayout, so the dates always parse.
		s.BookingDate, _ = time.Parse(DateLayout, ev.BookingDate)
		s.TransactionDate, _ = time.Parse(DateLayout, ev.TransactionDate)
		s.ValueDate = time.Time{}

		if ev.ValueDate != "" {
			s.ValueDate, _ = time.Parse(DateLayout, ev.ValueDate)
		}

		s.Description, s.Payee, s.Postings = ev.Description, ev.Payee, ev.Postings
	}

	return s
}
