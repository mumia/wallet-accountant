package domain_test

import (
	"testing"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
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

func TestCreatingARootCategory(t *testing.T) {
	t.Parallel()

	decidertest.Given(t, domain.Category).
		When(domain.CreateCategory{Name: " Food ", Kind: domain.KindExpense, Siblings: []string{"Salary", "Uncategorised"}}).
		Then(domain.CategoryCreated{Name: "Food", Kind: domain.KindExpense})
}

var (
	food        = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-000000000001")
	eatingOut   = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-000000000002")
	salary      = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-000000000003")
	uncategoris = sharedkernel.MustParseID("019a8b2c-0000-7000-8000-000000000004")
)

// Food › Eating out, an Expense branch.
var underEatingOut = &domain.Parent{ID: eatingOut, Kind: domain.KindExpense, Path: []sharedkernel.ID{food, eatingOut}}

func TestASubCategoryInheritsItsParentsKind(t *testing.T) {
	t.Parallel()

	for _, kind := range []domain.Kind{"", domain.KindExpense} {
		t.Run("kind "+string(kind), func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Category).
				When(domain.CreateCategory{Name: "Coffee", Kind: kind, Parent: underEatingOut, Siblings: []string{"Restaurants"}}).
				Then(domain.CategoryCreated{Name: "Coffee", Kind: domain.KindExpense, ParentID: eatingOut})
		})
	}
}

func TestNestingRefusals(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		cmd  domain.CreateCategory
		want error
	}{
		{
			"Income under an Expense parent",
			domain.CreateCategory{Name: "Refunds", Kind: domain.KindIncome, Parent: underEatingOut},
			domain.ErrCategoryKindMismatch,
		},
		{
			"under an Internal Category",
			domain.CreateCategory{Name: "Later", Parent: &domain.Parent{
				ID: uncategoris, Kind: domain.KindInternal, Path: []sharedkernel.ID{uncategoris},
			}},
			domain.ErrInternalCategory,
		},
		{
			"name taken under the parent",
			domain.CreateCategory{Name: "COFFEE", Parent: underEatingOut, Siblings: []string{"Coffee"}},
			domain.ErrCategoryNameTaken,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Category).When(tc.cmd).ThenError(tc.want)
		})
	}
}

func TestCreateCategoryRefusals(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		cmd  domain.CreateCategory
		want error
	}{
		{"blank name", domain.CreateCategory{Name: "  ", Kind: domain.KindExpense}, domain.ErrCategoryNameRequired},
		{"root without a kind", domain.CreateCategory{Name: "Food"}, domain.ErrCategoryKindRequired},
		{"unknown kind", domain.CreateCategory{Name: "Food", Kind: "equity"}, domain.ErrCategoryKindRequired},
		{"Internal kind", domain.CreateCategory{Name: "Equity", Kind: domain.KindInternal}, domain.ErrInternalCategory},
		{"name taken by a sibling", domain.CreateCategory{Name: "Food", Kind: domain.KindExpense, Siblings: []string{"Salary", "Food"}}, domain.ErrCategoryNameTaken},
		{"name taken, ignoring case and spaces", domain.CreateCategory{Name: " food ", Kind: domain.KindExpense, Siblings: []string{"Food"}}, domain.ErrCategoryNameTaken},
		{"name taken by an Internal root", domain.CreateCategory{Name: "Uncategorised", Kind: domain.KindExpense, Siblings: []string{"Uncategorised"}}, domain.ErrCategoryNameTaken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Category).When(tc.cmd).ThenError(tc.want)
		})
	}

	t.Run("already created", func(t *testing.T) {
		t.Parallel()

		decidertest.Given(t, domain.Category, domain.CategoryEvent(domain.CategoryCreated{Name: "Food", Kind: domain.KindExpense})).
			When(domain.CreateCategory{Name: "Food", Kind: domain.KindExpense}).
			ThenError(domain.ErrCategoryAlreadyCreated)
	})
}

func ptr[T any](v T) *T { return &v }

// coffee is Food › Eating out › Coffee.
func coffee(t *testing.T) *decidertest.Scenario[domain.CategoryState, domain.CategoryCommand, domain.CategoryEvent] {
	t.Helper()

	return decidertest.Given(t, domain.Category, domain.CategoryEvent(domain.CategoryCreated{
		Name: "Coffee", Kind: domain.KindExpense, ParentID: eatingOut,
	}))
}

func TestRenamingACategory(t *testing.T) {
	t.Parallel()

	coffee(t).
		When(domain.ChangeCategory{Name: ptr(" Tea "), Siblings: []string{"Restaurants"}}).
		Then(domain.CategoryRenamed{Name: "Tea"})
}

func TestChangingOnlyTheCaseOfTheNameIsARename(t *testing.T) {
	t.Parallel()

	coffee(t).
		When(domain.ChangeCategory{Name: ptr("coffee")}).
		Then(domain.CategoryRenamed{Name: "coffee"})
}

func TestRenamingToTheSameNameRecordsNothing(t *testing.T) {
	t.Parallel()

	coffee(t).When(domain.ChangeCategory{Name: ptr("Coffee")}).Then()
}

func TestMovingACategory(t *testing.T) {
	t.Parallel()

	t.Run("under another parent of its kind", func(t *testing.T) {
		t.Parallel()

		coffee(t).
			When(domain.ChangeCategory{Move: true, Parent: &domain.Parent{
				ID: food, Kind: domain.KindExpense, Path: []sharedkernel.ID{food},
			}, Siblings: []string{"Eating out", "Groceries"}}).
			Then(domain.CategoryMoved{ParentID: food})
	})

	t.Run("to the root", func(t *testing.T) {
		t.Parallel()

		coffee(t).
			When(domain.ChangeCategory{Move: true, Siblings: []string{"Food", "Uncategorised"}}).
			Then(domain.CategoryMoved{})
	})

	t.Run("to where it already is", func(t *testing.T) {
		t.Parallel()

		coffee(t).When(domain.ChangeCategory{Move: true, Parent: underEatingOut}).Then()
	})

	t.Run("and renaming it at once", func(t *testing.T) {
		t.Parallel()

		// The old name is taken in the new place, the new one is not.
		coffee(t).
			When(domain.ChangeCategory{Name: ptr("Espresso"), Move: true, Siblings: []string{"Coffee"}}).
			Then(domain.CategoryRenamed{Name: "Espresso"}, domain.CategoryMoved{})
	})
}

func TestChangeCategoryRefusals(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		cmd  domain.ChangeCategory
		want error
	}{
		{"blank name", domain.ChangeCategory{Name: ptr(" ")}, domain.ErrCategoryNameRequired},
		{"renamed to a sibling's name", domain.ChangeCategory{Name: ptr("restaurants"), Siblings: []string{"Restaurants"}}, domain.ErrCategoryNameTaken},
		{"moved where its name is taken", domain.ChangeCategory{Move: true, Siblings: []string{"Coffee"}}, domain.ErrCategoryNameTaken},
		{"moved under an Income parent", domain.ChangeCategory{Move: true, Parent: &domain.Parent{
			ID: salary, Kind: domain.KindIncome, Path: []sharedkernel.ID{salary},
		}}, domain.ErrCategoryKindMismatch},
		{"moved under an Internal Category", domain.ChangeCategory{Move: true, Parent: &domain.Parent{
			ID: uncategoris, Kind: domain.KindInternal, Path: []sharedkernel.ID{uncategoris},
		}}, domain.ErrInternalCategory},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			coffee(t).When(tc.cmd).ThenError(tc.want)
		})
	}

	t.Run("moved under itself or below it", func(t *testing.T) {
		t.Parallel()

		s := coffee(t)
		self := s.State().ID

		s.When(domain.ChangeCategory{Move: true, Parent: &domain.Parent{
			ID: self, Kind: domain.KindExpense, Path: []sharedkernel.ID{food, eatingOut, self},
		}}).ThenError(domain.ErrCategoryCycle)

		below := sharedkernel.MustParseID("019a8b2c-0000-7000-8000-000000000009")
		s.When(domain.ChangeCategory{Move: true, Parent: &domain.Parent{
			ID: below, Kind: domain.KindExpense, Path: []sharedkernel.ID{food, eatingOut, self, below},
		}}).ThenError(domain.ErrCategoryCycle)
	})

	t.Run("not created", func(t *testing.T) {
		t.Parallel()

		decidertest.Given(t, domain.Category).
			When(domain.ChangeCategory{Name: ptr("Tea")}).
			ThenError(domain.ErrCategoryNotFound)
	})
}

func TestInternalCategoriesCanNotBeRenamedOrMoved(t *testing.T) {
	t.Parallel()

	for name, cmd := range map[string]domain.ChangeCategory{
		"renamed": {Name: ptr("Suspense")},
		"moved":   {Move: true, Parent: underEatingOut},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Category, domain.CategoryEvent(domain.CategoryCreated{
				Name: "Uncategorised", Kind: domain.KindInternal, InternalRole: domain.RoleUncategorised,
			})).When(cmd).ThenError(domain.ErrInternalCategory)
		})
	}
}
