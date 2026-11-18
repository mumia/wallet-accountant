package sharedkernel

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrUnknownCurrency is returned for a currency code Wallet Accountant does
// not know.
var ErrUnknownCurrency = errors.New("unknown currency")

// Currency is a currency Wallet Accountant knows, with the number of decimal
// places its amounts use (ADR-0007).
type Currency struct {
	code     string
	decimals int
}

// Code returns the currency's code, for example EUR.
func (c Currency) Code() string { return c.code }

// Decimals returns how many decimal places the currency's amounts use.
func (c Currency) Decimals() int { return c.decimals }

// ParseCurrency returns the currency with code, which must be written
// exactly as listed (upper case, no spaces).
func ParseCurrency(code string) (Currency, error) {
	decimals, ok := currencies[code]
	if !ok {
		return Currency{}, fmt.Errorf("%w: %q", ErrUnknownCurrency, code)
	}

	return Currency{code: code, decimals: decimals}, nil
}

// Currencies returns every currency ParseCurrency accepts, sorted by code.
func Currencies() []Currency {
	list := make([]Currency, 0, len(currencies))
	for code, decimals := range currencies {
		list = append(list, Currency{code: code, decimals: decimals})
	}

	slices.SortFunc(list, func(a, b Currency) int { return cmp.Compare(a.code, b.code) })

	return list
}

// currencies maps every known code to its decimal places: the ISO 4217
// currencies in circulation, plus the crypto currencies ADR-0007 names.
var currencies = func() map[string]int {
	m := map[string]int{}

	for decimals, codes := range map[int]string{
		0: "BIF CLP DJF GNF ISK JPY KMF KRW PYG RWF UGX UYI VND VUV XAF XOF XPF",
		2: "AED AFN ALL AMD ANG AOA ARS AUD AWG AZN BAM BBD BDT BGN BMD BND BOB BRL BSD BTN BWP BYN BZD " +
			"CAD CDF CHF CNY COP CRC CUP CVE CZK DKK DOP DZD EGP ERN ETB EUR FJD FKP GBP GEL GHS GIP GMD " +
			"GTQ GYD HKD HNL HTG HUF IDR ILS INR IRR JMD KES KGS KHR KPW KYD KZT LAK LBP LKR LRD LSL MAD " +
			"MDL MGA MKD MMK MNT MOP MRU MUR MVR MWK MXN MYR MZN NAD NGN NIO NOK NPR NZD PAB PEN PGK PHP " +
			"PKR PLN QAR RON RSD RUB SAR SBD SCR SDG SEK SGD SHP SLE SOS SRD SSP STN SVC SYP SZL THB TJS " +
			"TMT TOP TRY TTD TWD TZS UAH USD UYU UZS VES WST XCD YER ZAR ZMW ZWG",
		3:  "BHD IQD JOD KWD LYD OMR TND",
		4:  "CLF UYW",
		8:  "BTC",
		18: "ETH",
	} {
		for code := range strings.FieldsSeq(codes) {
			m[code] = decimals
		}
	}

	return m
}()
