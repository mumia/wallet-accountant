package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

func TestSeedingEachInternalCategory(t *testing.T) {
	t.Parallel()

	for role, name := range map[domain.InternalRole]string{
		domain.RoleOpeningBalance:    "Opening balance",
		domain.RoleHouseholdTransfer: "Household transfer",
		domain.RoleUncategorised:     "Uncategorised",
	} {
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Category).
				When(domain.SeedInternalCategory{Role: role}).
				Then(domain.CategoryCreated{Name: name, Kind: domain.KindInternal, InternalRole: role})
		})
	}
}

func TestThereAreExactlyThreeInternalCategories(t *testing.T) {
	t.Parallel()

	if got := domain.InternalRoles(); len(got) != 3 {
		t.Errorf("InternalRoles() = %v, want Opening balance, Household transfer and Uncategorised", got)
	}
}

func TestSeedingRefusals(t *testing.T) {
	t.Parallel()

	t.Run("already created", func(t *testing.T) {
		t.Parallel()

		decidertest.Given(t, domain.Category, domain.CategoryEvent(domain.CategoryCreated{
			Name: "Uncategorised", Kind: domain.KindInternal, InternalRole: domain.RoleUncategorised,
		})).
			When(domain.SeedInternalCategory{Role: domain.RoleUncategorised}).
			ThenError(domain.ErrCategoryAlreadyCreated)
	})

	t.Run("unknown role", func(t *testing.T) {
		t.Parallel()

		decidertest.Given(t, domain.Category).
			When(domain.SeedInternalCategory{Role: "equity"}).
			ThenError(domain.ErrUnknownInternalRole)
	})
}
