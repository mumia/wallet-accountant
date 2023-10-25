package movementtype

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediator{}

type QueryMediatorer interface {
	MovementType(ctx *gin.Context, accountId *Id) (*Entity, *definitions.WalletAccountantError)
	MovementTypes(ctx *gin.Context) ([]*Entity, *definitions.WalletAccountantError)
}

type QueryMediator struct {
	repository ReadModeler
}

func NewQueryMediator(repository ReadModeler) *QueryMediator {
	return &QueryMediator{repository: repository}
}

func (mediator QueryMediator) MovementType(ctx *gin.Context, movementTypeId *Id) (*Entity, *definitions.WalletAccountantError) {
	entity, err := mediator.repository.GetByMovementTypeId(ctx, movementTypeId)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	if entity == nil {
		return nil, NonExistentMovementTypeError(movementTypeId.String())
	}

	return entity, nil
}

func (mediator QueryMediator) MovementTypes(ctx *gin.Context) ([]*Entity, *definitions.WalletAccountantError) {
	entities, err := mediator.repository.GetAll(ctx)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entities, nil
}
