package domain

import (
	"errors"
	"fmt"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// ErrCategoryDirection is returned, as a *DirectionError, for choosing an
// Expense Category for money in or an Income Category for money out.
var ErrCategoryDirection = errors.New("the category's kind doesn't fit the amount's direction")

// Direction is which way an amount moves on an Account.
type Direction string

// The Directions.
const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

// DirectionError is a Category refused because its kind doesn't fit the
// direction of the amount.
type DirectionError struct {
	Kind      Kind
	Direction Direction
}

func (e *DirectionError) Error() string {
	return fmt.Sprintf("%s: an %s category on money %s", ErrCategoryDirection, e.Kind, e.Direction)
}

func (e *DirectionError) Unwrap() error { return ErrCategoryDirection }

// FitsDirection refuses, with a *DirectionError, a Category kind chosen for
// an amount moving the other way: an Expense Category takes only money out
// and an Income Category only money in. amount is the Account Posting's,
// positive for money in; its Category Posting carries the opposite sign.
// Internal Categories, such as Uncategorised, and a zero amount, which has
// no direction, take either.
func FitsDirection(kind Kind, amount sharedkernel.Decimal) error {
	switch {
	case kind == KindExpense && amount.IsPositive():
		return &DirectionError{Kind: kind, Direction: DirectionIn}
	case kind == KindIncome && amount.IsNegative():
		return &DirectionError{Kind: kind, Direction: DirectionOut}
	default:
		return nil
	}
}
