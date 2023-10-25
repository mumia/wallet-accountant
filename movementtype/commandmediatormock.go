package movementtype

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/definitions"
)

var _ CommandMediatorer = &CommandMediatorMock{}

type CommandMediatorMock struct {
	RegisterNewMovementTypeFn func(
		ctx *gin.Context,
		transferObject RegisterNewMovementTypeTransferObject,
	) (*Id, *definitions.WalletAccountantError)
}

func (mock *CommandMediatorMock) RegisterNewMovementType(
	ctx *gin.Context,
	transferObject RegisterNewMovementTypeTransferObject,
) (*Id, *definitions.WalletAccountantError) {
	if mock != nil && mock.RegisterNewMovementTypeFn != nil {
		return mock.RegisterNewMovementTypeFn(ctx, transferObject)
	}

	return nil, nil
}
