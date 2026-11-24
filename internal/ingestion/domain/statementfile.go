package domain

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// ErrUnreadableRows is returned for a statement file with rows the mapping
// can't read. The whole file is refused (spec: all-or-nothing).
var ErrUnreadableRows = errors.New("statement file has unreadable rows")

// StatementRow is one row of a statement file, read with a CSV mapping.
type StatementRow struct {
	// Line is the row's line in the file, from 1, counting skipped lines
	// and the header row.
	Line        int
	BookingDate time.Time
	// TransactionDate is the Booking date when the file has none.
	TransactionDate time.Time
	// ValueDate is zero when the file has none.
	ValueDate         time.Time
	Description       string
	Payee             string
	BankTransactionID string
	// Amount is signed: positive is money in, negative money out.
	Amount sharedkernel.Decimal
	// Resembles is the earlier Imported transaction this row only looks
	// like, which makes it a Suspected duplicate; the zero ID otherwise.
	Resembles sharedkernel.ID
}

// RowError is why one line of a statement file can't be read.
type RowError struct {
	Line   int
	Reason string
}

// RowsError lists every line of a statement file that can't be read.
type RowsError struct {
	Rows []RowError
}

func (e *RowsError) Error() string {
	reasons := make([]string, 0, len(e.Rows))
	for _, r := range e.Rows {
		reasons = append(reasons, fmt.Sprintf("line %d: %s", r.Line, r.Reason))
	}

	return "unreadable statement file: " + strings.Join(reasons, "; ")
}

// Is reports whether target is ErrUnreadableRows.
func (*RowsError) Is(target error) bool { return target == ErrUnreadableRows }

var plainDecimal = regexp.MustCompile(`^[+-]?[0-9]+(\.[0-9]+)?$`)

// ParseStatementFile reads a statement file with the mapping. It returns
// every row, or a *RowsError listing every line it can't read with the
// reason; an invalid mapping is refused as by Validate.
func ParseStatementFile(m CSVMapping, file []byte) ([]StatementRow, error) {
	return parseStatementFile(m, file, -1)
}

// parseStatementFile is ParseStatementFile that also refuses every amount
// with more than places decimal places; a negative places checks none.
func parseStatementFile(m CSVMapping, file []byte, places int32) ([]StatementRow, error) {
	rows, unreadable, err := collectStatementFile(m, file, places)
	if err != nil {
		return nil, err
	}

	if len(unreadable) > 0 {
		return nil, &RowsError{Rows: unreadable}
	}

	return rows, nil
}

// collectStatementFile reads a statement file with the mapping, collecting
// every readable row and every line it can't read together; an invalid
// mapping is refused as by Validate. A line that isn't valid CSV ends the
// reading, since nothing after it can be told apart.
func collectStatementFile(m CSVMapping, file []byte, places int32) ([]StatementRow, []RowError, error) {
	if err := m.Validate(); err != nil {
		return nil, nil, err
	}

	text, err := m.Encoding.decode(file)
	if err != nil {
		return nil, []RowError{{Line: 1, Reason: err.Error()}}, nil
	}

	file, skipped := skipLines(text, m.SkipLeadingLines)

	r := csv.NewReader(bytes.NewReader(file))
	r.Comma, _ = utf8.DecodeRuneInString(m.Delimiter)
	r.FieldsPerRecord = -1
	r.ReuseRecord = false

	p := parser{m: m, layout: dateFormats[m.DateFormat], columns: map[string]int{}, places: places}
	rows := []StatementRow{}
	header := m.HasHeaderRow

	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			line := skipped + 1

			var parseErr *csv.ParseError
			if errors.As(err, &parseErr) {
				line = skipped + parseErr.StartLine
			}

			p.refuse(line, "not valid CSV: "+err.Error())

			break
		}

		line, _ := r.FieldPos(0)
		line += skipped

		if header {
			header = false

			if !p.readHeader(line, record) {
				break
			}

			continue
		}

		if row, ok := p.readRow(line, record); ok {
			rows = append(rows, row)
		}
	}

	return rows, p.errors, nil
}

// skipLines drops the first n lines of file, returning the rest and how
// many lines it dropped.
func skipLines(file []byte, n int) ([]byte, int) {
	skipped := 0

	for ; skipped < n && len(file) > 0; skipped++ {
		i := bytes.IndexByte(file, '\n')
		if i < 0 {
			return nil, skipped + 1
		}

		file = file[i+1:]
	}

	return file, skipped
}

type parser struct {
	m       CSVMapping
	layout  string
	columns map[string]int // header name (lower case) to index
	places  int32          // the most decimal places an amount may have; none checked if negative
	errors  []RowError
}

func (p *parser) refuse(line int, reason string) {
	p.errors = append(p.errors, RowError{Line: line, Reason: reason})
}

func (p *parser) readHeader(line int, record []string) bool {
	for i, name := range record {
		key := strings.ToLower(strings.TrimSpace(name))
		if _, dup := p.columns[key]; !dup {
			p.columns[key] = i
		}
	}

	ok := true

	for _, f := range p.m.fields() {
		if f.column.Header == "" {
			continue
		}

		if _, found := p.columns[strings.ToLower(strings.TrimSpace(f.column.Header))]; !found {
			p.refuse(line, fmt.Sprintf("the header row has no %q column for the %s", f.column.Header, f.name))
			ok = false
		}
	}

	return ok
}

// cell returns the trimmed value of the column in record; false when the
// row is too short to have it.
func (p *parser) cell(record []string, c Column) (string, bool) {
	i := c.Position - 1
	if c.Header != "" {
		i = p.columns[strings.ToLower(strings.TrimSpace(c.Header))]
	}

	if i < 0 || i >= len(record) {
		return "", false
	}

	return strings.TrimSpace(record[i]), true
}

func (p *parser) readRow(line int, record []string) (StatementRow, bool) {
	row := StatementRow{Line: line}
	var problems []string

	text := func(name string, c Column) string {
		if !c.IsSet() {
			return ""
		}

		v, ok := p.cell(record, c)
		if !ok {
			problems = append(problems, fmt.Sprintf("the row has no %s column", name))
		}

		return v
	}

	date := func(name string, c Column, required bool) time.Time {
		v := text(name, c)
		if v == "" {
			if required && c.IsSet() {
				problems = append(problems, fmt.Sprintf("the %s is blank", name))
			}

			return time.Time{}
		}

		d, err := time.Parse(p.layout, v)
		if err != nil {
			problems = append(problems, fmt.Sprintf("the %s %q is not a date in the format %s", name, v, p.m.DateFormat))
		}

		return d
	}

	row.BookingDate = date("Booking date", p.m.BookingDate, true)

	row.TransactionDate = date("Transaction date", p.m.TransactionDate, false)
	if row.TransactionDate.IsZero() {
		row.TransactionDate = row.BookingDate
	}

	row.ValueDate = date("Value date", p.m.ValueDate, false)
	row.Description = text("description", p.m.Description)
	row.Payee = text("payee", p.m.Payee)
	row.BankTransactionID = text("bank transaction ID", p.m.BankTransactionID)

	amount, problem := p.amount(text)
	if problem == "" && p.places >= 0 && !amount.Equal(amount.Round(p.places)) {
		problem = fmt.Sprintf("the amount %s has more than the %d decimal places of the Account currency", amount, p.places)
	}

	if problem != "" {
		problems = append(problems, problem)
	}

	row.Amount = amount

	if len(problems) > 0 {
		p.refuse(line, strings.Join(problems, "; "))

		return StatementRow{}, false
	}

	return row, true
}

// amount reads the row's signed amount, or why it can't.
func (p *parser) amount(text func(string, Column) string) (sharedkernel.Decimal, string) {
	if p.m.AmountLayout == AmountLayoutSigned {
		v := text("amount", p.m.Amount)
		if v == "" {
			return sharedkernel.Decimal{}, "the amount is blank"
		}

		d, ok := p.number(v)
		if !ok {
			return sharedkernel.Decimal{}, fmt.Sprintf("the amount %q is not a number", v)
		}

		if p.m.PositiveAmountMeans == MoneyOut {
			d = d.Neg()
		}

		return d, ""
	}

	debit, credit := text("debit", p.m.Debit), text("credit", p.m.Credit)

	switch {
	case debit == "" && credit == "":
		return sharedkernel.Decimal{}, "both the debit and the credit are blank"
	case debit != "" && credit != "":
		return sharedkernel.Decimal{}, "the row has both a debit and a credit"
	case debit != "":
		d, ok := p.number(debit)
		if !ok {
			return sharedkernel.Decimal{}, fmt.Sprintf("the debit %q is not a number", debit)
		}

		return d.Abs().Neg(), ""
	default:
		d, ok := p.number(credit)
		if !ok {
			return sharedkernel.Decimal{}, fmt.Sprintf("the credit %q is not a number", credit)
		}

		return d.Abs(), ""
	}
}

// number reads a number written with the mapping's separators.
func (p *parser) number(v string) (sharedkernel.Decimal, bool) {
	if p.m.ThousandsSeparator != "" {
		v = strings.ReplaceAll(v, p.m.ThousandsSeparator, "")
		if p.m.ThousandsSeparator == " " {
			v = strings.ReplaceAll(v, " ", "")
		}
	}

	v = strings.Replace(v, p.m.DecimalSeparator, ".", 1)
	if !plainDecimal.MatchString(v) {
		return sharedkernel.Decimal{}, false
	}

	d, err := sharedkernel.ParseDecimal(v)

	return d, err == nil
}

// SampleRows is how many readable rows a mapping test shows.
const SampleRows = 10

// Sample is a sample statement file as an unsaved CSV mapping reads it
// (spec: Mapping test).
type Sample struct {
	// Rows is the first SampleRows readable rows, in file order.
	Rows []StatementRow
	// Readable counts the readable rows of the whole file.
	Readable int
	// Unreadable is every line of the whole file that can't be read; empty
	// when all can.
	Unreadable []RowError
}

// ReadSample reads a sample statement file with an unsaved mapping, as an
// upload to an Account in the currency would, but without refusing the
// file: readable rows and unreadable lines come back together. Rows before
// the Account's start date are kept, since it tests the mapping, not an
// import. An invalid mapping is refused as by Validate, and a file with no
// rows at all is ErrEmptyStatementFile.
func ReadSample(m CSVMapping, file []byte, currency sharedkernel.Currency) (Sample, error) {
	rows, unreadable, err := collectStatementFile(m, file, int32(currency.Decimals())) //nolint:gosec // at most 18 places
	if err != nil {
		return Sample{}, err
	}

	if len(rows) == 0 && len(unreadable) == 0 {
		return Sample{}, ErrEmptyStatementFile
	}

	return Sample{
		Rows:       rows[:min(len(rows), SampleRows)],
		Readable:   len(rows),
		Unreadable: append([]RowError{}, unreadable...),
	}, nil
}
