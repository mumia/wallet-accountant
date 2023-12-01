package accountmonth

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/definitions"
)

var _ CommandMediatorer = &CommandMediator{}

type CommandMediatorMock struct {
	RegisterNewAccountMovementFn func(
		ctx *gin.Context,
		transferObject RegisterNewAccountMovementTransferObject,
	) *definitions.WalletAccountantError
	EndAccountMonthFn func(
		ctx *gin.Context,
		transferObject EndAccountMonthTransferObject,
	) *definitions.WalletAccountantError
}

func (mock *CommandMediatorMock) RegisterNewAccountMovement(
	ctx *gin.Context,
	transferObject RegisterNewAccountMovementTransferObject,
) *definitions.WalletAccountantError {
	if mock != nil && mock.RegisterNewAccountMovementFn != nil {
		return mock.RegisterNewAccountMovementFn(ctx, transferObject)
	}

	return nil
}

func (mock *CommandMediatorMock) EndAccountMonth(
	ctx *gin.Context,
	transferObject EndAccountMonthTransferObject,
) *definitions.WalletAccountantError {
	if mock != nil && mock.EndAccountMonthFn != nil {
		return mock.EndAccountMonthFn(ctx, transferObject)
	}

	return nil
}
