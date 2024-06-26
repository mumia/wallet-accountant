package movementtypequery

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
	"walletaccountant/movementtypereadmodel"
)

var _ QueryMediatorer = &QueryMediatorMock{}

type QueryMediatorMock struct {
	MovementTypeFn             func(ctx *gin.Context, movementTypeId *movementtype.Id) (*movementtypereadmodel.Entity, *definitions.WalletAccountantError)
	MovementTypesFn            func(ctx *gin.Context) ([]*movementtypereadmodel.Entity, *definitions.WalletAccountantError)
	MovementTypesByAccountIdFn func(
		ctx *gin.Context,
		accountId *account.Id,
	) ([]*movementtypereadmodel.Entity, *definitions.WalletAccountantError)
}

func (mock *QueryMediatorMock) MovementTypesByAccountId(
	ctx *gin.Context,
	accountId *account.Id,
) ([]*movementtypereadmodel.Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.MovementTypesByAccountIdFn != nil {
		return mock.MovementTypesByAccountIdFn(ctx, accountId)
	}

	return nil, nil
}

func (mock *QueryMediatorMock) MovementType(
	ctx *gin.Context,
	movementTypeId *movementtype.Id,
) (*movementtypereadmodel.Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.MovementTypeFn != nil {
		return mock.MovementTypeFn(ctx, movementTypeId)
	}

	return nil, nil
}

func (mock *QueryMediatorMock) MovementTypes(ctx *gin.Context) ([]*movementtypereadmodel.Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.MovementTypesFn != nil {
		return mock.MovementTypesFn(ctx)
	}

	return nil, nil
}
