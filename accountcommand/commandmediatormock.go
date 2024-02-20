package accountcommand

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/account"
	"walletaccountant/definitions"
)

var _ CommandMediatorer = &CommandMediatorMock{}

type CommandMediatorMock struct {
	RegisterNewAccountFn func(
		ctx *gin.Context,
		transferObject RegisterNewAccountTransferObject,
	) (*account.Id, *definitions.WalletAccountantError)
	StartNextMonthFn func(ctx *gin.Context, accountId *account.Id) *definitions.WalletAccountantError
}

func (mock *CommandMediatorMock) RegisterNewAccount(
	ctx *gin.Context,
	transferObject RegisterNewAccountTransferObject,
) (*account.Id, *definitions.WalletAccountantError) {
	if mock != nil && mock.RegisterNewAccountFn != nil {
		return mock.RegisterNewAccountFn(ctx, transferObject)
	}

	return nil, nil
}

func (mock *CommandMediatorMock) StartNextMonth(ctx *gin.Context, accountId *account.Id) *definitions.WalletAccountantError {
	if mock != nil && mock.StartNextMonthFn != nil {
		return mock.StartNextMonthFn(ctx, accountId)
	}

	return nil
}
