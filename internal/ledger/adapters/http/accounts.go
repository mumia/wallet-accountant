package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mumia/wallet-accountant/internal/ledger/app"
	"github.com/mumia/wallet-accountant/internal/ledger/domain"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/amount"
	"github.com/mumia/wallet-accountant/internal/platform/httpapi/oas"
	"github.com/mumia/wallet-accountant/internal/sharedkernel"
)

// InvalidAccountType is the problem type for an Account the domain refuses
// to add.
const InvalidAccountType = "/problems/invalid-account"

// The API's Account kinds and the domain's. CASH is listed so that it gets
// its own refusal rather than "unknown kind".
var (
	accountKindsIn = map[oas.AccountKind]domain.AccountKind{
		"BANK_ACCOUNT":  domain.AccountKindBankAccount,
		"CREDIT_CARD":   domain.AccountKindCreditCard,
		"LOAN":          domain.AccountKindLoan,
		"CRYPTO_WALLET": domain.AccountKindCryptoWallet,
		"CASH":          "cash",
	}
	accountKindsOut = map[domain.AccountKind]oas.AccountKind{
		domain.AccountKindBankAccount:  "BANK_ACCOUNT",
		domain.AccountKindCreditCard:   "CREDIT_CARD",
		domain.AccountKindLoan:         "LOAN",
		domain.AccountKindCryptoWallet: "CRYPTO_WALLET",
	}
)

// AccountHandler implements the Account operations of oas.Handler.
type AccountHandler struct {
	accounts *app.Accounts
}

// NewAccountHandler returns the Account operations of the Ledger HTTP adapter.
func NewAccountHandler(accounts *app.Accounts) *AccountHandler {
	return &AccountHandler{accounts: accounts}
}

// AddAccount implements addAccount.
func (h *AccountHandler) AddAccount(ctx context.Context, req *oas.AccountCreation, params oas.AddAccountParams) (oas.AddAccountRes, error) {
	kind, ok := accountKindsIn[req.Kind]
	if !ok {
		// Not an API kind, so the domain must refuse it as unknown, even when
		// it happens to spell a domain kind (such as "bank_account"). It
		// still goes through the request transaction, so a non-member gets
		// 404 rather than 422.
		kind = domain.AccountKind("api:" + string(req.Kind))
	}

	added, err := h.accounts.Add(ctx, params.HouseholdID, app.NewAccount{
		Name: req.Name, Kind: kind, Currency: string(req.Currency), StartDate: req.StartDate,
	})

	invalid := func(detail string) (oas.AddAccountRes, error) {
		p := httpapi.Problem(http.StatusUnprocessableEntity, InvalidAccountType, "Invalid Account", detail)

		return new(oas.AddAccountUnprocessableEntity(p)), nil
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound):
		return new(oas.AddAccountNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, domain.ErrAccountNameRequired):
		return invalid("The name must not be blank.")
	case errors.Is(err, domain.ErrCashAccountsNotSupported):
		return invalid("Cash Accounts are not supported yet.")
	case errors.Is(err, domain.ErrUnknownAccountKind):
		return invalid("The kind must be BANK_ACCOUNT, CREDIT_CARD, LOAN or CRYPTO_WALLET.")
	case errors.Is(err, sharedkernel.ErrUnknownCurrency):
		return invalid("The Account currency is not a known currency.")
	case errors.Is(err, domain.ErrStartDateRequired):
		return invalid("The start date is required.")
	case err != nil:
		return nil, fmt.Errorf("add account: %w", err)
	}

	return accountResponse(added)
}

// ListAccounts implements listAccounts.
func (h *AccountHandler) ListAccounts(ctx context.Context, params oas.ListAccountsParams) (oas.ListAccountsRes, error) {
	list, err := h.accounts.List(ctx, params.HouseholdID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound):
		return new(oas.ListAccountsNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	items := make([]oas.Account, 0, len(list))

	for _, a := range list {
		res, err := accountResponse(a)
		if err != nil {
			return nil, err
		}

		items = append(items, *res)
	}

	return &oas.AccountList{Items: items}, nil
}

// GetAccount implements getAccount.
func (h *AccountHandler) GetAccount(ctx context.Context, params oas.GetAccountParams) (oas.GetAccountRes, error) {
	got, err := h.accounts.Get(ctx, params.HouseholdID, params.AccountID)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.GetAccountNotFound(httpapi.NotFoundProblem())), nil
	case err != nil:
		return nil, fmt.Errorf("get account: %w", err)
	}

	return accountResponse(got)
}

// accountResponse leaves the balance out while the Account awaits its first
// Statement import: it has none until its Opening balance is recorded.
func accountResponse(a app.Account) (*oas.Account, error) {
	out := &oas.Account{
		AccountID:                    a.ID,
		Name:                         a.Name,
		Kind:                         accountKindsOut[a.Kind],
		Currency:                     oas.CurrencyCode(a.Currency),
		StartDate:                    a.StartDate,
		AwaitingFirstStatementImport: a.AwaitingFirstStatementImport,
	}

	if a.Balance != nil {
		balance, err := money(*a.Balance, a.Currency)
		if err != nil {
			return nil, err
		}

		out.Balance = oas.NewOptMoney(balance)
	}

	return out, nil
}

// money is an amount in the currency as the API writes it, with the
// currency's decimal places (ADR-0007).
func money(value sharedkernel.Decimal, currency string) (oas.Money, error) {
	a, err := amount.Of(value, currency)
	if err != nil {
		return oas.Money{}, fmt.Errorf("write %s %s: %w", value, currency, err)
	}

	return oas.Money{Amount: a, Currency: oas.CurrencyCode(currency)}, nil
}
