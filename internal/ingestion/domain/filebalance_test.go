package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
)

// balanceMapping is signedMapping with a running balance column.
func balanceMapping() domain.CSVMapping {
	m := signedMapping()
	m.Balance = domain.Column{Header: "Balance"}

	return m
}

func readWithBalance(t *testing.T, file string) domain.Statement {
	t.Helper()

	s, err := domain.ReadStatement(balanceMapping(), []byte(file), currency(t, "EUR"), accountStart)
	if err != nil {
		t.Fatalf("ReadStatement: %v", err)
	}

	return s
}

func assertFileBalance(t *testing.T, s domain.Statement, amount string, line int) {
	t.Helper()

	got, err := s.FileBalance()
	if err != nil {
		t.Fatalf("FileBalance: %v", err)
	}

	if !got.Amount.Equal(dec(amount)) || got.Line != line {
		t.Errorf("file balance = %s (line %d), want %s (line %d)", got.Amount, got.Line, amount, line)
	}
}

func TestEachRowReadsItsBalanceWithTheMappingsSeparators(t *testing.T) {
	t.Parallel()

	s := readWithBalance(t, "Date;Description;Amount;Balance\n"+
		"20/01/2026;Coffee;-1,50;1.998,50\n"+
		"21/01/2026;Rent;-2.000,00;-1,50\n")

	if !s.Rows[0].Balance.Equal(dec("1998.50")) || !s.Rows[1].Balance.Equal(dec("-1.50")) {
		t.Errorf("balances = %s, %s, want 1998.50, -1.50", s.Rows[0].Balance, s.Rows[1].Balance)
	}
}

func TestTheFileBalanceOfAnOldestFirstFileIsItsLastRowsBalance(t *testing.T) {
	t.Parallel()

	s := readWithBalance(t, "Date;Description;Amount;Balance\n"+
		"20/01/2026;Salary;2.000,00;2.100,00\n"+
		"21/01/2026;Coffee;-1,50;2.098,50\n"+
		"22/01/2026;Rent;-1.000,00;1.098,50\n")

	assertFileBalance(t, s, "1098.50", 4)
}

func TestTheFileBalanceOfANewestFirstFileIsItsFirstRowsBalance(t *testing.T) {
	t.Parallel()

	s := readWithBalance(t, "Date;Description;Amount;Balance\n"+
		"22/01/2026;Rent;-1.000,00;1.098,50\n"+
		"21/01/2026;Coffee;-1,50;2.098,50\n"+
		"20/01/2026;Salary;2.000,00;2.100,00\n")

	assertFileBalance(t, s, "1098.50", 2)
}

func TestTheFileBalanceOfAOneRowFileIsThatRowsBalance(t *testing.T) {
	t.Parallel()

	s := readWithBalance(t, "Date;Description;Amount;Balance\n"+
		"20/01/2026;Coffee;-1,50;8.758,21\n")

	assertFileBalance(t, s, "8758.21", 2)
}

func TestTheFileBalanceCantBeWorkedOutWhenTheColumnDoesNotAddUp(t *testing.T) {
	t.Parallel()

	s := readWithBalance(t, "Date;Description;Amount;Balance\n"+
		"20/01/2026;Salary;2.000,00;2.100,00\n"+
		"21/01/2026;Coffee;-1,50;2.098,50\n"+
		"22/01/2026;Rent;-1.000,00;1.000,00\n")

	_, err := s.FileBalance()
	if !errors.Is(err, domain.ErrFileBalanceUnknown) {
		t.Fatalf("err = %v, want ErrFileBalanceUnknown", err)
	}

	if !strings.Contains(err.Error(), "doesn't add up from one end of the file to the other") {
		t.Errorf("err = %q, want the reason", err)
	}
}

// Two rows whose balances add up read both ways (+10 to 10, then -10 to 0;
// or -10 to 0, then +10 to 10) leave the newest row unknown.
func TestTheFileBalanceCantBeWorkedOutWhenTheColumnAddsUpFromBothEnds(t *testing.T) {
	t.Parallel()

	s := readWithBalance(t, "Date;Description;Amount;Balance\n"+
		"20/01/2026;In;10,00;10,00\n"+
		"21/01/2026;Out;-10,00;0,00\n")

	if _, err := s.FileBalance(); !errors.Is(err, domain.ErrFileBalanceUnknown) {
		t.Fatalf("err = %v, want ErrFileBalanceUnknown", err)
	}
}

// Rows before the Account's start date aren't imported, but the file's
// balance is about the file: they count.
func TestRowsBeforeTheStartDateCountForTheFileBalance(t *testing.T) {
	t.Parallel()

	s := readWithBalance(t, "Date;Description;Amount;Balance\n"+
		"10/01/2026;Before;-99,99;900,01\n"+
		"20/01/2026;Coffee;-1,50;898,51\n")

	if s.Ignored != 1 {
		t.Fatalf("ignored = %d, want 1", s.Ignored)
	}

	assertFileBalance(t, s, "898.51", 3)
}

func TestABalanceCellThatIsNotANumberIsAnUnreadableRow(t *testing.T) {
	t.Parallel()

	_, err := domain.ReadStatement(balanceMapping(), []byte("Date;Description;Amount;Balance\n"+
		"20/01/2026;Coffee;-1,50;n/a\n"+
		"21/01/2026;Tea;-1,00;\n"+
		"22/01/2026;Cake;-2,00;1,005\n"+
		"23/01/2026;Bun;-2,00;10,00\n"), currency(t, "EUR"), accountStart)

	var rows *domain.RowsError
	if !errors.As(err, &rows) || len(rows.Rows) != 3 {
		t.Fatalf("err = %v, want lines 2, 3 and 4 unreadable", err)
	}

	for i, want := range []string{`the balance "n/a" is not a number`, "the balance is blank", "decimal places"} {
		if !strings.Contains(rows.Rows[i].Reason, want) {
			t.Errorf("line %d: reason = %q, want %q", rows.Rows[i].Line, rows.Rows[i].Reason, want)
		}
	}
}

func TestAnExportBalanceThatIsTheFilesBalancePasses(t *testing.T) {
	t.Parallel()

	s := readWithBalance(t, "Date;Description;Amount;Balance\n"+
		"20/01/2026;Coffee;-1,50;8.758,21\n")

	if err := s.CheckExportBalanceAgainstFile(dec("8758.210")); err != nil {
		t.Errorf("err = %v, want none", err)
	}
}

func TestAnExportBalanceThatIsNotTheFilesBalanceIsRefused(t *testing.T) {
	t.Parallel()

	s := readWithBalance(t, "Date;Description;Amount;Balance\n"+
		"20/01/2026;Coffee;-1,50;8.758,21\n")

	err := s.CheckExportBalanceAgainstFile(dec("1000.00"))

	var differs *domain.ExportBalanceDiffersFromFileError
	if !errors.As(err, &differs) {
		t.Fatalf("err = %v, want *ExportBalanceDiffersFromFileError", err)
	}

	if !differs.ExportBalance.Equal(dec("1000")) || !differs.File.Amount.Equal(dec("8758.21")) ||
		differs.File.Line != 2 || differs.Currency != "EUR" {
		t.Errorf("err = %+v", differs)
	}
}

func TestAnExportBalanceIsRefusedWhenTheFilesBalanceCantBeWorkedOut(t *testing.T) {
	t.Parallel()

	s := readWithBalance(t, "Date;Description;Amount;Balance\n"+
		"20/01/2026;Salary;2.000,00;2.100,00\n"+
		"21/01/2026;Coffee;-1,50;5,00\n")

	if err := s.CheckExportBalanceAgainstFile(dec("5")); !errors.Is(err, domain.ErrFileBalanceUnknown) {
		t.Errorf("err = %v, want ErrFileBalanceUnknown", err)
	}
}

func TestWithoutABalanceColumnTheExportBalanceIsNotCheckedAgainstTheFile(t *testing.T) {
	t.Parallel()

	s := read(t, "Date;Description;Amount\n20/01/2026;Coffee;-1,50\n", "EUR")

	if s.HasFileBalance() {
		t.Error("HasFileBalance = true, want false")
	}

	if err := s.CheckExportBalanceAgainstFile(dec("1000")); err != nil {
		t.Errorf("err = %v, want none", err)
	}
}

func TestTheBalanceColumnCantShareAColumnWithAnotherField(t *testing.T) {
	t.Parallel()

	m := balanceMapping()
	m.Balance = domain.Column{Header: "amount"}

	if err := m.Validate(); !errors.Is(err, domain.ErrColumnUsedTwice) {
		t.Errorf("err = %v, want ErrColumnUsedTwice", err)
	}
}

func TestASampleGivesTheFilesBalance(t *testing.T) {
	t.Parallel()

	sample, err := domain.ReadSample(balanceMapping(), []byte("Date;Description;Amount;Balance\n"+
		"10/01/2026;Before;-99,99;900,01\n"+
		"20/01/2026;Coffee;-1,50;898,51\n"), currency(t, "EUR"))
	if err != nil {
		t.Fatal(err)
	}

	if sample.FileBalance == nil || !sample.FileBalance.Amount.Equal(dec("898.51")) || sample.FileBalance.Line != 3 ||
		sample.FileBalanceProblem != "" {
		t.Errorf("file balance = %+v, problem %q", sample.FileBalance, sample.FileBalanceProblem)
	}
}

func TestASampleSaysWhyItsBalanceCantBeWorkedOut(t *testing.T) {
	t.Parallel()

	for name, file := range map[string]string{
		"doesn't add up": "Date;Description;Amount;Balance\n" +
			"20/01/2026;Salary;2.000,00;2.100,00\n21/01/2026;Coffee;-1,50;5,00\n",
		"unreadable rows": "Date;Description;Amount;Balance\n" +
			"20/01/2026;Salary;2.000,00;2.100,00\n21/01/2026;Coffee;-1,50;n/a\n",
	} {
		sample, err := domain.ReadSample(balanceMapping(), []byte(file), currency(t, "EUR"))
		if err != nil {
			t.Fatal(err)
		}

		if sample.FileBalance != nil || sample.FileBalanceProblem == "" {
			t.Errorf("%s: file balance = %+v, problem %q, want only a problem", name, sample.FileBalance, sample.FileBalanceProblem)
		}
	}
}

func TestASampleWithoutABalanceColumnHasNoFileBalance(t *testing.T) {
	t.Parallel()

	sample, err := domain.ReadSample(signedMapping(), []byte("Date;Description;Amount\n20/01/2026;Coffee;-1,50\n"),
		currency(t, "EUR"))
	if err != nil {
		t.Fatal(err)
	}

	if sample.FileBalance != nil || sample.FileBalanceProblem != "" {
		t.Errorf("file balance = %+v, problem %q, want neither", sample.FileBalance, sample.FileBalanceProblem)
	}
}
