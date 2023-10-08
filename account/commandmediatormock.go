package account

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/definitions"
)

var _ CommandMediatorer = &CommandMediatorMock{}

type CommandMediatorMock struct {
	RegisterNewAccountFn func(
		ctx *gin.Context,
		transferObject RegisterNewAccountTransferObject,
	) (*Id, *definitions.WalletAccountantError)
	StartNextMonthFn func(ctx *gin.Context, accountId *Id) *definitions.WalletAccountantError
}

func (mock *CommandMediatorMock) RegisterNewAccount(
	ctx *gin.Context,
	transferObject RegisterNewAccountTransferObject,
) (*Id, *definitions.WalletAccountantError) {
	if mock != nil && mock.RegisterNewAccountFn != nil {
		return mock.RegisterNewAccountFn(ctx, transferObject)
	}

	return nil, nil
}

func (mock *CommandMediatorMock) StartNextMonth(ctx *gin.Context, accountId *Id) *definitions.WalletAccountantError {
	if mock != nil && mock.StartNextMonthFn != nil {
		return mock.StartNextMonthFn(ctx, accountId)
	}

	return nil
}
