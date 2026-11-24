package domain_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
)

func TestASampleShowsTheFirstTenRowsAndCountsThemAll(t *testing.T) {
	t.Parallel()

	var file strings.Builder

	file.WriteString("Date;Description;Amount\n")

	for day := 1; day <= 12; day++ {
		fmt.Fprintf(&file, "%02d/12/2025;Row %d;-%d,00\n", day, day, day)
	}

	// The Account's start date doesn't matter: December 2025 rows are shown.
	s, err := domain.ReadSample(signedMapping(), []byte(file.String()), currency(t, "EUR"))
	if err != nil {
		t.Fatalf("ReadSample: %v", err)
	}

	if s.Readable != 12 || len(s.Unreadable) != 0 {
		t.Errorf("sample counts %d readable and %v unreadable, want 12 and none", s.Readable, s.Unreadable)
	}

	if len(s.Rows) != domain.SampleRows {
		t.Fatalf("sample shows %d rows, want %d", len(s.Rows), domain.SampleRows)
	}

	if first, last := s.Rows[0], s.Rows[9]; first.Line != 2 || first.Description != "Row 1" ||
		last.Line != 11 || last.Description != "Row 10" || last.Amount.String() != "-10" {
		t.Errorf("sample rows run from %+v to %+v, want lines 2 to 11", first, last)
	}
}

func TestASampleShowsTheLastReadableRowsNotAlreadyShown(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		rows      int
		wantLines []int
	}{
		{rows: 5, wantLines: []int{}},
		{rows: 10, wantLines: []int{}},
		{rows: 15, wantLines: []int{12, 13, 14, 15, 16}},
		{rows: 30, wantLines: []int{22, 23, 24, 25, 26, 27, 28, 29, 30, 31}},
	} {
		t.Run(fmt.Sprintf("%d rows", tc.rows), func(t *testing.T) {
			t.Parallel()

			var file strings.Builder

			file.WriteString("Date;Description;Amount\n")

			for i := 1; i <= tc.rows; i++ {
				fmt.Fprintf(&file, "01/12/2025;Row %d;-%d,00\n", i, i)
			}

			// An unreadable line among the last rows isn't one of them.
			file.WriteString("Total;;-1,00\n")

			s, err := domain.ReadSample(signedMapping(), []byte(file.String()), currency(t, "EUR"))
			if err != nil {
				t.Fatalf("ReadSample: %v", err)
			}

			lines := make([]int, 0, len(s.LastRows))
			for _, r := range s.LastRows {
				lines = append(lines, r.Line)
			}

			if !reflect.DeepEqual(lines, tc.wantLines) {
				t.Errorf("last rows are lines %v, want %v", lines, tc.wantLines)
			}

			if s.Readable != tc.rows || len(s.Rows) != min(tc.rows, domain.SampleRows) || len(s.Unreadable) != 1 {
				t.Errorf("sample = %d rows of %d readable, %v unreadable", len(s.Rows), s.Readable, s.Unreadable)
			}
		})
	}
}

func TestASampleCollectsReadableRowsAndUnreadableLinesTogether(t *testing.T) {
	t.Parallel()

	s, err := domain.ReadSample(signedMapping(), []byte("Date;Description;Amount\n"+
		"01/02/2026;Coffee;-1,50\n"+
		"2026-02-02;Bad Booking date;-1,00\n"+
		"03/02/2026;Salary;2.000,00\n"+
		"04/02/2026;Too precise;-1,001\n"), currency(t, "EUR"))
	if err != nil {
		t.Fatalf("ReadSample: %v", err)
	}

	lines := make([]int, 0, len(s.Rows))
	for _, r := range s.Rows {
		lines = append(lines, r.Line)
	}

	if !reflect.DeepEqual(lines, []int{2, 4}) || s.Readable != 2 {
		t.Errorf("readable lines %v (count %d), want [2 4]", lines, s.Readable)
	}

	if len(s.Unreadable) != 2 || s.Unreadable[0].Line != 3 || s.Unreadable[1].Line != 5 ||
		!strings.Contains(s.Unreadable[1].Reason, "decimal places") {
		t.Errorf("unreadable = %+v, want lines 3 and 5, the last for its decimal places", s.Unreadable)
	}
}

func TestASampleOfAnUnreadableFile(t *testing.T) {
	t.Parallel()

	for name, file := range map[string]string{
		"every row":                 "Date;Description;Amount\nnot a date;Coffee;-1,50\n01/02/2026;No amount;\n",
		"a header missing a column": "Date;Text;Amount\n01/02/2026;Coffee;-1,50\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s, err := domain.ReadSample(signedMapping(), []byte(file), currency(t, "EUR"))
			if err != nil {
				t.Fatalf("ReadSample: %v", err)
			}

			if s.Readable != 0 || len(s.Rows) != 0 || len(s.Unreadable) == 0 {
				t.Errorf("sample = %+v, want no readable rows and the unreadable lines", s)
			}
		})
	}
}

func TestASampleWithNoRows(t *testing.T) {
	t.Parallel()

	for name, file := range map[string]string{"empty": "", "only a header": "Date;Description;Amount\n"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := domain.ReadSample(signedMapping(), []byte(file), currency(t, "EUR"))
			if !errors.Is(err, domain.ErrEmptyStatementFile) {
				t.Errorf("err = %v, want %v", err, domain.ErrEmptyStatementFile)
			}
		})
	}
}

func TestASampleWithAnInvalidMapping(t *testing.T) {
	t.Parallel()

	m := signedMapping()
	m.DateFormat = "later"

	_, err := domain.ReadSample(m, []byte("Date;Description;Amount\n01/02/2026;Coffee;-1,50\n"), currency(t, "EUR"))
	if !errors.Is(err, domain.ErrInvalidCSVMapping) {
		t.Errorf("err = %v, want %v", err, domain.ErrInvalidCSVMapping)
	}
}
