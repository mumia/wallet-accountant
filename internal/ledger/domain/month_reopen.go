package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrMonthReopenRefused is returned, as a *MonthReopenRefusedError, for a
// month that can't be reopened.
var ErrMonthReopenRefused = errors.New("month reopen refused")

// MonthReopenRefusedError lists every reason a month can't be reopened
// (spec story 8). A false flag or a zero count is a condition that holds.
type MonthReopenRefusedError struct {
	// Month is the month asked to reopen, and OpenMonth the Account's Open
	// month (MonthLayout).
	Month     string
	OpenMonth string
	// NotClosed says Month isn't closed.
	NotClosed bool
	// NotLatestClosed says Month is closed, but isn't the latest closed
	// month, LatestClosed.
	NotLatestClosed bool
	LatestClosed    string
	// Accepted counts the Open month's Transactions that aren't reversed.
	Accepted int
}

func (e *MonthReopenRefusedError) Error() string {
	var reasons []string

	if e.NotClosed {
		reasons = append(reasons, e.Month+" isn't closed")
	}

	if e.NotLatestClosed {
		reasons = append(reasons, "the latest closed month is "+e.LatestClosed)
	}

	if e.Accepted > 0 {
		reasons = append(reasons, fmt.Sprintf("open month %s has %d accepted transactions", e.OpenMonth, e.Accepted))
	}

	return fmt.Sprintf("%s: %s", ErrMonthReopenRefused, strings.Join(reasons, "; "))
}

func (e *MonthReopenRefusedError) Unwrap() error { return ErrMonthReopenRefused }

// ReopenMonth takes back the Month close of the Booking month Month (any
// day in it).
type ReopenMonth struct {
	Month time.Time
}

func (ReopenMonth) isAccountCommand() {}

// MonthReopened is recorded when a closed Booking month (MonthLayout) is
// reopened: its Month close is taken back, and it is the Open month again.
type MonthReopened struct {
	Month string `json:"month"`
}

func (MonthReopened) isAccountEvent() {}

// LatestClosed is the Account's latest closed month (MonthLayout), the one
// just before the Open month; empty while no month is closed.
func (s AccountState) LatestClosed() string {
	if len(s.Closed) == 0 {
		return ""
	}

	return PreviousMonth(s.OpenMonth)
}

// ReopenRefusal is every reason the Booking month (MonthLayout) can't be
// reopened now; nil when it can. Only the latest closed month can, while
// the Open month has no Transactions that aren't reversed (ADR-0024). A
// cash Account skips that last condition: it books in later months anyway,
// and the month closes again with a new Cash count.
func (s AccountState) ReopenRefusal(month string) *MonthReopenRefusedError {
	_, closed := s.Closed[month]

	accepted := s.Live[s.OpenMonth]
	if s.IsCash() {
		accepted = 0
	}

	refused := &MonthReopenRefusedError{
		Month:           month,
		OpenMonth:       s.OpenMonth,
		NotClosed:       !closed,
		NotLatestClosed: closed && month != s.LatestClosed(),
		LatestClosed:    s.LatestClosed(),
		Accepted:        accepted,
	}

	if refused.NotClosed || refused.NotLatestClosed || refused.Accepted > 0 {
		return refused
	}

	return nil
}

// reopenMonth takes back the latest Month close, refusing with every
// reason that fails together. The month it reopens is the Open month
// again, and the month after it is not open yet.
func reopenMonth(s AccountState, c ReopenMonth) ([]AccountEvent, error) {
	if !s.Added {
		return nil, ErrAccountNotAdded
	}

	month := c.Month.Format(MonthLayout)

	if refused := s.ReopenRefusal(month); refused != nil {
		return nil, refused
	}

	return []AccountEvent{MonthReopened{Month: month}}, nil
}

// reopened returns a copy of closes without month, so a fold never shares
// its map with an earlier state.
func reopened(closes map[string]MonthClose, month string) map[string]MonthClose {
	out := make(map[string]MonthClose, len(closes))
	for m, v := range closes {
		if m != month {
			out[m] = v
		}
	}

	return out
}
