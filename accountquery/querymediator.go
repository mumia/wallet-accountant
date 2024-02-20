package accountquery

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/account"
	"walletaccountant/accountreadmodel"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediator{}

type QueryMediatorer interface {
	Account(ctx *gin.Context, accountId *account.Id) (*accountreadmodel.Entity, *definitions.WalletAccountantError)
	Accounts(ctx *gin.Context) ([]*accountreadmodel.Entity, *definitions.WalletAccountantError)
}

type QueryMediator struct {
	repository accountreadmodel.ReadModeler
}

func NewQueryMediator(repository accountreadmodel.ReadModeler) *QueryMediator {
	return &QueryMediator{repository: repository}
}

func (mediator QueryMediator) Account(ctx *gin.Context, accountId *account.Id) (*accountreadmodel.Entity, *definitions.WalletAccountantError) {
	entity, err := mediator.repository.GetByAccountId(ctx, accountId)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entity, nil
}

func (mediator QueryMediator) Accounts(ctx *gin.Context) ([]*accountreadmodel.Entity, *definitions.WalletAccountantError) {
	entities, err := mediator.repository.GetAll(ctx)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entities, nil
}
