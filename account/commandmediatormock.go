package account

import (
	"github.com/gin-gonic/gin"
)

var _ CommandMediatorer = &CommandMediatorMock{}

type CommandMediatorMock struct {
	RegisterNewAccountFn func(
		ctx *gin.Context,
		transferObject RegisterNewAccountTransferObject,
	) (*Id, error)
	StartNextMonthFn func(ctx *gin.Context, accountId *Id) error
}

func (mock *CommandMediatorMock) RegisterNewAccount(
	ctx *gin.Context,
	transferObject RegisterNewAccountTransferObject,
) (*Id, error) {
	if mock != nil && mock.RegisterNewAccountFn != nil {
		return mock.RegisterNewAccountFn(ctx, transferObject)
	}

	return nil, nil
}

func (mock *CommandMediatorMock) StartNextMonth(ctx *gin.Context, accountId *Id) error {
	if mock != nil && mock.StartNextMonthFn != nil {
		return mock.StartNextMonthFn(ctx, accountId)
	}

	return nil
}
