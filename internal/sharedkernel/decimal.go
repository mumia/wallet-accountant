package sharedkernel

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Decimal is an exact decimal number, for amounts and rates (ADR-0007);
// never a binary floating-point value. It is an alias, so values pass freely
// to adapters that use shopspring/decimal.
type Decimal = decimal.Decimal

// ParseDecimal parses a plain decimal such as "-1234.50".
func ParseDecimal(s string) (Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Decimal{}, fmt.Errorf("parse decimal %q: %w", s, err)
	}

	return d, nil
}
