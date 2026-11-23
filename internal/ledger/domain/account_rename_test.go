package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

func added() domain.AccountEvent {
	return domain.AccountAdded{Name: "Main", Kind: domain.AccountKindBankAccount, Currency: "EUR", StartDate: "2026-01-01"}
}

func TestRenamingAnAccount(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Account, added()).
		When(domain.RenameAccount{Name: "  Joint  "}).
		Then(domain.AccountRenamed{Name: "Joint"})

	s := decidertest.Given(t, domain.Account, added(), domain.AccountEvent(domain.AccountRenamed{Name: "Joint"})).State()
	if s.Name != "Joint" || s.Kind != domain.AccountKindBankAccount || s.Currency != "EUR" ||
		!s.StartDate.Equal(startDate) || !s.AwaitingFirstStatementImport {
		t.Errorf("state = %+v, want only the name changed", s)
	}

	// Only the case changes: that is a new name.
	decidertest.Given(t, domain.Account, added()).
		When(domain.RenameAccount{Name: "MAIN"}).
		Then(domain.AccountRenamed{Name: "MAIN"})
}

func TestRenamingAnAccountToItsOwnNameRecordsNothing(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Account, added()).
		When(domain.RenameAccount{Name: " Main  "}).
		Then()
}

func TestRenamingAnAccountRefusals(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		past []domain.AccountEvent
		name string
		want error
	}{
		"not added":  {nil, "Joint", domain.ErrAccountNotAdded},
		"blank name": {[]domain.AccountEvent{added()}, "   ", domain.ErrAccountNameRequired},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Account, tc.past...).
				When(domain.RenameAccount{Name: tc.name}).
				ThenError(tc.want)
		})
	}
}
