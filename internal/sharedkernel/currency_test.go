package sharedkernel_test

import (
	"errors"
	"testing"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

func TestCurrenciesKnowTheirDecimalPlaces(t *testing.T) {
	t.Parallel()

	// ADR-0007's examples, and a three-decimal currency.
	for code, want := range map[string]int{"EUR": 2, "JPY": 0, "BHD": 3, "BTC": 8, "ETH": 18} {
		c, err := sharedkernel.ParseCurrency(code)
		if err != nil {
			t.Errorf("ParseCurrency(%q): %v", code, err)

			continue
		}

		if c.Code() != code || c.Decimals() != want {
			t.Errorf("ParseCurrency(%q) = %s with %d decimals, want %s with %d", code, c.Code(), c.Decimals(), code, want)
		}
	}
}

func TestUnknownCurrenciesAreRefused(t *testing.T) {
	t.Parallel()

	for _, code := range []string{"", "XYZ", "eur", "EURO", " EUR"} {
		if _, err := sharedkernel.ParseCurrency(code); !errors.Is(err, sharedkernel.ErrUnknownCurrency) {
			t.Errorf("ParseCurrency(%q): err = %v, want ErrUnknownCurrency", code, err)
		}
	}
}

func TestCurrenciesListsEveryKnownCurrencySortedByCode(t *testing.T) {
	t.Parallel()

	list := sharedkernel.Currencies()
	if len(list) == 0 {
		t.Fatal("Currencies() is empty")
	}

	for i, c := range list {
		if i > 0 && list[i-1].Code() >= c.Code() {
			t.Errorf("Currencies() not sorted: %s before %s", list[i-1].Code(), c.Code())
		}

		parsed, err := sharedkernel.ParseCurrency(c.Code())
		if err != nil || parsed != c {
			t.Errorf("ParseCurrency(%q) = %v, %v; want %v", c.Code(), parsed, err, c)
		}
	}
}
