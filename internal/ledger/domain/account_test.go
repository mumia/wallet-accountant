package domain_test

import (
	"testing"
	"time"

	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
	"github.com/mumia/wallet-accountant/internal/sharedkernel/decider/decidertest"
)

var startDate = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

func TestAddingAnAccountOfEachKind(t *testing.T) {
	t.Parallel()

	for _, kind := range []domain.AccountKind{
		domain.AccountKindBankAccount,
		domain.AccountKindCreditCard,
		domain.AccountKindLoan,
		domain.AccountKindCryptoWallet,
	} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()

			decidertest.Given(t, domain.Account).
				When(domain.AddAccount{Name: "  Main  ", Kind: kind, Currency: "EUR", StartDate: startDate}).
				Then(domain.AccountAdded{Name: "Main", Kind: kind, Currency: "EUR", StartDate: "2026-01-01"})
		})
	}
}

func TestANewAccountAwaitsItsFirstStatementImport(t *testing.T) {
	t.Parallel()

	s := decidertest.Given(t, domain.Account, domain.AccountEvent(domain.AccountAdded{
		Name: "Main", Kind: domain.AccountKindBankAccount, Currency: "BTC", StartDate: "2026-01-01",
	})).State()

	if !s.Added || s.Name != "Main" || s.Kind != domain.AccountKindBankAccount || s.Currency != "BTC" ||
		!s.StartDate.Equal(startDate) || !s.AwaitingFirstStatementImport {
		t.Errorf("state = %+v", s)
	}

	if s.HasBalance() || len(s.Months) != 0 {
		t.Errorf("a new Account has a balance: %+v", s)
	}
}

func TestAddingAnAccountRefusals(t *testing.T) {
	t.Parallel()

	valid := domain.AddAccount{Name: "Main", Kind: domain.AccountKindBankAccount, Currency: "EUR", StartDate: startDate}

	for name, tc := range map[string]struct {
		mutate func(*domain.AddAccount)
		want   error
	}{
		"blank name":       {func(c *domain.AddAccount) { c.Name = "  " }, domain.ErrAccountNameRequired},
		"cash kind":        {func(c *domain.AddAccount) { c.Kind = "cash" }, domain.ErrCashAccountsNotSupported},
		"unknown kind":     {func(c *domain.AddAccount) { c.Kind = "savings_jar" }, domain.ErrUnknownAccountKind},
		"unknown currency": {func(c *domain.AddAccount) { c.Currency = "XYZ" }, sharedkernel.ErrUnknownCurrency},
		"no start date":    {func(c *domain.AddAccount) { c.StartDate = time.Time{} }, domain.ErrStartDateRequired},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := valid
			tc.mutate(&cmd)

			decidertest.Given(t, domain.Account).When(cmd).ThenError(tc.want)
		})
	}

	t.Run("already added", func(t *testing.T) {
		t.Parallel()

		decidertest.Given(t, domain.Account, domain.AccountEvent(domain.AccountAdded{
			Name: "Main", Kind: domain.AccountKindBankAccount, Currency: "EUR", StartDate: "2026-01-01",
		})).
			When(valid).
			ThenError(domain.ErrAccountAlreadyAdded)
	})
}
