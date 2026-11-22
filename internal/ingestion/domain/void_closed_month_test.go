package domain_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mumia/wallet-accountant/internal/ingestion/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

// recordedWith is an import recorded by c with its earliest Booking month.
func recordedWith(t *testing.T, c domain.RecordStatementImport, earliest string) []domain.StatementImportEvent {
	t.Helper()

	c.EarliestBookingMonth = earliest

	events, err := domain.StatementImport.Decide(domain.StatementImport.Initial, c)
	if err != nil {
		t.Fatal(err)
	}

	return events
}

// voidIn is a Void of the Account's latest import with nothing Accepted,
// the start date in January and the Open month given.
func voidIn(openMonth string) domain.VoidStatementImport {
	return domain.VoidStatementImport{Latest: true, OpenMonth: openMonth, StartMonth: "2026-01"}
}

// An import touches a closed month when the earliest Booking month of the
// rows it recorded is before the Open month, or when it is the first
// import and the start date's month is; Void then refuses, naming the
// earliest such month (spec story 14).
func TestVoidingAnImportThatTouchesAClosedMonthIsRefused(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		past      []domain.StatementImportEvent
		openMonth string
		touched   string
	}{
		{"later import, rows in a closed month", recordedWith(t, laterImport(), "2026-02"), "2026-03", "2026-02"},
		{"later import, rows from the Open month", recordedWith(t, laterImport(), "2026-03"), "2026-03", ""},
		{"later import with no rows", recordedWith(t, laterImport(), ""), "2026-03", ""},
		{"first import, start month closed", recordedWith(t, firstImport(), "2026-02"), "2026-02", "2026-01"},
		{"first import, rows earlier than its start month's close", recordedWith(t, firstImport(), "2026-01"), "2026-03", "2026-01"},
		{"first import, start month open", recordedWith(t, firstImport(), "2026-01"), "2026-01", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := decidertest.Given(t, domain.StatementImport, tc.past...).State()
			if got := s.ClosedMonthTouched(tc.openMonth, "2026-01"); got != tc.touched {
				t.Errorf("closed month touched = %q, want %q", got, tc.touched)
			}

			if tc.touched == "" {
				decidertest.Given(t, domain.StatementImport, tc.past...).
					When(voidIn(tc.openMonth)).
					Then(domain.StatementImportVoided{})

				return
			}

			_, err := domain.StatementImport.Decide(s, voidIn(tc.openMonth))

			want := &domain.TouchesClosedMonthError{Month: tc.touched, OpenMonth: tc.openMonth}
			if got, ok := errors.AsType[*domain.TouchesClosedMonthError](err); !ok || !reflect.DeepEqual(got, want) {
				t.Errorf("error = %v, want %v", err, want)
			}

			if !errors.Is(err, domain.ErrStatementImportTouchesClosedMonth) {
				t.Errorf("error = %v, want it to be ErrStatementImportTouchesClosedMonth", err)
			}
		})
	}
}

// The closed month is refused after the existing refusals (spec "Void").
func TestATouchedClosedMonthIsRefusedLast(t *testing.T) {
	t.Parallel()

	past := recordedWith(t, laterImport(), "2026-01")

	notLatest := voidIn("2026-03")
	notLatest.Latest = false
	decidertest.Given(t, domain.StatementImport, past...).When(notLatest).
		ThenError(domain.ErrNotTheLatestStatementImport)

	accepted := voidIn("2026-03")
	accepted.AcceptedRows = 2
	decidertest.Given(t, domain.StatementImport, past...).When(accepted).
		ThenError(domain.ErrStatementImportHasAcceptedRows)

	decidertest.Given(t, domain.StatementImport, append(past, domain.StatementImportVoided{})...).
		When(voidIn("2026-03")).
		ThenError(domain.ErrStatementImportAlreadyVoided)
}

// void_blocked_by lists every reason at once, in the order they are
// checked, and none for a voided import (spec story 15).
func TestVoidBlockedByListsEveryReason(t *testing.T) {
	t.Parallel()

	s := decidertest.Given(t, domain.StatementImport, recordedWith(t, laterImport(), "2026-01")...).State()

	all := domain.VoidStatementImport{AcceptedRows: 1, OpenMonth: "2026-02", StartMonth: "2026-01"}
	want := []domain.VoidBlock{
		domain.VoidBlockedNotLatest, domain.VoidBlockedHasAcceptedRows, domain.VoidBlockedTouchesClosedMonth,
	}

	if got := s.VoidBlockedBy(all); !reflect.DeepEqual(got, want) {
		t.Errorf("void blocked by = %v, want %v", got, want)
	}

	if got := s.VoidBlockedBy(voidIn("2026-01")); len(got) != 0 {
		t.Errorf("void blocked by = %v, want none", got)
	}

	voided := decidertest.Given(t, domain.StatementImport,
		append(recordedWith(t, laterImport(), "2026-01"), domain.StatementImportVoided{})...).State()
	if got := voided.VoidBlockedBy(all); len(got) != 0 {
		t.Errorf("voided import blocked by = %v, want none", got)
	}
}
