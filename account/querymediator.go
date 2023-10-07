package account

import (
	"github.com/gin-gonic/gin"
)

var _ QueryMediatorer = &QueryMediator{}

type QueryMediatorer interface {
	Account(ctx *gin.Context, accountId *Id) (*Entity, error)
	Accounts(ctx *gin.Context) ([]*Entity, error)
}

type QueryMediator struct {
	repository ReadModeler
}

func NewQueryMediator(repository ReadModeler) *QueryMediator {
	return &QueryMediator{repository: repository}
}

func (mediator QueryMediator) Account(ctx *gin.Context, accountId *Id) (*Entity, error) {
	entity, err := mediator.repository.GetByAccountId(ctx, accountId)
	if err != nil {
		return nil, GenericError(err, nil)
	}

	return entity, nil
}

func (mediator QueryMediator) Accounts(ctx *gin.Context) ([]*Entity, error) {
	entities, err := mediator.repository.GetAll(ctx)
	if err != nil {
		return nil, GenericError(err, nil)
	}

	return entities, nil
}
