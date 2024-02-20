package movementtypequery

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
	"walletaccountant/movementtypereadmodel"
)

var _ QueryMediatorer = &QueryMediator{}

type QueryMediatorer interface {
	MovementType(ctx *gin.Context, accountId *movementtype.Id) (*movementtypereadmodel.Entity, *definitions.WalletAccountantError)
	MovementTypesByAccountId(ctx *gin.Context, accountId *account.Id) ([]*movementtypereadmodel.Entity, *definitions.WalletAccountantError)
	MovementTypes(ctx *gin.Context) ([]*movementtypereadmodel.Entity, *definitions.WalletAccountantError)
}

type QueryMediator struct {
	repository movementtypereadmodel.ReadModeler
}

func NewQueryMediator(repository movementtypereadmodel.ReadModeler) *QueryMediator {
	return &QueryMediator{repository: repository}
}

func (mediator QueryMediator) MovementType(ctx *gin.Context, movementTypeId *movementtype.Id) (*movementtypereadmodel.Entity, *definitions.WalletAccountantError) {
	entity, err := mediator.repository.GetByMovementTypeId(ctx, movementTypeId)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	if entity == nil {
		return nil, movementtype.NonExistentMovementTypeError(movementTypeId.String())
	}

	return entity, nil
}

func (mediator QueryMediator) MovementTypesByAccountId(
	ctx *gin.Context,
	accountId *account.Id,
) ([]*movementtypereadmodel.Entity, *definitions.WalletAccountantError) {
	entities, err := mediator.repository.GetByAccountId(ctx, accountId)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entities, nil
}

func (mediator QueryMediator) MovementTypes(ctx *gin.Context) ([]*movementtypereadmodel.Entity, *definitions.WalletAccountantError) {
	entities, err := mediator.repository.GetAll(ctx)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entities, nil
}
