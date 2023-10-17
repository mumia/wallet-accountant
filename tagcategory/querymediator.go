package tagcategory

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediator{}

type QueryMediatorer interface {
	Tags(ctx *gin.Context) ([]*CategoryEntity, *definitions.WalletAccountantError)
}

type QueryMediator struct {
	repository ReadModeler
}

func NewQueryMediator(repository ReadModeler) *QueryMediator {
	return &QueryMediator{repository: repository}
}

func (mediator QueryMediator) Tags(ctx *gin.Context) ([]*CategoryEntity, *definitions.WalletAccountantError) {
	entities, err := mediator.repository.GetAll(ctx)
	if err != nil {
		return nil, GenericError(err, nil)
	}

	return entities, nil
}
