package tagcategory

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediator{}

type QueryMediatorer interface {
	Tags(ctx *gin.Context, filters FiltersTransferObject) ([]*CategoryEntity, *definitions.WalletAccountantError)
}

type QueryMediator struct {
	repository ReadModeler
}

func NewQueryMediator(repository ReadModeler) *QueryMediator {
	return &QueryMediator{repository: repository}
}

func (mediator QueryMediator) Tags(
	ctx *gin.Context,
	filters FiltersTransferObject,
) ([]*CategoryEntity, *definitions.WalletAccountantError) {
	if len(filters.Filters) > 0 {
		return mediator.filterTags(ctx, filters)
	}

	entities, err := mediator.repository.GetAll(ctx)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entities, nil
}

func (mediator QueryMediator) filterTags(
	ctx *gin.Context,
	filters FiltersTransferObject,
) ([]*CategoryEntity, *definitions.WalletAccountantError) {
	var tagIds []TagId
	for _, filter := range filters.Filters {
		tagIds = append(tagIds, uuid.MustParse(filter))
	}

	entities, err := mediator.repository.GetByTagIds(ctx, tagIds)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entities, nil
}
