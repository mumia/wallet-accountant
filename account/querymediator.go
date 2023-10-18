package account

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediator{}

type QueryMediatorer interface {
	Account(ctx *gin.Context, accountId *Id) (*Entity, *definitions.WalletAccountantError)
	Accounts(ctx *gin.Context) ([]*Entity, *definitions.WalletAccountantError)
}

type QueryMediator struct {
	repository ReadModeler
}

func NewQueryMediator(repository ReadModeler) *QueryMediator {
	return &QueryMediator{repository: repository}
}

func (mediator QueryMediator) Account(ctx *gin.Context, accountId *Id) (*Entity, *definitions.WalletAccountantError) {
	entity, err := mediator.repository.GetByAccountId(ctx, accountId)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entity, nil
}

func (mediator QueryMediator) Accounts(ctx *gin.Context) ([]*Entity, *definitions.WalletAccountantError) {
	entities, err := mediator.repository.GetAll(ctx)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entities, nil
}
