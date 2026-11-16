package domain_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
)

// row is a parsed row written as plain strings, so expected rows read as
// they would in the file's own terms.
type row struct {
	Line                                    int
	BookingDate, TransactionDate, ValueDate string
	Description, Payee, BankTransactionID   string
	Amount                                  string
}

func parse(t *testing.T, m domain.CSVMapping, file string) []row {
	t.Helper()

	rows, err := domain.ParseStatementFile(m, []byte(file))
	if err != nil {
		t.Fatalf("ParseStatementFile: %v", err)
	}

	got := make([]row, 0, len(rows))

	for _, r := range rows {
		valueDate := ""
		if !r.ValueDate.IsZero() {
			valueDate = r.ValueDate.Format("2006-01-02")
		}

		got = append(got, row{
			Line:              r.Line,
			BookingDate:       r.BookingDate.Format("2006-01-02"),
			TransactionDate:   r.TransactionDate.Format("2006-01-02"),
			ValueDate:         valueDate,
			Description:       r.Description,
			Payee:             r.Payee,
			BankTransactionID: r.BankTransactionID,
			Amount:            r.Amount.String(),
		})
	}

	return got
}

func assertRows(t *testing.T, got, want []row) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("rows:\n  got  %+v\n  want %+v", got, want)
	}
}

func TestParsingASignedFileWithAHeader(t *testing.T) {
	t.Parallel()

	got := parse(t, signedMapping(), "Date;Description;Amount\n"+
		"01/02/2026;Coffee;-1.234,50\r\n"+
		"03/02/2026;\"Salary; February\";2.000,00\n")

	// Transaction date defaults to the Booking date.
	assertRows(t, got, []row{
		{Line: 2, BookingDate: "2026-02-01", TransactionDate: "2026-02-01", Description: "Coffee", Amount: "-1234.5"},
		{Line: 3, BookingDate: "2026-02-03", TransactionDate: "2026-02-03", Description: "Salary; February", Amount: "2000"},
	})
}

func TestParsingAnInvertedSign(t *testing.T) {
	t.Parallel()

	m := signedMapping()
	m.PositiveAmountMeans = domain.MoneyOut

	got := parse(t, m, "Date;Description;Amount\n01/02/2026;Card;12,30\n02/02/2026;Refund;-5,00\n")

	assertRows(t, got, []row{
		{Line: 2, BookingDate: "2026-02-01", TransactionDate: "2026-02-01", Description: "Card", Amount: "-12.3"},
		{Line: 3, BookingDate: "2026-02-02", TransactionDate: "2026-02-02", Description: "Refund", Amount: "5"},
	})
}

func TestParsingDebitAndCreditColumnsByPosition(t *testing.T) {
	t.Parallel()

	got := parse(t, debitCreditMapping(), "2026-02-01,Coffee,3.50,\n2026-02-02,Refund,,10.00\n2026-02-03,Fee,-1.00,\n")

	// Debits are money out and credits money in, whatever sign the bank writes.
	assertRows(t, got, []row{
		{Line: 1, BookingDate: "2026-02-01", TransactionDate: "2026-02-01", Description: "Coffee", Amount: "-3.5"},
		{Line: 2, BookingDate: "2026-02-02", TransactionDate: "2026-02-02", Description: "Refund", Amount: "10"},
		{Line: 3, BookingDate: "2026-02-03", TransactionDate: "2026-02-03", Description: "Fee", Amount: "-1"},
	})
}

func TestParsingPositionalColumnsInAFileWithAHeader(t *testing.T) {
	t.Parallel()

	m := signedMapping()
	m.Description = domain.Column{Position: 3}

	got := parse(t, m, "Date;Amount;Whatever the bank calls it\n01/02/2026;1,00;Coffee\n")

	assertRows(t, got, []row{
		{Line: 2, BookingDate: "2026-02-01", TransactionDate: "2026-02-01", Description: "Coffee", Amount: "1"},
	})
}

func TestParsingSkipsLeadingLines(t *testing.T) {
	t.Parallel()

	m := signedMapping()
	m.SkipLeadingLines = 2

	got := parse(t, m, "Bank \"Example\" export\nAccount;123;456\nDate;Description;Amount\n01/02/2026;Coffee;-1,00\n")

	assertRows(t, got, []row{
		{Line: 4, BookingDate: "2026-02-01", TransactionDate: "2026-02-01", Description: "Coffee", Amount: "-1"},
	})
}

func TestParsingEveryOptionalField(t *testing.T) {
	t.Parallel()

	m := signedMapping()
	everyOptionalField(&m)

	got := parse(t, m, "\xef\xbb\xbfReference;Date;Transaction date;Value date;Description;Payee;Amount\n"+
		"TX-1;03/02/2026;01/02/2026;04/02/2026;Groceries; MERCADONA ;-45,10\n"+
		";05/02/2026;;;Cash;;-20,00\n")

	// Blank optional cells: the Transaction date falls back to the Booking
	// date and the Value date stays empty.
	assertRows(t, got, []row{
		{
			Line: 2, BookingDate: "2026-02-03", TransactionDate: "2026-02-01", ValueDate: "2026-02-04",
			Description: "Groceries", Payee: "MERCADONA", BankTransactionID: "TX-1", Amount: "-45.1",
		},
		{Line: 3, BookingDate: "2026-02-05", TransactionDate: "2026-02-05", Description: "Cash", Amount: "-20"},
	})
}

func TestParsingDecimalFormats(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		decimal, thousands, amount, want string
	}{
		"point, comma thousands": {".", ",", "1,234,567.89", "1234567.89"},
		"comma, point thousands": {",", ".", "1.234,56", "1234.56"},
		"comma, space thousands": {",", " ", "-1 234,56", "-1234.56"},
		"point, apostrophe":      {".", "'", "1'234.50", "1234.5"},
		"point, no thousands":    {".", "", "1234.5", "1234.5"},
		"explicit plus":          {".", "", "+7.25", "7.25"},
		"no decimals":            {",", ".", "12", "12"},
		"many decimals (crypto)": {".", "", "0.000000000000000001", "0.000000000000000001"},
		"surrounding whitespace": {",", "", "  3,10 ", "3.1"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := debitCreditMapping()
			m.AmountLayout, m.PositiveAmountMeans = domain.AmountLayoutSigned, domain.MoneyIn
			m.Debit, m.Credit = domain.Column{}, domain.Column{}
			m.Amount = domain.Column{Position: 3}
			m.Delimiter, m.DecimalSeparator, m.ThousandsSeparator = "|", tc.decimal, tc.thousands

			got := parse(t, m, "2026-02-01|Row|"+tc.amount+"\n")
			if len(got) != 1 || got[0].Amount != tc.want {
				t.Errorf("rows = %+v, want one with amount %s", got, tc.want)
			}
		})
	}
}

func TestParsingDateFormats(t *testing.T) {
	t.Parallel()

	for format, date := range map[string]string{
		"YYYY-MM-DD": "2026-02-07",
		"YYYY/MM/DD": "2026/02/07",
		"YYYYMMDD":   "20260207",
		"DD/MM/YYYY": "07/02/2026",
		"DD-MM-YYYY": "07-02-2026",
		"DD.MM.YYYY": "07.02.2026",
		"MM/DD/YYYY": "02/07/2026",
		"MM-DD-YYYY": "02-07-2026",
		"DD/MM/YY":   "07/02/26",
		"MM/DD/YY":   "02/07/26",
	} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			m := debitCreditMapping()
			m.DateFormat = format

			got := parse(t, m, date+",Row,1.00,\n")
			if len(got) != 1 || got[0].BookingDate != "2026-02-07" {
				t.Errorf("rows = %+v, want one booked on 2026-02-07", got)
			}
		})
	}
}

func TestParsingReportsEveryBadRow(t *testing.T) {
	t.Parallel()

	m := signedMapping()
	everyOptionalField(&m)

	_, err := domain.ParseStatementFile(m, []byte("Reference;Date;Transaction date;Value date;Description;Payee;Amount\n"+
		"A;01/02/2026;;;Fine;;-1,00\n"+
		"B;2026-02-02;;;Bad Booking date;;-1,00\n"+
		"C;03/02/2026;;;Bad amount;;one euro\n"+
		"D;04/02/2026;31/02/2026;;Bad Transaction date;;-1,00\n"+
		"E;05/02/2026\n"+
		"F;;;;No Booking date;;-1,00\n"+
		"G;07/02/2026;;;No amount;;\n"+
		"H;08/02/2026;;;Two decimal points;;1,000,1\n"+
		"I;09/02/2026;;xx;Bad Value date;;-1,00\n"))

	assertRowErrors(t, err, []int{3, 4, 5, 6, 7, 8, 9, 10})
}

func TestParsingRefusesADebitAndCreditRowWithNoAmountOrBoth(t *testing.T) {
	t.Parallel()

	_, err := domain.ParseStatementFile(debitCreditMapping(),
		[]byte("2026-02-01,Fine,1.00,\n2026-02-02,Neither,,\n2026-02-03,Both,1.00,2.00\n"))

	assertRowErrors(t, err, []int{2, 3})
}

func TestParsingRefusesAHeaderWithoutAMappedColumn(t *testing.T) {
	t.Parallel()

	_, err := domain.ParseStatementFile(signedMapping(), []byte("Date;Text;Amount\n01/02/2026;Coffee;-1,00\n"))

	assertRowErrors(t, err, []int{1})
}

func TestParsingRefusesAFileThatIsNotCSV(t *testing.T) {
	t.Parallel()

	_, err := domain.ParseStatementFile(debitCreditMapping(), []byte("2026-02-01,\"unterminated,1.00,\n"))

	assertRowErrors(t, err, []int{1})
}

func TestParsingWithAnInvalidMapping(t *testing.T) {
	t.Parallel()

	m := signedMapping()
	m.DateFormat = "later"

	_, err := domain.ParseStatementFile(m, []byte("Date;Description;Amount\n"))
	if !errors.Is(err, domain.ErrInvalidCSVMapping) {
		t.Errorf("err = %v, want %v", err, domain.ErrInvalidCSVMapping)
	}
}

func TestParsingAFileWithNoRows(t *testing.T) {
	t.Parallel()

	assertRows(t, parse(t, signedMapping(), "Date;Description;Amount\n"), []row{})
}

func assertRowErrors(t *testing.T, err error, wantLines []int) {
	t.Helper()

	var rowsErr *domain.RowsError
	if !errors.As(err, &rowsErr) || !errors.Is(err, domain.ErrUnreadableRows) {
		t.Fatalf("err = %v, want a %T", err, rowsErr)
	}

	lines := make([]int, 0, len(rowsErr.Rows))

	for _, r := range rowsErr.Rows {
		if r.Reason == "" {
			t.Errorf("line %d has no reason", r.Line)
		}

		lines = append(lines, r.Line)
	}

	if !reflect.DeepEqual(lines, wantLines) {
		t.Errorf("refused lines = %v, want %v (%v)", lines, wantLines, rowsErr.Rows)
	}
}
