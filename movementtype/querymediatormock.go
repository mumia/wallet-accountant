package movementtype

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/account"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediatorMock{}

type QueryMediatorMock struct {
	MovementTypeFn             func(ctx *gin.Context, movementTypeId *Id) (*Entity, *definitions.WalletAccountantError)
	MovementTypesFn            func(ctx *gin.Context) ([]*Entity, *definitions.WalletAccountantError)
	MovementTypesByAccountIdFn func(
		ctx *gin.Context,
		accountId *account.Id,
	) ([]*Entity, *definitions.WalletAccountantError)
}

func (mock *QueryMediatorMock) MovementTypesByAccountId(
	ctx *gin.Context,
	accountId *account.Id,
) ([]*Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.MovementTypesByAccountIdFn != nil {
		return mock.MovementTypesByAccountIdFn(ctx, accountId)
	}

	return nil, nil
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
