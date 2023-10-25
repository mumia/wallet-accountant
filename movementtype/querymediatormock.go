package movementtype

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediatorMock{}

type QueryMediatorMock struct {
	MovementTypeFn  func(ctx *gin.Context, movementTypeId *Id) (*Entity, *definitions.WalletAccountantError)
	MovementTypesFn func(ctx *gin.Context) ([]*Entity, *definitions.WalletAccountantError)
}

func (mock *QueryMediatorMock) MovementType(
	ctx *gin.Context,
	movementTypeId *Id,
) (*Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.MovementTypeFn != nil {
		return mock.MovementTypeFn(ctx, movementTypeId)
	}

	return nil, nil
}

func (mock *QueryMediatorMock) MovementTypes(ctx *gin.Context) ([]*Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.MovementTypesFn != nil {
		return mock.MovementTypesFn(ctx)
	}

	return nil, nil
}
