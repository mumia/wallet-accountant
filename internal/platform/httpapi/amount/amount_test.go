package amount_test

import (
	"testing"

	"github.com/go-faster/jx"
	"github.com/shopspring/decimal"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi/amount"
)

// An amount goes out with its currency's exact number of decimal places
// (ADR-0007, spec story 20), trailing zeros included.
func TestAnAmountIsWrittenWithItsCurrencysDecimalPlaces(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		value, currency, want string
	}{
		{"100", "EUR", "100.00"},
		{"-12.5", "EUR", "-12.50"},
		{"0", "EUR", "0.00"},
		{"1500", "JPY", "1500"},
		{"1.2", "BHD", "1.200"},
		{"0.5", "BTC", "0.50000000"},
		{"1", "ETH", "1.000000000000000000"},
	} {
		a, err := amount.Of(decimal.RequireFromString(tc.value), tc.currency)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.value, tc.currency, err)
		}

		var e jx.Encoder
		a.Encode(&e)

		if got := e.String(); got != tc.want {
			t.Errorf("%s %s written as %s, want %s", tc.value, tc.currency, got, tc.want)
		}
	}
}

func TestAnUnknownCurrencyHasNoDecimalPlaces(t *testing.T) {
	t.Parallel()

	if _, err := amount.Of(decimal.NewFromInt(1), "XXX"); err == nil {
		t.Fatal("an amount in an unknown currency was accepted")
	}
}

func TestAnAmountIsReadExactly(t *testing.T) {
	t.Parallel()

	var a amount.Amount
	if err := a.Decode(jx.DecodeStr("1234.50")); err != nil {
		t.Fatal(err)
	}

	if !a.Value.Equal(decimal.RequireFromString("1234.5")) {
		t.Errorf("read %s, want 1234.5", a.Value)
	}

	var e jx.Encoder
	a.Encode(&e)

	if got := e.String(); got != "1234.50" {
		t.Errorf("written back as %s, want 1234.50 as it was read", got)
	}
}
