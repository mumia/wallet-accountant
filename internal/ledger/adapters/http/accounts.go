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

// Problem types of adding an Account and of its Opening count.
const (
	// InvalidAccountType is for an Account the domain refuses to add or
	// rename, or an Opening count it refuses.
	InvalidAccountType = "/problems/invalid-account"
	// OpeningCountMismatchType is for a cash Account's two Opening counts
	// that differ.
	OpeningCountMismatchType = "/problems/opening-count-mismatch"
	// OpeningCountLockedType is for recounting once the start month closed.
	OpeningCountLockedType = "/problems/opening-count-locked"
	// NotACashAccountType is for a command only a cash Account takes.
	NotACashAccountType = "/problems/not-a-cash-account"
)

// The API's Account kinds and the domain's.
var (
	accountKindsIn = map[oas.AccountKind]domain.AccountKind{
		"BANK_ACCOUNT":  domain.AccountKindBankAccount,
		"CREDIT_CARD":   domain.AccountKindCreditCard,
		"LOAN":          domain.AccountKindLoan,
		"CRYPTO_WALLET": domain.AccountKindCryptoWallet,
		"CASH":          domain.AccountKindCash,
	}
	accountKindsOut = map[domain.AccountKind]oas.AccountKind{
		domain.AccountKindBankAccount:  "BANK_ACCOUNT",
		domain.AccountKindCreditCard:   "CREDIT_CARD",
		domain.AccountKindLoan:         "LOAN",
		domain.AccountKindCryptoWallet: "CRYPTO_WALLET",
		domain.AccountKindCash:         "CASH",
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
		OpeningCount:             countOf(req.OpeningCount),
		OpeningCountConfirmation: countOf(req.OpeningCountConfirmation),
	})

	invalid := func(detail string) (oas.AddAccountRes, error) {
		p := httpapi.Problem(http.StatusUnprocessableEntity, InvalidAccountType, "Invalid Account", detail)

		return new(oas.AddAccountUnprocessableEntity(p)), nil
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound):
		return new(oas.AddAccountNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, domain.ErrAccountNameRequired):
		return invalid(accountNameRequiredDetail)
	case errors.Is(err, domain.ErrUnknownAccountKind):
		return invalid("The kind must be BANK_ACCOUNT, CREDIT_CARD, LOAN, CRYPTO_WALLET or CASH.")
	case errors.Is(err, app.ErrOpeningCountMismatch):
		p := httpapi.Problem(http.StatusUnprocessableEntity, OpeningCountMismatchType, "Opening counts differ",
			openingCountMismatchDetail)

		return new(oas.AddAccountUnprocessableEntity(p)), nil
	case errors.Is(err, app.ErrOpeningCountRequired):
		return invalid("A cash Account needs opening_count and opening_count_confirmation.")
	case errors.Is(err, app.ErrOpeningCountNotForStatementAccount):
		return invalid("Only a cash Account takes an Opening count: a statement Account's first Statement import " +
			"works out its Opening balance.")
	case errors.Is(err, app.ErrCurrencyMismatch):
		return invalid("The Opening count must be in the Account currency.")
	case errors.Is(err, domain.ErrTooManyDecimalPlaces):
		return invalid("The Opening count has more decimal places than the Account currency.")
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

// ChangeAccount implements changeAccount: it renames the Account.
func (h *AccountHandler) ChangeAccount(ctx context.Context, req *oas.AccountChange, params oas.ChangeAccountParams,
) (oas.ChangeAccountRes, error) {
	renamed, err := h.accounts.Rename(ctx, params.HouseholdID, params.AccountID, req.Name)

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.ChangeAccountNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, domain.ErrAccountNameRequired):
		p := httpapi.Problem(http.StatusUnprocessableEntity, InvalidAccountType, "Invalid Account", accountNameRequiredDetail)

		return new(oas.ChangeAccountUnprocessableEntity(p)), nil
	case err != nil:
		return nil, fmt.Errorf("rename account: %w", err)
	}

	return accountResponse(renamed)
}

// RecountOpening implements recountOpening: it replaces a cash Account's
// Opening count while its start month is open.
func (h *AccountHandler) RecountOpening(ctx context.Context, req *oas.OpeningRecount,
	params oas.RecountOpeningParams,
) (oas.RecountOpeningRes, error) {
	recounted, err := h.accounts.RecountOpening(ctx, params.HouseholdID, params.AccountID,
		*countOf(oas.NewOptMoney(req.OpeningCount)), *countOf(oas.NewOptMoney(req.OpeningCountConfirmation)))

	invalid := func(typ, title, detail string) (oas.RecountOpeningRes, error) {
		p := httpapi.Problem(http.StatusUnprocessableEntity, typ, title, detail)

		return new(oas.RecountOpeningUnprocessableEntity(p)), nil
	}

	switch {
	case errors.Is(err, app.ErrHouseholdNotFound), errors.Is(err, app.ErrAccountNotFound):
		return new(oas.RecountOpeningNotFound(httpapi.NotFoundProblem())), nil
	case errors.Is(err, app.ErrNotACashAccount):
		return invalid(NotACashAccountType, "Not a cash Account",
			"Only a cash Account has an Opening count: a statement Account's Opening balance comes from its first "+
				"Statement import.")
	case errors.Is(err, app.ErrOpeningCountMismatch):
		return invalid(OpeningCountMismatchType, "Opening counts differ", openingCountMismatchDetail)
	case errors.Is(err, app.ErrCurrencyMismatch):
		return invalid(InvalidAccountType, "Invalid Account", "The Opening count must be in the Account currency.")
	case errors.Is(err, domain.ErrTooManyDecimalPlaces):
		return invalid(InvalidAccountType, "Invalid Account",
			"The Opening count has more decimal places than the Account currency.")
	case errors.Is(err, app.ErrOpeningCountLocked):
		p := httpapi.Problem(http.StatusConflict, OpeningCountLockedType, "Opening count locked",
			"The start date's month has closed, so the Opening count can't change. Reopen back to it first.")

		return new(oas.RecountOpeningConflict(p)), nil
	case err != nil:
		return nil, fmt.Errorf("recount opening: %w", err)
	}

	return accountResponse(recounted)
}

// countOf is an Opening count as the Ledger takes it; nil when absent.
func countOf(m oas.OptMoney) *app.Count {
	v, ok := m.Get()
	if !ok {
		return nil
	}

	return &app.Count{Amount: v.Amount.Value, Currency: string(v.Currency)}
}

// Details shared by the Account operations.
const (
	accountNameRequiredDetail  = "The name must not be blank."
	openingCountMismatchDetail = "The Opening count and its confirmation differ: count the cash again."
)

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
		OpenMonth:                    a.OpenMonth,
		Pending:                      int32(a.Pending),             //nolint:gosec // a count of rows
		SuspectedDuplicates:          int32(a.SuspectedDuplicates), //nolint:gosec // a count of rows
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
