package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

var accountStart = time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)

func currency(t *testing.T, code string) sharedkernel.Currency {
	t.Helper()

	c, err := sharedkernel.ParseCurrency(code)
	if err != nil {
		t.Fatal(err)
	}

	return c
}

func dec(s string) sharedkernel.Decimal {
	d, err := sharedkernel.ParseDecimal(s)
	if err != nil {
		panic(err)
	}

	return d
}

func read(t *testing.T, file, code string) domain.Statement {
	t.Helper()

	s, err := domain.ReadStatement(signedMapping(), []byte(file), currency(t, code), accountStart)
	if err != nil {
		t.Fatalf("ReadStatement: %v", err)
	}

	return s
}

// Rows booked before the Account's start date are ignored and counted; the
// rest are the statement's rows, and their total is what the Opening
// balance is worked out from (ADR-0018).
func TestRowsBeforeTheStartDateAreIgnoredAndCounted(t *testing.T) {
	t.Parallel()

	s := read(t, "Date;Description;Amount\n"+
		"10/01/2026;Before;-99,99\n"+
		"14/01/2026;Day before;-0,01\n"+
		"15/01/2026;On the start date;-10,50\n"+
		"20/01/2026;Salary;2.000,00\n", "EUR")

	if len(s.Rows) != 2 || s.Rows[0].Description != "On the start date" || s.Rows[1].Line != 5 {
		t.Errorf("rows = %+v, want the two on or after the start date", s.Rows)
	}

	if s.Ignored != 2 {
		t.Errorf("ignored = %d, want 2", s.Ignored)
	}

	if !s.Total.Equal(dec("1989.50")) {
		t.Errorf("total = %s, want 1989.50", s.Total)
	}

	if got := s.OpeningBalance(dec("2500.00")); !got.Equal(dec("510.50")) {
		t.Errorf("opening balance = %s, want 2500.00 - 1989.50 = 510.50", got)
	}
}

func TestAStatementWithEveryRowBeforeTheStartDateHasNoRows(t *testing.T) {
	t.Parallel()

	s := read(t, "Date;Description;Amount\n01/01/2026;Old;-5,00\n", "EUR")

	if len(s.Rows) != 0 || s.Ignored != 1 || !s.Total.IsZero() {
		t.Errorf("statement = %+v, want no rows, one ignored, total 0", s)
	}

	if got := s.OpeningBalance(dec("12.34")); !got.Equal(dec("12.34")) {
		t.Errorf("opening balance = %s, want the Export balance 12.34", got)
	}
}

// An amount with more decimal places than the Account currency has can't
// be the bank's: the row is refused, like any other unreadable row.
func TestARowWithMoreDecimalPlacesThanTheCurrencyIsUnreadable(t *testing.T) {
	t.Parallel()

	file := "Date;Description;Amount\n" +
		"15/01/2026;Fine;-1,50\n" +
		"16/01/2026;Trailing zero;-1,500\n" +
		"17/01/2026;Too precise;-1,505\n" +
		"18/01/2026;One decimal;-1,5\n"

	_, err := domain.ReadStatement(signedMapping(), []byte(file), currency(t, "EUR"), accountStart)

	var rows *domain.RowsError
	if !errors.As(err, &rows) || len(rows.Rows) != 1 || rows.Rows[0].Line != 4 {
		t.Fatalf("err = %v, want only line 4 refused", err)
	}

	if !strings.Contains(rows.Rows[0].Reason, "decimal places") {
		t.Errorf("reason = %q, want it to name the decimal places", rows.Rows[0].Reason)
	}

	// The same amounts are fine in a currency with three decimal places.
	if s := read(t, file, "BHD"); len(s.Rows) != 4 {
		t.Errorf("BHD rows = %+v, want all four", s.Rows)
	}

	// And a currency with none refuses every fractional amount.
	_, err = domain.ReadStatement(signedMapping(), []byte(file), currency(t, "JPY"), accountStart)
	assertRowErrors(t, err, []int{2, 3, 4, 5})
}

// Decimal places are checked together with every other problem, so the
// whole file's problems come back at once.
func TestDecimalPlacesAreReportedWithEveryOtherUnreadableRow(t *testing.T) {
	t.Parallel()

	_, err := domain.ReadStatement(signedMapping(), []byte("Date;Description;Amount\n"+
		"15/01/2026;Too precise;-1,505\n"+
		"not a date;Bad;-1,50\n"), currency(t, "EUR"), accountStart)
	assertRowErrors(t, err, []int{2, 3})
}

// A file with no rows at all is refused: there is nothing to import, and
// it is far more likely a wrong export than a real statement.
func TestAStatementFileWithNoRowsIsRefused(t *testing.T) {
	t.Parallel()

	for name, file := range map[string]string{
		"empty":       "",
		"header only": "Date;Description;Amount\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := domain.ReadStatement(signedMapping(), []byte(file), currency(t, "EUR"), accountStart)
			if !errors.Is(err, domain.ErrEmptyStatementFile) {
				t.Errorf("err = %v, want ErrEmptyStatementFile", err)
			}
		})
	}
}

func TestAnExportBalanceMustFitTheCurrency(t *testing.T) {
	t.Parallel()

	if err := domain.CheckExportBalance(dec("10.25"), currency(t, "EUR")); err != nil {
		t.Errorf("10.25 EUR refused: %v", err)
	}

	if err := domain.CheckExportBalance(dec("10.255"), currency(t, "EUR")); !errors.Is(err, domain.ErrInvalidExportBalance) {
		t.Errorf("10.255 EUR: err = %v, want ErrInvalidExportBalance", err)
	}
}

// The preview ID ties a confirm to its preview with no server state: it
// changes with the Account, the Connection, the file, the Export balance or
// the mapping version, and with nothing else.
func TestThePreviewIDChangesWithWhatThePreviewShowed(t *testing.T) {
	t.Parallel()

	account := sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000001")
	connection := sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000002")
	other := sharedkernel.MustParseID("0198f0a0-0000-7000-8000-000000000003")
	file := domain.FileHash([]byte("Date;Description;Amount\n"))

	base := domain.PreviewID(account, connection, file, dec("100.50"), 1)

	if got := domain.PreviewID(account, connection, file, dec("100.5"), 1); got != base {
		t.Error("the same Export balance written differently changed the preview ID")
	}

	for name, got := range map[string]string{
		"account":         domain.PreviewID(other, connection, file, dec("100.50"), 1),
		"connection":      domain.PreviewID(account, other, file, dec("100.50"), 1),
		"file":            domain.PreviewID(account, connection, domain.FileHash([]byte("other")), dec("100.50"), 1),
		"export balance":  domain.PreviewID(account, connection, file, dec("100.51"), 1),
		"mapping version": domain.PreviewID(account, connection, file, dec("100.50"), 2),
	} {
		if got == base {
			t.Errorf("a different %s kept the preview ID", name)
		}
	}
}
