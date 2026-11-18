package httpapi

import (
	"context"

	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// CurrencyHandler implements listCurrencies. No bounded context owns the
// currency list, so it lives here and needs no registered User (ADR-0007).
type CurrencyHandler struct{}

// ListCurrencies implements listCurrencies.
func (CurrencyHandler) ListCurrencies(context.Context) (oas.ListCurrenciesRes, error) {
	list := sharedkernel.Currencies()

	items := make([]oas.Currency, 0, len(list))
	for _, c := range list {
		items = append(items, oas.Currency{
			Code:          oas.CurrencyCode(c.Code()),
			DecimalPlaces: int32(c.Decimals()), //nolint:gosec // decimal places are at most 18
		})
	}

	return &oas.CurrencyList{Items: items}, nil
}
