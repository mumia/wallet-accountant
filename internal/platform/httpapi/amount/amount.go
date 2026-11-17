// Package amount is how the API writes and reads an amount of money: an
// exact decimal with its currency's number of decimal places (ADR-0007).
// The contract names Amount as the Go type of every amount (x-ogen-type), so
// ogen writes "100.00" for EUR 100, where decimal.Decimal would write "100".
package amount

import (
	"fmt"

	"github.com/go-faster/jx"
	"github.com/shopspring/decimal"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// Amount is an exact decimal written with Places decimal places.
type Amount struct {
	Value  decimal.Decimal
	Places int32
}

// Of returns value as an amount in the currency, written with the
// currency's decimal places.
func Of(value decimal.Decimal, currency string) (Amount, error) {
	c, err := sharedkernel.ParseCurrency(currency)
	if err != nil {
		return Amount{}, fmt.Errorf("amount: %w", err)
	}

	return Amount{Value: value, Places: int32(c.Decimals())}, nil //nolint:gosec // at most 18 places
}

// Encode writes the amount as a JSON number with exactly Places decimal
// places. It never rounds an amount that already fits them.
func (a *Amount) Encode(e *jx.Encoder) {
	e.Num(jx.Num(a.Value.StringFixed(a.Places)))
}

// Decode reads a JSON number exactly, keeping the decimal places it was
// written with.
func (a *Amount) Decode(d *jx.Decoder) error {
	n, err := d.Num()
	if err != nil {
		return fmt.Errorf("read amount: %w", err)
	}

	v, err := decimal.NewFromString(n.String())
	if err != nil {
		return fmt.Errorf("read amount: %w", err)
	}

	a.Value, a.Places = v, max(0, -v.Exponent())

	return nil
}
