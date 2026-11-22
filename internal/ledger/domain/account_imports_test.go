package domain_test

import (
	"reflect"
	"testing"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
)

// Every month of an Account read at once agrees with each month read on
// its own: its counts, and the unchecked imports with rows in or before it
// (spec stories 17, 19).
func TestAccountImportsByMonth(t *testing.T) {
	t.Parallel()

	waiting := domain.UncheckedImport{StatementImportID: laterImport, Check: domain.ImportCheckWaiting}
	failed := domain.UncheckedImport{StatementImportID: thirdImport, Check: domain.ImportCheckFailed}

	a := domain.AccountImports{
		Months: map[string]domain.MonthImports{
			"2026-02": {Pending: 2, SuspectedDuplicates: 1},
			"2026-05": {Pending: 1},
		},
		Unchecked: []domain.UncheckedImportFrom{
			{UncheckedImport: waiting, From: "2026-02"},
			{UncheckedImport: failed, From: "2026-03"},
		},
	}

	if got := a.LatestMonth(); got != "2026-05" {
		t.Errorf("latest month = %q, want 2026-05", got)
	}

	for month, want := range map[string]domain.MonthImports{
		"2026-01": {},
		"2026-02": {Pending: 2, SuspectedDuplicates: 1, UncheckedImports: []domain.UncheckedImport{waiting}},
		"2026-03": {UncheckedImports: []domain.UncheckedImport{waiting, failed}},
		"2026-05": {Pending: 1, UncheckedImports: []domain.UncheckedImport{waiting, failed}},
	} {
		if got := a.Month(month); !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %+v, want %+v", month, got, want)
		}
	}

	if got := (domain.AccountImports{}).LatestMonth(); got != "" {
		t.Errorf("latest month of none = %q, want none", got)
	}

	if got := domain.PreviousMonth("2026-01"); got != "2025-12" {
		t.Errorf("month before January 2026 = %q, want 2025-12", got)
	}
}
