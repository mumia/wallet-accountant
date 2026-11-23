package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

var (
	// ErrInvalidCSVMapping is returned for a CSV mapping that can't read a
	// statement file. Every other mapping refusal is also this error.
	ErrInvalidCSVMapping = errors.New("invalid CSV mapping")
	// ErrMissingMappedField is returned for a mapping that leaves out a
	// required field.
	ErrMissingMappedField = errors.New("required field not mapped")
	// ErrUnknownDateFormat is returned for a date format that is not one of
	// DateFormats.
	ErrUnknownDateFormat = errors.New("unknown date format")
	// ErrColumnUsedTwice is returned for a mapping that reads two fields from
	// one column.
	ErrColumnUsedTwice = errors.New("column used twice")
)

// AmountLayout is how a statement file holds its amounts.
type AmountLayout string

// The amount layouts.
const (
	// AmountLayoutSigned is one column with a signed amount.
	AmountLayoutSigned AmountLayout = "signed"
	// AmountLayoutDebitCredit is one column for money out and one for money in.
	AmountLayoutDebitCredit AmountLayout = "debit_credit"
)

// AmountSign is what a positive amount in a signed column means.
type AmountSign string

// The meanings of a positive signed amount.
const (
	MoneyIn  AmountSign = "money_in"
	MoneyOut AmountSign = "money_out"
)

// Column picks the column a field is read from: by its name in the header
// row or by its position, counted from 1. The zero Column maps nothing.
type Column struct {
	Header   string `json:"header,omitempty"`
	Position int    `json:"position,omitempty"`
}

// IsSet reports whether the Column maps anything.
func (c Column) IsSet() bool { return c != Column{} }

// dateFormats maps every date format a mapping may name to its Go layout.
var dateFormats = map[string]string{
	"YYYY-MM-DD": "2006-01-02",
	"YYYY/MM/DD": "2006/01/02",
	"YYYYMMDD":   "20060102",
	"DD/MM/YYYY": "02/01/2006",
	"DD-MM-YYYY": "02-01-2006",
	"DD.MM.YYYY": "02.01.2006",
	"MM/DD/YYYY": "01/02/2006",
	"MM-DD-YYYY": "01-02-2006",
	"DD/MM/YY":   "02/01/06",
	"MM/DD/YY":   "01/02/06",
}

// DateFormats lists every date format a mapping may name.
func DateFormats() []string {
	formats := make([]string, 0, len(dateFormats))
	for f := range dateFormats {
		formats = append(formats, f)
	}

	slices.Sort(formats)

	return formats
}

var (
	decimalSeparators   = []string{".", ","}
	thousandsSeparators = []string{"", ".", ",", " ", "'"}
)

// CSVMapping describes how one bank lays out its statement CSV: how to read
// the file and which column holds each field. Booking date, description and
// the amount (one signed column, or debit and credit columns) are required;
// Transaction date, Value date, payee and bank transaction ID are optional.
type CSVMapping struct {
	// Encoding is how the file's bytes are decoded; empty is EncodingUTF8.
	Encoding           Encoding `json:"encoding,omitempty"`
	Delimiter          string   `json:"delimiter"`
	HasHeaderRow       bool     `json:"hasheaderrow"`
	SkipLeadingLines   int      `json:"skipleadinglines"`
	DateFormat         string   `json:"dateformat"`
	DecimalSeparator   string   `json:"decimalseparator"`
	ThousandsSeparator string   `json:"thousandsseparator"`

	AmountLayout AmountLayout `json:"amountlayout"`
	// PositiveAmountMeans is what a positive amount in the signed column
	// means; banks that invert the sign write money out as positive. Only
	// for AmountLayoutSigned.
	PositiveAmountMeans AmountSign `json:"positiveamountmeans,omitempty"`

	BookingDate Column `json:"bookingdate"`
	Description Column `json:"description"`
	Amount      Column `json:"amount"`
	Debit       Column `json:"debit"`
	Credit      Column `json:"credit"`

	TransactionDate   Column `json:"transactiondate"`
	ValueDate         Column `json:"valuedate"`
	Payee             Column `json:"payee"`
	BankTransactionID Column `json:"banktransactionid"`
}

// MappingProblem is one reason a CSV mapping is invalid. It is
// ErrInvalidCSVMapping and its own kind of refusal.
type MappingProblem struct {
	kind   error
	detail string
}

func (p MappingProblem) Error() string { return p.detail }

// Is reports whether the problem is target.
func (p MappingProblem) Is(target error) bool {
	return target == ErrInvalidCSVMapping || target == p.kind
}

// InvalidMappingError lists every problem of an invalid CSV mapping.
type InvalidMappingError struct {
	Problems []MappingProblem
}

func (e *InvalidMappingError) Error() string {
	details := make([]string, 0, len(e.Problems))
	for _, p := range e.Problems {
		details = append(details, p.detail)
	}

	return "invalid CSV mapping: " + strings.Join(details, "; ")
}

// Unwrap returns every problem.
func (e *InvalidMappingError) Unwrap() []error {
	errs := make([]error, 0, len(e.Problems))
	for _, p := range e.Problems {
		errs = append(errs, p)
	}

	return errs
}

// Validate checks the mapping as a whole and returns every problem
// together, as an *InvalidMappingError, or nil for a valid mapping.
func (m CSVMapping) Validate() error {
	v := &validation{}

	m.validateFile(v)
	m.validateColumns(v)

	if len(v.problems) == 0 {
		return nil
	}

	return &InvalidMappingError{Problems: v.problems}
}

type validation struct{ problems []MappingProblem }

func (v *validation) add(kind error, format string, args ...any) {
	v.problems = append(v.problems, MappingProblem{kind: kind, detail: fmt.Sprintf(format, args...)})
}

func (m CSVMapping) validateFile(v *validation) {
	switch m.Encoding {
	case "", EncodingUTF8, EncodingWindows1252:
	default:
		v.add(ErrInvalidCSVMapping, "the encoding must be UTF-8 or Windows-1252")
	}

	if utf8.RuneCountInString(m.Delimiter) != 1 || strings.ContainsAny(m.Delimiter, "\"\r\n") {
		v.add(ErrInvalidCSVMapping, "the delimiter must be one character other than a quote or a line break")
	}

	if m.SkipLeadingLines < 0 {
		v.add(ErrInvalidCSVMapping, "the number of leading lines to skip must not be negative")
	}

	if _, ok := dateFormats[m.DateFormat]; !ok {
		v.add(ErrUnknownDateFormat, "the date format %q is not one of %s", m.DateFormat, strings.Join(DateFormats(), ", "))
	}

	if !slices.Contains(decimalSeparators, m.DecimalSeparator) {
		v.add(ErrInvalidCSVMapping, "the decimal separator must be \".\" or \",\"")
	}

	switch {
	case !slices.Contains(thousandsSeparators, m.ThousandsSeparator):
		v.add(ErrInvalidCSVMapping, "the thousands separator must be empty, \".\", \",\", a space or \"'\"")
	case m.ThousandsSeparator != "" && m.ThousandsSeparator == m.DecimalSeparator:
		v.add(ErrInvalidCSVMapping, "the thousands separator must differ from the decimal separator")
	}
}

// field is a mapped field: its name in problems, its column and whether the
// mapping needs it.
type field struct {
	name     string
	column   Column
	required bool
}

func (m CSVMapping) fields() []field {
	signed, debitCredit := m.AmountLayout == AmountLayoutSigned, m.AmountLayout == AmountLayoutDebitCredit

	return []field{
		{"Booking date", m.BookingDate, true},
		{"description", m.Description, true},
		{"amount", m.Amount, signed},
		{"debit", m.Debit, debitCredit},
		{"credit", m.Credit, debitCredit},
		{"Transaction date", m.TransactionDate, false},
		{"Value date", m.ValueDate, false},
		{"payee", m.Payee, false},
		{"bank transaction ID", m.BankTransactionID, false},
	}
}

func (m CSVMapping) validateColumns(v *validation) {
	m.validateAmountLayout(v)

	headers, positions := map[string]string{}, map[int]string{}

	for _, f := range m.fields() {
		c := f.column

		switch {
		case !c.IsSet():
			if f.required {
				v.add(ErrMissingMappedField, "the %s column is required", f.name)
			}

			continue
		case c.Header != "" && c.Position != 0:
			v.add(ErrInvalidCSVMapping, "the %s column must be given by header or by position, not both", f.name)

			continue
		case c.Position < 0:
			v.add(ErrInvalidCSVMapping, "the %s column's position must be 1 or more", f.name)

			continue
		case c.Position > 0:
			if other, used := positions[c.Position]; used {
				v.add(ErrColumnUsedTwice, "the %s and %s are both read from column %d", other, f.name, c.Position)
			}

			positions[c.Position] = f.name

			continue
		}

		header := strings.ToLower(strings.TrimSpace(c.Header))

		switch {
		case header == "":
			v.add(ErrInvalidCSVMapping, "the %s column's header must not be blank", f.name)
		case !m.HasHeaderRow:
			v.add(ErrInvalidCSVMapping, "the %s column is given by header, but the file has no header row", f.name)
		default:
			if other, used := headers[header]; used {
				v.add(ErrColumnUsedTwice, "the %s and %s are both read from column %q", other, f.name, c.Header)
			}

			headers[header] = f.name
		}
	}
}

func (m CSVMapping) validateAmountLayout(v *validation) {
	switch m.AmountLayout {
	case AmountLayoutSigned:
		switch m.PositiveAmountMeans {
		case MoneyIn, MoneyOut:
		case "":
			v.add(ErrMissingMappedField, "a signed amount needs to say whether a positive amount is money in or money out")
		default:
			v.add(ErrInvalidCSVMapping, "a positive amount must mean money in or money out")
		}

		if m.Debit.IsSet() || m.Credit.IsSet() {
			v.add(ErrInvalidCSVMapping, "a signed amount has no debit or credit columns")
		}
	case AmountLayoutDebitCredit:
		if m.Amount.IsSet() {
			v.add(ErrInvalidCSVMapping, "debit and credit columns leave no room for a signed amount column")
		}

		if m.PositiveAmountMeans != "" {
			v.add(ErrInvalidCSVMapping, "debit and credit columns have no sign to interpret")
		}
	case "":
		v.add(ErrMissingMappedField, "the amount layout is required: one signed column, or debit and credit columns")
	default:
		v.add(ErrInvalidCSVMapping, "the amount layout must be signed or debit and credit")
	}
}
